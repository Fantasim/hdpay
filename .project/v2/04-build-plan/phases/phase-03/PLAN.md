# Phase 3: TX Safety — Core (Outline)

> Expanded to detailed plan when Phase 1 is complete.

## Goal
Fix the 5 most critical TX safety issues: concurrent send protection, BTC confirmation, SOL confirmation error handling, in-flight TX persistence, SOL blockhash refresh.

## Audit IDs Covered
A1, A2, A3, A4, A5

## Key Changes

### Concurrent Send Mutex (A1)
Per-chain `sync.Mutex` in the send handler layer:
- Map of `chain → *sync.Mutex`
- `ExecuteSend` acquires lock before proceeding
- Returns HTTP 409 Conflict if chain is already sending
- Lock released when sweep completes (via defer)

### BTC Confirmation Polling (A2)
Add `WaitForBTCConfirmation(ctx, txHash)` function:
- Poll Blockstream/Mempool API for TX existence in mempool
- Then poll for 1+ confirmation
- Timeout: `BTCConfirmationTimeout` (configurable, default 10 minutes)
- Poll interval: `BTCConfirmationPollInterval` (default 15 seconds)
- Wire into BTC broadcaster after successful broadcast

### SOL Confirmation Error Handling (A3)
Fix `WaitForSOLConfirmation`:
- If RPC returns error during polling → mark TX as `uncertain` (not `failed`)
- Return `TxStateUncertain` status with error description
- Frontend shows "Transaction may have succeeded — check explorer" instead of "failed"
- Add specific error classification: `ErrSOLConfirmationUncertain`

### In-Flight TX Persistence (A4)
Before each address sweep in BSC/SOL/BTC engines:
1. Write `tx_state` row with status `pending`
2. Update to `broadcasting` before broadcast call
3. Update to `confirming` after broadcast succeeds (with txHash)
4. Update to `confirmed`/`failed`/`uncertain` after confirmation

New endpoint: `GET /api/send/pending` — returns all pending/uncertain TXs
New endpoint: `POST /api/send/dismiss/{id}` — mark uncertain TX as resolved

### SOL Blockhash Refresh (A5)
Replace single-blockhash-per-sweep with per-TX freshness:
- Cache blockhash with timestamp
- Before each TX: if cache age > 20s, fetch fresh blockhash
- Log blockhash refresh events

## Files Modified
- `internal/api/handlers/send.go` — Per-chain mutex, pending/dismiss endpoints
- `internal/tx/btc_tx.go` — Confirmation polling, tx_state persistence
- `internal/tx/btc_utxo.go` — (minor) confirmation helper
- `internal/tx/bsc_tx.go` — tx_state persistence
- `internal/tx/sol_tx.go` — Confirmation fix, blockhash refresh, tx_state persistence
- `internal/tx/broadcaster.go` — Confirmation polling integration
- `internal/api/router.go` — New routes
- `internal/config/constants.go` — BTC confirmation constants
- `internal/config/errors.go` — Uncertain error types

## Estimated Tests: ~25
