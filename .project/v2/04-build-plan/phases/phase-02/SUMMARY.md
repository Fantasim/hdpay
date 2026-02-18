# Phase 2 Summary: Scanner Resilience

## Completed: 2026-02-18

## What Was Built

- **Error collection pattern (B1)**: All 6 providers (Blockstream, Mempool, BscScan, BSC RPC, Solana RPC x2) now continue scanning remaining addresses when individual lookups fail. Failed addresses get `BalanceResult.Error` annotations instead of aborting the batch.
- **Partial result validation (B3)**: Solana `getMultipleAccounts` validates response length vs request length. Missing results are annotated rather than silently lost.
- **Atomic scan state (B4)**: New `BeginTx()`, `UpsertBalanceBatchTx()`, `UpsertScanStateTx()` methods. Scanner writes balances + scan state in a single DB transaction per batch.
- **Circuit breaker wiring (B5)**: Pool creates per-provider circuit breakers. `Allow()` checked before calls, `RecordSuccess/Failure` after. Open breakers are skipped with failover to next provider.
- **Retry-After parsing (B6)**: All HTTP-based providers parse `Retry-After` header on 429 responses. Duration is wrapped in `TransientErrorWithRetry` for backoff decisions.
- **Token error SSE events (B7)**: New `scan_token_error` SSE event broadcasts token scan failures with chain, token, error code, and message.
- **Decoupled native/token (B8)**: Native balance failure no longer aborts token scans. Each runs independently per batch.
- **Error aggregation (B9)**: Pool uses `errors.Join()` to return all provider errors, not just the last one.
- **SSE resync on reconnect (B10)**: New `scan_state` SSE event sent on client connect. Contains current status and running state for all chains.
- **Exponential backoff (B11)**: Scanner backs off 1s→2s→4s...30s on consecutive all-provider failures. Aborts after 5 consecutive failures.

## Files Created/Modified

### New Files
- `internal/scanner/retry_after.go` — Retry-After header parser (seconds + HTTP-date formats)
- `internal/scanner/retry_after_test.go` — 9 tests for Retry-After parsing

### Modified Files
- `internal/config/constants.go` — Added `ExponentialBackoffBase`, `ExponentialBackoffMax`, `MaxConsecutivePoolFails`
- `internal/config/errors.go` — Added `ErrPartialResults`, `ErrAllProvidersFailed`, error codes `ErrorPartialResults`, `ErrorAllProvidersFailed`, `ErrorTokenScanFailed`
- `internal/scanner/btc_blockstream.go` — Error collection, Retry-After wrapping, TransientError on 429/non-200
- `internal/scanner/btc_blockstream_test.go` — Rewritten with 8 tests including partial failure
- `internal/scanner/btc_mempool.go` — Same error collection and Retry-After changes
- `internal/scanner/bsc_bscscan.go` — Missing address detection, error collection, Retry-After wrapping
- `internal/scanner/bsc_bscscan_test.go` — Rewritten with 7 tests including missing address detection
- `internal/scanner/bsc_rpc.go` — Error collection, malformed contract response error
- `internal/scanner/sol_rpc.go` — Partial result validation, ATA failure annotation, Retry-After wrapping
- `internal/scanner/pool.go` — Per-provider circuit breakers, `SuggestBackoff()`, `errors.Join()` aggregation
- `internal/scanner/pool_test.go` — Rewritten with 11 tests including circuit breaker integration
- `internal/scanner/sse.go` — Added `ScanTokenErrorData`, `ScanStateSnapshotData` structs
- `internal/scanner/scanner.go` — Full V2 rewrite: atomic DB writes, decoupled native/token, backoff, token error SSE, fixed `Found: 0` in scan_complete
- `internal/db/sqlite.go` — Added `BeginTx()` method
- `internal/db/balances.go` — Added `UpsertBalanceBatchTx()` (transaction-aware variant)
- `internal/db/scans.go` — Added `UpsertScanStateTx()`, `GetAllScanStates()`
- `internal/api/handlers/scan.go` — `ScanSSE` now takes scanner+db args, sends `scan_state` snapshots on connect
- `internal/api/handlers/scan_test.go` — Updated for new `ScanSSE` signature
- `internal/api/router.go` — Updated `ScanSSE` call with new args
- `web/src/lib/types.ts` — Added `ScanTokenErrorEvent`, `ScanStateSnapshot` interfaces
- `web/src/lib/stores/scan.svelte.ts` — Added `lastTokenError` state, `scan_token_error` + `scan_state` SSE listeners

## Decisions Made
- **Native failure is non-blocking**: If native balance fetch fails for a batch, token scans still proceed. This maximizes data collection per batch.
- **Consecutive failure abort**: After 5 consecutive all-provider failures, scan aborts rather than spinning indefinitely. Configurable via `MaxConsecutivePoolFails`.
- **Atomic writes at batch level**: Each batch writes all balances (native + all tokens) and scan state in a single transaction, preventing inconsistent states on crash.

## Deviations from Plan
- Tasks 2.9, 2.10, and 2.11 were implemented together since they all affect scanner.go — more efficient than three separate rewrites.

## Notes for Next Phase
- Circuit breaker state is not persisted across restarts (in-memory only). Acceptable for V2 scope.
- Pre-existing frontend type errors in `BalanceBreakdown.svelte` and `PortfolioCharts.svelte` are unrelated to this phase.
