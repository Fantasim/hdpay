# Phase 11 Summary: History, Settings & Deployment

## Completed: 2026-02-18

## What Was Built

### Transaction History API + Frontend
- `ListTransactionsFiltered` DB method with dynamic WHERE clause builder supporting chain, direction, token, status filters
- `ListTransactions` handler serving both `GET /api/transactions` and `GET /api/transactions/{chain}` with query param validation
- Full transactions page: filter toolbar (chain/direction/token chip groups), sortable table with chain badges, direction icons, explorer links, copy-to-clipboard, status badges, pagination with page numbers

### Settings API + Frontend
- `GetSetting`, `SetSetting` (upsert), `GetAllSettings` (fills defaults) DB methods
- `ResetBalances` (preserves addresses, clears balances/scan_state/transactions) and `ResetAll` (clears everything) in single DB transactions
- `GetSettings`, `UpdateSettings` (partial updates with key validation), `ResetBalancesHandler`, `ResetAllHandler` (requires `{"confirm": true}`)
- Full settings page: Network mode radio cards, Scanning config (max scan ID, auto-resume toggle, resume threshold), Transaction config (BTC fee rate, BSC gas pre-seed), Display config (log level), Danger zone with two-step confirmation

### Embedded Static File Server
- `web/embed.go` with `//go:embed all:build` embedding SvelteKit static output
- `SPAHandler` in `internal/api/handlers/spa.go`: serves static files, immutable cache headers for `_app/` assets, SPA fallback to index.html for client-side routing
- Router `NotFound` handler wired as catch-all after `/api` routes
- `fs.Sub` to strip `build/` prefix from embed FS

### Build & Deployment
- `make build` produces 22MB single binary with embedded SPA
- Version injected via `-ldflags "-X main.version=$(VERSION)"`
- Makefile targets already existed from Phase 1 — verified working

### Tests
- 8 DB settings tests: GetSetting default/unknown, SetSetting/Get, upsert, GetAllSettings defaults/overrides, ResetBalances, ResetAll
- 5 DB transaction filter tests: by direction, token, status, multiple filters combined
- 9 handler settings tests: get, update, invalid key, invalid body, reset confirmation flow
- 13 handler transaction tests: all, chain path/query, direction, token, status, pagination, invalid params
- 5 SPA handler tests: static file serving, immutable cache headers, SPA fallback, API 404, root index

## Files Created/Modified
- `internal/db/transactions.go` — Added `TransactionFilter`, `ListTransactionsFiltered`, `joinConditions`
- `internal/db/settings.go` — NEW: settings CRUD, reset operations
- `internal/api/handlers/transactions.go` — NEW: ListTransactions handler
- `internal/api/handlers/settings.go` — Expanded from stub to full handler file
- `internal/api/handlers/spa.go` — NEW: SPA handler with fallback
- `internal/api/router.go` — Added transaction, settings, SPA routes + fs.FS param
- `cmd/server/main.go` — Added web embed import, fs.Sub, pass to router
- `web/embed.go` — NEW: embed directive for SvelteKit build
- `web/src/routes/transactions/+page.svelte` — Full transaction history page
- `web/src/routes/settings/+page.svelte` — Full settings page
- `web/src/lib/constants.ts` — Added TX/settings constants
- `web/src/lib/types.ts` — Added Settings, TransactionListParams types
- `web/src/lib/utils/api.ts` — Added transaction/settings API functions
- `internal/db/settings_test.go` — NEW: 8 tests
- `internal/db/transactions_test.go` — Added 5 filter tests
- `internal/api/handlers/settings_test.go` — NEW: 9 tests
- `internal/api/handlers/transactions_test.go` — NEW: 13 tests
- `internal/api/handlers/spa_test.go` — NEW: 5 tests

## Decisions Made
- **SPA via NotFound handler**: Uses Chi's `r.NotFound()` so API routes take priority, unknown paths get SPA fallback
- **readSeeker interface**: SPA handler uses type assertion to `io.ReadSeeker` for `http.ServeContent` — works with `embed.FS` files
- **Cache strategy**: `_app/immutable/**` gets `max-age=31536000, immutable`; everything else gets `no-cache`
- **Settings defaults in Go**: `defaultSettings` map in `settings.go` provides defaults when DB has no value

## Deviations from Plan
- Makefile already existed from Phase 1 with all needed targets — no new build script needed
- No PROJECT-MAP.md file existed to update (was not created in earlier phases)

## Notes for Next Phase
- V1 is complete! All 11 build phases done.
- Binary builds to 22MB with embedded SPA
- Full test suite passes (40 new tests in this phase)
