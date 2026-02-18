# Phase 7 Summary: BTC Transaction Engine

## Completed: 2026-02-18

## What Was Built
- On-demand key derivation service: reads mnemonic file, derives BIP-84 private keys per index, caller zeros after use
- UTXO fetcher: fetches confirmed UTXOs from Blockstream/Mempool APIs with round-robin rotation and rate limiting
- Dynamic fee estimator: fetches fee rates from mempool.space `/v1/fees/recommended` with fallback to config constant
- Multi-input P2WPKH transaction builder: vsize estimation, wire.MsgTx construction, consolidation to single output
- P2WPKH witness signer: uses `MultiPrevOutFetcher` + `NewTxSigHashes` (once) + `WitnessSignature` per input
- BTC broadcaster: POST raw hex as `text/plain` with ordered provider fallback (no retry on 400)
- Transaction DB CRUD: insert, update status, get by ID/hash, paginated list with chain filter
- Shared `Broadcaster` interface for future BSC/SOL reuse
- 38 new tests (31 tx package + 7 DB transaction tests)

## Files Created/Modified
- `internal/tx/key_service.go` — On-demand BIP-84 private key derivation from mnemonic file
- `internal/tx/btc_utxo.go` — UTXO fetching with round-robin provider rotation
- `internal/tx/btc_fee.go` — Dynamic fee estimation from mempool.space with fallback
- `internal/tx/btc_tx.go` — TX building, signing, serialization, consolidation orchestrator
- `internal/tx/broadcaster.go` — Shared Broadcaster interface + BTC implementation
- `internal/tx/key_service_test.go` — 7 tests (known vector, public/private match, errors)
- `internal/tx/btc_utxo_test.go` — 7 tests (fetch, filter unconfirmed, empty, errors, round-robin)
- `internal/tx/btc_fee_test.go` — 4 tests (API fetch, fallback, min enforcement, default rate)
- `internal/tx/btc_tx_test.go` — 10 tests (vsize, build, sign integration, dust, insufficient, max inputs)
- `internal/tx/broadcaster_test.go` — 4 tests (broadcast, fallback, bad tx no retry, all fail)
- `internal/db/transactions.go` — Transaction CRUD operations (was empty stub)
- `internal/db/transactions_test.go` — 7 tests (insert/retrieve, status update, list, pagination, filter)
- `internal/config/constants.go` — Added BTC TX constants (dust threshold, vsize weights, fee estimation)
- `internal/config/errors.go` — Added 7 new sentinel errors + 4 new error codes
- `internal/models/types.go` — Added UTXO, FeeEstimate, SendPreview, SendResult types

## Decisions Made
- **MultiPrevOutFetcher**: Critical for multi-input signing — CannedPrevOutputFetcher would produce wrong signatures on inputs 1+
- **Confirmed UTXOs only**: Unconfirmed UTXOs are filtered out during fetch to avoid spending unconfirmed outputs
- **No mnemonic caching**: Mnemonic is read from file each derivation call to minimize time secrets spend in memory
- **HalfHourFee default**: mempool.space `halfHourFee` (~3 blocks) chosen as default priority for consolidation sweeps
- **text/plain broadcast**: Both Blockstream and Mempool expect raw hex as text/plain, NOT JSON
- **No retry on 400**: Bad transaction (400) means the TX itself is invalid, so don't retry on other providers
- **One transaction record per input**: Each input address gets its own DB record for history tracking

## Deviations from Plan
- `btc_utxo.go` created as a standalone file rather than extending existing scanner providers (cleaner separation of concerns)
- Key derivation uses `NewKeyService` pattern (reads file path, not mnemonic) for security
- No separate UTXO cache table — UTXOs are fetched fresh each time (simpler, avoids stale data)

## Issues Encountered
- Test initially used 12-word mnemonic but `ReadMnemonicFromFile` validates for 24 words — fixed to use 24-word test mnemonic

## Notes for Next Phase
- `Broadcaster` interface in `broadcaster.go` is ready for BSC/SOL implementations
- `KeyService` can be extended with `DeriveBSCPrivateKey` and `DeriveSOLPrivateKey` methods
- The consolidation service (`BTCConsolidationService`) is backend-only — API endpoints come in Phase 10
- `BTCFeeRateSatPerVByte` constant renamed to `BTCDefaultFeeRate` for clarity
- Private keys are zeroed after each input signing via `privKey.Zero()`
