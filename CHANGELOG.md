# Changelog

## [Unreleased]

### 2026-02-18

#### Added
- Go module with full directory structure (cmd/, internal/)
- Central constants (`internal/config/constants.go`) — all numeric/string values
- Central error codes (`internal/config/errors.go`) — all error types
- Config loading via envconfig (`internal/config/config.go`)
- Structured logging with dual output: stdout + daily rotated file (`internal/logging/logger.go`)
- SQLite database with WAL mode, migration system, initial schema with 5 tables (`internal/db/`)
- Chi router with middleware stack: request logging, host check, CORS, CSRF (`internal/api/`)
- Health endpoint (`GET /api/health`)
- Server entry point with graceful shutdown (`cmd/server/main.go`)
- SvelteKit frontend with adapter-static, TypeScript strict mode, Tailwind CSS v4
- Design tokens from mockup phase ported to `app.css`
- Sidebar layout component matching mockup (240px, icons, active states, network badge)
- All route placeholders (Dashboard, Addresses, Scan, Send, Transactions, Settings)
- Frontend TypeScript types (`lib/types.ts`) and constants (`lib/constants.ts`)
- API client with CSRF token handling (`lib/utils/api.ts`)
- Formatting utilities (`lib/utils/formatting.ts`)
- Header component with title and optional actions slot
- Makefile with dev, build, test, lint targets
- `.env.example` with documented environment variables
- `.gitignore` for Go binary, data, logs, node_modules
- Unit tests for logging and database modules
