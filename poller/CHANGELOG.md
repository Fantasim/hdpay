# Changelog

## [Unreleased]

### 2026-02-20

#### Added (Phase 6: Frontend Setup & Auth)
- SvelteKit project scaffolded at `web/poller/` with adapter-static (SPA mode, `fallback: 'index.html'`)
- Tailwind CSS v4 via `@tailwindcss/vite` plugin, Vite dev proxy `/api` → `localhost:8081`
- shadcn-svelte initialized (Slate base, dark theme): button, card, input, label, badge, separator components
- Design system ported from mockup tokens into `app.css` — dark-only theme, Linear-inspired palette (`web/poller/src/app.css`)
- TypeScript types: 25+ interfaces matching all backend models exactly (`web/poller/src/lib/types.ts`)
- Constants file: chains, colors, statuses, error codes, nav items, explorer URLs (`web/poller/src/lib/constants.ts`)
- API client: fetch wrapper with auto 401→/login redirect, 20 endpoint functions (`web/poller/src/lib/utils/api.ts`)
- Auth store: login/logout/checkSession with Svelte writable stores (`web/poller/src/lib/stores/auth.ts`)
- Login page matching mockup: centered card, brand icon, error display, network badge (`web/poller/src/routes/login/+page.svelte`)
- Sidebar component: 240px fixed, 6 nav items with inline SVG icons, active state, logout, network badge (`web/poller/src/lib/components/layout/Sidebar.svelte`)
- Header component: title, optional subtitle, actions snippet slot (`web/poller/src/lib/components/layout/Header.svelte`)
- Root layout with auth gating: session check, redirect to /login, loading spinner, ModeWatcher (`web/poller/src/routes/+layout.svelte`)
- Formatting utilities: truncateAddress, formatUsd, formatPoints, formatDate, formatRelativeTime, copyToClipboard (`web/poller/src/lib/utils/formatting.ts`)
- 6 stub route pages: overview, transactions, watches, points, errors, settings

#### Added (Phase 5: API Layer)
- Chi v5 router with Dependencies struct, layered middleware, route groups (`internal/poller/api/router.go`)
- JSON response helpers: envelope, paginated list, error response (`internal/poller/httputil/response.go`)
- IP allowlist middleware: cache with DB refresh, private IP bypass (`internal/poller/api/middleware/ipallow.go`)
- Session auth middleware: bcrypt, crypto/rand tokens, HttpOnly cookies, 1h expiry (`internal/poller/api/middleware/session.go`)
- CORS middleware: Allow-Origin *, preflight 204 (`internal/poller/api/middleware/cors.go`)
- 17 API endpoints: health, auth (login/logout), watch (create/cancel/list), points (get/pending/claim), admin (allowlist CRUD, settings, tiers, watch-defaults), dashboard (stats/transactions/charts/errors) (`internal/poller/api/handlers/`)
- Dashboard aggregate queries: stats, daily, charts by chain/token/tier (`internal/poller/pollerdb/dashboard.go`)
- 4 discrepancy check methods: points mismatch, unclaimed>total, orphaned txs, stale pending (`internal/poller/pollerdb/discrepancy.go`)
- API tests: 40 new tests (21 handler integration + 19 middleware unit)

