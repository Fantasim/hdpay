# Session 016 — 2026-02-18

## Version: V2
## Phase: building
## Summary: V2 Build Phase 4 complete: TX Safety Advanced — UTXO re-validation (A6), BSC balance recheck (A7), partial sweep resume (A8), gas pre-seed idempotency (A9), SOL ATA confirmation (A10), BSC gas re-estimation (A11), nonce gap handling (A12). 14 new tests. All tests passing.

## What Was Done
- Implemented BTC UTXO re-validation at execute time (A6): `ValidateUTXOsAgainstPreview()` rejects if UTXO count drops >20% or value drops >10%
- Added BSC on-chain balance recheck (A7): `BalanceOfBEP20()` for token balance verification, native sweep uses DB-vs-realtime divergence logging with conservative value selection
- Built partial sweep resume (A8): `GET /api/send/resume/{sweepID}` and `POST /api/send/resume` endpoints, `GetRetryableTxStates()` and `GetSweepMeta()` DB methods
- Added gas pre-seed idempotency (A9): tx_state tracking per gas pre-seed TX, `HasConfirmedTxForAddress()` deduplication, skip already-confirmed targets on retry
- Implemented SOL ATA visibility polling (A10): `waitForATAVisibility()` polls GetAccountInfo after ATA creation with 30s timeout
- Added BSC gas price spike detection (A11): `ValidateGasPriceAgainstPreview()` rejects if current gas >2x preview price
- Implemented nonce gap handling (A12): `isNonceTooLowError()` detection with single retry and fresh nonce re-fetch
- Added 14 new tests across bsc_tx_test.go and gas_test.go
- Wrote Phase 4 SUMMARY.md
- Updated state.json and STATE.md

## Decisions Made
- Variadic params for backward compatibility: `ExecuteNativeSweep`, `ExecuteTokenSweep`, and gas `Execute` use variadic params for new optional arguments
- Conservative balance selection: DB-vs-chain divergence uses the lower value for BSC token sweeps
- Non-blocking ATA check: SOL ATA visibility failure logs error but doesn't abort sweep
- Single retry for nonce conflicts: Only one retry attempt on nonce-too-low to avoid infinite loops

## Issues / Blockers
- None — all 7 tasks compiled and tested on first pass

## Next Steps
- Start V2 Build Phase 5: Provider Health & Broadcast Fallback
- Read `.project/v2/04-build-plan/phases/phase-05/PLAN.md`
