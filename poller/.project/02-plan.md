# Poller — V1 Feature Plan

> Crypto-to-Points microservice: watch addresses, detect incoming transactions, award points.

## Product Summary

Poller is a standalone microservice that monitors cryptocurrency addresses for incoming payments and converts them into **points** for a video game economy. A game server tells Poller to watch an address, Poller detects transactions via free-tier blockchain APIs, calculates USD value at confirmation time, applies a configurable tier multiplier, and exposes the resulting points for the game server to claim.

**Core loop**: `POST /watch` → detect tx → confirm → calculate points → `GET /points` → `POST /points/claim`

---

## Feature Tiers

### Must-Have (35 features)

#### A. Watch & Polling Engine (6)

| # | Feature | Description |
|---|---------|-------------|
| 1 | Watch Management API | `POST /watch` (create), `DELETE /watch/:id` (cancel), `GET /watches` (list, filterable by status/chain) |
| 2 | Watch Lifecycle Engine | One goroutine per active watch. States: ACTIVE→COMPLETED/EXPIRED/CANCELLED. Stop conditions: timeout, manual cancel, all confirmed + at least one tx. Duplicate address rejection (ERROR_ALREADY_WATCHING). |
| 3 | Smart Cutoff Detection | On watch start, cutoff = MAX(last recorded tx for address, START_DATE). First-ever watch scans from START_DATE. |
| 4 | Poll Loop | Per-chain intervals (BTC:60s, BSC:5s, SOL:5s). On each tick: query blockchain, process new txs, re-check pending txs, update watch metadata, check stop conditions. |
| 5 | Transaction Deduplication | `tx_hash` unique in DB. SOL composite key (`sig:TOKEN`) for multi-token transactions. |
| 6 | Confirmation Tracking | BTC: 1 confirmation (status.confirmed=true). BSC: 12 confirmations (currentBlock - txBlock). SOL: finalized commitment. PENDING→CONFIRMED lifecycle. |

#### B. Chain-Specific Detection (3)

| # | Feature | Description |
|---|---------|-------------|
| 7 | BTC Detection | Query Blockstream/Mempool `/api/address/{addr}/txs`. Parse vout for matching scriptpubkey_address. Sum incoming satoshis. Handle pagination (25/page). |
| 8 | BSC Detection | BscScan `txlist` (native BNB, filter to=address, isError!=1) + `tokentx` (USDC/USDT by contract address). Wei amounts (18 decimals). |
| 9 | SOL Detection | `getSignaturesForAddress` + `getTransaction` per new sig. Parse pre/postBalances for native SOL (9 decimals). Parse pre/postTokenBalances for USDC/USDT (6 decimals, by mint address). |

#### C. Points System (3)

| # | Feature | Description |
|---|---------|-------------|
| 10 | Points Calculation | `cents = floor(usd * 100)`, `points = round(cents * multiplier)`. Flat tier (not marginal). Rounded integers. |
| 11 | Tier Configuration | Load from `tiers.json` (create with defaults if missing). 9 default tiers ($0-1 ignored, $1-12 1.0x, up to $1200+ 3.0x). Validation: no gaps, sorted, last tier unbounded, min 2 tiers. |
| 12 | Points API | `GET /points` (accounts with unclaimed confirmed points + tx details). `GET /points/pending` (unconfirmed points). `POST /points/claim` (reset unclaimed for explicit address list, skip unknown/zero silently). |

#### D. Price Service (1)

| # | Feature | Description |
|---|---------|-------------|
| 13 | CoinGecko Price Fetching | Fetch at confirmation time (not detection). 60s cache. IDs: bitcoin, binancecoin, solana. Stablecoins (USDC/USDT) hardcoded $1.00. Retry 3x with 5s backoff. On failure: leave tx PENDING, retry next poll. |

#### E. Provider Layer (2)

| # | Feature | Description |
|---|---------|-------------|
| 14 | Provider Round-Robin | Per-chain provider sets. Rotate on failure. Per-provider rate limiters (token bucket). Never stop a watch due to provider failures — log error, rotate, retry next tick. |
| 15 | Provider Failure Logging | Record failures to `system_errors` table (category=PROVIDER). |

#### F. Infrastructure (6)

