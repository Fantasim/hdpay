# HDPay — Project Map

## Directory Tree

```
hdpay/
|-- CHANGELOG.md
|-- CLAUDE.md
|-- Makefile
|-- cmd/
|   └-- server/
|       └-- main.go                  # Entry point: server, init, export commands
|-- go.mod
|-- go.sum
|-- internal/
|   |-- api/
|   |   |-- handlers/
|   |   |   |-- address.go           # GET /api/addresses/{chain}, GET .../export
|   |   |   |-- address_test.go      # Address handler tests (6 tests)
|   |   |   |-- dashboard.go         # GET /api/dashboard/prices, GET .../portfolio
|   |   |   |-- dashboard_test.go    # Dashboard handler tests (3 tests)
|   |   |   |-- health.go            # GET /api/health
|   |   |   |-- scan.go              # POST start/stop, GET status, GET SSE
|   |   |   |-- scan_test.go         # Scan handler tests (11 tests)
|   |   |   |-- send.go              # POST preview/execute/gas-preseed, GET SSE
|   |   |   |-- send_test.go         # Send handler tests (24 tests)
|   |   |   └-- settings.go          # Settings endpoints (stub)
|   |   |-- middleware/
|   |   |   |-- logging.go           # Request/response logging
|   |   |   └-- security.go          # CORS, CSRF, localhost-only
|   |   └-- router.go                # Chi router setup
|   |-- config/
|   |   |-- config.go                # Config struct (envconfig)
|   |   |-- constants.go             # ALL numeric/string constants + V2 circuit breaker/TX state/provider health
|   |   |-- errors.go                # ALL error codes + TransientError type + IsTransient/GetRetryAfter
|   |   └-- errors_test.go           # TransientError tests (4 tests)
|   |-- db/
|   |   |-- addresses.go             # Address CRUD + GetAddressesWithBalances (filtered, paginated)
|   |   |-- addresses_test.go        # Address DB tests (5 tests)
|   |   |-- balances.go              # Balance CRUD + batch upsert, funded queries, aggregates, GetFundedAddressesJoined
|   |   |-- balances_test.go         # Balance DB tests (10 tests)
|   |   |-- migrations/
|   |   |   |-- 001_initial.sql      # Initial schema: 5 tables
|   |   |   |-- 005_tx_state.sql     # V2: TX state tracking table
|   |   |   └-- 006_provider_health.sql # V2: Provider health + circuit breaker table
|   |   |-- provider_health.go       # V2: Provider health CRUD (6 methods)
|   |   |-- provider_health_test.go  # Provider health tests (6 tests)
|   |   |-- scans.go                 # Scan state: GetScanState, UpsertScanState, ShouldResume
|   |   |-- scans_test.go            # Scan state DB tests (8 tests)
|   |   |-- sqlite.go                # SQLite connection, WAL mode, migrations
|   |   |-- sqlite_test.go           # DB tests
|   |   |-- transactions.go          # Transaction CRUD: insert, update status, get, list
|   |   |-- transactions_test.go     # Transaction DB tests (7 tests)
|   |   |-- tx_state.go              # V2: TX state CRUD (6 methods)
|   |   └-- tx_state_test.go         # TX state tests (8 tests)
|   |-- logging/
|   |   |-- logger.go                # slog: stdout + daily rotated files
|   |   └-- logger_test.go           # Logger tests
|   |-- models/
|   |   └-- types.go                 # Domain types: Chain, Address, ScanState, Token, Send types
|   |-- price/
|   |   |-- coingecko.go             # CoinGecko price service with 5-min cache
|   |   └-- coingecko_test.go        # Price service tests (6 tests)
|   |-- scanner/
|   |   |-- btc_blockstream.go       # Blockstream Esplora provider
|   |   |-- btc_blockstream_test.go
|   |   |-- btc_mempool.go           # Mempool.space provider
|   |   |-- bsc_bscscan.go           # BscScan REST API provider
|   |   |-- bsc_bscscan_test.go
|   |   |-- bsc_rpc.go               # BSC ethclient JSON-RPC provider
|   |   |-- circuit_breaker.go       # V2: Circuit breaker state machine (closed/open/half-open)
|   |   |-- circuit_breaker_test.go  # Circuit breaker tests (8 tests)
|   |   |-- pool.go                  # Provider pool: round-robin + failover
|   |   |-- pool_test.go
|   |   |-- provider.go              # Provider interface + BalanceResult (with Error+Source fields)
|   |   |-- ratelimiter.go           # Per-provider rate limiter (x/time/rate)
|   |   |-- scanner.go               # Scanner orchestrator: multi-chain, resume, token scan
|   |   |-- scanner_test.go
|   |   |-- setup.go                 # Scanner factory + test helpers
|   |   |-- sol_ata.go               # Manual Solana ATA derivation via PDA
|   |   |-- sol_ata_test.go
|   |   |-- sol_rpc.go               # Solana JSON-RPC provider (batch 100)
|   |   |-- sse.go                   # SSE hub: subscribe/unsubscribe/broadcast (scan events)
|   |   └-- sse_test.go
|   |-- tx/
|   |   |-- broadcaster.go           # Shared Broadcaster interface + BTC implementation
|   |   |-- broadcaster_test.go      # Broadcaster tests (4 tests)
|   |   |-- bsc_tx.go                # BSC native BNB + BEP-20 TX building, signing, consolidation
|   |   |-- bsc_tx_test.go           # BSC TX tests (18 tests)
|   |   |-- btc_fee.go               # Dynamic fee estimation from mempool.space
|   |   |-- btc_fee_test.go          # Fee estimator tests (4 tests)
|   |   |-- btc_tx.go                # Multi-input P2WPKH TX building, signing, consolidation
|   |   |-- btc_tx_test.go           # TX builder tests (10 tests)
|   |   |-- btc_utxo.go              # UTXO fetching with round-robin provider rotation
|   |   |-- btc_utxo_test.go         # UTXO fetcher tests (7 tests)
|   |   |-- gas.go                   # Gas pre-seeding service (BSC BNB distribution)
|   |   |-- gas_test.go              # Gas pre-seed tests (4 tests)
|   |   |-- key_service.go           # On-demand BTC/BSC private key derivation from mnemonic
|   |   |-- key_service_test.go      # Key service tests (11 tests)
|   |   |-- sol_serialize.go         # Raw Solana binary TX serialization
|   |   |-- sol_serialize_test.go    # Serialization tests
|   |   |-- sol_tx.go                # SOL native + SPL token consolidation service
|   |   |-- sol_tx_test.go           # SOL TX tests
|   |   |-- sse.go                   # TX SSE hub: subscribe/unsubscribe/broadcast (tx events)
|   |   └-- sweep.go                 # V2: Sweep ID generator (crypto/rand)
|   └-- wallet/
|       |-- bsc.go                   # BSC/EVM BIP-44 address derivation
|       |-- bsc_test.go              # BSC tests with known vectors
|       |-- btc.go                   # BTC BIP-84 Native SegWit bech32 derivation
|       |-- btc_test.go              # BTC tests with known vectors
|       |-- errors.go                # Wallet-specific errors
|       |-- export.go                # Streaming JSON export
|       |-- export_test.go           # Export tests
|       |-- generator.go             # Bulk address generation with progress callbacks
|       |-- generator_test.go        # Generator tests
|       |-- hd.go                    # BIP-39 mnemonic, seed, master key
|       |-- hd_test.go               # Mnemonic & master key tests
|       |-- sol.go                   # SOL SLIP-10 ed25519 derivation (manual)
|       └-- sol_test.go              # SOL tests + SLIP-10 spec vectors
|-- web/
|   |-- src/
|   |   |-- app.css                  # Tailwind v4 + design tokens
|   |   |-- app.html                 # HTML template
|   |   |-- lib/
|   |   |   |-- components/
|   |   |   |   |-- address/
|   |   |   |   |   └-- AddressTable.svelte  # Address table with badges, copy, tokens
|   |   |   |   |-- dashboard/
|   |   |   |   |   |-- PortfolioOverview.svelte   # Total value + 4 stat cards
|   |   |   |   |   |-- BalanceBreakdown.svelte    # Balance table by chain+token
|   |   |   |   |   └-- PortfolioCharts.svelte     # ECharts pie chart (chain distribution)
|   |   |   |   |-- layout/          # Sidebar, Header
|   |   |   |   |-- scan/
|   |   |   |   |   |-- ScanControl.svelte   # Chain selector, max ID, start/stop
|   |   |   |   |   |-- ScanProgress.svelte  # Per-chain progress bars + stats
|   |   |   |   |   └-- ProviderStatus.svelte # Provider health grid
|   |   |   |   └-- send/
|   |   |   |       |-- SelectStep.svelte    # Step 1: chain/token/destination selection
|   |   |   |       |-- PreviewStep.svelte   # Step 2: TX summary, fees, funded addresses
|   |   |   |       |-- GasPreSeedStep.svelte # Step 3: BSC gas pre-seeding
|   |   |   |       └-- ExecuteStep.svelte   # Step 4: sweep execution + results
|   |   |   |-- constants.ts         # Frontend constants
|   |   |   |-- stores/
|   |   |   |   |-- addresses.ts     # Reactive address store (chain, pagination, filters)
|   |   |   |   |-- scan.svelte.ts   # Scan store with SSE connection + reconnect
|   |   |   |   └-- send.svelte.ts   # Send wizard store with SSE + step management
|   |   |   |-- types.ts             # TypeScript interfaces
|   |   |   └-- utils/
|   |   |       |-- api.ts           # API client with CSRF + all API functions
|   |   |       |-- chains.ts        # Chain color, label, explorer URL, token decimals
|   |   |       |-- formatting.ts    # Number/address/time formatting + clipboard
|   |   |       |-- validation.ts    # Chain-specific address validation
|   |   |       └-- validation.test.ts # Vitest address validation tests (23 tests)
|   |   └-- routes/
|   |       |-- +layout.svelte       # Root layout with Sidebar
|   |       |-- +page.svelte         # Dashboard with portfolio overview + charts
|   |       |-- addresses/+page.svelte  # Address explorer with tabs, filters, pagination
|   |       |-- scan/+page.svelte    # Scan page with SSE progress visualization
|   |       |-- send/+page.svelte    # Send wizard: 4-step stepper with collapsed summaries
|   |       |-- settings/+page.svelte
|   |       └-- transactions/+page.svelte
|   |-- svelte.config.js
|   |-- tsconfig.json
|   └-- vite.config.ts
```

