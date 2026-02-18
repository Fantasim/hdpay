# Phase 3: TX Safety — Core

## Objective

Fix the 5 most critical TX safety issues: concurrent send protection, BTC confirmation polling, SOL confirmation error handling, in-flight TX persistence, and SOL blockhash caching.

## Audit IDs Covered

A1 (concurrent send), A2 (BTC confirmation), A3 (SOL confirmation uncertainty), A4 (in-flight persistence), A5 (SOL blockhash refresh)

---

## Tasks

### 3.1 New Constants and Error Types

Add constants and errors needed by the rest of the phase.

**File: `internal/config/constants.go`**

Add to new group `// BTC Confirmation Polling`:
```go
BTCConfirmationTimeout      = 10 * time.Minute
BTCConfirmationPollInterval = 15 * time.Second
```

Add to new group `// SOL Blockhash Cache`:
```go
SOLBlockhashCacheTTL = 20 * time.Second
```

**File: `internal/config/errors.go`**

Add sentinel errors:
```go
ErrBTCConfirmationTimeout  = errors.New("BTC transaction confirmation timeout")
ErrSOLConfirmationUncertain = errors.New("SOL confirmation uncertain due to RPC error")
```

Add error codes:
```go
ErrorBTCConfirmationTimeout  = "ERROR_BTC_CONFIRMATION_TIMEOUT"
ErrorSOLConfirmationUncertain = "ERROR_SOL_CONFIRMATION_UNCERTAIN"
ErrorSendBusy                = "ERROR_SEND_BUSY"
```

**Verification:** `go build ./internal/config/...` passes.

---

### 3.2 Concurrent Send Mutex (A1)

Prevent concurrent sweep executions for the same chain. Two simultaneous BTC sweeps could spend the same UTXOs; two BSC sweeps could use the same nonce.

**File: `internal/api/handlers/send.go`**

1. Add a `chainLocks` field to `SendDeps`:
   ```go
   ChainLocks map[models.Chain]*sync.Mutex
   ```

2. Initialize in the constructor or wherever `SendDeps` is created (likely `cmd/server/main.go`):
   ```go
   ChainLocks: map[models.Chain]*sync.Mutex{
       models.ChainBTC: {},
       models.ChainBSC: {},
       models.ChainSOL: {},
   },
   ```

3. In `ExecuteSend` handler, after validation and before `executeSweep`:
   ```go
   mu := deps.ChainLocks[req.Chain]
   if !mu.TryLock() {
       slog.Warn("send already in progress for chain", "chain", req.Chain)
       writeError(w, http.StatusConflict, config.ErrorSendBusy,
           fmt.Sprintf("send operation already in progress for %s", req.Chain))
       return
   }
   defer mu.Unlock()
   ```

**File: `cmd/server/main.go`**

Add `ChainLocks` initialization to `SendDeps` construction.

**Tests (in `internal/api/handlers/send_test.go`):**
- `TestExecuteSendConcurrentBlock`: Send two parallel requests for the same chain. First should succeed (or proceed). Second should get 409.
- `TestExecuteSendDifferentChainsParallel`: Two parallel requests for different chains should both proceed.

**Verification:** Run concurrent HTTP test, verify 409 on second request.

---

### 3.3 BTC Confirmation Polling (A2)

Currently `BTCConsolidationService.Execute()` broadcasts and returns immediately — no confirmation check. Add polling to verify the TX appears in the mempool and gets confirmed.

**File: `internal/tx/btc_tx.go`**

Add `WaitForBTCConfirmation` function:
```go
func WaitForBTCConfirmation(ctx context.Context, client *http.Client, providerURLs []string, txHash string) error
```

Logic:
1. Create a context with `BTCConfirmationTimeout`
2. Poll each provider (round-robin) via `GET /tx/{txHash}/status`
3. Parse the JSON response: `{ "confirmed": true, "block_height": 123 }`
4. If `confirmed: true` → return nil (success)
5. If response is valid but not confirmed → sleep `BTCConfirmationPollInterval` and retry
6. If HTTP error → log warning, try next provider
7. If context timeout → return `ErrBTCConfirmationTimeout`

