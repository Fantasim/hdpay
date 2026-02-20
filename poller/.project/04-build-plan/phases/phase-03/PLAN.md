# Phase 3: Blockchain Providers

<objective>
Implement the blockchain provider layer for transaction detection. Poller needs a DIFFERENT Provider interface than HDPay's (HDPay scans balances, Poller detects individual transactions), but reuses HDPay's rate limiter and circuit breaker. Chain-specific detection for BTC, BSC, and SOL with testnet verification.
</objective>

<features>
F7 — BTC Detection (Blockstream/Mempool tx parsing, pagination)
F8 — BSC Detection (BscScan txlist + tokentx, confirmation counting)
F9 — SOL Detection (getSignaturesForAddress + getTransaction, composite tx_hash)
F14 — Provider Round-Robin (per-chain sets, rotate on failure, rate-limited)
F15 — Provider Failure Logging (system_errors table)
</features>

<tasks>

## Task 1: Provider Interface & Round-Robin

**Create** Poller's provider abstraction and rotation logic.

**Files to create:**
- `internal/poller/provider/provider.go` — Provider interface, ProviderSet (round-robin), RawTransaction struct

**What to import from HDPay (not rewrite):**
- `internal/scanner/ratelimiter.go` — `RateLimiter`, `NewRateLimiter()`, `Wait(ctx)`
- `internal/scanner/circuit_breaker.go` — `CircuitBreaker`, `NewCircuitBreaker()`, `Allow()`, `RecordSuccess/Failure()`
- `internal/config/constants.go` — provider URLs, rate limit values

**Details:**
- Poller's `Provider` interface (DIFFERENT from HDPay's — detects transactions, not balances):
  ```go
  type Provider interface {
      Name() string
      Chain() models.Chain
      FetchTransactions(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error)
      CheckConfirmation(ctx context.Context, txHash string, blockNumber int64) (confirmed bool, confirmations int, err error)
      GetCurrentBlock(ctx context.Context) (uint64, error)
  }
  ```
- `RawTransaction` struct: TxHash, Token, AmountRaw (string), AmountHuman (string), Decimals (int), BlockTime (int64), Confirmed (bool), Confirmations (int), BlockNumber (int64)
- `ProviderSet`: holds `[]Provider` for one chain, each with HDPay's RateLimiter + CircuitBreaker
  - `Execute(ctx context.Context, fn func(Provider) ([]RawTransaction, error)) ([]RawTransaction, error)` — try current, rotate on failure
  - On failure: log WARN, rotate. On all fail: log system error to DB, return error
- Thread-safe rotation with mutex

**Verification:**
- Round-robin cycles correctly
- On provider failure, rotates and retries
- Rate limiter throttles per provider
- Circuit breaker trips after threshold failures

## Task 2: BTC Provider

**Create** BTC transaction detection via Blockstream and Mempool.space APIs.

**Files to create:**
- `internal/poller/provider/btc.go` — BlockstreamProvider, MempoolProvider

**What to import from HDPay:**
- `internal/config` — `BlockstreamMainnetURL`, `BlockstreamTestnetURL`, `MempoolMainnetURL`, `MempoolTestnetURL`, `BTCDecimals`

**Details:**
- Two providers implementing Poller's Provider interface

- `FetchTransactions(ctx, address, cutoffUnix)`:
  1. `GET /api/address/{address}/txs` — returns tx list
  2. For each tx:
     - Skip if `tx.status.block_time < cutoffUnix`
     - Parse `tx.vout[]` — find outputs where `scriptpubkey_address == address`
     - Sum matching output values (satoshis)
     - Skip if no outputs to our address (outgoing tx)
     - Convert: `amount_human = satoshis / 100_000_000`
     - Confirmed = `tx.status.confirmed == true`
  3. Pagination: 25 per page. If 25 results, fetch next page with `?after_txid={last_txid}`. Stop when tx older than cutoff.

- `CheckConfirmation(ctx, txHash, blockNumber)`:
  - `GET /api/tx/{txid}`
  - Return `tx.status.confirmed`, 1 if confirmed else 0

- Network-aware URL construction (mainnet vs testnet from config)

**Verification:**
- Fetch transactions for `tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr` (funded testnet address)
- Correctly identifies incoming amounts
- Pagination works
- Confirmation status parsed correctly

## Task 3: BSC Provider

**Create** BSC transaction detection via BscScan API.

**Files to create:**
- `internal/poller/provider/bsc.go` — BscScanProvider

**What to import from HDPay:**
- `internal/config` — `BscScanAPIURL`, `BscScanTestnetAPIURL`, `BSCUSDCContract`, `BSCUSDTContract`, `BNBDecimals`, `BSCUSDCDecimals`, `BSCUSDTDecimals`

