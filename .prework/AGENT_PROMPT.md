# HDPay — Agent Build Prompt

You are building **HDPay**, a self-hosted cryptocurrency payment management tool. Read the `CLAUDE.md` file at the project root FIRST — it contains all conventions, constants, file structure, and rules you must follow. Below is the complete specification.

---

## What You're Building

A single Go binary + SvelteKit dashboard that:

1. **Generates** 500,000 HD wallet addresses per chain (BTC, BSC, SOL) from a 24-word BIP-44 mnemonic
2. **Scans** balances across all chains using free-tier blockchain APIs (round-robin, rate-limited)
3. **Tracks** full transaction history and balance state in a local SQLite database
4. **Consolidates** funds: sweep all funded addresses into a single destination via batch transactions
5. **Pre-seeds gas**: distribute small BNB amounts to BSC addresses that need gas for token transfers
6. All controlled from a **Svelte/TypeScript dashboard** running on localhost

---

## Phase 1: Project Setup & Address Generation

### 1.1 Project Scaffolding
- Initialize Go module: `github.com/[user]/hdpay`
- Create the full directory structure per `CLAUDE.md`
- Set up `internal/config/constants.go` and `internal/config/errors.go` with ALL initial constants
- Set up SQLite with `modernc.org/sqlite` (pure Go, no CGO)
- Set up `log/slog` with dual output: stdout + daily rotating log files in `./logs/`
- Set up Chi router with localhost-only binding, CORS middleware, health endpoint
- Set up SvelteKit project in `web/` with adapter-static, TypeScript strict mode, Tailwind + shadcn-svelte
- Create `web/src/lib/constants.ts` and `web/src/lib/types.ts` with all initial values
- Set up `.env.example` with all config vars

### 1.2 HD Wallet — Key Derivation

Implement `internal/wallet/` package:

**BTC** — Native SegWit (bech32, `bc1...` addresses):
- Derivation path: `m/44'/0'/0'/0/N` for mainnet, `m/44'/1'/0'/0/N` for testnet
- Libraries: `github.com/tyler-smith/go-bip39` for mnemonic→seed, `github.com/btcsuite/btcd/btcutil/hdkeychain` for BIP-32 derivation
- Generate bech32 address: derive child key → public key → witness program → `btcutil.NewAddressWitnessPubKeyHash`
- Test vectors: verify against known mnemonic→address mappings

**BSC** — EVM addresses (same as Ethereum):
- Derivation path: `m/44'/60'/0'/0/N` (coin type 60, same as ETH)
- Libraries: `github.com/btcsuite/btcd/btcutil/hdkeychain` for derivation, `github.com/ethereum/go-ethereum/crypto` for secp256k1→address
- Generate: derive child key → private key bytes → `crypto.ToECDSA` → `crypto.PubkeyToAddress`
- Checksum format (EIP-55)

**SOL** — Ed25519 addresses:
- Derivation path: `m/44'/501'/N'/0'` (standard Phantom/Solflare compatible)
- Solana uses SLIP-10 ed25519 derivation (NOT standard BIP-32 secp256k1)
- Libraries: `github.com/tyler-smith/go-bip39` for seed, then SLIP-10 ed25519 child key derivation, `github.com/gagliardetto/solana-go` for PublicKey
- OR use `github.com/dmitrymomot/solana/common` which has `DeriveAccountFromMnemonicBip44` / `DeriveAccountsListFromMnemonicBip44`
- Base58-encoded public key is the address

### 1.3 Address Storage

