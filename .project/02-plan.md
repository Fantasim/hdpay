# Plan: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Scope Summary

HDPay V1 is a complete, self-contained cryptocurrency payment management tool. It generates 1.5M HD wallet addresses across 3 chains (BTC, BSC, SOL), scans them for balances (native tokens + USDC/USDT) using free-tier blockchain APIs with intelligent round-robin rotation, and sweeps funded addresses into a consolidation destination via chain-specific batch transactions. Everything runs as a single Go binary on localhost with an embedded SvelteKit dashboard. V1 does NOT include multi-user support, cloud deployment, additional chains/tokens, responsive design, or any paid API dependencies.

## Feature Tiers

### Must-Have (V1 Core)

| # | Feature | Description | Complexity | Dependencies |
|---|---------|-------------|------------|--------------|
| 1 | Project Foundation | Go module, directory structure, constants/errors/config, slog dual-output logging, SQLite with migrations (WAL mode), Chi router with security middleware (CORS, CSRF, host validation), SvelteKit scaffold with Tailwind+shadcn-svelte, sidebar layout | M | None |
| 2 | HD Wallet Address Generation | BIP-44 derivation: BTC (bech32 via btcsuite, m/44'/0'/0'/0/N), BSC (EVM via go-ethereum, m/44'/60'/0'/0/N), SOL (SLIP-10 ed25519, m/44'/501'/N'/0'). Init CLI: read mnemonic, validate, generate 500K per chain, batch SQLite insert (10K batches), JSON export | L | #1 |
| 3 | Address Explorer | Paginated API per chain, virtual-scrolled table (tanstack-virtual), chain tab selector, has-balance filter, address copy, pagination controls | S | #1, #2 |
| 4 | Scanner Engine | Provider interface with round-robin pool, per-provider rate limiting (golang.org/x/time/rate), 7 providers (Blockstream, Mempool.space, Blockchain.info, BscScan, BSC RPC, Solana RPC, Helius), token scanning (native + USDC/USDT), resumable with scan_state persistence, SSE hub for fan-out broadcasting | L | #1, #2 |
| 5 | Scan Control UI | Per-chain scan cards, max ID input, start/stop buttons, real-time progress bar via EventSource (scanned/total/found/elapsed/ETA) | M | #1, #4 |
| 6 | Price Service | CoinGecko free API (/api/v3/simple/price), 5-minute cache, USD prices for BTC/BNB/SOL/USDC/USDT | S | #1 |
| 7 | Dashboard | Portfolio summary API (aggregate balances x prices), total USD value, per-chain cards with native + token balances and USD values, quick stats (total addresses, funded count, last scan time) | M | #1, #4, #6 |
| 8 | BTC Transaction Engine | UTXO fetching via Blockstream/Mempool API, multi-input tx building (btcd/wire), P2WPKH witness signing (txscript), fee estimation (vsize x feeRate), hex broadcast, on-demand key derivation + zeroing | L | #1, #2, #4 |
| 9 | BSC Transaction Engine | Sequential native BNB transfers (ethclient), BEP-20 token transfers (ABI-encoded balanceOf/transfer), nonce management, EIP-155 signing, receipt waiting, on-demand key derivation + zeroing | L | #1, #2, #4 |
| 10 | SOL Transaction Engine | Multi-instruction batch transfers (system.Transfer for native, spl_token.Transfer for SPL), ATA derivation, multi-signer transactions, batch splitting (~20 instructions/tx), blockhash refresh per batch, on-demand key derivation + zeroing | L | #1, #2, #4 |
| 11 | Gas Pre-Seeding | Identify BSC addresses with tokens but 0 BNB, source address selection, BNB distribution (0.005 BNB per address), insufficient-balance validation, confirmation tracking | M | #9 |
| 12 | Send Interface | Chain/token selector, funded address list, destination input with chain-specific validation, preview API (fees, amounts, address count, gas-needs assessment), execute API, gas pre-seed sub-step for BSC tokens, real-time tx progress via SSE, receipt display with explorer links | L | #8, #9, #10, #11 |
| 13 | Transaction History | Store transactions from scans + sends, paginated/filterable API (chain, direction, token, date range), sortable table, explorer links per chain | M | #1, #4 |

### Should-Have (V1 Enhanced)

| # | Feature | Description | Complexity | Dependencies |
|---|---------|-------------|------------|--------------|
| 14 | Portfolio Charts | ECharts pie chart (chain distribution), bar chart (token distribution) on dashboard | S | #7 |
| 15 | Settings Page | Settings API (GET/PUT), max scan ID per chain, network toggle (mainnet/testnet), log level selector | S | #1 |
| 16 | Address Export | JSON export endpoint per chain, download button in address explorer | S | #3 |
| 17 | Provider Status Display | Health check per provider, connectivity indicators in scan page | S | #4 |
| 18 | Scan History | Persist last 10 scans per chain with results summary, display in scan page | S | #4 |
| 19 | Dynamic BTC Fee Estimation | Fetch from mempool.space /api/v1/fees/recommended, user selects priority (fast/medium/economy), fallback to config constant | S | #8 |

