# Poller — Crypto-to-Points Microservice

## What Is This?

Poller is a standalone microservice that monitors cryptocurrency addresses for incoming payments and converts them into **points** for a video game economy. It has zero runtime dependency on HDPay — it watches whatever address is passed to it via API, detects incoming transactions across all supported tokens, and awards points based on a configurable USD-to-points tier system.

**The loop**: Player deposits crypto -> Game server tells Poller to watch -> Poller detects the transaction -> USD value calculated -> Points awarded -> Game server picks them up -> Points credited in-game.

---

## How It Works

### The Full Flow

```
Game Server                     Poller                        Blockchain
    |                              |                              |
    |-- POST /watch (address) ---->|                              |
    |                              | 1. Look up last known tx     |
    |                              |    for this address in DB    |
    |                              | 2. Query on-chain txs after  |
    |                              |    that date (or START_DATE) |
    |                              |                              |
    |                              |-- check for new txs -------->|
    |                              |<-- pending tx found! --------|
    |                              |                              |
    |                              | 3. Record as PENDING         |
    |                              |    (pending_points calculated)|
    |                              |                              |
    |-- GET /points/pending ------>|                              |
    |<-- [{addr, pending_pts}] ----|                              |
    |                              |                              |
    | (shows "payment detected"    |-- poll for confirmations --->|
    |  in game UI)                 |<-- confirmed! ---------------|
    |                              |                              |
    |                              | 4. Calculate USD value       |
    |                              | 5. Apply points tier         |
    |                              | 6. PENDING -> CONFIRMED      |
    |                              |    pending_points -> points   |
    |                              | 7. Record tx_hash (dedup)    |
    |                              |                              |
    |-- GET /points --------------->|                              |
    |<-- [{addr, points, txs}] ----|                              |
    |                              |                              |
    | (credits points in-game)     |                              |
    |                              |                              |
    |-- POST /points/claim -------->|                              |
    |     {addresses: [...]}       | 8. Reset claimed to 0        |
    |<-- OK ------------------------|                              |
```

### Step by Step

1. **A player wants to buy in-game currency.** The game server assigns a crypto address to the player and shows it in the game UI.

2. **The game server tells Poller to watch.** `POST /watch` with the address, chain, and a timeout (default 30 min). Poller begins monitoring.

3. **Poller checks for new transactions.** It looks up the last transaction it recorded for this address in its own DB. If none exists, it uses the global `START_DATE` from `.env`. It then queries the blockchain for all transactions on this address after that cutoff — catching everything, including transactions that arrived while Poller was offline.

4. **Transaction detected (unconfirmed).** Poller records it as PENDING and calculates `pending_points` (informational — the game server can show "payment detected, waiting for confirmation").

5. **Transaction confirmed.** Once the required number of confirmations is reached (BTC: 1, BSC: 12, SOL: finalized), Poller fetches the current USD price, calculates the dollar value, applies the points tier, and records the result. The tx hash is stored to prevent double-counting.

6. **Watch expires.** If the timeout is reached, the watch is simply cancelled. No notification, no retry.

7. **Game server collects confirmed points.** Periodically calls `GET /api/points` to fetch all accounts with unclaimed confirmed points.

8. **Game server collects pending points.** Calls `GET /api/points/pending` every 10-15s to show "incoming payment" in the game UI.

9. **Game server confirms claim.** After crediting points in-game, calls `POST /api/points/claim` with the explicit list of addresses it handled. Those accounts are reset to zero.

---

## Smart Transaction Detection

Poller doesn't blindly re-scan everything. It's intelligent about what to check:

### On Watch Request (`POST /watch`)
1. Look up this address in Poller's DB — find the timestamp of the last recorded transaction
2. If no history exists, use `START_DATE` (global, set in `.env` at deployment time)
3. Query blockchain for all transactions on this address after that cutoff
4. Any tx not already in Poller's DB with a supported token → process it
5. Each tx is evaluated independently for tier (a $10 + $40 = tier 1 + tier 3, not combined)

### On Startup (Recovery)
Poller doesn't re-scan all known addresses. It only handles incomplete state:
- **Active watches** that were interrupted by shutdown → mark as EXPIRED
- **PENDING transactions** that never got their confirmation status updated → re-check their confirmation status on-chain

### Token Detection
Poller checks for ALL supported tokens on a watched address in a single operation where possible:
- **BTC**: Only native BTC (no tokens on BTC)
- **BSC**: Native BNB + USDC + USDT (BEP-20)
- **SOL**: Native SOL + USDC + USDT (SPL tokens)

Unsupported tokens (e.g., DAI on SOL) are ignored — no points credited.

---

## Points Calculation

### Tier Table (Flat — Not Marginal)

Points are awarded per transaction based on its **total USD value**. The entire transaction amount uses a single tier multiplier. Tiers are stored in a JSON config file (`tiers.json`) and are editable from the dashboard — changes apply to new transactions only.

