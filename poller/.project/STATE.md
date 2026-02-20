# Project State: Poller

> A standalone microservice that monitors cryptocurrency addresses for incoming payments and converts them into points for a video game economy. Zero runtime dependency on HDPay (at runtime). Imports HDPay's Go packages at compile time.

## Current Position
- **Version:** V1
- **Phase:** Building (Phase 7 of 8 next)
- **Status:** Phase 6 (Frontend Setup & Auth) complete. SvelteKit project at `web/poller/`, Tailwind v4 + shadcn-svelte, design system ported, login page, sidebar, auth store, API client, 6 stub routes.
- **Last session:** 2026-02-20 — Phase 6 Frontend Setup & Auth built

## Build Progress

| Phase | Name | Status | Sessions |
|-------|------|--------|----------|
| 1 | Foundation | **DONE** | 1 |
| 2 | Core Services | **DONE** | 1 |
| 3 | Blockchain Providers | **DONE** | 1 |
| 4 | Watch Engine | **DONE** | 1 |
| 5 | API Layer | **DONE** | 1 |
| 6 | Frontend Setup & Auth | **DONE** | 1 |
| 7 | Dashboard Pages | **NEXT** | ~2-3 |
| 8 | Embedding & Polish | Pending | ~1 |

**Total estimate: 10-13 sessions (6 done)**

## Phase 6 Deliverables (Complete)
- `web/poller/` — Standalone SvelteKit project (adapter-static, SPA mode)
- `web/poller/src/app.css` — Design system: shadcn variables + Poller mockup tokens (dark-only)
- `web/poller/src/lib/types.ts` — 25+ TypeScript interfaces matching backend models
- `web/poller/src/lib/constants.ts` — All constants (chains, colors, statuses, error codes, nav items, explorer URLs)
- `web/poller/src/lib/utils/api.ts` — Fetch wrapper + 20 endpoint functions (auto 401 redirect)
- `web/poller/src/lib/utils/formatting.ts` — Number/address/date formatting utilities
- `web/poller/src/lib/stores/auth.ts` — Auth state (login/logout/checkSession)
- `web/poller/src/lib/components/layout/Sidebar.svelte` — 240px sidebar, 6 nav items, network badge
- `web/poller/src/lib/components/layout/Header.svelte` — Page header with title/subtitle/actions
- `web/poller/src/routes/+layout.svelte` — Auth gating, sidebar shell, ModeWatcher
- `web/poller/src/routes/login/+page.svelte` — Login page (matches mockup)
- `web/poller/src/routes/{transactions,watches,points,errors,settings}/+page.svelte` — Stub pages
- `web/poller/src/lib/components/ui/` — shadcn components (button, card, input, label, badge, separator)

## Phase 7: Dashboard Pages (Next)
- Overview dashboard (stats cards, daily chart, chain/token breakdown)
- Transactions page (filterable table, pagination)
- Watches page (active/completed list, create watch form)
- Points page (accounts table, claim actions)
- Errors page (discrepancies, system errors, stale pending)
- Settings page (tiers editor, watch defaults, IP allowlist)
- Install Apache ECharts for chart components

## Key Decisions
- **Module structure**: Poller lives inside HDPay's Go module (`cmd/poller/` + `internal/poller/`). Full access to HDPay's `internal/` packages. Two binaries from one module.
- **HDPay reuse**: Import logging.SetupWithPrefix(), config constants/errors, models, PriceService, RateLimiter, CircuitBreaker, request logging middleware. Write new: watcher, providers (tx detection), points calculator, session auth, IP allowlist, handlers, DB CRUD, frontend.
- **Independent frontend**: `web/poller/` is a fully standalone SvelteKit project, not sharing code with HDPay's `web/`. Shares visual design language only.
- **httputil package**: Response helpers in separate package to avoid import cycle between api/ and handlers/
- **CORS Allow-Origin: ***: IP allowlist is the security boundary, not CORS
- **Login IP-exempt**: POST /api/admin/login and GET /api/health bypass both IP allowlist and session auth
- **Composite tx_hash for SOL**: `"signature:SOL"`, `"signature:USDC"` — one Solana tx can produce multiple RawTransactions
- **Goroutine-per-watch**: context.Background() with timeout, WaitGroup for graceful shutdown
- **Smart cutoff**: MAX(last recorded tx detected_at, POLLER_START_DATE)
- Build output: `bin/poller` via Makefile (avoids conflict with `poller/` directory)
- Backend-first build order (all Go before any frontend)
- Desktop-only (no responsive), manual refresh (no SSE)
- Dark-only app: `:root` and `.dark` have identical values, ModeWatcher defaultMode="dark"

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
| Frontend | SvelteKit (adapter-static, TS strict, Tailwind v4+shadcn-svelte) |
| Charts | Apache ECharts (to be added in Phase 7) |
| Deployment | Single Go binary with embedded SPA (go:embed) |

## Next Actions
- Run `/cf-next` to start Phase 7: Dashboard Pages
- Phase 7 deliverable: All 6 dashboard pages with real data fetching, charts, tables, forms

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/02-plan.md` | V1 feature plan (35+3+5 features) |
| `.project/03-mockups/` | HTML mockups (7 screens + component gallery) |
| `.project/04-build-plan/BUILD-PLAN.md` | Overall build direction + HDPay reuse strategy |
| `.project/04-build-plan/phases/phase-NN/PLAN.md` | Phase N plan |
| `.project/04-build-plan/phases/phase-NN/SUMMARY.md` | Phase N summary |
| `.project/custom/PROMPT.md` | Development guidelines |

## Session History
| # | Date | Phase | Summary |
|---|------|-------|---------|
| 1 | 2026-02-20 | plan+mockup | Project init, plan phase completed, mockup phase completed (7 screens + gallery) |
| 2 | 2026-02-20 | build_plan | Build plan completed + audited. HDPay reuse: Poller as cmd in HDPay module. |
| 3 | 2026-02-20 | building | Phase 1 Foundation completed. Config, DB (5 tables, 30 CRUD methods), main, Makefile. |
| 4 | 2026-02-20 | building | Phase 2 Core Services completed. Tier config, points calculator, price wrapper, address validation. |
| 5 | 2026-02-20 | building | Phase 3 Blockchain Providers completed. BTC/BSC/SOL tx detection, ProviderSet round-robin. |
| 6 | 2026-02-20 | building | Phase 4 Watch Engine completed. Goroutine-per-watch, poll loop, startup recovery, graceful shutdown. |
| 7 | 2026-02-20 | building | Phase 5 API Layer completed. Chi router, middleware, 17 endpoints, dashboard DB, 40 tests. |
| 8 | 2026-02-20 | building | Phase 6 Frontend Setup & Auth completed. SvelteKit project, design system, login, sidebar, auth, API client, 6 stub routes. |
