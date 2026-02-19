# Phase 5: Provider Health & Broadcast Fallback

<objective>
Add provider health visibility for users and broadcast redundancy for BSC/SOL transactions. Wire the existing provider_health DB table and circuit breaker infrastructure into the scanner pool so that provider status is tracked in real time. Expose via a new API endpoint and update the frontend ProviderStatus component to show live data.
</objective>

## Audit IDs Covered
- **D2**: Provider health endpoint (from robustness audit)
- **Plan #24**: Health dashboard visibility

## Tasks

### Task 1: Wire DB Health Recording into Scanner Pool

**Goal**: The Pool already has in-memory circuit breakers. Wire it to also persist health state to the `provider_health` DB table on every success/failure.

**Changes to `internal/scanner/pool.go`**:
1. Add optional `*db.DB` field to `Pool` struct
2. Add `SetDB(database *db.DB)` method (optional setter, keeps backward compatibility with tests)
3. After `cb.RecordSuccess()`, call `db.RecordProviderSuccess(provider.Name())` (non-blocking, log-only errors)
4. After `cb.RecordFailure()`, call `db.RecordProviderFailure(provider.Name(), err.Error())` (non-blocking)
5. On Pool creation with DB, upsert initial `provider_health` rows for all providers (chain, name, type=scan, status=healthy, circuit_state=closed)

**Non-blocking pattern** (critical — DB writes must not slow scans):
```go
func (p *Pool) recordSuccess(providerName string) {
    if p.db == nil { return }
    if err := p.db.RecordProviderSuccess(providerName); err != nil {
        slog.Error("failed to record provider success", "provider", providerName, "error", err)
    }
}
```

Also update the DB row's `circuit_state` field to match the in-memory circuit breaker state. Add a helper method `UpdateProviderCircuitState(name, state string)` to `db/provider_health.go`.

**Files modified**:
- `internal/scanner/pool.go` — Add DB field, health recording calls
- `internal/db/provider_health.go` — Add `UpdateProviderCircuitState` method

<verification>
- Pool with nil DB works exactly as before (no panics, no changes)
- Pool with DB records health on every provider call
- Circuit breaker state changes are reflected in DB
- `go test ./internal/scanner/...` passes
</verification>

---

### Task 2: Provider Health API Endpoint

**Goal**: `GET /api/health/providers` returns per-chain provider health status.

**New file `internal/api/handlers/provider_health.go`**:
```go
func GetProviderHealth(database *db.DB) http.HandlerFunc {
    // Read all provider_health rows from DB
    // Group by chain
    // Return as JSON
}
```

**Response format**:
```json
{
  "data": {
    "BTC": [
      {
        "name": "blockstream",
        "chain": "BTC",
        "type": "scan",
        "status": "healthy",
        "circuitState": "closed",
        "consecutiveFails": 0,
        "lastSuccess": "2026-02-19T10:30:00Z",
        "lastError": "",
        "lastErrorMsg": ""
      }
    ],
    "BSC": [...],
    "SOL": [...]
  }
}
```

**Wire into router** (`internal/api/router.go`):
- Add `r.Get("/health/providers", handlers.GetProviderHealth(database))` inside the `/api` route group

**Files created/modified**:
- `internal/api/handlers/provider_health.go` — NEW handler
- `internal/api/router.go` — New route

<verification>
- `GET /api/health/providers` returns valid JSON with provider status grouped by chain
- Empty DB returns empty arrays per chain (not errors)
- After a scan runs, provider statuses are populated
</verification>

---

### Task 3: BSC Broadcast Fallback

**Goal**: On BSC transaction broadcast failure, retry with a secondary RPC endpoint.

**Approach**: Create a `FallbackEthClient` wrapper that implements `EthClientWrapper`. For `SendTransaction`, tries primary first, then fallback. All other methods use primary only.

**New file `internal/tx/bsc_fallback.go`**:
```go
type FallbackEthClient struct {
    primary  EthClientWrapper
    fallback EthClientWrapper
}

func (f *FallbackEthClient) SendTransaction(ctx context.Context, tx *types.Transaction) error {
    err := f.primary.SendTransaction(ctx, tx)
    if err == nil {
        return nil
    }
    slog.Warn("BSC primary broadcast failed, trying fallback",
        "txHash", tx.Hash().Hex(),
        "primaryError", err,
    )
    fallbackErr := f.fallback.SendTransaction(ctx, tx)
    if fallbackErr == nil {
        slog.Info("BSC fallback broadcast succeeded", "txHash", tx.Hash().Hex())
        return nil
    }
    slog.Error("BSC fallback broadcast also failed",
        "txHash", tx.Hash().Hex(),
        "fallbackError", fallbackErr,
    )
    return err // return primary error
}

// All other methods delegate to primary
func (f *FallbackEthClient) PendingNonceAt(...) { return f.primary.PendingNonceAt(...) }
// ... etc
```

**Wire in `cmd/server/main.go`**:
- After creating the primary `ethclient`, create a secondary one:
  - Mainnet fallback: `rpc.ankr.com/bsc`
  - Testnet: no fallback available (single testnet URL), use primary only
- Wrap both into `FallbackEthClient`
- Pass to BSC services

**New constants in `internal/config/constants.go`**:
```go
// BSC Broadcast Fallback
BscRPCFallbackMainnetURL = "https://rpc.ankr.com/bsc"
```
Note: `BscRPCMainnetURL2` already exists as `rpc.ankr.com/bsc` — use that directly.

