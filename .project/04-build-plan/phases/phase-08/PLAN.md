# Phase 8: BSC Transaction Engine + Gas Pre-Seed

> **Status: Detailed** — Ready to build.

<objective>
Build the BSC transaction engine for native BNB transfers and BEP-20 token transfers (USDC/USDT), with EIP-155 signing, receipt polling, sequential nonce management, and a gas pre-seeding system that distributes BNB to token-holding addresses that lack gas.
</objective>

## Architecture Overview

Unlike BTC (multi-input UTXO consolidation in one TX), BSC uses **sequential per-address transactions**:
- **Native BNB sweep**: For each funded address → build LegacyTx, sign with EIP-155, broadcast, wait receipt
- **BEP-20 sweep**: For each address with tokens → encode `transfer(address,uint256)`, sign, broadcast, wait receipt
- **Gas pre-seed**: From a gas-source address → send 0.005 BNB to each address needing gas

The ethclient from go-ethereum (already v1.17.0 in go.mod) provides all RPC methods. BSC uses chain ID 56 (mainnet) / 97 (testnet).

---

<tasks>

## Task 1: Extend KeyService with DeriveBSCPrivateKey

**File**: `internal/tx/key_service.go`

Add a `DeriveBSCPrivateKey` method that derives an ECDSA private key at BIP-44 path `m/44'/60'/0'/0/N`. Reuses the existing `deriveMasterKey()` method.

