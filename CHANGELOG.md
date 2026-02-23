# Changelog

## Provider Audit & Expansion — 2026-02-23

#### Added
- **BTC**: Bitaps as 3rd provider in wallet scanner and poller (free, no-key, Esplora-like REST API)
- **BSC scanner**: LlamaNodes, dRPC, NodeReal MegaNode, and 4 additional BNB Chain dataseed nodes added to round-robin pool (up from 2 to 8 total RPC endpoints)
- **BSC scanner**: NodeReal BSCTrace optional batch provider (`HDPAY_NODEREAL_API_KEY`) — restores 20-address batch balance queries lost when BscScan shut down Dec 18, 2025 (`internal/shared/scanner/bsc_nodereal.go`)
- **SOL scanner**: Ankr, dRPC, OnFinality added as no-key providers (up from 2 to 5); Alchemy support added as optional key-based provider (`HDPAY_ALCHEMY_API_KEY`)
- **SOL poller**: Ankr, dRPC, OnFinality, Alchemy added to `initProviderSets` in `cmd/poller/main.go`
- **BSC poller**: Expanded fallback URL list from 2 to 8 endpoints for `BSCRPCPollerProvider` initialization
- New API key config fields: `NodeRealAPIKey`, `AlchemyAPIKey`, `BlockCypherAPIKey` in both wallet and poller config
- Updated `.env.wallet.example` and `.env.poller.example` with all optional API keys, inline documentation, and removed deprecated `BSCSCAN_API_KEY`

#### Changed
- `NewBSCRPCProvider` signature changed from `(rl, network)` to `(rl, name, rpcURL)` to support multi-instance rotation with named providers
- Fixed BSC poller using wrong `RateLimitSolanaRPC` constant; now correctly uses `RateLimitBSCRPC`
- `setup.go` now loops over all BSC RPC URLs, skipping any that fail to connect
- `bsc_rpc_test.go` updated to set `name: "BSCRPC"` on directly-constructed structs

#### Provider Count Summary

| Chain | Before | After |
|-------|--------|-------|
| BTC scanner | 2 | 3 |
| BTC poller | 2 | 3 |
| BSC scanner | 2 active | up to 9 (8 RPC + optional NodeReal batch) |
| BSC poller | 2 | 8 (failover chain) |
| SOL scanner | 2 | 5 no-key + up to 2 key-based |
| SOL poller | 1–2 | 4 no-key + up to 2 key-based |

---

## Poller Robustness Audit — 18 Fixes — 2026-02-23

#### Fixed (Critical Safety)
- **ClaimPoints TOCTOU race condition**: Wrapped SELECT+UPDATE in a single `BEGIN/COMMIT` transaction to prevent double-claiming under contention (`internal/poller/pollerdb/points.go`)
- **Crash between confirm+points-move loses points**: New `ConfirmTxAndMovePoints()` atomic method wraps both `UpdateToConfirmed` and `MovePendingToUnclaimed` in a single DB transaction (`internal/poller/pollerdb/transactions.go`, `internal/poller/watcher/poll.go`, `internal/poller/watcher/recovery.go`)
- **Pending points can go negative**: `MovePendingToUnclaimed` now uses `MAX(pending - ?, 0)` SQL floor (`internal/poller/pollerdb/points.go`)
- **No instance lock prevents dual-run corruption**: Added PID file lock with stale-PID detection at startup (`cmd/poller/main.go`)

#### Fixed (Optimization)
- **BSC/SOL poll intervals too aggressive (5s)**: Increased to 15s — 66% API load reduction with minimal latency impact (`internal/poller/config/constants.go`)
- **CountByWatchID makes 2 queries**: Merged into single query with conditional aggregation (`internal/poller/pollerdb/transactions.go`)
- **No local price cache in Pricer**: Added 60s TTL in-memory cache with `sync.RWMutex` to reduce CoinGecko API calls (`internal/poller/points/pricer.go`)
- **No limit on claim batch size**: Added `MaxClaimBatchSize=500` constant and validation (`internal/poller/api/handlers/points.go`, `internal/poller/config/constants.go`)
- **Hardcoded chain list in claim handler**: Replaced with `SupportedChains` constant (`internal/poller/config/constants.go`)

#### Fixed (Recovery)
- **Recovery doesn't fix negative pending balances**: Added `FixNegativePending()` recovery step at startup (`internal/poller/watcher/recovery.go`, `internal/poller/pollerdb/points.go`)
- **Price failure logging insufficient**: Improved log context when price fetch fails during recheck (`internal/poller/watcher/poll.go`)

#### Added
- `ConfirmTxAndMovePoints()` atomic DB method
- `FixNegativePending()` DB method + recovery step
- `SupportedChains` and `MaxClaimBatchSize` constants
- PID file instance lock (`acquirePIDLock`/`isProcessAlive`)
- Safety boundary documentation on `SetMaxOpenConns(1)` (`internal/poller/pollerdb/db.go`)
- 10 new tests: `TestMovePendingToUnclaimed_FloorsAtZero`, `TestFixNegativePending`, `TestConfirmTxAndMovePoints_Atomic`, `TestClaimPoints_Concurrent` (10 goroutines racing), `TestCountByWatchID_SingleQuery`, `TestConfirmTxAndMovePoints_AtomicWithWatch`, `TestClaimPoints_BatchLimit`, `TestClaimPoints_MultiChain`, `TestGetPendingPoints_Empty`

#### Changed
- `PollIntervalBSC` and `PollIntervalSOL` from 5s to 15s
- `recheckPending` in poll.go uses atomic `ConfirmTxAndMovePoints` instead of two separate DB calls
- Recovery startup uses `ConfirmTxAndMovePoints` and adds negative-pending fix step

## Provider Fixes + API Usage Counters — 2026-02-23

#### Fixed
- **BscScan API removed** — `api.bscscan.com` was shut down December 18, 2025. Replaced poller BSC provider (`BSCRPCPollerProvider`) with `eth_getLogs` (USDC/USDT Transfer events) + `eth_getBalance` delta (native BNB). Wallet scanner BSC pool now RPC-only (dead `BscScanProvider` removed from pool).
- **Wrong rate limit constants** — Blockstream corrected 10→5 req/s, Mempool 10→3 req/s (per nginx configs), CoinGecko 10→30 req/min (Demo plan = 30 RPM).

#### Added
- **ProviderMetrics** (`internal/shared/scanner/metrics.go`) — per-provider atomic counters with lazy-reset period buckets (daily/weekly/monthly). No background goroutine needed.
- **RateLimiter extended** — `NewRateLimiter(name, rps, knownMonthlyLimit)` now embeds `ProviderMetrics`. Added `RecordSuccess()`, `RecordFailure(is429 bool)`, `Stats()` methods.
- **Known monthly limit constants** — `KnownMonthlyLimitBlockstream` (500K), `KnownMonthlyLimitHelius` (1M), `KnownMonthlyLimitCoinGecko` (10K), etc. in `constants.go`.
- **Wallet `GET /api/health/providers`** — now merges DB circuit-breaker state with in-memory `MetricsSnapshot` per provider (`metrics` field in response).
- **Poller `GET /api/admin/provider-stats`** — new authenticated admin endpoint returning daily/weekly/monthly request counters grouped by chain.
- **Wallet scan page** — `ProviderStatus.svelte` extended with per-provider usage metrics row (today req, 429 count, monthly progress bar).
- **Poller settings page** — new "Provider Usage" section with table showing today/month request counts and monthly limit progress bar per provider.

#### Changed
- Poller `NewProviderSet` now accepts a `monthlyLimits []int64` 4th argument for metrics tracking.
- All provider test files updated for new `NewRateLimiter` (3-arg) and `NewProviderSet` (4-arg) signatures.
- Old `BscScanProvider` tests replaced with `BSCRPCPollerProvider` tests (constructor + `weiToHuman` + synthetic hash confirmation).

## Poller Playwright Audit + Fund Transfer Testing — 2026-02-23

#### Fixed
- **Nil-slice JSON serialization**: `ListWatchesHandler` and `GetAllowlistHandler` returned `{"data":null}` instead of `{"data":[]}` when empty, causing frontend crashes on iteration. Added nil-to-empty-slice conversion before JSON serialization (`internal/poller/api/handlers/watch.go`, `internal/poller/api/handlers/admin.go`)
- **SOL explorer URLs missing devnet suffix**: Testnet SOL transaction links pointed to mainnet Solscan (missing `?cluster=devnet`). Added `EXPLORER_TX_URL_TESTNET_SUFFIX` constant and updated `getTxExplorerUrl()` to append suffix (`web/poller/src/lib/constants.ts`, `web/poller/src/lib/utils/explorer.ts`)
- **Explorer URLs hardcoded to mainnet**: Transactions and errors pages used hardcoded `'mainnet'` network. Added `fetchNetwork()` to query `/api/health` for the actual network setting (`web/poller/src/routes/transactions/+page.svelte`, `web/poller/src/routes/errors/+page.svelte`)
- **Frontend null-safety on API responses**: Added `?? []` fallback on `.data` for watches, points, pending points, and allowlist API responses to prevent crashes when backend returns null arrays (`web/poller/src/routes/watches/+page.svelte`, `web/poller/src/routes/points/+page.svelte`, `web/poller/src/routes/settings/+page.svelte`)