**Files created/modified**:
- `internal/tx/bsc_fallback.go` — NEW: FallbackEthClient wrapper
- `internal/tx/bsc_fallback_test.go` — NEW: Tests for fallback behavior
- `cmd/server/main.go` — Wire fallback client for BSC

<verification>
- When primary BSC broadcast fails, fallback is attempted
- When both fail, primary error is returned
- When primary succeeds, fallback is not called
- Non-SendTransaction methods always use primary
- Testnet correctly uses primary only (no fallback)
- `go test ./internal/tx/...` passes
</verification>

---

### Task 4: SOL Broadcast Fallback

**Goal**: On SOL transaction broadcast failure, retry with remaining RPC URLs.

**Approach**: Modify `DefaultSOLRPCClient.SendTransaction` to try all available URLs when broadcast fails, instead of just one round-robin URL. Other RPC methods continue using single round-robin.

**Changes to `internal/tx/sol_tx.go`**:
- Add `doRPCAllURLs(ctx, method, params)` method that tries each URL in order
- Modify `SendTransaction` to use `doRPCAllURLs` instead of `doRPC`
- On first success, return immediately
- If all URLs fail, return the first error

**Why not a wrapper**: Unlike BSC (separate ethclient instances), SOL uses a single HTTP-based RPC client that already manages multiple URLs. Adding retry logic directly is simpler.

**Files modified**:
- `internal/tx/sol_tx.go` — Add `doRPCAllURLs`, modify `SendTransaction`

<verification>
- SOL broadcast with multiple URLs tries all on failure
- SOL broadcast with single URL behaves as before
- First successful URL short-circuits remaining attempts
- Non-broadcast RPC methods unaffected
- `go test ./internal/tx/...` passes
</verification>

---

### Task 5: Frontend — Live Provider Health

**Goal**: Replace the hardcoded ProviderStatus component with real API data.

**Changes to `web/src/lib/types.ts`**:
```typescript
// Provider health from GET /api/health/providers
export interface ProviderHealth {
    name: string;
    chain: Chain;
    type: string;
    status: 'healthy' | 'degraded' | 'down';
    circuitState: 'closed' | 'open' | 'half_open';
    consecutiveFails: number;
    lastSuccess: string;
    lastError: string;
    lastErrorMsg: string;
}

export type ProviderHealthMap = Record<Chain, ProviderHealth[]>;
```

**Changes to `web/src/lib/utils/api.ts`**:
```typescript
export function getProviderHealth(): Promise<APIResponse<ProviderHealthMap>> {
    return api.get<ProviderHealthMap>('/health/providers');
}
```

**Changes to `web/src/lib/components/scan/ProviderStatus.svelte`**:
- Replace hardcoded array with API fetch on mount
- Color-coded status dots: green (healthy/closed), yellow (degraded/half_open), red (down/open)
- Show provider name, chain, status, and circuit state
- Show last error message on hover/tooltip for degraded/down providers
- Auto-refresh when scan page mounts (already triggered by component lifecycle)
- Handle loading and error states gracefully

**Files modified**:
- `web/src/lib/types.ts` — Add ProviderHealth types
- `web/src/lib/utils/api.ts` — Add getProviderHealth function
- `web/src/lib/components/scan/ProviderStatus.svelte` — Wire to API, dynamic rendering

<verification>
- Provider status section shows real data from API
- Colors match provider health (green/yellow/red)
- Empty state (no providers in DB yet) shows graceful message
- Circuit breaker state is displayed
- `cd web && npm run build` succeeds
</verification>

---

### Task 6: Initialize Provider Health Rows on Scanner Setup

**Goal**: When the scanner is initialized, create provider_health DB rows for all configured providers so the health endpoint has data before the first scan.

**Changes to `internal/scanner/scanner.go`** (or `setup.go` if that's where `SetupScanner` lives):
- After creating provider pools, call `pool.SetDB(database)` to enable health recording
- This triggers the upsert of initial provider_health rows

Also need to ensure pools in the `Scanner` struct can accept the DB reference.

**Files modified**:
- `internal/scanner/scanner.go` — Wire DB into pools at setup time
- `internal/scanner/pool.go` — Add initial row upsert in `SetDB`

<verification>
- After server start, `provider_health` table has rows for all configured providers
- All providers start as "healthy" with circuit_state "closed"
- Health endpoint returns data even before first scan
</verification>

---

## Success Criteria

- [ ] `GET /api/health/providers` returns per-chain provider health with status, circuit state, and error history
- [ ] Scanner pool records success/failure to `provider_health` table on every provider call
- [ ] BSC broadcast falls back to Ankr RPC on primary broadcast failure (mainnet only)
- [ ] SOL broadcast tries all configured RPC URLs before reporting failure
- [ ] Frontend ProviderStatus component shows live health data with color-coded indicators
- [ ] All existing tests pass: `go test ./...`
- [ ] Frontend builds: `cd web && npm run build`
- [ ] Provider health rows are pre-populated on server startup

## Estimated Tests: ~12
- Pool health recording: 3 tests (success recording, failure recording, nil DB)
- BSC fallback client: 4 tests (primary succeeds, primary fails+fallback succeeds, both fail, non-send delegates to primary)
- SOL broadcast fallback: 2 tests (single URL failure, multi-URL fallback)
- Provider health endpoint: 2 tests (empty DB, populated DB)
- Circuit state sync: 1 test (breaker trips → DB updated)
