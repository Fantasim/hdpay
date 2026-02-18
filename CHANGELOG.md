# Changelog

## [Unreleased]

### 2026-02-18 (Phase 6)

#### Added
- CoinGecko price service with 5-minute in-memory cache, thread-safe (`internal/price/coingecko.go`)
- `GET /api/dashboard/prices` — returns USD prices keyed by symbol (BTC, BNB, SOL, USDC, USDT)
- `GET /api/dashboard/portfolio` — aggregated balances with USD values, address counts, last scan time
- Balance aggregation DB queries: `GetBalanceAggregates`, `GetLatestScanTime` (`internal/db/balances.go`)
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
- Scan API handlers: `POST /api/scan/start`, `POST /api/scan/stop`, `GET /api/scan/status`, `GET /api/scan/sse` (`internal/api/handlers/scan.go`)
- SSE streaming handler with keepalive ticker and hub subscribe/unsubscribe lifecycle
- Scanner wired into `main.go`: SSEHub creation, SetupScanner, hub.Run goroutine
- Scan routes added to Chi router (`internal/api/router.go`)
- Exported test helpers: `NewPoolForTest`, `testProvider` in `internal/scanner/setup.go`
- 11 scan handler tests covering start/stop/status/SSE endpoints (`scan_test.go`)
- Frontend scan store with SSE connection management (`web/src/lib/stores/scan.svelte.ts`)
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
- BTC providers: Blockstream Esplora, Mempool.space (`internal/scanner/btc_blockstream.go`, `btc_mempool.go`)
- BSC providers: BscScan REST API (batch 20 native, single token), ethclient JSON-RPC (`bsc_bscscan.go`, `bsc_rpc.go`)
- SOL provider: Solana JSON-RPC with `getMultipleAccounts` batch 100 (`sol_rpc.go`)
- Manual Solana ATA derivation via PDA + Edwards25519 on-curve check (`sol_ata.go`)
- Provider interface + BalanceResult type (`internal/scanner/provider.go`)
- Per-provider rate limiter using `golang.org/x/time/rate` (`ratelimiter.go`)
- Provider pool with round-robin + failover on rate limit/unavailable errors (`pool.go`)
- SSE event hub with subscribe/unsubscribe/broadcast, non-blocking slow client handling (`sse.go`)
- Scanner orchestrator: StartScan, StopScan, resume logic, token scanning, context cancellation (`scanner.go`)
- SetupScanner factory function wiring all providers and pools (`setup.go`)
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
- `AddressWithBalance` and `TokenBalanceItem` Go response types (`internal/models/types.go`)
- Pagination constants: DefaultPage, DefaultPageSize, MaxPageSize (`internal/config/constants.go`)
- Handler tests: 6 tests covering pagination, invalid chain, export, case insensitivity
- DB tests: 5 tests covering pagination, hasBalance filter, token filter, balance hydration, empty chain
- Frontend address explorer page with chain tabs, filter chips, paginated table
- AddressTable component: chain badges, truncated addresses, copy-to-clipboard, token balance rows
- Address store with reactive state: chain/page/filter switching triggers API refetch
- `getAddresses` and `exportAddresses` API client functions (`lib/utils/api.ts`)
- `formatRelativeTime` and `copyToClipboard` utilities (`lib/utils/formatting.ts`)
- Chain utilities: `getChainColor`, `getChainLabel`, `getTokenDecimals`, `getExplorerUrl` (`lib/utils/chains.ts`)

#### Changed
- Renamed `AddressBalance` to `AddressWithBalance` in frontend types (field `index` → `addressIndex` to match backend)

### 2026-02-18 (Phase 2)

#### Added
- BIP-39 mnemonic validation, seed derivation, file reading (`internal/wallet/hd.go`)
- BTC Native SegWit (bech32) address derivation via BIP-84: `m/84'/0'/0'/0/N` (`internal/wallet/btc.go`)
- BSC/EVM address derivation via BIP-44: `m/44'/60'/0'/0/N` with EIP-55 checksum (`internal/wallet/bsc.go`)
- SOL address derivation via SLIP-10 ed25519: `m/44'/501'/N'/0'` — manual implementation, zero extra deps (`internal/wallet/sol.go`)
- Address generator for bulk generation with progress callbacks (`internal/wallet/generator.go`)
- JSON export with streaming (no OOM on 500K addresses) (`internal/wallet/export.go`)
- `init` CLI command: generates 500K addresses per chain, batch inserts (10K/tx), idempotent
- `export` CLI command: exports addresses to `./data/export/*.json`
- DB address methods: InsertAddressBatch, CountAddresses, GetAddresses, StreamAddresses (`internal/db/addresses.go`)
- Domain types: NetworkMode, AllChains, AddressExport, AddressExportItem (`internal/models/types.go`)
- BIP84Purpose constant for Native SegWit derivation (`internal/config/constants.go`)
- 37 unit tests for wallet package with 83.3% coverage
- SLIP-10 spec test vectors verified (master key + child derivation)
- Known test vectors: BTC `bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu`, BSC `0x9858EfFD232B4033E47d90003D41EC34EcaEda94`, SOL `HAgk14JpMQLgt6rVgv7cBQFJWFto5Dqxi472uT3DKpqk`

#### Changed
- BTC derivation uses BIP-84 (purpose=84) instead of BIP-44 (purpose=44) for correct bech32 standard
- `cmd/server/main.go` now supports `init` and `export` subcommands

### 2026-02-18 (Phase 1)

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
