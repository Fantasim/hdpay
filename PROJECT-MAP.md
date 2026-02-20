# HDPay — Project Map

## Directory Tree

```
hdpay/
|-- CHANGELOG.md
|-- CLAUDE.md
|-- Makefile
|-- PROJECT-MAP.md
|-- cmd/
|   |-- wallet/
|   |   └-- main.go                     # Entry point: server, init, export commands
|   |-- poller/
|   |   └-- main.go                     # Entry point: poller service
|   └-- verify/
|       └-- main.go                     # Address verification utility
|-- go.mod
|-- go.sum
|-- internal/
|   |-- wallet/
|   |   |-- api/
|   |   |   |-- handlers/
|   |   |   |   |-- address.go           # GET /api/addresses/{chain}, GET .../export
|   |   |   |   |-- address_test.go
|   |   |   |   |-- dashboard.go         # GET /api/dashboard/prices, GET .../portfolio
|   |   |   |   |-- dashboard_test.go
|   |   |   |   |-- health.go            # GET /api/health
|   |   |   |   |-- provider_health.go   # GET /api/health/providers
|   |   |   |   |-- scan.go              # POST start/stop, GET status, GET SSE
|   |   |   |   |-- scan_test.go
|   |   |   |   |-- send.go              # POST preview/execute/gas-preseed, GET SSE
|   |   |   |   |-- send_test.go
|   |   |   |   |-- settings.go          # GET/PUT settings, reset-balances, reset-all
|   |   |   |   |-- settings_test.go
|   |   |   |   |-- transactions.go      # GET /api/transactions (filtered, paginated)
|   |   |   |   └-- transactions_test.go
|   |   |   |-- middleware/
|   |   |   |   |-- security.go          # CORS, CSRF, localhost-only
|   |   |   |   └-- security_test.go
|   |   |   └-- router.go               # Chi router setup
|   |   |-- db/
|   |   |   |-- addresses.go             # Address CRUD + GetAddressesWithBalances (filtered, paginated)
|   |   |   |-- addresses_test.go
|   |   |   |-- balances.go              # Balance CRUD, batch upsert, funded queries, aggregates
|   |   |   |-- balances_test.go
|   |   |   |-- migrations/
|   |   |   |   |-- 001_initial.sql      # Initial schema: 5 tables
|   |   |   |   |-- 005_tx_state.sql     # V2: TX state tracking table
|   |   |   |   └-- 006_provider_health.sql # V2: Provider health + circuit breaker table
|   |   |   |-- provider_health.go       # V2: Provider health CRUD
|   |   |   |-- provider_health_test.go
|   |   |   |-- scans.go                 # Scan state: GetScanState, UpsertScanState, ShouldResume
|   |   |   |-- scans_test.go
|   |   |   |-- sqlite.go               # SQLite connection, WAL mode, auto-migrations
|   |   |   |-- sqlite_test.go
|   |   |   |-- transactions.go          # Transaction CRUD: insert, update status, get, list
|   |   |   |-- transactions_test.go
|   |   |   |-- tx_state.go              # V2: TX state CRUD
|   |   |   └-- tx_state_test.go
|   |   |-- hd/
|   |   |   |-- bsc.go                   # BSC/EVM BIP-44 address derivation
|   |   |   |-- bsc_test.go
|   |   |   |-- btc.go                   # BTC BIP-84 Native SegWit bech32 derivation
|   |   |   |-- btc_test.go
|   |   |   |-- errors.go               # Wallet-specific errors
|   |   |   |-- export.go               # Streaming JSON export
|   |   |   |-- export_test.go
|   |   |   |-- generator.go            # Bulk address generation with progress callbacks
|   |   |   |-- generator_test.go
|   |   |   |-- hd.go                   # BIP-39 mnemonic validation, seed, master key
|   |   |   |-- hd_test.go
|   |   |   |-- sol.go                  # SOL SLIP-10 ed25519 derivation (manual)
|   |   |   └-- sol_test.go
|   |   └-- tx/
|   |       |-- broadcaster.go           # Shared Broadcaster interface + BTC implementation
|   |       |-- broadcaster_test.go
|   |       |-- bsc_fallback.go          # V2: FallbackEthClient (primary + secondary RPC)
|   |       |-- bsc_fallback_test.go
|   |       |-- bsc_tx.go               # BSC native BNB + BEP-20 TX building, signing, consolidation
|   |       |-- bsc_tx_test.go
|   |       |-- btc_fee.go              # Dynamic fee estimation from mempool.space
|   |       |-- btc_fee_test.go
|   |       |-- btc_tx.go               # Multi-input P2WPKH TX building, signing, consolidation
|   |       |-- btc_tx_test.go
|   |       |-- btc_utxo.go             # UTXO fetching with round-robin provider rotation
|   |       |-- btc_utxo_test.go
|   |       |-- gas.go                  # Gas pre-seeding service + idempotency + nonce gap handling
|   |       |-- gas_test.go
|   |       |-- key_service.go          # On-demand BTC/BSC private key derivation from mnemonic
|   |       |-- key_service_test.go
|   |       |-- sol_serialize.go        # Raw Solana binary TX serialization
|   |       |-- sol_serialize_test.go
|   |       |-- sol_tx.go               # SOL native + SPL token consolidation + ATA visibility polling
|   |       |-- sol_tx_test.go
|   |       |-- sse.go                  # TX SSE hub: subscribe/unsubscribe/broadcast (tx events)
|   |       |-- sse_test.go
|   |       └-- sweep.go               # V2: Sweep ID generator (crypto/rand)
|   |-- poller/
|   |   |-- api/
|   |   |   |-- handlers/
|   |   |   |   |-- admin.go
|   |   |   |   |-- auth.go
|   |   |   |   |-- dashboard.go
|   |   |   |   |-- handlers_test.go
|   |   |   |   |-- health.go
|   |   |   |   |-- points.go
|   |   |   |   └-- watch.go
|   |   |   |-- middleware/
|   |   |   |   |-- cors.go
|   |   |   |   |-- ipallow.go
|   |   |   |   |-- ipallow_test.go
|   |   |   |   |-- session.go
|   |   |   |   └-- session_test.go
|   |   |   └-- router.go
|   |   |-- config/
|   |   |   |-- config.go
|   |   |   |-- config_test.go
|   |   |   |-- constants.go
|   |   |   └-- errors.go
|   |   |-- httputil/
|   |   |   └-- response.go
|   |   |-- models/
|   |   |   └-- types.go
|   |   |-- points/
|   |   |   |-- calculator.go
|   |   |   |-- calculator_test.go
|   |   |   |-- pricer.go
|   |   |   |-- pricer_test.go
|   |   |   |-- tiers.go
|   |   |   └-- tiers_test.go
|   |   |-- pollerdb/
|   |   |   |-- allowlist.go
|   |   |   |-- allowlist_test.go
|   |   |   |-- dashboard.go
|   |   |   |-- dashboard_test.go
|   |   |   |-- db.go
|   |   |   |-- db_test.go
|   |   |   |-- discrepancy.go
|   |   |   |-- discrepancy_test.go
|   |   |   |-- errors.go
|   |   |   |-- errors_test.go
|   |   |   |-- migrations/
|   |   |   |   └-- 001_init.sql
|   |   |   |-- points.go
|   |   |   |-- points_test.go
|   |   |   |-- transactions.go
|   |   |   |-- transactions_test.go
|   |   |   |-- watches.go
|   |   |   └-- watches_test.go
|   |   |-- provider/
|   |   |   |-- bsc.go
|   |   |   |-- bsc_test.go
|   |   |   |-- btc.go
|   |   |   |-- btc_test.go
|   |   |   |-- provider.go
|   |   |   |-- provider_test.go
|   |   |   |-- sol.go
|   |   |   └-- sol_test.go
|   |   |-- validate/
|   |   |   |-- address.go
|   |   |   └-- address_test.go
|   |   └-- watcher/
|   |       |-- poll.go
|   |       |-- recovery.go
|   |       |-- testhelpers_test.go
|   |       |-- watcher.go
|   |       └-- watcher_test.go
|   └-- shared/
|       |-- config/
|       |   |-- config.go               # Config struct (envconfig)
|       |   |-- config_test.go
|       |   |-- constants.go            # ALL numeric/string constants
|       |   |-- errors.go               # ALL error codes + TransientError type
|       |   └-- errors_test.go
|       |-- httputil/
|       |   |-- spa.go                  # Embedded SPA handler with immutable cache
|       |   |-- spa_test.go
|       |   |-- logging.go             # Request/response logging middleware
|       |   └-- logging_test.go
|       |-- logging/
|       |   |-- logger.go              # slog: stdout + daily rotated files
|       |   └-- logger_test.go
|       |-- models/
|       |   └-- types.go               # Shared domain types
|       |-- price/
|       |   |-- coingecko.go           # CoinGecko price service with 5-min cache
|       |   └-- coingecko_test.go
|       └-- scanner/
|           |-- btc_blockstream.go      # Blockstream Esplora provider
|           |-- btc_blockstream_test.go
|           |-- btc_mempool.go          # Mempool.space provider
|           |-- btc_mempool_test.go
|           |-- bsc_bscscan.go          # BscScan REST API provider
|           |-- bsc_bscscan_test.go
|           |-- bsc_rpc.go              # BSC ethclient JSON-RPC provider
|           |-- bsc_rpc_test.go
|           |-- circuit_breaker.go      # V2: Circuit breaker (closed/open/half-open)
|           |-- circuit_breaker_test.go
|           |-- healthcheck.go          # Provider health check logic
|           |-- healthcheck_test.go
|           |-- pool.go                 # Provider pool: round-robin + failover
|           |-- pool_test.go
|           |-- provider.go             # Provider interface + BalanceResult
|           |-- ratelimiter.go          # Per-provider rate limiter (x/time/rate)
|           |-- ratelimiter_test.go
|           |-- retry_after.go          # Retry-After header parser
|           |-- retry_after_test.go
|           |-- scanner.go              # Scanner orchestrator: multi-chain, resume, token scan
|           |-- scanner_test.go
|           |-- setup.go                # Scanner factory + test helpers
|           |-- sol_ata.go              # Manual Solana ATA derivation via PDA
|           |-- sol_ata_test.go
|           |-- sol_rpc.go              # Solana JSON-RPC provider (batch 100)
|           |-- sol_rpc_test.go
|           |-- sse.go                  # SSE hub: subscribe/unsubscribe/broadcast
|           └-- sse_test.go
|-- web/
|   |-- wallet/
|   |   |-- embed.go                    # Go embed for wallet SvelteKit build
|   |   |-- src/
|   |   |   |-- app.css                 # Tailwind v4 + design tokens
|   |   |   |-- app.html                # HTML template
|   |   |   |-- lib/
|   |   |   |   |-- components/
|   |   |   |   |   |-- address/
|   |   |   |   |   |   └-- AddressTable.svelte  # Address table with badges, copy, tokens
|   |   |   |   |   |-- dashboard/
|   |   |   |   |   |   |-- PortfolioOverview.svelte   # Total value + 4 stat cards
|   |   |   |   |   |   |-- BalanceBreakdown.svelte    # Balance table by chain+token
|   |   |   |   |   |   └-- PortfolioCharts.svelte     # ECharts pie chart (chain distribution)
|   |   |   |   |   |-- layout/
|   |   |   |   |   |   |-- Header.svelte
|   |   |   |   |   |   └-- Sidebar.svelte
|   |   |   |   |   |-- scan/
|   |   |   |   |   |   |-- ScanControl.svelte   # Chain selector, max ID, start/stop
|   |   |   |   |   |   |-- ScanProgress.svelte  # Per-chain progress bars + stats
|   |   |   |   |   |   └-- ProviderStatus.svelte # Provider health grid
|   |   |   |   |   |-- send/
|   |   |   |   |   |   |-- SelectStep.svelte    # Step 1: chain/token/destination selection
|   |   |   |   |   |   |-- PreviewStep.svelte   # Step 2: TX summary, fees, funded addresses
|   |   |   |   |   |   |-- GasPreSeedStep.svelte # Step 3: BSC gas pre-seeding
|   |   |   |   |   |   └-- ExecuteStep.svelte   # Step 4: sweep execution + results
|   |   |   |   |   └-- ui/                      # shadcn-svelte components
|   |   |   |   |-- constants.ts         # Frontend constants
|   |   |   |   |-- stores/
|   |   |   |   |   |-- addresses.svelte.ts     # Reactive address store (chain, pagination, filters)
|   |   |   |   |   |-- addresses.svelte.test.ts
|   |   |   |   |   |-- scan.svelte.ts          # Scan store with SSE connection + reconnect
|   |   |   |   |   |-- scan.svelte.test.ts
|   |   |   |   |   |-- send.svelte.ts          # Send wizard store with SSE + step management
|   |   |   |   |   └-- send.svelte.test.ts
|   |   |   |   |-- types.ts             # TypeScript interfaces
|   |   |   |   └-- utils/
|   |   |   |       |-- api.ts           # API client with CSRF + all API functions
|   |   |   |       |-- api.test.ts
|   |   |   |       |-- chains.ts        # Chain color, label, explorer URL, token decimals
|   |   |   |       |-- chains.test.ts
|   |   |   |       |-- formatting.ts    # Number/address/time formatting + clipboard
|   |   |   |       |-- formatting.test.ts
|   |   |   |       |-- providers.ts     # Provider health helpers
|   |   |   |       |-- providers.test.ts
|   |   |   |       |-- validation.ts    # Chain-specific address validation
|   |   |   |       └-- validation.test.ts
|   |   |   └-- routes/
|   |   |       |-- +layout.svelte       # Root layout with Sidebar
|   |   |       |-- +layout.ts
|   |   |       |-- +page.svelte         # Dashboard with portfolio overview + charts
|   |   |       |-- addresses/+page.svelte  # Address explorer with tabs, filters, pagination
|   |   |       |-- scan/+page.svelte    # Scan page with SSE progress visualization
|   |   |       |-- send/+page.svelte    # Send wizard: 4-step stepper
|   |   |       |-- settings/+page.svelte
|   |   |       └-- transactions/+page.svelte
|   |   |-- svelte.config.js
|   |   |-- tsconfig.json
|   |   └-- vite.config.ts
|   └-- poller/
|       |-- embed.go                    # Go embed for poller SvelteKit build
|       |-- src/
|       |   |-- app.css
|       |   |-- app.html
|       |   |-- lib/
|       |   |   |-- components/
|       |   |   |   |-- charts/         # ECharts: chain breakdown, points, tiers, tokens, etc.
|       |   |   |   |-- dashboard/      # StatsCard, TimeRangeSelector
|       |   |   |   |-- layout/         # Header, Sidebar
|       |   |   |   └-- ui/            # shadcn-svelte: badge, button, card, input, label, select, separator
|       |   |   |-- constants.ts
|       |   |   |-- stores/
|       |   |   |   └-- auth.ts
|       |   |   |-- types.ts
|       |   |   └-- utils/
|       |   |       |-- api.ts
|       |   |       |-- explorer.ts
|       |   |       └-- formatting.ts
|       |   └-- routes/
|       |       |-- +layout.svelte
|       |       |-- +layout.ts
|       |       |-- +page.svelte         # Dashboard
|       |       |-- errors/+page.svelte
|       |       |-- login/+page.svelte
|       |       |-- points/+page.svelte
|       |       |-- settings/+page.svelte
|       |       |-- transactions/+page.svelte
|       |       └-- watches/+page.svelte
|       |-- svelte.config.js
|       |-- tsconfig.json
|       └-- vite.config.ts
```

