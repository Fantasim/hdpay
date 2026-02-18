# Phase 2 Summary: HD Wallet & Address Generation

## Completed: 2026-02-18

## What Was Built
- BIP-39 mnemonic validation, seed derivation, and file reading
- BTC Native SegWit (bech32) address derivation via BIP-84 path `m/84'/0'/0'/0/N`
- BSC/EVM EIP-55 checksummed address derivation via BIP-44 path `m/44'/60'/0'/0/N`
- SOL address derivation via manual SLIP-10 ed25519 implementation, path `m/44'/501'/N'/0'`
- Bulk address generator with progress callbacks for all three chains
- Streaming JSON export (handles 500K addresses without OOM)
- `init` CLI command — generates 1.5M addresses, batch inserts 10K/tx, idempotent
- `export` CLI command — exports addresses to `./data/export/*.json`
- DB address methods — batch insert, count, paginated fetch, stream
- 37 wallet tests with 83.3% coverage, including SLIP-10 spec test vectors

## Files Created/Modified
- `internal/wallet/hd.go` — BIP-39 mnemonic handling, master key derivation
- `internal/wallet/btc.go` — BTC bech32 address derivation (BIP-84)
- `internal/wallet/bsc.go` — BSC/EVM address derivation (BIP-44, coin type 60)
- `internal/wallet/sol.go` — SOL address derivation (SLIP-10 ed25519, manual ~120 lines)
- `internal/wallet/generator.go` — Bulk address generation with progress callbacks
- `internal/wallet/export.go` — Streaming JSON export
- `internal/wallet/errors.go` — Wallet-specific error types
- `internal/wallet/hd_test.go` — Mnemonic & master key tests
- `internal/wallet/btc_test.go` — BTC derivation tests with known vectors
- `internal/wallet/bsc_test.go` — BSC derivation tests with known vectors
- `internal/wallet/sol_test.go` — SOL derivation tests + SLIP-10 spec vectors
- `internal/wallet/generator_test.go` — Generator tests
- `internal/wallet/export_test.go` — Export tests with mock streamer
- `internal/db/addresses.go` — Address CRUD: batch insert, count, paginated get, stream, delete
- `internal/models/types.go` — Added NetworkMode, AllChains, AddressExport types
- `internal/config/constants.go` — Added BIP84Purpose constant
- `cmd/server/main.go` — Added `init` and `export` subcommands

## Decisions Made
- **BIP-84 for BTC**: Used purpose=84 (BIP-84) instead of purpose=44 (BIP-44) for Native SegWit bech32, matching industry standard
- **Manual SLIP-10**: Implemented SLIP-10 ed25519 manually (~120 lines) instead of using a library — zero additional dependencies, verified against official spec test vectors
- **SOL test vector**: Verified against Node.js SLIP-10 implementation; address is `HAgk14JpMQLgt6rVgv7cBQFJWFto5Dqxi472uT3DKpqk` for 12-word abandon mnemonic
- **base58 library**: Added `github.com/mr-tron/base58` for Solana address encoding
- **Multi-value INSERT**: Used batch INSERT with multi-value syntax for 10K rows/transaction for best SQLite performance

## Deviations from Plan
- BTC uses BIP-84 (purpose=84) instead of BIP-44 (purpose=44) as specified in CLAUDE.md — corrected to match bech32 standard and known test vectors
- SOL known vector address differs from plan estimate — verified correct via independent Node.js implementation
- `DeriveSOLPrivateKey` function added for future transaction signing use

## Issues Encountered
- BTC test vector `bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu` is BIP-84, not BIP-44 — switched to BIP-84 to match
- go-ethereum v1.17.0 pulls in a large dependency tree (including `ProjectZKM/Ziren`) but is necessary for `crypto.PubkeyToAddress`
- Research agent's SOL test vector was incorrect — verified independently with Node.js

## Notes for Next Phase
- Address generation for 500K per chain takes ~3-5 minutes total (primarily BTC is slowest due to secp256k1 derivation)
- DB has addresses table with (chain, address_index) primary key and indexes on chain and address
- `StreamAddresses` method available for any future operations that need to iterate all addresses
- Mnemonic file path is read from `--mnemonic-file` flag or `HDPAY_MNEMONIC_FILE` env var
- Private keys are never stored — only derived on-demand from seed for signing
