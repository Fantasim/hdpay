# Phase 3 Summary: TX Safety — Core

## Completed: 2026-02-18

## What Was Built

### A1: Concurrent Send Protection
- Per-chain `sync.Mutex` with `TryLock()` in `ExecuteSend` handler — returns HTTP 409 Conflict if a sweep is already in progress for the same chain
- `ChainLocks` map initialized in `cmd/server/main.go` and threaded through `SendDeps`
- SweepID generation moved to handler level (before calling executeSweep)

### A2: BTC Confirmation Polling
- `WaitForBTCConfirmation()` function polls Esplora `/tx/{txid}/status` endpoints
- Round-robin across provider URLs (same Blockstream/Mempool used for UTXO/broadcast)
- Polls every 15s for up to 10 minutes
- Returns `ErrBTCConfirmationTimeout` on timeout (TX recorded as "uncertain", not "failed")
- `BTCConsolidationService` updated with `httpClient` and `confirmationURLs` fields

### A3: SOL Confirmation Error Handling
- `WaitForSOLConfirmation` now tracks consecutive RPC errors
- After 3 consecutive RPC failures → returns `ErrSOLConfirmationUncertain` instead of retrying forever
- Callers distinguish uncertain from failed: `errors.Is(err, config.ErrSOLConfirmationUncertain)` → status `uncertain`
- On-chain errors (tx reverted) still return `ErrSOLTxFailed` → status `failed`

### A4: In-Flight TX Persistence
- Full tx_state lifecycle wired into all 3 chains:
  - **BTC**: Single consolidated TX tracked through pending→broadcasting→confirming→confirmed/uncertain
  - **BSC**: Per-address TX tracked (native + token sweeps), same lifecycle
  - **SOL**: Per-address TX tracked (native + token sweeps), same lifecycle with uncertain support
- All tx_state writes are non-blocking: `createTxState()` and `updateTxState()` helpers log errors but don't abort sweeps
- Nil-safe: works correctly when database is nil (test environments)
- New API endpoints: `GET /api/send/pending` (list in-flight TXs) and `POST /api/send/dismiss/{id}` (dismiss uncertain/failed TXs)

### A5: SOL Blockhash Cache
- `getOrRefreshBlockhash()` method on `SOLConsolidationService`
- Cache with 20s TTL — avoids redundant RPC calls during multi-address sweeps
- Thread-safe with `sync.Mutex`
- Both native and token sweep methods use the cache

## Files Created/Modified

### Modified
- `internal/config/constants.go` — BTCConfirmationTimeout, BTCConfirmationPollInterval, BTCTxStatusPath, SOLBlockhashCacheTTL, SOLMaxConfirmationRPCErrors, TxStateDismissed
- `internal/config/errors.go` — ErrBTCConfirmationTimeout, ErrSOLConfirmationUncertain, ErrorBTCConfirmationTimeout, ErrorSOLConfirmationUncertain, ErrorSendBusy
- `internal/tx/sweep.go` — GenerateTxStateID()
- `internal/tx/btc_tx.go` — BTCConsolidationService (httpClient, confirmationURLs), Execute (sweepID, tx_state), WaitForBTCConfirmation, btcTxStatus, updateTxState
- `internal/tx/bsc_tx.go` — ExecuteNativeSweep/ExecuteTokenSweep (sweepID), sweepNativeAddress/sweepTokenAddress (sweepID, tx_state), updateTxState, createTxState
- `internal/tx/sol_tx.go` — SOLConsolidationService (blockhash cache), ExecuteNativeSweep/ExecuteTokenSweep (sweepID), sweepNativeAddress/sweepTokenAddress (sweepID, tx_state, blockhash cache, uncertain), WaitForSOLConfirmation (consecutive error tracking), updateTxState, createTxState, getOrRefreshBlockhash
- `internal/api/handlers/send.go` — ChainLocks, TryLock, sweepID generation, GetPendingTxStates, DismissTxState
- `internal/api/router.go` — /pending and /dismiss/{id} routes
- `internal/db/tx_state.go` — GetAllPendingTxStates()
- `cmd/server/main.go` — ChainLocks, updated BTC service constructor
- `internal/tx/sol_tx_test.go` — Updated call signatures for sweepID parameter

## Decisions Made
- **Non-blocking tx_state writes**: DB failures in tx_state don't abort sweeps — the sweep result matters more than audit trail
- **Uncertain vs failed**: Clear distinction — `uncertain` means "TX was broadcast but we couldn't verify", `failed` means "TX definitely didn't work"
- **BTC confirmation is best-effort**: Timeout doesn't fail the sweep, just marks TX as uncertain
- **SOL blockhash cache 20s TTL**: Blockhashes valid for ~60s on Solana, 20s cache is conservative and safe
- **409 Conflict for concurrent sends**: User-friendly error, not 500 — UI can handle it gracefully

## Deviations from Plan
- None — all 5 audit items (A1-A5) implemented as planned

## Issues Encountered
- SOL/BSC test files pass `nil` for database — required nil-safe `createTxState()` and `updateTxState()` helpers instead of direct database calls

## Notes for Next Phase
- Phase 4 (TX Safety — Advanced) will add: BSC nonce tracking (A6), double-spend detection (A7), sweep timeout guard (A8), BTC change output (A9)
- The tx_state table is now being populated — Phase 4 should query it for nonce deduplication (A7)