| # | Feature | Description |
|---|---------|-------------|
| 16 | SQLite Database | 5 tables (watches, points, transactions, ip_allowlist, system_errors) + schema_version. WAL mode, busy_timeout=5000. Numbered migrations in `internal/db/migrations/`. |
| 17 | IP Allowlist Middleware | Extract client IP (X-Forwarded-For first, then RemoteAddr). Localhost/private network always allowed. Internet IPs checked against DB-backed in-memory cache (sync.RWMutex map). Exempt: /api/health, /api/admin/login. |
| 18 | Session Auth | bcrypt hash password at startup (plaintext in .env). Random 32-byte hex token. In-memory session store (map + RWMutex). 1h expiry. HttpOnly SameSite=Strict cookie. Lost on restart (acceptable). |
| 19 | Config & Validation | envconfig struct. Required: START_DATE, ADMIN_USERNAME, ADMIN_PASSWORD. Defaults for port (8081), DB path, log level, network, timeouts. Validate at boot. |
| 20 | Logging | slog dual output: stdout + daily rotated files split by level (info.log, warn.log, error.log, debug.log). 30-day retention. |
| 21 | Startup & Shutdown | Startup: load config → init slog → open DB → run migrations → load tiers → hash password → load IP cache → init providers → init watcher → recovery → start HTTP. Shutdown: SIGTERM/SIGINT → cancel watches → wait with 10s timeout → expire remaining → close DB. |

#### G. HTTP Server (2)

| # | Feature | Description |
|---|---------|-------------|
| 22 | Chi Router | Middleware stack: IP allowlist, session auth (dashboard routes only), request logging. CORS: `Access-Control-Allow-Origin: *` (IP allowlist is the security boundary). |
| 23 | Single Binary + Embedded SPA | `go:embed web/build`. Serve static files with SPA fallback. Immutable cache for `_app/`. |

#### H. Address Validation (1)

| # | Feature | Description |
|---|---------|-------------|
| 24 | Address Format Validation | BTC: bc1/1/3 (mainnet), tb1/m/n/2 (testnet) — bech32 checksum via btcutil. BSC: 0x + 40 hex chars. SOL: base58, decodes to 32 bytes. |

#### I. Startup Recovery (1)

| # | Feature | Description |
|---|---------|-------------|
| 25 | Recovery on Boot | Expire all ACTIVE watches in DB (set EXPIRED). Re-check all PENDING transactions on-chain (3 retries, 30s spacing). If still pending after retries → log system error. |

#### J. Dashboard Frontend (10)

| # | Feature | Description |
|---|---------|-------------|
| 26 | Login Page | Username + password form. On success → redirect to overview. On failure → error message. |
| 27 | Layout | Sidebar navigation (6 pages + logout). Header with page title. Auth-gated routing (redirect to /login if no session). Match HDPay dashboard style. |
| 28 | Overview Page | Stats cards (active watches, total watches, USD received, points awarded, pending points, unique addresses, avg tx size, largest tx). Time range selector (today/week/month/quarter/all). 7 charts (USD/points/tx count over time, chain breakdown pie, token breakdown pie, tier distribution bar, watches over time line). |
| 29 | Transactions Page | Full history table: timestamp, address (truncated), chain, token, crypto amount, USD value, tier, points, tx hash (block explorer link), watch ID, status badge. Filters: chain, token, date range, tier, status, min/max USD. Server-side pagination (25/50/100). |
| 30 | Watches Page | Table: address, chain, status (color badge), started_at, time remaining (countdown for ACTIVE), poll count, last poll result. Filterable by status and chain. |
| 31 | Pending Points Page | Table: address, chain, unclaimed points, pending points, tx count, last tx timestamp, all-time total. |
| 32 | Errors Page | Three sections: discrepancies (auto-detected), stale pending txs (>24h), system errors (from DB). Each with severity, category, message, timestamp. |
| 33 | Settings: Tier Editor | Editable table of tiers (min_usd, max_usd, multiplier). Save writes to tiers.json. Client + server validation. |
| 34 | Settings: IP Allowlist | Table of allowed IPs with description. Add (IP + optional label) and remove buttons. Immediate effect (cache refresh). |
| 35 | Settings: System Info & Watch Defaults | Read-only: uptime, version, network mode, active watch count, DB size, START_DATE. Editable: default timeout, max active watches. |

---

### Should-Have (3 features)

