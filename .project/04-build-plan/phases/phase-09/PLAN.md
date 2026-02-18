# Phase 9: SOL Transaction Engine

> **Status: Outline** — Will be expanded with full task details when this phase is reached.

<objective>
Build the SOL transaction engine for multi-instruction batch transfers (native SOL and SPL tokens), Associated Token Account derivation, multi-signer transaction building, batch splitting (~20 instructions per TX), and blockhash management.
</objective>

## Key Deliverables

1. **Native SOL Batch Sweep** — multi-instruction transactions:
   - Up to 20 `system.Transfer` instructions per TX
   - First signer is fee payer
   - All source addresses sign the transaction
   - Reserve 5000 lamports per address for fee
2. **SPL Token Sweep** — `spl_token.Transfer` instructions:
   - Derive Associated Token Account (ATA) for each source + mint
   - Derive/create destination ATA if needed
   - Authority = source keypair
3. **ATA Derivation** — deterministic PDA derivation: `findProgramAddress([wallet, TOKEN_PROGRAM_ID, mint], ASSOCIATED_TOKEN_PROGRAM_ID)`
4. **Batch Splitting** — split funded addresses into groups of ~20, one TX per group
5. **Blockhash Management** — fetch fresh blockhash per batch (valid ~60 seconds)
6. **Multi-Signer Transactions** — transaction signed by all source addresses in the batch

## Files to Create/Modify

- `internal/tx/sol_tx.go` — SOL native + SPL token transactions
- Tests with mock RPC responses

## Edge Cases
- Transaction too large (too many signers) → reduce batch size
- Blockhash expired between build and send → fetch new blockhash
- ATA doesn't exist for destination → create instruction
- Insufficient lamports after fee → skip address

<research_needed>
- gagliardetto/solana-go: transaction building API, multi-signer support
- SPL token transfer instruction format
- CreateAssociatedTokenAccount instruction format
- Solana transaction size calculation and limits
</research_needed>
