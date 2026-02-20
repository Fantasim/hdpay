# Project State: Poller

> A standalone microservice that monitors cryptocurrency addresses for incoming payments and converts them into points for a video game economy. Zero runtime dependency on HDPay (at runtime). Imports HDPay's Go packages at compile time.

## Current Position
- **Version:** V1
- **Phase:** Building — ALL 8 PHASES COMPLETE
- **Status:** V1 build finished. Single 21MB binary with embedded SvelteKit frontend. Security hardened, test coverage boosted.
- **Last session:** 2026-02-20 — Phase 8 Embedding & Polish completed (final phase)

## Build Progress

| Phase | Name | Status | Sessions |
|-------|------|--------|----------|
| 1 | Foundation | **DONE** | 1 |
| 2 | Core Services | **DONE** | 1 |
| 3 | Blockchain Providers | **DONE** | 1 |
| 4 | Watch Engine | **DONE** | 1 |
| 5 | API Layer | **DONE** | 1 |
| 6 | Frontend Setup & Auth | **DONE** | 1 |
| 7 | Dashboard Pages | **DONE** | 1 |
| 8 | Embedding & Polish | **DONE** | 1 |

**Total: 10 sessions (all 8 phases complete)**

## Phase 7 Deliverables (Complete)

### Pages Built
- **Overview** (`/`) — 8 stats cards (2x4 grid), time range selector, 7 ECharts components
- **Transactions** (`/transactions`) — 11-col table, 6 filters, page size control (25/50/100), server-side pagination
- **Watches** (`/watches`) — Filter chips (status + chain), table with live countdown timers (1s interval)
- **Points** (`/points`) — 3 summary cards (unclaimed/pending/all-time), points accounts table
- **Errors** (`/errors`) — 3 card sections (discrepancies, stale pending, system errors)
- **Settings** (`/settings`) — Tier editor (inline inputs), IP allowlist (add/remove), watch defaults, system info grid

### Components Created
- `ChartWrapper.svelte` — Reusable ECharts wrapper with tree-shaking (Bar, Line, Pie)
- 7 chart components: UsdOverTime, PointsOverTime, TxCount, ChainBreakdown, TokenBreakdown, TierDistribution, WatchesOverTime
- `TimeRangeSelector.svelte`, `StatsCard.svelte`
- `explorer.ts` — Block explorer URL helper (handles SOL composite hashes)

### Dependencies Added
- `echarts`, `svelte-echarts@1.0.0`, `@tanstack/table-core`

## Phase 8: Embedding & Polish (Complete)
- Single 21MB Go binary with embedded SvelteKit frontend via `go:embed`
- SPA handler with immutable asset caching and client-side routing fallback
- Security fixes: Helius API key leak, cookie Secure flag
- Constants audit: date range values extracted to config
- Utility dedup: shared `badgeClass()` replacing inline duplicates
- Test coverage boost: pollerdb 47.6% → 77.3%

## Key Decisions
- **Module structure**: Poller lives inside HDPay's Go module (`cmd/poller/` + `internal/poller/`). Full access to HDPay's `internal/` packages. Two binaries from one module.
- **HDPay reuse**: Import logging.SetupWithPrefix(), config constants/errors, models, PriceService, RateLimiter, CircuitBreaker, request logging middleware. Write new: watcher, providers (tx detection), points calculator, session auth, IP allowlist, handlers, DB CRUD, frontend.
- **Independent frontend**: `web/poller/` is a fully standalone SvelteKit project, not sharing code with HDPay's `web/`. Shares visual design language only.
- **ECharts tree-shaking**: Import from `echarts/core` subpaths, use built-in `dark` theme
- **Server-side pagination**: Transactions page sends filter/page params to API
- **Shared utilities**: `chainBadgeClass()` in formatting.ts, `getTxExplorerUrl()` in explorer.ts
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
| Charts | Apache ECharts (svelte-echarts@1.0.0, tree-shaking) |
| Deployment | Single Go binary with embedded SPA (go:embed) |

## Next Actions
- All V1 build phases complete!
- Run `/cf-save` to save the final session state
- Run `/cf-new-version` to start planning V2

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
| 9 | 2026-02-20 | building | Phase 7 Dashboard Pages completed. All 6 pages (overview+charts, transactions, watches, points, errors, settings). ECharts integrated. |
| 10 | 2026-02-20 | building | Phase 8 Embedding & Polish completed (FINAL). 21MB binary, security fixes, constants/dedup audit, pollerdb coverage 77.3%. |
