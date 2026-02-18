# Phase 7: BTC Transaction Engine

> **Status: Outline** — Will be expanded with full task details when this phase is reached.

<objective>
Build the BTC transaction engine: UTXO fetching, multi-input transaction building with P2WPKH witness signing, dynamic fee estimation (mempool.space API), transaction broadcast, and on-demand private key derivation with immediate zeroing.
</objective>

## Key Deliverables

1. **On-Demand Key Derivation** — service that reads mnemonic file, derives specific private key(s), returns key, caller zeros after use
2. **UTXO Fetcher** — fetch UTXOs for funded BTC addresses via Blockstream/Mempool API
3. **Transaction Builder** — multi-input P2WPKH transaction:
   - Multiple inputs (one per funded address UTXO)
   - Single output to destination
   - Fee calculation: vsize × feeRate
4. **Dynamic Fee Estimation** — auto-fetch from mempool.space `/api/v1/fees/recommended`, auto-select "halfHour" (medium) by default, show all tiers in preview for user override, fallback to config constant only if API unreachable
5. **Witness Signing** — P2WPKH witness signature per input using `txscript.WitnessSignature`
6. **Broadcaster** — serialize to hex, POST to Blockstream/Mempool broadcast endpoint
7. **Transaction Recording** — store in transactions table with status tracking

## Files to Create/Modify

- `internal/tx/key_derivation.go` — on-demand key derivation service
- `internal/tx/btc_tx.go` — UTXO fetch, TX build, sign, broadcast
- `internal/tx/broadcaster.go` — shared broadcast + confirmation tracking
- `internal/db/transactions.go` — transaction CRUD
- Tests with mock UTXO responses

## Edge Cases
- TX too large (>100KB) → split into multiple transactions
- UTXO already spent (race condition) → broadcast fails, suggest re-scan
- Total inputs < minimum fee → skip address
- Fee estimation fails → fall back to config constant

<research_needed>
- btcd/wire transaction building API: exact usage for P2WPKH inputs
- Blockstream broadcast endpoint: POST format and response
- vsize calculation for SegWit transactions with multiple inputs
</research_needed>
