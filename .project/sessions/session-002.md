# Session 002 — 2026-02-18

## Version: V1
## Phase: building (Phase 2 of 11)
## Summary: Phase 2 complete: BIP-39 mnemonic, BTC/BSC/SOL address derivation, init CLI, JSON export, 37 tests at 83.3% coverage

## What Was Done
- Installed HD wallet dependencies (go-bip39, btcsuite/btcd, go-ethereum, mr-tron/base58)
- Added domain types: NetworkMode, AllChains, AddressExport, AddressExportItem
- Implemented BIP-39 mnemonic validation, seed derivation, and file reading (hd.go)
- Implemented BTC Native SegWit (bech32) address derivation via BIP-84 (btc.go)
- Implemented BSC/EVM EIP-55 checksummed address derivation via BIP-44 (bsc.go)
- Implemented SOL address derivation via manual SLIP-10 ed25519 (~120 lines, zero extra deps) (sol.go)
- Built bulk address generator with progress callbacks for all three chains (generator.go)
- Built streaming JSON export (handles 500K addresses without OOM) (export.go)
- Created `init` CLI command: generates 500K addresses per chain, batch inserts 10K/tx, idempotent
- Created `export` CLI command: exports addresses to `./data/export/*.json`
- Added DB address methods: InsertAddressBatch, CountAddresses, GetAddresses, StreamAddresses
- Wrote 37 unit tests with 83.3% coverage, including SLIP-10 spec test vectors
- Verified known test vectors: BTC, BSC, SOL addresses for 12-word abandon mnemonic

## Decisions Made
- BTC uses BIP-84 (purpose=84) instead of BIP-44 (purpose=44) for correct bech32 standard — test vector confirmed
- SOL uses manual SLIP-10 ed25519 implementation instead of external library — verified against spec vectors
- SOL derivation path: m/44'/501'/N'/0' (Phantom standard, all hardened segments)
- Added mr-tron/base58 as only extra dependency for SOL address encoding

## Issues / Blockers
- BTC test vector mismatch: bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu is BIP-84, not BIP-44 — fixed by switching to BIP-84
- Research agent's SOL test vector was incorrect — verified independently with Node.js
- TestDeriveSOLPrivateKey panic on type assertion — fixed by using privKey[32:] slice
- go mod tidy removed deps when wallet was a stub — re-ran after adding actual imports

## Next Steps
- Start Phase 3: Address Explorer
- Build address listing API endpoints (GET /api/addresses/:chain with pagination)
- Build paginated address table UI with chain filtering
- Build address detail view
