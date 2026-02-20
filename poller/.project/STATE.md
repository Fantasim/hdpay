# Project State: Poller

> A standalone microservice that monitors cryptocurrency addresses for incoming payments and converts them into points for a video game economy. Zero runtime dependency on HDPay (at runtime). Imports HDPay's Go packages at compile time.

## Current Position
- **Version:** V1
- **Phase:** Building (Phase 2 of 8 next)
- **Status:** Phase 1 (Foundation) complete. Config, DB, CRUD, health endpoint all working with tests.
- **Last session:** 2026-02-20 — Phase 1 Foundation built and tested

## Build Progress

| Phase | Name | Status | Sessions |
|-------|------|--------|----------|
| 1 | Foundation | **DONE** | 1 |
| 2 | Core Services | **NEXT** | ~1 |
| 3 | Blockchain Providers | Pending | ~1-2 |
| 4 | Watch Engine | Pending | ~2 |
| 5 | API Layer | Pending | ~1-2 |
| 6 | Frontend Setup & Auth | Pending | ~1 |
| 7 | Dashboard Pages | Pending | ~2-3 |
| 8 | Embedding & Polish | Pending | ~1 |

**Total estimate: 10-13 sessions (1 done)**

## Phase 1 Deliverables (Complete)
- `cmd/poller/main.go` — Entry point with Chi server, health endpoint, graceful shutdown
- `internal/poller/config/` — Config struct (envconfig), constants, error codes
- `internal/poller/models/types.go` — Domain types (Watch, Transaction, PointsAccount, etc.)
- `internal/poller/pollerdb/` — SQLite DB layer (5 tables, 30 CRUD methods, migration runner)
- Parameterized HDPay's `logging.Setup()` → `SetupWithPrefix()` for Poller prefix
- Makefile targets: `dev-poller`, `build-poller`, `test-poller`
- Tests: config 82.6%, pollerdb 76.9% coverage

## Phase 2: Core Services (Next)
- PriceService integration (import HDPay's CoinGecko client)
- TierCalculator (load tiers.json, compute multiplier)
- PointsCalculator (USD → tier → multiplier → points)
- ConfirmationChecker (chain-specific confirmation counting)
- Unit tests for all services

## Key Decisions
- **Module structure**: Poller lives inside HDPay's Go module (`cmd/poller/` + `internal/poller/`). Full access to HDPay's `internal/` packages. Two binaries from one module.
- **HDPay reuse**: Import logging.SetupWithPrefix(), config constants/errors, models, PriceService, RateLimiter, CircuitBreaker, request logging middleware. Write new: watcher, providers (tx detection), points calculator, session auth, IP allowlist, handlers, DB CRUD, frontend.
- **Logging parameterization**: `SetupWithPrefix()` added. Poller uses `poller-*` prefix, HDPay uses `hdpay-*`. Backward-compatible.
- **Login exempt from IP checks**: `/api/admin/login` and `/api/health` bypass IP allowlist
- **Watch defaults runtime-only**: Loaded from env vars, editable at runtime from dashboard, lost on restart. No settings table.
- **block_number column**: Added to transactions table for BSC confirmation counting
- **Build output**: `bin/poller` via Makefile (avoids conflict with `poller/` directory)
- Brainstorm & Market phases skipped (project fully specified in custom docs)
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
- Run `/cf-next` to start Phase 2: Core Services
- Phase 2 deliverable: PriceService, TierCalculator, PointsCalculator, ConfirmationChecker with tests

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
| `.project/custom/PROMPT.md` | Development guidelines (future CLAUDE.md) |

## Session History
| # | Date | Phase | Summary |
|---|------|-------|---------|
| 1 | 2026-02-20 | plan+mockup | Project init, plan phase completed, mockup phase completed (7 screens + gallery) |
| 2 | 2026-02-20 | build_plan | Build plan completed + audited. HDPay reuse: Poller as cmd in HDPay module. Decisions: login IP-exempt, watch defaults runtime-only, block_number column added. |
| 3 | 2026-02-20 | building | Phase 1 Foundation completed. Scaffolded config, DB (5 tables, 30 CRUD methods), main entry point, Makefile targets. Parameterized HDPay logging. Tests pass (config 82.6%, pollerdb 76.9%). |
