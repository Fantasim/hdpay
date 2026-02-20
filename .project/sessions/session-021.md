# Session 021 -- 2026-02-19

## Version: V2 (post-build)
## Phase: Async Send Execution & TX Status Fixes
## Summary: Converted synchronous send execution to async (202 Accepted + SSE progress). Fixed TX status stuck on "pending" forever with startup reconciler. Fixed send stuck on "Executing..." for all 3 chains. Fixed SOL USDC balance display and token preview issues.

## What Was Done

### Async Send Execution
- `POST /api/send/execute` now returns 202 Accepted immediately with `sweepID`
- All 5 sweep paths (BTC, BSC native/token, SOL native/token) broadcast `tx_status` SSE events per TX
- Added `tx_complete` SSE event with full `TxResults` array
- Added `GET /api/send/sweep/{sweepID}` polling fallback for SSE disconnects
- Added `SweepStarted` response model
- Frontend polling fallback: polls every 3s when SSE drops
- Navigation guard: `beforeNavigate` + `beforeunload` warn user during active sweep
- Progress counter: "N of M transactions processed"
- Added `SEND_POLL_INTERVAL_MS` constant

### TX Status Stuck on "Pending"
- Added startup TX reconciler: checks all non-terminal `tx_state` rows against blockchain
- Re-launches polling goroutines for recent pending TXs
- Marks old ones (>1h) as "uncertain"
- Added `UpdateTransactionStatusByHash` DB method
- Added `ReconcileMaxAge` / `ReconcileCheckTimeout` constants
- Tests for reconciler (SOL confirmed, BSC confirmed, no-hash failed, empty)

### Send Flow Fixes
- Fixed BTC send stuck: moved confirmation polling to background goroutine
- Fixed SOL/BSC send stuck: moved confirmation polling to background goroutines for all sweep paths
- Fixed SOL USDC balance display: SQLite `CAST(SUM(...))` was appending ".0", switched to `printf('%.0f', ...)`
- Fixed SOL token preview undercounts: `buildSOLTokenPreview()` now calculates `HasGas` per-address
- Fixed SOL token execute crashes on gas-less addresses: checks native balance before attempting transfer
- Fixed "Last Scanned" showing stale timestamps: scans now always start from index 0
- Fixed `hydrateBalances` picking arbitrary timestamp: uses MAX across all tokens

## Decisions Made
- **Async 202 pattern**: Handler returns immediately, background goroutine owns the sweep lifecycle
- **`context.Background()` for sweeps**: HTTP request context is dead after 202 response
- **Per-chain mutex with `TryLock()`**: Handler checks, goroutine holds and defers unlock
- **Reconciler max age**: 1 hour — TXs pending longer than this are marked "uncertain" rather than re-polled
- **Always scan from 0**: Remove resume optimization that caused stale timestamps

## Issues / Blockers
- None

## Next Steps
- SOL token sweep needs fee payer mechanism
- Comprehensive security audit
