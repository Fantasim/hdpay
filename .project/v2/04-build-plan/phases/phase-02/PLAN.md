# Phase 2: Scanner Resilience

<objective>
Fix all scanner/provider reliability issues identified in the V2 robustness audit. Replace early-return error handling with error collection, validate partial results, wire circuit breakers, add Retry-After parsing, make scan state atomic, decouple native/token failures, surface token errors via SSE, add SSE resync on reconnect, and add exponential backoff when all providers fail.
</objective>

## Audit IDs Covered
B1, B3, B4, B5 (wiring), B6, B7, B8, B10, B11

## Dependencies
- Phase 1 deliverables: `CircuitBreaker`, `TransientError`, `BalanceResult.Error` field, `BalanceResult.Source` field

---

## Tasks

### Task 2.1: New Constants & Error Types

Add constants and errors needed by this phase.

**File: `internal/config/constants.go`**
```go
// Scanner Resilience (Phase 2)
const (
    ExponentialBackoffBase    = 1 * time.Second  // base delay for backoff
    ExponentialBackoffMax     = 30 * time.Second  // max delay cap
    PartialResultRetryMax     = 1                 // max retry attempts for missing results
    SSEResyncBufferSize       = 100               // last N events kept for resync
)
```

**File: `internal/config/errors.go`**
```go
// Scanner Resilience
var (
    ErrPartialResults    = errors.New("partial results returned")
    ErrAllProvidersFailed = errors.New("all providers failed")
)

const (
    ErrorPartialResults     = "ERROR_PARTIAL_RESULTS"
    ErrorAllProvidersFailed = "ERROR_ALL_PROVIDERS_FAILED"
    ErrorTokenScanFailed    = "ERROR_TOKEN_SCAN_FAILED"
)
```

**Verification:** Compiles cleanly. Existing tests pass.

---

### Task 2.2: Provider Error Collection (B1) — BTC Providers

Change `btc_blockstream.go` and `btc_mempool.go` to continue on per-address errors instead of early-returning. Failed addresses get `BalanceResult.Error` populated.

**Current behavior (both providers):**
```go
for _, addr := range addresses {
    balance, err := p.fetchAddressBalance(ctx, addr.Address)
    if err != nil {
        return results, fmt.Errorf(...)  // EARLY RETURN — remaining addresses never queried
    }
    results = append(results, BalanceResult{...})
}
```

**New behavior:**
```go
var errs []error
for _, addr := range addresses {
    if err := p.rl.Wait(ctx); err != nil {
        // Context cancelled — stop iteration (not retriable)
        return results, fmt.Errorf("rate limiter wait: %w", err)
    }

    balance, err := p.fetchAddressBalance(ctx, addr.Address)
    if err != nil {
        slog.Warn("address balance fetch failed",
            "provider", p.Name(),
            "address", addr.Address,
            "index", addr.AddressIndex,
            "error", err,
        )
        errs = append(errs, err)
        results = append(results, BalanceResult{
            Address:      addr.Address,
            AddressIndex: addr.AddressIndex,
            Balance:      "0",
            Error:        err.Error(),
            Source:       p.Name(),
        })
        continue
    }

    results = append(results, BalanceResult{
        Address:      addr.Address,
        AddressIndex: addr.AddressIndex,
        Balance:      balance,
        Source:       p.Name(),
    })
}

// All addresses queried. Return results + aggregated error if any failed.
if len(errs) == len(addresses) {
    return results, fmt.Errorf("all addresses failed: %w", errs[0])
}
return results, nil
```

**Key rules:**
- Context cancellation (rate limiter Wait returns ctx.Err) → hard return (scan is stopping)
- Individual address failure → annotate result, continue loop
- 100% failure → return results AND an error (pool should try next provider)
- Partial failure → return results, nil error (partial data is better than none)

**Files modified:**
- `internal/scanner/btc_blockstream.go` — `FetchNativeBalances()`
- `internal/scanner/btc_mempool.go` — `FetchNativeBalances()`

