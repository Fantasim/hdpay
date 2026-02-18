# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Version:** V2
- **Phase:** Building (Phase 1 of 6 complete)
- **Status:** V2 Build Phase 1 complete — foundation infrastructure laid
- **Last session:** 2026-02-18 — Built V2 Phase 1: schema, error types, circuit breaker

## Version History
| Version | Completed | Summary |
|---------|-----------|---------|
| V1 | 2026-02-18 | 11 phases, 22MB binary, full scan→send pipeline BTC/BSC/SOL, USDC/USDT, dashboard, history, settings |

## V2 Build Progress
| Phase | Name | Status |
|-------|------|--------|
| 1 | Foundation: Schema, Error Types & Circuit Breaker | **Completed** |
| 2 | Scanner Resilience | Pending |
| 3 | TX Safety — Core | Pending |
| 4 | TX Safety — Advanced | Pending |
| 5 | Provider Health & Broadcast Fallback | Pending |
| 6 | Security Tests & Infrastructure | Pending |

## Key Decisions
- **V2 scope**: Robustness only — no new features, no tech stack changes
- **Circuit breaker**: threshold=3, 30s cooldown, 1 half-open test request per provider
- **TransientError**: Wrapper with optional RetryAfter for retry decisions
- **TX state tracking**: tx_state table for full TX lifecycle (pending→broadcasting→confirming→confirmed|failed|uncertain)
- All V1 decisions carry forward unchanged

## Tech Stack
No changes from V1.

## Next Actions
- Start V2 Build Phase 2: Scanner Resilience
- Key files to read: `.project/v2/04-build-plan/phases/phase-02/PLAN.md`

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/v2/00-robustness-audit.md` | Full audit findings (37 issues) |
| `.project/v2/02-plan.md` | V2 plan |
| `.project/v2/04-build-plan/` | V2 build plan (6 phases) |
| `.project/v2/04-build-plan/phases/phase-01/PLAN.md` | Phase 1 plan (completed) |

## Session History
| # | Date | Phase | Summary |
|---|------|-------|---------|
| 13 | 2026-02-18 | building | V2 Build Phase 1: tx_state + provider_health DB, TransientError, circuit breaker, BalanceResult enhancement, sweep ID generator. 24 tests. |
| 12 | 2026-02-18 | planning | V2 planning: robustness audit (37 issues), V2 plan + 6-phase build plan |
| 11 | 2026-02-18 | building | Phase 11: History, Settings & Deployment — V1 COMPLETE |
| 10 | 2026-02-18 | building | Phase 10: Send Interface |
| 9 | 2026-02-18 | building | Phase 9: SOL Transaction Engine |