The function does NOT fail the sweep if confirmation times out — it logs a warning and the TX status gets set to `confirming` (not `failed`), since the broadcast already succeeded.

Wire into `BTCConsolidationService.Execute()` after broadcast step:
```go
// 7b. Wait for confirmation (best-effort — timeout is not a failure)
if err := WaitForBTCConfirmation(ctx, httpClient, providerURLs, txHash); err != nil {
    slog.Warn("BTC confirmation polling timed out or failed",
        "txHash", txHash,
        "error", err,
    )
    // Still success — broadcast was accepted
} else {
    slog.Info("BTC transaction confirmed", "txHash", txHash)
}
```

Add `httpClient` and `confirmationURLs` fields to `BTCConsolidationService` struct.

**Tests (in `internal/tx/btc_tx_test.go`):**
- `TestWaitForBTCConfirmation_Confirmed`: Mock server returns confirmed after 2 polls
- `TestWaitForBTCConfirmation_Timeout`: Mock server always returns unconfirmed → expect `ErrBTCConfirmationTimeout`
- `TestWaitForBTCConfirmation_ProviderFallback`: First provider fails, second confirms

**Verification:** Run tests with mock HTTP server.

---

### 3.4 SOL Confirmation Error Handling (A3)

Fix `WaitForSOLConfirmation` to distinguish RPC errors from TX failures. When the RPC itself errors during polling (network issue, rate limit), the TX may still have succeeded. Mark as `uncertain` instead of `failed`.

**File: `internal/tx/sol_tx.go`**

Modify `WaitForSOLConfirmation`:
1. Track consecutive RPC errors with a counter
2. If RPC errors exceed 3 consecutive without ever getting a status → return `ErrSOLConfirmationUncertain` (not the raw error)
3. On-chain failure (`status.Err != nil`) still returns `ErrSOLTxFailed`
4. The function now returns `(slot uint64, err error)` where:
   - `err == nil` → confirmed
   - `err == ErrSOLTxFailed` → TX failed on-chain
   - `err == ErrSOLConfirmationTimeout` → timeout
   - `err == ErrSOLConfirmationUncertain` → RPC too flaky to determine status

Modify callers (`sweepNativeAddress`, `sweepTokenAddress`):
```go
slot, err := WaitForSOLConfirmation(ctx, s.rpcClient, txResult.TxSignature)
if err != nil {
    if errors.Is(err, config.ErrSOLConfirmationUncertain) {
        txResult.Status = config.TxStateUncertain
        txResult.Error = "confirmation uncertain: RPC errors during polling — check explorer"
    } else {
        txResult.Status = config.TxStateFailed
        txResult.Error = fmt.Sprintf("confirmation: %s", err)
    }
    s.recordSOLTransaction(addr, txResult.TxSignature, txResult.Amount, dest, token, txResult.Status)
    return txResult
}
```

**Tests (in `internal/tx/sol_tx_test.go`):**
- `TestWaitForSOLConfirmation_RPCErrors_Uncertain`: Mock RPC always errors → returns `ErrSOLConfirmationUncertain`
- `TestWaitForSOLConfirmation_OnChainFailure`: Mock RPC returns `Err` in status → returns `ErrSOLTxFailed`
- `TestWaitForSOLConfirmation_IntermittentErrors`: Mock RPC errors once then confirms → success

**Verification:** Run tests, verify `uncertain` vs `failed` distinction.

---

### 3.5 In-Flight TX Persistence (A4)

Use the `tx_state` table (created in Phase 1) to track every individual TX through its lifecycle. This enables crash recovery and provides visibility into pending/uncertain TXs.

#### 3.5a: Wire tx_state into BSC sweep

**File: `internal/tx/bsc_tx.go`**