**Also update `fetchAddressBalance()` in both files** to wrap rate limit / unavailable errors as `TransientError`:
```go
if resp.StatusCode == http.StatusTooManyRequests {
    retryAfter := parseRetryAfter(resp.Header)  // Task 2.5
    return "0", config.NewTransientErrorWithRetry(config.ErrProviderRateLimit, retryAfter)
}
```

**Verification:** Unit test: 5-address batch, address #3 returns error → results has 5 entries, #3 has Error set, others have balances. No early return.

---

### Task 2.3: Provider Error Collection (B1) — BSC Providers

Same error collection pattern for BSC.

**`bsc_bscscan.go` — `FetchNativeBalances()`:**
This is a batch call (up to 20 addresses in one HTTP request). The batch call itself can fail (HTTP error, rate limit) — that's a hard failure for the whole batch. But the result mapping can have missing addresses.

Changes:
- After mapping results, detect missing addresses (requested but not in response)
- Missing addresses → BalanceResult with Error="address not returned by provider"
- If BscScan `data.Status != "1"`, wrap as TransientError for rate limit cases

**`bsc_bscscan.go` — `FetchTokenBalances()`:**
Currently loops one-at-a-time. Apply same error collection as BTC:
- Per-address failure → annotate, continue
- All failures → return results + error
- Partial → return results, nil

**`bsc_rpc.go` — `FetchNativeBalances()` and `FetchTokenBalances()`:**
Same error collection pattern. Also handle `callBalanceOf` returning < 32 bytes:
- Currently silently returns 0. Change to: set `BalanceResult.Error = "malformed contract response"`.

**Files modified:**
- `internal/scanner/bsc_bscscan.go`
- `internal/scanner/bsc_rpc.go`

**Verification:** Unit test: 20-address batch to BscScan, response returns only 18 → 2 missing addresses get Error annotation. Token fetch: 5 addresses, #2 fails → continues, returns 5 results.

---

### Task 2.4: Provider Error Collection + Partial Result Validation (B1, B3) — SOL Provider

SOL provider has unique considerations:
1. `getMultipleAccounts` can return fewer results than requested (B3)
2. JSON unmarshal errors for individual accounts should annotate, not fail

**`sol_rpc.go` — `FetchNativeBalances()`:**

Changes:
- After RPC call, validate: `len(respBody.Result.Value) == len(pubkeys)`
- If fewer results:
  - Log warning with expected vs actual count
  - Returned results correspond to indices 0..len(Value)-1
  - Missing indices → BalanceResult with Error="not returned by RPC"
  - Return results with nil error (partial is usable)
- JSON unmarshal error per account → already logs WARN, but now also set `BalanceResult.Error`

```go
resultCount := len(respBody.Result.Value)
requestCount := len(addresses)

if resultCount < requestCount {
    slog.Warn("solana RPC returned partial results",
        "provider", p.name,
        "requested", requestCount,
        "received", resultCount,
    )
}

for i := 0; i < requestCount; i++ {
    if i >= resultCount {
        // Missing from response
        results = append(results, BalanceResult{
            Address:      addresses[i].Address,
            AddressIndex: addresses[i].AddressIndex,
            Balance:      "0",
            Error:        "not returned by RPC",
            Source:       p.Name(),
        })
        continue
    }
    // ... existing parsing with error annotation on unmarshal failure
}
```

**`sol_rpc.go` — `FetchTokenBalances()`:**
Same partial result validation for ATA queries.

**Files modified:**
- `internal/scanner/sol_rpc.go`

**Verification:** Unit test: Request 100 accounts, mock returns 80 → 80 have balances, 20 have Error="not returned by RPC". Unmarshal error → Error annotation set.

---

### Task 2.5: Retry-After Header Parsing (B6)

Add a utility function to parse `Retry-After` headers from HTTP responses.

**New file: `internal/scanner/retry_after.go`**

