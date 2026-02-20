# Poller Development Guidelines

## Project Overview

Poller is a standalone microservice that monitors cryptocurrency addresses for incoming payments and converts them into points for a video game economy. It has zero runtime dependency on HDPay. It watches whatever address is passed via API, detects incoming transactions on all supported tokens, and awards points based on a configurable USD-to-points tier system.

**Companion to**: HDPay (parent directory `../`) — shares code patterns, never modifies HDPay
**Go Module**: `github.com/<user>/hdpay/poller` (sub-module of HDPay)
**Supported Chains**: BTC, BSC, SOL
**Supported Tokens**: Native (BTC, BNB, SOL) + USDC and USDT on BSC and SOL
**Core Philosophy**: Self-hosted, zero external paid services, single binary deployment.

---

## Tech Stack

### Backend (Go 1.22+)
- **Router**: Chi (`github.com/go-chi/chi/v5`)
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO) — single DB, Poller's own
- **Logging**: `log/slog` (structured, multi-output: stdout + daily rotated files split by level)
- **Config**: `github.com/kelseyhightower/envconfig`
- **HTTP Client**: Standard `net/http` with rate-limiting middleware for blockchain API calls
- **Price Feed**: CoinGecko free API (`/api/v3/simple/price`)
- **Session Auth**: Secure cookie-based sessions (bcrypt hashed password, 1h expiry)
- **Frontend Embedding**: `go:embed` directive to serve SvelteKit static build from the binary

### Frontend (SvelteKit)
- **Framework**: SvelteKit with `adapter-static`
- **Language**: TypeScript (strict mode, zero `any`)
- **Styling**: Tailwind CSS + shadcn-svelte
- **Charts**: Apache ECharts (analytics/dashboard)
- **State**: Svelte stores (built-in)
- **Style**: Match HDPay's dashboard theme

### Deployment
- **Build**: Single Go binary with `//go:embed web/build` serves the SvelteKit static output
- **Database**: Single SQLite file at `./data/poller.sqlite`
- **Network**: IP allowlist — localhost/private-network always allowed, internet IPs managed via dashboard
- **Security**: IP allowlist on API, session auth on dashboard, no private keys stored
- **CORS**: Allow all origins (API is protected by IP allowlist, dashboard by session cookie)

---

## Blockchain API Providers (Free Tier)

Same providers as HDPay. All polling uses free-tier APIs with round-robin rotation.

### BTC
| Provider | Endpoint | Rate Limit |
|----------|----------|------------|
| Blockstream Esplora | `blockstream.info/api` | ~10 req/s |
| Mempool.space | `mempool.space/api` | ~10 req/s |

### BSC
| Provider | Endpoint | Rate Limit |
|----------|----------|------------|
| BscScan API | `api.bscscan.com/api` | 5 req/s |
| BSC Public RPC | `bsc-dataseed.binance.org` | ~10 req/s |
| Ankr Public | `rpc.ankr.com/bsc` | ~30 req/s |

### SOL
| Provider | Endpoint | Rate Limit |
|----------|----------|------------|
| Solana Public RPC | `api.mainnet-beta.solana.com` | ~10 req/s |
| Helius Free | `mainnet.helius-rpc.com` | ~10 req/s |

### Testnet
| Chain | Provider |
|-------|----------|
| BTC Testnet | `blockstream.info/testnet/api`, `mempool.space/testnet/api` |
| BSC Testnet | `data-seed-prebsc-1-s1.binance.org:8545`, BscScan testnet API |
| SOL Devnet | `api.devnet.solana.com` |

---

## Core Domain

### Watch Lifecycle

```
POST /api/watch {address, chain}
    |
    v
[ACTIVE] -- polling every N seconds
    |
    |-- new tx detected (unconfirmed) --> record as PENDING tx
    |                                      calculate pending_points
    |                                      keep polling for confirmation
    |
    |-- tx reaches confirmation threshold --> PENDING -> CONFIRMED
    |                                          fetch USD price NOW
    |                                          calculate points
    |                                          pending_points -> unclaimed points
    |                                          store tx_hash (dedup)
    |
    |-- timeout reached with pending txs --> [EXPIRED]
    |    (pending txs stay PENDING in DB,     pending_points remain
    |     startup recovery will re-check)     until next watch or recovery
    |
    |-- timeout reached, no pending txs ---> [EXPIRED]
    |-- manual cancel ----------------------> [CANCELLED]
    |-- all pending txs confirmed + -------> [COMPLETED]
         at least one tx was found
```

### Watch States
- **ACTIVE**: Currently polling the blockchain at regular intervals
- **COMPLETED**: At least one tx detected AND all detected pending txs are now confirmed. Polling stopped.
- **EXPIRED**: Timeout reached. If there were pending txs, they remain PENDING in the DB — startup recovery or a future watch will re-check them.
- **CANCELLED**: Manually cancelled via `DELETE /watch/:id`. Same handling as EXPIRED for pending txs.

### Watch Stop Conditions (Exact Rules)
A watch stops polling when ANY of these is true:
1. **Timeout reached** (`expires_at <= now`) → status = EXPIRED
2. **Manually cancelled** → status = CANCELLED
3. **All done**: at least one CONFIRMED tx found during this watch AND zero PENDING txs remaining → status = COMPLETED

A watch does NOT stop just because it found a confirmed tx if there are still pending txs — it keeps polling until those are confirmed or it times out.

### Duplicate Watch Handling
If `POST /watch` is called for an address that already has an ACTIVE watch:
- **Reject with `ERROR_ALREADY_WATCHING`** — one active watch per address at a time
- The caller should wait for the existing watch to complete/expire before creating a new one
- A new watch CAN be created after the previous one is COMPLETED/EXPIRED/CANCELLED

### Transaction States
- **PENDING**: Transaction detected but below confirmation threshold — `pending_points` calculated (informational, not claimable)
- **CONFIRMED**: Required confirmations reached — USD price fetched at this moment, points calculated, points become `unclaimed` (claimable)

**Important**: USD price is fetched at **confirmation time**, not at detection time. This ensures the price used reflects when the transaction is actually finalized.

### Confirmation Thresholds
| Chain | Confirmations | Approximate Time | How to Check |
|-------|---------------|------------------|--------------|
| BTC | 1 | ~10 minutes | tx `status.confirmed` = true in Blockstream/Mempool API |
| BSC | 12 | ~36 seconds | `currentBlock - txBlock >= 12` via `eth_blockNumber` |
| SOL | Finalized | ~13 seconds | `getSignatureStatuses` with `commitment: "finalized"` |

### Polling Intervals
- **BTC**: Every 60 seconds (block time ~10 min)
- **BSC**: Every 5 seconds (block time ~3s)
- **SOL**: Every 5 seconds (block time ~0.4s)

---

## Chain-Specific Transaction Detection

This is the core of the system. Each chain has different APIs and different ways to detect incoming transactions.

### BTC Detection

**Goal**: Find all incoming transactions to an address after a cutoff timestamp.

**API call per poll**: 1 request (transaction list)

**Algorithm**:
```
1. GET /api/address/{address}/txs  (Blockstream or Mempool)
   Returns: list of transactions involving this address

2. For each tx in the list:
   a. Skip if tx.status.block_time < cutoff_timestamp
   b. Skip if tx_hash already in Poller's DB (dedup)
   c. Parse tx.vout[] — find outputs where scriptpubkey_address == our address
   d. Sum all matching output values (in satoshis) — this is the INCOMING amount
   e. Skip tx if it has NO outputs to our address (it's an outgoing tx)
   f. Convert satoshis to BTC: amount_raw = satoshis / 100_000_000 (store as string)
   g. Check tx.status.confirmed:
      - If true (confirmed=true, block_height exists): status = CONFIRMED
      - If false (in mempool): status = PENDING

3. The list is paginated (25 per page on Blockstream). If we get 25 results,
   fetch next page. Stop when we see a tx older than cutoff or reach end.
```

**amount_raw format**: BTC as decimal string (e.g., `"0.00168841"`)

**Confirmation check for PENDING → CONFIRMED**: On each poll, re-fetch the transaction by hash:
```
GET /api/tx/{txid}
Check: tx.status.confirmed == true
```