## Key Files

| File | Purpose |
|------|---------|
| `cmd/server/main.go` | Entry point: `serve`, `init`, `export` subcommands + setupSendDeps |
| `internal/config/constants.go` | ALL numeric/string constants (sacred — no hardcoding) |
| `internal/config/errors.go` | ALL error codes shared with frontend |
| `internal/config/config.go` | Config struct loaded via envconfig |
| `internal/db/sqlite.go` | SQLite connection, WAL mode, auto-migrations |
| `internal/db/addresses.go` | Address CRUD + filtered paginated queries with balance hydration |
| `internal/db/balances.go` | Balance CRUD, batch upsert, funded queries, aggregates, GetFundedAddressesJoined |
| `internal/db/scans.go` | Scan state persistence with resume support |
| `internal/db/transactions.go` | Transaction CRUD: insert, update status, get by ID/hash, paginated list |
| `internal/price/coingecko.go` | CoinGecko price service with 5-min cache |
| `internal/api/handlers/address.go` | Address list + export handlers with validation/logging |
| `internal/api/handlers/scan.go` | Scan start/stop/status handlers + SSE streaming |
| `internal/api/handlers/dashboard.go` | Prices + portfolio API handlers |
| `internal/api/handlers/send.go` | Send preview/execute/gas-preseed handlers + SSE + chain dispatch |
| `internal/scanner/scanner.go` | Scanner orchestrator: multi-chain, resume, token scanning |
| `internal/scanner/pool.go` | Provider pool with round-robin rotation + failover |
| `internal/scanner/sse.go` | SSE hub for real-time scan progress broadcasting |
| `internal/scanner/setup.go` | Scanner factory + test helpers |
| `internal/wallet/hd.go` | BIP-39 mnemonic validation, seed derivation, master key |
| `internal/wallet/btc.go` | BTC bech32 via BIP-84: `m/84'/0'/0'/0/N` |
| `internal/wallet/bsc.go` | BSC EIP-55 via BIP-44: `m/44'/60'/0'/0/N` |
| `internal/wallet/sol.go` | SOL via manual SLIP-10 ed25519: `m/44'/501'/N'/0'` |
| `internal/wallet/generator.go` | Bulk generation with progress callbacks |
| `internal/api/router.go` | Chi router with middleware stack |
| `internal/tx/key_service.go` | On-demand BTC/BSC private key derivation from mnemonic file |
| `internal/tx/btc_utxo.go` | UTXO fetching with round-robin Blockstream/Mempool rotation |
| `internal/tx/btc_fee.go` | Dynamic fee estimation from mempool.space with fallback |
| `internal/tx/btc_tx.go` | Multi-input P2WPKH TX building, signing, consolidation orchestrator |
| `internal/tx/broadcaster.go` | Shared Broadcaster interface + BTC broadcast with provider fallback |
| `internal/tx/bsc_tx.go` | BSC native BNB + BEP-20 TX building, EIP-155 signing, consolidation |
| `internal/tx/gas.go` | Gas pre-seeding service: distribute BNB to addresses needing gas |
| `internal/tx/sol_tx.go` | SOL native + SPL token consolidation service |
| `internal/tx/sol_serialize.go` | Raw Solana binary TX serialization |
| `internal/tx/sse.go` | TX SSE hub for real-time transaction status broadcasting |
| `web/src/lib/types.ts` | ALL TypeScript interfaces |
| `web/src/lib/constants.ts` | ALL frontend constants |
| `web/src/lib/utils/api.ts` | API client (CSRF) + all backend API functions |
| `web/src/lib/utils/validation.ts` | Chain-specific address validation (BTC, BSC, SOL) |
| `web/src/lib/utils/chains.ts` | Chain metadata: colors, labels, explorer URLs, token decimals |
| `web/src/lib/stores/scan.svelte.ts` | Scan store with SSE lifecycle + exponential backoff |
| `web/src/lib/stores/send.svelte.ts` | Send wizard store with SSE + step management |
| `web/src/routes/+page.svelte` | Dashboard with portfolio overview + ECharts |
| `web/src/routes/scan/+page.svelte` | Scan page with real-time progress |
| `web/src/routes/send/+page.svelte` | Send wizard with 4-step stepper |

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
| `echarts` / `svelte-echarts` | Charts (portfolio pie chart) |
| `@tanstack/svelte-virtual` | Table virtualization (installed, ready for use) |
| `vitest` | Frontend unit testing |
| `typescript` | Type safety |