Add `sweepID` parameter to `ExecuteNativeSweep` and `ExecuteTokenSweep`.
In each `sweepNativeAddress` / `sweepTokenAddress`:
1. At the start: `CreateTxState` with status `pending`, using `GenerateTxStateID()`
2. Before broadcast: `UpdateTxStatus` → `broadcasting`
3. After broadcast success: `UpdateTxStatus` → `confirming` + txHash
4. After receipt: `UpdateTxStatus` → `confirmed` or `failed` (with error)
5. On receipt timeout (broadcast succeeded): `UpdateTxStatus` → `uncertain`

Helper function:
```go
func GenerateTxStateID() string {
    // Use same pattern as GenerateSweepID but with 8 bytes for shorter IDs
    b := make([]byte, 8)
    rand.Read(b)
    return hex.EncodeToString(b)
}
```
Add to `internal/tx/sweep.go`.

#### 3.5b: Wire tx_state into SOL sweep

**File: `internal/tx/sol_tx.go`**

Same pattern as BSC. Add `sweepID` parameter to `ExecuteNativeSweep` and `ExecuteTokenSweep`.
In each per-address sweep function, write tx_state at each stage.

#### 3.5c: Wire tx_state into BTC sweep

**File: `internal/tx/btc_tx.go`**

BTC is a single-TX consolidation, so one tx_state row per sweep:
1. Before broadcast: `CreateTxState` with status `pending` (one row for the consolidated TX)
2. Before broadcast: `UpdateTxStatus` → `broadcasting`
3. After broadcast: `UpdateTxStatus` → `confirming` + txHash
4. After confirmation polling: `UpdateTxStatus` → `confirmed` / `uncertain`

#### 3.5d: ExecuteSend generates sweepID

**File: `internal/api/handlers/send.go`**

In `ExecuteSend`, generate a `sweepID` before calling `executeSweep`. Pass it through to the chain-specific execute functions. The sweepID groups all individual TX states for a single user-initiated sweep.

Update the `executeSweep` and chain-specific functions to accept `sweepID string` parameter.

#### 3.5e: Pending TX endpoints

**File: `internal/api/handlers/send.go`**

New handlers:
```go
// GetPendingTxStates handles GET /api/send/pending
// Query params: ?chain=BTC (optional, defaults to all chains)
func GetPendingTxStates(deps *SendDeps) http.HandlerFunc

// DismissTxState handles POST /api/send/dismiss/{id}
// Marks an uncertain TX as resolved (user confirmed in explorer)
func DismissTxState(deps *SendDeps) http.HandlerFunc
```

`GetPendingTxStates`:
- If `?chain=` provided: call `deps.DB.GetPendingTxStates(chain)`
- Otherwise: call for all 3 chains, merge results
- Return as `APIResponse{Data: txStates}`

`DismissTxState`:
- Get `{id}` from URL param
- Call `deps.DB.UpdateTxStatus(id, "dismissed", "", "")`
- Return 200 OK

Add `dismissed` to the `tx_state` statuses:
- `internal/config/constants.go`: `TxStateDismissed = "dismissed"`

**File: `internal/api/router.go`**

Add routes:
```go
r.Get("/pending", handlers.GetPendingTxStates(sendDeps))
r.Post("/dismiss/{id}", handlers.DismissTxState(sendDeps))
```

**File: `internal/db/tx_state.go`**

Add `GetAllPendingTxStates()` method that returns pending/broadcasting/confirming/uncertain for all chains:
```go
func (d *DB) GetAllPendingTxStates() ([]TxStateRow, error)
```

**Tests:**
- `TestGetPendingTxStates`: Insert some pending + confirmed tx_states, verify only pending returned
- `TestDismissTxState`: Create uncertain tx, dismiss it, verify status is "dismissed"
- `TestTxStateLifecycle`: Simulate full pending→broadcasting→confirming→confirmed flow

**Verification:** Run all tests. Check that `GET /api/send/pending` returns expected data.

---

### 3.6 SOL Blockhash Cache (A5)

Currently each `sweepNativeAddress`/`sweepTokenAddress` fetches a fresh blockhash. For a 50-address sweep, that's 50 blockhash RPC calls, most redundant. Add caching with a 20s TTL.

**File: `internal/tx/sol_tx.go`**