### BSC Detection

**Goal**: Find all incoming native BNB + BEP-20 USDC/USDT transfers after a cutoff timestamp.

**API calls per poll**: 2 requests (one for normal txs, one for token txs) via BscScan, OR 1 call if using only RPC (less efficient for history)

**Algorithm (BscScan API — preferred for tx history)**:
```
1. Normal transactions (native BNB):
   GET /api?module=account&action=txlist&address={address}
       &startblock=0&endblock=99999999&sort=desc

   For each tx:
   a. Skip if tx.timeStamp < cutoff_unix
   b. Skip if tx_hash already in Poller's DB
   c. Skip if tx.to != our address (outgoing tx)
   d. Skip if tx.isError == "1" (failed tx)
   e. amount_raw = tx.value (in wei as string)
   f. Convert to BNB: value_bnb = wei / 10^18
   g. Check confirmations: current_block - tx.blockNumber
      - >= 12: CONFIRMED
      - < 12: PENDING

2. Token transactions (USDC + USDT):
   GET /api?module=account&action=tokentx&address={address}
       &startblock=0&endblock=99999999&sort=desc

   For each tx:
   a. Skip if tx.timeStamp < cutoff_unix
   b. Skip if tx_hash already in Poller's DB
   c. Skip if tx.to != our address (outgoing transfer)
   d. Skip if tx.contractAddress NOT IN [BSC_USDC_Contract, BSC_USDT_Contract]
   e. amount_raw = tx.value (in token smallest unit as string)
   f. Convert: USDC/USDT both have 18 decimals on BSC → value = raw / 10^18
   g. Check confirmations same as above
```

**amount_raw format**:
- BNB: wei as decimal string (e.g., `"5000000000000000"` = 0.005 BNB)
- USDC/USDT on BSC: smallest unit as decimal string (18 decimals)

**Confirmation check for PENDING → CONFIRMED**:
```
current_block = eth_blockNumber (RPC call)
confirmations = current_block - tx.blockNumber
If confirmations >= 12: CONFIRMED
```

**Fallback if BscScan is rate-limited**: Use RPC `eth_getBlockByNumber("latest")` and process logs. More complex but avoids BscScan dependency.

### SOL Detection

**Goal**: Find all incoming native SOL + SPL USDC/USDT transfers after a cutoff timestamp.

**API calls per poll**: 1-2 requests

**Algorithm**:
```
1. Get recent transaction signatures:
   RPC: getSignaturesForAddress(address, {limit: 20, commitment: "confirmed"})
   Returns: list of {signature, blockTime, confirmationStatus, err}

2. For each signature:
   a. Skip if blockTime < cutoff_unix
   b. Skip if tx_hash (signature) already in Poller's DB
   c. Skip if err != null (failed tx)
   d. Fetch full transaction:
      RPC: getTransaction(signature, {commitment: "confirmed", maxSupportedTransactionVersion: 0})

   e. Parse for native SOL transfer:
      - Check pre/postBalances for our address index
      - incoming_lamports = postBalance - preBalance (if positive, it's incoming)
      - If incoming_lamports > 0: token = "SOL", amount_raw = lamports as string

   f. Parse for SPL token transfers:
      - Check meta.preTokenBalances and meta.postTokenBalances
      - Find entries where owner == our address AND mint IN [SOL_USDC_Mint, SOL_USDT_Mint]
      - delta = postAmount - preAmount (if positive, it's incoming)
      - USDC: 6 decimals, USDT: 6 decimals on Solana
      - amount_raw = delta in smallest unit as string

   g. Confirmation status:
      - confirmationStatus == "finalized": CONFIRMED
      - confirmationStatus == "confirmed" but not "finalized": PENDING

3. A single SOL transaction can contain BOTH a native SOL transfer AND
   a token transfer. Record them as SEPARATE transactions in Poller's DB
   (different token, same tx_hash is NOT possible — use tx_hash + token as
   the dedup key, or record as tx_hash_SOL and tx_hash_USDC).
```

**IMPORTANT tx_hash uniqueness for SOL**: A single Solana transaction can transfer SOL and USDC simultaneously. Since `tx_hash` must be unique in the DB, use the composite key `tx_hash:token` as the stored tx_hash. Example: `"5xYz...abc:SOL"` and `"5xYz...abc:USDC"`. This is SOL-specific — BTC and BSC transactions only carry one token type.

**amount_raw format**:
- SOL: lamports as decimal string (e.g., `"1000000000"` = 1 SOL, 9 decimals)
- USDC on SOL: smallest unit as decimal string (6 decimals, e.g., `"50000000"` = 50 USDC)
- USDT on SOL: smallest unit as decimal string (6 decimals)

### API Call Budget Per Poll (Summary)

| Chain | Calls per poll | What |
|-------|---------------|------|
| BTC | 1 | `GET /api/address/{addr}/txs` |
| BSC | 2 | BscScan `txlist` + `tokentx` |
| SOL | 1 + N | `getSignaturesForAddress` + `getTransaction` per new sig |

To minimize SOL calls: only fetch `getTransaction` for signatures not already in the DB.

---

## Smart Transaction Detection

### Cutoff Timestamp Resolution
When a watch is created for an address, the cutoff is determined as follows:
```
cutoff = MAX(
    last_recorded_tx.detected_at for this address in Poller's DB,
    START_DATE from .env
)

If no transactions exist for this address in DB:
    cutoff = START_DATE
```