```go
package scanner

import (
    "net/http"
    "strconv"
    "time"
)

// parseRetryAfter extracts duration from Retry-After header.
// Supports seconds format (e.g., "30") and HTTP-date format.
// Returns 0 if header is missing or unparseable.
func parseRetryAfter(header http.Header) time.Duration {
    val := header.Get("Retry-After")
    if val == "" {
        return 0
    }

    // Try seconds format first (most common for APIs)
    if seconds, err := strconv.Atoi(val); err == nil && seconds > 0 {
        return time.Duration(seconds) * time.Second
    }

    // Try HTTP-date format
    if t, err := http.ParseTime(val); err == nil {
        d := time.Until(t)
        if d > 0 {
            return d
        }
    }

    return 0
}
```

**Update all providers** that handle 429 responses to use this:
- `btc_blockstream.go` — `fetchAddressBalance()`
- `btc_mempool.go` — `fetchAddressBalance()`
- `bsc_bscscan.go` — `FetchNativeBalances()`, `fetchTokenBalance()`
- `bsc_rpc.go` — not applicable (uses ethclient, no raw HTTP)
- `sol_rpc.go` — `doRPCCall()`

Pattern:
```go
if resp.StatusCode == http.StatusTooManyRequests {
    retryAfter := parseRetryAfter(resp.Header)
    slog.Warn("provider rate limited",
        "provider", p.Name(),
        "retryAfter", retryAfter,
    )
    return "0", config.NewTransientErrorWithRetry(config.ErrProviderRateLimit, retryAfter)
}
```

**New file: `internal/scanner/retry_after_test.go`**
- Missing header → 0
- "30" → 30s
- "0" → 0
- HTTP-date in future → correct duration
- HTTP-date in past → 0
- Garbage value → 0

**Verification:** 6 tests pass. All 429 handlers wrap RetryAfter.

---

### Task 2.6: Circuit Breaker Wiring in Pool (B5)

Wire the circuit breaker (from Phase 1) into the Pool so failed providers are skipped.

**Changes to `internal/scanner/pool.go`:**

Add per-provider circuit breakers:
```go
type Pool struct {
    providers []Provider
    breakers  []*CircuitBreaker  // one per provider, same index
    current   atomic.Int32
    chain     models.Chain
}

func NewPool(chain models.Chain, providers ...Provider) *Pool {
    breakers := make([]*CircuitBreaker, len(providers))
    for i, p := range providers {
        breakers[i] = NewCircuitBreaker(p.Name(),
            config.CircuitBreakerThreshold,
            config.CircuitBreakerCooldown,
        )
    }
    // ...
}
```

Update `FetchNativeBalances` and `FetchTokenBalances`:
```go
func (p *Pool) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
    var allErrors []error

    for range len(p.providers) {
        idx := int(p.current.Add(1)-1) % len(p.providers)
        provider := p.providers[idx]
        cb := p.breakers[idx]

        // Check circuit breaker
        if !cb.Allow() {
            slog.Debug("circuit breaker open, skipping provider",
                "chain", p.chain,
                "provider", provider.Name(),
                "state", cb.State(),
            )
            allErrors = append(allErrors, fmt.Errorf("%s: %w", provider.Name(), config.ErrCircuitOpen))
            continue
        }

        results, err := provider.FetchNativeBalances(ctx, addresses)
        if err == nil {
            cb.RecordSuccess()
            return results, nil
        }

        // Record failure in circuit breaker
        cb.RecordFailure()
        allErrors = append(allErrors, fmt.Errorf("%s: %w", provider.Name(), err))

        if errors.Is(err, config.ErrProviderRateLimit) || errors.Is(err, config.ErrProviderUnavailable) || config.IsTransient(err) {
            slog.Warn("provider failed, trying next",
                "chain", p.chain,
                "provider", provider.Name(),
                "circuitState", cb.State(),
                "error", err,
            )
            continue
        }

        // Non-retriable (context cancelled, etc.) — return immediately
        return results, err
    }

    return nil, fmt.Errorf("all %s providers failed: %w", p.chain, errors.Join(allErrors...))
}
```

