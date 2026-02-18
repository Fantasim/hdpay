# Phase 8 Summary: BSC Transaction Engine + Gas Pre-Seed

## Completed: 2026-02-18

## What Was Built
- BSC private key derivation: BIP-44 m/44'/60'/0'/0/N → ECDSA private key + EIP-55 address
- Native BNB transfer: LegacyTx building with EIP-155 signing (chain ID 56 mainnet / 97 testnet)
- BEP-20 token transfer: Manual ABI encoding of `transfer(address,uint256)` — 68-byte calldata
- Receipt polling: WaitForReceipt with `ethereum.NotFound` detection, revert handling, configurable timeout
- BSC Consolidation Service: Sequential per-address sweep orchestrator for native BNB + BEP-20 tokens
- Gas pre-seeding service: Distributes 0.005 BNB from source to targets needing gas, sequential nonce management
- 20% gas price buffer on SuggestGasPrice for BSC reliability
- EthClientWrapper interface for full testability
- 22 new tests (16 BSC TX + 4 gas pre-seed + 4 key derivation — but 2 key derivation are extensions)

## Files Created/Modified
- `internal/tx/key_service.go` — Added `DeriveBSCPrivateKey` method + `deriveBSCPrivKeyAtIndex` helper
- `internal/tx/bsc_tx.go` — EthClientWrapper interface, BNB/BEP-20 TX building, EIP-155 signing, receipt polling, BSCConsolidationService
- `internal/tx/gas.go` — GasPreSeedService (preview + execute) with sequential nonce management
- `internal/config/constants.go` — BSC chain IDs, gas price buffer, receipt polling, BEP-20 selector
- `internal/config/errors.go` — 5 new sentinel errors + 4 new error codes
- `internal/models/types.go` — BSCSendPreview, BSCSendResult, BSCTxResult, GasPreSeedPreview, GasPreSeedResult
- `internal/tx/bsc_tx_test.go` — 18 tests for BSC TX engine
- `internal/tx/gas_test.go` — 4 tests for gas pre-seeding
- `internal/tx/key_service_test.go` — 4 new tests for BSC key derivation

## Decisions Made
- **EthClientWrapper interface**: Minimal interface over ethclient for testability — only the 5 methods we actually need
- **Manual ABI encoding**: `transfer(address,uint256)` selector `0xa9059cbb` + LeftPadBytes — simpler than abi.Pack for this single use case
- **20% gas price buffer**: `SuggestGasPrice * 12/10` to prevent stuck TXs on BSC's occasional gas spikes
- **Sequential per-address sends**: Each funded address gets its own TX (BSC doesn't support multi-input like BTC UTXO)
- **PendingNonceAt once + local increment**: For gas pre-seed batch sends, fetch nonce once then increment per TX to avoid nonce conflicts
- **No retry on reverts**: If a BEP-20 transfer reverts (receipt.Status == 0), it's recorded as failed — don't retry
- **Gas pre-seed as separate direction**: Recorded in DB with direction "gas-preseed" for clear history tracking
- **Real-time balance check**: Each native sweep TX re-fetches balance from ethclient rather than trusting scan data
- **Address verification**: Derived address is compared against expected address before signing as safety check

## Deviations from Plan
- Combined all BSC TX logic into `bsc_tx.go` rather than splitting into separate files — keeps related code together
- `checkGasForTokenSweep` queries real-time BNB balance from ethclient rather than relying on scan DB (more reliable)
- Test count slightly higher than planned (22 vs 16) due to added edge case coverage

## Issues Encountered
- None — go-ethereum v1.17.0 API worked as expected, all patterns from research phase applied cleanly

## Notes for Next Phase
- `EthClientWrapper` interface is ready for SOL-equivalent (different interface since Solana isn't EVM)
- `KeyService` still needs `DeriveSOLPrivateKey` for Phase 9
- BSC consolidation service and gas pre-seed are backend-only — API endpoints come in Phase 10 (Send Interface)
- `WaitForReceipt` could be reused if any future chain needs receipt polling (unlikely for SOL which uses different confirmation model)
- The `models.BSCTxResult` type can be generalized to `TxResult` in Phase 10 if SOL uses a similar structure
