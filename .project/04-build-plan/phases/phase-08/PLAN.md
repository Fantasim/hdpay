# Phase 8: BSC Transaction Engine + Gas Pre-Seed

> **Status: Outline** — Will be expanded with full task details when this phase is reached.

<objective>
Build the BSC transaction engine for native BNB transfers and BEP-20 token transfers (USDC/USDT), nonce management, EIP-155 signing, receipt waiting, and the gas pre-seeding system that distributes BNB to token-holding addresses with 0 gas.
</objective>

## Key Deliverables

1. **Native BNB Sweep** — sequential transfers from each funded address to destination:
   - Get nonce, estimate gas price
   - Calculate sendAmount = balance - gasCost
   - Build LegacyTx, sign with EIP-155 signer (chain ID 56 mainnet / 97 testnet)
   - Broadcast, wait for receipt
2. **BEP-20 Token Sweep** — ABI-encoded `transfer(address,uint256)`:
   - Encode `transfer` function call
   - Gas limit: 65,000 (BEP-20)
   - Requires BNB for gas → ties into gas pre-seeding
3. **Nonce Management** — sequential nonce tracking per address, handle nonce gaps
4. **Dynamic Gas Price** — auto-fetch via `ethclient.SuggestGasPrice()` from BSC RPC, no manual gas input needed, shown in preview for transparency
5. **Gas Pre-Seeding**:
   - Identify addresses with tokens but 0 BNB
   - User selects gas source address
   - Send 0.005 BNB to each target
   - Track confirmations
   - Validate source has sufficient BNB
5. **Gas Pre-Seed API** — `POST /api/send/gas-preseed`

## Files to Create/Modify

- `internal/tx/bsc_tx.go` — BSC native + BEP-20 transactions
- `internal/tx/gas.go` — gas pre-seeding logic
- `internal/api/handlers/send.go` — gas pre-seed endpoint (partial, completed in Phase 10)
- Tests with mock ethclient

## Edge Cases
- Nonce conflict during batch send
- Gas price spike during sequential sends
- Source has insufficient BNB for all pre-seeds
- BEP-20 transfer fails (contract revert)

<research_needed>
- go-ethereum ethclient: exact API for pending nonce, gas suggestion, send transaction
- EIP-155 signer: chain ID handling for BSC mainnet vs testnet
- BEP-20 transfer ABI encoding: manual vs go-ethereum/accounts/abi
</research_needed>