SQLite schema:
```sql
CREATE TABLE addresses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chain TEXT NOT NULL,          -- 'BTC', 'BSC', 'SOL'
    address_index INTEGER NOT NULL,
    address TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chain, address_index)
);
CREATE INDEX idx_addresses_chain ON addresses(chain);
CREATE INDEX idx_addresses_address ON addresses(address);

CREATE TABLE balances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chain TEXT NOT NULL,
    address_index INTEGER NOT NULL,
    token TEXT NOT NULL,          -- 'NATIVE', 'USDC', 'USDT'
    balance TEXT NOT NULL DEFAULT '0',  -- Store as string to handle big numbers
    last_scanned TIMESTAMP,
    UNIQUE(chain, address_index, token),
    FOREIGN KEY (chain, address_index) REFERENCES addresses(chain, address_index)
);
CREATE INDEX idx_balances_nonzero ON balances(chain, token) WHERE balance != '0';

CREATE TABLE transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chain TEXT NOT NULL,
    address_index INTEGER NOT NULL,
    tx_hash TEXT NOT NULL,
    direction TEXT NOT NULL,      -- 'in', 'out'
    token TEXT NOT NULL,
    amount TEXT NOT NULL,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    block_number INTEGER,
    block_timestamp TIMESTAMP,
    gas_used TEXT,
    gas_price TEXT,
    status TEXT NOT NULL,         -- 'pending', 'confirmed', 'failed'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chain, tx_hash, address_index)
);
CREATE INDEX idx_tx_chain ON transactions(chain);
CREATE INDEX idx_tx_address ON transactions(chain, address_index);

CREATE TABLE scan_state (
    chain TEXT PRIMARY KEY,
    last_scanned_index INTEGER NOT NULL DEFAULT 0,
    max_scan_id INTEGER NOT NULL DEFAULT 5000,
    status TEXT NOT NULL DEFAULT 'idle',   -- 'idle', 'scanning', 'paused'
    started_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 1.4 Init Command

CLI command: `./hdpay init --mnemonic-file /path/to/mnemonic.txt`
- Read 24-word mnemonic from file
- Validate mnemonic (BIP-39)
- Generate 500K addresses per chain (3 chains = 1.5M addresses)
- Batch INSERT into SQLite (use transactions, batch of 10,000)
- Log progress every 10,000 addresses
- Expected runtime: ~2-5 minutes
- Export JSON files per chain to `./data/export/`

### 1.5 API & Frontend — Address Explorer

Backend:
- `GET /api/addresses/:chain?page=1&pageSize=100` — paginated list
- `GET /api/addresses/:chain/export` — JSON export download

Frontend:
- Address table with virtual scrolling (tanstack virtual)
- Filter by chain
- Pagination controls
- Export button (downloads JSON)

---

## Phase 2: Balance Scanning

### 2.1 Provider Architecture

Implement `internal/scanner/` package:

**Provider Interface:**
```go
type BalanceProvider interface {
    Name() string
    Chain() string
    // GetBalances returns balances for a batch of addresses
    // Returns map[address]map[token]balance
    GetBalances(ctx context.Context, addresses []string) (map[string]map[string]string, error)
    // GetTransactions returns tx history for an address
    GetTransactions(ctx context.Context, address string) ([]Transaction, error)
    // MaxBatchSize returns how many addresses can be queried at once
    MaxBatchSize() int
}
```

**BTC Providers:**
1. `BlockstreamProvider` — `GET https://blockstream.info/api/address/{addr}` → parse `chain_stats.funded_txo_sum - chain_stats.spent_txo_sum` for balance. 1 address/call, ~10 req/s.
2. `MempoolProvider` — `GET https://mempool.space/api/address/{addr}` → same format. 1 address/call, ~10 req/s.
3. `BlockchainInfoProvider` — `GET https://blockchain.info/multiaddr?active=addr1|addr2|...` → batch multiple addresses. Up to 50/call, ~5 req/s.

Testnet: `blockstream.info/testnet/api`, `mempool.space/testnet/api`

**BSC Providers:**
1. `BscScanProvider` — `GET https://api.bscscan.com/api?module=account&action=balancemulti&address=addr1,addr2,...` → batch up to 20 addresses for native BNB. For BEP-20 tokens, use `action=tokenbalance`. 5 req/s with API key.
2. `BscRPCProvider` — Direct RPC calls via `github.com/ethereum/go-ethereum/ethclient`. `BalanceAt` for native, ERC-20 `balanceOf` call for tokens. ~10 req/s on public nodes.

Testnet: `data-seed-prebsc-1-s1.binance.org:8545`, BscScan testnet API