#### Added (Phase 4: Watch Engine)
- Watcher orchestrator: central `Watcher` struct managing one goroutine per active watch, with `sync.WaitGroup` for graceful shutdown (`internal/poller/watcher/watcher.go`)
- Watch creation: validate chain/address, duplicate rejection (`ERROR_ALREADY_WATCHING`), max watch limit (`ERROR_MAX_WATCHES`), UUID generation, context.Background() with timeout per watch
- Watch cancellation: cancel context triggers goroutine cleanup, DB status update to CANCELLED
- Smart cutoff resolution: `MAX(last recorded tx detected_at, POLLER_START_DATE)` — subsequent watches skip already-seen transactions
- Poll loop goroutine: per-chain ticker intervals (BTC:60s, BSC:5s, SOL:5s), fetch new txs → process → recheck pending → check stop conditions
- Transaction processing pipeline: dedup by `tx_hash`, CONFIRMED tx: fetch price + calculate points + add to unclaimed ledger; PENDING tx: insert + estimate pending points
- Confirmation tracking: re-check PENDING txs each tick via `ProviderSet.ExecuteConfirmation()`, on confirm: fetch price, calculate points, `MovePendingToUnclaimed()`
- Watch stop conditions: EXPIRED (timeout), COMPLETED (all txs confirmed + ≥1 tx), CANCELLED (manual)
- Startup recovery: expire all stale ACTIVE watches, re-check orphaned PENDING txs with 3 retries at 30s intervals, log system error if unresolvable (`internal/poller/watcher/recovery.go`)
- Runtime-mutable settings: `MaxActiveWatches` and `DefaultWatchTimeout` with thread-safe getters/setters
- New DB methods: `ListPendingByWatchID()`, `CountByWatchID()` for per-watch tx queries (`internal/poller/pollerdb/transactions.go`)
- New constants: `WatchContextGracePeriod` (5s), `RecoveryTimeout` (5m) (`internal/poller/config/constants.go`)
- main.go integration: full startup sequence (config → logging → DB → tiers → PriceService → Pricer → Calculator → Providers → Watcher → Recovery → HTTP), graceful shutdown (watcher.Stop() before HTTP shutdown)
- Provider initialization: `initProviderSets()` creates BTC (Blockstream+Mempool), BSC (BscScan), SOL (SolanaRPC+Helius) sets with network-aware URLs
- Watcher tests: 31 tests covering lifecycle, creation, cancellation, dedup (incl. SOL composite), cutoff resolution, stop conditions, recovery, concurrent watches, graceful shutdown — 71.0% coverage, 0 race conditions (`internal/poller/watcher/watcher_test.go`)

#### Added (Phase 3: Blockchain Providers)
- Provider interface for transaction detection: `Provider` interface with `FetchTransactions`, `CheckConfirmation`, `GetCurrentBlock` (`internal/poller/provider/provider.go`)
- `RawTransaction` struct: TxHash, Token, AmountRaw, AmountHuman, Decimals, BlockTime, Confirmed, Confirmations, BlockNumber
- `ProviderSet`: thread-safe round-robin rotation with HDPay's `RateLimiter` + `CircuitBreaker`, per-provider rate limiting, circuit breaker trip/recovery (`internal/poller/provider/provider.go`)
- `NewHTTPClient`: configured HTTP client using HDPay's connection pool constants
- BTC provider: `BlockstreamProvider` + `MempoolProvider` (embedded, shared Esplora API format), pagination at 25/page, multi-output aggregation, satoshi→human conversion (`internal/poller/provider/btc.go`)
- BSC provider: `BscScanProvider` with normal txlist (BNB) + tokentx (USDC/USDT by contract address), network-aware contract resolution, block-number-based confirmation counting via `eth_blockNumber` proxy, weiToHuman converter (`internal/poller/provider/bsc.go`)
- SOL provider: `SolanaRPCProvider` via JSON-RPC (`getSignaturesForAddress` + `getTransaction`), native SOL detection via pre/postBalances delta, SPL token detection via pre/postTokenBalances with owner matching, composite tx_hash format (`signature:TOKEN`), `NewHeliusProvider` factory, lamportsToHuman converter (`internal/poller/provider/sol.go`)
- Provider tests: 49 tests covering round-robin rotation, circuit breaker integration, BTC/BSC/SOL response parsing, error handling (HTTP 429/500, malformed JSON, RPC errors), cutoff filtering, pagination, composite tx_hash — 82.5% coverage (`internal/poller/provider/*_test.go`)
- Provider error constants: `ErrorCategoryProvider`, `ErrorCategoryWatcher`, `ErrorSeverityWarn/Error/Critical` (`internal/poller/config/constants.go`)

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