This means:
- First-ever watch for an address: scans from START_DATE
- Subsequent watches: scans from the most recent tx Poller already knows about
- START_DATE acts as a floor — even if a previous tx was recorded before START_DATE (shouldn't happen, but safety net)

### On Each Poll Iteration
```
1. Query blockchain for transactions after cutoff (chain-specific, see above)
2. For each new tx not in DB:
   a. Insert into transactions table with status PENDING or CONFIRMED
   b. If PENDING: calculate pending_points, update points.pending
   c. If CONFIRMED: fetch USD price, calculate points, update points.unclaimed
3. For each existing PENDING tx in DB for this address:
   a. Re-check confirmation status on-chain
   b. If now confirmed: update to CONFIRMED, fetch price, calculate points,
      move from points.pending to points.unclaimed
4. Update watch: poll_count++, last_poll_at, last_poll_result
5. Check stop conditions (see Watch Stop Conditions above)
```

### Startup Recovery
On startup, Poller does NOT re-scan all addresses. It handles two specific cases:

**1. Interrupted watches** (status = ACTIVE in DB):
```sql
UPDATE watches SET status = 'EXPIRED', completed_at = CURRENT_TIMESTAMP
WHERE status = 'ACTIVE';
```

**2. Orphaned pending transactions** (status = PENDING, no active watch):
```sql
SELECT * FROM transactions WHERE status = 'PENDING';
```
For each: spawn a one-shot goroutine that checks on-chain confirmation status. If confirmed, update to CONFIRMED and award points. If still pending after 3 retries with 30s spacing, log as a system error.

---

## Points Calculation

### Tier System (Flat — Not Marginal)

The entire transaction amount uses a **single tier multiplier** based on its total USD value. Tiers are stored in `tiers.json` (not in the database) and are editable from the dashboard. Changes apply to new transactions only — existing recorded points are never recalculated.

### Default Tiers (`tiers.json`)
```json
[
  {"min_usd": 0,      "max_usd": 1,       "multiplier": 0.0},
  {"min_usd": 1,      "max_usd": 12,      "multiplier": 1.0},
  {"min_usd": 12,     "max_usd": 30,      "multiplier": 1.1},
  {"min_usd": 30,     "max_usd": 60,      "multiplier": 1.2},
  {"min_usd": 60,     "max_usd": 120,     "multiplier": 1.3},
  {"min_usd": 120,    "max_usd": 240,     "multiplier": 1.4},
  {"min_usd": 240,    "max_usd": 600,     "multiplier": 1.5},
  {"min_usd": 600,    "max_usd": 1200,    "multiplier": 2.0},
  {"min_usd": 1200,   "max_usd": null,    "multiplier": 3.0}
]
```

### First Run Behavior
If `tiers.json` does not exist at startup, Poller creates it with the defaults above and logs an INFO message. This ensures the service always starts successfully.

### Tier Validation Rules (on dashboard update)
- `min_usd` must be >= 0
- Each tier's `min_usd` must equal the previous tier's `max_usd` (no gaps)
- `multiplier` must be >= 0
- Last tier must have `max_usd: null` (unbounded)
- At least 2 tiers required (one "ignore" tier + one earning tier)
- Tiers must be sorted by `min_usd` ascending

### Formula
```
cents = floor(usd_value * 100)
points = round(cents * tier_multiplier)
```

Points are always **rounded integers** (`math.Round()` in Go).

### Price Fetching
- USD prices are fetched from CoinGecko at **confirmation time** (not detection time)
- Price cache: 60 seconds. If cache is fresh, use cached price
- If CoinGecko is down: retry 3 times with 5s backoff. If still failing, log a system error and leave the tx as PENDING — it will be retried on the next poll
- Price is fetched for the specific token: `bitcoin`, `binancecoin`, `solana` for natives; stablecoins (USDC/USDT) are assumed to be $1.00 (no price fetch needed)

### Stablecoin Price Handling
USDC and USDT are **assumed to be $1.00 per unit**. No CoinGecko lookup needed. This avoids unnecessary API calls and reflects that these are pegged stablecoins. The `usd_price` field in the DB will be `1.0` for stablecoins.

---

## Points Claim Edge Cases

### Claim While Pending Txs Exist
If an address has both `unclaimed` (confirmed) and `pending` (unconfirmed) points:
- `POST /points/claim` only resets `unclaimed` to 0
- `pending` points are NOT touched
- When those pending txs later confirm, they create NEW `unclaimed` points
- The game server will pick them up on the next `GET /api/points` call

### Address Gets New Funds After Claim
After a claim resets an address to 0, that address can absolutely accumulate new points from future transactions. The `total` field keeps growing (lifetime total), only `unclaimed` resets.

### Claim for Non-Existent Address
If `POST /points/claim` includes an address not in the DB or with 0 unclaimed: skip it silently (don't error). Return the count and total of what was actually claimed.

---

## Code Structure

```
poller/
  cmd/
    server/
      main.go                    # Entry point, minimal
  internal/
    api/
      router.go                  # Chi router setup, middleware, go:embed
      handlers/
        watch.go                 # POST /watch, DELETE /watch/:id, GET /watches
        points.go                # GET /points, GET /points/pending, POST /points/claim
        admin.go                 # Login, logout, IP allowlist, settings, tiers
        dashboard.go             # Stats, transactions, charts, errors
      middleware/
        ipallow.go               # IP allowlist middleware (hot-reloadable from DB)
        session.go               # Session cookie auth middleware (in-memory store)
        logging.go               # Request/response logging
    watcher/
      watcher.go                 # Watch orchestrator: manages all active polls
      poller.go                  # Single-address polling loop (goroutine per watch)
      recovery.go                # Startup recovery (expired watches + orphaned pending txs)
    points/
      calculator.go              # USD -> points tier logic (reads tiers.json, cached)
      calculator_test.go         # Test all tier boundaries
    price/
      coingecko.go               # USD price fetching with cache
    provider/
      provider.go                # Blockchain query interface + round-robin
      btc.go                     # BTC: tx list via Blockstream/Mempool
      bsc.go                     # BSC: txlist + tokentx via BscScan, fallback to RPC
      sol.go                     # SOL: getSignaturesForAddress + getTransaction
      ratelimiter.go             # Per-provider rate limiting (token bucket)
    db/
      sqlite.go                  # Connection setup, migrations runner
      migrations/                # Numbered SQL migration files (001_init.sql, etc.)
      watches.go                 # Watch CRUD
      points.go                  # Points ledger (per address)
      transactions.go            # Recorded tx hashes + points awarded
      allowlist.go               # IP allowlist CRUD
      errors.go                  # System errors CRUD
    config/
      config.go                  # Configuration struct (envconfig)
      constants.go               # ALL numeric/string constants
      errors.go                  # ALL error types and codes
    models/
      types.go                   # Shared domain types
    logging/
      logger.go                  # slog setup: stdout + daily file rotation, split by level
    validate/
      address.go                 # Address format validation (BTC/BSC/SOL)
      address_test.go            # Validation tests for each chain + testnet
  web/                           # Dashboard frontend (SvelteKit)
    src/
      lib/
        components/
          ui/                    # shadcn-svelte components
          layout/
            Sidebar.svelte
            Header.svelte
          dashboard/
            StatsCards.svelte
            TimeRangeSelector.svelte
            USDChart.svelte
            PointsChart.svelte
            TxCountChart.svelte
            ChainBreakdown.svelte
            TokenBreakdown.svelte
            TierDistribution.svelte
            WatchesOverTime.svelte
          transactions/
            TransactionTable.svelte
            TransactionFilters.svelte
          watches/
            WatchesTable.svelte
          points/
            PendingPointsTable.svelte
          errors/
            ErrorsList.svelte
            DiscrepancyDetector.svelte
          settings/
            TierEditor.svelte
            IPAllowlist.svelte
            SystemInfo.svelte
            WatchDefaults.svelte
          auth/
            LoginForm.svelte
        stores/
          auth.ts
          stats.ts
          transactions.ts
          watches.ts
          points.ts
          errors.ts
          settings.ts
        utils/
          api.ts                 # API client (single source of truth)
          formatting.ts          # Number/address/date formatting
          validation.ts          # Input validation
        constants.ts
        types.ts
      routes/
        +layout.svelte
        +page.svelte             # Overview (stats + charts)
        login/
          +page.svelte           # Login page
        transactions/
          +page.svelte
        watches/
          +page.svelte
        points/
          +page.svelte           # Pending points
        errors/
          +page.svelte
        settings/
          +page.svelte
      app.css
    static/
  data/                          # SQLite database (created at runtime)
  logs/                          # Log files (created at runtime, split by level)
  tiers.json                     # Points tier config (created with defaults if missing)
  go.mod
  go.sum
  CLAUDE.md
  DESCRIPTION.md
  CHANGELOG.md
```

---

## Code Conventions

### Go Backend

#### Naming Conventions
- **Files**: lowercase with underscores (`btc.go`, `calculator_test.go`)
- **Packages**: short, lowercase, no underscores (`watcher`, `points`, `provider`)
- **Exported**: PascalCase (`StartWatch`, `CalculatePoints`)
- **Unexported**: camelCase (`pollAddress`, `applyTier`)
- **Constants**: PascalCase (`PollIntervalBTC`, `DefaultWatchTimeout`)
- **Errors**: `Err` prefix (`ErrWatchExpired`, `ErrAlreadyWatching`)

#### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to poll BTC address %s: %w", address, err)
}

// All errors defined in config/errors.go — NEVER inline error strings
```

#### Logging

**EVERY SINGLE ACTION MUST BE LOGGED. WE NEVER HAVE ENOUGH LOGS.**

```go
// DEBUG: Poll results, price lookups, tier calculations, API responses
slog.Debug("polling address",
    "chain", "BTC",
    "address", address,
    "pollCount", count,
    "cutoffTime", cutoff,
)

// INFO: Watch start/stop, points awarded, claims, admin actions, startup/shutdown
slog.Info("points awarded",
    "address", address,
    "txHash", txHash,
    "token", token,
    "amountRaw", amountRaw,
    "usdValue", usdValue,
    "usdPrice", usdPrice,
    "tier", tier,
    "multiplier", multiplier,
    "points", points,
)

// WARN: Provider fallback, retries, stale cache, approaching rate limit
slog.Warn("provider rate limited, rotating",
    "provider", provider.Name(),
    "chain", chain,
    "nextProvider", next.Name(),
)

// ERROR: Failed polls, price fetch failures, DB errors, discrepancies
slog.Error("poll failed",
    "chain", "BSC",
    "address", address,
    "provider", provider.Name(),
    "error", err,
    "retryIn", retryDuration,
)
```

**Log output**: Dual output — stdout (terminal) + daily rotated files split by level (matching HDPay: `info.log`, `warn.log`, `error.log`, `debug.log`).

#### Context Usage
- Pass `context.Context` as first parameter everywhere
- Use for cancellation (watch cancellation), timeouts (API calls, poll loops)
- Each watch goroutine gets a context derived from the watcher's context via `context.WithCancel`
- Never store context in structs

#### Database Conventions
- Use `modernc.org/sqlite` (pure Go, no CGO)
- WAL mode enabled at connection time: `PRAGMA journal_mode=WAL`
- Busy timeout: `PRAGMA busy_timeout=5000`
- All migrations in `internal/db/migrations/` as numbered SQL files (`001_init.sql`, `002_add_index.sql`)
- Migration runner at startup: reads current version from `schema_version` table, applies missing migrations in order
- Prepared statements for all queries
- All table/column names in snake_case

---

## Database Schema

```sql
-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY
);