**SOL Providers:**
1. `SolanaRPCProvider` — `getMultipleAccounts` (up to 100 pubkeys/call) for native SOL balance (lamports field). For SPL tokens, derive Associated Token Account (ATA) addresses and query those. ~10 req/s.
2. `HeliusProvider` — Same RPC interface, different endpoint. ~10 req/s with free API key.

Devnet: `api.devnet.solana.com`

### 2.2 Round-Robin + Rate Limiting

```go
type ProviderPool struct {
    providers []BalanceProvider
    current   int
    limiters  map[string]*rate.Limiter // per-provider
}

func (p *ProviderPool) Next(ctx context.Context) BalanceProvider {
    // Round-robin with rate limit awareness
    // If current provider is rate-limited, skip to next
    // If all rate-limited, wait for the first one to become available
}
```

Use `golang.org/x/time/rate` for token bucket rate limiting per provider.

### 2.3 Scanner Orchestrator

```go
func (s *Scanner) Scan(ctx context.Context, chain string, maxID int) error {
    // 1. Check scan_state for resume point
    // 2. Load addresses from DB (resume index → maxID)
    // 3. Batch addresses per provider's MaxBatchSize
    // 4. For each batch:
    //    a. Get provider from pool (round-robin)
    //    b. Wait for rate limit
    //    c. Fetch balances
    //    d. Upsert into balances table
    //    e. Update scan_state.last_scanned_index
    //    f. Send SSE progress event
    //    g. Log progress
    // 5. Fetch transactions for addresses with non-zero balances
    // 6. Mark scan complete
}
```

Resume logic:
- Check `scan_state.updated_at` — if less than `ScanResumeThreshold` (24h), resume from `last_scanned_index`
- Otherwise, reset to 0

### 2.4 API & Frontend — Scan Control

Backend:
- `POST /api/scan/start` `{ "chain": "BTC", "maxID": 5000 }` — start scan (runs in goroutine)
- `POST /api/scan/stop` — cancel current scan via context cancellation
- `GET /api/scan/status` — current scan state for all chains
- `GET /api/scan/sse` — Server-Sent Events stream for real-time progress

Frontend:
- Scan control panel per chain
- Max ID input
- Start/Stop buttons
- Real-time progress bar (via SSE)
- Scan history/status display
- Results summary (addresses found with balance, total value)

---

## Phase 3: Dashboard & Portfolio

### 3.1 API

- `GET /api/balances/summary` — aggregated balances per chain per token + USD values
- `GET /api/balances/:chain?hasBalance=true&token=USDC&page=1` — filterable address list
- `GET /api/dashboard/portfolio` — total portfolio value in USD
- `GET /api/dashboard/prices` — current coin/token prices from CoinGecko

**CoinGecko integration:**
```
GET https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,binancecoin,solana,usd-coin,tether&vs_currencies=usd
```
Cache for 5 minutes.

### 3.2 Frontend Dashboard

- **Portfolio Overview**: Total USD value, per-chain breakdown, per-token breakdown
- **Charts**: Pie chart (chain distribution), bar chart (token distribution) using ECharts
- **Address Explorer**: Filterable/sortable table of all addresses with balances
- **Quick Actions**: "Scan All", "Send Funds" shortcuts

---

## Phase 4: Sending Transactions

### 4.1 BTC — Multi-Input UTXO Transaction

```go
func BuildBTCSweepTx(ctx context.Context, fundedAddresses []FundedAddress, destination string, feeRate int) (*wire.MsgTx, error) {
    // 1. For each funded address, fetch UTXOs (via Blockstream/Mempool API)
    // 2. Create tx with all UTXOs as inputs
    // 3. Calculate fee: estimatedSize * feeRate
    // 4. Single output: destination address, amount = totalInput - fee
    // 5. For each input:
    //    a. Derive private key from mnemonic + index (on-demand)
    //    b. Sign input
    //    c. Immediately discard private key
    // 6. Serialize and broadcast via Blockstream/Mempool API POST
}
```