#### Added
- 3 regression tests: `TestListWatches_EmptyReturnsArray`, `TestGetAllowlist_EmptyReturnsArray`, `TestHealth_ReturnsNetworkField` (`internal/poller/api/handlers/handlers_test.go`)

#### Changed
- **Wallet `init` command**: `--count` flag is now required (was defaulting to 500K), with validation for range 1 to 500,000 (`cmd/wallet/main.go`)
- **Makefile**: Renamed `build` to `build-wallet`, added `build-all` (wallet+poller+verify), `build-verify`, unified `clean` to remove `bin/` (`Makefile`)

#### Verified (End-to-End)
- SOL native sweep: 9.49 SOL from index 0 → index 1, confirmed on-chain (`4Q4a1pq...cTRX`)
- BTC sweep: 168,551 sats from index 0 → index 1, confirmed on-chain (`42bb190...2646`)
- Poller watch detection: Both SOL and BTC transfers detected by poller watches with correct points calculation
- All explorer links verified correct via Playwright (BTC: blockstream.info/testnet, SOL: solscan.io/?cluster=devnet)

## Poller Bug Fixes — 2026-02-23

#### Fixed
- Cookie `Secure: true` blocking login on HTTP localhost (cookie was silently discarded by browsers, causing all authenticated pages to return 401)
- `SameSiteStrictMode` changed to `SameSiteLaxMode` for login/logout cookies
- `GetSettingsHandler` response missing `db_path`, `tiers_file`, `default_watch_timeout_min` fields; `start_date` sent as RFC3339 string instead of Unix timestamp
- `HealthHandler` response missing `uptime` and `version` fields expected by frontend
- `UpdateTiersHandler` expecting flat `[]Tier` array but frontend sends `{tiers: [...]}`
- `UpdateWatchDefaultsHandler` JSON tag `default_watch_timeout` not matching frontend's `default_watch_timeout_min`
- `PaginationMeta.pageSize` in frontend not matching backend's `page_size` snake_case JSON, breaking transaction pagination

#### Added
- `Version` constant in poller config (`1.0.0`)
- `formatUptime()` helper and `serverStartTime` for health endpoint uptime tracking

## Restructure — 2026-02-20

#### Changed
- Symmetric directory structure: `internal/wallet/`, `internal/shared/`, `internal/poller/`
- `cmd/server/` renamed to `cmd/wallet/`
- Wallet frontend moved from `web/` to `web/wallet/`
- Shared packages (config, logging, models, price, scanner) moved to `internal/shared/`
- SPAHandler and RequestLogging extracted to `internal/shared/httputil/`
- HD key derivation package renamed from `wallet` to `hd` (`internal/wallet/hd/`)
- `.env` files renamed to `.env.wallet` and `.env.poller`
- Fixed BSC key signing (btcec to geth secp256k1 curve conversion)
- Merged poller CHANGELOG into root
- Updated PROJECT-MAP.md, .gitignore, CLAUDE.md

## Poller [1.0.0] — 2026-02-20

### 2026-02-20

#### Added (Phase 8: Embedding & Polish)
- Go `embed.FS` directive for SvelteKit build output (`web/poller/embed.go`)
- SPA handler wired into Chi router via `r.NotFound()` with immutable asset caching (`internal/poller/api/router.go`)
- `StaticFS` field in router Dependencies, wired from `cmd/poller/main.go` with `fs.Sub()` prefix stripping
- Makefile `build-poller-frontend` target, chained into `build-poller` for single-command build
- Dashboard DB tests: 8 test functions covering all dashboard queries (`internal/poller/pollerdb/dashboard_test.go`)
- Discrepancy + transaction DB tests: 7 test functions (`internal/poller/pollerdb/discrepancy_test.go`)
- `badgeClass()` generic badge helper and `abbreviateNumber()` chart formatter (`web/poller/src/lib/utils/formatting.ts`)

#### Changed (Phase 8: Embedding & Polish)
- **SECURITY**: Helius provider no longer logs RPC URL containing API key — logs `hasAPIKey` boolean instead (`internal/poller/provider/sol.go`)
- **SECURITY**: Session cookies now set `Secure: true` on both login and logout (`internal/poller/api/handlers/auth.go`)
- Dashboard handler uses `pollerconfig.DateRangeWeekDays/MonthDays/QuarterDays` constants instead of hardcoded `-7/-30/-90` (`internal/poller/api/handlers/dashboard.go`)
- Watches page imports shared `badgeClass()` instead of inline `statusBadgeClass()` (`web/poller/src/routes/watches/+page.svelte`)
- Makefile `clean` target now removes `web/poller/build` and `web/poller/.svelte-kit`
- pollerdb test coverage improved from 47.6% to 77.3%

#### Added (Phase 7: Dashboard Pages)
- Overview page (`/`): 8 stats cards (2x4 grid), time range selector, 7 ECharts chart components (`web/poller/src/routes/+page.svelte`)
- Transactions page (`/transactions`): 11-column table, 6 filters, page size control (25/50/100), server-side pagination (`web/poller/src/routes/transactions/+page.svelte`)
- Watches page (`/watches`): filter chips (status+chain), table with live countdown timers at 1s interval (`web/poller/src/routes/watches/+page.svelte`)
- Points page (`/points`): 3 summary cards (unclaimed/pending/all-time), merged points+pending data, color-coded values (`web/poller/src/routes/points/+page.svelte`)
- Errors page (`/errors`): 3 card sections (discrepancies, stale pending, system errors), severity indicators, explorer links (`web/poller/src/routes/errors/+page.svelte`)
- Settings page (`/settings`): tier editor with inline inputs + live example, IP allowlist CRUD, watch defaults form, system info grid (`web/poller/src/routes/settings/+page.svelte`)
- `ChartWrapper.svelte`: reusable ECharts wrapper with tree-shaking from `echarts/core` subpaths
- 7 chart components: UsdOverTime, PointsOverTime, TxCount, ChainBreakdown, TokenBreakdown, TierDistribution, WatchesOverTime (`web/poller/src/lib/components/charts/`)
- `TimeRangeSelector.svelte`, `StatsCard.svelte` dashboard sub-components (`web/poller/src/lib/components/dashboard/`)
- `explorer.ts`: block explorer URL helper handling SOL composite tx hashes (`web/poller/src/lib/utils/explorer.ts`)
- `formatTimestamp()`, `chainBadgeClass()` shared utilities (`web/poller/src/lib/utils/formatting.ts`)
- Constants: TOKEN_COLORS, TABLE_PAGE_SIZES, TABLE_DEFAULT_PAGE_SIZE, ALL_TOKENS, MAX_TIER_INDEX, EXPLORER_TX_URL, CONFIRMATIONS_REQUIRED (`web/poller/src/lib/constants.ts`)
- Dependencies: `echarts`, `svelte-echarts@1.0.0`, `@tanstack/table-core`

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
- HDPay logging parameterized: added `SetupWithPrefix()` for configurable log file prefix, `CleanOldLogs()` accepts prefix param (`internal/shared/logging/logger.go`)
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

## Wallet [1.0.0] — 2026-02-20

### 2026-02-20 — Safety Audit: User Money Loss & Misleading Information Fixes

#### Added
- **Network consistency check on startup**: Server refuses to start if DB contains addresses from a different network than configured (`HDPAY_NETWORK`). Prevents silent mainnet/testnet confusion (`internal/wallet/db/sqlite.go`, `cmd/wallet/main.go`)
- **Testnet banner**: Prominent yellow "TESTNET MODE" banner visible on every page when running on testnet (`web/wallet/src/routes/+layout.svelte`)
- **Price unavailability indicators**: Dashboard shows "Price unavailable" instead of misleading "$0.00" when CoinGecko price fetch fails. Stale price warning shown when prices are outdated (`internal/wallet/api/handlers/dashboard.go`, `PortfolioOverview.svelte`, `BalanceBreakdown.svelte`)
- **Token scan error visibility**: Warning displayed in scan progress UI when a token scan fails (e.g., "USDT scan failed — balances may be incomplete") (`web/wallet/src/lib/components/scan/ScanProgress.svelte`)
- **BSC token sweep balance divergence detection**: If on-chain token balance drops >5% from DB between preview and execute, address is skipped with clear error instead of silently sweeping less (`internal/wallet/tx/bsc_tx.go`, `internal/shared/config/constants.go`)
- **BSC EIP-55 checksum validation**: Mixed-case BSC destination addresses are validated against EIP-55 checksum to catch single-character typos that would send funds to unrecoverable addresses (`internal/wallet/api/handlers/send.go`)
- **Settings validation**: `max_scan_id` must be 1-500,000; `btc_fee_rate` must be non-negative; `resume_threshold_hours` must be >= 1 (`internal/wallet/api/handlers/settings.go`)
- **Per-chain scan timestamps**: Balance breakdown table shows "Last Scanned" per chain; portfolio overview shows "Oldest Scan" stat card identifying which chain has the most stale data (`BalanceBreakdown.svelte`, `PortfolioOverview.svelte`, `internal/wallet/db/balances.go`)
- Tests for all new functionality: `ValidateNetworkConsistency`, `GetScanTimesByChain`, `validateSettingValue`, BSC EIP-55 checksum validation, settings validation rejection

