# Phase 4: Scanner Engine (Backend)

> **Status: Detailed** — Ready to build.

<objective>
Build the complete scanning engine backend: provider interface with round-robin rotation, per-provider rate limiting, 6 provider implementations (2 BTC, 2 BSC, 2 SOL), provider pool, balance + scan state DB operations, scanner orchestrator with resume logic, token scanning (BSC BEP-20 + SOL SPL via ATA), and the SSE hub for broadcasting progress events.
</objective>

## Research Findings

1. **Blockchain.info multiaddr API**: Uncertain bech32 (bc1…) support — historically broken for native SegWit addresses. **Decision: Drop Blockchain.info**. Use only Blockstream + Mempool.space for BTC (both have reliable bech32 support).

2. **BscScan token balance**: `tokenbalance` endpoint is **single-address, single-token** per call (`?module=account&action=tokenbalance&contractaddress=X&address=Y`). `balancemulti` supports batch native balance (up to 20 addresses). `addresstokenbalance` requires paid plan — skip it.

3. **Solana getMultipleAccounts**: Max **100 accounts** per call. Use `jsonParsed` encoding to get structured SPL token data with `tokenAmount.amount`. For SPL tokens: derive Associated Token Account (ATA) addresses, batch-query them, parse `parsed.info.tokenAmount.amount`. Null response means no ATA/zero balance.

4. **Helius free tier**: Same JSON-RPC interface as Solana public RPC, ~10 req/s. Use same provider type, different URL + API key.

5. **ATA derivation**: Implement manually (~40 lines) using `FindProgramAddress` algorithm (SHA-256 + ed25519 curve check). No need for full solana-go dependency.

## Architecture Overview

```
Scanner Orchestrator
  ├── ProviderPool (per chain) → round-robin across providers
  │     ├── Provider 1 (e.g., Blockstream)
  │     └── Provider 2 (e.g., Mempool.space)
  ├── RateLimiter (per provider)
  ├── DB (balances, scan_state)
  └── SSEHub (broadcast progress events)
```

**Scan flow per chain**:
1. Load/resume scan state from DB
2. Fetch addresses from DB in batches (size = provider MaxBatchSize)
3. For each batch → query provider pool for native balance
4. For BSC/SOL: query token balances (USDC, USDT)
5. Upsert results into balances table
6. Update scan_state after each batch
7. Broadcast progress via SSE hub

**Token scanning strategy**:
- **BTC**: Native only (satoshis)
- **BSC native**: BscScan `balancemulti` (batch 20) or ethclient `BalanceAt`
- **BSC tokens**: BscScan `tokenbalance` or ethclient `balanceOf` (single address per call)
- **SOL native**: `getMultipleAccounts` on wallet addresses (batch 100), read `lamports`
- **SOL tokens**: Derive ATA for each wallet+mint, `getMultipleAccounts` on ATAs (batch 100), parse `tokenAmount.amount`

## Tasks

### Task 1: Add Dependencies + Constants

**Files**: `go.mod`, `internal/config/constants.go`

1. Add `golang.org/x/time` dependency for `rate.Limiter`
2. Add provider URL constants to `constants.go`:
   - Blockstream mainnet/testnet URLs
   - Mempool.space mainnet/testnet URLs
   - BscScan API URL
   - BSC RPC URLs (binance + ankr)
   - Solana mainnet RPC URL
   - Helius mainnet RPC URL
   - Solana devnet URL, BSC testnet URL
3. Add Solana program ID constants:
   - `SOLTokenProgramID` = `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`
   - `SOLAssociatedTokenProgramID` = `ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL`
4. Add scanner-related constants:
   - `ScanProgressBroadcastInterval` = 500ms
   - `ProviderRequestTimeout` = 15s
   - `ProviderMaxRetries` = 3
   - `ProviderRetryBaseDelay` = 1s

<verification>
- `go mod tidy` succeeds
- New constants compile without errors
</verification>

### Task 2: Provider Interface + Types

**Files**: `internal/scanner/provider.go`

Define the core abstractions:

```go
// BalanceResult holds balance for a single address
type BalanceResult struct {
    Address      string
    AddressIndex int
    Balance      string // raw balance string (satoshis, wei, lamports)
}

// Provider fetches balance data from an external blockchain API
type Provider interface {
    Name() string
    Chain() models.Chain
    MaxBatchSize() int
    // FetchNativeBalances returns native token balances for a batch of addresses
    FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error)
    // FetchTokenBalances returns token balances for a batch of addresses.
    // contractOrMint is the token contract (BSC) or mint address (SOL).
    // BTC providers should return ErrTokensNotSupported.
    FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractOrMint string) ([]BalanceResult, error)
}
```

Also define:
- `ErrTokensNotSupported` sentinel error
- `ErrProviderRateLimited` sentinel error
- `ErrProviderUnavailable` sentinel error
- Helper type `AddressInfo` if needed

<verification>
- Package compiles
- Interface is usable from other packages
</verification>

### Task 3: Rate Limiter

**Files**: `internal/scanner/ratelimiter.go`

Wrap `golang.org/x/time/rate` with a provider-aware rate limiter:

```go
type RateLimiter struct {
    limiter *rate.Limiter
    name    string
}

func NewRateLimiter(name string, rps int) *RateLimiter
func (rl *RateLimiter) Wait(ctx context.Context) error
func (rl *RateLimiter) Name() string
```

- Uses token bucket algorithm (x/time/rate)
- Logs when waiting, warns when context cancelled
- One limiter per provider instance

<verification>
- Unit test: limiter allows `rps` requests per second, blocks on excess
</verification>

### Task 4: BTC Providers

**Files**: `internal/scanner/btc_blockstream.go`, `internal/scanner/btc_mempool.go`

**Blockstream Esplora** (single address):
- URL: `GET /api/address/{address}`
- Response: `{ "chain_stats": { "funded_txo_sum": N, "spent_txo_sum": M } }`
- Balance = `funded_txo_sum - spent_txo_sum` (in satoshis)
- MaxBatchSize: 1
- `FetchTokenBalances` → returns `ErrTokensNotSupported`

**Mempool.space** (single address):
- URL: `GET /api/address/{address}`
- Same response format as Blockstream
- MaxBatchSize: 1

Both providers:
- Create with `http.Client` + `RateLimiter`
- Handle HTTP errors (429 → rate limited, 5xx → unavailable)
- Log every request/response at DEBUG level
- Parse JSON response, extract balance

<verification>
- Unit tests with httptest mock server
- Test: valid address returns correct balance
- Test: 429 returns ErrProviderRateLimited
- Test: 500 returns ErrProviderUnavailable
- Test: invalid JSON returns appropriate error
</verification>

### Task 5: BSC Providers

**Files**: `internal/scanner/bsc_bscscan.go`, `internal/scanner/bsc_rpc.go`

**BscScan API**:
- Native batch: `?module=account&action=balancemulti&address=addr1,addr2,...&apikey=KEY`
  - MaxBatchSize: 20 (for native)
  - Response: `{ "result": [{ "account": "0x...", "balance": "123456" }] }`
- Token (single): `?module=account&action=tokenbalance&contractaddress=X&address=Y&apikey=KEY`
  - Response: `{ "result": "123456" }` (balance string)
- Handle BscScan error responses (`"status": "0"`)
- API key from config (optional for some endpoints)

**BSC RPC (ethclient)**:
- Native: `ethclient.BalanceAt(ctx, common.HexToAddress(addr), nil)` → returns `*big.Int`
  - MaxBatchSize: 1
- Token: Call `balanceOf(address)` on ERC-20 contract
  - ABI encode: `keccak256("balanceOf(address)")[:4] + padded_address`
  - Use `ethclient.CallContract()` with crafted data
  - Decode `*big.Int` from response
- Connect ethclient with configurable RPC URL

<verification>
- Unit tests: BscScan with httptest mock
- Unit tests: BSC RPC with mock (or test against testnet if available)
- Test: batch native balance parsing
- Test: token balance parsing
- Test: error handling for both providers
</verification>

### Task 6: SOL Providers + ATA Derivation

**Files**: `internal/scanner/sol_rpc.go`, `internal/scanner/sol_ata.go`

**ATA Derivation** (`sol_ata.go`):
- Implement `FindProgramAddress(seeds [][]byte, programID [32]byte) ([32]byte, error)`
  - SHA-256 hash of (seeds + nonce byte + programID + "ProgramDerivedAddress")
  - Try nonce 255→0, return first that is NOT on ed25519 curve