- Use `github.com/btcsuite/btcd/wire` for tx construction
- Use `github.com/btcsuite/btcd/txscript` for signing (P2WPKH witness program for bech32)
- Fetch fee estimate from `mempool.space/api/v1/fees/recommended`
- Broadcast via `POST https://blockstream.info/api/tx` (raw hex)

### 4.2 BSC — Sequential Automated Transfers

**Native BNB:**
```go
func SweepBSCNative(ctx context.Context, fundedAddresses []FundedAddress, destination string) ([]string, error) {
    // For each address:
    // 1. Derive private key from mnemonic + index
    // 2. Build tx: from → destination, amount = balance - gasEstimate
    // 3. Sign with go-ethereum types.SignTx
    // 4. Broadcast via ethclient.SendTransaction
    // 5. Wait for receipt
    // 6. Discard private key
    // 7. Log tx hash
    // Returns list of tx hashes
}
```

**BEP-20 Tokens (USDC/USDT):**
```go
func SweepBSCToken(ctx context.Context, fundedAddresses []FundedAddress, tokenContract string, destination string) ([]string, error) {
    // Same as above but:
    // 1. Build ERC-20 transfer data: transfer(destination, amount)
    // 2. Address must have BNB for gas — check first, error if not
    // 3. Gas limit = 65000 (BEP-20 transfer)
}
```

Use `github.com/ethereum/go-ethereum/ethclient` with BSC RPC endpoints.
BSC Chain ID: 56 (mainnet), 97 (testnet).

### 4.3 Gas Pre-Seeding (BSC)

```go
func PreSeedGas(ctx context.Context, sourceIndex int, targetAddresses []FundedAddress, amountWei string) ([]string, error) {
    // 1. Derive private key for sourceIndex
    // 2. Check source has enough BNB for all transfers + gas
    // 3. For each target:
    //    a. Send amountWei BNB (default: 0.005 BNB)
    //    b. Wait for confirmation
    //    c. Log
    // Returns tx hashes
}
```

The user selects a source address (one of their funded BNB addresses) in the UI, then the tool distributes small BNB amounts to all addresses that need gas for token transfers.

### 4.4 SOL — Multi-Instruction Transaction

```go
func SweepSOLNative(ctx context.Context, fundedAddresses []FundedAddress, destination string) ([]string, error) {
    // Group addresses into batches of ~20 (SOL tx size limit)
    // For each batch:
    // 1. Create transaction with multiple transfer instructions
    // 2. Each instruction: system.Transfer from addr[i] to destination
    // 3. Derive all private keys for the batch
    // 4. Sign transaction with all signers
    // 5. Broadcast via solana-go RPC client
    // 6. Discard all private keys
}
```

**SPL Tokens (USDC/USDT):**
```go
func SweepSOLToken(ctx context.Context, fundedAddresses []FundedAddress, mint string, destination string) ([]string, error) {
    // Similar to native but:
    // 1. Derive Associated Token Account (ATA) for destination
    // 2. Create ATA if it doesn't exist (include CreateAssociatedTokenAccount instruction)
    // 3. For each source: spl_token.Transfer instruction
    // 4. Source needs SOL for rent/fees — each signer adds to fee
}
```

Use `github.com/gagliardetto/solana-go` for transaction building, RPC, and SPL token program interactions.

### 4.5 API & Frontend — Send Interface

Backend:
- `POST /api/send/preview` `{ "chain": "BSC", "token": "USDC", "destination": "0x..." }` — returns list of addresses to sweep, estimated fees, total amount
- `POST /api/send/execute` `{ "chain": "BSC", "token": "USDC", "destination": "0x..." }` — execute the sweep
- `POST /api/send/gas-preseed` `{ "sourceIndex": 0, "targetIndices": [1,2,3], "amountWei": "5000000000000000" }` — pre-seed gas
- SSE events for transaction progress

Frontend:
- Chain + Token selector
- View all funded addresses for selection
- Total balance display
- Destination address input with validation
- "Send All" button
- For BSC tokens: gas pre-seed step (shows which addresses need gas, select source, execute)
- Transaction confirmation dialog (shows fees, net amount)
- Real-time progress for multi-tx sends (BSC sequential, SOL batches)
- Transaction receipt list after completion

