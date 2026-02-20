# Phase 8 Summary: Embedding & Polish

## Completed: 2026-02-20

## What Was Built
- Single 21MB Go binary with embedded SvelteKit frontend via `go:embed`
- SPA handler with immutable asset caching and client-side routing fallback
- Makefile target that chains frontend build → Go build in one command
- Security fixes: API key leak prevention, cookie hardening
- Constants audit: extracted hardcoded date range values
- Utility dedup: shared `badgeClass()` replacing inline duplicates
- Test coverage boost: pollerdb from 47.6% → 77.3% (dashboard + discrepancy tests)

## Files Created
- `web/poller/embed.go` — `//go:embed all:build` directive
- `internal/poller/pollerdb/dashboard_test.go` — 8 test functions for dashboard DB queries
- `internal/poller/pollerdb/discrepancy_test.go` — 7 test functions for discrepancy checks + transaction queries

## Files Modified
- `internal/poller/api/router.go` — Added `StaticFS` to Dependencies, wired SPA handler
- `cmd/poller/main.go` — Added `fs.Sub()` for embedded FS, passed to router deps
- `Makefile` — Added `build-poller-frontend` target, chained into `build-poller`, updated `clean`
- `internal/poller/provider/sol.go` — **SECURITY**: Removed API key from log output
- `internal/poller/api/handlers/auth.go` — **SECURITY**: Added `Secure: true` to session cookies
- `internal/poller/api/handlers/dashboard.go` — Replaced hardcoded date range values with constants
- `internal/poller/config/constants.go` — Added `DateRangeWeekDays/MonthDays/QuarterDays` constants
- `web/poller/src/lib/utils/formatting.ts` — Added `badgeClass()` and `abbreviateNumber()`
- `web/poller/src/routes/watches/+page.svelte` — Uses shared `badgeClass()` instead of inline function

## Decisions Made
- **Reuse HDPay's SPAHandler**: Imported `internal/api/handlers.SPAHandler()` directly rather than duplicating — same logic (file existence check, immutable cache headers, SPA fallback)
- **StaticFS nil check**: Router only registers SPA handler when `StaticFS != nil`, allowing dev mode without embedded frontend
- **fs.Sub in main.go**: Strip "build/" prefix at startup, not in router — cleaner dependency injection
- **Secure cookie always**: Set `Secure: true` even though app is localhost-only — defense in depth

## Deviations from Plan
- Skipped creating `api.ts` HTTP GET helper consolidation — existing pattern with per-endpoint functions is clearer and more type-safe
- Skipped chart formatter dedup across pages — each chart page has specific formatting needs, extracting would over-abstract

## Issues Encountered
- **Import alias mismatch**: Dashboard handler imported config as `pollerconfig` but constants were referenced as `config.` — fixed immediately
- **Build output conflict**: `go build ./cmd/poller/` fails because `poller/` directory exists — already solved in Makefile with explicit `-o bin/poller`

## Test Coverage (Final)
| Package | Coverage |
|---------|----------|
| watcher | 71.0% |
| points | 93.7% |
| provider | 82.5% |
| pollerdb | 77.3% |
| validate | 100.0% |

## Notes for V2
- Large chunk warning for ECharts (533KB) — consider dynamic import for code splitting
- No frontend tests yet (Vitest + Testing Library) — should be V2 priority
- IP allowlist + session auth cover security, but rate limiting on API endpoints not yet implemented