### Nice-to-Have (Defer to V2)

| # | Feature | Description | Why Defer |
|---|---------|-------------|-----------|
| 20 | Address Search | Search by address string within explorer | Explorer works fine with filters + pagination |
| 21 | Transaction Amount Filtering | Filter history by amount range | Basic chain/token/direction filters sufficient |
| 22 | Bulk Address Selection | Select specific addresses to sweep instead of "send all" | Send-all covers primary use case |
| 23 | Automated Scheduled Scanning | Cron-like periodic re-scans | Manual scan trigger is sufficient |
| 24 | Notification System | Email/desktop alerts when balances found | User monitors dashboard directly |

### Explicitly NOT in V1

- **Multi-user / authentication**: Single-user localhost tool, no accounts needed
- **Cloud deployment**: Localhost-only by design; cloud would require auth, TLS, and security hardening
- **Additional chains (ETH, Polygon, Tron, etc.)**: Three chains cover the target use case; architecture supports future additions
- **Additional tokens**: Only USDC/USDT on BSC and SOL; adding tokens is a config change later
- **Mobile responsive design**: Desktop-only tool, no mobile use case
- **WebSocket**: SSE covers all real-time needs (unidirectional server→client)
- **Paid API services**: Free-tier APIs are sufficient with round-robin rotation
- **Fiat on/off ramp**: Out of scope — this is a fund management tool, not an exchange
- **Multi-wallet (multiple mnemonics)**: Single wallet per instance; run multiple instances for multiple wallets
- **Address labeling/grouping**: Addresses are identified by chain + index; labeling adds UI complexity for minimal value

## User Stories

### Primary Workflow
As a **crypto fund operator**, I want to:
1. Generate thousands of receiving addresses from a single HD wallet mnemonic
2. Distribute those addresses to receive payments
3. Periodically scan which addresses received funds
4. See my total portfolio value across all chains and tokens in USD
5. Sweep all funded addresses into a single consolidation address
6. Track all transactions in a local history

So that I can **manage high-volume crypto payments at scale without manual tracking or paid infrastructure**.

### Secondary Workflows

**Resume Interrupted Scan**
As a user, I want to resume a scan that was interrupted (server restart, network issue) from where it left off — so I don't waste time re-scanning already-checked addresses.

**Testnet Verification**
As a user, I want to switch to testnet mode and test the full scan→send flow with testnet coins — so I can verify correctness before handling real funds.

**Gas Pre-Seeding**
As a user, when sweeping BSC tokens (USDC/USDT), I want the system to identify addresses needing gas, let me select a BNB source, and distribute gas automatically — so I don't have to manually send BNB to each address.

**Portfolio Monitoring**
As a user, I want to see my total portfolio value in USD updated with current prices, broken down by chain and token — so I know my overall position at a glance.

**Address Distribution**
As a user, I want to export address lists as JSON per chain — so I can integrate them with external payment systems that assign addresses to customers.

## Tech Stack

### Recommended Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Backend Language | Go 1.22+ | Performance for 1.5M address generation, goroutines for concurrent scanning, strong crypto library ecosystem |
| HTTP Router | Chi v5 | Lightweight, idiomatic Go, good middleware support |
| Database | SQLite via modernc.org/sqlite | Pure Go (no CGO), single file, WAL mode for concurrent reads, perfect for single-user localhost |
| Logging | slog (stdlib) | Structured logging, built-in, dual output (stdout + file), zero dependencies |
| Config | envconfig | Simple env var mapping to struct, minimal boilerplate |
| Rate Limiting | golang.org/x/time/rate | Standard token bucket, per-provider instances |
| BTC Crypto | btcsuite/btcd, go-bip39 | Industry standard Go Bitcoin libraries, BIP-32/BIP-44 support |
| EVM Crypto | go-ethereum | Official Ethereum client library, works for BSC (same EVM) |
| SOL Crypto | gagliardetto/solana-go | Most complete Go Solana library, RPC + transaction building + SPL |
| SOL HD Derivation | dmitrymomot/solana or manual SLIP-10 | BIP-44 ed25519 derivation for Solana paths |
| Frontend Framework | SvelteKit (adapter-static) | Compiles to static files for embedding, excellent DX, small bundle |
| Frontend Language | TypeScript (strict) | Type safety, zero `any` policy, explicit return types |
| CSS | Tailwind + shadcn-svelte | Utility-first CSS with pre-built accessible components |
| Charts | Apache ECharts | Feature-rich, good Svelte integration, free |
| Table Virtualization | @tanstack/svelte-virtual | Handle 500K row address lists without DOM bloat |
| Testing (Backend) | Go stdlib testing | Table-driven tests, built-in coverage |
| Testing (Frontend) | Vitest + Testing Library | Fast, Svelte-native, good component testing |

### Stack Rationale