-- Active and historical watches
CREATE TABLE watches (
    id            TEXT PRIMARY KEY,       -- UUID v4
    chain         TEXT NOT NULL,          -- BTC, BSC, SOL
    address       TEXT NOT NULL,
    status        TEXT NOT NULL,          -- ACTIVE, COMPLETED, EXPIRED, CANCELLED
    started_at    DATETIME NOT NULL,
    expires_at    DATETIME NOT NULL,
    completed_at  DATETIME,
    poll_count    INTEGER NOT NULL DEFAULT 0,
    last_poll_at  DATETIME,
    last_poll_result TEXT,               -- JSON: {"new_txs": 0, "pending_txs": 1, "confirmed_txs": 0}
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Points ledger per address (one row per unique address)
CREATE TABLE points (
    address       TEXT NOT NULL,
    chain         TEXT NOT NULL,
    unclaimed     INTEGER NOT NULL DEFAULT 0,   -- Confirmed points waiting for game server claim
    pending       INTEGER NOT NULL DEFAULT 0,   -- Unconfirmed points (informational, not claimable)
    total         INTEGER NOT NULL DEFAULT 0,   -- All-time confirmed points (never decreases)
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (address, chain)                -- Composite PK: same address on different chains = different rows
);

-- Individual transactions (dedup by tx_hash + audit trail)
CREATE TABLE transactions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    watch_id      TEXT NOT NULL REFERENCES watches(id),
    tx_hash       TEXT NOT NULL UNIQUE,   -- Prevent double-counting. For SOL multi-token: "sig:TOKEN"
    chain         TEXT NOT NULL,
    address       TEXT NOT NULL,
    token         TEXT NOT NULL,           -- BTC, BNB, SOL, USDC, USDT
    amount_raw    TEXT NOT NULL,           -- Raw amount in smallest unit (satoshis/wei/lamports/token-units)
    amount_human  TEXT NOT NULL,           -- Human-readable amount (e.g., "0.005", "50.00")
    decimals      INTEGER NOT NULL,        -- Token decimals used for conversion (8 BTC, 18 BNB, 9 SOL, 6 USDC)
    usd_value     REAL NOT NULL,           -- USD value at confirmation time
    usd_price     REAL NOT NULL,           -- Price per unit used (e.g., 97000.00 for BTC, 1.00 for USDC)
    tier          INTEGER NOT NULL,        -- Which tier was applied (0-8)
    multiplier    REAL NOT NULL,           -- Multiplier used (1.0, 1.1, etc.)
    points        INTEGER NOT NULL,        -- Points awarded (rounded integer)
    status        TEXT NOT NULL,           -- PENDING, CONFIRMED
    confirmations INTEGER NOT NULL DEFAULT 0,
    detected_at   DATETIME NOT NULL,       -- When first seen on-chain (block_time or now if mempool)
    confirmed_at  DATETIME,                -- When confirmation threshold reached (NULL while PENDING)
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- IP allowlist (managed from dashboard, read by middleware)
CREATE TABLE ip_allowlist (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    ip            TEXT NOT NULL UNIQUE,     -- IPv4 or IPv6 address (no CIDR for now)
    description   TEXT,                    -- Optional label (e.g., "game-server-prod")
    added_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- System errors / discrepancies (for error dashboard page)
CREATE TABLE system_errors (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    severity      TEXT NOT NULL,           -- ERROR, WARNING
    category      TEXT NOT NULL,           -- PROVIDER, WATCH, POINTS, DISCREPANCY, PRICE, RECOVERY
    message       TEXT NOT NULL,
    details       TEXT,                    -- JSON: additional context
    resolved      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_transactions_created_at ON transactions(created_at);
CREATE INDEX idx_transactions_chain ON transactions(chain);
CREATE INDEX idx_transactions_address ON transactions(address);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_detected_at ON transactions(detected_at);
CREATE INDEX idx_watches_status ON watches(status);
CREATE INDEX idx_watches_address ON watches(address);
CREATE INDEX idx_watches_expires_at ON watches(expires_at);
CREATE INDEX idx_points_unclaimed ON points(unclaimed) WHERE unclaimed > 0;
CREATE INDEX idx_points_pending ON points(pending) WHERE pending > 0;
CREATE INDEX idx_system_errors_resolved ON system_errors(resolved) WHERE resolved = FALSE;
CREATE INDEX idx_system_errors_category ON system_errors(category);
```

### Token Decimals Reference
| Token | Chain | Decimals | amount_raw example | amount_human |
|-------|-------|----------|-------------------|--------------|
| BTC | BTC | 8 | `"168841"` | `"0.00168841"` |
| BNB | BSC | 18 | `"5000000000000000"` | `"0.005"` |
| USDC | BSC | 18 | `"50000000000000000000"` | `"50.00"` |
| USDT | BSC | 18 | `"50000000000000000000"` | `"50.00"` |
| SOL | SOL | 9 | `"1000000000"` | `"1.00"` |
| USDC | SOL | 6 | `"50000000"` | `"50.00"` |
| USDT | SOL | 6 | `"50000000"` | `"50.00"` |

---

## Security

### IP Allowlist Middleware

**Logic** (checked on every API request except `/api/health` and `/api/admin/login`):
```
1. Extract client IP from request (X-Forwarded-For header first, then RemoteAddr)
2. If IP is localhost (127.0.0.1, ::1) → ALLOW
3. If IP is private network (10.x.x.x, 172.16-31.x.x, 192.168.x.x) → ALLOW
4. Query ip_allowlist table (cached in memory, refreshed on every add/remove)
5. If IP is in allowlist → ALLOW
6. Otherwise → 403 Forbidden with ERROR_IP_NOT_ALLOWED
```

**Initial state**: On first run with empty allowlist, only localhost and private-network IPs can access the API. The admin must log into the dashboard (also only accessible from localhost/private-network initially) and add the game server's IP to the allowlist.

**Cache**: The allowlist is loaded into an `sync.RWMutex`-protected `map[string]bool` at startup and refreshed whenever the dashboard adds/removes an IP. No DB query per request.

### Session Auth (Dashboard)

**Storage**: Sessions are stored **in-memory** (`map[string]*Session` protected by `sync.RWMutex`). They are lost on restart — the admin must re-login. This is acceptable because the session timeout is only 1 hour.

**Login flow**:
```
1. POST /api/admin/login {username, password}
2. Compare username with POLLER_ADMIN_USERNAME (exact match)
3. Compare password with bcrypt hash of POLLER_ADMIN_PASSWORD
4. If match: generate random session token (32 bytes, hex-encoded)
5. Store session: {token, created_at, expires_at: now + 1h}
6. Set cookie: poller_session={token}; HttpOnly; SameSite=Strict; Path=/
7. Return 200 OK
```

**Password hashing**: On startup, Poller bcrypt-hashes the plaintext password from `.env` and holds the hash in memory. The plaintext is never stored. The `.env` file contains the plaintext for simplicity — this is acceptable because the `.env` file is already a sensitive file that should be protected.

**Session middleware**:
```
1. Read cookie "poller_session"
2. Look up token in session store
3. If not found or expired → 401 with ERROR_SESSION_EXPIRED
4. If valid → attach session to context, proceed
```

### CORS
Since the API is internet-facing (protected by IP allowlist) and the dashboard is served from the same origin:
- Dashboard routes: same-origin, no CORS issues
- API routes called by game server: set `Access-Control-Allow-Origin: *` (safe because IP allowlist is the security boundary, not CORS)
- Expose `Content-Type` and standard headers

---

## Discrepancy Detection

The errors/discrepancy page catches problems automatically. These checks run on demand when the dashboard errors page is loaded.

### Check 1: Points Sum Mismatch
```sql
-- For each address: does the sum of confirmed tx points match the total in the points table?
SELECT t.address, t.chain,
       SUM(t.points) as calculated_total,
       p.total as stored_total
FROM transactions t
JOIN points p ON t.address = p.address AND t.chain = p.chain
WHERE t.status = 'CONFIRMED'
GROUP BY t.address, t.chain
HAVING calculated_total != stored_total;
```
If any rows returned → DISCREPANCY error: "Points total mismatch for address X: calculated N, stored M"

### Check 2: Unclaimed Exceeds Total
```sql
SELECT address, chain, unclaimed, total
FROM points
WHERE unclaimed > total;
```
If any rows → DISCREPANCY error: "Unclaimed exceeds total for address X"

### Check 3: Orphaned Transactions
```sql
-- Transactions referencing watches that don't exist
SELECT t.id, t.tx_hash, t.watch_id
FROM transactions t
LEFT JOIN watches w ON t.watch_id = w.id
WHERE w.id IS NULL;
```

### Check 4: Stale Pending Transactions
```sql
-- PENDING transactions older than 24 hours (something is wrong)
SELECT * FROM transactions
WHERE status = 'PENDING'
AND detected_at < datetime('now', '-24 hours');
```
If any → WARNING: "Transaction {tx_hash} has been PENDING for over 24 hours"

### Check 5: Provider Errors (from system_errors table)
```sql
SELECT category, COUNT(*) as count, MAX(created_at) as latest
FROM system_errors
WHERE resolved = FALSE
GROUP BY category;
```

All discrepancies are logged to `system_errors` table with category `DISCREPANCY` when detected.

---

## API Design

### Security Layers

**API endpoints** (`/api/watch`, `/api/points`, etc.):
- Localhost / private-network: always allowed
- Internet: IP must be in allowlist (stored in DB, cached in memory)
- No API keys or tokens needed

**Admin/Dashboard endpoints** (`/api/admin/*`, `/api/dashboard/*`):
- Session cookie required (username + password login)
- 1-hour session timeout
- On first run, only accessible from localhost/private-network (until IPs are added to allowlist)

### Endpoints

```
# Watch Management (IP-restricted)
POST   /api/watch                      # Start watching an address
DELETE /api/watch/:id                  # Cancel a watch
GET    /api/watches                    # List watches (filterable: status, chain)

# Points (IP-restricted)
GET    /api/points                     # Accounts with unclaimed confirmed points
GET    /api/points/pending             # Accounts with pending (unconfirmed) points
POST   /api/points/claim              # Claim points for explicit list of addresses

# Auth (IP-restricted, no session needed)
POST   /api/admin/login                # Login (username + password -> session cookie)
POST   /api/admin/logout               # Logout (clear session)

# Admin (session-authenticated + IP-restricted)
GET    /api/admin/allowlist            # View IP allowlist
POST   /api/admin/allowlist            # Add IP to allowlist
DELETE /api/admin/allowlist/:id        # Remove IP from allowlist
GET    /api/admin/settings             # View all settings
PUT    /api/admin/tiers                # Update tier config (writes tiers.json)
PUT    /api/admin/watch-defaults       # Update watch defaults

# Dashboard Data (session-authenticated + IP-restricted)
GET    /api/dashboard/stats            # Stats with time range query param
GET    /api/dashboard/transactions     # Transaction history (paginated, filterable)
GET    /api/dashboard/charts           # Chart data for all visualizations
GET    /api/dashboard/errors           # System errors and discrepancies (runs checks)

# System (no restriction)
GET    /api/health                     # Health check (always open)
```

### Request/Response Formats

#### POST /api/watch
```json
// Request
{
  "chain": "BTC",
  "address": "bc1q...",
  "timeout_minutes": 30       // Optional, defaults to POLLER_DEFAULT_WATCH_TIMEOUT_MIN
}

// Response — 201 Created
{
  "data": {
    "watch_id": "550e8400-e29b-41d4-a716-446655440000",
    "chain": "BTC",
    "address": "bc1q...",
    "status": "ACTIVE",
    "started_at": "2026-02-20T12:00:00Z",
    "expires_at": "2026-02-20T12:30:00Z",
    "poll_interval_seconds": 60
  }
}

// Error — 409 Conflict (already watching)
{
  "error": {
    "code": "ERROR_ALREADY_WATCHING",
    "message": "Address bc1q... already has an active watch (id: 550e8400...)"
  }
}

// Error — 400 Bad Request (invalid address)
{
  "error": {
    "code": "ERROR_ADDRESS_INVALID",
    "message": "Invalid BTC address format: xyz123"
  }
}

// Error — 429 Too Many (max watches)
{
  "error": {
    "code": "ERROR_MAX_WATCHES",
    "message": "Maximum active watches (100) reached"
  }
}
```

#### GET /api/points
```json
// Response — only accounts with unclaimed > 0
{
  "data": [
    {
      "address": "bc1q...",
      "chain": "BTC",
      "unclaimed": 13000,
      "total": 25000,
      "transactions": [
        {
          "tx_hash": "abc123def...",
          "token": "BTC",
          "amount_raw": "168841",
          "amount_human": "0.00168841",
          "usd_value": 100.00,
          "usd_price": 59250.00,
          "tier": 4,
          "multiplier": 1.3,
          "points": 13000,
          "confirmed_at": "2026-02-20T12:15:00Z"
        }
      ]
    }
  ]
}
```

#### GET /api/points/pending
```json
// Response — accounts with pending > 0 (unconfirmed txs)
{
  "data": [
    {
      "address": "0xF278...",
      "chain": "BSC",
      "pending_points": 6000,
      "transactions": [
        {
          "tx_hash": "0xdef456...",
          "token": "USDC",
          "amount_raw": "50000000000000000000",
          "amount_human": "50.00",
          "usd_value": 50.00,
          "tier": 3,
          "points": 6000,
          "status": "PENDING",
          "confirmations": 5,
          "confirmations_required": 12,
          "detected_at": "2026-02-20T12:10:00Z"
        }
      ]
    }
  ]
}
```

#### POST /api/points/claim
```json
// Request — explicit list of addresses
{
  "addresses": ["bc1q...", "0xF278..."]
}

// Response — 200 OK
{
  "data": {
    "claimed": [
      {"address": "bc1q...", "chain": "BTC", "points_claimed": 13000},
      {"address": "0xF278...", "chain": "BSC", "points_claimed": 6000}
    ],
    "skipped": [],                // Addresses with 0 unclaimed (not an error)
    "total_claimed": 19000
  }
}
```

#### GET /api/dashboard/stats?range=week
```json
{
  "data": {
    "range": "week",
    "active_watches": 5,
    "total_watches": 147,
    "watches_completed": 128,
    "watches_expired": 14,
    "usd_received": 4540.50,
    "points_awarded": 598000,
    "pending_points": {
      "accounts": 3,
      "total": 45000
    },
    "unique_addresses": 89,
    "avg_tx_usd": 36.68,
    "largest_tx_usd": 500.00,
    "by_day": [
      {"date": "2026-02-14", "usd": 620.00, "points": 78000, "txs": 8},
      {"date": "2026-02-15", "usd": 840.50, "points": 112000, "txs": 12},
      // ... one entry per day in the range
    ]
  }
}
```

**Time range query param values**:
- `today`: current calendar day (UTC)
- `week`: last 7 days
- `month`: last 30 days
- `quarter`: last 90 days
- `all`: all-time (since START_DATE)

The `by_day` array provides daily granularity for charts regardless of the selected range.

#### GET /api/dashboard/charts
```json
{
  "data": {
    "usd_over_time": [
      {"date": "2026-02-14", "usd": 620.00}
    ],
    "points_over_time": [
      {"date": "2026-02-14", "points": 78000}
    ],
    "tx_count_over_time": [
      {"date": "2026-02-14", "count": 8}
    ],
    "by_chain": [
      {"chain": "BTC", "usd": 1200.00, "count": 15},
      {"chain": "BSC", "usd": 2100.50, "count": 42},
      {"chain": "SOL", "usd": 1240.00, "count": 31}
    ],
    "by_token": [
      {"token": "BTC", "usd": 1200.00, "count": 15},
      {"token": "BNB", "usd": 800.00, "count": 20},
      {"token": "USDC", "usd": 1500.50, "count": 35},
      {"token": "USDT", "usd": 1040.00, "count": 18}
    ],
    "by_tier": [
      {"tier": 1, "count": 25, "total_points": 15000},
      {"tier": 2, "count": 18, "total_points": 28000},
      // ... one per tier that has transactions
    ],
    "watches_over_time": [
      {"date": "2026-02-14", "active": 3, "completed": 8, "expired": 1}
    ]
  }
}
```

#### GET /api/dashboard/errors
```json
{
  "data": {
    "discrepancies": [
      {
        "type": "POINTS_MISMATCH",
        "address": "bc1q...",
        "chain": "BTC",
        "calculated": 13000,
        "stored": 12500,
        "message": "Points total mismatch"
      }
    ],
    "errors": [
      {
        "id": 1,
        "severity": "ERROR",
        "category": "PROVIDER",
        "message": "BscScan API returned 502 for 3 consecutive requests",
        "details": "{\"provider\": \"BscScan\", \"status_code\": 502}",
        "resolved": false,
        "created_at": "2026-02-20T10:30:00Z"
      }
    ],
    "stale_pending": [
      {
        "tx_hash": "0xabc...",
        "chain": "BSC",
        "address": "0x123...",
        "detected_at": "2026-02-19T08:00:00Z",
        "hours_pending": 28
      }
    ]
  }
}
```

### Response Format (standard)
```json
// Success (single object)
{"data": { ... }}

// Success (list with pagination)
{"data": [...], "meta": {"page": 1, "pageSize": 50, "total": 120}}

// Error
{"error": {"code": "ERROR_CODE_HERE", "message": "Human-readable message"}}
```

HTTP status codes:
- 200: Success
- 201: Created (POST /watch)
- 400: Bad request (invalid input)
- 401: Unauthorized (session expired/missing)
- 403: Forbidden (IP not allowed)
- 404: Not found
- 409: Conflict (already watching)
- 429: Too many (max watches reached)
- 500: Internal server error

---

## Constants

### Go (`internal/config/constants.go`)
```go
package config

import "time"

// Polling Intervals
const (
    PollIntervalBTC = 60 * time.Second
    PollIntervalBSC = 5 * time.Second
    PollIntervalSOL = 5 * time.Second
)

// Confirmation Thresholds
const (
    ConfirmationsBTC = 1
    ConfirmationsBSC = 12
    // SOL uses "finalized" commitment level (no numeric threshold)
    SOLCommitment = "finalized"
)

// Token Decimals
const (
    DecimalsBTC     = 8
    DecimalsBNB     = 18
    DecimalsBSCUSDC = 18
    DecimalsBSCUSDT = 18
    DecimalsSOL     = 9
    DecimalsSOLUSDC = 6
    DecimalsSOLUSDT = 6
)

// Watch Defaults
const (
    DefaultWatchTimeoutMinutes = 30
    MaxWatchTimeoutMinutes     = 120
    MaxActiveWatches           = 100
)

// Token Contract Addresses — BSC Mainnet
const (
    BSC_USDC_Contract = "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"
    BSC_USDT_Contract = "0x55d398326f99059fF775485246999027B3197955"
)

// Token Contract Addresses — BSC Testnet
const (
    BSC_Testnet_USDC_Contract = "" // Set when available
    BSC_Testnet_USDT_Contract = "" // Set when available
)

// Token Mint Addresses — SOL Mainnet
const (
    SOL_USDC_Mint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
    SOL_USDT_Mint = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
)

// Token Mint Addresses — SOL Devnet
const (
    SOL_Devnet_USDC_Mint = "" // Set when available
    SOL_Devnet_USDT_Mint = "" // Set when available
)

// Rate Limiting (per provider)
const (
    RateLimit_Blockstream  = 10  // requests/second
    RateLimit_Mempool      = 10
    RateLimit_BscScan      = 5
    RateLimit_BscRPC       = 10
    RateLimit_AnkrBSC      = 30
    RateLimit_SolanaRPC    = 10
    RateLimit_Helius       = 10
    RateLimit_CoinGecko    = 10  // requests/minute
)

// Server
const (
    ServerPort           = 8081  // Different from HDPay (8080)
    ServerReadTimeout    = 30 * time.Second
    ServerWriteTimeout   = 60 * time.Second
    APITimeout           = 30 * time.Second
)

// Session
const (
    SessionCookieName  = "poller_session"
    SessionTimeout     = 1 * time.Hour
    SessionTokenLength = 32  // bytes, hex-encoded = 64 chars
)

// Database
const (
    DBPath        = "./data/poller.sqlite"
    DBTestPath    = "./data/poller_test.sqlite"
    DBWALMode     = true
    DBBusyTimeout = 5000 // milliseconds
)

// Price
const (
    CoinGeckoBaseURL   = "https://api.coingecko.com/api/v3"
    PriceCacheDuration = 60 * time.Second
    PriceRetryCount    = 3
    PriceRetryDelay    = 5 * time.Second
    StablecoinPrice    = 1.0  // USDC and USDT assumed $1.00
)

// CoinGecko IDs
const (
    CoinGeckoID_BTC = "bitcoin"
    CoinGeckoID_BNB = "binancecoin"
    CoinGeckoID_SOL = "solana"
)

// Logging
const (
    LogDir         = "./logs"
    LogFilePattern = "poller-%s-%s.log" // level-date
    LogMaxAgeDays  = 30
)

// Tiers
const (
    TiersConfigFile = "./tiers.json"
    MinTierCount    = 2  // At least one "ignore" tier + one earning tier
)

// Recovery
const (
    RecoveryPendingRetries  = 3
    RecoveryPendingInterval = 30 * time.Second
    StalePendingThreshold   = 24 * time.Hour
)

// Graceful Shutdown
const (
    ShutdownTimeout = 10 * time.Second  // Max time to wait for goroutines to drain
)

// Pagination
const (
    DefaultPageSize = 50
    MaxPageSize     = 200
)
```

### Go (`internal/config/errors.go`)
```go
package config

const (
    ERROR_ALREADY_WATCHING     = "ERROR_ALREADY_WATCHING"
    ERROR_WATCH_NOT_FOUND      = "ERROR_WATCH_NOT_FOUND"
    ERROR_WATCH_EXPIRED        = "ERROR_WATCH_EXPIRED"
    ERROR_ADDRESS_NOT_FOUND    = "ERROR_ADDRESS_NOT_FOUND"
    ERROR_ADDRESS_INVALID      = "ERROR_ADDRESS_INVALID"
    ERROR_INVALID_CHAIN        = "ERROR_INVALID_CHAIN"
    ERROR_INVALID_TOKEN        = "ERROR_INVALID_TOKEN"
    ERROR_INVALID_TIMEOUT      = "ERROR_INVALID_TIMEOUT"
    ERROR_MAX_WATCHES          = "ERROR_MAX_WATCHES"
    ERROR_TX_ALREADY_RECORDED  = "ERROR_TX_ALREADY_RECORDED"
    ERROR_NOTHING_TO_CLAIM     = "ERROR_NOTHING_TO_CLAIM"
    ERROR_PROVIDER_UNAVAILABLE = "ERROR_PROVIDER_UNAVAILABLE"
    ERROR_PROVIDER_RATE_LIMIT  = "ERROR_PROVIDER_RATE_LIMIT"
    ERROR_PRICE_FETCH_FAILED   = "ERROR_PRICE_FETCH_FAILED"
    ERROR_DATABASE             = "ERROR_DATABASE"
    ERROR_TIERS_INVALID        = "ERROR_TIERS_INVALID"
    ERROR_TIERS_FILE           = "ERROR_TIERS_FILE"
    ERROR_UNAUTHORIZED         = "ERROR_UNAUTHORIZED"
    ERROR_FORBIDDEN            = "ERROR_FORBIDDEN"
    ERROR_SESSION_EXPIRED      = "ERROR_SESSION_EXPIRED"
    ERROR_IP_NOT_ALLOWED       = "ERROR_IP_NOT_ALLOWED"
    ERROR_INVALID_CREDENTIALS  = "ERROR_INVALID_CREDENTIALS"
    ERROR_DISCREPANCY          = "ERROR_DISCREPANCY"
    ERROR_INVALID_REQUEST      = "ERROR_INVALID_REQUEST"
    ERROR_INTERNAL             = "ERROR_INTERNAL"
)
```

### TypeScript (`web/src/lib/constants.ts`)
```typescript
export const API_BASE = '/api';

// Display
export const BALANCE_DECIMAL_PLACES = 6;
export const ADDRESS_TRUNCATE_LENGTH = 8;

// Chains
export const SUPPORTED_CHAINS = ['BTC', 'BSC', 'SOL'] as const;
export const CHAIN_COLORS = { BTC: '#f7931a', BSC: '#F0B90B', SOL: '#9945FF' } as const;
export const CHAIN_NATIVE_SYMBOLS = { BTC: 'BTC', BSC: 'BNB', SOL: 'SOL' } as const;
export const CHAIN_TOKENS = {
  BTC: ['BTC'],
  BSC: ['BNB', 'USDC', 'USDT'],
  SOL: ['SOL', 'USDC', 'USDT'],
} as const;

// Watch statuses
export const WATCH_STATUSES = ['ACTIVE', 'COMPLETED', 'EXPIRED', 'CANCELLED'] as const;
export const STATUS_COLORS = {
  ACTIVE: '#3b82f6',
  COMPLETED: '#10b981',
  EXPIRED: '#ef4444',
  CANCELLED: '#6b7280',
} as const;

// Transaction statuses
export const TX_STATUSES = ['PENDING', 'CONFIRMED'] as const;
export const TX_STATUS_COLORS = {
  PENDING: '#f59e0b',
  CONFIRMED: '#10b981',
} as const;

// Time ranges for dashboard
export const TIME_RANGES = ['today', 'week', 'month', 'quarter', 'all'] as const;
export const TIME_RANGE_LABELS = {
  today: 'Today',
  week: 'This Week',
  month: 'This Month',
  quarter: 'This Quarter',
  all: 'All Time',
} as const;

// Chart colors
export const CHART_COLORS = ['#f7931a', '#F0B90B', '#9945FF', '#3b82f6', '#10b981'] as const;

// Block explorer URLs (mainnet)
export const EXPLORER_TX_URL = {
  BTC: 'https://blockstream.info/tx/',
  BSC: 'https://bscscan.com/tx/',
  SOL: 'https://solscan.io/tx/',
} as const;

// Block explorer URLs (testnet)
export const EXPLORER_TX_URL_TESTNET = {
  BTC: 'https://blockstream.info/testnet/tx/',
  BSC: 'https://testnet.bscscan.com/tx/',
  SOL: 'https://solscan.io/tx/?cluster=devnet/',
} as const;

// Error codes (mirror backend)
export const ERROR_ALREADY_WATCHING = 'ERROR_ALREADY_WATCHING';
export const ERROR_ADDRESS_INVALID = 'ERROR_ADDRESS_INVALID';
export const ERROR_MAX_WATCHES = 'ERROR_MAX_WATCHES';
export const ERROR_SESSION_EXPIRED = 'ERROR_SESSION_EXPIRED';
export const ERROR_INVALID_CREDENTIALS = 'ERROR_INVALID_CREDENTIALS';
export const ERROR_IP_NOT_ALLOWED = 'ERROR_IP_NOT_ALLOWED';
// ... mirror all backend error codes
```

---

## Environment Variables

```env
# .env.example
POLLER_DB_PATH=./data/poller.sqlite
POLLER_PORT=8081
POLLER_LOG_LEVEL=info
POLLER_LOG_DIR=./logs
POLLER_NETWORK=mainnet                    # mainnet, testnet

# Global start date — ignore all on-chain transactions before this (unix timestamp)
POLLER_START_DATE=1740000000

# Dashboard auth (REQUIRED)
POLLER_ADMIN_USERNAME=admin
POLLER_ADMIN_PASSWORD=changeme            # Plaintext in .env, bcrypt-hashed in memory at startup

# API Keys (free tier, optional but recommended)
POLLER_BSCSCAN_API_KEY=
POLLER_HELIUS_API_KEY=

# Optional overrides
POLLER_MAX_ACTIVE_WATCHES=100
POLLER_DEFAULT_WATCH_TIMEOUT_MIN=30
POLLER_TIERS_FILE=./tiers.json
```

```go
type Config struct {
    DBPath              string `envconfig:"POLLER_DB_PATH" default:"./data/poller.sqlite"`
    Port                int    `envconfig:"POLLER_PORT" default:"8081"`
    LogLevel            string `envconfig:"POLLER_LOG_LEVEL" default:"info"`
    LogDir              string `envconfig:"POLLER_LOG_DIR" default:"./logs"`
    Network             string `envconfig:"POLLER_NETWORK" default:"mainnet"`
    StartDate           int64  `envconfig:"POLLER_START_DATE" required:"true"`
    AdminUsername       string `envconfig:"POLLER_ADMIN_USERNAME" required:"true"`
    AdminPassword       string `envconfig:"POLLER_ADMIN_PASSWORD" required:"true"`
    BscScanAPIKey       string `envconfig:"POLLER_BSCSCAN_API_KEY"`
    HeliusAPIKey        string `envconfig:"POLLER_HELIUS_API_KEY"`
    MaxActiveWatches    int    `envconfig:"POLLER_MAX_ACTIVE_WATCHES" default:"100"`
    DefaultWatchTimeout int    `envconfig:"POLLER_DEFAULT_WATCH_TIMEOUT_MIN" default:"30"`
    TiersFile           string `envconfig:"POLLER_TIERS_FILE" default:"./tiers.json"`
}
```

---

## Watcher Design

### Orchestrator Pattern
```go
type Watcher struct {
    mu        sync.RWMutex
    watches   map[string]*ActiveWatch  // watchID -> ActiveWatch
    db        *DB
    providers map[Chain][]Provider
    pricer    *PriceService
    calc      *PointsCalculator
    cfg       *Config
    wg        sync.WaitGroup           // Track goroutines for graceful shutdown
}

type ActiveWatch struct {
    ID        string
    Chain     Chain
    Address   string
    ExpiresAt time.Time
    PollCount int
    Cancel    context.CancelFunc       // Cancel the polling goroutine
}
```

### Polling Goroutine (per watch)
```go
func (w *Watcher) pollLoop(ctx context.Context, watch *ActiveWatch) {
    defer w.wg.Done()

    // 1. Determine cutoff timestamp
    cutoff := w.getCutoff(watch.Address, watch.Chain)

    interval := w.getPollInterval(watch.Chain)
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            // Cancelled or shutdown
            return
        case <-ticker.C:
            // 2. Query blockchain for txs after cutoff
            newTxs, err := w.detectTransactions(ctx, watch, cutoff)
            if err != nil {
                slog.Error("poll failed", "watchID", watch.ID, "error", err)
                w.recordSystemError("PROVIDER", err.Error())
                continue // Don't stop watching on transient errors
            }

            // 3. Process new transactions
            for _, tx := range newTxs {
                w.processTransaction(ctx, watch, tx)
            }

            // 4. Re-check pending txs for confirmation
            w.recheckPending(ctx, watch)

            // 5. Update watch metadata
            watch.PollCount++
            w.updateWatchPollResult(watch, newTxs)

            // 6. Check stop conditions
            if time.Now().After(watch.ExpiresAt) {
                w.expireWatch(watch)
                return
            }
            if w.allConfirmedAndAtLeastOne(watch) {
                w.completeWatch(watch)
                return
            }
        }
    }
}
```

### Provider Failure Handling
When a blockchain API call fails:
1. Log the error
2. Rotate to next provider (round-robin)
3. If ALL providers for a chain fail: log system error, continue polling on next tick
4. NEVER stop a watch due to provider failures — keep retrying

### Graceful Shutdown
```go
func (w *Watcher) Shutdown(ctx context.Context) {
    // 1. Cancel all watch contexts
    w.mu.Lock()
    for _, watch := range w.watches {
        watch.Cancel()
    }
    w.mu.Unlock()

    // 2. Wait for goroutines with timeout
    done := make(chan struct{})
    go func() { w.wg.Wait(); close(done) }()

    select {
    case <-done:
        slog.Info("all watches stopped gracefully")
    case <-time.After(ShutdownTimeout):
        slog.Warn("shutdown timeout reached, some watches may not have finished")
    }

    // 3. Mark remaining active watches as EXPIRED in DB
    w.db.ExpireAllActiveWatches()
}
```

---

## Address Validation

Validate address format only — no on-chain check, no HDPay lookup.

### BTC
- **Mainnet**: starts with `bc1` (bech32/bech32m), `1` (P2PKH), or `3` (P2SH)
- **Testnet**: starts with `tb1` (bech32), `m` or `n` (P2PKH), or `2` (P2SH)
- Use `btcd/btcutil` for proper bech32 decoding and checksum validation

### BSC (EVM)
- Must start with `0x` followed by exactly 40 hexadecimal characters
- EIP-55 checksum validation optional (accept both checksummed and lowercased)
- Same format for mainnet and testnet

### SOL
- Base58 encoded, 32-44 characters
- Must decode to exactly 32 bytes (ed25519 public key)
- Same format for mainnet and devnet

---

## Dashboard (Frontend)

### Pages

#### 1. Overview (`/`)
Stats cards with **time range selector** (today / week / month / quarter / all-time):
- Active watches count
- Total watches (completed + expired + active)
- USD received
- Points awarded
- Pending points (unclaimed by game server)
- Unique funded addresses
- Average transaction size (USD)
- Largest transaction

Charts (using data from `GET /api/dashboard/charts`):
- USD received over time (bar chart, daily granularity)
- Points awarded over time (line chart)
- Transaction count over time (bar chart)
- Breakdown by chain (pie chart: BTC vs BSC vs SOL)
- Breakdown by token (pie chart: BTC/BNB/SOL/USDC/USDT)
- Active watches over time (line chart)
- Tier distribution (bar chart: how many txs per tier)

#### 2. Transactions (`/transactions`)
Full transaction history table with all columns:
- Timestamp (detected_at), address (truncated, hover for full), chain, token, crypto amount (amount_human), USD value, tier, points, TX hash (clickable link to block explorer), watch ID, status (PENDING/CONFIRMED with color badge)

Filters (all combinable):
- Chain (dropdown: BTC/BSC/SOL)
- Token (dropdown: BTC/BNB/SOL/USDC/USDT)
- Date range (date pickers: from/to)
- Tier (dropdown: 0-8)
- Status (dropdown: PENDING/CONFIRMED)
- Min USD / Max USD (number inputs)

Pagination: server-side, page size selector (25/50/100)

#### 3. Watches (`/watches`)
Active and historical watches table:
- Address, chain, status (color badge), time started, time remaining (countdown for ACTIVE, "—" for others), poll count, last poll result (e.g., "2 new txs, 1 pending")

Filterable by status and chain.

#### 4. Pending Points (`/points`)
Accounts with unclaimed points:
- Address, chain, unclaimed points, pending points, transaction count, last tx timestamp, all-time total

#### 5. Errors (`/errors`)
System health and discrepancy detection:
- **Discrepancies**: points mismatches, unclaimed > total, orphaned transactions (auto-detected)
- **Stale Pending**: transactions pending for > 24 hours
- **System Errors**: provider failures, recovery issues, from `system_errors` table
- Each error shows severity, category, message, timestamp, resolved status

#### 6. Settings (`/settings`)
- **Points Tier Editor**: Table showing current tiers with editable boundaries and multipliers. Save button writes to `tiers.json`. Validation rules enforced client-side and server-side.
- **IP Allowlist**: Table of allowed IPs with description. Add/remove buttons.
- **START_DATE**: Display only (set in `.env`, not editable from UI)
- **System Info**: Uptime, version, network mode (mainnet/testnet), active watch count, DB size
- **Watch Defaults**: Default timeout (minutes), max active watches. Saved to DB, take effect immediately.

### Style
Match HDPay's dashboard style. Manual refresh only (no auto-refresh, no SSE).

---

## Testing Requirements

### Every feature MUST have tests. No exceptions.

#### Critical Path Tests
- **Points calculator**: all tier boundaries ($0.99→0pts, $1.00→100pts, $11.99→1199pts, $12.00→1320pts, $1199.99, $1200.00), rounding behavior, tiers.json loading, validation
- **Watch lifecycle**: create → poll → detect pending → confirm → complete; create → timeout → expire; create → cancel; duplicate address rejection
- **IP allowlist**: localhost always allowed, private IPs allowed, unknown IP rejected, added IP allowed, removed IP rejected, cache refresh
- **Session auth**: login success, login failure (wrong user/pass), session timeout, logout, cookie validation
- **Address validation**: valid/invalid for each chain (mainnet + testnet formats)
- **Transaction dedup**: same tx_hash rejected, SOL composite key (tx_hash:token) works
- **Provider round-robin**: rotation on failure, all-providers-down handling
- **Startup recovery**: active watches → expired, pending txs → re-checked
- **Discrepancy detection**: all 5 checks return correct results

#### Coverage Target
- Minimum 70% for core packages (`watcher`, `points`, `provider`, `db`, `validate`)
- `go test -cover ./...`

---

## Startup Sequence

```
1. Load .env via envconfig → validate required fields
2. Initialize slog (stdout + split log files)
3. Open/create SQLite database → run migrations
4. Load tiers.json (create with defaults if missing)
5. Bcrypt-hash admin password from .env → store in memory
6. Load IP allowlist from DB → populate in-memory cache
7. Initialize price service (CoinGecko client with cache)
8. Initialize providers per chain (round-robin sets)
9. Initialize watcher orchestrator
10. Run startup recovery:
    a. Expire interrupted active watches
    b. Re-check orphaned pending transactions
11. Setup Chi router with middleware (IP allowlist, session, logging)
12. Embed and serve SvelteKit static build
13. Start HTTP server on POLLER_PORT
14. Listen for SIGTERM/SIGINT → graceful shutdown
```

---

## Commit Strategy

### Granular commits after every meaningful change

```
<type>: <short description>
```

Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

---

## AI Agent Instructions

When working on this codebase:

### CRITICAL: Always Changelog + Commit After Every Task
1. Update `CHANGELOG.md` with what changed
2. Commit all modified files
3. Never leave uncommitted work

### Before Adding Any Function
1. `grep -r "functionName" internal/` (this project)
2. `grep -r "functionName" ../internal/` (HDPay — check for reusable code)
3. If exists -> import or adapt
4. If new but reusable -> put in proper utils package

### Constants Are Sacred
- NEVER hardcode a number, string, URL, timeout, error message, or limit
- ALL values go in `internal/config/constants.go` or `web/src/lib/constants.ts`
- Error codes go in `internal/config/errors.go`

### Workflow Checklist
1. **Read before writing**: Always read existing code before modifying
2. **Follow patterns**: Match existing code style exactly
3. **Test everything**: Every feature needs tests
4. **Commit frequently**: After each meaningful change
5. **Update changelog**: Before committing
6. **No hardcoding**: Use constants files
7. **Check reusability**: Don't duplicate utilities (check HDPay too)
8. **Type everything**: No `any`, explicit return types (TypeScript)
9. **Handle errors**: Wrap with context, log at appropriate level
10. **Security first**: No private keys, IP allowlist enforced, sessions validated
11. **Log everything**: Every action, state change, API call, and error must be logged