**Key changes:**
- Circuit breaker checked before each provider call
- Success/failure recorded after each call
- `errors.Join()` returns all errors (B9 fix — was returning only last)
- TransientError is treated as retriable

Same changes for `FetchTokenBalances`.

**Verification:** Unit test: 2 providers, first has circuit open → skipped, second succeeds. Test: both fail → circuit breakers record failures. Test: provider fails 3 times → circuit opens → subsequent calls skip it.

---

### Task 2.7: Exponential Backoff in Pool (B11)

When all providers fail for a batch, the scanner should wait before retrying the next batch.

**Changes to `internal/scanner/pool.go`:**

Add backoff tracking method:
```go
// SuggestBackoff returns backoff duration based on consecutive all-provider failures.
// Called by scanner after pool returns an error. Reset by scanner on success.
func (p *Pool) SuggestBackoff(consecutiveFailures int) time.Duration {
    if consecutiveFailures <= 0 {
        return 0
    }
    delay := config.ExponentialBackoffBase * time.Duration(1<<uint(consecutiveFailures-1))
    if delay > config.ExponentialBackoffMax {
        delay = config.ExponentialBackoffMax
    }

    // If any provider has a RetryAfter hint, use the max of that and backoff
    // (The caller should pass the error and we check RetryAfter)
    return delay
}
```

**Changes to `internal/scanner/scanner.go` — `runScan()`:**

Add backoff loop around native balance fetch:
```go
var consecutivePoolFailures int

// ... in the batch loop:

results, err := pool.FetchNativeBalances(ctx, addresses)
if err != nil {
    if ctx.Err() != nil {
        // Cancelled — exit
        return
    }

    consecutivePoolFailures++
    backoff := pool.SuggestBackoff(consecutivePoolFailures)

    // Check if error contains RetryAfter hint
    if retryAfter := config.GetRetryAfter(err); retryAfter > backoff {
        backoff = retryAfter
    }

    slog.Warn("all providers failed for batch, backing off",
        "chain", chain,
        "batch", i,
        "consecutiveFailures", consecutivePoolFailures,
        "backoff", backoff,
        "error", err,
    )

    // If too many consecutive failures, stop the scan
    if consecutivePoolFailures >= config.ProviderMaxRetries {
        slog.Error("max consecutive pool failures reached, stopping scan",
            "chain", chain,
            "failures", consecutivePoolFailures,
        )
        s.finishScan(chain, maxID, scanned, db.ScanStatusFailed, startTime, err)
        return
    }

    // Wait with backoff
    select {
    case <-ctx.Done():
        return
    case <-time.After(backoff):
    }

    // Retry this batch (don't advance i)
    i -= batchSize
    continue
}

// Success — reset backoff
consecutivePoolFailures = 0
```

**Verification:** Unit test: Mock pool that fails first 2 calls, succeeds on 3rd. Verify backoff durations (1s, 2s). Verify reset after success.

---

### Task 2.8: Atomic Scan State (B4)

Wrap scan state + balance updates in a DB transaction so they either both succeed or both fail.

**New method in `internal/db/sqlite.go`:**
```go
// BeginTx starts a new database transaction.
func (d *DB) BeginTx() (*sql.Tx, error) {
    return d.conn.Begin()
}
```

**New method in `internal/db/balances.go`:**
```go
// UpsertBalanceBatchTx upserts balances within an existing transaction.
func (d *DB) UpsertBalanceBatchTx(tx *sql.Tx, balances []models.Balance) error {
    // Same logic as UpsertBalanceBatch but uses tx.Exec instead of d.conn.Exec
}
```

**New method in `internal/db/scans.go`:**
```go
// UpsertScanStateTx upserts scan state within an existing transaction.
func (d *DB) UpsertScanStateTx(tx *sql.Tx, state models.ScanState) error {
    // Same logic as UpsertScanState but uses tx.Exec instead of d.conn.Exec
}
```

**Changes to `internal/scanner/scanner.go` — `runScan()`:**

