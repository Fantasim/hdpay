# Project State: Poller

> A standalone microservice that monitors cryptocurrency addresses for incoming payments and converts them into points for a video game economy. Zero runtime dependency on HDPay (at runtime). Imports HDPay's Go packages at compile time.

## Current Position
- **Version:** V1
- **Phase:** Building (Phase 5 of 8 next)
- **Status:** Phase 4 (Watch Engine) complete. Watcher orchestrator, poll loop, tx detection, confirmation tracking, points calculation, startup recovery, main.go integration.
- **Last session:** 2026-02-20 — Phase 4 Watch Engine built and tested

## Build Progress

| Phase | Name | Status | Sessions |
|-------|------|--------|----------|
| 1 | Foundation | **DONE** | 1 |
| 2 | Core Services | **DONE** | 1 |
| 3 | Blockchain Providers | **DONE** | 1 |
| 4 | Watch Engine | **DONE** | 1 |
| 5 | API Layer | **NEXT** | ~1-2 |
| 6 | Frontend Setup & Auth | Pending | ~1 |
| 7 | Dashboard Pages | Pending | ~2-3 |
| 8 | Embedding & Polish | Pending | ~1 |

**Total estimate: 10-13 sessions (4 done)**

## Phase 4 Deliverables (Complete)
- `internal/poller/watcher/watcher.go` — Watcher orchestrator: CreateWatch, CancelWatch, Stop, goroutine-per-watch, context timeouts, WaitGroup shutdown, runtime settings (MaxActiveWatches, DefaultWatchTimeout)
- `internal/poller/watcher/poll.go` — Poll loop: resolveCutoff, processTransaction (dedup, price, points), recheckPending (confirmation tracking), checkStopConditions (EXPIRED/COMPLETED/CANCELLED), SOL composite tx_hash handling
- `internal/poller/watcher/recovery.go` — Startup recovery: expire stale ACTIVE watches, retry orphaned PENDING txs (3 retries, 30s intervals), log unresolvable to system_errors
- `cmd/poller/main.go` — Full integration: config→logging→DB→tiers→PriceService→Pricer→Calculator→ProviderSets→Watcher→Recovery→router→server→shutdown
- `internal/poller/pollerdb/transactions.go` — Added ListPendingByWatchID, CountByWatchID
- `internal/poller/config/constants.go` — Added WatchContextGracePeriod (5s), RecoveryTimeout (5m)
- Tests: 31 tests, 71% coverage, 0 race conditions

## Phase 5: API Layer (Next)
- Chi router with middleware (auth, IP allowlist, logging, CORS)
- Session auth (bcrypt + cookie sessions)
- Watch CRUD endpoints (create, cancel, list, get)
- Points/transaction query endpoints
- Admin endpoints (settings, system errors)
- Health check with component status

## Key Decisions
- **Module structure**: Poller lives inside HDPay's Go module (`cmd/poller/` + `internal/poller/`). Full access to HDPay's `internal/` packages. Two binaries from one module.
- **HDPay reuse**: Import logging.SetupWithPrefix(), config constants/errors, models, PriceService, RateLimiter, CircuitBreaker, request logging middleware. Write new: watcher, providers (tx detection), points calculator, session auth, IP allowlist, handlers, DB CRUD, frontend.
- **MempoolProvider embeds BlockstreamProvider**: Same Esplora API, only Name() and baseURL differ
- **Composite tx_hash for SOL**: `"signature:SOL"`, `"signature:USDC"` — one Solana tx can produce multiple RawTransactions
- **BTC confirmation**: 1-conf from Esplora `confirmed=true` (no block counting)
- **BSC confirmation**: `currentBlock - txBlock >= 12` via BscScan proxy
- **SOL confirmation**: `confirmationStatus == "finalized"` (no block counting)
- **Helius**: Falls back to Solana devnet in testnet mode
- **Goroutine-per-watch**: context.Background() with timeout (not HTTP request context), WaitGroup for graceful shutdown
- **Smart cutoff**: MAX(last recorded tx detected_at, POLLER_START_DATE) — prevents re-processing old txs
- **SOL composite dedup**: extractBaseSignature strips ":TOKEN" suffix before confirmation checks
- **Watch stop**: EXPIRED (context deadline), COMPLETED (all confirmed + ≥1 tx), CANCELLED (manual cancel)
- Build output: `bin/poller` via Makefile (avoids conflict with `poller/` directory)
- Backend-first build order (all Go before any frontend)
- Desktop-only (no responsive), manual refresh (no SSE)