## Key Files

| File | Purpose |
|------|---------|
| **Entry Points** | |
| `cmd/wallet/main.go` | Wallet entry point: `serve`, `init`, `export` subcommands + setupSendDeps |
| `cmd/poller/main.go` | Poller service entry point |
| `cmd/verify/main.go` | Address verification utility |
| **Shared Config** | |
| `internal/shared/config/constants.go` | ALL numeric/string constants (sacred -- no hardcoding) |
| `internal/shared/config/errors.go` | ALL error codes shared with frontend + TransientError type |
| `internal/shared/config/config.go` | Config struct loaded via envconfig |
| **Shared Infrastructure** | |
| `internal/shared/httputil/spa.go` | Embedded SPA handler with immutable cache headers |
| `internal/shared/httputil/logging.go` | Request/response logging middleware |
| `internal/shared/logging/logger.go` | slog: stdout + daily rotated files |
| `internal/shared/models/types.go` | Shared domain types: Chain, Address, ScanState, Token, Send types |
| `internal/shared/price/coingecko.go` | CoinGecko price service with 5-min cache + stale-but-serve |
| **Shared Scanner** | |
| `internal/shared/scanner/scanner.go` | Scanner orchestrator: multi-chain, resume, token scanning |
| `internal/shared/scanner/pool.go` | Provider pool with round-robin rotation + failover |
| `internal/shared/scanner/provider.go` | Provider interface + BalanceResult (with Error+Source fields) |
| `internal/shared/scanner/circuit_breaker.go` | V2: Circuit breaker state machine (closed/open/half-open) |
| `internal/shared/scanner/healthcheck.go` | Provider health check logic |
| `internal/shared/scanner/sse.go` | SSE hub for real-time scan progress broadcasting |
| `internal/shared/scanner/setup.go` | Scanner factory + test helpers |
| **Wallet DB** | |
| `internal/wallet/db/sqlite.go` | SQLite connection, WAL mode, auto-migrations |
| `internal/wallet/db/addresses.go` | Address CRUD + filtered paginated queries with balance hydration |
| `internal/wallet/db/balances.go` | Balance CRUD, batch upsert, funded queries, aggregates |
| `internal/wallet/db/scans.go` | Scan state persistence with resume support |
| `internal/wallet/db/transactions.go` | Transaction CRUD: insert, update status, get by ID/hash, paginated list |
| `internal/wallet/db/tx_state.go` | V2: TX lifecycle tracking CRUD |
| `internal/wallet/db/provider_health.go` | V2: Provider health CRUD |
| **Wallet HD Derivation** | |
| `internal/wallet/hd/hd.go` | BIP-39 mnemonic validation, seed derivation, master key |
| `internal/wallet/hd/btc.go` | BTC bech32 via BIP-84: `m/84'/0'/0'/0/N` |
| `internal/wallet/hd/bsc.go` | BSC EIP-55 via BIP-44: `m/44'/60'/0'/0/N` |
| `internal/wallet/hd/sol.go` | SOL via manual SLIP-10 ed25519: `m/44'/501'/N'/0'` |
| `internal/wallet/hd/generator.go` | Bulk generation with progress callbacks |
| `internal/wallet/hd/export.go` | Streaming JSON export |
| **Wallet API** | |
| `internal/wallet/api/router.go` | Chi router with middleware stack |
| `internal/wallet/api/handlers/address.go` | Address list + export handlers with validation/logging |
| `internal/wallet/api/handlers/scan.go` | Scan start/stop/status handlers + SSE streaming |
| `internal/wallet/api/handlers/dashboard.go` | Prices + portfolio API handlers |
| `internal/wallet/api/handlers/send.go` | Send preview/execute/gas-preseed/resume handlers + SSE + chain dispatch |
| `internal/wallet/api/handlers/provider_health.go` | V2: Provider health endpoint -- GET /api/health/providers |
| `internal/wallet/api/handlers/transactions.go` | Transaction list handler (filtered, paginated) |
| `internal/wallet/api/handlers/settings.go` | Settings GET/PUT + reset-balances + reset-all |
| **Wallet TX** | |
| `internal/wallet/tx/key_service.go` | On-demand BTC/BSC private key derivation from mnemonic file |
| `internal/wallet/tx/btc_utxo.go` | UTXO fetching with round-robin Blockstream/Mempool rotation |
| `internal/wallet/tx/btc_fee.go` | Dynamic fee estimation from mempool.space with fallback |
| `internal/wallet/tx/btc_tx.go` | Multi-input P2WPKH TX building, signing, consolidation + confirmation polling |
| `internal/wallet/tx/broadcaster.go` | Shared Broadcaster interface + BTC broadcast with provider fallback |
| `internal/wallet/tx/bsc_tx.go` | BSC native BNB + BEP-20 TX building, EIP-155 signing, consolidation |
| `internal/wallet/tx/gas.go` | Gas pre-seeding service: distribute BNB + idempotency + nonce gap handling |
| `internal/wallet/tx/bsc_fallback.go` | V2: FallbackEthClient -- primary RPC with Ankr fallback |
| `internal/wallet/tx/sol_tx.go` | SOL native + SPL token consolidation + ATA visibility polling |
| `internal/wallet/tx/sol_serialize.go` | Raw Solana binary TX serialization |
| `internal/wallet/tx/sweep.go` | V2: Sweep ID generator (crypto/rand) |
| `internal/wallet/tx/sse.go` | TX SSE hub for real-time transaction status broadcasting |
| **Wallet Frontend** | |
| `web/wallet/embed.go` | Go embed directive for wallet SvelteKit build |
| `web/wallet/src/lib/types.ts` | ALL TypeScript interfaces |
| `web/wallet/src/lib/constants.ts` | ALL frontend constants |
| `web/wallet/src/lib/utils/api.ts` | API client (CSRF) + all backend API functions |
| `web/wallet/src/lib/utils/validation.ts` | Chain-specific address validation (BTC, BSC, SOL) |
| `web/wallet/src/lib/utils/chains.ts` | Chain metadata: colors, labels, explorer URLs, token decimals |
| `web/wallet/src/lib/stores/scan.svelte.ts` | Scan store with SSE lifecycle + exponential backoff |
| `web/wallet/src/lib/stores/send.svelte.ts` | Send wizard store with SSE + step management |
| `web/wallet/src/routes/+page.svelte` | Dashboard with portfolio overview + ECharts |
| `web/wallet/src/routes/scan/+page.svelte` | Scan page with real-time progress |
| `web/wallet/src/routes/send/+page.svelte` | Send wizard with 4-step stepper |
| **Poller Frontend** | |
| `web/poller/embed.go` | Go embed directive for poller SvelteKit build |
| `web/poller/src/lib/types.ts` | Poller TypeScript interfaces |
| `web/poller/src/lib/constants.ts` | Poller frontend constants |
| `web/poller/src/lib/utils/api.ts` | Poller API client |