Replace separate `UpsertBalanceBatch` + `UpsertScanState` calls with atomic block:
```go
// Atomic: store balances + update scan state together
dbTx, err := s.db.BeginTx()
if err != nil {
    slog.Error("failed to begin DB transaction", "chain", chain, "error", err)
    s.finishScan(chain, maxID, scanned, db.ScanStatusFailed, startTime, err)
    return
}

if err := s.db.UpsertBalanceBatchTx(dbTx, allBalances); err != nil {
    dbTx.Rollback()
    slog.Error("failed to store balances", "chain", chain, "error", err)
    s.finishScan(chain, maxID, scanned, db.ScanStatusFailed, startTime, err)
    return
}

newScanned := end
if err := s.db.UpsertScanStateTx(dbTx, models.ScanState{
    Chain:            chain,
    LastScannedIndex: newScanned,
    MaxScanID:        maxID,
    Status:           db.ScanStatusScanning,
}); err != nil {
    dbTx.Rollback()
    slog.Error("failed to update scan state", "chain", chain, "error", err)
    s.finishScan(chain, maxID, scanned, db.ScanStatusFailed, startTime, err)
    return
}

if err := dbTx.Commit(); err != nil {
    slog.Error("failed to commit scan batch", "chain", chain, "error", err)
    s.finishScan(chain, maxID, scanned, db.ScanStatusFailed, startTime, err)
    return
}

scanned = newScanned
```

This ensures: if balance writes succeed but scan state update fails, we rollback the balances too → no orphaned balance data without corresponding state update.

**Files modified:**
- `internal/db/sqlite.go` — `BeginTx()`
- `internal/db/balances.go` — `UpsertBalanceBatchTx()`
- `internal/db/scans.go` — `UpsertScanStateTx()`
- `internal/scanner/scanner.go` — atomic batch writes

**Verification:** Unit test: Mock DB where scan state write fails → balances are rolled back. Integration test: Crash recovery resumes from correct index.

---

### Task 2.9: Decouple Native/Token Failures (B8) + Token Error Visibility (B7)

Currently in `scanner.go`, if `pool.FetchNativeBalances()` returns an error, the entire batch fails and token scans never run. Change to run native and token scans independently.

**Changes to `internal/scanner/scanner.go` — batch loop in `runScan()`:**

```go
// Fetch native balances (non-fatal — continue to tokens even on failure)
nativeResults, nativeErr := pool.FetchNativeBalances(ctx, addresses)
if nativeErr != nil {
    if ctx.Err() != nil {
        return // scan cancelled
    }
    slog.Warn("native balance fetch failed, continuing to tokens",
        "chain", chain,
        "batch", i,
        "error", nativeErr,
    )
    // Broadcast native error but DON'T stop scan
    s.hub.Broadcast(Event{
        Type: "scan_error",
        Data: ScanErrorData{
            Chain:   string(chain),
            Error:   config.ErrorScanFailed,
            Message: fmt.Sprintf("native balance fetch failed for batch %d-%d: %s", i, end, nativeErr),
        },
    })
}

// Process native results (even if partial/errored)
nativeBalances := make([]models.Balance, 0)
if nativeResults != nil {
    for _, r := range nativeResults {
        nativeBalances = append(nativeBalances, models.Balance{...})
        if r.Balance != "0" && r.Error == "" {
            found++
        }
    }
}

// Fetch token balances INDEPENDENTLY of native result
for _, tc := range tokenConfig[chain] {
    tokenResults, tokenErr := pool.FetchTokenBalances(ctx, addresses, tc.Token, tc.Contract)
    if tokenErr != nil {
        if ctx.Err() != nil {
            return
        }
        slog.Warn("token balance fetch failed",
            "chain", chain,
            "token", tc.Token,
            "batch", i,
            "error", tokenErr,
        )
        // B7: Broadcast token-specific error event
        s.hub.Broadcast(Event{
            Type: "scan_token_error",
            Data: ScanTokenErrorData{
                Chain:   string(chain),
                Token:   string(tc.Token),
                Error:   config.ErrorTokenScanFailed,
                Message: tokenErr.Error(),
            },
        })
        continue
    }
    // ... process token results as before
}

// Only fail the scan if BOTH native AND all tokens failed AND no results at all
if nativeErr != nil && len(nativeBalances) == 0 {
    // Apply backoff logic (Task 2.7)
    // ...
}
```

