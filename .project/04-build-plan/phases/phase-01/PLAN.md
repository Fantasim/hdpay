# Phase 1: Project Foundation

<objective>
Set up the complete project infrastructure: Go backend with Chi router, SQLite database, structured logging, security middleware, and SvelteKit frontend scaffold with Tailwind + shadcn-svelte and the sidebar layout. The result is a running localhost application with a health endpoint and the shell UI.
</objective>

<tasks>

## Task 1: Go Module & Directory Structure

Create the Go module and full directory tree as specified in CLAUDE.md.

**Files to create:**
- `go.mod` — module `github.com/hdpay/hdpay`, Go 1.22+
- `cmd/server/main.go` — entry point with `init`, `serve`, `export`, `version` subcommands (stubs)
- All `internal/` package directories (empty `.go` files with package declarations):
  - `internal/api/router.go`
  - `internal/api/handlers/` (stub files)
  - `internal/api/middleware/security.go`, `logging.go`
  - `internal/wallet/` (empty, built in Phase 2)
  - `internal/scanner/` (empty, built in Phase 4)
  - `internal/tx/` (empty, built in Phase 7-9)
  - `internal/db/sqlite.go`, `internal/db/migrations/`
  - `internal/price/` (empty, built in Phase 6)
  - `internal/config/config.go`, `constants.go`, `errors.go`
  - `internal/models/types.go`
  - `internal/logging/logger.go`

**Verification:**
- `go build ./...` succeeds
- Directory structure matches CLAUDE.md specification

## Task 2: Constants, Errors & Config

Populate the central configuration files per CLAUDE.md "Constants Are Sacred" rule.

**Files:**
- `internal/config/constants.go` — ALL numeric/string constants (copy from CLAUDE.md spec)
- `internal/config/errors.go` — ALL error types and error code strings
- `internal/config/config.go` — Config struct with envconfig tags
- `.env.example` — documented environment variable template

**Constants include:** address limits, BIP-44 paths, token contracts, scan batch sizes, rate limits, transaction gas limits, server timeouts, log settings, DB settings, price API config.

**Verification:**
- All constants referenced in CLAUDE.md are present
- `go vet ./internal/config/...` passes

## Task 3: Structured Logging (slog)

Set up dual-output structured logging: stdout + daily rotated log files.

**Files:**
- `internal/logging/logger.go` — `SetupLogger(level, logDir)` function
  - Parse log level string → slog.Level
  - Create log directory if not exists
  - Open daily log file (`hdpay-YYYY-MM-DD.log`)
  - MultiWriter to stdout + file
  - JSON handler with configured level
  - Set as default logger

**Verification:**
- `slog.Info("test")` outputs to both stdout and log file
- Log file created in `./logs/` with correct date
- Log level filtering works (DEBUG messages hidden at INFO level)
- Unit test for logger setup

## Task 4: SQLite Database Setup

Initialize SQLite with WAL mode and migration system.

**Files:**
- `internal/db/sqlite.go`:
  - `NewDB(path string) (*DB, error)` — open connection, enable WAL, set busy timeout
  - `Close()` method
  - `RunMigrations()` — apply pending migrations from embedded SQL files
  - `schema_migrations` table to track applied migrations