#### Removed
- **ResetAll feature**: Removed handler, route, and UI entirely — too dangerous to exist as a one-click operation. Users who need to reset can manually delete the SQLite file (`internal/wallet/api/handlers/settings.go`, `internal/wallet/api/router.go`, `web/wallet/src/routes/settings/+page.svelte`)

### 2026-02-20 — Per-Level Log File Splitting

#### Changed
- Log output now splits into separate files per level: `hdpay-YYYY-MM-DD-info.log`, `hdpay-YYYY-MM-DD-warn.log`, `hdpay-YYYY-MM-DD-error.log` (and `-debug.log` when debug level is configured) instead of a single combined file (`internal/shared/logging/logger.go`)
- Custom `multiHandler` routes each slog record to stdout (all levels) plus the matching level-specific file
- `multiCloser` ensures all file handles are closed on shutdown
- `LogFilePattern` constant updated to `"hdpay-%s-%s.log"` (date + level) (`internal/shared/config/constants.go`)
- Updated and expanded logger tests: file creation per level, exclusive routing verification, JSON validity check (`internal/shared/logging/logger_test.go`)

### 2026-02-19 — Network Column: Mainnet/Testnet Coexistence

#### Added
- **Migration 007**: Adds `network` column to all core tables (addresses, balances, scan_state, transactions, tx_state) so mainnet and testnet data coexist in the same database
- **Network-scoped DB queries**: All queries automatically filter by the active network via `DB.network` field — no caller changes needed
- **Network isolation tests**: `TestNetworkIsolation` verifies two DB instances (testnet + mainnet) on the same file see only their own data across all tables
- **Network-scoped reset tests**: `TestResetBalances_NetworkScoped` and `TestResetAll_NetworkScoped` verify reset operations only affect the active network

#### Changed
- `db.New(path)` signature changed to `db.New(path, network)` — all callers updated
- Migration auto-detects network for existing data using BTC address prefix (`bc1` = mainnet, `tb1` = testnet)
- Model structs (`Address`, `Balance`, `ScanState`, `Transaction`, `AddressWithBalance`) now include `Network` field

#### Fixed
- **BTC scan fails on mainnet**: Testnet addresses (`tb1...`) were being sent to mainnet APIs (Blockstream, Mempool) which returned HTTP 400, causing scan abort after 5 consecutive failures. Now switching `HDPAY_NETWORK` seamlessly switches which addresses are visible/scanned

### 2026-02-19 — Network Setting: Env-Only, Default Testnet

#### Fixed
- **Default network changed to testnet**: Config default was `mainnet`, now `testnet` via `HDPAY_NETWORK` env var

#### Changed
- Network is now env-only (`HDPAY_NETWORK`), not editable from the Settings UI
- Settings page shows current network as a read-only badge with env var hint
- Removed interactive network toggle (radio buttons) from Settings page

### 2026-02-19 — Fix Transaction Status Stuck on "Pending"

#### Added
- **Startup TX reconciler**: On server start, checks all non-terminal `tx_state` rows against the blockchain and updates both `tx_state` and `transactions` tables. Re-launches polling goroutines for recent pending TXs, marks old ones (>1h) as "uncertain" (`internal/wallet/tx/reconciler.go`, `cmd/wallet/main.go`)
- **`UpdateTransactionStatusByHash` DB method**: Updates all `transactions` rows matching a chain + txHash, handles BTC's multiple rows per txHash (`internal/wallet/db/transactions.go`)
- **`ReconcileMaxAge` / `ReconcileCheckTimeout` constants** (`internal/shared/config/constants.go`)
- Tests for `UpdateTransactionStatusByHash` (single row, multi-row BTC, failed status) and reconciler (SOL confirmed, BSC confirmed, no-hash failed, empty)

#### Fixed
- **Transaction status stuck on "pending" forever**: Background confirmation goroutines updated the `tx_state` table but never propagated terminal states (confirmed/failed) to the `transactions` table, which is what the `/transactions` page reads. All three chain services' `updateTxState()` helpers now also call `UpdateTransactionStatusByHash` for terminal states (`internal/wallet/tx/btc_tx.go`, `internal/wallet/tx/bsc_tx.go`, `internal/wallet/tx/sol_tx.go`)
- **Server restart loses confirmation polling**: If the server shut down while goroutines were polling for confirmation, those TXs stayed pending forever. The new startup reconciler handles this case

### 2026-02-19 — Async Send Execution with SSE Progress