**New SSE event type:**
```go
// ScanTokenErrorData for scan_token_error events.
type ScanTokenErrorData struct {
    Chain   string `json:"chain"`
    Token   string `json:"token"`
    Error   string `json:"error"`
    Message string `json:"message"`
}
```

**Frontend changes (minimal — SSE handler):**
The frontend SSE handler in `web/src/lib/stores/scan.ts` needs to handle the new `scan_token_error` event type. Add event listener:
```typescript
eventSource.addEventListener('scan_token_error', (e) => {
    const data = JSON.parse(e.data);
    // Store token error in scan state for display
    update(s => ({
        ...s,
        tokenErrors: [...(s.tokenErrors || []), data]
    }));
});
```

**Files modified:**
- `internal/scanner/scanner.go` — decoupled native/token, token error broadcast
- `internal/scanner/sse.go` — `ScanTokenErrorData` struct
- `web/src/lib/stores/scan.ts` — handle `scan_token_error` event
- `web/src/lib/types.ts` — `ScanTokenError` interface

**Verification:** Unit test: Native fetch fails, token fetch succeeds → token balances stored. Test: Token fetch fails → `scan_token_error` event broadcast.

---

### Task 2.10: SSE Resync on Reconnect (B10)

When a client disconnects and reconnects, they miss events. Add a `scan_state` event that sends the full current scan status.

**Changes to `internal/scanner/sse.go`:**

Add event ID tracking and current state snapshot:
```go
type SSEHub struct {
    clients   map[chan Event]struct{}
    mu        sync.RWMutex
    broadcast chan Event
    done      chan struct{}
    eventID   atomic.Uint64  // monotonically increasing event ID
}

// NextEventID returns the next event ID for sequencing.
func (h *SSEHub) NextEventID() uint64 {
    return h.eventID.Add(1)
}
```

**Changes to `internal/api/handlers/scan.go` — SSE handler:**

When client connects, send a `scan_state` snapshot:
```go
func (h *ScanHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
    // ... existing setup ...

    // On connect, send current scan state for all chains
    states, err := h.db.GetAllScanStates()
    if err == nil {
        for _, state := range states {
            // Send scan_state event with full current state
            fmt.Fprintf(w, "event: scan_state\ndata: %s\n\n", marshalJSON(ScanStateData{
                Chain:            string(state.Chain),
                LastScannedIndex: state.LastScannedIndex,
                MaxScanID:        state.MaxScanID,
                Status:           state.Status,
            }))
        }
        flusher.Flush()
    }

    // ... existing event loop ...
}
```

**New SSE event type:**
```go
type ScanStateData struct {
    Chain            string `json:"chain"`
    LastScannedIndex int    `json:"lastScannedIndex"`
    MaxScanID        int    `json:"maxScanID"`
    Status           string `json:"status"`
}
```

**New DB method in `internal/db/scans.go`:**
```go
// GetAllScanStates returns current scan state for all chains.
func (d *DB) GetAllScanStates() ([]models.ScanState, error) {
    // SELECT * FROM scan_state
}
```

**Frontend changes:**
```typescript
eventSource.addEventListener('scan_state', (e) => {
    const data = JSON.parse(e.data);
    // Restore scan progress state from server
    update(s => ({
        ...s,
        [data.chain]: {
            scanned: data.lastScannedIndex,
            total: data.maxScanID,
            status: data.status,
        }
    }));
});
```

**Files modified:**
- `internal/scanner/sse.go` — event ID, ScanStateData type
- `internal/api/handlers/scan.go` — send state on SSE connect
- `internal/db/scans.go` — `GetAllScanStates()`
- `web/src/lib/stores/scan.ts` — handle `scan_state` event
- `web/src/lib/types.ts` — `ScanState` interface