Implementation:
- Derive path: purpose(44') → coin(60') → account(0') → change(0) → index(N)
- Use `child.ECPrivKey()` then `.ToECDSA()` to convert btcec → stdlib ecdsa
- Also return the `common.Address` derived from the public key (for verification)
- BSC uses the same coin type (60) for both mainnet and testnet — no network switch needed

Signature:
```go
func (ks *KeyService) DeriveBSCPrivateKey(ctx context.Context, index uint32) (*ecdsa.PrivateKey, common.Address, error)
```

**Verification**: Add tests in `key_service_test.go` — derive index 0 and confirm address matches `0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb` (known test vector from abandon...art mnemonic).

---

## Task 2: BSC Transaction Building — Native BNB Transfer

**File**: `internal/tx/bsc_tx.go` (new)

Create the core BSC transaction building functions:

### 2a: EthClientWrapper interface
Define a minimal interface over ethclient for testability:
```go
type EthClientWrapper interface {
    PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
    SuggestGasPrice(ctx context.Context) (*big.Int, error)
    SendTransaction(ctx context.Context, tx *types.Transaction) error
    TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
    BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
}
```

### 2b: BuildBSCNativeTransfer
Build a native BNB transfer from one address to a destination:
- Get nonce via `PendingNonceAt` (or accept pre-computed nonce for batch)
- Get gas price via `SuggestGasPrice` (with 20% buffer for BSC reliability)
- Calculate `sendAmount = balance - (gasLimit * gasPrice)` where gasLimit = 21,000
- Build `types.NewTx(&types.LegacyTx{...})`
- Return the unsigned tx + metadata

### 2c: SignBSCTx
Sign a BSC transaction with EIP-155:
- `chainID` from constants: `big.NewInt(56)` mainnet / `big.NewInt(97)` testnet
- `types.NewEIP155Signer(chainID)`
- `types.SignTx(tx, signer, privateKey)`

### 2d: WaitForReceipt
Poll `TransactionReceipt` with backoff until mined or context cancelled:
- Check for `ethereum.NotFound` (pending) → wait 3s → retry
- On success: check `receipt.Status` (1 = success, 0 = reverted)
- Timeout via context

**Verification**: Unit tests with mock EthClientWrapper.

---

## Task 3: BEP-20 Token Transfer

**File**: `internal/tx/bsc_tx.go` (same file)

### 3a: EncodeBEP20Transfer
Manual ABI encoding for `transfer(address,uint256)`:
- Function selector: `0xa9059cbb` (first 4 bytes of keccak256("transfer(address,uint256)"))
- Pad recipient address to 32 bytes with `common.LeftPadBytes`
- Pad amount to 32 bytes with `common.LeftPadBytes`
- Result: 68-byte calldata

### 3b: BuildBSCTokenTransfer
Build a BEP-20 token transfer:
- `To` = token contract address (not recipient)
- `Value` = 0 (not sending BNB)
- `Data` = encoded transfer calldata
- `Gas` = 65,000 (BSCGasLimitBEP20 from constants)
- Same nonce/gasPrice/signing flow as native

**Verification**: Test that encoded calldata matches known ABI encoding.

---

## Task 4: BSC Consolidation Service

**File**: `internal/tx/bsc_tx.go` (same file)

### 4a: BSCConsolidationService struct
Orchestrates the full BSC sweep flow. Dependencies:
- `keyService *KeyService`
- `ethClient EthClientWrapper`
- `database *db.DB`
- `chainID *big.Int`

### 4b: Preview method
Dry-run that calculates what would happen:
- For native BNB: sum all balances, subtract gas costs, show net output
- For BEP-20: sum all token balances, show gas cost needed
- Return a `BSCSendPreview` struct with per-address breakdown

### 4c: ExecuteNativeSweep
Sequential BNB consolidation:
1. Get gas price once (shared for all txs)
2. For each funded address:
   a. Derive private key
   b. Get balance from ethclient
   c. Calculate sendAmount = balance - gasCost
   d. Build + sign + broadcast TX
   e. Wait for receipt
   f. Record in DB
   g. Zero private key
3. Return results (success count, total swept, any failures)

### 4d: ExecuteTokenSweep
Sequential BEP-20 consolidation:
1. Verify all source addresses have BNB for gas (or fail with list of addresses needing gas)
2. Get gas price once
3. For each address with token balance:
   a. Derive private key
   b. Build BEP-20 transfer calldata
   c. Build + sign + broadcast TX
   d. Wait for receipt (check receipt.Status for reverts)
   e. Record in DB
   f. Zero private key
4. Return results

**Verification**: Integration-style tests with mock ethclient verifying the full flow.

---

## Task 5: Gas Pre-Seeding

**File**: `internal/tx/gas.go` (new)

### 5a: GasPreSeedService
Handles distributing BNB from a source address to multiple target addresses:

```go
type GasPreSeedService struct {
    keyService *KeyService
    ethClient  EthClientWrapper
    database   *db.DB
    chainID    *big.Int
}
```

### 5b: PreviewGasPreSeed
Calculate what's needed:
- Input: list of target addresses (those with tokens but 0 BNB), source address index
- Get source address balance
- Calculate: `totalNeeded = len(targets) * BSCGasPreSeedWei`
- Validate source has enough BNB (totalNeeded + gas for all sends)
- Return preview with per-target breakdown

### 5c: ExecuteGasPreSeed
Execute the pre-seeding:
1. Derive source private key
2. Get initial nonce from `PendingNonceAt`
3. Get gas price once
4. For each target address:
   a. Build native BNB transfer (value = 0.005 BNB)
   b. Sign with incremented nonce (nonce++ after each)
   c. Broadcast
   d. Record in DB as "gas-preseed" direction
5. Wait for all receipts (can batch-poll since we have all tx hashes)
6. Zero source private key
7. Return results

Key detail: nonce is managed locally — get once with `PendingNonceAt`, then increment for each subsequent TX. Do NOT re-query between sends.

**Verification**: Tests verifying nonce sequencing, insufficient balance rejection, correct amounts.

---

## Task 6: New Constants and Error Types

**File**: `internal/config/constants.go` — Add:
```go
// BSC Chain IDs
BSCMainnetChainID = 56
BSCTestnetChainID = 97

// BSC Transaction
BSCGasPriceBufferPercent = 20    // Add 20% buffer to suggested gas price
BSCReceiptPollInterval   = 3 * time.Second
BSCReceiptPollTimeout    = 120 * time.Second
BSCGasPreSeedAmount      = "5000000000000000" // 0.005 BNB in wei

// BEP-20 Transfer
BEP20TransferMethodID = "a9059cbb"  // keccak256("transfer(address,uint256)")[:4]
```

**File**: `internal/config/errors.go` — Add:
```go
// Sentinel errors
ErrNonceTooLow          = errors.New("nonce too low")
ErrTxReverted           = errors.New("transaction reverted")
ErrInsufficientBNBForGas = errors.New("insufficient BNB for gas")
ErrGasPreSeedFailed     = errors.New("gas pre-seed failed")
ErrReceiptTimeout       = errors.New("receipt polling timeout")

// Error codes
ErrorNonceTooLow         = "ERROR_NONCE_TOO_LOW"
ErrorTxReverted          = "ERROR_TX_REVERTED"
ErrorReceiptTimeout      = "ERROR_RECEIPT_TIMEOUT"
ErrorGasPreSeedFailed    = "ERROR_GAS_PRESEED_FAILED"
```

**File**: `internal/models/types.go` — Add:
```go
type BSCSendPreview struct {
    Chain       Chain   `json:"chain"`
    Token       Token   `json:"token"`
    InputCount  int     `json:"inputCount"`
    TotalAmount string  `json:"totalAmount"` // wei or token smallest unit
    GasCostWei  string  `json:"gasCostWei"`
    NetAmount   string  `json:"netAmount"`   // totalAmount - gasCost (native only)
    DestAddress string  `json:"destAddress"`
    GasPrice    string  `json:"gasPrice"`    // wei, for transparency
}

type BSCSendResult struct {
    Chain        Chain           `json:"chain"`
    Token        Token           `json:"token"`
    TxResults    []BSCTxResult   `json:"txResults"`
    SuccessCount int             `json:"successCount"`
    FailCount    int             `json:"failCount"`
    TotalSwept   string          `json:"totalSwept"`
}

type BSCTxResult struct {
    AddressIndex int    `json:"addressIndex"`
    FromAddress  string `json:"fromAddress"`
    TxHash       string `json:"txHash"`
    Amount       string `json:"amount"`
    Status       string `json:"status"` // "confirmed", "reverted", "failed"
    Error        string `json:"error,omitempty"`
}

type GasPreSeedPreview struct {
    SourceIndex   int      `json:"sourceIndex"`
    SourceAddress string   `json:"sourceAddress"`
    SourceBalance string   `json:"sourceBalance"` // wei
    TargetCount   int      `json:"targetCount"`
    AmountPerTarget string `json:"amountPerTarget"` // wei
    TotalNeeded   string   `json:"totalNeeded"`     // wei (amount + gas)
    Sufficient    bool     `json:"sufficient"`
}

type GasPreSeedResult struct {
    TxResults    []BSCTxResult `json:"txResults"`
    SuccessCount int           `json:"successCount"`
    FailCount    int           `json:"failCount"`
    TotalSent    string        `json:"totalSent"` // wei
}
```

**Verification**: Compiles cleanly.

---

## Task 7: Comprehensive Tests

**File**: `internal/tx/bsc_tx_test.go` (new)

Tests:
1. `TestEncodeBEP20Transfer` — verify calldata encoding matches known ABI output
2. `TestBuildBSCNativeTransfer` — correct nonce, gasPrice, value, gasLimit
3. `TestBuildBSCTokenTransfer` — correct calldata, to=contract, value=0
4. `TestSignBSCTx` — EIP-155 signing produces valid signature with correct chain ID
5. `TestWaitForReceipt_Success` — mock receipt returned, status checked
6. `TestWaitForReceipt_Reverted` — mock receipt with status 0 → ErrTxReverted
7. `TestWaitForReceipt_Timeout` — context deadline exceeded
8. `TestBSCConsolidation_NativeSweep` — full flow with mock ethclient
9. `TestBSCConsolidation_TokenSweep` — full flow including gas check
10. `TestBSCConsolidation_InsufficientGas` — token sweep fails if no BNB

**File**: `internal/tx/gas_test.go` (new)

Tests:
1. `TestGasPreSeed_Preview` — correct calculation of amounts needed
2. `TestGasPreSeed_Execute` — nonce sequencing, correct amounts sent
3. `TestGasPreSeed_InsufficientSource` — rejects if source can't cover all targets
4. `TestGasPreSeed_NonceIncrement` — verifies nonce increments correctly per TX

**File**: `internal/tx/key_service_test.go` (extend)

Tests:
1. `TestDeriveBSCPrivateKey` — known vector: index 0 → address matches `0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb`
2. `TestDeriveBSCPrivateKey_MultipleIndices` — indices 0-2 produce distinct keys

</tasks>

---

<success_criteria>
1. `DeriveBSCPrivateKey` returns correct ECDSA key + address for known test vector
2. Native BNB transfers build correctly with EIP-155 signing (chain ID in signature)
3. BEP-20 transfer calldata encodes correctly (68 bytes: 4 selector + 32 address + 32 amount)
4. Receipt polling handles success, revert, and timeout cases
5. Gas pre-seeding correctly sequences nonces and distributes exact amounts
6. All new sentinel errors and constants are in central config files
7. All tests pass: `go test ./internal/tx/... ./internal/config/...`
8. No private keys logged or stored — derived on-demand, used, zeroed/discarded
</success_criteria>

<verification>
```bash
cd /home/louis/dev/hdpay && /usr/local/go/bin/go test ./internal/tx/... -v -count=1
cd /home/louis/dev/hdpay && /usr/local/go/bin/go test ./internal/config/... -v -count=1
cd /home/louis/dev/hdpay && /usr/local/go/bin/go vet ./internal/tx/... ./internal/config/... ./internal/models/...
```
</verification>

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/tx/key_service.go` | Modify | Add `DeriveBSCPrivateKey` method |
| `internal/tx/bsc_tx.go` | Create | EthClientWrapper, native/token TX building, signing, receipt, consolidation service |
| `internal/tx/gas.go` | Create | Gas pre-seeding service (preview + execute) |
| `internal/config/constants.go` | Modify | BSC chain IDs, gas price buffer, receipt polling, BEP-20 selector |
| `internal/config/errors.go` | Modify | New BSC-specific sentinel errors and error codes |
| `internal/models/types.go` | Modify | BSCSendPreview, BSCSendResult, BSCTxResult, GasPreSeed types |
| `internal/tx/bsc_tx_test.go` | Create | 10 tests for BSC TX engine |
| `internal/tx/gas_test.go` | Create | 4 tests for gas pre-seeding |
| `internal/tx/key_service_test.go` | Modify | 2 new tests for BSC key derivation |

## Edge Cases Handled

- **Nonce conflict**: Use `PendingNonceAt` once, then increment locally per TX
- **Gas price spike**: 20% buffer on `SuggestGasPrice`
- **Insufficient BNB for token sweep**: Pre-check balances, return explicit error listing addresses needing gas
- **Contract revert**: Check `receipt.Status == 0` → `ErrTxReverted`
- **Receipt timeout**: Context-based timeout (120s default) → `ErrReceiptTimeout`
- **Source can't cover all pre-seeds**: Validate upfront before any sends