- Implement `DeriveATA(wallet, mint [32]byte) (string, error)`
  - seeds = [wallet, TOKEN_PROGRAM_ID, mint]
  - programID = ASSOCIATED_TOKEN_PROGRAM_ID
  - Return base58-encoded address
- Use `crypto/sha256`, `crypto/ed25519`, `github.com/mr-tron/base58`

**Solana RPC Provider** (`sol_rpc.go`):
- Constructor takes URL + name (reused for public RPC and Helius)
- Native: JSON-RPC `getMultipleAccounts` with base64 encoding
  - MaxBatchSize: 100
  - Read `lamports` from each account in response `value` array
  - null entries → balance "0"
- Token: For each wallet address, derive ATA for the given mint
  - Batch the ATA addresses via `getMultipleAccounts` with `jsonParsed` encoding
  - Parse `parsed.info.tokenAmount.amount` from each account
  - null entries → balance "0"
  - Map ATA results back to original wallet addresses

<verification>
- Unit test: ATA derivation matches known test vectors (USDC ATA for known wallet)
- Unit test: getMultipleAccounts native balance parsing with mock
- Unit test: getMultipleAccounts token balance parsing with mock
- Unit test: null account handling
</verification>

### Task 7: Provider Pool

**Files**: `internal/scanner/pool.go`

Round-robin provider pool with failover:

```go
type Pool struct {
    providers []Provider
    current   atomic.Int32
    mu        sync.Mutex
}

func NewPool(providers ...Provider) *Pool
func (p *Pool) Next() Provider  // round-robin
func (p *Pool) FetchNativeBalances(ctx, addresses) ([]BalanceResult, error)
func (p *Pool) FetchTokenBalances(ctx, addresses, token, contract) ([]BalanceResult, error)
```

- Round-robin: increment `current` atomically, mod by provider count
- On rate limit error: try next provider
- On unavailable error: try next provider
- If all providers fail: return last error
- Log provider rotation at WARN level

<verification>
- Unit test: round-robin distributes evenly
- Unit test: failover on rate limit tries next provider
- Unit test: all providers down returns error
</verification>

### Task 8: Balance DB Operations

**Files**: `internal/db/balances.go`

```go
// UpsertBalance inserts or updates a balance record
func (d *DB) UpsertBalance(chain models.Chain, addressIndex int, token models.Token, balance, lastScanned string) error

// UpsertBalanceBatch inserts or updates multiple balance records in one transaction
func (d *DB) UpsertBalanceBatch(balances []models.Balance) error

// GetFundedAddresses returns addresses with non-zero balance for a chain
func (d *DB) GetFundedAddresses(chain models.Chain, token models.Token) ([]models.Balance, error)

// GetBalanceSummary returns aggregated balance info for a chain
func (d *DB) GetBalanceSummary(chain models.Chain) (nativeTotal string, tokenTotals map[models.Token]string, fundedCount int, err error)
```

- `UpsertBalanceBatch`: Use `INSERT OR REPLACE` in a single transaction for performance
- All operations log at appropriate levels
- Balance stored as string (to preserve precision for large numbers)

<verification>
- Unit test: upsert creates new record
- Unit test: upsert updates existing record
- Unit test: batch upsert handles 100+ records
- Unit test: funded addresses query filters correctly
</verification>

### Task 9: Scan State DB Operations

**Files**: `internal/db/scans.go`

```go
// GetScanState returns the current scan state for a chain
func (d *DB) GetScanState(chain models.Chain) (*models.ScanState, error)

// UpsertScanState updates the scan state for a chain
func (d *DB) UpsertScanState(state models.ScanState) error

// ShouldResume returns true if a scan can be resumed (exists, not too old)
func (d *DB) ShouldResume(chain models.Chain) (bool, int, error)
```

- `ShouldResume`: Check if `updated_at` is within `ScanResumeThreshold` (24h) and status is "scanning" or "completed"
- Status values: "idle", "scanning", "completed", "failed"
- Update `updated_at` on every state change

