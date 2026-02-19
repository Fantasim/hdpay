# Phase 4 Summary: TX Safety — Advanced

## Completed: 2026-02-18

## What Was Built

7 robustness improvements covering audit IDs A6-A12, all focused on preventing data loss and race conditions during transaction execution.

### Task 1: BTC UTXO Re-Validation (A6)
- Added `ValidateUTXOsAgainstPreview()` function in `btc_tx.go`
- Compares actual UTXO set at execute time against preview expectations
- Rejects if UTXO count drops >20% or value drops >10%
- Preview response now includes `expectedInputCount` and `expectedTotalSats`
- 4 unit tests added

### Task 2: BSC Balance Recheck (A7)
- Added `BalanceOfBEP20()` for on-chain BEP-20 token balance verification
- Native sweep: fetches real-time balance, logs divergence from DB, checks minimum sweep threshold
- Token sweep: re-fetches on-chain token balance, uses conservative (lower) value
- Added `CallContract` to `EthClientWrapper` interface
- 5 unit tests added

### Task 3: Partial Sweep Resume (A8)
- Added `GET /api/send/resume/{sweepID}` — returns sweep summary with status counts
- Added `POST /api/send/resume` — retries failed/uncertain TXs from a previous sweep
- Added `GetRetryableTxStates()` and `GetSweepMeta()` DB methods
- Resume generates a new sweepID for the retry batch
- Filters funded addresses to only retry failed indices

### Task 4: Gas Pre-Seed Idempotency (A9)
- Gas pre-seed now tracks all sends via `tx_state` table
- Before sending, checks for already-confirmed sends to each target in the sweep
- Skips targets that already received gas — prevents double-sends on retry
- Added `HasConfirmedTxForAddress()` DB method
- `Execute()` now accepts optional `sweepID` parameter

### Task 5: SOL ATA Confirmation (A10)
- Added `waitForATAVisibility()` — polls `GetAccountInfo` after ATA creation
- Prevents race condition where subsequent token transfers fail because ATA isn't visible yet
- Uses `SOLATAConfirmationTimeout` (30s) and `SOLATAConfirmationPollInterval` (2s)
- Wired into `ExecuteTokenSweep` after first confirmed TX with ATA creation

### Task 6: BSC Gas Re-Estimation (A11)
- Added `ValidateGasPriceAgainstPreview()` function
- Rejects sweep if current gas price exceeds 2x the preview's gas price
- Preview response now includes `expectedGasPrice`
- Both `ExecuteNativeSweep` and `ExecuteTokenSweep` accept optional `expectedGasPrice`
- 4 unit tests added

### Task 7: Nonce Gap Handling (A12)
- Gas pre-seed detects "nonce too low" errors during broadcast
- On nonce conflict: re-fetches pending nonce from RPC and retries once
- Added `isNonceTooLowError()` helper covering common BSC error patterns
- 1 unit test (8 sub-cases) added

## Files Created/Modified

### Modified
- `internal/tx/btc_tx.go` — UTXO validation, Execute signature change
- `internal/tx/btc_tx_test.go` — 4 UTXO validation tests
- `internal/tx/bsc_tx.go` — BalanceOfBEP20, gas price validation, balance recheck, EthClientWrapper.CallContract
- `internal/tx/bsc_tx_test.go` — 9 new tests (balance recheck + gas price), mock update
- `internal/tx/sol_tx.go` — ATA visibility polling
- `internal/tx/gas.go` — Idempotency, nonce gap handling, tx_state tracking
- `internal/tx/gas_test.go` — Nonce-too-low detection test
- `internal/api/handlers/send.go` — Resume handlers, gas price passthrough
- `internal/api/router.go` — Resume routes
- `internal/db/tx_state.go` — GetRetryableTxStates, GetSweepMeta, HasConfirmedTxForAddress
- `internal/config/constants.go` — New constants for all tasks
- `internal/config/errors.go` — New sentinel errors and error codes
- `internal/models/types.go` — SendRequest fields, ResumeRequest, ResumeSummary

## Decisions Made

- **Variadic params for backward compatibility**: `ExecuteNativeSweep`, `ExecuteTokenSweep`, and gas pre-seed `Execute` use variadic params for new optional arguments to avoid breaking existing callers.
- **Conservative balance selection**: When DB and on-chain balances diverge, we use the lower value (BSC token sweep) to prevent overspending.
- **Non-blocking ATA check**: SOL ATA visibility failure logs an error but doesn't abort the sweep — subsequent transfers may still succeed.
- **Single retry for nonce conflicts**: Only one retry attempt on nonce-too-low to avoid infinite loops.

## Deviations from Plan
- None. All 7 tasks implemented as planned.

## Issues Encountered
- None — all tasks compiled and tested on first pass.

## Notes for Next Phase
- Phase 5 (Provider Health & Monitoring) is next
- All tx_state infrastructure is now robust enough for the health monitoring layer
- The resume endpoint provides the foundation for future automated retry policies
