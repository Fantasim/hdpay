# HDPay — Technical Reference & Deep Dive

This document supplements `CLAUDE.md` (conventions) and `AGENT_PROMPT.md` (build spec) with the detailed context, edge cases, decision rationale, data flows, and implementation specifics needed to plan and execute work without ambiguity.

---

## Table of Contents

1. [System Architecture](#1-system-architecture)
2. [HD Wallet Deep Dive](#2-hd-wallet-deep-dive)
3. [Address Generation Pipeline](#3-address-generation-pipeline)
4. [Scanning Engine Deep Dive](#4-scanning-engine-deep-dive)
5. [Provider Implementation Details](#5-provider-implementation-details)
6. [Database Design & Queries](#6-database-design--queries)
7. [Transaction Engine Deep Dive](#7-transaction-engine-deep-dive)
8. [Gas Pre-Seeding Workflow](#8-gas-pre-seeding-workflow)
9. [SSE (Server-Sent Events) Architecture](#9-sse-server-sent-events-architecture)
10. [Frontend Architecture](#10-frontend-architecture)
11. [Security Model](#11-security-model)
12. [Error Handling Strategy](#12-error-handling-strategy)
13. [Logging Architecture](#13-logging-architecture)
14. [Testing Strategy](#14-testing-strategy)
15. [Build & Deployment](#15-build--deployment)
16. [Edge Cases & Failure Modes](#16-edge-cases--failure-modes)
17. [Data Flow Diagrams](#17-data-flow-diagrams)
18. [Task Dependency Graph](#18-task-dependency-graph)

---

## 1. System Architecture

### High-Level Data Flow

```
User Machine (localhost only)
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  ┌──────────────┐    HTTP/SSE     ┌──────────────────────────┐  │
│  │  SvelteKit   │◄──────────────►│  Go Backend (Chi)         │  │
│  │  Dashboard    │   localhost     │                          │  │
│  │  :5173 (dev)  │   :8080        │  ┌────────────────────┐  │  │
│  │  embedded     │                │  │  Scanner Engine     │  │  │
│  │  (prod)       │                │  │  (goroutines)       │  │  │
│  └──────────────┘                │  └─────────┬──────────┘  │  │
│                                  │            │              │  │
│                                  │  ┌─────────▼──────────┐  │  │
│                                  │  │  SQLite DB          │  │  │
│                                  │  │  ./data/hdpay.sqlite│  │  │
│                                  │  └────────────────────┘  │  │
│                                  │            │              │  │
│                                  │  ┌─────────▼──────────┐  │  │
│                                  │  │  Provider Pool      │  │  │
│                                  │  │  (rate-limited)     │  │  │
│                                  │  └─────────┬──────────┘  │  │
│                                  └────────────┼──────────────┘  │
│                                               │                  │
└───────────────────────────────────────────────┼──────────────────┘
                                                │ HTTPS (outbound only)
                    ┌───────────────────────────┼───────────────────────┐
                    │                           │                       │
            ┌───────▼───────┐  ┌────────────────▼──┐  ┌───────────────▼──┐
            │ BTC APIs      │  │ BSC APIs           │  │ SOL APIs         │
            │ - Blockstream │  │ - BscScan          │  │ - Solana RPC     │
            │ - Mempool     │  │ - Public RPCs      │  │ - Helius         │
            │ - Blockchain  │  │ - Ankr             │  │                  │
            └───────────────┘  └────────────────────┘  └──────────────────┘
```

### Process Lifecycle

```
1. INIT (one-time)
   ./hdpay init --mnemonic-file /path/to/secret.txt
   → Reads mnemonic → Generates 1.5M addresses → Stores in SQLite → Exports JSON

2. RUN (normal operation)
   ./hdpay serve
   → Starts HTTP server on 127.0.0.1:8080
   → Serves embedded SvelteKit static files
   → User opens http://localhost:8080

3. SCAN (user-triggered)
   Dashboard → Click "Scan BTC up to 5000"
   → Backend spawns goroutine
   → Fetches balances via provider pool
   → Updates DB in real-time
   → Sends SSE progress events to frontend

4. SEND (user-triggered)
   Dashboard → Select chain/token → Click "Send All"
   → Backend builds transaction(s)
   → Derives private keys on-demand
   → Signs and broadcasts
   → Tracks confirmations
   → Updates DB
```

### Go Binary Structure

The binary has two modes controlled by subcommands:

```go
func main() {
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(1)
    }

    switch os.Args[1] {
    case "init":
        runInit()      // Address generation
    case "serve":
        runServe()     // HTTP server
    case "export":
        runExport()    // Re-export addresses to JSON
    case "version":
        printVersion()
    default:
        printUsage()
        os.Exit(1)
    }
}
```

---

## 2. HD Wallet Deep Dive

### BIP-44 Path Structure

```
m / purpose' / coin_type' / account' / change / address_index

For HDPay:
m / 44' / coin_type' / 0' / 0 / N

Where N = 0 to 499,999
```

- `purpose` = 44 (BIP-44 standard)
- `account` = 0 (we use only one account)
- `change` = 0 (external chain, receiving addresses)
- `address_index` = 0 to 499,999

### Per-Chain Derivation

**BTC (secp256k1 → bech32)**:
```
Path:      m/44'/0'/0'/0/N    (mainnet)
           m/44'/1'/0'/0/N    (testnet)
Curve:     secp256k1
Address:   Native SegWit (bech32) — bc1q... / tb1q...
Why bech32: Lowest transaction fees, most widely supported modern format
Library:   btcsuite/btcd/btcutil/hdkeychain → Child(N) → ECPubKey() → Hash160 → btcutil.NewAddressWitnessPubKeyHash
```

Derivation steps:
1. Mnemonic → Seed (BIP-39, with empty passphrase)
2. Seed → Master key (`hdkeychain.NewMaster(seed, netParams)`)
3. Master → `m/44'` → `m/44'/0'` → `m/44'/0'/0'` → `m/44'/0'/0'/0` → `m/44'/0'/0'/0/N`
4. Child key → `ECPubKey()` → `btcutil.Hash160(pubkey.SerializeCompressed())` → `btcutil.NewAddressWitnessPubKeyHash(hash, netParams)`
5. Address = bech32 encoded string

**BSC / EVM (secp256k1 → hex address)**:
```
Path:      m/44'/60'/0'/0/N   (same path for BSC and ETH)
Curve:     secp256k1
Address:   0x-prefixed EIP-55 checksummed hex
Why 60:    BSC is EVM-compatible, uses same derivation as ETH. All wallets do this.
Library:   hdkeychain → Child(N) → ECPrivKey() → go-ethereum/crypto.ToECDSA → crypto.PubkeyToAddress
```

Derivation steps:
1. Same master key derivation as BTC
2. Path: `m/44'/60'/0'/0/N` (coin type 60)
3. Child key → `ECPrivKey()` → raw 32 bytes → `crypto.ToECDSA(bytes)`
4. `crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey))` → `common.Address`
5. Address = EIP-55 checksummed hex string

**SOL (ed25519 via SLIP-10)**:
```
Path:      m/44'/501'/N'/0'   (Phantom/Solflare standard)
Curve:     ed25519 (NOT secp256k1)
Address:   Base58-encoded public key
Why this:  Standard path used by Phantom, Solflare, and most SOL wallets
CRITICAL:  Solana does NOT use standard BIP-32. It uses SLIP-10 for ed25519.
```

**SLIP-10 vs BIP-32**: Standard BIP-32 only works with secp256k1. Solana uses ed25519, which requires SLIP-10 (a different child key derivation algorithm). The key difference is that SLIP-10 ed25519 only supports hardened derivation (all path elements must have `'`).

Derivation steps:
1. Mnemonic → Seed (BIP-39, same as BTC)
2. Seed → SLIP-10 Master key: `HMAC-SHA512(key="ed25519 seed", data=seed)` → left 32 bytes = key, right 32 bytes = chain code
3. For each hardened path element (`44'`, `501'`, `N'`, `0'`):
   - `HMAC-SHA512(key=chainCode, data=0x00 || parentKey || index_with_hardened_bit)`
   - Left 32 = child key, right 32 = child chain code
4. Final 32-byte secret → ed25519 keypair: `ed25519.NewKeyFromSeed(secret)`
5. Public key (32 bytes) → Base58 encode = Solana address

**Option A**: Implement SLIP-10 manually (straightforward, ~50 lines of code).
**Option B**: Use `github.com/dmitrymomot/solana/common.DeriveAccountsListFromMnemonicBip44(mnemonic, count)` which wraps this.

**Recommendation**: Start with Option B for speed, but verify output against Phantom wallet addresses. If the library has issues, implement SLIP-10 manually — it's not complex.

### Mnemonic Security Lifecycle

```
INIT:
  1. Read mnemonic from file (HDPAY_MNEMONIC_FILE env var)
  2. Validate: must be 24 words, valid BIP-39
  3. Derive master key
  4. Generate all addresses (public keys only — no private keys stored)
  5. Mnemonic goes out of scope, GC'd

SERVE (normal operation):
  - No mnemonic in memory
  - No private keys in memory
  - Only public addresses in DB

SIGNING (on-demand):
  1. Read mnemonic from file
  2. Derive ONLY the specific private key(s) needed
  3. Sign transaction
  4. Zero out private key bytes: copy(privKeyBytes, make([]byte, 32))
  5. Mnemonic goes out of scope, GC'd
  6. Total time in memory: milliseconds
```

---

## 3. Address Generation Pipeline

### Performance Considerations

500K addresses × 3 chains = 1.5M addresses. Each derivation involves:
- BIP-32 child key derivation (5 levels deep)
- Public key computation
- Address encoding

**Estimated speed**: ~5,000-10,000 addresses/second per chain on modern hardware.
**Estimated total time**: ~2-5 minutes for all 1.5M addresses.

### Batched Database Inserts

Do NOT insert one row at a time. Use batched transactions:

```go
const insertBatchSize = 10000

func (db *DB) InsertAddresses(chain string, addresses []Address) error {
    for i := 0; i < len(addresses); i += insertBatchSize {
        end := min(i+insertBatchSize, len(addresses))
        batch := addresses[i:end]

        tx, err := db.conn.Begin()
        if err != nil {
            return fmt.Errorf("begin tx: %w", err)
        }

        stmt, err := tx.Prepare("INSERT INTO addresses (chain, address_index, address) VALUES (?, ?, ?)")
        if err != nil {
            tx.Rollback()
            return fmt.Errorf("prepare: %w", err)
        }

        for _, addr := range batch {
            _, err = stmt.Exec(chain, addr.Index, addr.Address)
            if err != nil {
                tx.Rollback()
                return fmt.Errorf("exec: %w", err)
            }
        }

        stmt.Close()
        if err := tx.Commit(); err != nil {
            return fmt.Errorf("commit: %w", err)
        }

        slog.Info("addresses inserted",
            "chain", chain,
            "batch", i/insertBatchSize+1,
            "count", len(batch),
            "total", len(addresses),
        )
    }
    return nil
}
```

### JSON Export Format

```json
{
  "chain": "BTC",
  "network": "mainnet",
  "derivation_path_template": "m/44'/0'/0'/0/{index}",
  "generated_at": "2026-02-18T12:00:00Z",
  "count": 500000,
  "addresses": [
    { "index": 0, "address": "bc1q..." },
    { "index": 1, "address": "bc1q..." },
    ...
  ]
}
```

Export path: `./data/export/btc_addresses.json`, `./data/export/bsc_addresses.json`, `./data/export/sol_addresses.json`

### Init Idempotency

Running `init` twice should be safe:
- Check if addresses already exist for a chain
- If count matches expected (500K), skip that chain
- If count differs (partial init), wipe and regenerate that chain
- Log clearly what's happening

---

## 4. Scanning Engine Deep Dive

### Scanner State Machine

```
                  ┌─────────┐
                  │  IDLE   │ ◄────── Stop / Complete
                  └────┬────┘
                       │ Start
                  ┌────▼────┐
         ┌────── │ SCANNING │ ──────┐
         │       └────┬────┘        │
         │            │              │
    Error │      Progress        Cancel
    (retry)│          │              │
         │       ┌────▼────┐   ┌────▼────┐
         └────── │ SCANNING │   │ PAUSED  │
                 └──────────┘   └─────────┘
```

Only ONE scan per chain can run at a time. Multiple chains can scan concurrently.

### Scan Flow — Detailed

```
StartScan(chain="BTC", maxID=5000):

1. LOCK: Acquire per-chain mutex
2. CHECK: Is this chain already scanning? → reject with error
3. RESUME CHECK:
   - Query scan_state for this chain
   - If status == "scanning" AND updated_at < 1 hour ago → stale, reset to idle
   - If status == "idle" AND updated_at < ScanResumeThreshold (24h) → resume from last_scanned_index
   - If status == "idle" AND updated_at >= ScanResumeThreshold → start from 0
   - If status == "paused" → resume from last_scanned_index
4. UPDATE scan_state: status="scanning", started_at=now
5. LOAD addresses from DB: chain=chain, address_index BETWEEN startIndex AND maxID
6. CHUNK addresses into batches based on provider MaxBatchSize
7. FOR each batch:
   a. SELECT provider from pool (round-robin)
   b. WAIT on provider's rate limiter
   c. CALL provider.GetBalances(ctx, batchAddresses)
   d. ON SUCCESS:
      - UPSERT balances into DB
      - UPDATE scan_state.last_scanned_index
      - EMIT SSE: scan_progress
      - LOG: info level
   e. ON RATE LIMIT (429):
      - LOG: warn
      - ROTATE to next provider
      - RETRY this batch (max 3 retries across providers)
   f. ON ERROR (network, timeout):
      - LOG: error
      - ROTATE to next provider
      - RETRY this batch
   g. ON ALL PROVIDERS EXHAUSTED:
      - PAUSE scan
      - EMIT SSE: scan_error
      - WAIT 60 seconds
      - RETRY from paused position
   h. CHECK ctx.Done() → if cancelled, save state and exit cleanly
8. AFTER all batches:
   - For addresses with non-zero balances, fetch transaction history
   - UPDATE scan_state: status="idle", updated_at=now
   - EMIT SSE: scan_complete
   - LOG: info with summary
9. UNLOCK per-chain mutex
```

### Token Scanning — Extra Complexity

**BTC**: Only native balance. Simple.

**BSC**: For each address, need to check:
- Native BNB balance: `eth_getBalance` or BscScan `balancemulti`
- USDC balance: ERC-20 `balanceOf` call against USDC contract
- USDT balance: ERC-20 `balanceOf` call against USDT contract

This triples the number of API calls for BSC. Strategy:
1. First pass: check native BNB balance only (fast, batch 20 at BscScan)
2. Second pass: for ALL addresses (not just those with BNB), check USDC/USDT
   - Someone could have USDC but 0 BNB
   - Use BscScan `tokenbalance` endpoint OR direct RPC `eth_call`

**SOL**: For each address, need to check:
- Native SOL: `getMultipleAccounts` → lamports field (batch 100)
- USDC: Derive Associated Token Account (ATA) for each address+USDC mint → `getMultipleAccounts` on ATAs
- USDT: Same as USDC but with USDT mint

ATA derivation is deterministic (no API call needed):
```
ATA = findProgramAddress(
    seeds=[walletAddress, TOKEN_PROGRAM_ID, mintAddress],
    programId=ASSOCIATED_TOKEN_PROGRAM_ID
)
```

Strategy for SOL:
1. Batch 100 wallet addresses → `getMultipleAccounts` → native SOL balances
2. Derive 100 USDC ATAs → `getMultipleAccounts` → USDC balances
3. Derive 100 USDT ATAs → `getMultipleAccounts` → USDT balances
4. Total: 3 RPC calls per 100 addresses

### Scan Time Estimates

| Chain | Max ID | Calls Needed | Rate (optimistic) | Estimated Time |
|-------|--------|-------------|-------------------|----------------|
| BTC | 5,000 | ~100 (batch 50 via blockchain.info) + 5000 singles across 2 providers | ~15 req/s combined | ~6 minutes |
| BSC | 5,000 | ~250 BNB (batch 20) + ~5000 USDC + ~5000 USDT | ~5 req/s BscScan | ~35 minutes |
| SOL | 5,000 | ~50 native (batch 100) + ~50 USDC + ~50 USDT | ~10 req/s | ~15 seconds |
| BTC | 50,000 | ~50,000 | ~15 req/s | ~55 minutes |
| BSC | 50,000 | ~52,500 | ~5 req/s | ~3 hours |
| SOL | 50,000 | ~1,500 | ~10 req/s | ~2.5 minutes |

SOL is by far the fastest due to `getMultipleAccounts` batching 100 at once.
BSC is the slowest due to per-token queries and low BscScan rate limit.

**Optimization**: For BSC, alternate between BscScan API (batch 20 for native) and direct RPC calls (for token balances) to use both rate limit pools simultaneously.

---

## 5. Provider Implementation Details

### BTC — Blockstream Esplora

```
GET https://blockstream.info/api/address/{address}

Response:
{
  "address": "bc1q...",
  "chain_stats": {
    "funded_txo_count": 2,
    "funded_txo_sum": 150000,     // satoshis received
    "spent_txo_count": 1,
    "spent_txo_sum": 50000,       // satoshis spent
    "tx_count": 3
  },
  "mempool_stats": {
    "funded_txo_count": 0,
    "funded_txo_sum": 0,
    "spent_txo_count": 0,
    "spent_txo_sum": 0,
    "tx_count": 0
  }
}

Balance = chain_stats.funded_txo_sum - chain_stats.spent_txo_sum + mempool (if including unconfirmed)
In this case: 150000 - 50000 = 100000 satoshis = 0.001 BTC
```

For transaction history:
```
GET https://blockstream.info/api/address/{address}/txs

Returns array of full transaction objects. Paginate with ?after_txid={last_txid}
```

### BTC — Blockchain.info (Batch)

```
GET https://blockchain.info/multiaddr?active=addr1|addr2|addr3...&n=0

Response:
{
  "addresses": [
    {
      "address": "bc1q...",
      "final_balance": 100000,
      "n_tx": 3,
      "total_received": 150000,
      "total_sent": 50000
    },
    ...
  ]
}
```

`n=0` means don't include transactions (faster). Up to ~50-100 addresses per call.

### BSC — BscScan Multi-Balance

```
GET https://api.bscscan.com/api?module=account&action=balancemulti&address=0x...,0x...&tag=latest&apikey=KEY

Response:
{
  "status": "1",
  "result": [
    { "account": "0x...", "balance": "1000000000000000000" },  // wei
    ...
  ]
}
```

Up to 20 addresses per call. Balance in wei (divide by 10^18 for BNB).

For BEP-20 token balance (single address):
```
GET https://api.bscscan.com/api?module=account&action=tokenbalance&contractaddress=USDC_CONTRACT&address=0x...&tag=latest&apikey=KEY
```

### BSC — Direct RPC (go-ethereum ethclient)

```go
client, _ := ethclient.Dial("https://bsc-dataseed.binance.org")

// Native BNB balance
balance, _ := client.BalanceAt(ctx, common.HexToAddress(addr), nil) // *big.Int in wei

// BEP-20 token balance (requires ABI call)
// Encode: balanceOf(address) → 0x70a08231 + padded address
data := crypto.Keccak256([]byte("balanceOf(address)"))[:4]
data = append(data, common.LeftPadBytes(common.HexToAddress(addr).Bytes(), 32)...)
result, _ := client.CallContract(ctx, ethereum.CallMsg{
    To:   &tokenContract,
    Data: data,
}, nil)
// result is 32-byte big-endian uint256
```

### SOL — getMultipleAccounts

```json
POST https://api.mainnet-beta.solana.com
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "getMultipleAccounts",
  "params": [
    ["pubkey1", "pubkey2", ..., "pubkey100"],
    { "encoding": "base64", "commitment": "confirmed" }
  ]
}

Response:
{
  "result": {
    "value": [
      {
        "lamports": 1000000000,  // 1 SOL = 1,000,000,000 lamports
        "owner": "11111111111111111111111111111111",
        "executable": false,
        "data": ["", "base64"]
      },
      null,  // account doesn't exist
      ...
    ]
  }
}
```

For SPL tokens, query the Associated Token Account:
```go
ata, _, _ := solana.FindAssociatedTokenAddress(walletPubkey, mintPubkey)
// Then getMultipleAccounts on the ATA addresses
// Parse the SPL Token Account data to extract balance
// SPL Token Account layout: first 64 bytes = mint(32) + owner(32), then 8 bytes = amount (u64 little-endian)
```

### SOL — Transaction History

Solana doesn't have a free batch transaction history API like Etherscan. Options:
1. `getSignaturesForAddress` — returns recent transaction signatures for an address (paginated, up to 1000)
2. `getTransaction` — get full transaction details by signature

Strategy: Only fetch tx history for addresses with non-zero balances (these are the ones we care about).

---

## 6. Database Design & Queries

### Common Query Patterns

**Portfolio summary:**
```sql
SELECT
    b.chain,
    b.token,
    SUM(CAST(b.balance AS REAL)) as total_balance,
    COUNT(*) as address_count
FROM balances b
WHERE b.balance != '0'
GROUP BY b.chain, b.token
ORDER BY b.chain, b.token;
```

**Addresses with balance for a chain/token:**
```sql
SELECT
    a.address_index,
    a.address,
    b.balance,
    b.last_scanned
FROM addresses a
JOIN balances b ON a.chain = b.chain AND a.address_index = b.address_index
WHERE a.chain = ? AND b.token = ? AND b.balance != '0'
ORDER BY CAST(b.balance AS REAL) DESC
LIMIT ? OFFSET ?;
```

**Scan resume point:**
```sql
SELECT last_scanned_index, max_scan_id, status, updated_at
FROM scan_state
WHERE chain = ?;
```

**Funded addresses for sending (with all token balances):**
```sql
SELECT
    a.address_index,
    a.address,
    b.token,
    b.balance
FROM addresses a
JOIN balances b ON a.chain = b.chain AND a.address_index = b.address_index
WHERE a.chain = ? AND b.token = ? AND b.balance != '0'
ORDER BY a.address_index;
```

### Balance Storage

Balances are stored as strings to avoid floating-point precision issues with big numbers.

- BTC: stored in satoshis (integer string, e.g., "100000" = 0.001 BTC)
- BSC/BNB: stored in wei (e.g., "1000000000000000000" = 1 BNB)
- BSC tokens: stored in token smallest unit (USDC has 18 decimals on BSC, USDT has 18 decimals on BSC)
- SOL: stored in lamports (e.g., "1000000000" = 1 SOL)
- SOL tokens: stored in token smallest unit (USDC has 6 decimals on SOL, USDT has 6 decimals on SOL)

**IMPORTANT**: USDC/USDT decimals differ by chain:
| Token | BSC Decimals | SOL Decimals |
|-------|-------------|-------------|
| USDC | 18 | 6 |
| USDT | 18 | 6 |

Store a `token_decimals` mapping in constants:
```go
var TokenDecimals = map[string]map[string]int{
    "BSC": {"NATIVE": 18, "USDC": 18, "USDT": 18},
    "SOL": {"NATIVE": 9, "USDC": 6, "USDT": 6},
    "BTC": {"NATIVE": 8},
}
```

### Migration Strategy

Use numbered SQL files in `internal/db/migrations/`:
```
001_initial.sql
002_add_scan_state.sql
003_add_settings.sql
```

On startup, check which migrations have been applied (use a `schema_migrations` table) and apply pending ones in order.

---

## 7. Transaction Engine Deep Dive

### BTC — Multi-Input UTXO Sweep

**Concept**: Bitcoin uses UTXOs (Unspent Transaction Outputs). Each funded address has one or more UTXOs. To sweep, we create a single transaction that spends all UTXOs from all funded addresses into one output.

**Step-by-step:**

1. **Fetch UTXOs** for each funded address:
```
GET https://blockstream.info/api/address/{addr}/utxo

Response: [
  { "txid": "abc...", "vout": 0, "value": 50000, "status": { "confirmed": true } },
  { "txid": "def...", "vout": 1, "value": 30000, "status": { "confirmed": true } }
]
```

2. **Calculate total input value**: sum of all UTXO values
3. **Estimate transaction size** (for fee calculation):
```
Size ≈ 10 + (numInputs × 68) + (numOutputs × 31)
For SegWit (P2WPKH): vsize ≈ 10.5 + (numInputs × 68) + (numOutputs × 31)
More accurately: weight = 40 + (numInputs × 272) + (numOutputs × 124)
vsize = ceil(weight / 4)
```

4. **Get fee rate**:
```
GET https://mempool.space/api/v1/fees/recommended
Response: { "fastestFee": 15, "halfHourFee": 10, "hourFee": 5, "economyFee": 2 }
```

5. **Calculate fee**: `vsize × feeRate` (satoshis)
6. **Calculate output amount**: `totalInputValue - fee`
7. **Build transaction**:
```go
tx := wire.NewMsgTx(wire.TxVersion)

// Add all inputs
for _, utxo := range allUTXOs {
    hash, _ := chainhash.NewHashFromStr(utxo.TxID)
    outPoint := wire.NewOutPoint(hash, utxo.Vout)
    txIn := wire.NewTxIn(outPoint, nil, nil)
    tx.AddTxIn(txIn)
}

// Add single output
destAddr, _ := btcutil.DecodeAddress(destination, netParams)
pkScript, _ := txscript.PayToAddrScript(destAddr)
tx.AddTxOut(wire.NewTxOut(outputAmount, pkScript))
```

8. **Sign each input** with corresponding private key:
```go
for i, utxo := range allUTXOs {
    privKey := derivePrivateKey(mnemonic, utxo.AddressIndex) // on-demand
    
    // For P2WPKH (bech32), use witness signing
    sigHashes := txscript.NewTxSigHashes(tx, prevOutFetcher)
    witness, _ := txscript.WitnessSignature(tx, sigHashes, i, utxo.Value,
        utxo.PkScript, txscript.SigHashAll, privKey, true)
    tx.TxIn[i].Witness = witness
    
    zeroKey(privKey) // immediately discard
}
```

9. **Broadcast**: Serialize to hex, POST to `https://blockstream.info/api/tx`

**Edge cases:**
- Address with 0 confirmed UTXOs but pending ones → skip (don't spend unconfirmed)
- Total inputs < minimum fee → skip this address
- Too many inputs (tx > 100KB) → split into multiple transactions
- Fee estimation fails → fall back to conservative constant from config

### BSC — Sequential EVM Transfers

**Native BNB sweep:**
```go
for _, addr := range fundedAddresses {
    privKey := deriveEVMPrivateKey(mnemonic, addr.Index)
    
    nonce, _ := client.PendingNonceAt(ctx, addr.Address)
    gasPrice, _ := client.SuggestGasPrice(ctx)
    gasLimit := config.BSC_GasLimit_Transfer // 21000
    
    gasCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
    sendAmount := new(big.Int).Sub(addr.Balance, gasCost)
    
    if sendAmount.Sign() <= 0 {
        slog.Warn("balance too low to cover gas", "address", addr.Address, "balance", addr.Balance)
        continue
    }
    
    tx := types.NewTx(&types.LegacyTx{
        Nonce:    nonce,
        GasPrice: gasPrice,
        Gas:      gasLimit,
        To:       &destination,
        Value:    sendAmount,
    })
    
    chainID := big.NewInt(56) // BSC mainnet (97 for testnet)
    signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(chainID), privKey)
    client.SendTransaction(ctx, signedTx)
    
    zeroKey(privKey)
    
    // Wait for receipt
    receipt := waitForReceipt(ctx, client, signedTx.Hash())
    // Log, update DB
}
```

**BEP-20 Token sweep:**
Same as above but:
- `To` = token contract address
- `Value` = 0 (not sending BNB)
- `Data` = ABI-encoded `transfer(destination, amount)`
- `Gas` = 65,000 (higher for contract interaction)
- Address MUST have BNB for gas → check first, error if not (this is where gas pre-seeding comes in)

```go
// ABI encode: transfer(address,uint256)
transferFnSig := crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
paddedDest := common.LeftPadBytes(destination.Bytes(), 32)
paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)
data := append(transferFnSig, paddedDest...)
data = append(data, paddedAmount...)
```

### SOL — Multi-Instruction Batch

```go
func SweepSOLNative(ctx context.Context, funded []FundedAddress, dest solana.PublicKey) ([]solana.Signature, error) {
    rpcClient := rpc.New("https://api.mainnet-beta.solana.com")
    
    // Solana tx has ~1232 byte limit. Each transfer instruction ≈ 52 bytes.
    // With overhead, ~20 transfers per tx is safe.
    batchSize := config.SOL_MaxInstructions // 20
    
    var allSigs []solana.Signature
    
    for i := 0; i < len(funded); i += batchSize {
        end := min(i+batchSize, len(funded))
        batch := funded[i:end]
        
        var instructions []solana.Instruction
        var signers []solana.PrivateKey
        
        // Fee payer = first signer
        feePayerKey := deriveSolanaKey(mnemonic, batch[0].Index)
        signers = append(signers, feePayerKey)
        
        for _, addr := range batch {
            key := deriveSolanaKey(mnemonic, addr.Index)
            
            // Leave enough for rent exemption + fee share
            sendAmount := addr.Lamports - 5000 // reserve for fee
            
            instructions = append(instructions, system.NewTransferInstruction(
                sendAmount,
                key.PublicKey(),
                dest,
            ).Build())
            
            if addr.Index != batch[0].Index { // fee payer already added
                signers = append(signers, key)
            }
        }
        
        recent, _ := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
        
        tx, _ := solana.NewTransaction(
            instructions,
            recent.Value.Blockhash,
            solana.TransactionPayer(signers[0].PublicKey()),
        )
        
        _, err := tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
            for _, s := range signers {
                if s.PublicKey() == key {
                    return &s
                }
            }
            return nil
        })
        
        sig, _ := rpcClient.SendTransaction(ctx, tx)
        allSigs = append(allSigs, sig)
        
        // Zero all private keys
        for _, s := range signers {
            zeroKey(s)
        }
    }
    
    return allSigs, nil
}
```

**SPL Token sweep**: Similar but uses `spl_token.NewTransferInstruction` instead of `system.NewTransferInstruction`. Requires:
1. Source = ATA of the sender for this mint
2. Destination = ATA of the receiver for this mint (create if doesn't exist)
3. Authority = sender's keypair

---

## 8. Gas Pre-Seeding Workflow

This only applies to BSC when sweeping BEP-20 tokens. Each address that holds USDC/USDT but has 0 BNB needs gas to execute the token transfer.

### User Flow

```
1. User selects "Send BSC USDC"
2. Backend identifies addresses with USDC but 0 BNB:
   - Address 142: 50 USDC, 0 BNB ← needs gas
   - Address 287: 100 USDC, 0.01 BNB ← has gas
   - Address 891: 25 USDC, 0 BNB ← needs gas
3. Frontend shows: "2 addresses need gas pre-seeding"
4. User selects gas source: address 0 (which has 1 BNB)
5. User clicks "Pre-seed Gas"
6. Backend sends 0.005 BNB to address 142 and 0.005 BNB to address 891
   - 2 separate transactions from address 0
   - Total cost: ~0.01 BNB + gas fees
7. Wait for confirmations
8. Now user can proceed with "Send All USDC"
```

### Pre-seed Amount Calculation

Default: `BSC_GasPreSeed_Wei = 0.005 BNB` per address.
BEP-20 transfer gas: ~65,000 gas × ~3 gwei = ~0.000195 BNB.
So 0.005 BNB gives ~25× safety margin for gas price fluctuation.

### Edge Case: Source Has Insufficient BNB

Before starting pre-seed, calculate:
```
requiredBNB = numAddressesNeedingGas × preSeedAmount + numAddressesNeedingGas × estimatedGasCost
```
If source balance < requiredBNB, reject with clear error message showing the deficit.

---

## 9. SSE (Server-Sent Events) Architecture

### Why SSE Over WebSocket

- Simpler: unidirectional (server → client only)
- The client never needs to send data over the stream (scan control uses REST endpoints)
- Native browser support via `EventSource`
- Automatic reconnection built into the browser
- Works through HTTP/1.1 without upgrade negotiation

### Backend Implementation

```go
func (h *Handler) HandleSSE(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming unsupported", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    // Create channel for this client
    clientChan := make(chan SSEEvent, 100)
    h.sseHub.Register(clientChan)
    defer h.sseHub.Unregister(clientChan)

    ctx := r.Context()
    keepAlive := time.NewTicker(config.SSEKeepAliveInterval)
    defer keepAlive.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case event := <-clientChan:
            fmt.Fprintf(w, "event: %s\n", event.Type)
            fmt.Fprintf(w, "data: %s\n\n", event.Data)
            flusher.Flush()
        case <-keepAlive.C:
            fmt.Fprintf(w, ": keepalive\n\n")
            flusher.Flush()
        }
    }
}
```

### SSE Hub (Fan-out)

```go
type SSEHub struct {
    mu      sync.RWMutex
    clients map[chan SSEEvent]bool
}

func (h *SSEHub) Broadcast(event SSEEvent) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    for ch := range h.clients {
        select {
        case ch <- event:
        default:
            // Client buffer full, skip (will get next event)
            slog.Warn("SSE client buffer full, dropping event", "type", event.Type)
        }
    }
}
```

### Frontend EventSource

```typescript
function connectSSE(): void {
  const source = new EventSource(`${API_BASE}/scan/sse`);

  source.addEventListener('scan_progress', (e: MessageEvent) => {
    const data: ScanProgress = JSON.parse(e.data);
    scanStore.updateProgress(data);
  });

  source.addEventListener('scan_complete', (e: MessageEvent) => {
    const data: ScanComplete = JSON.parse(e.data);
    scanStore.markComplete(data);
  });

  source.addEventListener('tx_status', (e: MessageEvent) => {
    const data: TxStatus = JSON.parse(e.data);
    transactionStore.updateStatus(data);
  });

  source.onerror = () => {
    // EventSource automatically reconnects
    // Log for debugging
    console.warn('SSE connection lost, reconnecting...');
  };
}
```

---

## 10. Frontend Architecture

### Page Layout

```
┌─────────────────────────────────────────────────────┐
│  HDPay                              [mainnet ▼]     │
├──────────┬──────────────────────────────────────────┤
│          │                                          │
│  📊 Dash │  [Main Content Area]                     │
│  📋 Addr │                                          │
│  🔍 Scan │                                          │
│  📤 Send │                                          │
│  📝 Txns │                                          │
│  ⚙ Settings                                        │
│          │                                          │
└──────────┴──────────────────────────────────────────┘
```

### Dashboard Page
- Total portfolio value (USD) — large number
- Per-chain cards: BTC (₿), BSC (BNB), SOL
- Each card shows: native balance + USD, token balances + USD
- Pie chart: chain distribution
- Bar chart: token distribution
- Quick stats: total addresses, addresses with balance, last scan time

### Addresses Page
- Chain selector tabs (BTC | BSC | SOL)
- Virtual scrolled table: Index, Address (truncated, copyable), Balance, Last Scanned
- Filter: "Has balance only" toggle
- Export JSON button
- Pagination for non-virtual mode

### Scan Page
- Per-chain scan cards
- Max ID input (number, with saved value)
- Start/Stop button
- Progress bar with: scanned/total, found count, elapsed time, ETA
- Scan history (last 10 scans per chain)

### Send Page
- Step 1: Select chain + token
- Step 2: Review funded addresses (table with checkboxes? or just "Send All")
  - Shows total amount, number of addresses, estimated fees
- Step 3: Enter destination address (with chain-specific validation)
- Step 4: For BSC tokens — gas pre-seed sub-step:
  - Show addresses needing gas
  - Select gas source
  - Pre-seed button
  - Wait for confirmations
- Step 5: Confirm and execute
- Step 6: Progress tracking (real-time tx status via SSE)
- Step 7: Receipt (list of tx hashes, each linked to explorer)

### Explorer Links

```typescript
export const EXPLORER_URLS = {
  BTC: {
    mainnet: { tx: 'https://mempool.space/tx/', address: 'https://mempool.space/address/' },
    testnet: { tx: 'https://mempool.space/testnet/tx/', address: 'https://mempool.space/testnet/address/' },
  },
  BSC: {
    mainnet: { tx: 'https://bscscan.com/tx/', address: 'https://bscscan.com/address/' },
    testnet: { tx: 'https://testnet.bscscan.com/tx/', address: 'https://testnet.bscscan.com/address/' },
  },
  SOL: {
    mainnet: { tx: 'https://solscan.io/tx/', address: 'https://solscan.io/account/' },
    testnet: { tx: 'https://solscan.io/tx/?cluster=devnet', address: 'https://solscan.io/account/?cluster=devnet' },
  },
} as const;
```

---

## 11. Security Model

### Threat Model

Since HDPay runs localhost-only, the main threats are:

1. **CSRF via visited websites**: A malicious website could trigger fetch() to `http://localhost:8080/api/send/execute`. Mitigation: CSRF tokens on all mutating endpoints.
2. **DNS rebinding**: Attacker binds their domain to 127.0.0.1. Mitigation: Check Host header, reject anything not `localhost` or `127.0.0.1`.
3. **Mnemonic exposure**: File on disk. Mitigation: User responsibility (file permissions). Never stored in DB or logs.
4. **DB exposure**: SQLite file contains all addresses. Mitigation: No private keys in DB. Addresses are public by nature.

### CSRF Implementation

```go
func CSRFMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
            // Generate token, set cookie
            token := generateCSRFToken()
            http.SetCookie(w, &http.Cookie{
                Name:     "csrf_token",
                Value:    token,
                HttpOnly: false, // JS needs to read it
                SameSite: http.SameSiteStrictMode,
                Path:     "/",
            })
            next.ServeHTTP(w, r)
            return
        }

        // Validate token on POST/PUT/DELETE
        cookie, err := r.Cookie("csrf_token")
        header := r.Header.Get("X-CSRF-Token")
        if err != nil || cookie.Value != header {
            http.Error(w, "CSRF token mismatch", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Host Header Validation

```go
func HostCheckMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        host := r.Host
        if !isAllowedHost(host) {
            slog.Warn("rejected request with invalid host", "host", host, "remote", r.RemoteAddr)
            http.Error(w, "forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}

func isAllowedHost(host string) bool {
    // Strip port
    h := strings.Split(host, ":")[0]
    return h == "localhost" || h == "127.0.0.1"
}
```

---

## 12. Error Handling Strategy

### Backend Error Propagation

```
Provider error → Scanner → Handler → API response → Frontend

Each layer adds context:
  provider: "blockstream: GET /address/bc1q...: 429 Too Many Requests"
  scanner:  "scan BTC batch 15/100: blockstream: GET /address/bc1q...: 429"
  handler:  returns JSON { "error": { "code": "ERROR_PROVIDER_RATE_LIMIT", "message": "..." } }
  frontend: shows toast notification with user-friendly message
```

### Error Code → User Message Mapping (Frontend)

```typescript
export const ERROR_MESSAGES: Record<string, string> = {
  ERROR_INVALID_MNEMONIC: 'Invalid mnemonic phrase. Please check your 24-word seed.',
  ERROR_SCAN_FAILED: 'Scan failed. Check provider connectivity.',
  ERROR_PROVIDER_RATE_LIMIT: 'API rate limit reached. Scan will resume automatically.',
  ERROR_INSUFFICIENT_BALANCE: 'Insufficient balance to cover transaction fees.',
  ERROR_INSUFFICIENT_GAS: 'Addresses need gas (BNB) before sending tokens. Use gas pre-seeding.',
  ERROR_TX_BROADCAST_FAILED: 'Transaction broadcast failed. Please retry.',
  // ...
};
```

### Retry Strategy

| Error Type | Retry? | Strategy |
|-----------|--------|----------|
| Rate limit (429) | Yes | Rotate provider, exponential backoff |
| Network timeout | Yes | Rotate provider, retry 3x |
| Server error (5xx) | Yes | Rotate provider, retry 3x |
| Client error (4xx) | No | Log and skip |
| Invalid address | No | Log and skip |
| Insufficient balance | No | Report to user |
| Mnemonic file not found | No | Fatal error on startup |

---

## 13. Logging Architecture

### Dual Output Setup

```go
func SetupLogger(level string, logDir string) {
    // Parse level
    var slogLevel slog.Level
    switch level {
    case "debug": slogLevel = slog.LevelDebug
    case "info":  slogLevel = slog.LevelInfo
    case "warn":  slogLevel = slog.LevelWarn
    case "error": slogLevel = slog.LevelError
    default:      slogLevel = slog.LevelInfo
    }

    // File output: daily rotation
    today := time.Now().Format("2006-01-02")
    logFile := filepath.Join(logDir, fmt.Sprintf(config.LogFilePattern, today))
    file, _ := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)

    // Multi-writer
    multiWriter := io.MultiWriter(os.Stdout, file)

    handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
        Level: slogLevel,
    })

    slog.SetDefault(slog.New(handler))
}
```

### Log Rotation

At midnight (or on new day detection), close old file and open new one. Simplest approach: check date on each log write, or use a goroutine that ticks at midnight.

Delete logs older than `LogMaxAgeDays` (30 days).

### What Gets Logged

| Event | Level | Fields |
|-------|-------|--------|
| Server start | INFO | port, network, dbPath |
| HTTP request | INFO | method, path, status, duration, remoteAddr |
| Address generation batch | INFO | chain, batchNum, count, elapsed |
| Scan start | INFO | chain, maxID, resumeFrom, providerCount |
| Scan batch | DEBUG | chain, batchIndex, addresses, provider |
| Balance found | INFO | chain, index, address, token, balance |
| Provider call | DEBUG | provider, endpoint, batchSize, duration, statusCode |
| Provider rate limit | WARN | provider, retryAfter |
| Provider error | ERROR | provider, endpoint, error |
| Provider rotation | INFO | from, to, reason |
| Scan complete | INFO | chain, total, found, duration |
| TX build start | INFO | chain, token, addressCount, destination |
| TX sign | DEBUG | chain, index, txHash |
| TX broadcast | INFO | chain, txHash, status |
| TX confirmed | INFO | chain, txHash, blockNumber, confirmations |
| TX failed | ERROR | chain, txHash, error |
| Gas pre-seed | INFO | source, targets, amount, txHash |
| Price fetch | DEBUG | source, prices |
| DB migration | INFO | version, file |
| Config loaded | INFO | network, port, logLevel (NEVER log mnemonic or keys) |

---

## 14. Testing Strategy

### Test Vectors for Address Derivation

Use the standard BIP-39 test mnemonic:
```
abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art
```

Known addresses for this mnemonic (verify these against real wallets):

**BTC (m/44'/0'/0'/0/0, bech32)**:
- Index 0: `bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu`
- Verify against: Electrum or any BIP-44 bech32 wallet

**BSC/ETH (m/44'/60'/0'/0/0)**:
- Index 0: `0x9858EfFD232B4033E47d90003D41EC34EcaEda94`
- Verify against: MetaMask "abandon" mnemonic

**SOL (m/44'/501'/0'/0')**:
- Index 0: Verify against Phantom wallet with same mnemonic

### Mock Providers for Scanner Tests

```go
type MockProvider struct {
    Name_      string
    Balances   map[string]map[string]string // address → token → balance
    CallCount  int
    ShouldFail bool
    FailAfter  int
}

func (m *MockProvider) GetBalances(ctx context.Context, addrs []string) (map[string]map[string]string, error) {
    m.CallCount++
    if m.ShouldFail || (m.FailAfter > 0 && m.CallCount > m.FailAfter) {
        return nil, errors.New("mock provider error")
    }
    result := make(map[string]map[string]string)
    for _, addr := range addrs {
        if bal, ok := m.Balances[addr]; ok {
            result[addr] = bal
        }
    }
    return result, nil
}
```

### Testnet Integration Tests

These are slower and require network access. Tag them separately:

```go
//go:build integration

func TestBTCScanTestnet(t *testing.T) {
    // Fund a testnet address via faucet first
    // Then scan and verify balance
}
```

Run with: `go test -tags=integration ./...`

### Frontend Component Tests

Focus on:
1. ScanControl: start/stop behavior, progress display
2. SendPanel: validation, confirmation dialog, step flow
3. AddressTable: pagination, filtering, virtual scroll
4. PortfolioOverview: correct number formatting, USD calculation

---

## 15. Build & Deployment

### Embedding Frontend in Go Binary

```go
//go:embed web/build/*
var staticFiles embed.FS

func setupStaticServer(r chi.Router) {
    // Serve SvelteKit static build
    sub, _ := fs.Sub(staticFiles, "web/build")
    fileServer := http.FileServer(http.FS(sub))

    r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Try to serve static file first
        // Fall back to index.html for SPA routing
        path := r.URL.Path
        if _, err := fs.Stat(sub, strings.TrimPrefix(path, "/")); err != nil {
            // File not found → serve index.html (SPA routing)
            r.URL.Path = "/"
        }
        fileServer.ServeHTTP(w, r)
    }))
}
```

### Build Script

```bash
#!/bin/bash
set -e

echo "Building frontend..."
cd web
npm ci
npm run build
cd ..

echo "Building Go binary..."
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags --always)" -o hdpay ./cmd/server

echo "Done: ./hdpay"
ls -lh hdpay
```

`CGO_ENABLED=0` because we use `modernc.org/sqlite` (pure Go). No CGO needed.

### Directory Structure at Runtime

```
./hdpay                     # Binary
./data/
  hdpay.sqlite              # Database
  export/
    btc_addresses.json
    bsc_addresses.json
    sol_addresses.json
./logs/
  hdpay-2026-02-18.log
  hdpay-2026-02-17.log
.env                        # Configuration
```

---

## 16. Edge Cases & Failure Modes

### Address Generation
- **Mnemonic file has trailing newline**: Trim whitespace before validation
- **Mnemonic file has wrong permissions**: Check readability, clear error message
- **DB already has addresses**: Idempotent — skip if correct count exists
- **Partial init (crash mid-generation)**: Detect via count mismatch, wipe chain and regenerate
- **Disk full during generation**: Transaction rollback keeps DB consistent

### Scanning
- **All providers down**: Pause scan, emit error SSE, retry after 60s
- **Provider returns garbage data**: Validate response structure, skip and log
- **Address has dust balance** (e.g., 1 satoshi): Still track it, but warn in UI if below threshold
- **Address has pending (unconfirmed) tx**: Track separately, show in UI
- **Scan interrupted by server restart**: Resume from `scan_state.last_scanned_index`
- **Two concurrent scan requests for same chain**: Reject second with 409 Conflict
- **Max ID changed during scan**: Use the max ID that was set at scan start
- **Provider returns balance in unexpected format**: Parse error, log, skip address, continue scan

### Sending
- **Destination address invalid for chain**: Validate format before preview
- **Destination is one of our own addresses**: Allow but warn
- **Balance changed between preview and execute**: Re-fetch at execute time, adjust amounts
- **BTC: UTXO already spent** (race condition): Broadcast will fail, show error, suggest re-scan
- **BSC: Nonce conflict**: If sequential sends fail, the nonce gets stuck. Implement nonce management with gap detection.
- **BSC: Gas price spike during batch send**: Later transactions may fail if gas increased. Allow user to set manual gas price.
- **SOL: Transaction too large** (too many signers): Split into multiple transactions of ≤20 instructions
- **SOL: Blockhash expired**: SOL blockhashes are valid for ~60 seconds. For large batches, fetch new blockhash per batch.
- **Network disconnect during batch send**: Track which txs were sent successfully, resume from failure point
- **Private key derivation fails**: Should never happen with valid mnemonic, but handle gracefully

### Database
- **WAL file grows large**: Periodic checkpoint (`PRAGMA wal_checkpoint(TRUNCATE)`)
- **Concurrent reads during write**: WAL mode handles this
- **Corruption**: Integrity check on startup (`PRAGMA integrity_check`)
- **Migration fails**: Rollback migration, log error, refuse to start

### Frontend
- **SSE connection lost**: `EventSource` auto-reconnects, show reconnecting indicator
- **Large table rendering** (500K rows): MUST use virtual scrolling
- **Long-running scan blocks UI**: Scan runs in backend goroutine, UI only receives SSE events
- **Multiple browser tabs**: All receive SSE events independently (fine)

---

## 17. Data Flow Diagrams

### Scan Flow

```
User clicks "Scan BTC to 5000"
         │
         ▼
POST /api/scan/start {"chain":"BTC","maxID":5000}
         │
         ▼
Handler validates → Creates context with cancel → Spawns goroutine
         │                                              │
         ▼                                              ▼
Returns 200 {"status":"started"}          Scanner.Scan(ctx, "BTC", 5000)
         │                                              │
         ▼                                              ▼
Frontend connects to                     Load addresses[0:5000] from DB
GET /api/scan/sse                                       │
         │                                              ▼
         │                                 Chunk into batches of 50
         │                                              │
         │                               ┌──────────────┼──────────────┐
         │                               │              │              │
         │                          Blockstream     Mempool.space  Blockchain.info
         │                          (batch 1)      (batch 2)      (batch 3)
         │                               │              │              │
         │                               └──────────────┼──────────────┘
         │                                              │
         │◄──── SSE: scan_progress ─────────────────────┤
         │      {"scanned":50,"total":5000}             │
         │                                              │
         │                                    Upsert balances in DB
         │                                              │
         │                                         ... repeat ...
         │                                              │
         │◄──── SSE: scan_complete ─────────────────────┤
         │      {"found":12,"duration":"6m"}            │
         ▼                                              ▼
Frontend updates UI                           Scan goroutine exits
```

### Send Flow

```
User selects "Send BSC USDC → 0xDest"
         │
         ▼
POST /api/send/preview {"chain":"BSC","token":"USDC","destination":"0xDest"}
         │
         ▼
Backend queries: SELECT funded BSC USDC addresses
         │
         ▼
Returns: {
  "addresses": [...],
  "totalAmount": "175000000000000000000",  // 175 USDC
  "addressCount": 28,
  "needsGasPreSeed": 15,     // 15 addresses have 0 BNB
  "estimatedGasCost": "...",
  "estimatedTxCount": 28     // BSC = sequential
}
         │
         ▼
Frontend shows preview → User sees "15 addresses need gas"
         │
         ▼ (User clicks "Pre-seed Gas")
POST /api/send/gas-preseed {"sourceIndex":0,"targetIndices":[142,287,...],"amountWei":"5000000000000000"}
         │
         ▼
Backend sends BNB to each target (sequential)
SSE: tx_status for each
         │
         ▼
All gas pre-seeded → User clicks "Send All USDC"
         │
         ▼
POST /api/send/execute {"chain":"BSC","token":"USDC","destination":"0xDest"}
         │
         ▼
Backend: for each funded address:
  1. Derive private key
  2. Build BEP-20 transfer tx
  3. Sign
  4. Broadcast
  5. Wait for receipt
  6. SSE: tx_status
  7. Zero key
         │
         ▼
All complete → Frontend shows receipt with all tx hashes
```

---

## 18. Task Dependency Graph

```
┌─────────────────────────────────────────────────────────────────┐
│ Phase 0: Foundation                                             │
│                                                                 │
│  [Go module init] ──► [Directory structure] ──► [Constants]     │
│         │                     │                      │          │
│         ▼                     ▼                      ▼          │
│  [SQLite setup] ──► [Migrations] ──► [Logger setup]             │
│         │                                    │                  │
│         ▼                                    ▼                  │
│  [Chi router + middleware] ──► [Health endpoint]                 │
│         │                                                       │
│         ▼                                                       │
│  [SvelteKit scaffold] ──► [Tailwind + shadcn] ──► [Layout]     │
│                                                                 │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│ Phase 1: Wallet & Addresses                                     │
│                                                                 │
│  [BIP-39 mnemonic validation] ──► [BIP-32 master key]          │
│         │                              │                        │
│         ├──► [BTC derivation + test] ──┤                        │
│         ├──► [BSC derivation + test] ──┤                        │
│         └──► [SOL derivation + test] ──┤                        │
│                                        │                        │
│                                   [Init CLI command]            │
│                                        │                        │
│                                   [Batch DB insert]             │
│                                        │                        │
│                                   [JSON export]                 │
│                                        │                        │
│  [Address API endpoints] ──► [Address frontend page]            │
│                                                                 │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│ Phase 2: Scanning                                               │
│                                                                 │
│  [Provider interface] ──► [Rate limiter]                        │
│         │                      │                                │
│         ├──► [Blockstream provider + test]──┐                   │
│         ├──► [Mempool provider + test]──────┤                   │
│         ├──► [Blockchain.info provider]─────┤                   │
│         ├──► [BscScan provider + test]──────┤                   │
│         ├──► [BSC RPC provider + test]──────┤                   │
│         ├──► [Solana RPC provider + test]───┤                   │
│         └──► [Helius provider + test]───────┤                   │
│                                             │                   │
│                              [Provider pool + rotation]         │
│                                             │                   │
│                              [Scanner orchestrator]             │
│                                    │                            │
│                              [Resume logic]                     │
│                                    │                            │
│  [SSE hub] ──► [Scan SSE endpoint]                              │
│                       │                                         │
│  [Scan API endpoints] ──► [Scan frontend page]                  │
│                                                                 │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│ Phase 3: Dashboard                                              │
│                                                                 │
│  [CoinGecko price service] ──► [Price caching]                  │
│                                      │                          │
│  [Balance summary API] ──► [Portfolio API]                      │
│                                      │                          │
│  [Dashboard frontend] ──► [ECharts integration]                 │
│                                                                 │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│ Phase 4: Transactions                                           │
│                                                                 │
│  [On-demand key derivation service]                             │
│         │                                                       │
│         ├──► [BTC UTXO fetcher] ──► [BTC tx builder + signer]  │
│         │         │                        │                    │
│         │    [BTC broadcaster] ◄───────────┘                    │
│         │                                                       │
│         ├──► [BSC tx builder] ──► [BSC signer + broadcaster]   │
│         │         │                                             │
│         │    [BSC nonce manager]                                │
│         │                                                       │
│         ├──► [SOL tx builder] ──► [SOL signer + broadcaster]   │
│         │                                                       │
│         └──► [Gas pre-seeder (BSC)]                             │
│                                                                 │
│  [Send preview API] ──► [Send execute API]                      │
│                                │                                │
│  [Send frontend page] ──► [Gas pre-seed UI] ──► [TX progress]  │
│                                                                 │
└──────────────────────────────┬──────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────┐
│ Phase 5: History & Settings                                     │
│                                                                 │
│  [Transaction history API] ──► [History frontend page]          │
│                                                                 │
│  [Settings API] ──► [Settings frontend page]                    │
│                                                                 │
│  [Build script] ──► [Embed frontend] ──► [Final binary]        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Estimated Effort Per Phase

| Phase | Estimated Time | Core Files | Tests |
|-------|---------------|------------|-------|
| 0: Foundation | 2-3 hours | ~15 files | Setup only |
| 1: Wallet & Addresses | 4-6 hours | ~12 files | ~8 test files |
| 2: Scanning | 8-12 hours | ~15 files | ~10 test files |
| 3: Dashboard | 3-4 hours | ~8 files | ~4 test files |
| 4: Transactions | 10-15 hours | ~12 files | ~8 test files |
| 5: History & Settings | 3-4 hours | ~6 files | ~4 test files |
| **Total** | **~30-44 hours** | **~68 files** | **~34 test files** |

Phase 4 (Transactions) is the most complex and riskiest — it involves real money. Test extensively on testnet before touching mainnet.

---

## Appendix: Token Decimal Reference

| Chain | Token | Contract/Mint | Decimals | 1.0 = Raw Value |
|-------|-------|--------------|----------|------------------|
| BTC | BTC | N/A | 8 | 100,000,000 satoshis |
| BSC | BNB | Native | 18 | 1,000,000,000,000,000,000 wei |
| BSC | USDC | 0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d | 18 | 1,000,000,000,000,000,000 |
| BSC | USDT | 0x55d398326f99059fF775485246999027B3197955 | 18 | 1,000,000,000,000,000,000 |
| SOL | SOL | Native | 9 | 1,000,000,000 lamports |
| SOL | USDC | EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v | 6 | 1,000,000 |
| SOL | USDT | Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB | 6 | 1,000,000 |

## Appendix: Chain ID Reference

| Chain | Mainnet | Testnet |
|-------|---------|---------|
| BTC | N/A (use chaincfg.MainNetParams) | chaincfg.TestNet3Params |
| BSC | 56 | 97 |
| SOL | N/A (use RPC URL) | N/A (use devnet RPC URL) |

## Appendix: BIP-44 Coin Types

| Chain | Coin Type | Hardened |
|-------|-----------|----------|
| BTC (mainnet) | 0 | 0x80000000 |
| BTC (testnet) | 1 | 0x80000001 |
| BSC (EVM) | 60 | 0x8000003c |
| SOL | 501 | 0x800001f5 |
