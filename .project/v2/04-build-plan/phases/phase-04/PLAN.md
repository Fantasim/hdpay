# Phase 4: TX Safety — Advanced (Outline)

> Expanded to detailed plan when Phase 3 is complete.

## Goal
Fix remaining TX safety issues: UTXO re-validation, balance rechecks, partial sweep resume, gas pre-seed idempotency, ATA confirmation, gas re-estimation.

## Audit IDs Covered
A6, A7, A8, A9, A10, A11, A12

## Key Changes

### BTC UTXO Re-Validation (A6)
At execute time, re-fetch UTXOs and compare with preview:
- If UTXO count changed by more than 10% → return error with diff
- If total value decreased → return error with old vs new amounts
- User must re-preview before executing

### BSC Balance Recheck (A7)
Before each BSC address sweep:
- Call `BalanceAt()` for real-time balance
- If balance < minimum sweep amount → skip with warning
- Log the old (DB) vs new (real-time) balance

### Partial Sweep Resume (A8)
Use `tx_state` table for resume capability:
- On execute, check for existing sweep_id with pending/failed TXs
- Skip addresses with `confirmed` status
- Retry `failed` addresses
- Surface `uncertain` addresses for user decision
- New endpoint: `POST /api/send/resume/{sweepID}`

### Gas Pre-Seed Idempotency (A9)
Before sending gas to an address:
- Check `tx_state` for existing TX at same nonce from same source
- If found with status `confirming`/`uncertain` → wait for it instead of re-sending
- If found with status `confirmed` → skip (already funded)
- If found with status `failed` → retry with fresh nonce

### SOL ATA Confirmation (A10)
After TX that creates ATA:
- Poll `GetAccountInfo(destATA)` until ATA exists
- Timeout: 30s
- Only then build next TX

### BSC Gas Re-Estimation (A11)
At execute time:
- Re-fetch gas price via `SuggestGasPrice()`
- Compare with preview gas price
- If increase > 2x → return error, require re-preview
- If increase 1-2x → proceed with new price + 20% buffer
- Log the change

## Files Modified
- `internal/tx/btc_tx.go` — UTXO comparison
- `internal/tx/btc_utxo.go` — UTXO diff helper
- `internal/tx/bsc_tx.go` — Balance recheck, gas re-estimation
- `internal/tx/sol_tx.go` — ATA confirmation
- `internal/tx/gas.go` — Nonce idempotency
- `internal/api/handlers/send.go` — Resume endpoint

## Estimated Tests: ~20