Add fields to `SOLConsolidationService`:
```go
type SOLConsolidationService struct {
    // ... existing fields ...
    blockhashCache    [32]byte
    blockhashCachedAt time.Time
    blockhashMu       sync.Mutex
}
```

Add method:
```go
func (s *SOLConsolidationService) getOrRefreshBlockhash(ctx context.Context) ([32]byte, error) {
    s.blockhashMu.Lock()
    defer s.blockhashMu.Unlock()

    if time.Since(s.blockhashCachedAt) < config.SOLBlockhashCacheTTL {
        slog.Debug("using cached SOL blockhash",
            "age", time.Since(s.blockhashCachedAt).Round(time.Millisecond),
        )
        return s.blockhashCache, nil
    }

    blockhash, _, err := s.rpcClient.GetLatestBlockhash(ctx)
    if err != nil {
        return [32]byte{}, err
    }

    s.blockhashCache = blockhash
    s.blockhashCachedAt = time.Now()

    slog.Debug("refreshed SOL blockhash",
        "blockhash", base58.Encode(blockhash[:]),
    )

    return blockhash, nil
}
```

Replace direct `s.rpcClient.GetLatestBlockhash(ctx)` calls in `sweepNativeAddress` and `sweepTokenAddress` with `s.getOrRefreshBlockhash(ctx)`.

**Tests (in `internal/tx/sol_tx_test.go`):**
- `TestSOLBlockhashCache_UsesCachedValue`: Call twice within 20s, verify only 1 RPC call made
- `TestSOLBlockhashCache_RefreshesAfterTTL`: Call, wait 20s, call again, verify 2 RPC calls

**Verification:** Run tests, verify fewer blockhash RPCs during multi-address sweep.

---

## Files Created/Modified Summary

| File | Change |
|------|--------|
| `internal/config/constants.go` | BTC confirmation, SOL blockhash TTL, TxStateDismissed |
| `internal/config/errors.go` | BTC/SOL confirmation errors + codes |
| `internal/api/handlers/send.go` | Chain mutex, sweepID, pending/dismiss handlers |
| `internal/api/router.go` | New routes: pending, dismiss |
| `internal/tx/btc_tx.go` | WaitForBTCConfirmation, tx_state wiring, httpClient field |
| `internal/tx/bsc_tx.go` | tx_state wiring in sweep functions |
| `internal/tx/sol_tx.go` | Uncertain status, blockhash cache, tx_state wiring |
| `internal/tx/sweep.go` | GenerateTxStateID |
| `internal/db/tx_state.go` | GetAllPendingTxStates |
| `cmd/server/main.go` | ChainLocks initialization |

## Tests: ~25

| Area | Count |
|------|-------|
| Concurrent mutex | 2 |
| BTC confirmation polling | 3 |
| SOL confirmation uncertainty | 3 |
| TX state lifecycle | 3 |
| Pending/dismiss endpoints | 2 |
| SOL blockhash cache | 2 |
| Integration (end-to-end sweep with tx_state) | ~3 |
| Various edge cases | ~7 |

## Success Criteria

- [ ] Concurrent `POST /api/send/execute` for the same chain returns 409 Conflict
- [ ] BTC sweep polls for confirmation after broadcast (with configurable timeout)
- [ ] SOL RPC errors during confirmation polling → `uncertain` status (not `failed`)
- [ ] Every TX goes through tx_state lifecycle: pending→broadcasting→confirming→confirmed/failed/uncertain
- [ ] `GET /api/send/pending` returns non-terminal TXs
- [ ] `POST /api/send/dismiss/{id}` resolves uncertain TXs
- [ ] SOL blockhash is cached for 20s, reducing redundant RPC calls
- [ ] All tests pass: `go test ./...`
- [ ] No hardcoded constants — all values in config/constants.go

## Reminders

- Constants/errors/config → central files only
- Check utils before writing new functions
- All tx_state writes are non-blocking: if DB write fails, log error but don't abort the sweep
- Private keys are never stored in tx_state
- Update CHANGELOG.md before committing