- `internal/db/migrations/001_initial.sql`:
  ```sql
  CREATE TABLE IF NOT EXISTS addresses (
      chain TEXT NOT NULL,
      address_index INTEGER NOT NULL,
      address TEXT NOT NULL,
      created_at TEXT NOT NULL DEFAULT (datetime('now')),
      PRIMARY KEY (chain, address_index)
  );
  CREATE INDEX idx_addresses_chain ON addresses(chain);
  CREATE INDEX idx_addresses_address ON addresses(address);

  CREATE TABLE IF NOT EXISTS balances (
      chain TEXT NOT NULL,
      address_index INTEGER NOT NULL,
      token TEXT NOT NULL DEFAULT 'NATIVE',
      balance TEXT NOT NULL DEFAULT '0',
      last_scanned TEXT,
      PRIMARY KEY (chain, address_index, token)
  );
  CREATE INDEX idx_balances_nonzero ON balances(chain, token) WHERE balance != '0';

  CREATE TABLE IF NOT EXISTS scan_state (
      chain TEXT PRIMARY KEY,
      last_scanned_index INTEGER NOT NULL DEFAULT 0,
      max_scan_id INTEGER NOT NULL DEFAULT 0,
      status TEXT NOT NULL DEFAULT 'idle',
      started_at TEXT,
      updated_at TEXT
  );

  CREATE TABLE IF NOT EXISTS transactions (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      chain TEXT NOT NULL,
      address_index INTEGER NOT NULL,
      tx_hash TEXT NOT NULL,
      direction TEXT NOT NULL,
      token TEXT NOT NULL DEFAULT 'NATIVE',
      amount TEXT NOT NULL,
      from_address TEXT NOT NULL,
      to_address TEXT NOT NULL,
      block_number INTEGER,
      status TEXT NOT NULL DEFAULT 'pending',
      created_at TEXT NOT NULL DEFAULT (datetime('now')),
      confirmed_at TEXT
  );
  CREATE INDEX idx_transactions_chain ON transactions(chain);
  CREATE INDEX idx_transactions_hash ON transactions(tx_hash);

  CREATE TABLE IF NOT EXISTS settings (
      key TEXT PRIMARY KEY,
      value TEXT NOT NULL,
      updated_at TEXT NOT NULL DEFAULT (datetime('now'))
  );

  CREATE TABLE IF NOT EXISTS schema_migrations (
      version INTEGER PRIMARY KEY,
      applied_at TEXT NOT NULL DEFAULT (datetime('now'))
  );
  ```

**Verification:**
- DB file created at configured path
- WAL mode enabled (`PRAGMA journal_mode` returns `wal`)
- All tables exist after migration
- Running migrations twice is idempotent
- Unit tests for DB open, migrate, close

## Task 5: Chi Router & Security Middleware

Set up the HTTP router with CORS, CSRF, host validation, and request logging.

**Files:**
- `internal/api/router.go`:
  - `NewRouter(db *DB, cfg *Config) chi.Router`
  - Mount middleware: logging, host check, CORS, CSRF
  - Mount `/api/health` endpoint
  - Mount static file server (placeholder for now)
- `internal/api/middleware/security.go`:
  - `HostCheckMiddleware` — reject non-localhost Host headers
  - `CORSMiddleware` — allow only `http://localhost:*` and `http://127.0.0.1:*`
  - `CSRFMiddleware` — generate token on GET, validate on POST/PUT/DELETE via `X-CSRF-Token` header matching `csrf_token` cookie
- `internal/api/middleware/logging.go`:
  - Request/response logging middleware (method, path, status, duration, remote addr)
- `internal/api/handlers/health.go`:
  - `GET /api/health` — returns `{"status":"ok","version":"...","network":"...","dbPath":"..."}`

**Verification:**
- `curl http://localhost:8080/api/health` returns 200 with JSON
- Request with non-localhost Host header returns 403
- POST without CSRF token returns 403
- GET request sets csrf_token cookie
- Request logging appears in stdout and log file

## Task 6: Server Entry Point (serve command)

Wire up the `serve` subcommand in `main.go`.

**Files:**
- `cmd/server/main.go`:
  - Parse subcommand (`init`, `serve`, `export`, `version`)
  - `serve`: load config via envconfig → setup logger → open DB → run migrations → create router → start HTTP server on `127.0.0.1:PORT`
  - Graceful shutdown on SIGINT/SIGTERM
  - Version string via ldflags

**Verification:**
- `go run ./cmd/server serve` starts server on :8080
- Ctrl+C gracefully shuts down
- All startup actions logged (port, network, DB path)

## Task 7: SvelteKit Scaffold

Create the SvelteKit frontend with adapter-static, TypeScript strict, Tailwind, and shadcn-svelte.

