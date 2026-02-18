# Phase 2: HD Wallet & Address Generation

<objective>
Implement BIP-44 HD wallet address derivation for all three chains (BTC bech32, BSC EVM, SOL ed25519), the `init` CLI command that generates 500K addresses per chain and stores them in SQLite, and JSON export. Verify correctness against known test vectors.
</objective>

<tasks>

## Task 1: BIP-39 Mnemonic Handling

Implement mnemonic validation and seed generation.

**Files:**
- `internal/wallet/hd.go`:
  - `ValidateMnemonic(mnemonic string) error` — validate 24-word BIP-39 mnemonic
  - `MnemonicToSeed(mnemonic string) ([]byte, error)` — BIP-39 seed derivation (empty passphrase)
  - `ReadMnemonicFromFile(path string) (string, error)` — read file, trim whitespace, validate
  - `DeriveMasterKey(seed []byte, net *chaincfg.Params) (*hdkeychain.ExtendedKey, error)` — BIP-32 master key

**Dependencies:** `github.com/tyler-smith/go-bip39`, `github.com/btcsuite/btcd/btcutil/hdkeychain`

**Verification:**
- Known mnemonic "abandon...art" produces expected seed
- Invalid mnemonic (wrong word count, invalid words) returns error
- File reading trims trailing newlines

## Task 2: BTC Address Derivation

Implement BTC Native SegWit (bech32) address generation via BIP-44 path `m/44'/0'/0'/0/N`.

**Files:**
- `internal/wallet/btc.go`:
  - `DeriveBTCAddress(masterKey *hdkeychain.ExtendedKey, index uint32, net *chaincfg.Params) (string, error)`
  - Path: `m/44'/0'/0'/0/N` (mainnet) or `m/44'/1'/0'/0/N` (testnet)
  - Output: bech32 address (bc1q... / tb1q...)
  - Uses: `hdkeychain.Child()` → `ECPubKey()` → `btcutil.Hash160` → `btcutil.NewAddressWitnessPubKeyHash`
- `internal/wallet/btc_test.go`:
  - Test with "abandon...art" mnemonic
  - Verify index 0 address matches known value: `bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu`
  - Test indices 0-4 produce distinct, valid bech32 addresses
  - Test mainnet vs testnet produces different addresses
  - Test invalid index handling

**Verification:**
- Index 0 with test mnemonic produces exact known address
- All generated addresses start with `bc1q` (mainnet) or `tb1q` (testnet)
- Sequential indices produce sequential, deterministic addresses

## Task 3: BSC/EVM Address Derivation

Implement EVM address generation via BIP-44 path `m/44'/60'/0'/0/N`.

**Files:**
- `internal/wallet/bsc.go`:
  - `DeriveBSCAddress(masterKey *hdkeychain.ExtendedKey, index uint32) (string, error)`
  - Path: `m/44'/60'/0'/0/N` (same for mainnet and testnet, coin type 60)
  - Output: EIP-55 checksummed hex address (0x...)
  - Uses: `hdkeychain.Child()` → `ECPrivKey()` → raw bytes → `crypto.ToECDSA` → `crypto.PubkeyToAddress`
- `internal/wallet/bsc_test.go`:
  - Test with "abandon...art" mnemonic
  - Verify index 0 address matches known MetaMask value: `0x9858EfFD232B4033E47d90003D41EC34EcaEda94`
  - Test indices 0-4 produce distinct, valid EIP-55 addresses
  - Test EIP-55 checksum correctness

**Dependencies:** `github.com/ethereum/go-ethereum/crypto`

**Verification:**
- Index 0 with test mnemonic produces exact known address
- All addresses are valid EIP-55 checksummed
- Deterministic across runs

## Task 4: SOL Address Derivation (SLIP-10)

Implement Solana ed25519 address generation via SLIP-10 path `m/44'/501'/N'/0'`.

**Files:**
- `internal/wallet/sol.go`:
  - `DeriveSOLAddress(seed []byte, index uint32) (string, error)`
  - SLIP-10 ed25519 derivation (NOT BIP-32):
    1. `HMAC-SHA512(key="ed25519 seed", data=seed)` → master key + chain code
    2. For each hardened path element: `HMAC-SHA512(key=chainCode, data=0x00||parentKey||index_with_hardened_bit)`
    3. Final 32-byte key → `ed25519.NewKeyFromSeed(key)` → public key
    4. Base58-encode public key = Solana address
  - Path: `m/44'/501'/N'/0'` (all hardened, Phantom standard)
- `internal/wallet/sol_test.go`:
  - Test with "abandon...art" mnemonic
  - Verify index 0 address against Phantom wallet
  - Test indices 0-4 produce distinct, valid Base58 addresses
  - Test that addresses are 32-44 characters (Base58 encoded 32 bytes)

**Decision:** Implement SLIP-10 manually (~50 lines) rather than using `dmitrymomot/solana` library, for control and fewer dependencies. Fall back to library if manual implementation produces incorrect addresses.

<research_needed>
- Verify exact SLIP-10 ed25519 derivation algorithm matches Phantom wallet output for "abandon...art" mnemonic
</research_needed>

