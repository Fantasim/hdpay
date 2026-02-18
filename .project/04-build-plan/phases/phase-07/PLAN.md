# Phase 7: BTC Transaction Engine

> **Status: Detailed** — Ready to build.

<objective>
Build the BTC transaction engine: on-demand key derivation, UTXO fetching via Blockstream/Mempool APIs, dynamic fee estimation, multi-input P2WPKH transaction building with witness signing, broadcast to network, and transaction recording in DB. This phase is backend-only — API handlers and UI come in Phase 10.
</objective>

## Dependencies

- Phase 2: `wallet/hd.go` (mnemonic → seed → master key), `wallet/btc.go` (BIP-84 derivation path)
- Phase 4: `scanner/ratelimiter.go` (reusable rate limiter), provider URL constants
- Existing: `models/types.go` (Transaction type), `db/migrations/001_initial.sql` (transactions table schema)

## Architecture Overview

```
internal/tx/
  key_service.go       ← On-demand key derivation (shared, all chains)
  btc_utxo.go          ← UTXO fetching from Blockstream/Mempool
  btc_fee.go           ← Fee estimation from mempool.space
  btc_tx.go            ← TX building, signing, orchestration
  broadcaster.go       ← Shared broadcast interface + BTC implementation

internal/db/
  transactions.go      ← Transaction CRUD (currently empty stub)

internal/config/
  constants.go         ← New BTC TX constants
  errors.go            ← New BTC TX error codes

internal/models/
  types.go             ← New UTXO, FeeEstimate, SendPreview types
```

---

<tasks>

## Task 1: Constants, Errors, and Types

### 1a. Add constants to `internal/config/constants.go`

Add new constant blocks:

```go
// BTC Transaction Building
const (
    BTCDustThresholdSats    = 546     // Minimum output value (P2WPKH dust limit)
    BTCMaxTxWeight          = 400_000 // Max standard tx weight (100K vbytes)
    BTCMaxInputsPerTx       = 500     // Practical limit before hitting size
    BTCDefaultFeeRate       = 10      // Fallback sat/vB if fee estimation fails
    BTCMinFeeRate           = 1       // Absolute minimum sat/vB
)

// BTC Transaction vsize estimation (weight units)
const (
    BTCTxOverheadWU       = 42  // version(16) + marker(1) + flag(1) + vinCount(4) + voutCount(4) + locktime(16)
    BTCP2WPKHInputNonWitWU = 164 // outpoint(36) + scriptLen(1) + sequence(4) = 41 bytes × 4
    BTCP2WPKHInputWitWU    = 108 // stackCount(1) + sigLen(1) + sig(72) + pkLen(1) + pk(33) = 108 × 1
    BTCP2WPKHOutputWU      = 124 // value(8) + scriptLen(1) + script(22) = 31 bytes × 4
)

// BTC Fee Estimation
const (
    MempoolFeeEstimateURL = "/v1/fees/recommended"  // Appended to Mempool base URL
    FeeEstimateTimeout    = 5 * time.Second
)
```

### 1b. Add errors to `internal/config/errors.go`

```go
// New sentinel errors
var (
    ErrUTXOFetchFailed     = errors.New("UTXO fetch failed")
    ErrFeeEstimateFailed   = errors.New("fee estimation failed")
    ErrInsufficientUTXO    = errors.New("insufficient UTXO value to cover fee")
    ErrTxTooLarge          = errors.New("transaction exceeds maximum weight")
    ErrDustOutput          = errors.New("output below dust threshold")
    ErrMnemonicFileNotSet  = errors.New("mnemonic file path not configured")
    ErrKeyDerivation       = errors.New("key derivation failed")
)

// New error codes
const (
    ErrorUTXOFetchFailed    = "ERROR_UTXO_FETCH_FAILED"
    ErrorFeeEstimateFailed  = "ERROR_FEE_ESTIMATE_FAILED"
    ErrorInsufficientUTXO   = "ERROR_INSUFFICIENT_UTXO"
    ErrorTxTooLarge         = "ERROR_TX_TOO_LARGE"
)
```

### 1c. Add new types to `internal/models/types.go`