**Files:**
- `web/` — SvelteKit project:
  - `package.json` with dependencies: `@sveltejs/adapter-static`, `tailwindcss`, `shadcn-svelte`, `@tanstack/svelte-virtual`, `echarts`
  - `svelte.config.js` — adapter-static
  - `tsconfig.json` — strict mode, no `any`
  - `tailwind.config.js` — dark theme, custom colors from design tokens
  - `src/app.css` — Tailwind imports + CSS custom properties from mockup tokens
  - `src/lib/constants.ts` — ALL frontend constants (copy from CLAUDE.md spec)
  - `src/lib/types.ts` — core TypeScript interfaces (Chain, Token, AddressBalance, ScanStatus, etc.)
  - `src/lib/utils/api.ts` — API client (single source of truth for all backend calls, with CSRF token handling)
  - `src/lib/utils/formatting.ts` — number/address/date formatting helpers
  - `src/routes/+layout.svelte` — app shell with sidebar
  - `src/routes/+page.svelte` — dashboard placeholder

**Verification:**
- `cd web && npm install && npm run build` succeeds
- `npm run dev` serves on :5173
- Sidebar navigation renders with all 6 items (Dashboard, Addresses, Scan, Send, Transactions, Settings)
- Dark theme matches mockup design tokens
- TypeScript strict mode enforced (no `any` allowed)

## Task 8: Sidebar Layout Component

Build the sidebar + main content layout matching the mockup exactly.

**Files:**
- `web/src/lib/components/layout/Sidebar.svelte` — sidebar with:
  - Brand section (HDPay logo/icon)
  - Navigation items with SVG icons (matching mockup icons)
  - Active state highlighting
  - System section (Settings)
  - Footer with network badge (mainnet/testnet)
- `web/src/lib/components/layout/Header.svelte` — page header with title + optional actions
- `web/src/routes/+layout.svelte` — integrates sidebar + main content area

**Reference mockup:** `.project/03-mockups/components/nav.html` and any screen's sidebar

**Verification:**
- Layout matches mockup: 240px sidebar, main content area
- Active nav item highlighted
- Network badge shows correct state
- All navigation links present and routed

## Task 9: Makefile & Dev Tooling

Create build tooling for development workflow.

**Files:**
- `Makefile`:
  - `make dev` — run Go backend with hot reload (or just `go run`)
  - `make dev-frontend` — run SvelteKit dev server
  - `make build` — build frontend + embed in Go binary
  - `make test` — run all Go tests
  - `make test-frontend` — run Vitest
  - `make lint` — go vet + frontend lint
- `.gitignore` — Go binary, node_modules, .env, data/, logs/, web/build/

**Verification:**
- `make build` produces working `./hdpay` binary
- `make test` runs Go tests
- `.gitignore` correctly excludes build artifacts

</tasks>

<success_criteria>
- [ ] `go build ./...` compiles without errors
- [ ] `go run ./cmd/server serve` starts HTTP server on 127.0.0.1:8080
- [ ] `curl localhost:8080/api/health` returns JSON with status "ok"
- [ ] Security middleware rejects non-localhost requests and CSRF-less mutations
- [ ] SQLite database created with all tables from migration
- [ ] Structured logs output to stdout and daily log file
- [ ] SvelteKit dev server runs with Tailwind + dark theme
- [ ] Sidebar layout matches mockup design (240px, icons, active states, network badge)
- [ ] Frontend TypeScript strict mode enforced (zero `any`)
- [ ] All constants in central files (Go: config/constants.go, TS: lib/constants.ts)
- [ ] All errors in central file (Go: config/errors.go)
</success_criteria>

<verification>
1. Start backend: `go run ./cmd/server serve` → logs show server start
2. Health check: `curl -v localhost:8080/api/health` → 200 OK with JSON
3. CSRF test: `curl -X POST localhost:8080/api/health` → 403 (wrong method, but demonstrates middleware)
4. Host test: `curl -H "Host: evil.com" localhost:8080/api/health` → 403
5. Start frontend: `cd web && npm run dev` → page loads with sidebar
6. Check DB: `sqlite3 ./data/hdpay.sqlite ".tables"` → shows all tables
7. Check logs: `ls ./logs/` → shows today's log file
8. Build: `make build` → produces binary
</verification>