No research was needed — the tech stack was pre-selected based on project requirements:
- **Pure Go (no CGO)**: Enables single binary cross-compilation without C toolchain
- **SQLite over Postgres**: No separate database server; single file, zero config, embedded
- **SvelteKit over Next.js/React**: Smaller bundle, compiled to static, no SSR needed (localhost)
- **SSE over WebSocket**: Simpler (unidirectional), native browser EventSource with auto-reconnect, sufficient for scan progress + tx status
- **Free-tier APIs with rotation**: Eliminates paid service dependency; round-robin distributes load across rate limits

## Data Model (High-Level)

### Entities

- **Address**: chain, address_index, address string. 500K per chain (1.5M total). Immutable after init.
- **Balance**: chain, address_index, token (NATIVE/USDC/USDT), balance (string for big number precision), last_scanned timestamp. Updated by scanner.
- **Transaction**: chain, address_index, tx_hash, direction (in/out), token, amount, from/to addresses, block info, status (pending/confirmed/failed). Populated by scanner (incoming) and send engine (outgoing).
- **ScanState**: Per-chain singleton. last_scanned_index, max_scan_id, status (idle/scanning/paused), timestamps. Enables resume.
- **Settings**: Key-value pairs. Stores user-configurable values (max scan IDs, network mode).
- **SchemaMigrations**: Tracks applied database migrations.

### Relationships
```
Address (chain, address_index) ← 1:N → Balance (chain, address_index, token)
Address (chain, address_index) ← 1:N → Transaction (chain, address_index)
ScanState (chain) — one per chain, independent
```

### Balance Storage Convention
All balances stored as **string integers in smallest unit**:
| Chain | Token | Decimals | Example: "1.0" stored as |
|-------|-------|----------|--------------------------|
| BTC | BTC | 8 | "100000000" (satoshis) |
| BSC | BNB | 18 | "1000000000000000000" (wei) |
| BSC | USDC | 18 | "1000000000000000000" |
| BSC | USDT | 18 | "1000000000000000000" |
| SOL | SOL | 9 | "1000000000" (lamports) |
| SOL | USDC | 6 | "1000000" |
| SOL | USDT | 6 | "1000000" |

## Authentication & Authorization

- **Method**: None — no user accounts
- **Security model**: Localhost-only with defense-in-depth:
  - Server binds to `127.0.0.1` only (never `0.0.0.0`)
  - CORS allows only `http://localhost:*` and `http://127.0.0.1:*`
  - Host header validation rejects non-localhost requests (DNS rebinding protection)
  - CSRF tokens on all mutating endpoints (SameSite=Strict cookies)
  - Mnemonic never stored in DB or logs
  - Private keys derived on-demand, zeroed immediately after signing

## Open Questions for Next Phases

- **Mockup**: Exact layout of the send page multi-step flow — how to present the gas pre-seed sub-step inline vs. as a separate modal
- **Mockup**: Dashboard card layout — how to compactly show 3 chains x (native + 2 tokens) without overwhelming the user
- **Mockup**: Scan page — should each chain be a separate card or a tabbed interface
- **Build Plan**: Optimal build phase ordering — foundation+wallet first, then scanning, then dashboard, then transactions
- **Build Plan**: Should scanning all 3 chains run concurrently or one at a time (UX decision affects SSE design)

## User Answers Archive

### 1. Must-have features
HD wallet address generation (BTC/BSC/SOL, 500K per chain), SQLite storage, balance scanning with round-robin providers + rate limiting, resumable scans, token scanning (native + USDC/USDT), SSE progress, transaction sweeping (BTC UTXO/BSC sequential/SOL batch), gas pre-seeding, on-demand key derivation, dashboard with USD prices, address explorer with virtual scroll, send interface (preview/execute/progress), transaction history, localhost security (CORS/CSRF), single binary deployment, init CLI.

### 2. Nice-to-have features
ECharts portfolio charts, address JSON export, settings page, provider status display, scan history, dynamic BTC fee estimation.

### 3. Explicitly NOT in V1
Multi-user, cloud deployment, additional chains/tokens, responsive design, WebSocket, paid APIs, fiat ramp, scheduled scanning, notifications, address labeling, multi-wallet.

### 4. Core user workflow
Init (generate 1.5M addresses) → Serve (start localhost server) → Scan (select chain, set max ID, watch progress) → Dashboard (view portfolio in USD) → Send (select chain/token, preview, gas pre-seed if BSC tokens, execute sweep) → History (review transactions).

### 5. Secondary workflows
Resume interrupted scans, address exploration with filters, re-scanning for new funds, JSON export, mainnet/testnet switching, monitoring pending transactions.

### 6. Tech stack
Fully specified: Go 1.22+ (Chi, SQLite/modernc, slog, envconfig) + SvelteKit (adapter-static, TypeScript strict, Tailwind+shadcn-svelte, ECharts, tanstack-virtual) + crypto libs (btcsuite, go-ethereum, solana-go).

### 7. Authentication
None — localhost-only with CSRF, CORS, host validation.

### 8. Data requirements
1.5M addresses in SQLite, balances as strings (big numbers), transaction history, scan state, settings. WAL mode. No private keys or mnemonics in DB. Token decimals vary by chain.