| # | Feature | Description |
|---|---------|-------------|
| S1 | Discrepancy Auto-Detection | 5 SQL checks on errors page load: points sum mismatch, unclaimed>total, orphaned transactions, stale pending (>24h), unresolved system errors. Results logged to system_errors. |
| S2 | Block Explorer Links | Chain-aware + network-aware tx hash links in transaction table. Mainnet: blockstream/bscscan/solscan. Testnet: corresponding testnet explorers. |
| S3 | Health Check | `GET /api/health` — always open, no auth, no IP check. Returns 200 with basic status. |

---

### Nice-to-Have / V2 (5 features)

| # | Feature | Description |
|---|---------|-------------|
| N1 | BscScan RPC Fallback | When BscScan is rate-limited, fall back to RPC eth_getBlockByNumber + process logs for tx history. |
| N2 | CIDR Notation in IP Allowlist | Support `192.168.1.0/24` patterns instead of individual IPs only. |
| N3 | Testnet Token Contracts | Deploy or configure mock USDC/USDT contracts for BSC testnet and SOL devnet testing. |
| N4 | Webhook Notifications | POST callback to game server URL on points confirmation (push instead of poll). |
| N5 | Transaction Export | CSV/JSON export of transaction history from dashboard. |

---

### Explicitly Deferred

- **Auto-refresh / SSE on dashboard** — manual refresh only, matches HDPay's pattern
- **Multi-wallet support** — single network mode, one config
- **Email/Slack alerting** — log-based monitoring only
- **Rate limit dashboard indicators** — internal logging sufficient
- **Mobile responsive design** — desktop-only admin tool
- **API keys/tokens for game server auth** — IP allowlist is sufficient
- **Historical price lookups** — price at confirmation time only, no backfill
- **Batch watch creation** — one address per POST /watch call

---

## Tech Stack (Confirmed)

| Layer | Technology | Notes |
|-------|-----------|-------|
| Language | Go 1.22+ | Same as HDPay |
| Router | Chi v5 | Same as HDPay |
| Database | SQLite (modernc.org/sqlite) | Pure Go, no CGO, own DB file |
| Logging | slog | Structured, split by level |
| Config | envconfig | POLLER_* prefix |
| Price Feed | CoinGecko free API | 60s cache, no API key |
| Session Auth | bcrypt + cookie | In-memory store, 1h expiry |
| BTC Validation | btcsuite/btcd/btcutil | Bech32 checksum validation |
| Frontend | SvelteKit (adapter-static) | Embedded via go:embed |
| TypeScript | Strict mode | Zero `any` |
| CSS | Tailwind + shadcn-svelte | Match HDPay style |
| Charts | Apache ECharts | Via svelte-echarts wrapper |
| Testing | Go stdlib + Vitest | Min 70% coverage on core |
| Deployment | Single binary | go:embed all:build |

---

## Scope Boundaries

### In Scope
- All 35 must-have + 3 should-have features
- BTC + BSC + SOL chains
- Native tokens + USDC + USDT
- Mainnet and testnet support (configured via env)
- Free-tier blockchain APIs only

### Out of Scope
- No private key storage or wallet functionality
- No transaction sending (read-only blockchain access)
- No auto-refresh or real-time dashboard updates
- No API authentication beyond IP allowlist
- No responsive/mobile design
- No external paid services

---

## Key Architecture Decisions

1. **One goroutine per watch**: Simple, isolated, cancellable via context. WaitGroup for graceful shutdown.
2. **Price at confirmation**: USD price fetched when tx reaches confirmation threshold, not at detection. More accurate.
3. **Stablecoins = $1.00**: No CoinGecko lookup for USDC/USDT. Saves API calls.
4. **In-memory sessions**: Lost on restart, but 1h timeout makes this acceptable. No external session store needed.
5. **IP allowlist over API keys**: Simpler for game server integration. Localhost/private always allowed.
6. **tiers.json over DB**: Tiers are simple config, JSON file is easier to inspect and backup than DB rows.
7. **Manual refresh dashboard**: No SSE complexity. Game server polls API, admin refreshes dashboard manually.
8. **SOL composite tx_hash**: `sig:TOKEN` format handles multi-token Solana transactions without schema changes.
9. **Companion to HDPay**: Shares code patterns and conventions, never imports HDPay code directly.
