# Session 007 — 2026-02-18

## Version: V1
## Phase: building (Phase 7 of 11)
## Summary: Phase 7 complete — BTC Transaction Engine: on-demand key derivation, UTXO fetching, dynamic fee estimation, multi-input P2WPKH TX building+signing, broadcasting, transaction DB CRUD, 38 new tests

## What Was Done
- Expanded Phase 7 PLAN.md from outline to fully detailed plan (8 tasks)
- Researched btcd P2WPKH signing API (MultiPrevOutFetcher, WitnessSignature, NewTxSigHashes)
- Researched Blockstream/Mempool broadcast endpoints and UTXO fetching APIs
- Added BTC TX constants to `internal/config/constants.go` (dust threshold, vsize weights, fee estimation, max inputs)
- Added 7 sentinel errors + 4 error codes to `internal/config/errors.go`
- Added UTXO, FeeEstimate, SendPreview, SendResult types to `internal/models/types.go`
- Created `internal/tx/key_service.go` — on-demand BIP-84 private key derivation from mnemonic file
- Created `internal/tx/btc_utxo.go` — UTXO fetching with round-robin Blockstream/Mempool rotation
- Created `internal/tx/btc_fee.go` — dynamic fee estimation from mempool.space with fallback
- Created `internal/tx/btc_tx.go` — multi-input P2WPKH TX building, signing, consolidation orchestrator
- Implemented `internal/tx/broadcaster.go` — shared Broadcaster interface + BTC broadcast with provider fallback
- Implemented `internal/db/transactions.go` — full transaction CRUD (insert, update status, get, list)
- Created 6 test files with 38 total tests (all passing)
- Wrote Phase 7 SUMMARY.md

## Decisions Made
- **MultiPrevOutFetcher**: Critical for multi-input signing — CannedPrevOutputFetcher would produce wrong signatures on inputs 1+
- **Confirmed UTXOs only**: Unconfirmed UTXOs filtered during fetch to avoid spending unconfirmed outputs
- **No mnemonic caching**: Read from file each derivation call for security (minimize time secrets in memory)
- **HalfHourFee default**: mempool.space `halfHourFee` (~3 blocks) chosen as default priority
- **text/plain broadcast**: Blockstream/Mempool expect raw hex as text/plain, not JSON
- **No retry on 400**: Bad TX (400) means TX is invalid, don't retry on other providers

## Issues / Blockers
- Tests initially used 12-word mnemonic but `ReadMnemonicFromFile` validates for 24 words — fixed to use 24-word test mnemonic
- `go mod tidy` needed after adding btcd/wire and txscript imports

## Next Steps
- Start Phase 8: BSC Transaction Engine + Gas Pre-Seed
- Extend `KeyService` with `DeriveBSCPrivateKey` method
- Implement BSC native + BEP-20 transfer logic
- Implement gas pre-seeding for token transfers
- Add `BSCBroadcaster` using the shared `Broadcaster` interface
