# Session 015 — 2026-02-18

## Version: V2
## Phase: building
## Summary: V2 Build Phase 3: TX Safety Core — concurrent send mutex (A1), BTC confirmation polling (A2), SOL confirmation uncertainty (A3), in-flight TX persistence for all 3 chains (A4), SOL blockhash cache (A5).

## What Was Done
- Added per-chain `sync.Mutex` with `TryLock()` for concurrent send protection — HTTP 409 Conflict if busy (A1)
- Implemented BTC confirmation polling via Esplora `/tx/{txid}/status` — 15s intervals, 10min timeout, round-robin providers (A2)
- Added SOL confirmation uncertainty tracking — 3 consecutive RPC errors returns `ErrSOLConfirmationUncertain` (A3)
- Wired in-flight TX persistence (tx_state lifecycle) into all 3 chains: BTC single TX, BSC per-address, SOL per-address (A4)
- Added SOL blockhash cache with 20s TTL to reduce redundant RPC calls during multi-address sweeps (A5)
- Added `GET /api/send/pending` and `POST /api/send/dismiss/{id}` endpoints
- Added nil-safe `createTxState()` and `updateTxState()` helpers for SOL and BSC services
- Updated all Execute method signatures to accept `sweepID string` parameter
- Fixed test compilation: added `sweepID` parameter to all test call sites
- Fixed nil pointer dereference: nil-safe DB wrappers for test environments with nil database
- New constants: BTCConfirmationTimeout, BTCConfirmationPollInterval, SOLBlockhashCacheTTL, SOLMaxConfirmationRPCErrors, TxStateDismissed
- New errors: ErrBTCConfirmationTimeout, ErrSOLConfirmationUncertain, ErrorSendBusy

## Decisions Made
- **Non-blocking tx_state writes**: DB failures in tx_state don't abort sweeps — the sweep result matters more than audit trail
- **Uncertain vs failed**: Clear distinction — "uncertain" means TX was broadcast but unverifiable, "failed" means definitively didn't work
- **BTC confirmation best-effort**: Timeout doesn't fail the sweep, just marks TX as uncertain
- **SOL blockhash cache 20s TTL**: Blockhashes valid for ~60s, 20s is conservative and safe
- **409 Conflict for concurrent sends**: User-friendly error, UI can handle gracefully

## Issues / Blockers
- SOL/BSC test files pass `nil` for database — required nil-safe wrappers instead of direct DB calls
- Context window ran out mid-session — continued in new context with full summary

## Next Steps
- Run `/cf-next` to start Phase 4: TX Safety — Advanced
- Phase 4 covers: BSC nonce tracking (A6), double-spend detection (A7), sweep timeout guard (A8), BTC change output (A9)