## API Endpoints

| Method | Path | Status | Handler |
|--------|------|--------|---------|
| GET | `/api/health` | Implemented | `handlers/health.go` |
| GET | `/api/addresses/{chain}` | Implemented | `handlers/address.go` |
| GET | `/api/addresses/{chain}/export` | Implemented | `handlers/address.go` |
| POST | `/api/scan/start` | Implemented | `handlers/scan.go` |
| POST | `/api/scan/stop` | Implemented | `handlers/scan.go` |
| GET | `/api/scan/status` | Implemented | `handlers/scan.go` |
| GET | `/api/scan/sse` | Implemented | `handlers/scan.go` |
| GET | `/api/dashboard/prices` | Implemented | `handlers/dashboard.go` |
| GET | `/api/dashboard/portfolio` | Implemented | `handlers/dashboard.go` |
| POST | `/api/send/preview` | Implemented | `handlers/send.go` |
| POST | `/api/send/execute` | Implemented | `handlers/send.go` |
| POST | `/api/send/gas-preseed` | Implemented | `handlers/send.go` |
| GET | `/api/send/sse` | Implemented | `handlers/send.go` |
| GET | `/api/settings` | Stub | `handlers/settings.go` |

## Database Schema

| Table | Migration | Purpose | Key Columns |
|-------|-----------|---------|-------------|
| `addresses` | 001 | HD wallet addresses | chain, address_index (PK), address |
| `balances` | 001 | Latest scanned balances | chain, address_index, token, balance, last_scanned |
| `transactions` | 001 | Transaction history | chain, tx_hash, status |
| `scan_state` | 001 | Scan state for resume | chain, last_scanned_index, updated_at |
| `settings` | 001 | User settings | key, value |
| `tx_state` | 005 | V2: TX lifecycle tracking | id (PK), sweep_id, chain, token, status, tx_hash, nonce |
| `provider_health` | 006 | V2: Provider health + circuit breaker | provider_name (PK), chain, status, circuit_state, consecutive_fails |