---

## Phase 5: Transaction History & Settings

### 5.1 Transaction History

Backend:
- `GET /api/transactions?chain=BSC&page=1&pageSize=50` — paginated, filterable
- Store ALL transactions found during scanning + all outgoing sends

Frontend:
- Table with: chain, tx hash (linked to explorer), direction, token, amount, from, to, status, timestamp
- Filters: chain, direction, token, date range
- Pagination

### 5.2 Settings

Backend:
- `GET /api/settings` — current config
- `PUT /api/settings` — update (max scan IDs per chain, network mode)

Frontend:
- Max ID per chain inputs
- Network toggle: mainnet / testnet
- Log level selector
- Provider status display (which APIs are reachable)

---

## Critical Implementation Notes

### Security
- **Localhost only**: Bind to `127.0.0.1`, never `0.0.0.0`
- **CORS**: Allow only `http://localhost:*` and `http://127.0.0.1:*`
- **CSRF**: Token-based protection on all POST/PUT/DELETE
- **Mnemonic**: Read from file, use for derivation, never stored in DB, never logged
- **Private keys**: Derived on-demand for signing only, immediately zeroed after use
- **No secrets in logs**: Sanitize any log output that might contain keys

### Logging Requirements
- **Every API call** (inbound): method, path, status, duration
- **Every provider call** (outbound): provider name, endpoint, batch size, duration, status code
- **Every scan step**: chain, batch index, addresses scanned, balances found
- **Every transaction**: chain, tx hash, from, to, amount, status
- **Every error**: full context, stack trace where applicable
- **Dual output**: stdout (for terminal) + `./logs/hdpay-YYYY-MM-DD.log` (daily rotation)
- **Levels**: DEBUG for internal state, INFO for user-visible actions, WARN for recoverable issues, ERROR for failures

### Constants Rule
- **ZERO hardcoded values** — every number, string, URL, timeout, limit, error code lives in `internal/config/constants.go`, `internal/config/errors.go`, or `web/src/lib/constants.ts`
- The agent must check these files before adding any new value
- If a value appears in code as a literal, it's a bug

### Testing Rule
- Every Go package must have a corresponding `_test.go`
- Every Svelte component must have a corresponding `.test.ts`
- Test address derivation against known BIP-44 test vectors
- Test scanning with mock providers
- Test transaction building with testnet

### Commit Rule
- Commit after EVERY meaningful change
- Update `CHANGELOG.md` before every commit
- Format: `type: description` (feat/fix/refactor/test/docs/chore)

### Utility Rule
- Before creating ANY helper function: `grep -r "functionName"` the entire codebase
- Frontend utils go in `web/src/lib/utils/`
- Backend utils go in appropriate `internal/` package
- NEVER duplicate a function

---

## Go Module Dependencies

```
github.com/go-chi/chi/v5                    # HTTP router
modernc.org/sqlite                           # SQLite (pure Go)
github.com/tyler-smith/go-bip39              # BIP-39 mnemonic
github.com/btcsuite/btcd                     # BTC: HD keys, tx building, signing
github.com/btcsuite/btcd/btcutil             # BTC: address utils
github.com/btcsuite/btcd/btcutil/hdkeychain  # BIP-32 HD key derivation
github.com/ethereum/go-ethereum              # BSC/EVM: ethclient, crypto, types
github.com/gagliardetto/solana-go            # SOL: RPC, tx building, SPL tokens
github.com/dmitrymomot/solana                # SOL: BIP-44 mnemonic derivation helpers
github.com/kelseyhightower/envconfig         # Environment config
github.com/nanmu42/etherscan-api             # BscScan API client (works for BSC)
golang.org/x/time/rate                       # Rate limiting
```

## Frontend Dependencies

```
@sveltejs/adapter-static
tailwindcss
shadcn-svelte
echarts
@tanstack/svelte-virtual
vitest
@testing-library/svelte
```
