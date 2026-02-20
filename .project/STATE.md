# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation -- all via a localhost Svelte dashboard.

## Current Position
- **Version:** V2 (build complete + post-build hardening complete)
- **Phase:** All 6 planned build phases + 5 post-build hardening sessions completed
- **Status:** Production-ready with comprehensive security hardening, async send flow, network coexistence, and end-to-end testing
- **Last session:** 2026-02-19 -- Network mainnet/testnet coexistence (Migration 007, env-only network, default testnet)

## Version History
| Version | Completed | Summary |
|---------|-----------|---------|
| V1 | 2026-02-18 | 11 phases, 22MB binary, full scan->send pipeline BTC/BSC/SOL, USDC/USDT, dashboard, history, settings |
| V2 | 2026-02-19 | 6 planned phases + 5 post-build sessions: robustness hardening, async sends, security audit (21 fixes), SOL fee payer, network coexistence, end-to-end testing |

## V2 Build Progress
| Phase | Name | Status |
|-------|------|--------|
| 1 | Foundation: Schema, Error Types & Circuit Breaker | **Completed** |
| 2 | Scanner Resilience | **Completed** |
| 3 | TX Safety -- Core | **Completed** |
| 4 | TX Safety -- Advanced | **Completed** |
| 5 | Provider Health & Broadcast Fallback | **Completed** |
| 6 | Security Tests & Infrastructure | **Completed** |

## Post-V2 Hardening (Sessions 19-23)
| Session | Focus | Summary |
|---------|-------|---------|
| 19 | Bug Fixes & E2E Testing | 16 bug fixes: balance display, SSE streaming, send flow, BscScan V2, godotenv, provider health checks |
| 20 | Comprehensive Audit | 14 fixes: CSRF bypass, BSC key zeroing, formatRawBalance precision, log rotation, config validation |
| 21 | Async Send & TX Status | Async 202 execute, SSE progress, polling fallback, TX reconciler, send stuck fixes for all 3 chains |
| 22 | SOL Fee Payer & Security | SOL fee payer mechanism, 21 security fixes (5 critical, 6 high, 5 medium) |
| 23 | Network Coexistence | Migration 007, network column on all tables, env-only setting, default testnet |

## Key Decisions

### V1 Architecture
- BTC uses BIP-84 (purpose=84) for Native SegWit bech32
- SOL uses manual SLIP-10 ed25519 (~120 lines, zero external deps)
- SOL TX serialization from scratch (no Solana SDK)
- BSC: EIP-155 signing, manual ABI encoding, 20% gas buffer
- SPA served via go:embed with immutable cache for _app/
- CoinGecko public API (no key), 5-min server-side cache

### V2 Robustness
- Circuit breaker: threshold=3, 30s cooldown, 1 half-open request per provider
- TransientError wrapper with optional RetryAfter for retry decisions
- TX state tracking: pending->broadcasting->confirming->confirmed|failed|uncertain
- Concurrent send: per-chain mutex with TryLock, HTTP 409 if busy
- SOL blockhash cache (10s TTL after security audit tightened from 20s)
- BTC confirmation: best-effort polling via Esplora, timeout -> uncertain
- UTXO re-validation: count >5% drop or value >3% drop rejects sweep
- BSC balance recheck: real-time re-fetch at execute, conservative value
- Gas pre-seed idempotency: checks tx_state before sending
- BSC gas price cap: rejects if >2x preview gas price
- Provider health DB: scanner pool records success/failure (non-blocking)
- BSC/SOL broadcast fallback: FallbackEthClient, doRPCAllURLs
- Stale-but-serve prices: 30-min tolerance
- DB connection pool: 25 open / 5 idle / 5 min lifetime

### Post-V2 Hardening
- Async send: 202 Accepted + SSE progress + polling fallback, context.Background() for sweeps
- TX reconciler: startup check of non-terminal tx_state rows, >1h -> uncertain
- SOL fee payer: native fee payer mechanism for token sweeps (no gas pre-seeding)
- Security: typed CONFIRM + 3s countdown modal, full destination address, network badge at send
- BTC fee safety margin: 2% vsize rounding buffer
- BSC per-TX gas check: skip gas-less addresses mid-sweep
- formatRawBalance: string-based decimal placement (no parseFloat precision loss)
- BscScan V2 API migration (V1 deprecated)
- BSC private key zeroing: ZeroECDSAKey() + defer at all signing callsites
- Config.Validate() at boot: fail fast on invalid network/port
- Network coexistence: migration 007, db.New(path, network), auto-filter queries
- Default testnet: env-only HDPAY_NETWORK, not editable from UI

## Tech Stack
No changes from V1, except:
- Added `godotenv` for .env file loading
- BscScan API migrated from V1 to V2

## Next Actions
- Run `/cf-new-version` to start planning V3
- Potential V3 items: monitoring dashboard, alerting, performance benchmarks, multi-mnemonic support, UI polish

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
| `.project/v2/04-build-plan/phases/phase-06/PLAN.md` | Phase 6 plan (completed) |

## Session History
| # | Date | Phase | Summary |
|---|------|-------|---------|
| 23 | 2026-02-19 | post-v2 | Network mainnet/testnet coexistence: Migration 007, env-only network, default testnet |
| 22 | 2026-02-19 | post-v2 | SOL fee payer mechanism + security audit (21 fixes: blockhash, CONFIRM modal, network badge, UTXO thresholds) |
| 21 | 2026-02-19 | post-v2 | Async send (202 + SSE), TX reconciler, send stuck fixes for all 3 chains, SOL USDC display fixes |
| 20 | 2026-02-19 | post-v2 | Post-V2 audit: CSRF bypass, BSC key zeroing, formatRawBalance precision, log rotation, config validation |
| 19 | 2026-02-19 | post-v2 | Post-V2 bug fixes: 16 bugs (balance display, SSE, BscScan V2, godotenv), provider health checks |
| 18 | 2026-02-19 | building | V2 Build Phase 6 (FINAL): Security Tests & Infrastructure -- 55 new tests, server hardening, DB pool, stale-but-serve prices, graceful shutdown. ALL V2 COMPLETE. |
| 17 | 2026-02-19 | building | V2 Build Phase 5: Provider Health & Broadcast Fallback -- DB health recording, health API, BSC FallbackEthClient, SOL doRPCAllURLs, frontend health indicators. 5 new tests. |
| 16 | 2026-02-18 | building | V2 Build Phase 4: TX Safety Advanced -- UTXO re-validation, BSC balance recheck, partial sweep resume, gas pre-seed idempotency, SOL ATA confirmation, BSC gas re-estimation, nonce gap handling. 14 new tests. |
| 15 | 2026-02-18 | building | V2 Build Phase 3: TX Safety Core -- concurrent send mutex, BTC confirmation polling, SOL confirmation uncertainty, in-flight TX persistence, SOL blockhash cache. |
| 14 | 2026-02-18 | building | V2 Build Phase 2: Scanner resilience -- error collection, partial result validation, atomic DB writes, circuit breaker wiring, backoff, decoupled native/token. |
| 13 | 2026-02-18 | building | V2 Build Phase 1: tx_state + provider_health DB, TransientError, circuit breaker, BalanceResult enhancement, sweep ID generator. 24 tests. |
| 12 | 2026-02-18 | planning | V2 planning: robustness audit (37 issues), V2 plan + 6-phase build plan |
| 11 | 2026-02-18 | building | Phase 11: History, Settings & Deployment -- V1 COMPLETE |
| 10 | 2026-02-18 | building | Phase 10: Send Interface |
| 9 | 2026-02-18 | building | Phase 9: SOL Transaction Engine |