## Module Dependencies

### Go
| Module | Purpose |
|--------|---------|
| `github.com/go-chi/chi/v5` | HTTP router |
| `modernc.org/sqlite` | Pure-Go SQLite driver |
| `github.com/kelseyhightower/envconfig` | Config from env vars |
| `github.com/tyler-smith/go-bip39` | BIP-39 mnemonic/seed |
| `github.com/btcsuite/btcd` | BTC BIP-32 key derivation, bech32 |
| `github.com/ethereum/go-ethereum` | BSC/EVM address derivation + ethclient |
| `github.com/mr-tron/base58` | SOL base58 encoding |
| `golang.org/x/time` | Rate limiting (token bucket) |

### Frontend (npm)
| Package | Purpose |
|---------|---------|
| `svelte` / `@sveltejs/kit` | UI framework |
| `@sveltejs/adapter-static` | Static site generation |
| `tailwindcss` / `@tailwindcss/vite` | CSS utility framework |
| `echarts` / `svelte-echarts` | Charts (portfolio pie chart, poller dashboards) |
| `@tanstack/svelte-virtual` | Table virtualization (installed, ready for use) |
| `vitest` | Frontend unit testing |
| `typescript` | Type safety |

## API Endpoints

### Wallet API

| Method | Path | Status | Handler |
|--------|------|--------|---------|
| GET | `/api/health` | Implemented | `internal/wallet/api/handlers/health.go` |
| GET | `/api/health/providers` | Implemented | `internal/wallet/api/handlers/provider_health.go` |
| GET | `/api/addresses/{chain}` | Implemented | `internal/wallet/api/handlers/address.go` |
| GET | `/api/addresses/{chain}/export` | Implemented | `internal/wallet/api/handlers/address.go` |
| POST | `/api/scan/start` | Implemented | `internal/wallet/api/handlers/scan.go` |
| POST | `/api/scan/stop` | Implemented | `internal/wallet/api/handlers/scan.go` |
| GET | `/api/scan/status` | Implemented | `internal/wallet/api/handlers/scan.go` |
| GET | `/api/scan/sse` | Implemented | `internal/wallet/api/handlers/scan.go` |
| GET | `/api/dashboard/prices` | Implemented | `internal/wallet/api/handlers/dashboard.go` |
| GET | `/api/dashboard/portfolio` | Implemented | `internal/wallet/api/handlers/dashboard.go` |
| POST | `/api/send/preview` | Implemented | `internal/wallet/api/handlers/send.go` |
| POST | `/api/send/execute` | Implemented | `internal/wallet/api/handlers/send.go` |
| POST | `/api/send/gas-preseed` | Implemented | `internal/wallet/api/handlers/send.go` |
| GET | `/api/send/sse` | Implemented | `internal/wallet/api/handlers/send.go` |
| GET | `/api/send/pending` | Implemented | `internal/wallet/api/handlers/send.go` |
| POST | `/api/send/dismiss/{id}` | Implemented | `internal/wallet/api/handlers/send.go` |
| GET | `/api/send/resume/{sweepID}` | Implemented | `internal/wallet/api/handlers/send.go` |
| POST | `/api/send/resume` | Implemented | `internal/wallet/api/handlers/send.go` |
| GET | `/api/transactions` | Implemented | `internal/wallet/api/handlers/transactions.go` |
| GET | `/api/settings` | Implemented | `internal/wallet/api/handlers/settings.go` |
| PUT | `/api/settings` | Implemented | `internal/wallet/api/handlers/settings.go` |
| POST | `/api/settings/reset-balances` | Implemented | `internal/wallet/api/handlers/settings.go` |
| POST | `/api/settings/reset-all` | Implemented | `internal/wallet/api/handlers/settings.go` |

