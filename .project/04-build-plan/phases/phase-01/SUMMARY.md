# Phase 1 Summary: Project Foundation

## Completed: 2026-02-18

## What Was Built
- Go module (`github.com/Fantasim/hdpay`) with full directory structure matching CLAUDE.md spec
- Central constants, error codes, and config loading via envconfig
- Structured logging (slog) with dual output: stdout + daily rotated log file
- SQLite database with WAL mode, embedded migration system, initial schema (5 tables: addresses, balances, scan_state, transactions, settings)
- Chi v5 router with middleware stack: request logging, host check (localhost-only), CORS, CSRF (double-submit cookie)
- Health endpoint (`GET /api/health`) returning JSON with status, version, network, dbPath
- Server entry point with `serve` subcommand and graceful shutdown (SIGINT/SIGTERM)
- SvelteKit frontend with adapter-static, TypeScript strict mode, Tailwind CSS v4
- Design tokens from mockup phase ported to app.css
- Sidebar layout component (240px, SVG icons, active state highlighting, network badge)
- All 6 route placeholders (Dashboard, Addresses, Scan, Send, Transactions, Settings)
- Core TypeScript types, constants, API client with CSRF handling, formatting utilities
- Makefile with dev/build/test/lint targets
- GitHub repo created and initial commit pushed

## Files Created/Modified
- `cmd/server/main.go` — Entry point with serve/version subcommands
- `internal/config/constants.go` — All numeric/string constants
- `internal/config/errors.go` — Error codes and sentinel errors
- `internal/config/config.go` — Config struct with envconfig
- `internal/logging/logger.go` — Dual-output slog setup
- `internal/logging/logger_test.go` — Logger tests
- `internal/db/sqlite.go` — SQLite connection, WAL mode, migration runner
- `internal/db/sqlite_test.go` — DB tests (open, migrate, idempotent)
- `internal/db/migrations/001_initial.sql` — Initial schema
- `internal/api/router.go` — Chi router with middleware
- `internal/api/middleware/security.go` — HostCheck, CORS, CSRF middleware
- `internal/api/middleware/logging.go` — Request logging middleware
- `internal/api/handlers/health.go` — Health endpoint
- `internal/models/types.go` — Domain types
- `web/src/app.css` — Tailwind + design tokens
- `web/src/lib/types.ts` — TypeScript interfaces
- `web/src/lib/constants.ts` — Frontend constants
- `web/src/lib/utils/api.ts` — API client
- `web/src/lib/utils/formatting.ts` — Formatting utilities
- `web/src/lib/components/layout/Sidebar.svelte` — Sidebar navigation
- `web/src/lib/components/layout/Header.svelte` — Page header
- `web/src/routes/+layout.svelte` — App shell with sidebar
- `web/src/routes/*/+page.svelte` — 6 route placeholders

## Decisions Made
- Go 1.25.7 used (compatible with go 1.22+ requirement)
- Tailwind CSS v4 with @tailwindcss/vite plugin (no separate config file needed)
- Svelte 5 with runes syntax ($props, $state, etc.)
- `page` imported from `$app/state` (Svelte 5 pattern)
- Vite proxy configured to forward /api to Go backend on port 8080

## Deviations from Plan
- shadcn-svelte not installed yet — will add component-by-component as needed in later phases
- `init` and `export` CLI subcommands deferred to Phase 2 (wallet init) and Phase 3 (export)

## Issues Encountered
- Port 8080 was already in use during testing — verified on port 8099 instead
- Go binary not on PATH by default — need `export PATH=$PATH:/usr/local/go/bin`

## Notes for Next Phase
- Go PATH needs `/usr/local/go/bin` added
- Frontend dev server proxies `/api` to `http://127.0.0.1:8080`
- Database created at `./data/hdpay.sqlite` — directory auto-created
- All stub packages (wallet, scanner, tx, price) have empty `.go` files ready
