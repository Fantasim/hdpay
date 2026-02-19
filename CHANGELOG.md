# Changelog

## [Unreleased]

### 2026-02-18 (V2 Phase 4) — TX Safety — Advanced

#### Added
- BTC UTXO re-validation at execute time (A6): `ValidateUTXOsAgainstPreview()` — rejects if UTXO count drops >20% or value drops >10%
- BSC on-chain balance recheck (A7): `BalanceOfBEP20()` for BEP-20 token balance verification via `eth_call`
- `bep20BalanceOfSelector` and `bscMinNativeSweepWei` package-level vars for BSC balance operations
- Partial sweep resume endpoints (A8): `GET /api/send/resume/{sweepID}` (summary) and `POST /api/send/resume` (retry failed/uncertain)
- `GetRetryableTxStates()`, `GetSweepMeta()`, `HasConfirmedTxForAddress()` DB methods in `tx_state.go`
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
- Retry-After header parser (`internal/scanner/retry_after.go`) — handles seconds and HTTP-date formats, 9 tests
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
- Scanner orchestrator (`scanner.go`) rewritten for V2: atomic DB writes, decoupled native/token, backoff loop, token error SSE
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
- Circuit breaker state machine (closed/open/half-open) in `internal/scanner/circuit_breaker.go`
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
- Embedded SPA serving via `go:embed all:build` in `web/embed.go`
- `SPAHandler` with immutable cache headers for `_app/` assets and SPA fallback to index.html
- 40 new tests: DB settings (8), DB transaction filters (5), handler settings (9), handler transactions (13), SPA handler (5)

#### Changed
- `NewRouter` now accepts optional `fs.FS` for embedded SPA serving
- `cmd/server/main.go` imports `web` package and passes embedded FS to router
- Router uses `r.NotFound(SPAHandler)` catch-all for client-side routing after `/api` routes
- `make build` produces 22MB single binary with embedded SPA

### 2026-02-18 (Phase 10)

#### Added
- Unified send API: `POST /api/send/preview`, `POST /api/send/execute`, `POST /api/send/gas-preseed` dispatching to chain-specific TX engines
- TX SSE hub for real-time transaction status streaming (`internal/tx/sse.go`)
- Send handler with chain-specific preview/execute dispatch (`internal/api/handlers/send.go`)
- SendDeps dependency injection struct wiring all TX services
- `setupSendDeps` in main.go initializing all TX services from config
- Unified send types: SendRequest, UnifiedSendPreview, UnifiedSendResult, TxResult, FundedAddressInfo
- `GetFundedAddressesJoined` DB query — JOIN between addresses and balances tables
- Frontend address validation: BTC bech32/legacy, BSC hex, SOL base58 (`web/src/lib/utils/validation.ts`)
- Send wizard store with Svelte 5 runes and SSE integration (`web/src/lib/stores/send.svelte.ts`)
- 4-step wizard UI: SelectStep, PreviewStep, GasPreSeedStep, ExecuteStep components
- Stepper component with collapsed completed-step summaries
- `getExplorerTxUrl` convenience function in chains.ts
- Backend tests: 24 tests for validateDestination + isValidToken (`internal/api/handlers/send_test.go`)
- Frontend tests: 23 vitest tests for address validation (`web/src/lib/utils/validation.test.ts`)
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
- Raw Solana binary transaction serialization: compact-u16 encoding, message headers, compiled instructions, ed25519 signing (`internal/tx/sol_serialize.go`)
- SystemProgram.Transfer instruction builder for native SOL transfers
- SPL Token.Transfer instruction builder for USDC/USDT token transfers
- CreateAssociatedTokenAccount instruction builder for auto-ATA creation
- SOL private key derivation via SLIP-10 ed25519 path m/44'/501'/N'/0' (`internal/tx/key_service.go`)
- SOL JSON-RPC client with round-robin URL selection: getLatestBlockhash, sendTransaction, getSignatureStatuses, getAccountInfo, getBalance (`internal/tx/sol_tx.go`)
- Confirmation polling with configurable timeout and "confirmed" commitment level
- SOLConsolidationService: sequential per-address native SOL sweep + SPL token sweep with auto-ATA creation
- Preview methods for native and token sweeps (dry-run cost/balance calculation)
- SOL model types: `SOLSendPreview`, `SOLSendResult`, `SOLTxResult` (`internal/models/types.go`)
- SOL transaction constants: lamports, fees, TX size limit, confirmation timing, program IDs (`internal/config/constants.go`)
- 5 new sentinel errors + 5 error codes for SOL-specific failures (`internal/config/errors.go`)
- 29 new tests: serialization (13), consolidation service (14), key derivation (2)

