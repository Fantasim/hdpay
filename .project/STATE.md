# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Version:** V2
- **Phase:** Building (Phase 5 of 6 complete)
- **Status:** V2 Build Phase 5 complete — Provider health & broadcast fallback
- **Last session:** 2026-02-19 — Built V2 Phase 5: Provider Health & Broadcast Fallback

## Version History
| Version | Completed | Summary |
|---------|-----------|---------|
| V1 | 2026-02-18 | 11 phases, 22MB binary, full scan->send pipeline BTC/BSC/SOL, USDC/USDT, dashboard, history, settings |

## V2 Build Progress
| Phase | Name | Status |
|-------|------|--------|
| 1 | Foundation: Schema, Error Types & Circuit Breaker | **Completed** |
| 2 | Scanner Resilience | **Completed** |
| 3 | TX Safety -- Core | **Completed** |
| 4 | TX Safety -- Advanced | **Completed** |
| 5 | Provider Health & Broadcast Fallback | **Completed** |
| 6 | Security Tests & Infrastructure | Pending |

## Key Decisions
- **V2 scope**: Robustness only -- no new features, no tech stack changes
- **Circuit breaker**: threshold=3, 30s cooldown, 1 half-open test request per provider
- **TransientError**: Wrapper with optional RetryAfter for retry decisions
- **TX state tracking**: tx_state table for full TX lifecycle (pending->broadcasting->confirming->confirmed|failed|uncertain)
- **Concurrent send**: Per-chain mutex with TryLock, HTTP 409 Conflict if busy
- **Uncertain vs failed**: TX broadcast but unverifiable -> "uncertain"; TX definitively failed -> "failed"
- **Non-blocking tx_state**: DB writes for tx_state don't abort sweeps on failure
- **SOL blockhash cache**: 20s TTL, thread-safe, reduces RPC calls during multi-address sweeps
- **BTC confirmation**: Best-effort polling via Esplora, timeout -> uncertain (not failed)
- **UTXO re-validation**: Count >20% drop or value >10% drop rejects sweep (A6)
- **BSC balance recheck**: Real-time re-fetch at execute, uses conservative (lower) value (A7)
- **Partial sweep resume**: New sweepID per retry batch, filters to failed/uncertain indices (A8)
- **Gas pre-seed idempotency**: Checks tx_state before sending, skips confirmed targets (A9)
- **SOL ATA visibility**: 30s polling after creation, non-blocking on failure (A10)
- **BSC gas price cap**: Rejects if >2x preview gas price (A11)
- **Nonce gap handling**: Single retry with fresh nonce on nonce-too-low errors (A12)
- **Provider health DB**: Scanner pool records health success/failure to provider_health table (non-blocking)
- **BSC broadcast fallback**: FallbackEthClient wrapper tries primary then Ankr RPC
- **SOL broadcast fallback**: doRPCAllURLs tries all configured URLs before failing
- All V1 decisions carry forward unchanged

## Tech Stack
No changes from V1.

## Next Actions
- Start V2 Build Phase 6: Security Tests & Infrastructure
- Key files to read: `.project/v2/04-build-plan/phases/phase-06/PLAN.md`

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file -- resume context |
| `.project/v2/00-robustness-audit.md` | Full audit findings (37 issues) |
| `.project/v2/02-plan.md` | V2 plan |
| `.project/v2/04-build-plan/` | V2 build plan (6 phases) |
| `.project/v2/04-build-plan/phases/phase-01/PLAN.md` | Phase 1 plan (completed) |
| `.project/v2/04-build-plan/phases/phase-02/PLAN.md` | Phase 2 plan (completed) |
| `.project/v2/04-build-plan/phases/phase-03/PLAN.md` | Phase 3 plan (completed) |
| `.project/v2/04-build-plan/phases/phase-04/PLAN.md` | Phase 4 plan (completed) |
| `.project/v2/04-build-plan/phases/phase-05/PLAN.md` | Phase 5 plan (completed) |

## Session History
| # | Date | Phase | Summary |
|---|------|-------|---------|
| 17 | 2026-02-19 | building | V2 Build Phase 5: Provider Health & Broadcast Fallback -- DB health recording, health API, BSC FallbackEthClient, SOL doRPCAllURLs, frontend health indicators. 5 new tests. |
| 16 | 2026-02-18 | building | V2 Build Phase 4: TX Safety Advanced -- UTXO re-validation (A6), BSC balance recheck (A7), partial sweep resume (A8), gas pre-seed idempotency (A9), SOL ATA confirmation (A10), BSC gas re-estimation (A11), nonce gap handling (A12). 14 new tests. |
| 15 | 2026-02-18 | building | V2 Build Phase 3: TX Safety Core -- concurrent send mutex (A1), BTC confirmation polling (A2), SOL confirmation uncertainty (A3), in-flight TX persistence all chains (A4), SOL blockhash cache (A5). |
| 14 | 2026-02-18 | building | V2 Build Phase 2: Scanner resilience -- error collection, partial result validation, atomic DB writes, circuit breaker wiring, backoff, decoupled native/token, token error SSE, SSE resync. |
| 13 | 2026-02-18 | building | V2 Build Phase 1: tx_state + provider_health DB, TransientError, circuit breaker, BalanceResult enhancement, sweep ID generator. 24 tests. |
| 12 | 2026-02-18 | planning | V2 planning: robustness audit (37 issues), V2 plan + 6-phase build plan |
| 11 | 2026-02-18 | building | Phase 11: History, Settings & Deployment -- V1 COMPLETE |
| 10 | 2026-02-18 | building | Phase 10: Send Interface |
| 9 | 2026-02-18 | building | Phase 9: SOL Transaction Engine |