```go
// UTXO represents an unspent transaction output.
type UTXO struct {
    TxID         string `json:"txid"`
    Vout         uint32 `json:"vout"`
    Value        int64  `json:"value"` // satoshis
    Confirmed    bool   `json:"confirmed"`
    BlockHeight  int64  `json:"blockHeight,omitempty"`
    Address      string `json:"address"`
    AddressIndex int    `json:"addressIndex"`
}

// FeeEstimate contains recommended fee rates from mempool.space.
type FeeEstimate struct {
    FastestFee  int64 `json:"fastestFee"`  // sat/vB, next block
    HalfHourFee int64 `json:"halfHourFee"` // sat/vB, ~3 blocks
    HourFee     int64 `json:"hourFee"`     // sat/vB, ~6 blocks
    EconomyFee  int64 `json:"economyFee"`  // sat/vB, several hours
    MinimumFee  int64 `json:"minimumFee"`  // sat/vB, eventually
}

// SendPreview contains the preview of a consolidation transaction.
type SendPreview struct {
    Chain           Chain  `json:"chain"`
    InputCount      int    `json:"inputCount"`
    TotalInputSats  int64  `json:"totalInputSats"`
    OutputSats      int64  `json:"outputSats"`
    FeeSats         int64  `json:"feeSats"`
    FeeRate         int64  `json:"feeRate"`
    EstimatedVsize  int    `json:"estimatedVsize"`
    DestAddress     string `json:"destAddress"`
}

// SendResult contains the result of broadcasting a transaction.
type SendResult struct {
    TxHash string `json:"txHash"`
    Chain  Chain  `json:"chain"`
}
```

<verification>
- `go build ./internal/config/...` compiles
- `go build ./internal/models/...` compiles
- No duplicate constant names
</verification>

---

## Task 2: On-Demand Key Derivation Service (`internal/tx/key_service.go`)

Shared service that reads the mnemonic file, derives private keys on demand, and returns them for signing. The caller is responsible for zeroing the key after use.

**Functions:**
- `NewKeyService(mnemonicFilePath string, network string) *KeyService` — constructor
- `DeriveBTCPrivateKey(ctx context.Context, index uint32) (*btcec.PrivateKey, error)` — derives BIP-84 key at index
- Internal: reads mnemonic file, derives master key, walks BIP-84 path to get `ECPrivKey()`

**Security:**
- Mnemonic is read from file each time `DeriveXxx` is called (no caching in memory)
- Private key is returned to caller who must zero it after signing
- Never log the mnemonic or private key bytes
- Log only: derivation index, chain, success/failure

**Implementation notes:**
- Reuse `wallet.ReadMnemonicFromFile()`, `wallet.MnemonicToSeed()`, `wallet.DeriveMasterKey()`
- For BTC: walk the same BIP-84 path as `wallet/btc.go` but call `child.ECPrivKey()` instead of `ECPubKey()`
- Use `wallet.NetworkParams()` to get chaincfg.Params
- Pass context for cancellation support

<verification>
- Unit test: derive key at index 0 from known mnemonic, verify corresponding public key matches known address
- Test: invalid mnemonic file path returns error
- Test: context cancellation is respected
</verification>

---

## Task 3: UTXO Fetching (`internal/tx/btc_utxo.go`)

Fetch UTXOs for funded BTC addresses using Blockstream/Mempool APIs (same providers as scanner).

**API endpoints:**
- `GET {baseURL}/address/{address}/utxo` — returns JSON array of UTXOs
- Response: `[{"txid":"...","vout":0,"status":{"confirmed":true,"block_height":N},"value":12345}]`

**Functions:**
- `NewBTCUTXOFetcher(httpClient *http.Client, providerURLs []string, rateLimiter ...) *BTCUTXOFetcher`
- `FetchUTXOs(ctx context.Context, address string, addressIndex int) ([]models.UTXO, error)` — fetch UTXOs for one address
- `FetchAllUTXOs(ctx context.Context, addresses []models.Address) ([]models.UTXO, error)` — fetch for multiple addresses, round-robin across providers, respecting rate limits