**Verification:**
- Index 0 with test mnemonic matches Phantom wallet address
- All addresses are valid Base58-encoded 32-byte public keys
- SLIP-10 hardened derivation correctly uses 0x80000000 offset

## Task 5: Init CLI Command

Implement the `init` subcommand that generates all addresses and stores them.

**Files:**
- `cmd/server/main.go` — wire up `init` subcommand:
  - Parse `--mnemonic-file` flag (required)
  - Read and validate mnemonic
  - For each chain (BTC, BSC, SOL):
    1. Check if addresses already exist → skip if count = 500K
    2. Generate 500K addresses
    3. Batch insert into SQLite (10K per transaction)
    4. Log progress every batch
  - Export JSON after generation
- `internal/wallet/generator.go`:
  - `GenerateAddresses(chain string, masterKey/seed, count int, net) ([]Address, error)` — generates addresses in memory
  - Progress callback for logging
- `internal/db/addresses.go`:
  - `InsertAddressBatch(chain string, addresses []Address) error` — batch insert (10K per tx)
  - `CountAddresses(chain string) (int, error)` — for idempotency check
  - `GetAddresses(chain string, offset, limit int) ([]Address, error)` — paginated fetch
  - `GetAddressByIndex(chain string, index int) (*Address, error)` — single fetch

**Verification:**
- `go run ./cmd/server init --mnemonic-file ./test_mnemonic.txt` generates 1.5M addresses
- Running init twice skips already-generated chains
- Partial init (interrupted) detects count mismatch and regenerates
- Progress logged every 10K addresses
- DB contains exactly 500K rows per chain

## Task 6: JSON Export

Implement address export to JSON files.

**Files:**
- `internal/wallet/export.go`:
  - `ExportAddresses(db *DB, chain, network, outputDir string) error`
  - Streams from DB to JSON file (don't load all 500K into memory at once)
  - Output format per TECHNICAL_REFERENCE.md:
    ```json
    {
      "chain": "BTC",
      "network": "mainnet",
      "derivation_path_template": "m/44'/0'/0'/0/{index}",
      "generated_at": "...",
      "count": 500000,
      "addresses": [{"index": 0, "address": "bc1q..."}, ...]
    }
    ```
  - Export path: `./data/export/{chain}_addresses.json`
- `cmd/server/main.go` — wire up `export` subcommand

**Verification:**
- Export creates valid JSON files
- File size reasonable (~30-50MB per chain)
- JSON contains correct count and derivation path template
- Streaming export doesn't OOM on 500K addresses

## Task 7: Shared Domain Types

Define all shared types used across packages.

**Files:**
- `internal/models/types.go`:
  - `Chain` type (string enum: "BTC", "BSC", "SOL")
  - `Token` type (string enum: "NATIVE", "USDC", "USDT")
  - `Address` struct: Chain, Index, Address string
  - `Balance` struct: Chain, Index, Token, Balance string, LastScanned
  - `ScanState` struct: Chain, LastScannedIndex, MaxScanID, Status, timestamps
  - `Transaction` struct: all fields from DB schema
  - `NetworkMode` type: "mainnet" / "testnet"

**Verification:**
- Types compile and are importable by all packages
- No circular dependencies

## Task 8: Go Module Dependencies

Install and verify all Go dependencies needed for this phase.

**Commands:**
```
go get github.com/tyler-smith/go-bip39
go get github.com/btcsuite/btcd
go get github.com/btcsuite/btcd/btcutil
go get github.com/btcsuite/btcd/btcutil/hdkeychain
go get github.com/ethereum/go-ethereum/crypto
go get github.com/go-chi/chi/v5
go get modernc.org/sqlite
go get github.com/kelseyhightower/envconfig
go mod tidy
```

**Verification:**
- `go mod tidy` produces clean go.sum
- `go build ./...` succeeds
- No unused dependencies

</tasks>

<success_criteria>
- [ ] BTC derivation: index 0 with test mnemonic = `bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu`
- [ ] BSC derivation: index 0 with test mnemonic = `0x9858EfFD232B4033E47d90003D41EC34EcaEda94`
- [ ] SOL derivation: index 0 with test mnemonic matches Phantom wallet
- [ ] `init` command generates 1.5M addresses (500K per chain) in SQLite
- [ ] Init is idempotent (skip existing chains)
- [ ] Batch insert uses 10K transaction batches
- [ ] JSON export produces valid files with streaming (no OOM)
- [ ] All derivation tests pass with known test vectors
- [ ] `go test ./internal/wallet/...` passes with >80% coverage
- [ ] No private keys stored in DB — only public addresses
- [ ] Mnemonic read from file, never logged
</success_criteria>

<verification>
1. Run tests: `go test -v -cover ./internal/wallet/...`
2. Init: `go run ./cmd/server init --mnemonic-file ./test_mnemonic.txt`
3. Check DB: `sqlite3 ./data/hdpay.sqlite "SELECT chain, COUNT(*) FROM addresses GROUP BY chain"`
4. Export: `go run ./cmd/server export`
5. Check exports: `ls -lh ./data/export/` → 3 JSON files
6. Idempotency: run init again → "addresses already exist, skipping"
</verification>