| Tier | USD Range | Multiplier (pts per $0.01) | Example |
|------|-----------|---------------------------|---------|
| 0 | < $1.00 | 0 (ignored) | $0.50 → 0 pts |
| 1 | $1.00 - $11.99 | 1.0 | $5.00 → 500 pts |
| 2 | $12.00 - $29.99 | 1.1 | $20.00 → 2,200 pts |
| 3 | $30.00 - $59.99 | 1.2 | $50.00 → 6,000 pts |
| 4 | $60.00 - $119.99 | 1.3 | $100.00 → 13,000 pts |
| 5 | $120.00 - $239.99 | 1.4 | $200.00 → 28,000 pts |
| 6 | $240.00 - $599.99 | 1.5 | $500.00 → 75,000 pts |
| 7 | $600.00 - $1,199.99 | 2.0 | $1,000.00 → 200,000 pts |
| 8 | $1,200.00+ | 3.0 | $2,000.00 → 600,000 pts |

### Formula
```
cents = floor(usd_value * 100)
points = round(cents * tier_multiplier)
```

Points are always **rounded integers**.

---

## Transaction States

```
PENDING ──────────────────> CONFIRMED
(tx in mempool/             (enough confirmations)
 unconfirmed)               points become real
 pending_points              unclaimed points
 (informational)
```

### Confirmation Thresholds (Standard per Chain)
| Chain | Confirmations Required | Approximate Time |
|-------|----------------------|------------------|
| BTC | 1 confirmation | ~10 minutes |
| BSC | 12 confirmations | ~36 seconds |
| SOL | Finalized commitment | ~13 seconds |

---

## Security Model

### API Access
- **Localhost / same network**: Always allowed
- **Internet**: IP allowlist only (no API keys or tokens)
- **IP allowlist**: Managed from the dashboard (admin-authenticated endpoint), hot-reloadable without restart

### Dashboard Access
- **Username + password** (stored in `.env`)
- **Session cookie** with 1-hour timeout
- **Accessible over internet** (HTTP — put behind reverse proxy for TLS in production)

### Data Safety
- No private keys anywhere in Poller
- No mnemonic, no wallet functionality
- Only reads blockchain data, never writes transactions
- tx_hash uniqueness prevents double-counting

---

## Architecture Overview

### Fully Standalone
- Zero runtime dependency on HDPay
- Own SQLite database for watches, points, transactions
- Own CoinGecko price fetching
- Own blockchain API providers (same free-tier APIs as HDPay)
- Single network mode configured at startup (mainnet or testnet)

### Go Sub-Module
- Module path: `github.com/<user>/hdpay/poller`
- Reuses HDPay's code patterns and constants where applicable
- Never modifies HDPay code — dependency flows one way: HDPay → Poller (copy/adapt)

---

## API Summary

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/watch` | IP | Start watching an address |
| `DELETE` | `/api/watch/:id` | IP | Cancel a watch |
| `GET` | `/api/watches` | IP | List watches (filterable) |
| `GET` | `/api/points` | IP | Accounts with unclaimed confirmed points |
| `GET` | `/api/points/pending` | IP | Accounts with pending (unconfirmed) points |
| `POST` | `/api/points/claim` | IP | Claim points for specific addresses (reset to 0) |
| `GET` | `/api/health` | None | Health check |
| `POST` | `/api/admin/login` | - | Dashboard login (username + password) |
| `POST` | `/api/admin/logout` | Session | Dashboard logout |
| `GET` | `/api/admin/allowlist` | Session | View IP allowlist |
| `POST` | `/api/admin/allowlist` | Session | Add/remove IPs |
| `GET` | `/api/admin/settings` | Session | View settings (tiers, watch defaults, system info) |
| `PUT` | `/api/admin/tiers` | Session | Update tier configuration |
| `PUT` | `/api/admin/watch-defaults` | Session | Update watch defaults |
| `GET` | `/api/dashboard/stats` | Session | Stats (with time range: today/week/month/quarter) |
| `GET` | `/api/dashboard/transactions` | Session | Transaction history (filterable, paginated) |
| `GET` | `/api/dashboard/charts` | Session | Chart data (USD, points, txs over time) |
| `GET` | `/api/dashboard/errors` | Session | System errors and discrepancies |

---

## Dashboard

Multi-page monitoring dashboard (matches HDPay's style). Manual refresh. Secured with username/password login (1h session).

### Pages

1. **Overview** — Stats cards with time range selector (today / week / month / quarter / all-time):
   - Active watches, total watches, USD received, points awarded
   - Pending points (unclaimed), unique funded addresses
   - Average tx size, largest transaction
   - Charts: USD over time, points over time, tx count, chain/token breakdown, tier distribution

2. **Transactions** — Full transaction history table:
   - All columns: timestamp, address, chain, token, crypto amount, USD value, tier, points, tx hash (explorer link), watch ID, status
   - Filterable by: chain, token, date range, tier, status, min/max USD

3. **Watches** — Active and historical watches:
   - Address, chain, time started, time remaining, poll count, last poll result

4. **Pending Points** — Accounts with unclaimed points:
   - Address, chain, unclaimed points, transaction count, last tx timestamp, all-time total

5. **Errors** — System health and discrepancy detection:
   - Discrepancies between USD received and points awarded
   - Failed provider calls, watch failures
   - Any anomalies the system can detect

6. **Settings** — Configuration management:
   - Points tier table editor (read/write `tiers.json`)
   - IP allowlist management (add/remove)
   - View START_DATE
   - System info (uptime, version, network mode)
   - Watch defaults (default timeout, max active watches)