## Tech Stack
| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+ (shared module with HDPay) |
| Router | Chi v5 (shared dependency) |
| Database | SQLite (modernc.org/sqlite, own DB file) |
| Logging | HDPay's logging.SetupWithPrefix() (slog, split by level) |
| Price | HDPay's PriceService (CoinGecko, cached) |
| Rate Limiting | HDPay's scanner.RateLimiter + CircuitBreaker |
| Auth | bcrypt + cookie sessions (in-memory), IP allowlist |
| Frontend | SvelteKit (adapter-static, TS strict, Tailwind+shadcn-svelte, ECharts) |
| Deployment | Single Go binary with embedded SPA (go:embed) |

## Next Actions
- Run `/cf-next` to start Phase 5: API Layer
- Phase 5 deliverable: Chi router with auth, IP allowlist, watch CRUD, points/tx queries, admin endpoints

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/02-plan.md` | V1 feature plan (35+3+5 features) |
| `.project/03-mockups/` | HTML mockups (7 screens + component gallery) |
| `.project/04-build-plan/BUILD-PLAN.md` | Overall build direction + HDPay reuse strategy |
| `.project/04-build-plan/phases/phase-01/PLAN.md` | Phase 1 plan |
| `.project/04-build-plan/phases/phase-01/SUMMARY.md` | Phase 1 summary (complete) |
| `.project/04-build-plan/phases/phase-02/PLAN.md` | Phase 2 plan |
| `.project/04-build-plan/phases/phase-02/SUMMARY.md` | Phase 2 summary (complete) |
| `.project/04-build-plan/phases/phase-03/PLAN.md` | Phase 3 plan |
| `.project/04-build-plan/phases/phase-03/SUMMARY.md` | Phase 3 summary (complete) |
| `.project/04-build-plan/phases/phase-04/PLAN.md` | Phase 4 plan (detailed, complete) |
| `.project/04-build-plan/phases/phase-04/SUMMARY.md` | Phase 4 summary (complete) |
| `.project/custom/PROMPT.md` | Development guidelines (future CLAUDE.md) |

## Session History
| # | Date | Phase | Summary |
|---|------|-------|---------|
| 1 | 2026-02-20 | plan+mockup | Project init, plan phase completed, mockup phase completed (7 screens + gallery) |
| 2 | 2026-02-20 | build_plan | Build plan completed + audited. HDPay reuse: Poller as cmd in HDPay module. Decisions: login IP-exempt, watch defaults runtime-only, block_number column added. |
| 3 | 2026-02-20 | building | Phase 1 Foundation completed. Scaffolded config, DB (5 tables, 30 CRUD methods), main entry point, Makefile targets. Parameterized HDPay logging. Tests pass (config 82.6%, pollerdb 76.9%). |
| 4 | 2026-02-20 | building | Phase 2 Core Services completed. Tier config (load/validate/create), points calculator (flat, thread-safe), price wrapper (stablecoin $1, retry), address validation (BTC/BSC/SOL). Tests: points 93.7%, validate 100.0%. |
| 5 | 2026-02-20 | building | Phase 3 Blockchain Providers completed. Provider interface (tx detection), ProviderSet (round-robin, RateLimiter, CircuitBreaker), BTC (Blockstream+Mempool, pagination), BSC (BscScan txlist+tokentx, block confirmations), SOL (JSON-RPC, native+SPL, composite tx_hash). 49 tests, 82.5% coverage. |
| 6 | 2026-02-20 | building | Phase 4 Watch Engine completed. Watcher orchestrator (goroutine-per-watch, context timeouts, WaitGroup shutdown). Poll loop (tx detection, confirmation tracking, points calculation). Startup recovery (expire stale, retry pending). main.go full integration. 31 tests, 71% coverage, 0 races. |
