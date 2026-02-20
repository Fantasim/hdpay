# Project State: Poller

> A standalone microservice that monitors cryptocurrency addresses for incoming payments and converts them into points for a video game economy. Zero runtime dependency on HDPay (at runtime). Imports HDPay's Go packages at compile time.

## Current Position
- **Version:** V1
- **Phase:** Building (Phase 4 of 8 next)
- **Status:** Phase 3 (Blockchain Providers) complete. Provider interface, round-robin, BTC/BSC/SOL tx detection with tests.
- **Last session:** 2026-02-20 — Phase 3 Blockchain Providers built and tested

## Build Progress

| Phase | Name | Status | Sessions |
|-------|------|--------|----------|
| 1 | Foundation | **DONE** | 1 |
| 2 | Core Services | **DONE** | 1 |
| 3 | Blockchain Providers | **DONE** | 1 |
| 4 | Watch Engine | **NEXT** | ~2 |
| 5 | API Layer | Pending | ~1-2 |
| 6 | Frontend Setup & Auth | Pending | ~1 |
| 7 | Dashboard Pages | Pending | ~2-3 |
| 8 | Embedding & Polish | Pending | ~1 |

**Total estimate: 10-13 sessions (3 done)**

## Phase 3 Deliverables (Complete)
- `internal/poller/provider/provider.go` — Provider interface, RawTransaction, ProviderSet (round-robin, rate limit, circuit breaker), NewHTTPClient
- `internal/poller/provider/btc.go` — BlockstreamProvider, MempoolProvider (shared Esplora format, pagination, satoshi→human)
- `internal/poller/provider/bsc.go` — BscScanProvider (txlist+tokentx, USDC/USDT by contract, block-based confirmations, weiToHuman)
- `internal/poller/provider/sol.go` — SolanaRPCProvider (getSignaturesForAddress+getTransaction, native+SPL, composite tx_hash, lamportsToHuman)
- Tests: 49 tests, 82.5% coverage
- Constants: ErrorCategoryProvider/Watcher, ErrorSeverity levels added to poller config

## Phase 4: Watch Engine (Next)
- Watcher service: poll active watches on their chain-specific intervals
- Transaction detection: use ProviderSet.ExecuteFetch() per active watch
- Confirmation tracking: use ProviderSet.ExecuteConfirmation() for pending txs
- Points calculation: integrate PointsCalculator + Pricer on confirmed txs
- DB integration: store detected txs, update points accounts, log errors to system_errors
- Watch lifecycle: active → completed (on expiry or manual stop)

## Key Decisions
- **Module structure**: Poller lives inside HDPay's Go module (`cmd/poller/` + `internal/poller/`). Full access to HDPay's `internal/` packages. Two binaries from one module.
- **HDPay reuse**: Import logging.SetupWithPrefix(), config constants/errors, models, PriceService, RateLimiter, CircuitBreaker, request logging middleware. Write new: watcher, providers (tx detection), points calculator, session auth, IP allowlist, handlers, DB CRUD, frontend.
- **MempoolProvider embeds BlockstreamProvider**: Same Esplora API, only Name() and baseURL differ
- **Composite tx_hash for SOL**: `"signature:SOL"`, `"signature:USDC"` — one Solana tx can produce multiple RawTransactions
- **BTC confirmation**: 1-conf from Esplora `confirmed=true` (no block counting)
- **BSC confirmation**: `currentBlock - txBlock >= 12` via BscScan proxy
- **SOL confirmation**: `confirmationStatus == "finalized"` (no block counting)
- **Helius**: Falls back to Solana devnet in testnet mode
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
- Run `/cf-next` to start Phase 4: Watch Engine
- Phase 4 deliverable: Watcher service that polls active watches, detects txs, confirms, calculates points

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
| `.project/custom/PROMPT.md` | Development guidelines (future CLAUDE.md) |

## Session History
| # | Date | Phase | Summary |
|---|------|-------|---------|
| 1 | 2026-02-20 | plan+mockup | Project init, plan phase completed, mockup phase completed (7 screens + gallery) |
| 2 | 2026-02-20 | build_plan | Build plan completed + audited. HDPay reuse: Poller as cmd in HDPay module. Decisions: login IP-exempt, watch defaults runtime-only, block_number column added. |
| 3 | 2026-02-20 | building | Phase 1 Foundation completed. Scaffolded config, DB (5 tables, 30 CRUD methods), main entry point, Makefile targets. Parameterized HDPay logging. Tests pass (config 82.6%, pollerdb 76.9%). |
| 4 | 2026-02-20 | building | Phase 2 Core Services completed. Tier config (load/validate/create), points calculator (flat, thread-safe), price wrapper (stablecoin $1, retry), address validation (BTC/BSC/SOL). Tests: points 93.7%, validate 100.0%. |
| 5 | 2026-02-20 | building | Phase 3 Blockchain Providers completed. Provider interface (tx detection), ProviderSet (round-robin, RateLimiter, CircuitBreaker), BTC (Blockstream+Mempool, pagination), BSC (BscScan txlist+tokentx, block confirmations), SOL (JSON-RPC, native+SPL, composite tx_hash). 49 tests, 82.5% coverage. |