**Verification:** Test: Connect SSE, verify `scan_state` events received for each chain with active scan. Test: Disconnect and reconnect, verify state is resynced.

---

### Task 2.11: Scanner Integration — Tie It All Together

Final integration pass to make sure all changes work together in `scanner.go`.

**Refactored `runScan()` batch loop summary:**
1. Load address batch from DB
2. Fetch native balances (error collection, circuit breaker, backoff) — non-fatal
3. Fetch token balances independently per token — non-fatal, broadcast errors
4. Collect all balances (native + token, filtering out errored results)
5. Atomic DB write: balances + scan state in one transaction
6. Broadcast progress with accurate `found` count (only count non-errored, non-zero results)
7. On all-providers-fail: exponential backoff, retry batch up to `ProviderMaxRetries`

**Fix the `Found: 0` TODO in `finishScan()`:**
Pass `found` count through to `finishScan()` and `scan_complete` event.

**Files modified:**
- `internal/scanner/scanner.go` — full integration of all above changes

**Verification:** Full integration test: Start scan with mock providers, verify:
- Error collection works (partial results stored)
- Circuit breaker trips after failures
- Backoff applied on all-provider failure
- Atomic state commits
- Token errors broadcast independently
- Progress events have accurate found count

---

## Tests (~35 total)

### New Test Files
| File | Tests | Description |
|------|-------|-------------|
| `internal/scanner/retry_after_test.go` | 6 | Retry-After header parsing |
| `internal/scanner/pool_test.go` | 8 | Circuit breaker wiring, error aggregation, backoff |
| `internal/scanner/btc_blockstream_test.go` | 4 | Error collection (partial batch, all fail, context cancel) |
| `internal/scanner/btc_mempool_test.go` | 4 | Error collection (same patterns) |
| `internal/scanner/bsc_bscscan_test.go` | 4 | Error collection + missing address detection |
| `internal/scanner/bsc_rpc_test.go` | 3 | Error collection + malformed response |
| `internal/scanner/sol_rpc_test.go` | 4 | Partial result validation + error annotation |
| `internal/db/scans_test.go` | 2 | GetAllScanStates, atomic Tx methods |

### Existing Tests Updated
- Ensure all existing scanner tests still pass with new error collection behavior

---

## Implementation Order

1. **Task 2.1** — Constants & errors (foundation for everything else)
2. **Task 2.5** — Retry-After parsing (standalone utility, needed by providers)
3. **Task 2.2** — BTC provider error collection
4. **Task 2.3** — BSC provider error collection
5. **Task 2.4** — SOL provider error collection + partial validation
6. **Task 2.6** — Circuit breaker wiring in pool
7. **Task 2.7** — Exponential backoff in pool
8. **Task 2.8** — Atomic scan state (DB layer)
9. **Task 2.9** — Decoupled native/token + token error SSE
10. **Task 2.10** — SSE resync on reconnect
11. **Task 2.11** — Final integration in scanner.go

---

<success_criteria>
- [ ] No provider early-returns on per-address errors — all addresses in batch are always attempted
- [ ] `BalanceResult.Error` populated for failed addresses (distinguishable from true zero)
- [ ] SOL `getMultipleAccounts` partial results detected and annotated
- [ ] Circuit breaker skips providers after consecutive failures, recovers via half-open
- [ ] `Retry-After` header parsed and respected on 429 responses
- [ ] Scan state + balances committed atomically in DB transactions
- [ ] Native balance failure does NOT prevent token scanning for same batch
- [ ] Token scan failures broadcast as `scan_token_error` SSE events
- [ ] SSE reconnect receives `scan_state` snapshot for all active scans
- [ ] Exponential backoff (1s→2s→4s...30s cap) applied when all providers fail
- [ ] `scan_complete` event includes accurate `found` count
- [ ] All existing tests continue to pass
- [ ] ~35 new tests pass
- [ ] `go test ./...` passes with zero failures
- [ ] `cd web && npm run build` succeeds
</success_criteria>