### Poller API

| Method | Path | Status | Handler |
|--------|------|--------|---------|
| GET | `/api/health` | Implemented | `internal/poller/api/handlers/health.go` |
| POST | `/api/auth/login` | Implemented | `internal/poller/api/handlers/auth.go` |
| POST | `/api/auth/logout` | Implemented | `internal/poller/api/handlers/auth.go` |
| GET | `/api/watches` | Implemented | `internal/poller/api/handlers/watch.go` |
| POST | `/api/watches` | Implemented | `internal/poller/api/handlers/watch.go` |
| DELETE | `/api/watches/{id}` | Implemented | `internal/poller/api/handlers/watch.go` |
| GET | `/api/transactions` | Implemented | `internal/poller/api/handlers/watch.go` |
| GET | `/api/points` | Implemented | `internal/poller/api/handlers/points.go` |
| GET | `/api/dashboard` | Implemented | `internal/poller/api/handlers/dashboard.go` |
| GET | `/api/admin/allowlist` | Implemented | `internal/poller/api/handlers/admin.go` |
| POST | `/api/admin/allowlist` | Implemented | `internal/poller/api/handlers/admin.go` |
| DELETE | `/api/admin/allowlist/{ip}` | Implemented | `internal/poller/api/handlers/admin.go` |