<verification>
- Unit test: get/upsert round-trip
- Unit test: ShouldResume returns true within 24h
- Unit test: ShouldResume returns false after 24h
</verification>

### Task 10: SSE Hub

**Files**: `internal/scanner/sse.go`

Internal event broadcaster (no HTTP — that's Phase 5):

```go
type Event struct {
    Type string      // "scan_progress", "scan_complete", "scan_error"
    Data interface{} // JSON-serializable payload
}

type SSEHub struct {
    clients    map[chan Event]struct{}
    mu         sync.RWMutex
    register   chan chan Event
    unregister chan chan Event
    broadcast  chan Event
}

func NewSSEHub() *SSEHub
func (h *SSEHub) Run(ctx context.Context)        // goroutine: dispatch events
func (h *SSEHub) Subscribe() chan Event           // register new client
func (h *SSEHub) Unsubscribe(ch chan Event)       // remove client
func (h *SSEHub) Broadcast(event Event)           // send to all clients
```

- Buffered client channels (size 64) to prevent slow clients from blocking
- If client channel is full, drop event (log at WARN)
- Clean shutdown via context cancellation
- Keepalive not needed here (that's the HTTP layer in Phase 5)

<verification>
- Unit test: broadcast reaches all subscribers
- Unit test: unsubscribed client stops receiving
- Unit test: slow client doesn't block others
</verification>

### Task 11: Scanner Orchestrator

**Files**: `internal/scanner/scanner.go`

The main scan coordinator:

```go
type Scanner struct {
    db       *db.DB
    pools    map[models.Chain]*Pool
    hub      *SSEHub
    cfg      *config.Config
}

func New(database *db.DB, cfg *config.Config, hub *SSEHub) *Scanner
func (s *Scanner) RegisterPool(chain models.Chain, pool *Pool)

// StartScan begins scanning a chain up to maxID
func (s *Scanner) StartScan(ctx context.Context, chain models.Chain, maxID int) error

// StopScan cancels a running scan
func (s *Scanner) StopScan(chain models.Chain)

// Status returns current scan status for a chain
func (s *Scanner) Status(chain models.Chain) *models.ScanState
```

**StartScan flow**:
1. Check if scan already running for this chain → error if so
2. Check resume: `db.ShouldResume(chain)` → get `startIndex`
3. Update scan_state to "scanning"
4. Launch goroutine:
   a. For i = startIndex to maxID, step = pool.MaxBatchSize:
      - Load addresses [i:i+batchSize] from DB
      - Call `pool.FetchNativeBalances(ctx, addresses)`
      - Upsert native balances into DB
      - For BSC/SOL: For each token (USDC, USDT):
        - Call `pool.FetchTokenBalances(ctx, addresses, token, contractAddress)`
        - Upsert token balances into DB
      - Update scan_state (last_scanned_index = i + batchSize)
      - Broadcast progress event
      - Check ctx.Done() for cancellation
   b. On completion: update scan_state to "completed", broadcast scan_complete
   c. On error: update scan_state to "failed", broadcast scan_error
   d. On cancellation: update scan_state with current progress (resumable)

- Track active scans with `map[Chain]context.CancelFunc`
- `StopScan` calls the cancel function

<verification>
- Unit test: scan iterates through all addresses
- Unit test: scan resumes from last index
- Unit test: scan cancellation saves progress
- Unit test: scan broadcasts progress events
- Unit test: concurrent scans on different chains work
- Unit test: duplicate scan on same chain returns error
</verification>

### Task 12: Wire Provider Pools

**Files**: `internal/scanner/setup.go`

Factory function to create the full scanner with all providers:

```go
func SetupScanner(database *db.DB, cfg *config.Config, hub *SSEHub) *Scanner {
    scanner := New(database, cfg, hub)

    // BTC providers
    btcPool := NewPool(
        NewBlockstreamProvider(httpClient, btcRL1, cfg.Network),
        NewMempoolProvider(httpClient, btcRL2, cfg.Network),
    )
    scanner.RegisterPool(models.ChainBTC, btcPool)

    // BSC providers
    bscPool := NewPool(
        NewBscScanProvider(httpClient, bscRL1, cfg.BscScanAPIKey, cfg.Network),
        NewBSCRPCProvider(bscRL2, cfg.Network),
    )
    scanner.RegisterPool(models.ChainBSC, bscPool)

    // SOL providers
    solPool := NewPool(
        NewSolanaRPCProvider(httpClient, solRL1, solanaMainnetURL, "SolanaPublicRPC"),
        NewSolanaRPCProvider(httpClient, solRL2, heliusURL, "Helius"),
    )
    scanner.RegisterPool(models.ChainSOL, solPool)

    return scanner
}
```

- Create shared `http.Client` with `ProviderRequestTimeout`
- Create per-provider rate limiters with constants from config
- Select mainnet/testnet URLs based on `cfg.Network`

<verification>
- All providers register without error
- Scanner compiles and runs with test config
</verification>

### Task 13: Tests

**Files**: `internal/scanner/*_test.go`, `internal/db/balances_test.go`, `internal/db/scans_test.go`

Test files to create:
- `internal/scanner/ratelimiter_test.go` — rate limiter timing tests
- `internal/scanner/btc_blockstream_test.go` — mock HTTP tests
- `internal/scanner/btc_mempool_test.go` — mock HTTP tests
- `internal/scanner/bsc_bscscan_test.go` — mock HTTP tests
- `internal/scanner/bsc_rpc_test.go` — mock tests
- `internal/scanner/sol_ata_test.go` — ATA derivation test vectors
- `internal/scanner/sol_rpc_test.go` — mock HTTP tests
- `internal/scanner/pool_test.go` — round-robin + failover tests
- `internal/scanner/sse_test.go` — broadcast/subscribe tests
- `internal/scanner/scanner_test.go` — orchestrator integration test
- `internal/db/balances_test.go` — balance CRUD tests
- `internal/db/scans_test.go` — scan state CRUD tests

Minimum: **20+ tests** across these files.

<verification>
- `go test ./internal/scanner/... ./internal/db/...` all pass
- No data races (`go test -race`)
</verification>

## Success Criteria

- [ ] All 6 providers compile and pass unit tests with mock HTTP
- [ ] Provider pool round-robins correctly and fails over on errors
- [ ] Rate limiter enforces per-provider rate limits
- [ ] Balance upsert/query operations work correctly
- [ ] Scan state persist/resume works within 24h threshold
- [ ] Scanner orchestrator runs end-to-end scan (tested with mocks)
- [ ] SSE hub broadcasts events to subscribers correctly
- [ ] ATA derivation matches known Solana test vectors
- [ ] All tests pass with `go test -race ./internal/...`
- [ ] No hardcoded values — all constants in `config/constants.go`

## Files Created/Modified Summary

| File | Action | Purpose |
|------|--------|---------|
| `go.mod` / `go.sum` | Modified | Add `golang.org/x/time` |
| `internal/config/constants.go` | Modified | Provider URLs, scanner constants, SOL program IDs |
| `internal/config/errors.go` | Modified | Scanner-specific sentinel errors |
| `internal/scanner/provider.go` | Create | Provider interface + types |
| `internal/scanner/ratelimiter.go` | Create | Rate limiter wrapper |
| `internal/scanner/btc_blockstream.go` | Create | Blockstream provider |
| `internal/scanner/btc_mempool.go` | Create | Mempool.space provider |
| `internal/scanner/bsc_bscscan.go` | Create | BscScan provider |
| `internal/scanner/bsc_rpc.go` | Create | BSC ethclient provider |
| `internal/scanner/sol_ata.go` | Create | ATA derivation (manual) |
| `internal/scanner/sol_rpc.go` | Create | Solana RPC provider |
| `internal/scanner/pool.go` | Create | Provider pool |
| `internal/scanner/sse.go` | Create | SSE event hub |
| `internal/scanner/scanner.go` | Replace stub | Scanner orchestrator |
| `internal/scanner/setup.go` | Create | Factory/wiring |
| `internal/db/balances.go` | Replace stub | Balance CRUD |
| `internal/db/scans.go` | Replace stub | Scan state CRUD |
| `internal/scanner/*_test.go` | Create | Provider + scanner tests |
| `internal/db/balances_test.go` | Create | Balance DB tests |
| `internal/db/scans_test.go` | Create | Scan state DB tests |

## Estimated Session Time

4-5 hours (high complexity phase — most files to create)
