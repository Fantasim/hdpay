# Changelog

## [Unreleased]

### 2026-02-20

#### Added (Phase 2: Core Services)
- Tier configuration system: LoadTiers, ValidateTiers, CreateDefaultTiers, LoadOrCreateTiers — auto-creates default 9-tier config if missing (`internal/poller/points/tiers.go`)
- Points calculator: USD→tier lookup→cents×multiplier→points, thread-safe with RWMutex for hot-reload from dashboard (`internal/poller/points/calculator.go`)
- Price service wrapper: thin layer over HDPay's CoinGecko PriceService with stablecoin short-circuit ($1.00) and retry logic (3 attempts, 5s delay) (`internal/poller/points/pricer.go`)
- Address validation: BTC (btcutil.DecodeAddress + IsForNet), BSC (0x + 40 hex regex), SOL (base58 decode to 32 bytes) with network-aware BTC validation (`internal/poller/validate/address.go`)
- Core services tests: all 12 tier boundary values, validation edge cases, price retry/cancel/stablecoin, known testnet address vectors (points 93.7%, validate 100.0% coverage)

#### Added (Phase 1: Foundation)
- Poller config struct with envconfig (`POLLER_*` prefix), validation for required fields and value ranges (`internal/poller/config/config.go`)
- Poller-specific constants: polling intervals, confirmation thresholds, watch defaults, session config, tiers, recovery, pagination (`internal/poller/config/constants.go`)
- Poller-specific error codes: 19 error constants for watch/tx/auth/system errors (`internal/poller/config/errors.go`)
- Domain models: Watch, Transaction, PointsAccount, SystemError, IPAllowEntry, Tier structs with filters and pagination (`internal/poller/models/types.go`)
- SQLite database layer with WAL mode, busy_timeout, migration runner using embed.FS (`internal/poller/pollerdb/db.go`)
- Database schema: 5 tables (watches, points, transactions, ip_allowlist, system_errors) + 12 indexes (`internal/poller/pollerdb/migrations/001_init.sql`)
- Watch CRUD: CreateWatch, GetWatch, ListWatches, UpdateWatchStatus, UpdateWatchPollResult, ExpireAllActiveWatches, GetActiveWatchByAddress (`internal/poller/pollerdb/watches.go`)
- Points CRUD: GetOrCreatePoints, AddUnclaimed, AddPending, MovePendingToUnclaimed, ClaimPoints, ListWithUnclaimed, ListWithPending (`internal/poller/pollerdb/points.go`)
- Transaction CRUD: InsertTransaction, GetByTxHash, UpdateToConfirmed, ListPending, ListByAddress, ListAll, LastDetectedAt (`internal/poller/pollerdb/transactions.go`)
- IP allowlist CRUD: ListAllowedIPs, AddIP, RemoveIP, IsIPAllowed, LoadAllIPsIntoMap (`internal/poller/pollerdb/allowlist.go`)
- System errors CRUD: InsertError, ListUnresolved, MarkResolved, ListByCategory (`internal/poller/pollerdb/errors.go`)
- Main entry point: config loading, logging init, DB setup, Chi server with health endpoint, graceful shutdown (`cmd/poller/main.go`)
- Environment variable template (`.env.poller.example`)
- Makefile targets: `dev-poller`, `build-poller`, `test-poller`
- Foundation tests: config validation (82.6% coverage), DB migration + all CRUD (76.9% coverage)

#### Changed (Phase 1: Foundation)
- HDPay logging parameterized: added `SetupWithPrefix()` for configurable log file prefix, `CleanOldLogs()` accepts prefix param (`internal/logging/logger.go`)
- Makefile updated with Poller build/dev/test targets

#### Added (Planning)
- CFramework project structure initialized
- V1 feature plan: 35 must-have, 3 should-have, 5 nice-to-have features
- Custom documentation (DESCRIPTION.md, PROMPT.md) for project specification
- V1 build plan: 8 phases from foundation to deployment
- Detailed phase plans for phases 1-3 (Foundation, Core Services, Blockchain Providers)
- Outline phase plans for phases 4-8 (Watch Engine, API, Frontend, Dashboard, Embedding)
- Feature-to-phase mapping for all 38 features
- HDPay code reuse strategy: import logging, config, models, price, scanner packages
- Session log infrastructure

#### Changed
- Project structure: Poller as `cmd/poller/` + `internal/poller/` inside HDPay's Go module
- Database schema: added `block_number` column to transactions table for BSC confirmation
- API design: login and health endpoints exempt from IP allowlist
- Watch defaults: runtime-only (no settings table, lost on restart)