## Database Schema

### Wallet Database

| Table | Migration | Purpose | Key Columns |
|-------|-----------|---------|-------------|
| `addresses` | 001 | HD wallet addresses | chain, address_index (PK), address |
| `balances` | 001 | Latest scanned balances | chain, address_index, token, balance, last_scanned |
| `transactions` | 001 | Transaction history | chain, tx_hash, status |
| `scan_state` | 001 | Scan state for resume | chain, last_scanned_index, updated_at |
| `settings` | 001 | User settings | key, value |
| `tx_state` | 005 | V2: TX lifecycle tracking | id (PK), sweep_id, chain, token, status, tx_hash, nonce |
| `provider_health` | 006 | V2: Provider health + circuit breaker | provider_name (PK), chain, status, circuit_state, consecutive_fails |

### Poller Database

| Table | Migration | Purpose | Key Columns |
|-------|-----------|---------|-------------|
| `watches` | 001 | Watched addresses | id (PK), chain, address, label, created_at |
| `transactions` | 001 | Detected transactions | id (PK), watch_id, chain, tx_hash, amount, token |
| `points` | 001 | Points per watch | id (PK), watch_id, points, reason |
| `allowlist` | 001 | IP allowlist for access | ip (PK), created_at |
| `discrepancies` | 001 | Balance discrepancy tracking | id (PK), watch_id, expected, actual |
