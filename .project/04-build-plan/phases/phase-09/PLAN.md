# Phase 9: SOL Transaction Engine

> **Status: Detailed** — Ready to build.

<objective>
Build the SOL transaction engine for native SOL multi-instruction batch sweeps and SPL token transfers, with raw Solana transaction serialization (compact-u16, message format, multi-signer), blockhash management, confirmation polling, and ATA existence checking — all without external Solana SDK dependencies.
</objective>

## Architecture Overview

Unlike BSC (sequential per-address EVM transactions), Solana supports **multi-instruction transactions** where multiple transfers are bundled into one TX. However, for consolidation (sweeping funds FROM many addresses TO one), each source address must sign the transaction, creating size pressure:

- **Solana TX hard limit**: 1232 bytes total
- **Each signature**: 64 bytes, **each pubkey**: 32 bytes
- **Practical multi-signer batch**: ~7 source addresses per TX (not 20)

Two sweep modes:
- **Native SOL**: Multi-instruction batch — up to 7 SystemProgram.Transfer instructions per TX, all source addresses sign, destination is fee payer
- **SPL Token**: One TX per source address — each address signs its own SPL Token.Transfer from its ATA to the destination ATA (matches BSC's sequential pattern)

The entire engine uses only `crypto/ed25519`, `encoding/binary`, `mr-tron/base58`, and `net/http` — zero Solana SDK dependencies.

---

<tasks>

## Task 1: SOL Transaction Serialization Primitives

**File**: `internal/tx/sol_serialize.go` (new)

Build the low-level Solana binary serialization layer from scratch. This is the foundation for all SOL transactions.

### 1a: Core Types

```go
type SolPublicKey [32]byte
type SolSignature [64]byte

type SolAccountMeta struct {
    PubKey     SolPublicKey
    IsSigner   bool
    IsWritable bool
}

type SolInstruction struct {
    ProgramID SolPublicKey
    Accounts  []SolAccountMeta
    Data      []byte
}

type SolCompiledInstruction struct {
    ProgramIDIndex uint8
    AccountIndexes []uint8
    Data           []byte
}

type SolMessageHeader struct {
    NumRequiredSignatures       uint8
    NumReadonlySignedAccounts   uint8
    NumReadonlyUnsignedAccounts uint8
}

type SolMessage struct {
    Header          SolMessageHeader
    AccountKeys     []SolPublicKey
    RecentBlockhash [32]byte
    Instructions    []SolCompiledInstruction
}

type SolTransaction struct {
    Signatures []SolSignature
    Message    SolMessage
}
```

### 1b: Compact-u16 Encoding

Solana's custom variable-length integer encoding used for all array lengths:
- `val < 128` → 1 byte
- `128 <= val < 16384` → 2 bytes (continuation bit)
- `16384 <= val < 65536` → 3 bytes

```go
func EncodeCompactU16(buf *bytes.Buffer, val int) error
```

### 1c: SolPublicKeyFromBase58 / SolPublicKeyToBase58

Convert between base58 address strings and `SolPublicKey` type.

### 1d: CompileMessage

Takes high-level `[]SolInstruction` and compiles into a `SolMessage`:
1. Collect all unique accounts across all instructions (deduplicate)
2. Sort accounts by privilege: writable+signer → readonly+signer → writable+notsigner → readonly+notsigner
3. Fee payer is always account index 0 (writable + signer)
4. Count header fields (numRequiredSignatures, numReadonlySigned, numReadonlyUnsigned)
5. Replace pubkeys in instructions with their index into the account keys array → `SolCompiledInstruction`

```go
func CompileMessage(feePayer SolPublicKey, instructions []SolInstruction, recentBlockhash [32]byte) (SolMessage, error)
```

### 1e: SerializeMessage / SerializeTransaction

```go
func SerializeMessage(msg SolMessage) ([]byte, error)    // what gets signed
func SerializeTransaction(tx SolTransaction) ([]byte, error)  // full wire format
```

SerializeMessage layout: `[3-byte header][compact-u16 + account keys][32-byte blockhash][compact-u16 + compiled instructions]`

SerializeTransaction layout: `[compact-u16 + signatures][serialized message]`

### 1f: SignTransaction

Sign a serialized message with multiple private keys, producing signatures in account order:

```go
func SignTransaction(msg SolMessage, msgBytes []byte, signers map[SolPublicKey]ed25519.PrivateKey) (SolTransaction, error)
```

The `signers` map keys on public key. For each of the first `numRequiredSignatures` account keys, look up the corresponding private key and produce an ed25519 signature. Return error if any required signer is missing.

**Verification**: Test compact-u16 encoding for values 0, 1, 127, 128, 255, 16383, 16384. Test CompileMessage account ordering. Test round-trip serialize → check size.

---

## Task 2: System Program and SPL Token Instructions

**File**: `internal/tx/sol_serialize.go` (same file)

### 2a: Well-Known Program IDs

Parse from constants (already defined in `config.constants.go`):
- System Program: `11111111111111111111111111111111`
- SPL Token Program: `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`
- Associated Token Program: `ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL`
- Rent Sysvar: `SysvarRent111111111111111111111111111111111`

### 2b: BuildSystemTransferInstruction

```go
func BuildSystemTransferInstruction(from, to SolPublicKey, lamports uint64) SolInstruction
```

Data format (12 bytes): `[u32 LE: 2 (Transfer variant)][u64 LE: lamports]`
Accounts: `[from: writable+signer, to: writable]`

### 2c: BuildSPLTransferInstruction

```go
func BuildSPLTransferInstruction(sourceATA, destATA, owner SolPublicKey, amount uint64) SolInstruction
```

Data format (9 bytes): `[u8: 3 (Transfer variant)][u64 LE: amount]`
Accounts: `[sourceATA: writable, destATA: writable, owner: signer]`

### 2d: BuildCreateATAInstruction

```go
func BuildCreateATAInstruction(payer, ata, wallet, mint SolPublicKey) SolInstruction
```

Data: empty (0 bytes) — the ATA address is derived on-chain.
Accounts (7): `[payer: writable+signer, ata: writable, wallet: readonly, mint: readonly, systemProgram: readonly, tokenProgram: readonly, rentSysvar: readonly]`

**Verification**: Test instruction data encoding matches known byte patterns. Test SystemTransfer = 12 bytes, SPLTransfer = 9 bytes, CreateATA = 0 bytes.

---

## Task 3: SOL RPC Client for Transactions

**File**: `internal/tx/sol_tx.go` (new)

### 3a: SOLRPCClient interface

Minimal interface for testability (mirrors the EthClientWrapper pattern from BSC):

```go
type SOLRPCClient interface {
    GetLatestBlockhash(ctx context.Context) (blockhash [32]byte, lastValidBlockHeight uint64, err error)
    SendTransaction(ctx context.Context, txBase64 string) (signature string, err error)
    GetSignatureStatuses(ctx context.Context, signatures []string) ([]SOLSignatureStatus, error)
    GetAccountInfo(ctx context.Context, address string) (*SOLAccountInfo, error)
    GetBalance(ctx context.Context, address string) (uint64, error)
}
```

### 3b: DefaultSOLRPCClient

Implements `SOLRPCClient` using `net/http` JSON-RPC calls (reuses the pattern from `scanner/sol_rpc.go`):
- Uses round-robin RPC URLs (mainnet: Solana public + Helius, devnet: single URL)
- All methods use `"confirmed"` commitment
- `sendTransaction` sends base64-encoded bytes (not deprecated base58)

```go
type DefaultSOLRPCClient struct {
    httpClient *http.Client
    rpcURLs    []string
    currentIdx int
    mu         sync.Mutex
}
```

### 3c: WaitForSOLConfirmation

Polls `getSignatureStatuses` with interval until confirmed, failed, or timeout:

```go
func WaitForSOLConfirmation(ctx context.Context, client SOLRPCClient, signature string) (slot uint64, err error)
```

- Poll every `SOLConfirmationPollInterval` (2s)
- Timeout via context (default 60s)
- If status has `err` field → transaction failed on-chain → return `ErrSOLTxFailed`
- If `confirmationStatus == "confirmed"` or `"finalized"` → success, return slot

**Verification**: Test with mock SOLRPCClient. Test timeout, success, and failure cases.

---

## Task 4: Extend KeyService with DeriveSOLPrivateKey

**File**: `internal/tx/key_service.go`

Add `DeriveSOLPrivateKey` method. Unlike BTC/BSC (which go through BIP-32 `deriveMasterKey()`), SOL uses SLIP-10 from the raw BIP-39 seed — so the flow is: file → mnemonic → seed → `wallet.DeriveSOLPrivateKey(seed, index)`.

```go
func (ks *KeyService) DeriveSOLPrivateKey(ctx context.Context, index uint32) (ed25519.PrivateKey, error)
```

Steps:
1. Read mnemonic from file (`wallet.ReadMnemonicFromFile`)
2. Convert to seed (`wallet.MnemonicToSeed`)
3. Derive ed25519 private key via SLIP-10 (`wallet.DeriveSOLPrivateKey(seed, index)`)
4. Log derivation at DEBUG level (never log the key itself)
5. Return the 64-byte ed25519 private key

**Verification**: Test with abandon...art mnemonic — index 0 should produce public key matching `3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx`.

---

## Task 5: SOL Consolidation Service — Native SOL Sweep

**File**: `internal/tx/sol_tx.go` (same file)

### 5a: SOLConsolidationService struct

```go
type SOLConsolidationService struct {
    keyService *KeyService
    rpcClient  SOLRPCClient
    database   *db.DB
    network    string
}
```

### 5b: PreviewNativeSweep

Calculate the consolidation plan without executing:

```go
func (s *SOLConsolidationService) PreviewNativeSweep(ctx context.Context, addresses []models.AddressWithBalance, destAddress string) (*models.SOLSendPreview, error)
```

Steps:
1. Filter addresses with native SOL balance > 0
2. For each address, calculate sweepable amount: `balance - SOLBaseTransactionFee` (5000 lamports per signer in the batch)
3. Group into batches of `SOLMaxNativeTransferBatch` (7)
4. Calculate total fees: `numBatches * 5000 * batchSize` (each signer pays proportional fee)
5. Actually, fee is paid once per TX by the fee payer (destination). Each source sends `balance - rentExemptMinimum`. Wait — for native sweep, each source is transferring `balance - 5000` lamports to reserve for the per-signature fee. The fee payer (first signer = destination or one of the sources) pays the base fee of `5000 * numSignatures`.
6. Simpler model: destination is NOT a signer (just a recipient). Each batch: one source is designated fee payer, pays `5000 * numSigners` in fee. Other sources transfer `balance` fully. Fee payer transfers `balance - (5000 * numSigners)`.
7. Return preview with batch count, total amount, total fees, net amount.

**Simpler approach chosen**: Each address sends its own individual TX (one signer, one transfer instruction per TX), matching BSC's sequential pattern. This avoids the multi-signer complexity and the tight 1232-byte limit.

Rationale: Multi-signer batching saves on-chain fees (~5000 lamports = 0.000005 SOL ≈ $0.001 per TX) but adds significant implementation complexity. For a self-hosted tool, the simplicity of sequential per-address sends outweighs the marginal fee savings. We can always add multi-signer batching as an optimization later.

**Final architecture for native SOL sweep**: Sequential per-address, each source signs its own TX:
1. Source derives keypair → builds SystemTransfer(source → dest, balance - 5000) → signs → sends
2. Simple, testable, no multi-signer edge cases

### 5c: ExecuteNativeSweep

```go
func (s *SOLConsolidationService) ExecuteNativeSweep(ctx context.Context, addresses []models.AddressWithBalance, destAddress string) (*models.SOLSendResult, error)
```

For each funded address:
1. Derive private key via `KeyService.DeriveSOLPrivateKey(ctx, index)`
2. Fetch fresh balance via `rpcClient.GetBalance` (don't trust scan data)
3. If balance <= `SOLBaseTransactionFee` → skip (insufficient to cover fee)
4. Calculate sendAmount = balance - `SOLBaseTransactionFee`
5. Fetch recent blockhash via `rpcClient.GetLatestBlockhash`
6. Build SystemTransfer instruction (source → dest, sendAmount)
7. Compile message with source as fee payer
8. Serialize and sign (single signer)
9. Check `len(serialized) <= 1232`
10. Base64-encode and send via `rpcClient.SendTransaction`
11. Wait for confirmation via `WaitForSOLConfirmation`
12. Record transaction in DB
13. Collect result into `SOLTxResult`

Return aggregated `SOLSendResult` with success/fail counts and total swept.

**Verification**: Test full flow with mock SOLRPCClient. Test skip on insufficient balance. Test blockhash refresh.

---

## Task 6: SOL Consolidation Service — SPL Token Sweep

**File**: `internal/tx/sol_tx.go` (same file)

### 6a: PreviewTokenSweep

```go
func (s *SOLConsolidationService) PreviewTokenSweep(ctx context.Context, addresses []models.AddressWithBalance, destAddress string, token models.Token, mint string) (*models.SOLSendPreview, error)
```

Steps:
1. Filter addresses with token balance > 0
2. For each address, check native SOL balance >= `SOLBaseTransactionFee` (need SOL for fee)
3. Derive destination ATA using `scanner.DeriveATA(destAddress, mint)`
4. Check if destination ATA exists via `rpcClient.GetAccountInfo`
5. Calculate: if dest ATA doesn't exist, first TX needs CreateATA instruction (costs ~0.00204 SOL rent)
6. Return preview: input count, total token amount, fee estimate, whether ATA creation needed

### 6b: ExecuteTokenSweep

```go
func (s *SOLConsolidationService) ExecuteTokenSweep(ctx context.Context, addresses []models.AddressWithBalance, destAddress string, token models.Token, mint string) (*models.SOLSendResult, error)
```

For each address with token balance:
1. Derive private key via `KeyService.DeriveSOLPrivateKey`
2. Derive source ATA via `scanner.DeriveATA(sourceAddress, mint)`
3. Derive dest ATA via `scanner.DeriveATA(destAddress, mint)`
4. Check dest ATA existence via `rpcClient.GetAccountInfo`
5. Fetch recent blockhash
6. Build instructions:
   - If dest ATA doesn't exist AND this is the first TX: prepend `BuildCreateATAInstruction` (source pays rent)
   - `BuildSPLTransferInstruction(sourceATA, destATA, source, amount)`
7. Compile message with source as fee payer
8. Serialize, sign (single signer — the source address)
9. Send, confirm, record
10. After first successful send with CreateATA, mark dest ATA as created for subsequent TXs

Return aggregated `SOLSendResult`.

**Verification**: Test with mock client. Test ATA creation on first TX only. Test skip when insufficient SOL for fee.

---

## Task 7: New Constants and Error Types

**File**: `internal/config/constants.go` — Add:
```go
// SOL Transaction
SOLLamportsPerSOL              = 1_000_000_000
SOLBaseTransactionFee          = 5_000  // lamports per signature
SOLMaxTxSize                   = 1232   // bytes
SOLConfirmationTimeout         = 60 * time.Second
SOLConfirmationPollInterval    = 2 * time.Second
SOLMaxNativeTransferBatch      = 1      // sequential per-address (simplicity over batching)
SOLRentExemptMinimumLamports   = 890_880 // minimum for a token account (~0.00089 SOL)
SOLATARentLamports             = 2_039_280 // rent for ATA creation (~0.00204 SOL)
SOLSystemProgramID             = "11111111111111111111111111111111"
SOLRentSysvarID                = "SysvarRent111111111111111111111111111111111"
```

**File**: `internal/config/errors.go` — Add:
```go
// SOL sentinel errors
ErrSOLTxTooLarge           = errors.New("SOL transaction exceeds 1232 byte limit")
ErrSOLConfirmationTimeout  = errors.New("SOL transaction confirmation timeout")
ErrSOLTxFailed             = errors.New("SOL transaction failed on-chain")
ErrSOLInsufficientLamports = errors.New("insufficient lamports to cover transaction fee")
ErrSOLATACreationFailed    = errors.New("failed to create associated token account")
ErrSOLBlockhashExpired     = errors.New("recent blockhash expired")

// SOL error codes
ErrorSOLTxTooLarge          = "ERROR_SOL_TX_TOO_LARGE"
ErrorSOLConfirmationTimeout = "ERROR_SOL_CONFIRMATION_TIMEOUT"
ErrorSOLTxFailed            = "ERROR_SOL_TX_FAILED"
ErrorSOLInsufficientLamports = "ERROR_SOL_INSUFFICIENT_LAMPORTS"
ErrorSOLATACreationFailed   = "ERROR_SOL_ATA_CREATION_FAILED"
```

**File**: `internal/models/types.go` — Add:
```go
type SOLSendPreview struct {
    Chain           Chain  `json:"chain"`
    Token           Token  `json:"token"`
    InputCount      int    `json:"inputCount"`
    TotalAmount     string `json:"totalAmount"`     // lamports or token smallest unit
    TotalFee        string `json:"totalFee"`         // lamports
    NetAmount       string `json:"netAmount"`        // native only: totalAmount - totalFee
    DestAddress     string `json:"destAddress"`
    NeedATACreation bool   `json:"needATACreation"`  // SPL only
    ATARentCost     string `json:"ataRentCost"`      // lamports, if ATA creation needed
}

type SOLSendResult struct {
    Chain        Chain         `json:"chain"`
    Token        Token         `json:"token"`
    TxResults    []SOLTxResult `json:"txResults"`
    SuccessCount int           `json:"successCount"`
    FailCount    int           `json:"failCount"`
    TotalSwept   string        `json:"totalSwept"`
}

type SOLTxResult struct {
    AddressIndex int    `json:"addressIndex"`
    FromAddress  string `json:"fromAddress"`
    TxSignature  string `json:"txSignature"`
    Amount       string `json:"amount"`
    Status       string `json:"status"`  // "confirmed", "failed"
    Slot         uint64 `json:"slot,omitempty"`
    Error        string `json:"error,omitempty"`
}
```

**Verification**: Compiles cleanly.

---

## Task 8: Comprehensive Tests

**File**: `internal/tx/sol_serialize_test.go` (new)

Serialization tests:
1. `TestEncodeCompactU16` — values: 0, 1, 127, 128, 255, 16383, 16384
2. `TestSolPublicKeyFromBase58` — known address round-trip
3. `TestBuildSystemTransferInstruction` — data = 12 bytes, correct encoding
4. `TestBuildSPLTransferInstruction` — data = 9 bytes, correct encoding
5. `TestBuildCreateATAInstruction` — data = 0 bytes, 7 accounts
6. `TestCompileMessage_AccountOrdering` — writable+signer first, fee payer at index 0
7. `TestCompileMessage_Deduplication` — same account in multiple instructions counted once
8. `TestSerializeMessage_RoundTrip` — serialize, check size within 1232
9. `TestSignTransaction` — single signer, verify ed25519 signature
10. `TestSignTransaction_MultiSigner` — 3 signers, verify all signatures in order

**File**: `internal/tx/sol_tx_test.go` (new)

Service tests with mock SOLRPCClient:
1. `TestSOLNativeSweep_SingleAddress` — full flow: getBalance → getBlockhash → send → confirm
2. `TestSOLNativeSweep_MultipleAddresses` — sequential sweep of 3 addresses
3. `TestSOLNativeSweep_InsufficientBalance` — skip address with balance <= 5000 lamports
4. `TestSOLNativeSweep_ConfirmationTimeout` — returns error after timeout
5. `TestSOLNativeSweep_ConfirmationFailed` — tx fails on-chain
6. `TestSOLTokenSweep_WithExistingATA` — no CreateATA instruction
7. `TestSOLTokenSweep_WithATACreation` — first TX includes CreateATA
8. `TestSOLTokenSweep_InsufficientSOLForFee` — skip address without SOL for fee
9. `TestWaitForSOLConfirmation_Success` — status returns confirmed
10. `TestWaitForSOLConfirmation_Failed` — status has error

**File**: `internal/tx/key_service_test.go` (extend)

1. `TestDeriveSOLPrivateKey` — index 0 with abandon...art → public key matches `3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx`
2. `TestDeriveSOLPrivateKey_MultipleIndices` — indices 0-2 produce distinct keys

</tasks>

---

<success_criteria>
1. `DeriveSOLPrivateKey` returns correct ed25519 key for known test vector (abandon...art index 0)
2. Compact-u16 encoding handles all value ranges correctly
3. Transaction serialization produces valid wire format under 1232 bytes
4. `CompileMessage` correctly orders accounts (writable+signer first, fee payer at 0)
5. SystemProgram.Transfer data is exactly 12 bytes (u32 variant + u64 lamports)
6. SPL Token.Transfer data is exactly 9 bytes (u8 variant + u64 amount)
7. CreateATA instruction has 0 bytes data and 7 accounts
8. Native SOL sweep: derives key → fetches balance → builds TX → signs → sends → confirms → records
9. SPL token sweep: derives ATA → checks existence → creates if needed → transfers → confirms
10. All errors and constants in central config files
11. All tests pass: `go test ./internal/tx/... -v -count=1`
12. No private keys logged or stored — derived on-demand, used, discarded
</success_criteria>

<verification>
```bash
cd /home/louis/dev/hdpay && /usr/local/go/bin/go test ./internal/tx/... -v -count=1
cd /home/louis/dev/hdpay && /usr/local/go/bin/go vet ./internal/tx/... ./internal/config/... ./internal/models/...
cd /home/louis/dev/hdpay && /usr/local/go/bin/go build ./...
```
</verification>

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/tx/sol_serialize.go` | Create | Core types, compact-u16, message compilation, serialization, signing |
| `internal/tx/sol_tx.go` | Create | SOLRPCClient interface, DefaultSOLRPCClient, confirmation polling, SOLConsolidationService |
| `internal/tx/key_service.go` | Modify | Add `DeriveSOLPrivateKey` method |
| `internal/config/constants.go` | Modify | SOL TX constants (fee, size limit, confirmation params, program IDs) |
| `internal/config/errors.go` | Modify | SOL sentinel errors and error codes |
| `internal/models/types.go` | Modify | SOLSendPreview, SOLSendResult, SOLTxResult |
| `internal/tx/sol_serialize_test.go` | Create | 10 tests for serialization primitives |
| `internal/tx/sol_tx_test.go` | Create | 10 tests for consolidation service |
| `internal/tx/key_service_test.go` | Modify | 2 new tests for SOL key derivation |

## Key Design Decisions

1. **Sequential per-address sends (not multi-signer batch)**: Each source address sends its own TX. Multi-signer batching saves ~$0.001/TX in fees but adds significant complexity (1232-byte limit with ~7 signers max, complex account ordering). Sequential is simpler, more testable, matches BSC pattern, and can be optimized later.

2. **No gagliardetto/solana-go dependency**: Raw binary serialization using only stdlib + base58. Keeps the binary small and avoids dependency churn.

3. **ATA reuse from scanner package**: `scanner.DeriveATA()` already works — import it in `tx` package (no import cycle, `tx` already imports `scanner`).

4. **Dest ATA creation lazily**: Only prepend CreateATA instruction if `getAccountInfo` returns null. After first successful creation, skip for subsequent TXs in the same sweep.

## Edge Cases Handled

- **Insufficient lamports for fee**: Skip address if balance <= 5000 lamports
- **Blockhash expiry**: Fetch fresh blockhash per TX (valid ~60-90s, one TX takes <5s)
- **Dest ATA doesn't exist**: Create it in the first TX that sweeps tokens
- **Source ATA doesn't exist**: Skip (no tokens to sweep — shouldn't happen if scanner found balance)
- **TX too large**: Check serialized size before sending, error if > 1232 bytes
- **Confirmation timeout**: 60s poll with 2s interval, then error
- **On-chain failure**: `getSignatureStatuses` returns error field → record as failed
- **Context cancellation**: Check `ctx.Err()` between addresses for graceful stop