#### Added
- **Async send execution**: `POST /api/send/execute` now returns 202 Accepted immediately with a `sweepID`, sweep runs in background goroutine (`internal/wallet/api/handlers/send.go`)
- **Per-TX SSE broadcasts**: All 5 sweep paths (BTC, BSC native/token, SOL native/token) broadcast `tx_status` events after each TX via `TxSSEHub` (`internal/wallet/tx/bsc_tx.go`, `internal/wallet/tx/sol_tx.go`, `internal/wallet/tx/btc_tx.go`)
- **`tx_complete` SSE event**: Includes full `TxResults` array so frontend can build completion view from SSE alone (`internal/wallet/tx/sse.go`)
- **`GET /api/send/sweep/{sweepID}` polling fallback**: Returns all TX states for a sweep when SSE disconnects (`internal/wallet/api/handlers/send.go`, `internal/wallet/api/router.go`)
- **`SweepStarted` response model**: `{ sweepID, chain, token, addressCount }` returned by async execute (`internal/shared/models/types.go`)
- **Frontend polling fallback**: Polls sweep status every 3s when SSE drops, auto-detects completion when all TXs reach terminal state (`web/wallet/src/lib/stores/send.svelte.ts`)
- **Navigation guard**: `beforeNavigate` + `beforeunload` warn user when leaving during active sweep (`web/wallet/src/routes/send/+page.svelte`)
- **Progress counter**: Shows "N of M transactions processed" during execution (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- **Escape hatch link**: "Check Transactions page" visible during execution (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- **`SEND_POLL_INTERVAL_MS` constant** (`web/wallet/src/lib/constants.ts`)

#### Changed
- `TxStatusData` extended with `FromAddress` and `Error` fields (`internal/wallet/tx/sse.go`)
- `TxCompleteData` extended with `TxResults []TxStatusData` field (`internal/wallet/tx/sse.go`)
- `TxSSEHub` injected into all 3 consolidation services via constructor (`internal/wallet/tx/bsc_tx.go`, `internal/wallet/tx/sol_tx.go`, `internal/wallet/tx/btc_tx.go`)
- Per-chain mutex uses `TryLock()` in handler, `defer mu.Unlock()` in goroutine (`internal/wallet/api/handlers/send.go`)
- Background sweep uses `context.Background()` since HTTP request context is dead after 202 response
- Frontend `executeSend()` return type changed to `SweepStarted` (`web/wallet/src/lib/utils/api.ts`)

#### Fixed
- **Send page stuck on "Executing..." forever**: Handler blocked synchronously until all TXs completed. Now returns 202 in <1ms, SSE drives real-time progress updates

### 2026-02-19

#### Fixed
- **"Last Scanned" showing stale timestamps**: Scanner resume logic caused user-initiated scans to skip already-scanned addresses, leaving them with old `last_scanned` values. Scans now always start from index 0 (`internal/shared/scanner/scanner.go`)
- **`hydrateBalances` picking arbitrary timestamp**: When an address had multiple token balance rows with different `last_scanned` values, the code picked whichever came first instead of the most recent. Now uses MAX across all tokens (`internal/wallet/db/addresses.go`, `internal/wallet/db/balances.go`)

### 2026-02-19 — SOL Token Sweep: Fee Payer Mechanism

#### Added
- **SOL fee payer support**: SOL token sweeps (USDC/USDT) now use Solana's native fee payer mechanism — a single address pays all transaction fees instead of each token holder paying their own. No gas pre-seeding required on SOL (`internal/wallet/tx/sol_tx.go`)
- **`FeePayerIndex` field on `SendRequest`**: Backend and frontend support optional fee payer index for SOL token sweeps (`internal/shared/models/types.go`, `web/wallet/src/lib/types.ts`)
- **Chain-aware GasPreSeedStep**: Frontend shows "Fee Payer Selection" for SOL (just pick an index, no API call) vs "Gas Pre-Seed" for BSC (sends BNB to each address) (`web/wallet/src/lib/components/send/GasPreSeedStep.svelte`)
- **Fee payer balance validation**: Backend checks fee payer has sufficient SOL for all estimated fees upfront before starting sweep

#### Fixed
- **SOL USDC send broken**: Gas pre-seed step rejected SOL addresses with "invalid BSC address" because the entire gas pre-seed system was BSC-only. SOL now bypasses gas pre-seeding entirely via fee payer mechanism

### 2026-02-19 — Security Audit: Scanning, Fund Movement & UX Safety

#### Security (Critical)
- **SOL blockhash staleness** (TX-1 CRITICAL): Cached blockhash was not tracking `lastValidBlockHeight`, so during multi-address sweeps later TXs could use expired blockhashes and silently fail. Now stores `lastValidBlockHeight`, estimates block consumption rate, and forces refresh when nearing expiry. Reduced cache TTL from 20s to 10s (`internal/wallet/tx/sol_tx.go`, `internal/shared/config/constants.go`)
- **Confirmation modal for irreversible sends** (UX-2 CRITICAL): Added typed "CONFIRM" + 3-second countdown modal before sweep execution. No accidental sends possible (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- **Full destination address at execute step** (UX-1 CRITICAL): Was truncated to 10 chars — now shows full address with copy button (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- **Network badge at execution** (UX-3 CRITICAL): Now displays prominent MAINNET (red) / TESTNET (yellow) badge at the critical send moment (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)

#### Security (High)
- **BTC fee safety margin** (TX-2): Added 2% safety margin to fee estimation to prevent underestimation from vsize rounding (`internal/wallet/tx/btc_tx.go`, `internal/shared/config/constants.go`)
- **BTC UTXO divergence thresholds tightened** (TX-3): Reduced from 20%/10% to 5%/3% count/value thresholds. Added warning logs for ANY divergence (`internal/wallet/tx/btc_tx.go`, `internal/shared/config/constants.go`)
- **BSC per-TX gas check** (TX-5): Token sweep now checks gas balance before EACH transfer, skipping addresses that ran out of gas mid-sweep instead of failing entire batch (`internal/wallet/tx/bsc_tx.go`)
- **Scanner completion race** (SC-2): `removeScan()` was firing via defer before `finishScan()` DB writes completed. Moved into `finishScan()` after all writes, and added to all cancellation return paths (`internal/shared/scanner/scanner.go`)
- **Double-click protection** (UX-6): Added synchronous click guard + `pointer-events: none` CSS to prevent duplicate sweep execution (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- **Gas pre-seed skip warning** (UX-7): "Skip" button now shows warning modal explaining token transfers will fail without gas (`web/wallet/src/lib/components/send/GasPreSeedStep.svelte`)

#### Security (Medium)
- **Scan context timeout** (SC-6): Scan goroutine now has 24h upper bound via `context.WithTimeout` to prevent goroutine leaks (`internal/shared/scanner/scanner.go`, `internal/shared/config/constants.go`)
- **Token failure backoff** (SC-5): Token fetch failures now trigger exponential backoff like native failures, preventing provider hammering (`internal/shared/scanner/scanner.go`)
- **Non-atomic scan state retry** (SC-3): Added single retry with 100ms pause for scan state writes that fail outside transactions (`internal/shared/scanner/scanner.go`)
- **Rate limiter burst control** (SC-7): Changed from `Burst(rps)` to `Burst(1)` for even request distribution (`internal/shared/scanner/ratelimiter.go`)
- **HTTP connection pool limits** (SC-8): Configured `Transport` with `MaxConnsPerHost=10`, `MaxIdleConnsPerHost=5`, `MaxIdleConns=50` to prevent file descriptor exhaustion (`internal/shared/scanner/setup.go`)

#### Added
- USD value display on execute step using price API (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- Fee estimate display on execute step (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- TX count explanation for multi-TX chains ("one per funded address") (`web/wallet/src/lib/components/send/PreviewStep.svelte`)
- SSE connection status indicator during execution (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- "View Transactions" link after sweep completion (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- Copy buttons for destination address and TX hashes (`web/wallet/src/lib/components/send/ExecuteStep.svelte`)
- SOL blockhash safety constants: `SOLBlocksPerSecondEstimate`, `SOLBlockhashSafetyMarginBlocks` (`internal/shared/config/constants.go`)

### 2026-02-19

#### Fixed
- **SOL USDC balance display mangled**: SQLite `CAST(SUM(CAST(balance AS REAL)) AS TEXT)` appended ".0" to aggregated balances, causing `formatRawBalance` string slicing to produce "4000.0000." instead of "40" — switched SQL to `printf('%.0f', SUM(...))` and added defensive decimal-point truncation in frontend (`internal/wallet/db/balances.go`, `web/wallet/src/lib/utils/formatting.ts`)
- **SOL token preview undercounts funded addresses**: `buildSOLTokenPreview()` now calculates `HasGas` per-address by checking native SOL balance, sums total from ALL funded addresses, and sets `NeedsGasPreSeed` when addresses lack SOL for fees — mirrors BSC token preview pattern (`internal/wallet/api/handlers/send.go`)
- **SOL token execute crashes on gas-less addresses**: `ExecuteTokenSweep()` now checks native balance before attempting each transfer, skipping gas-less addresses gracefully with a logged warning instead of failing at broadcast (`internal/wallet/tx/sol_tx.go`)
- **All chains send stuck on "Executing..."**: SOL and BSC sweep services waited for TX confirmation synchronously inside the HTTP handler, exceeding `ServerWriteTimeout`. Moved confirmation polling to background goroutines for all sweep paths (SOL native, SOL token, BSC native, BSC token) — returns immediately after broadcast with "success" status, background updates tx_state (`internal/wallet/tx/sol_tx.go`, `internal/wallet/tx/bsc_tx.go`)

### 2026-02-19 — Post-V2 Comprehensive Audit Fixes

#### Security
- **CSRF empty token bypass**: `generateCSRFToken()` now returns error on `crypto/rand.Read` failure instead of empty string; middleware returns HTTP 500 (`internal/wallet/api/middleware/security.go`)
- **BSC private key zeroing**: Added `ZeroECDSAKey()` helper; all 3 BSC signing callsites now `defer ZeroECDSAKey(privKey)` (`internal/wallet/tx/key_service.go`, `internal/wallet/tx/bsc_tx.go`, `internal/wallet/tx/gas.go`)

#### Fixed
- **Hard-coded testnet explorer links**: `ExecuteStep.svelte` and transactions page now read network from `GET /api/settings` instead of hard-coding `'testnet'`
- **`formatRawBalance` precision loss**: Rewrote to use string-based decimal placement instead of `parseFloat()` — fixes incorrect display for values > 2^53 (`web/wallet/src/lib/utils/formatting.ts`)
- **`formatDate` invalid input**: Added `isNaN(date.getTime())` guard, returns `'N/A'` for invalid dates (`web/wallet/src/lib/utils/formatting.ts`)
- **Log rotation**: `Setup()` now returns `io.Closer` for graceful shutdown; added `CleanOldLogs()` that deletes hdpay-*.log files older than `LogMaxAgeDays` on startup (`internal/shared/logging/logger.go`)
- **Config validation**: Added `Validate()` method checking Network is "mainnet"|"testnet" and Port is 1-65535 (`internal/shared/config/config.go`)
- **ScanProgress ETA div-zero**: Guarded `elapsedMs <= 0` case (`web/wallet/src/lib/components/scan/ScanProgress.svelte`)
- **Copy timeout cleanup**: Stored timeout ID and clear on new copy in AddressTable and transactions page

#### Changed
- Removed misleading "All Chains" tab from addresses page — was silently defaulting to BTC (`web/wallet/src/routes/addresses/+page.svelte`)
- Added BTC fee estimation TTL cache (2-minute expiry) to avoid hammering mempool.space on repeated previews (`internal/wallet/tx/btc_fee.go`)
- Removed duplicate `TOKEN_DECIMALS` and unused `getTokenDecimals()` from `chains.ts` — canonical version in `constants.ts` is unchanged
- Settings API now includes `network` field for frontend to read server config (`internal/wallet/api/handlers/settings.go`)
- Added `ErrInvalidConfig` error and `FeeCacheTTL` constant (`internal/shared/config/errors.go`, `internal/shared/config/constants.go`)

### 2026-02-19 — Post-V2 Bug Fixes & End-to-End Testing

#### Added
- Startup provider health checks: parallel probe of all configured endpoints at boot (`internal/shared/scanner/healthcheck.go`)
  - BTC: Blockstream, Mempool (block tip height GET)
  - BSC: BscScan (eth_blockNumber), RPC (eth_blockNumber JSON-RPC)
  - SOL: Solana RPC (getHealth JSON-RPC), optional Helius
  - CoinGecko: /ping
  - Non-blocking — runs as goroutine, logs OK/WARN per provider with latency
- `HealthCheckTimeout = 10 * time.Second` constant (`internal/shared/config/constants.go`)
- Logging middleware test suite: Flusher interface, Unwrap method, status/size capture (`internal/wallet/api/middleware/logging_test.go`)

#### Fixed
- **`.env` file loading**: Added godotenv autoload so environment variables are actually read at startup
- **Scan context cancellation**: `StopScan()` now properly cancels the running scan context instead of being a no-op
- **Portfolio USD calculation**: `GetPortfolio` handler now converts raw blockchain units (satoshis/wei/lamports) to human-readable before USD multiplication — was showing ~0.001 USD instead of ~181,000 USD
- **SQLite BUSY errors**: Increased `DBBusyTimeout` from 5s to 30s to handle concurrent scan writes
- **BscScan API V1 deprecation**: Migrated from `api.bscscan.com/api` to `api.bscscan.com/v2/api` with `chainid=56` parameter
- **Network-aware token contracts**: Scanner now uses testnet/mainnet contract addresses based on config instead of always using mainnet
- **Svelte 5 rune file extensions**: Renamed `.ts` store files to `.svelte.ts` for `$state`/`$derived` rune support
- **SSE Flusher passthrough**: Logging middleware `responseWriter` now implements `http.Flusher` via delegation to underlying writer — SSE streaming was silently broken
- **Addresses table raw balance display**: Changed from `formatBalance()` (no conversion) to `formatRawBalance()` (divides by 10^decimals) — was showing "100000000000" instead of "100 SOL"
- **Scan funded count**: SSE `scan_complete` event now sends actual funded count instead of hardcoded 0
- **Send preview balance display**: All 4 `formatBalance` calls in `PreviewStep.svelte` replaced with `formatRawBalance` — fee, net amount, total, and per-address amounts
- **Gas pre-seed balance display**: 2 `formatBalance` calls in `GasPreSeedStep.svelte` replaced with `formatRawBalance` — address balance and total sent
- **Execute step balance display**: 4 `formatBalance` calls in `ExecuteStep.svelte` replaced with `formatRawBalance` — confirmation amount, tx amounts, and total swept
- **Transaction history display**: `formatBalance(tx.amount, 8)` in transactions page replaced with `formatRawBalance(tx.amount, tx.chain, tx.token)` — was showing raw units
- **Dashboard test data**: Updated `TestGetPortfolio_WithBalances` to use raw blockchain units (satoshis/wei/lamports) matching production storage format
- **BscScan test compatibility**: Fixed `TestBscScanProvider_FetchNativeBalances` and `_MissingAddress` tests for `json.RawMessage` Result field (V2 Phase 2 change)

#### Changed
- `cmd/wallet/main.go`: Startup health checks run as background goroutine after scanner setup
- All send wizard components (`PreviewStep`, `GasPreSeedStep`, `ExecuteStep`) import `formatRawBalance` instead of `formatBalance`
- Transactions page imports `formatRawBalance` instead of `formatBalance`

### 2026-02-19 (V2 Phase 6) — Security Tests & Infrastructure — V2 COMPLETE

#### Added
- Security middleware test suite: 21 tests covering HostCheck (7), CORS (7), CSRF (7) (`internal/wallet/api/middleware/security_test.go`)
- Mempool provider test suite: 9 tests — success, rate limit, server error, malformed JSON, partial failure, all fail, context cancellation, token not supported, metadata (`internal/shared/scanner/btc_mempool_test.go`)
- BSC RPC provider test suite: 8 tests — native balance, zero balance, all fail, partial failure, token balance, null token, context cancellation, metadata (`internal/shared/scanner/bsc_rpc_test.go`)
- Solana RPC provider test suite: 9 tests — native balance, null account, partial results, RPC error, nil result, rate limited, token balance, null ATA, metadata (`internal/shared/scanner/sol_rpc_test.go`)
- TX SSE hub test suite: 8 tests — subscribe/unsubscribe, broadcast, slow client drop, concurrent race safety, Run cancellation (`internal/wallet/tx/sse_test.go`)
- Price stale-but-serve: `GetPrices()` returns stale cache when live fetch fails (30-min tolerance), `IsStale()` method
- 3 price staleness tests: stale cache on error, no cache returns error, expired cache returns error
- Constants: `ServerIdleTimeout`, `ServerMaxHeaderBytes`, `DBMaxOpenConns`, `DBMaxIdleConns`, `DBConnMaxLifetime`, `ShutdownTimeout`, `PriceStaleTolerance`

#### Changed
- HTTP server: added `IdleTimeout` (5 min) and `MaxHeaderBytes` (1 MB) for hardening
- SQLite connection pool: `SetMaxOpenConns(25)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5 min)` from centralized config
- Graceful shutdown: timeout now uses `config.ShutdownTimeout` (10 min), ordered drain (cancel hub -> HTTP shutdown -> DB close)
- Dashboard prices response: now returns `{ prices: {...}, stale: bool }` instead of flat price map
- Frontend `getPrices()` return type updated to `PriceResponse` with stale field

### 2026-02-19 (V2 Phase 5) — Provider Health & Broadcast Fallback

#### Added
- DB-backed provider health recording in scanner Pool: `SetDB()` method with initial health row upsert, `recordHealthSuccess()` / `recordHealthFailure()` helpers called after circuit breaker state changes
- `UpdateProviderCircuitState()` DB method deriving status from circuit state (closed→healthy, half_open→degraded, open→down)
- `GET /api/health/providers` endpoint returning all provider health rows grouped by chain (`internal/wallet/api/handlers/provider_health.go`)
- `FallbackEthClient` BSC broadcast wrapper: tries primary RPC, falls back to Ankr on `SendTransaction` failure (`internal/wallet/tx/bsc_fallback.go`)
- SOL broadcast fallback: `doRPCAllURLs()` tries all configured RPC URLs before returning first error
- Live provider health frontend component with color-coded status dots (green=healthy, yellow=degraded, red=down), loading/error states
- Frontend types: `ProviderHealthStatus`, `CircuitState`, `ProviderHealth`, `ProviderHealthMap`
- `getProviderHealth()` API client function
- 5 new tests: BSC FallbackEthClient (primary success, fallback success, both fail, nil fallback, delegation)

#### Changed
- Scanner `Pool` now accepts optional DB via `SetDB()` for non-blocking health persistence
- `pool.go` records health success/failure after every circuit breaker state change
- `setup.go` injects DB into BTC, BSC, SOL pools after creation
- `cmd/wallet/main.go` creates BSC fallback client (Ankr) for mainnet deployments
- SOL `SendTransaction` uses `doRPCAllURLs` instead of `doRPC` for broadcast redundancy
- `ProviderStatus.svelte` rewritten from static hardcoded data to live API-driven component

### 2026-02-18 (V2 Phase 4) — TX Safety — Advanced

#### Added
- BTC UTXO re-validation at execute time (A6): `ValidateUTXOsAgainstPreview()` — rejects if UTXO count drops >20% or value drops >10%
- BSC on-chain balance recheck (A7): `BalanceOfBEP20()` for BEP-20 token balance verification via `eth_call`
- `bep20BalanceOfSelector` and `bscMinNativeSweepWei` package-level vars for BSC balance operations
- Partial sweep resume endpoints (A8): `GET /api/send/resume/{sweepID}` (summary) and `POST /api/send/resume` (retry failed/uncertain)
- `GetRetryableTxStates()`, `GetSweepMeta()`, `HasConfirmedTxForAddress()` DB methods in `internal/wallet/db/tx_state.go`
- Gas pre-seed idempotency (A9): filters already-confirmed targets via tx_state lookup before sending
- `updateGasTxState()` non-blocking helper for gas pre-seed tx_state lifecycle tracking
- SOL ATA visibility polling (A10): `waitForATAVisibility()` — polls `GetAccountInfo` after ATA creation (30s timeout, 2s poll)
- BSC gas price spike detection (A11): `ValidateGasPriceAgainstPreview()` — rejects if current gas >2x preview price
- Nonce gap handling (A12): `isNonceTooLowError()` detection covering common BSC error patterns
- Single retry with fresh nonce re-fetch on nonce-too-low errors in gas pre-seed Execute loop
- Constants: `BSCMinNativeSweepWei`, `BSCGasPriceMaxMultiplier`, `SOLATAConfirmationTimeout`, `SOLATAConfirmationPollInterval`
- Sentinel errors: `ErrUTXOSetChanged`, `ErrBalanceChangedSignificantly`, `ErrGasPriceSpiked`
- Error codes: `ErrorUTXOSetChanged`, `ErrorBalanceChanged`, `ErrorGasPriceSpiked`
- 14 new tests: BSC balance recheck (5), gas price validation (4), UTXO validation (4), nonce detection (1 with 8 sub-cases)

#### Changed
- `BTCConsolidationService.Execute()` now validates UTXOs against preview expectations before building TX
- `BSCConsolidationService.ExecuteNativeSweep/ExecuteTokenSweep` accept optional `expectedGasPrice` for spike detection
- `sweepNativeAddress` re-fetches real-time balance, logs divergence from DB, checks minimum sweep threshold
- `sweepTokenAddress` re-fetches on-chain token balance, uses conservative (lower) value
- `GasPreSeedService.Execute()` accepts optional `sweepID` for idempotency tracking
- `sendGasPreSeed()` creates tx_state rows and updates through full lifecycle
- `EthClientWrapper` interface extended with `CallContract` method
- Send handler passes `req.ExpectedGasPrice` to BSC sweep methods
- Router adds resume routes under `/api/send/`

### 2026-02-18 (V2 Phase 3) — TX Safety — Core

#### Added
- Per-chain concurrent send mutex with `TryLock()` — returns HTTP 409 Conflict if a sweep is already in progress (A1)
- BTC confirmation polling via Esplora `/tx/{txid}/status` — polls every 15s for up to 10 min, round-robin across providers (A2)
- SOL confirmation uncertainty tracking — 3 consecutive RPC errors → `uncertain` status instead of infinite retry (A3)
- In-flight TX persistence via `tx_state` table for all 3 chains: BTC (single consolidated TX), BSC (per-address), SOL (per-address) (A4)
- SOL blockhash cache with 20s TTL — reduces redundant RPC calls during multi-address sweeps (A5)
- `GET /api/send/pending` endpoint to list in-flight/uncertain transactions
- `POST /api/send/dismiss/{id}` endpoint to dismiss uncertain/failed transactions
- `GetAllPendingTxStates()` DB method
- `GenerateTxStateID()` for unique per-TX identifiers
- `WaitForBTCConfirmation()` with `btcTxStatus` struct
- Constants: `BTCConfirmationTimeout`, `BTCConfirmationPollInterval`, `BTCTxStatusPath`, `SOLBlockhashCacheTTL`, `SOLMaxConfirmationRPCErrors`, `TxStateDismissed`
- Sentinel errors: `ErrBTCConfirmationTimeout`, `ErrSOLConfirmationUncertain`
- Error codes: `ErrorBTCConfirmationTimeout`, `ErrorSOLConfirmationUncertain`, `ErrorSendBusy`

#### Changed
- `BTCConsolidationService.Execute()` now accepts `sweepID`, tracks full tx_state lifecycle, includes confirmation polling
- `BSCConsolidationService.ExecuteNativeSweep/ExecuteTokenSweep` now accept `sweepID`, track per-address tx_state
- `SOLConsolidationService.ExecuteNativeSweep/ExecuteTokenSweep` now accept `sweepID`, track per-address tx_state, use blockhash cache
- `WaitForSOLConfirmation` tracks consecutive RPC errors, returns `ErrSOLConfirmationUncertain` after 3 failures
- SOL sweep methods distinguish `uncertain` from `failed` via `errors.Is(err, config.ErrSOLConfirmationUncertain)`
- All tx_state writes are non-blocking with nil-safe helpers (`createTxState()`, `updateTxState()`)
- `ExecuteSend` handler generates `sweepID` before dispatching and acquires per-chain mutex

### 2026-02-18 (V2 Phase 2) — Scanner Resilience

#### Added
- Error collection pattern in all 6 providers: Blockstream, Mempool, BscScan, BSC RPC, Solana RPC — continue on per-address failure, annotate `BalanceResult.Error`
- Retry-After header parser (`internal/shared/scanner/retry_after.go`) — handles seconds and HTTP-date formats, 9 tests
- Partial result validation for Solana `getMultipleAccounts` — detects and annotates missing results (B3)
- Atomic batch DB writes: `BeginTx()`, `UpsertBalanceBatchTx()`, `UpsertScanStateTx()` — balances + scan state in single transaction (B4)
- Per-provider circuit breakers wired into Pool: `Allow()` check, `RecordSuccess/Failure`, failover on open (B5)
- Exponential backoff on consecutive all-provider failures: 1s→2s→4s...30s cap, abort after 5 consecutive (B11)
- `scan_token_error` SSE event for frontend visibility of token scan failures (B7)
- `scan_state` SSE event sent on client connect for resync (B10) — contains status + running state for all chains
- `ScanTokenErrorData` and `ScanStateSnapshotData` structs in `sse.go`
- `GetAllScanStates()` DB method for fetching all chain scan states
- Frontend types: `ScanTokenErrorEvent`, `ScanStateSnapshot`
- Frontend SSE listeners for `scan_token_error` and `scan_state` events
- `lastTokenError` state field in scan store
- New constants: `ExponentialBackoffBase`, `ExponentialBackoffMax`, `MaxConsecutivePoolFails`
- New errors: `ErrPartialResults`, `ErrAllProvidersFailed`, error codes `ErrorPartialResults`, `ErrorAllProvidersFailed`, `ErrorTokenScanFailed`

#### Changed
- Scanner orchestrator (`internal/shared/scanner/scanner.go`) rewritten for V2: atomic DB writes, decoupled native/token, backoff loop, token error SSE
- Native balance failure no longer aborts token scans for the same batch (B8)
- `finishScan` now receives and broadcasts accurate `found` count (was hardcoded 0)
- Pool uses `errors.Join()` to return all provider errors instead of just the last (B9)
- `ScanSSE` handler now takes scanner + db args for resync support
- All HTTP 429 responses wrapped with `NewTransientErrorWithRetry(ErrProviderRateLimit, retryAfter)`
- All non-200 HTTP responses wrapped with `NewTransientError(...)`
- BSC RPC `callBalanceOf` returns error on malformed response instead of silent zero
- BscScan `FetchNativeBalances` detects and annotates addresses missing from response
- Rewritten test suites: `btc_blockstream_test.go` (8 tests), `bsc_bscscan_test.go` (7 tests), `pool_test.go` (11 tests)

### 2026-02-18 (V2 Phase 1) — Foundation: Schema, Error Types & Circuit Breaker

#### Added
- `tx_state` DB table + migration (005) for tracking individual TX lifecycle through broadcast (pending→broadcasting→confirming→confirmed|failed|uncertain)
- `provider_health` DB table + migration (006) for per-provider health and circuit breaker state
- TX state CRUD methods: CreateTxState, UpdateTxStatus, GetPendingTxStates, GetTxStatesBySweepID, GetTxStateByNonce, CountTxStatesByStatus
- Provider health CRUD methods: UpsertProviderHealth, GetProviderHealth, GetProviderHealthByChain, GetAllProviderHealth, RecordProviderSuccess, RecordProviderFailure
- `TransientError` type with `IsTransient()` and `GetRetryAfter()` helpers for retry classification
- `ErrCircuitOpen` and `ErrProviderTimeout` sentinel errors
- Circuit breaker state machine (closed/open/half-open) in `internal/shared/scanner/circuit_breaker.go`
- Sweep ID generator (`GenerateSweepID`) using crypto/rand
- Constants: TX state statuses, provider health statuses, provider types, circuit states, circuit breaker config
- 24 new tests: tx_state (8), provider_health (6), circuit_breaker (8), errors (4)

#### Changed
- `BalanceResult` now includes `Error` and `Source` fields for error annotation and provider attribution
- All 6 scanner providers (Blockstream, Mempool, BscScan, BSCRPC, SolanaRPC x2) populate `Source` field
- `TestRunMigrationsIdempotent` made migration-count-agnostic

### 2026-02-18 (Phase 11) — V1 COMPLETE

#### Added
- Transaction history API: `ListTransactionsFiltered` DB method with dynamic WHERE clause supporting chain, direction, token, status filters
- `GET /api/transactions` and `GET /api/transactions/{chain}` handlers with query param validation
- Transaction history frontend: filter toolbar (chain/direction/token chip groups), table with chain badges, direction icons, explorer links, copy-to-clipboard, status badges, pagination
- Settings API: `GetSetting`, `SetSetting` (upsert), `GetAllSettings` (fills defaults), `ResetBalances`, `ResetAll` DB methods
- `GET /api/settings`, `PUT /api/settings`, `POST /api/settings/reset-balances`, `POST /api/settings/reset-all` handlers
- Settings frontend: Network mode radio cards, Scanning config, Transaction config, Display config, Danger zone with two-step confirmation
- Embedded SPA serving via `go:embed all:build` in `web/wallet/embed.go`
- `SPAHandler` with immutable cache headers for `_app/` assets and SPA fallback to index.html
- 40 new tests: DB settings (8), DB transaction filters (5), handler settings (9), handler transactions (13), SPA handler (5)

#### Changed
- `NewRouter` now accepts optional `fs.FS` for embedded SPA serving
- `cmd/wallet/main.go` imports `web` package and passes embedded FS to router
- Router uses `r.NotFound(SPAHandler)` catch-all for client-side routing after `/api` routes
- `make build` produces 22MB single binary with embedded SPA

### 2026-02-18 (Phase 10)

#### Added
- Unified send API: `POST /api/send/preview`, `POST /api/send/execute`, `POST /api/send/gas-preseed` dispatching to chain-specific TX engines
- TX SSE hub for real-time transaction status streaming (`internal/wallet/tx/sse.go`)
- Send handler with chain-specific preview/execute dispatch (`internal/wallet/api/handlers/send.go`)
- SendDeps dependency injection struct wiring all TX services
- `setupSendDeps` in main.go initializing all TX services from config
- Unified send types: SendRequest, UnifiedSendPreview, UnifiedSendResult, TxResult, FundedAddressInfo
- `GetFundedAddressesJoined` DB query — JOIN between addresses and balances tables
- Frontend address validation: BTC bech32/legacy, BSC hex, SOL base58 (`web/wallet/src/lib/utils/validation.ts`)
- Send wizard store with Svelte 5 runes and SSE integration (`web/wallet/src/lib/stores/send.svelte.ts`)
- 4-step wizard UI: SelectStep, PreviewStep, GasPreSeedStep, ExecuteStep components
- Stepper component with collapsed completed-step summaries
- `getExplorerTxUrl` convenience function in chains.ts
- Backend tests: 24 tests for validateDestination + isValidToken (`internal/wallet/api/handlers/send_test.go`)
- Frontend tests: 23 vitest tests for address validation (`web/wallet/src/lib/utils/validation.test.ts`)
- Vitest added as devDependency for frontend testing
- `EstimateGasPrice` method on BSCConsolidationService
- Explorer URL constants in backend config (aligned with frontend)
- Send-related error codes and constants

#### Changed
- Explorer URLs updated: mempool.space (BTC), solscan.io (SOL), bscscan.com unchanged (BSC)
- Router updated to accept SendDeps and wire /api/send/* routes
- SOL explorer URL path changed from `/address/` to `/account/` for solscan.io compatibility

### 2026-02-18 (Phase 9)

#### Added
- Raw Solana binary transaction serialization: compact-u16 encoding, message headers, compiled instructions, ed25519 signing (`internal/wallet/tx/sol_serialize.go`)
- SystemProgram.Transfer instruction builder for native SOL transfers
- SPL Token.Transfer instruction builder for USDC/USDT token transfers
- CreateAssociatedTokenAccount instruction builder for auto-ATA creation
- SOL private key derivation via SLIP-10 ed25519 path m/44'/501'/N'/0' (`internal/wallet/tx/key_service.go`)
- SOL JSON-RPC client with round-robin URL selection: getLatestBlockhash, sendTransaction, getSignatureStatuses, getAccountInfo, getBalance (`internal/wallet/tx/sol_tx.go`)
- Confirmation polling with configurable timeout and "confirmed" commitment level
- SOLConsolidationService: sequential per-address native SOL sweep + SPL token sweep with auto-ATA creation
- Preview methods for native and token sweeps (dry-run cost/balance calculation)
- SOL model types: `SOLSendPreview`, `SOLSendResult`, `SOLTxResult` (`internal/shared/models/types.go`)
- SOL transaction constants: lamports, fees, TX size limit, confirmation timing, program IDs (`internal/shared/config/constants.go`)
- 5 new sentinel errors + 5 error codes for SOL-specific failures (`internal/shared/config/errors.go`)
- 29 new tests: serialization (13), consolidation service (14), key derivation (2)

### 2026-02-18 (Phase 8)

#### Added
- BSC private key derivation: BIP-44 m/44'/60'/0'/0/N → ECDSA key + EIP-55 address (`internal/wallet/tx/key_service.go`)
- Native BNB transfer engine: LegacyTx building, EIP-155 signing (chain ID 56/97), 20% gas price buffer (`internal/wallet/tx/bsc_tx.go`)
- BEP-20 token transfer: Manual ABI encoding of `transfer(address,uint256)` with selector `0xa9059cbb`
- Receipt polling: `WaitForReceipt` with `ethereum.NotFound` detection, revert handling, configurable timeout
- BSC Consolidation Service: Sequential per-address sweep for native BNB + BEP-20 tokens with real-time balance checks
- Gas pre-seeding service: Distributes 0.005 BNB to targets needing gas, sequential nonce management (`internal/wallet/tx/gas.go`)
- EthClientWrapper interface for testability (PendingNonceAt, SuggestGasPrice, SendTransaction, TransactionReceipt, BalanceAt)
- BSC model types: `BSCSendPreview`, `BSCSendResult`, `BSCTxResult`, `GasPreSeedPreview`, `GasPreSeedResult`
- BSC constants: chain IDs, gas price buffer, receipt polling interval/timeout, BEP-20 selector
- 5 new sentinel errors + 4 new error codes for BSC-specific failures
- 22 new tests: BSC TX engine (18), gas pre-seed (4), BSC key derivation (4)

### 2026-02-18 (Phase 7)

#### Added
- On-demand BIP-84 private key derivation service — reads mnemonic from file, derives keys per index, caller zeros after use (`internal/wallet/tx/key_service.go`)
- UTXO fetcher — confirmed UTXOs from Blockstream/Mempool APIs with round-robin rotation and rate limiting (`internal/wallet/tx/btc_utxo.go`)
- Dynamic fee estimator — fetches from mempool.space `/v1/fees/recommended` with fallback to config constant (`internal/wallet/tx/btc_fee.go`)
- Multi-input P2WPKH transaction builder — vsize estimation, wire.MsgTx construction, consolidation to single output (`internal/wallet/tx/btc_tx.go`)
- P2WPKH witness signer — `MultiPrevOutFetcher` + `NewTxSigHashes` (once) + `WitnessSignature` per input
- BTC broadcaster — POST raw hex as `text/plain` with ordered provider fallback, no retry on 400 (`internal/wallet/tx/broadcaster.go`)
- Transaction DB CRUD — insert, update status, get by ID/hash, paginated list with chain filter (`internal/wallet/db/transactions.go`)
- Shared `Broadcaster` interface for future BSC/SOL reuse
- `BTCConsolidationService` orchestrator with `Preview()` and `Execute()` methods
- Domain types: `UTXO`, `FeeEstimate`, `SendPreview`, `SendResult` (`internal/shared/models/types.go`)
- BTC TX constants: dust threshold, vsize weights, fee estimation timeout, max inputs (`internal/shared/config/constants.go`)
- 7 new sentinel errors + 4 new error codes (`internal/shared/config/errors.go`)
- 38 new tests: key service (7), UTXO fetcher (7), fee estimator (4), TX builder (10), broadcaster (4), transaction DB (7)

### 2026-02-18 (Phase 6)

#### Added
- CoinGecko price service with 5-minute in-memory cache, thread-safe (`internal/shared/price/coingecko.go`)
- `GET /api/dashboard/prices` — returns USD prices keyed by symbol (BTC, BNB, SOL, USDC, USDT)
- `GET /api/dashboard/portfolio` — aggregated balances with USD values, address counts, last scan time
- Balance aggregation DB queries: `GetBalanceAggregates`, `GetLatestScanTime` (`internal/wallet/db/balances.go`)
- Dashboard route group in Chi router with PriceService dependency injection
- PortfolioOverview component: total value display + 4 stat cards (addresses, funded, chains, last scan)
- BalanceBreakdown component: table with chain badges, token, balance, USD value, funded count
- PortfolioCharts component: ECharts donut pie chart showing USD distribution by chain
- Dashboard page with auto-refresh (1-min portfolio interval, 5-min price cache server-side)
- ECharts + svelte-echarts v1.0.0 installed with tree-shaking (PieChart only)
- Frontend types: `PortfolioResponse`, `ChainPortfolio`, `TokenPortfolioItem`
- Frontend API functions: `getPrices`, `getPortfolio`
- Frontend constants: `PRICE_REFRESH_INTERVAL_MS`, `PORTFOLIO_REFRESH_INTERVAL_MS`
- Backend constants: `CoinGeckoIDs`, `ErrPriceFetchFailed`
- 9 new tests: price service (6), dashboard handlers (3)

#### Changed
- Updated `PriceData` interface to use symbol keys (BTC, BNB, SOL, USDC, USDT)
- Removed old `PortfolioSummary`/`ChainSummary`/`TokenSummaryItem` types (replaced by API-matching types)
- `NewRouter` now accepts `*price.PriceService` parameter

### 2026-02-18 (Phase 5)

#### Added
- Scan API handlers: `POST /api/scan/start`, `POST /api/scan/stop`, `GET /api/scan/status`, `GET /api/scan/sse` (`internal/wallet/api/handlers/scan.go`)
- SSE streaming handler with keepalive ticker and hub subscribe/unsubscribe lifecycle
- Scanner wired into `main.go`: SSEHub creation, SetupScanner, hub.Run goroutine
- Scan routes added to Chi router (`internal/wallet/api/router.go`)
- Exported test helpers: `NewPoolForTest`, `testProvider` in `internal/shared/scanner/setup.go`
- 11 scan handler tests covering start/stop/status/SSE endpoints (`scan_test.go`)
- Frontend scan store with SSE connection management (`web/wallet/src/lib/stores/scan.svelte.ts`)
- EventSource with named event listeners (`scan_progress`, `scan_complete`, `scan_error`)
- Exponential backoff reconnect (1s base, 30s cap, 2x multiplier)
- ScanControl component: chain selector, max ID input, start/stop buttons, info alert
- ScanProgress component: per-chain progress bars, status badges, ETA calculation
- ProviderStatus component: static provider health grid (V1)
- Scan page assembly with SSE lifecycle (connect on mount, disconnect on cleanup)
- SSE connection indicator in page header (Live/Connecting/Reconnecting/Disconnected)
- Frontend scan API functions: `startScan`, `stopScan`, `getScanStatus`, `getScanStatusForChain`
- Frontend types: `ScanCompleteEvent`, `ScanErrorEvent`, `ScanStateWithRunning`
- Frontend constants: `DEFAULT_MAX_SCAN_ID`, `MAX_SCAN_ID`, `SSE_MAX_RECONNECT_DELAY_MS`, `SSE_BACKOFF_MULTIPLIER`

### 2026-02-18 (Phase 4)

#### Added
- Multi-provider scanner engine with round-robin rotation and automatic failover
- BTC providers: Blockstream Esplora, Mempool.space (`internal/shared/scanner/btc_blockstream.go`, `internal/shared/scanner/btc_mempool.go`)
- BSC providers: BscScan REST API (batch 20 native, single token), ethclient JSON-RPC (`internal/shared/scanner/bsc_bscscan.go`, `internal/shared/scanner/bsc_rpc.go`)
- SOL provider: Solana JSON-RPC with `getMultipleAccounts` batch 100 (`internal/shared/scanner/sol_rpc.go`)
- Manual Solana ATA derivation via PDA + Edwards25519 on-curve check (`internal/shared/scanner/sol_ata.go`)
- Provider interface + BalanceResult type (`internal/shared/scanner/provider.go`)
- Per-provider rate limiter using `golang.org/x/time/rate` (`internal/shared/scanner/ratelimiter.go`)
- Provider pool with round-robin + failover on rate limit/unavailable errors (`internal/shared/scanner/pool.go`)
- SSE event hub with subscribe/unsubscribe/broadcast, non-blocking slow client handling (`internal/shared/scanner/sse.go`)
- Scanner orchestrator: StartScan, StopScan, resume logic, token scanning, context cancellation (`internal/shared/scanner/scanner.go`)
- SetupScanner factory function wiring all providers and pools (`internal/shared/scanner/setup.go`)
- Balance DB operations: UpsertBalance, UpsertBalanceBatch, GetFundedAddresses, GetBalanceSummary, GetAddressesBatch
- Scan state DB operations: GetScanState, UpsertScanState, ShouldResume (24h resume threshold)
- Provider URL constants for all chains (mainnet + testnet)
- Solana program ID constants (TOKEN_PROGRAM_ID, ASSOCIATED_TOKEN_PROGRAM_ID)
- Scanner orchestrator constants (broadcast interval, request timeout, retry settings)
- Sentinel errors: ErrProviderUnavailable, ErrTokensNotSupported, ErrScanAlreadyRunning
- 56 new tests: scanner orchestrator (6), SSE hub (6), pool (4), BTC provider (5), BSC provider (4), ATA (5), balances DB (10), scans DB (8), and more

#### Fixed
- Scan state `started_at` preservation: `COALESCE(NULLIF(..., ''), ...)` to treat empty string as NULL

### 2026-02-18 (Phase 3)

#### Added
- Address listing API: `GET /api/addresses/{chain}` with pagination, hasBalance/token filters
- Address export API: `GET /api/addresses/{chain}/export` with streaming JSON download
- DB method `GetAddressesWithBalances` — paginated address+balance query with filter support
- DB method `hydrateBalances` — batch loads balance data for a page of addresses
- `AddressWithBalance` and `TokenBalanceItem` Go response types (`internal/shared/models/types.go`)
- Pagination constants: DefaultPage, DefaultPageSize, MaxPageSize (`internal/shared/config/constants.go`)
- Handler tests: 6 tests covering pagination, invalid chain, export, case insensitivity
- DB tests: 5 tests covering pagination, hasBalance filter, token filter, balance hydration, empty chain
- Frontend address explorer page with chain tabs, filter chips, paginated table
- AddressTable component: chain badges, truncated addresses, copy-to-clipboard, token balance rows
- Address store with reactive state: chain/page/filter switching triggers API refetch
- `getAddresses` and `exportAddresses` API client functions (`web/wallet/src/lib/utils/api.ts`)
- `formatRelativeTime` and `copyToClipboard` utilities (`web/wallet/src/lib/utils/formatting.ts`)
- Chain utilities: `getChainColor`, `getChainLabel`, `getTokenDecimals`, `getExplorerUrl` (`web/wallet/src/lib/utils/chains.ts`)

#### Changed
- Renamed `AddressBalance` to `AddressWithBalance` in frontend types (field `index` → `addressIndex` to match backend)

### 2026-02-18 (Phase 2)

#### Added
- BIP-39 mnemonic validation, seed derivation, file reading (`internal/wallet/hd/hd.go`)
- BTC Native SegWit (bech32) address derivation via BIP-84: `m/84'/0'/0'/0/N` (`internal/wallet/hd/btc.go`)
- BSC/EVM address derivation via BIP-44: `m/44'/60'/0'/0/N` with EIP-55 checksum (`internal/wallet/hd/bsc.go`)
- SOL address derivation via SLIP-10 ed25519: `m/44'/501'/N'/0'` — manual implementation, zero extra deps (`internal/wallet/hd/sol.go`)
- Address generator for bulk generation with progress callbacks (`internal/wallet/hd/generator.go`)
- JSON export with streaming (no OOM on 500K addresses) (`internal/wallet/hd/export.go`)
- `init` CLI command: generates 500K addresses per chain, batch inserts (10K/tx), idempotent
- `export` CLI command: exports addresses to `./data/export/*.json`
- DB address methods: InsertAddressBatch, CountAddresses, GetAddresses, StreamAddresses (`internal/wallet/db/addresses.go`)
- Domain types: NetworkMode, AllChains, AddressExport, AddressExportItem (`internal/shared/models/types.go`)
- BIP84Purpose constant for Native SegWit derivation (`internal/shared/config/constants.go`)
- 37 unit tests for wallet package with 83.3% coverage
- SLIP-10 spec test vectors verified (master key + child derivation)
- Known test vectors: BTC `bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu`, BSC `0x9858EfFD232B4033E47d90003D41EC34EcaEda94`, SOL `HAgk14JpMQLgt6rVgv7cBQFJWFto5Dqxi472uT3DKpqk`

#### Changed
- BTC derivation uses BIP-84 (purpose=84) instead of BIP-44 (purpose=44) for correct bech32 standard
- `cmd/wallet/main.go` now supports `init` and `export` subcommands

### 2026-02-18 (Phase 1)

#### Added
- Go module with full directory structure (cmd/, internal/)
- Central constants (`internal/shared/config/constants.go`) — all numeric/string values
- Central error codes (`internal/shared/config/errors.go`) — all error types
- Config loading via envconfig (`internal/shared/config/config.go`)
- Structured logging with dual output: stdout + daily rotated file (`internal/shared/logging/logger.go`)
- SQLite database with WAL mode, migration system, initial schema with 5 tables (`internal/wallet/db/`)
- Chi router with middleware stack: request logging, host check, CORS, CSRF (`internal/wallet/api/`)
- Health endpoint (`GET /api/health`)
- Server entry point with graceful shutdown (`cmd/wallet/main.go`)
- SvelteKit frontend with adapter-static, TypeScript strict mode, Tailwind CSS v4
- Design tokens from mockup phase ported to `app.css`
- Sidebar layout component matching mockup (240px, icons, active states, network badge)
- All route placeholders (Dashboard, Addresses, Scan, Send, Transactions, Settings)
- Frontend TypeScript types (`web/wallet/src/lib/types.ts`) and constants (`web/wallet/src/lib/constants.ts`)
- API client with CSRF token handling (`web/wallet/src/lib/utils/api.ts`)
- Formatting utilities (`web/wallet/src/lib/utils/formatting.ts`)
- Header component with title and optional actions slot
- Makefile with dev, build, test, lint targets
- `.env.example` with documented environment variables
- `.gitignore` for Go binary, data, logs, node_modules
- Unit tests for logging and database modules
