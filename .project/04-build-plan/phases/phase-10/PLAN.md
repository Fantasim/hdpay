# Phase 10: Send Interface

> **Status: Outline** — Will be expanded with full task details when this phase is reached.

<objective>
Build the complete send/sweep interface: preview API (fees, addresses, gas assessment), execute API (sign + broadcast all), gas pre-seed API integration, and the wizard-style frontend with step progression, real-time TX progress via SSE, and receipt display with explorer links.
</objective>

## Key Deliverables

1. **Send Preview API** — `POST /api/send/preview`:
   - Input: chain, token, destination
   - Output: funded addresses, total amount, fee estimate, gas pre-seed needs, TX count
   - Validates destination address (chain-specific format)
2. **Send Execute API** — `POST /api/send/execute`:
   - Input: chain, token, destination
   - Re-fetches balances at execution time (may have changed since preview)
   - Dispatches to chain-specific TX engine
   - Broadcasts SSE events per TX: pending → confirmed / failed
   - Returns list of TX hashes
3. **Gas Pre-Seed Integration** — `POST /api/send/gas-preseed` fully wired
4. **Frontend Wizard** — matching `.project/03-mockups/screens/send.html`:
   - Stepper: Select → Preview → Gas Pre-Seed → Execute
   - Step 1: Chain/token selector, funded address count
   - Step 2: Transaction summary, fee estimate, gas warning, funded address table
   - Step 3: Gas pre-seed (if BSC tokens and addresses need gas)
   - Step 4: Execute with real-time progress, receipt with explorer links
   - Collapsed completed steps showing summary
5. **Address Validation** — chain-specific:
   - BTC: bech32 format (bc1...) or legacy
   - BSC: EIP-55 checksummed hex (0x...)
   - SOL: Base58, 32-44 characters
6. **TX Progress SSE** — `tx_status` events with hash, status, confirmations

## Files to Create/Modify

- `internal/api/handlers/send.go` — preview, execute, gas-preseed endpoints
- `web/src/routes/send/+page.svelte`
- `web/src/lib/components/send/SendPanel.svelte`
- `web/src/lib/components/send/GasPreSeed.svelte`
- `web/src/lib/components/send/TransactionConfirm.svelte`
- `web/src/lib/utils/validation.ts` — address validation

## Reference Mockup
- `.project/03-mockups/screens/send.html`

<research_needed>
- BTC bech32 address validation in TypeScript (regex or library)
- Solana Base58 address validation
</research_needed>