### 2026-02-18 (Phase 8)

#### Added
- BSC private key derivation: BIP-44 m/44'/60'/0'/0/N → ECDSA key + EIP-55 address (`internal/tx/key_service.go`)
- Native BNB transfer engine: LegacyTx building, EIP-155 signing (chain ID 56/97), 20% gas price buffer (`internal/tx/bsc_tx.go`)
- BEP-20 token transfer: Manual ABI encoding of `transfer(address,uint256)` with selector `0xa9059cbb`
- Receipt polling: `WaitForReceipt` with `ethereum.NotFound` detection, revert handling, configurable timeout
- BSC Consolidation Service: Sequential per-address sweep for native BNB + BEP-20 tokens with real-time balance checks
- Gas pre-seeding service: Distributes 0.005 BNB to targets needing gas, sequential nonce management (`internal/tx/gas.go`)
- EthClientWrapper interface for testability (PendingNonceAt, SuggestGasPrice, SendTransaction, TransactionReceipt, BalanceAt)
- BSC model types: `BSCSendPreview`, `BSCSendResult`, `BSCTxResult`, `GasPreSeedPreview`, `GasPreSeedResult`
- BSC constants: chain IDs, gas price buffer, receipt polling interval/timeout, BEP-20 selector
- 5 new sentinel errors + 4 new error codes for BSC-specific failures
- 22 new tests: BSC TX engine (18), gas pre-seed (4), BSC key derivation (4)

### 2026-02-18 (Phase 7)

#### Added
- On-demand BIP-84 private key derivation service — reads mnemonic from file, derives keys per index, caller zeros after use (`internal/tx/key_service.go`)
- UTXO fetcher — confirmed UTXOs from Blockstream/Mempool APIs with round-robin rotation and rate limiting (`internal/tx/btc_utxo.go`)
- Dynamic fee estimator — fetches from mempool.space `/v1/fees/recommended` with fallback to config constant (`internal/tx/btc_fee.go`)
- Multi-input P2WPKH transaction builder — vsize estimation, wire.MsgTx construction, consolidation to single output (`internal/tx/btc_tx.go`)
- P2WPKH witness signer — `MultiPrevOutFetcher` + `NewTxSigHashes` (once) + `WitnessSignature` per input
- BTC broadcaster — POST raw hex as `text/plain` with ordered provider fallback, no retry on 400 (`internal/tx/broadcaster.go`)
- Transaction DB CRUD — insert, update status, get by ID/hash, paginated list with chain filter (`internal/db/transactions.go`)
- Shared `Broadcaster` interface for future BSC/SOL reuse
- `BTCConsolidationService` orchestrator with `Preview()` and `Execute()` methods
- Domain types: `UTXO`, `FeeEstimate`, `SendPreview`, `SendResult` (`internal/models/types.go`)
- BTC TX constants: dust threshold, vsize weights, fee estimation timeout, max inputs (`internal/config/constants.go`)
- 7 new sentinel errors + 4 new error codes (`internal/config/errors.go`)
- 38 new tests: key service (7), UTXO fetcher (7), fee estimator (4), TX builder (10), broadcaster (4), transaction DB (7)

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
