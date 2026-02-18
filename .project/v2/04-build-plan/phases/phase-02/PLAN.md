# Phase 2: Scanner Resilience (Outline)

> Expanded to detailed plan when Phase 1 is complete.

## Goal
Fix all scanner/provider reliability issues: error collection instead of early-return, partial result validation, atomic scan state, circuit breaker wiring, Retry-After parsing, token error visibility.

## Audit IDs Covered
B1, B3, B4, B6, B7, B8, B10, B11

## Key Changes

### Provider Error Collection (B1)
All 6 providers (btc_blockstream, btc_mempool, bsc_bscscan, bsc_rpc, sol_rpc x2) currently return on first error. Change to:
- Collect errors per-address
- Continue processing remaining addresses
- Return partial results with error annotations in `BalanceResult.Error`
- Only return hard error if 100% of addresses fail

### Partial Result Validation (B3)
SOL `getMultipleAccounts` can return fewer results than requested:
- Compare `len(results)` vs `len(requested)`
- If fewer, identify missing indices and re-fetch in second pass
- Log warning with counts

### Atomic Scan State (B4)
Wrap scan state + balance updates in DB transaction:
- `BeginTx()` → `UpsertScanState()` + `UpsertBalances()` → `Commit()`
- On failure, rollback both → no partial state

### Circuit Breaker Wiring (B5)
Each provider in the pool gets a CircuitBreaker instance:
- Pool checks `cb.Allow()` before calling provider
- On success: `cb.RecordSuccess()`
- On failure: `cb.RecordFailure()`
- If circuit open: skip provider, try next

### Retry-After Parsing (B6)
On 429 responses:
- Extract `Retry-After` header (seconds or HTTP-date)
- Pass as `TransientError.RetryAfter`
- Rate limiter respects this delay

### Token Error Visibility (B7)
Token scan failures → new SSE event type:
```
event: scan_token_error
data: {"chain":"BSC","token":"USDC","error":"all providers failed"}
```

### Decouple Native/Token Failures (B8)
Native balance failure should not abort token scans. Run independently:
```go
nativeResults, nativeErr := pool.FetchNativeBalances(ctx, addresses)
// Process native results (even on error)

for _, tc := range tokenConfig[chain] {
    tokenResults, tokenErr := pool.FetchTokenBalances(ctx, addresses, tc.Token, tc.Contract)
    // Process token results independently
}
```

### SSE Resync (B10)
New event type `scan_state` that sends full current state:
- Client sends reconnect → server detects via `Last-Event-ID` header
- Server sends `scan_state` event with current totals

### Exponential Backoff (B11)
When all providers fail for a batch:
- Wait `baseDelay * 2^attempt` before next batch (capped at 30s)
- Reset on any success

## Files Modified
- `internal/scanner/btc_blockstream.go` — Error collection
- `internal/scanner/btc_mempool.go` — Error collection
- `internal/scanner/bsc_bscscan.go` — Error collection
- `internal/scanner/bsc_rpc.go` — Error collection
- `internal/scanner/sol_rpc.go` — Error collection + partial result validation
- `internal/scanner/pool.go` — Circuit breaker, error aggregation, backoff
- `internal/scanner/scanner.go` — Atomic state, decoupled native/token, token error events
- `internal/scanner/ratelimiter.go` — Retry-After support
- `internal/scanner/sse.go` — Resync event

## Estimated Tests: ~30