**Details:**
- `FetchTransactions(ctx, address, cutoffUnix)`:
  1. **Normal transactions (BNB)**: `GET /api?module=account&action=txlist&address={addr}&sort=desc&apikey={key}`
     - Skip if `timeStamp < cutoff`, `to != address`, `isError == "1"`
     - Token = "BNB", amount_raw = tx.value (wei)
  2. **Token transactions (USDC + USDT)**: `GET /api?module=account&action=tokentx&address={addr}&sort=desc&apikey={key}`
     - Skip if `timeStamp < cutoff`, `to != address`
     - Skip if `contractAddress` not in [USDC, USDT] constants
     - Determine token from contract address

- `CheckConfirmation(ctx, txHash, blockNumber)`:
  - `currentBlock = GetCurrentBlock(ctx)`
  - `confirmations = currentBlock - blockNumber`
  - Confirmed if `confirmations >= ConfirmationsBSC` (12)

- `GetCurrentBlock(ctx)`:
  - `GET /api?module=proxy&action=eth_blockNumber&apikey={key}`
  - Parse hex to uint64

**Verification:**
- Fetch transactions for `0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb` (funded BSC testnet)
- Correctly filters incoming vs outgoing
- Token transactions identified by contract address
- Confirmation counting works

## Task 4: SOL Provider

**Create** SOL transaction detection via Solana JSON-RPC.

**Files to create:**
- `internal/poller/provider/sol.go` — SolanaRPCProvider

**What to import from HDPay:**
- `internal/config` — `SolanaMainnetRPCURL`, `SolanaDevnetRPCURL`, `HeliusMainnetRPCURL`, `SOLUSDCMint`, `SOLUSDTMint`, `SOLDecimals`, `SOLUSDCDecimals`, `SOLUSDTDecimals`

**Details:**
- `FetchTransactions(ctx, address, cutoffUnix)`:
  1. RPC: `getSignaturesForAddress(address, {limit: 20, commitment: "confirmed"})`
  2. For each signature:
     - Skip if `blockTime < cutoffUnix`, skip if `err != null`
     - RPC: `getTransaction(signature, {commitment: "confirmed", maxSupportedTransactionVersion: 0})`
     - Parse **native SOL**: `incoming_lamports = postBalances[idx] - preBalances[idx]` (if positive)
     - Parse **SPL tokens**: check `pre/postTokenBalances` for owner==address AND mint in [USDC, USDT]
     - **A single tx can have BOTH native + token transfers** → separate RawTransaction entries
     - **Composite tx_hash**: `"signature:SOL"`, `"signature:USDC"`, etc.

  3. Confirmation: `confirmationStatus == "finalized"` → confirmed, else PENDING

- `CheckConfirmation(ctx, txHash, blockNumber)`:
  - Extract base signature (remove `:TOKEN` suffix)
  - RPC: `getSignatureStatuses([signature], {searchTransactionHistory: true})`
  - Confirmed if `confirmationStatus == "finalized"`

**Verification:**
- Fetch transactions for `3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx` (funded SOL devnet)
- Native SOL transfers detected
- SPL token (USDC) transfers detected
- Composite tx_hash format works
- Finalized vs confirmed status parsed

## Task 5: Provider Tests

**Create** tests for all providers.

**Files to create:**
- `internal/poller/provider/provider_test.go` — ProviderSet round-robin, circuit breaker integration
- `internal/poller/provider/btc_test.go` — BTC response parsing (mock HTTP)
- `internal/poller/provider/bsc_test.go` — BSC response parsing (mock HTTP)
- `internal/poller/provider/sol_test.go` — SOL response parsing (mock HTTP)

**Details:**
- Use `httptest.Server` to mock blockchain API responses
- Capture real response JSON from testnet APIs as test fixtures
- Test error handling: HTTP 429, 500, timeout, malformed JSON
- Test round-robin rotation on failure
- SOL: test composite tx_hash generation
- BSC: test confirmation counting logic

**Verification:**
- `go test ./internal/poller/provider/...` passes
- Coverage > 70% on provider package
- All error paths tested

</tasks>

<success_criteria>
- Provider interface is clean and tailored to transaction detection (not balance scanning)
- HDPay's rate limiter and circuit breaker imported and used (not rewritten)
- HDPay's constants imported for URLs, contracts, decimals (not hardcoded)
- Round-robin rotates on failure, circuit breaker trips after threshold
- BTC provider parses Blockstream/Mempool responses correctly with pagination
- BSC provider handles normal + token transactions, stores block_number for confirmation
- SOL provider handles native + SPL transfers with composite tx_hash
- All providers verified against testnet (manual check)
- Provider failures logged to system_errors table
- All tests pass with > 70% coverage
</success_criteria>

<research_needed>
- Solana JSON-RPC response format for getTransaction (complex nested structure — capture real devnet response as test fixture)
- Verify BscScan testnet API URL format (`api-testnet.bscscan.com` vs. other patterns)
</research_needed>
