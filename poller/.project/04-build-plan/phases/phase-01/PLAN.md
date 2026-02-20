# Phase 1: Foundation

<objective>
Scaffold Poller inside HDPay's Go module (`cmd/poller/` + `internal/poller/`), set up Poller-specific configuration, SQLite database with migrations, and domain models. Reuse HDPay's logging and shared types. By the end, `go run cmd/poller/main.go` compiles and starts an HTTP server that connects to SQLite and logs to stdout + split log files.
</objective>

<features>
F16 — SQLite Database (5 tables + block_number column, WAL mode, migrations)
F19 — Config & Validation (envconfig struct, required fields)
F20 — Logging (reuse HDPay's slog setup — split by level, daily rotation)
</features>

<tasks>

## Task 1: Project Scaffold & Config

**Create** the Poller directory structure inside HDPay's module.

**Files to create:**
- `cmd/poller/main.go` — entry point (minimal, calls internal/poller packages)
- `internal/poller/config/config.go` — Poller Config struct with envconfig tags (POLLER_* prefix)
- `internal/poller/config/constants.go` — Poller-specific constants (polling intervals, session config, tiers config, recovery, pagination, etc.)
- `internal/poller/config/errors.go` — Poller-specific error code constants
- `internal/poller/models/types.go` — Poller domain types

**What to import from HDPay (not rewrite):**
- `internal/config` — shared constants (provider URLs, rate limits, token contracts, decimals, explorer URLs)
- `internal/config` — shared sentinel errors (`ErrProviderRateLimit`, `ErrProviderUnavailable`, `TransientError`)
- `internal/models` — shared types (`Chain`, `Token`, `NetworkMode`, `ChainBTC`, `ChainBSC`, `ChainSOL`)
- `internal/logging` — `Setup()` function for slog initialization

**Poller Config struct** (POLLER_* env vars from PROMPT.md):
```go
type Config struct {
    DBPath              string `envconfig:"POLLER_DB_PATH" default:"./data/poller.sqlite"`
    Port                int    `envconfig:"POLLER_PORT" default:"8081"`
    LogLevel            string `envconfig:"POLLER_LOG_LEVEL" default:"info"`
    LogDir              string `envconfig:"POLLER_LOG_DIR" default:"./logs"`
    Network             string `envconfig:"POLLER_NETWORK" default:"mainnet"`
    StartDate           int64  `envconfig:"POLLER_START_DATE" required:"true"`
    AdminUsername       string `envconfig:"POLLER_ADMIN_USERNAME" required:"true"`
    AdminPassword       string `envconfig:"POLLER_ADMIN_PASSWORD" required:"true"`
    BscScanAPIKey       string `envconfig:"POLLER_BSCSCAN_API_KEY"`
    HeliusAPIKey        string `envconfig:"POLLER_HELIUS_API_KEY"`
    MaxActiveWatches    int    `envconfig:"POLLER_MAX_ACTIVE_WATCHES" default:"100"`
    DefaultWatchTimeout int    `envconfig:"POLLER_DEFAULT_WATCH_TIMEOUT_MIN" default:"30"`
    TiersFile           string `envconfig:"POLLER_TIERS_FILE" default:"./tiers.json"`
}
```

**Poller-specific constants** (from PROMPT.md, NOT already in HDPay):
- Polling intervals: `PollIntervalBTC=60s`, `PollIntervalBSC=5s`, `PollIntervalSOL=5s`
- Confirmation thresholds: `ConfirmationsBTC=1`, `ConfirmationsBSC=12`, `SOLCommitment="finalized"`
- Watch defaults: `DefaultWatchTimeoutMinutes=30`, `MaxWatchTimeoutMinutes=120`, `MaxActiveWatches=100`
- Session: `SessionCookieName="poller_session"`, `SessionTimeout=1h`, `SessionTokenLength=32`
- Database: `PollerDBPath="./data/poller.sqlite"`, `PollerDBTestPath="./data/poller_test.sqlite"`
- Price: `PriceRetryCount=3`, `PriceRetryDelay=5s`, `StablecoinPrice=1.0`
- CoinGecko IDs: `CoinGeckoIDBTC="bitcoin"`, etc.
- Tiers: `TiersConfigFile="./tiers.json"`, `MinTierCount=2`
- Recovery: `RecoveryPendingRetries=3`, `RecoveryPendingInterval=30s`, `StalePendingThreshold=24h`
- Shutdown: `ShutdownTimeout=10s`
- Pagination: `DefaultPageSize=50`, `MaxPageSize=200`
- Server: `PollerServerPort=8081`

**Poller-specific error codes** (from PROMPT.md, NOT already in HDPay):
- `ERROR_ALREADY_WATCHING`, `ERROR_WATCH_NOT_FOUND`, `ERROR_WATCH_EXPIRED`, `ERROR_ADDRESS_NOT_FOUND`, `ERROR_ADDRESS_INVALID`, `ERROR_INVALID_TIMEOUT`, `ERROR_MAX_WATCHES`, `ERROR_TX_ALREADY_RECORDED`, `ERROR_NOTHING_TO_CLAIM`, `ERROR_TIERS_INVALID`, `ERROR_TIERS_FILE`, `ERROR_UNAUTHORIZED`, `ERROR_FORBIDDEN`, `ERROR_SESSION_EXPIRED`, `ERROR_IP_NOT_ALLOWED`, `ERROR_INVALID_CREDENTIALS`, `ERROR_DISCREPANCY`, `ERROR_INVALID_REQUEST`, `ERROR_INTERNAL`

**Poller domain types** (`internal/poller/models/types.go`):
- `WatchStatus` (ACTIVE, COMPLETED, EXPIRED, CANCELLED)
- `TxStatus` (PENDING, CONFIRMED)
- `Watch` struct (id, chain, address, status, started_at, expires_at, completed_at, poll_count, last_poll_at, last_poll_result)
- `Transaction` struct (id, watch_id, tx_hash, chain, address, token, amount_raw, amount_human, decimals, usd_value, usd_price, tier, multiplier, points, status, confirmations, block_number, detected_at, confirmed_at)
- `PointsAccount` struct (address, chain, unclaimed, pending, total, updated_at)
- `SystemError` struct (id, severity, category, message, details, resolved, created_at)
- `IPAllowEntry` struct (id, ip, description, added_at)
- `Tier` struct (MinUSD, MaxUSD *float64, Multiplier)

**Verification:**
- `go build ./cmd/poller/` succeeds
- All Poller-specific constants from PROMPT.md present
- All Poller-specific error codes from PROMPT.md present
- HDPay imports resolve correctly (shared Chain/Token/etc.)

## Task 2: Logging (Reuse HDPay)

**Wire** HDPay's logging package into Poller's main.go.

**No new files needed** — import `internal/logging` and call `logging.Setup(cfg.LogLevel, cfg.LogDir)`.

**Details:**
- HDPay's `logging.Setup()` already handles: stdout + per-level daily rotated files, 30-day retention
- Log file naming uses HDPay pattern: `hdpay-{level}-{YYYY-MM-DD}.log`
- If Poller needs a different prefix (e.g., `poller-*`), parameterize `Setup()` or accept the HDPay prefix
- No new logging code to write

**Verification:**
- Log messages at different levels appear in correct split files
- Stdout shows human-readable output

## Task 3: SQLite Database & Migrations

**Create** Poller's database layer with its own tables.

**Files to create:**
- `internal/poller/pollerdb/db.go` — NewDB(), connection setup (reuse HDPay's WAL/busy_timeout pattern)
- `internal/poller/pollerdb/migrations/001_init.sql` — All 5 tables + indexes from PROMPT.md schema (with `block_number` column added to transactions)
- `internal/poller/pollerdb/watches.go` — Watch CRUD
- `internal/poller/pollerdb/points.go` — Points ledger CRUD
- `internal/poller/pollerdb/transactions.go` — Transaction CRUD
- `internal/poller/pollerdb/allowlist.go` — IP allowlist CRUD
- `internal/poller/pollerdb/errors.go` — System errors CRUD

**Schema changes from PROMPT.md:**
- Add `block_number INTEGER` column to `transactions` table (needed for BSC confirmation counting)

**DB methods (same as before):**
- `watches.go`: `CreateWatch`, `GetWatch`, `ListWatches(filters)`, `UpdateWatchStatus`, `UpdateWatchPollResult`, `ExpireAllActiveWatches`, `GetActiveWatchByAddress`
- `points.go`: `GetOrCreatePoints`, `AddUnclaimed`, `AddPending`, `MovePendingToUnclaimed`, `ClaimPoints(addresses)`, `ListWithUnclaimed`, `ListWithPending`
- `transactions.go`: `InsertTransaction`, `GetByTxHash`, `UpdateToConfirmed`, `ListPending`, `ListByAddress`, `ListAll(filters, pagination)`
- `allowlist.go`: `ListAllowedIPs`, `AddIP`, `RemoveIP(id)`, `IsIPAllowed`, `LoadAllIntoCache`
- `errors.go`: `InsertError`, `ListUnresolved`, `MarkResolved`, `ListByCategory`

**Verification:**
- Database file created at configured path
- All 5 tables exist with correct schema (including block_number)
- schema_version table tracks applied migrations
- CRUD operations work correctly

## Task 4: Main Entry Point

**Create** the main.go that ties everything together.

**Files to create/modify:**
- `cmd/poller/main.go` — Load config, init logger (via HDPay's logging.Setup), open DB, run migrations, start placeholder HTTP server
- `.env.example` (in poller directory or root) — Template with all POLLER_* variables

**Details:**
- Startup sequence (partial — phases 2-5 will complete it):
  1. Load .env via envconfig → validate required fields
  2. Initialize slog via `logging.Setup(cfg.LogLevel, cfg.LogDir)`
  3. Open/create Poller SQLite database → run migrations
  4. Start a basic Chi HTTP server on POLLER_PORT with just a health endpoint
  5. Listen for SIGTERM/SIGINT → graceful shutdown (close DB)
- Log startup banner with version, network mode, port, DB path
- Import Chi from HDPay's existing dependency (already in go.mod)

**Verification:**
- `go run cmd/poller/main.go` starts with a valid .env file
- Health endpoint returns 200
- SQLite database file is created
- Log files appear in configured directory
- Ctrl+C shuts down gracefully

## Task 5: Foundation Tests

**Create** tests for Poller config and DB operations.

**Files to create:**
- `internal/poller/config/config_test.go` — Test config loading, required field validation
- `internal/poller/pollerdb/db_test.go` — Test migration runner, DB setup
- `internal/poller/pollerdb/watches_test.go` — Test Watch CRUD
- `internal/poller/pollerdb/points_test.go` — Test Points CRUD (including claim edge cases)
- `internal/poller/pollerdb/transactions_test.go` — Test Transaction CRUD

**Details:**
- Use test SQLite DB (in-memory or temp file)
- Test all CRUD operations for each table
- Test migration idempotency
- Test config validation (missing required fields)
- Points claim tests: claim while pending exists, claim for unknown address (skip silently), new funds after claim

**Verification:**
- `go test ./internal/poller/...` passes
- Coverage > 70% on pollerdb package

</tasks>

<success_criteria>
- `go build ./cmd/poller/` compiles without errors (within HDPay's module)
- `go run cmd/poller/main.go` starts, creates DB, logs to stdout + files, serves health endpoint
- All 5 DB tables created with correct schema (including block_number on transactions)
- HDPay imports work: logging.Setup(), config constants, models.Chain/Token
- `go test ./internal/poller/...` passes with > 70% coverage on pollerdb
- Poller-specific constants contain ALL values from PROMPT.md
- Poller-specific error codes contain ALL codes from PROMPT.md
</success_criteria>

<research_needed>
- Check if HDPay's logging.Setup() needs parameterization for log file prefix (currently "hdpay-*")
- Verify HDPay's go.mod already has all dependencies Poller needs (Chi, modernc.org/sqlite, envconfig)
</research_needed>
