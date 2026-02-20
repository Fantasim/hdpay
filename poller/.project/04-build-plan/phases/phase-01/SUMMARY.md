# Phase 1 Summary: Foundation

## Completed: 2026-02-20

## What Was Built
- Poller project scaffold inside HDPay's Go module (`cmd/poller/` + `internal/poller/`)
- Poller-specific config struct with envconfig (POLLER_* prefix), validation
- Poller-specific constants (polling intervals, confirmations, tiers, sessions, recovery, pagination)
- Poller-specific error codes (19 error constants)
- Domain models (Watch, Transaction, PointsAccount, SystemError, IPAllowEntry, Tier, filters, pagination)
- SQLite database layer with WAL mode, migration runner (embed.FS)
- Schema: 5 tables (watches, points, transactions, ip_allowlist, system_errors) + 12 indexes
- Full CRUD for all 5 tables
- Main entry point with Chi server, health endpoint, graceful shutdown
- Parameterized HDPay's logging.Setup() to support Poller's `poller-*` log file prefix
- Makefile targets for Poller (dev-poller, build-poller, test-poller)
- Foundation tests with >70% coverage on all packages

## Files Created
- `cmd/poller/main.go` — Entry point (config, logging, DB, Chi server, graceful shutdown)
- `internal/poller/config/config.go` — Config struct with envconfig tags + validation
- `internal/poller/config/config_test.go` — Config loading and validation tests
- `internal/poller/config/constants.go` — All Poller-specific constants
- `internal/poller/config/errors.go` — All Poller-specific error codes
- `internal/poller/models/types.go` — Domain types (Watch, Transaction, PointsAccount, etc.)
- `internal/poller/pollerdb/db.go` — DB connection, migration runner
- `internal/poller/pollerdb/migrations/001_init.sql` — Schema (5 tables + indexes)
- `internal/poller/pollerdb/watches.go` — Watch CRUD (7 methods)
- `internal/poller/pollerdb/watches_test.go` — Watch tests (7 test functions)
- `internal/poller/pollerdb/points.go` — Points CRUD (7 methods)
- `internal/poller/pollerdb/points_test.go` — Points tests (11 test functions)
- `internal/poller/pollerdb/transactions.go` — Transaction CRUD (7 methods)
- `internal/poller/pollerdb/transactions_test.go` — Transaction tests (8 test functions)
- `internal/poller/pollerdb/allowlist.go` — IP allowlist CRUD (5 methods)
- `internal/poller/pollerdb/allowlist_test.go` — Allowlist tests (6 test functions)
- `internal/poller/pollerdb/errors.go` — System errors CRUD (4 methods)
- `internal/poller/pollerdb/errors_test.go` — Errors tests (5 test functions)
- `.env.poller.example` — Template with all POLLER_* environment variables

## Files Modified
- `internal/logging/logger.go` — Added SetupWithPrefix() for configurable log file prefix
- `internal/logging/logger_test.go` — Updated CleanOldLogs calls to pass prefix
- `Makefile` — Added Poller targets (dev-poller, build-poller, test-poller)

## Decisions Made
- **Logging parameterization**: Added `SetupWithPrefix()` that accepts file pattern and clean prefix. Original `Setup()` delegates to it for backward compatibility.
- **Package name `pollerdb`**: Avoids collision with HDPay's `db` package. Clean separation.
- **Migration runner**: Uses `embed.FS` to embed SQL files. Tracks version in `schema_version` table.
- **Test strategy**: Each test creates its own temp SQLite file via `newTestDB(t)`. No shared state between tests.
- **Build output**: Uses `bin/poller` via Makefile to avoid conflict with `poller/` directory at repo root.

## Deviations from Plan
- Plan said "accept the HDPay prefix" for logging, but we parameterized it instead (better separation)
- Added Makefile targets instead of raw `go build` commands (user preference)
- `.env.example` placed at repo root as `.env.poller.example` (not in poller/ subdirectory)

## Issues Encountered
- **go build conflict**: `go build ./cmd/poller/` failed because `poller/` directory exists at repo root. Solved via Makefile with `-o bin/poller`.
- **go not in PATH**: `/usr/local/go/bin/go` — Makefile already handles this with `GO := PATH=$(PATH):/usr/local/go/bin go`.
- **Test ordering**: SQLite `DATETIME('now')` can produce same timestamp for rapid inserts, causing ORDER BY assumptions to fail. Fixed tests to check set membership instead of ordering.

## Test Coverage
- `internal/poller/config`: 82.6%
- `internal/poller/pollerdb`: 76.9%
- Both above the 70% target

## Notes for Next Phase
- Phase 2 (Core Services) will add: PriceService integration, TierCalculator, PointsCalculator, ConfirmationChecker
- All CRUD methods are ready for Phase 2 to use
- HDPay imports work correctly (logging, config, models)
- The `LastDetectedAt()` method on transactions is ready for the watch engine (Phase 4)