**Implementation notes:**
- Round-robin provider rotation (reuse pattern from scanner)
- Reuse `scanner.RateLimiter` for per-provider rate limiting
- Only return confirmed UTXOs (filter `status.confirmed == true`)
- Reconstruct pkScript from address via `btcutil.DecodeAddress` + `txscript.PayToAddrScript` (API doesn't return scriptPubKey)
- Skip addresses with zero or dust-only UTXOs
- Log: provider used, address count, UTXO count found, total value

<verification>
- Test with mock HTTP server returning sample UTXO JSON
- Test round-robin rotation across providers
- Test filtering of unconfirmed UTXOs
- Test empty UTXO response
- Test HTTP error / rate limit handling
</verification>

---

## Task 4: Fee Estimation (`internal/tx/btc_fee.go`)

Dynamic fee rate from mempool.space, with fallback chain.

**API endpoint:**
- `GET {mempoolBaseURL}/v1/fees/recommended`
- Response: `{"fastestFee":15,"halfHourFee":12,"hourFee":9,"economyFee":6,"minimumFee":1}`

**Functions:**
- `NewBTCFeeEstimator(httpClient *http.Client, mempoolURL string) *BTCFeeEstimator`
- `EstimateFee(ctx context.Context) (*models.FeeEstimate, error)` — fetch all tiers
- `DefaultFeeRate(estimate *models.FeeEstimate) int64` — returns `HalfHourFee` (medium priority)

**Fallback chain:**
1. mempool.space `/v1/fees/recommended`
2. If fails: use `config.BTCDefaultFeeRate` constant (10 sat/vB)
3. Always enforce floor of `config.BTCMinFeeRate` (1 sat/vB)

**Implementation notes:**
- Short timeout (5s) — fee estimation shouldn't block long
- Log: estimated fee rates, which source was used (API vs fallback)

<verification>
- Test with mock HTTP returning valid fee JSON
- Test fallback to constant when API fails
- Test minimum fee rate enforcement
</verification>

---

## Task 5: BTC Transaction Builder (`internal/tx/btc_tx.go`)

Core transaction building, signing, and orchestration.

**Functions:**

### vsize estimation
- `EstimateBTCVsize(numInputs, numOutputs int) int` — pure calculation
  - Formula: `ceil((42 + numInputs*(164+108) + numOutputs*124) / 4)`

### Transaction building
- `BuildBTCConsolidationTx(params BTCBuildParams) (*BTCBuiltTx, error)`
  - `BTCBuildParams`: UTXOs, destAddress, feeRate, network params
  - Creates `wire.MsgTx` with all UTXO inputs, single output to dest
  - Calculates fee from estimated vsize × feeRate
  - Validates: total input > fee + dust, output > dust threshold, weight < max
  - Returns unsigned tx + metadata

### Signing
- `SignBTCTx(tx *wire.MsgTx, utxos []SigningUTXO) error`
  - `SigningUTXO`: UTXO + pkScript + privKey
  - Uses `txscript.NewMultiPrevOutFetcher` (NOT CannedPrevOutputFetcher — critical for multi-input)
  - Calls `txscript.NewTxSigHashes` ONCE for all inputs
  - Calls `txscript.WitnessSignature` per input
  - Sets `tx.TxIn[i].Witness` — leaves `SignatureScript` nil (native SegWit)
  - Zeros private keys after signing each input

### Serialization
- `SerializeBTCTx(tx *wire.MsgTx) (string, error)` — serialize to hex string

### Orchestration
- `BTCConsolidationService` struct — ties together KeyService, UTXOFetcher, FeeEstimator, Broadcaster, DB
- `Preview(ctx, addresses, destAddr, feeRate) (*SendPreview, error)` — dry run without signing
- `Execute(ctx, addresses, destAddr, feeRate) (*SendResult, error)` — full flow: fetch UTXOs → estimate fee → build tx → derive keys → sign → broadcast → record in DB

**Key implementation details:**
- `MultiPrevOutFetcher` maps each input's outpoint to its previous TxOut (amount + pkScript)
- pkScript reconstructed from address: `btcutil.DecodeAddress` → `txscript.PayToAddrScript`
- For consolidation: always 1 output (no change), all value minus fee goes to dest
- Private keys derived on-demand per input, zeroed immediately after `WitnessSignature`
- If total estimated vsize > MaxTxWeight/4, split into multiple transactions (return error for now, splitting logic can be added later)

<verification>
- Test vsize estimation against known values (1-in-1-out = 110 vbytes, 10-in-1-out = 722 vbytes)
- Test TX building with mock UTXOs — verify wire.MsgTx has correct inputs/outputs
- Test signing with known private key — verify witness is set on each input
- Test serialization produces valid hex
- Test insufficient UTXOs returns appropriate error
- Test dust threshold rejection
</verification>

---

## Task 6: Broadcaster (`internal/tx/broadcaster.go`)

Broadcast raw transactions to the BTC network. This file will also define a shared interface for future BSC/SOL use.

**API endpoints:**
- `POST {baseURL}/tx` — body is raw hex as `text/plain`
- Success: HTTP 200, body = txid (plain text)
- Error: HTTP 400, body = error message

**Interface:**
```go
type Broadcaster interface {
    Broadcast(ctx context.Context, rawHex string) (txHash string, err error)
}
```

**BTC implementation:**
- `NewBTCBroadcaster(httpClient *http.Client, providerURLs []string) *BTCBroadcaster`
- `Broadcast(ctx, rawHex)` — try first provider, fallback to next on failure
- Content-Type: `text/plain` (NOT application/json)
- Return trimmed txid string on success

**Implementation notes:**
- Try providers in order (not round-robin — broadcast should succeed on first working provider)
- Log: provider attempted, success/failure, txid on success
- Don't retry on 400 (bad tx) — only retry on network/5xx errors

<verification>
- Test with mock HTTP server returning txid
- Test fallback to second provider on first failure
- Test 400 response (bad tx) doesn't retry
</verification>

---

## Task 7: Transaction DB CRUD (`internal/db/transactions.go`)

Implement the empty `internal/db/transactions.go` with operations matching the existing `transactions` table schema.

**Functions:**
- `InsertTransaction(ctx, tx models.Transaction) (int64, error)` — insert, return ID
- `UpdateTransactionStatus(ctx, id int64, status string, confirmedAt *string) error`
- `GetTransaction(ctx, id int64) (*models.Transaction, error)`
- `GetTransactionByHash(ctx, chain models.Chain, txHash string) (*models.Transaction, error)`
- `ListTransactions(ctx, chain *models.Chain, page, pageSize int) ([]models.Transaction, int64, error)` — paginated, optional chain filter

**Implementation notes:**
- Follow patterns from `db/addresses.go` and `db/balances.go`
- Use prepared statements
- Direction: "send" for consolidation (outbound from our wallet)
- Status lifecycle: "pending" → "confirmed" or "failed"

<verification>
- Test insert + retrieve
- Test status update
- Test list with chain filter
- Test list pagination
- Test get by hash
</verification>

---

## Task 8: Tests

Create comprehensive test files:

### `internal/tx/key_service_test.go`
- Derive BTC private key at index 0 from test mnemonic → verify pubkey matches expected address
- Invalid mnemonic file → error
- Write temp mnemonic file for test isolation

### `internal/tx/btc_utxo_test.go`
- Mock HTTP server returning UTXO JSON → parse correctly
- Empty UTXO list → no error, empty slice
- HTTP error → appropriate error
- Unconfirmed UTXOs filtered out

### `internal/tx/btc_fee_test.go`
- Mock HTTP returning fee JSON → correct parsing
- API failure → falls back to default constant
- Minimum fee rate enforced

### `internal/tx/btc_tx_test.go`
- vsize estimation: known input/output counts → expected vsize values
- Build consolidation TX: 3 UTXOs → 1 output, verify amounts and structure
- Sign TX: verify witnesses are set on all inputs
- Dust output → rejected
- Insufficient value → rejected

### `internal/tx/broadcaster_test.go`
- Mock broadcast endpoint → returns txid
- First provider fails, second succeeds → fallback works
- Bad tx (400) → no retry, returns error

### `internal/db/transactions_test.go`
- Insert → retrieve by ID
- Update status → verify change
- List with pagination
- List filtered by chain

</tasks>

---

<success_criteria>
1. `go build ./internal/tx/...` and `go build ./internal/db/...` compile without errors
2. `go test ./internal/tx/... -v` — all tests pass
3. `go test ./internal/db/... -v` — all tests pass (including new transaction tests)
4. `go vet ./internal/tx/...` — no issues
5. No hardcoded values — all constants in `config/constants.go`
6. All errors defined in `config/errors.go`
7. Private keys are never logged and are zeroed after use in signing code
8. Mnemonic file is read on-demand, never cached in service struct
9. UTXO fetcher respects rate limits and does round-robin
10. Fee estimator has working fallback chain
11. Transaction building uses `MultiPrevOutFetcher` (not `CannedPrevOutputFetcher`)
12. Broadcaster sends `text/plain` Content-Type
13. All new functions have slog logging at appropriate levels
</success_criteria>

<files_to_read_before_building>
- `internal/wallet/hd.go` — mnemonic/seed/master key functions to reuse
- `internal/wallet/btc.go` — BIP-84 derivation path to mirror for private key
- `internal/scanner/btc_blockstream.go` — HTTP client pattern, provider structure
- `internal/scanner/ratelimiter.go` — rate limiter to reuse
- `internal/db/addresses.go` — DB access patterns
- `internal/db/balances.go` — batch operation patterns
- `internal/config/constants.go` — existing constants to extend
- `internal/config/errors.go` — existing errors to extend
- `internal/models/types.go` — existing types to extend
- `cmd/server/main.go` — see how services are wired (for future Phase 10 integration)
</files_to_read_before_building>
