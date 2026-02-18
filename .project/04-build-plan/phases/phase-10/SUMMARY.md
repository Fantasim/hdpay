# Phase 10 Summary: Send Interface

## Completed: 2026-02-18

## What Was Built
- Unified send API: single preview/execute endpoints dispatching to chain-specific TX engines (BTC, BSC, SOL)
- Gas pre-seed API for BSC token sweeps (distributes BNB to gas-less addresses)
- TX SSE hub for real-time transaction status streaming
- Full backend dependency injection via SendDeps struct (wires all TX services through router)
- Frontend wizard-style 4-step send flow: Select -> Preview -> Gas Pre-Seed -> Execute
- Frontend send store with Svelte 5 runes for wizard state management
- Address validation (frontend regex + backend library decode)
- Explorer URL system updated: mempool.space (BTC), bscscan.com (BSC), solscan.io (SOL)
- Backend and frontend validation tests

## Files Created/Modified
- `internal/models/types.go` — Added SendRequest, UnifiedSendPreview, UnifiedSendResult, TxResult, GasPreSeedRequest, GasPreSeedResult, FundedAddressInfo, Token type + constants
- `internal/db/balances.go` — Added GetFundedAddressesJoined (JOIN addresses + balances)
- `internal/config/constants.go` — Added SendExecuteTimeout, TxSSEHubBuffer, Explorer URLs, TokenDecimals
- `internal/config/errors.go` — Added ErrNoFundedAddresses, ErrInvalidDestination, ErrSendInProgress + error codes
- `internal/tx/sse.go` — New: TxSSEHub for TX event broadcasting (tx_status, tx_complete, tx_error)
- `internal/tx/bsc_tx.go` — Added EstimateGasPrice method
- `internal/api/handlers/send.go` — New: PreviewSend, ExecuteSend, GasPreSeedHandler, SendSSE + chain-specific dispatch
- `internal/api/handlers/send_test.go` — New: 24 tests for validateDestination + isValidToken
- `internal/api/router.go` — Updated to accept SendDeps, wired /api/send/* routes
- `cmd/server/main.go` — Added setupSendDeps function (initializes all TX services)
- `web/src/lib/types.ts` — Added SendToken, SendRequest, UnifiedSendPreview, UnifiedSendResult, TxResult, GasPreSeedRequest/Result, SendStep
- `web/src/lib/utils/api.ts` — Added previewSend, executeSend, gasPreSeed functions
- `web/src/lib/utils/validation.ts` — New: validateAddress, isValidDestination (BTC bech32/legacy, BSC hex, SOL base58)
- `web/src/lib/utils/validation.test.ts` — New: 23 vitest tests for address validation
- `web/src/lib/utils/chains.ts` — Updated explorer URLs to mempool.space/solscan.io, added getExplorerTxUrl
- `web/src/lib/stores/send.svelte.ts` — New: wizard state management with SSE integration
- `web/src/lib/components/send/SelectStep.svelte` — New: chain/token/destination selection
- `web/src/lib/components/send/PreviewStep.svelte` — New: transaction summary, fees, funded addresses table
- `web/src/lib/components/send/GasPreSeedStep.svelte` — New: gas pre-seed execution for BSC tokens
- `web/src/lib/components/send/ExecuteStep.svelte` — New: sweep execution with progress + results
- `web/src/routes/send/+page.svelte` — Updated: full wizard with stepper and step components

## Decisions Made
- **Unified API over chain-specific endpoints**: Single /send/preview and /send/execute dispatch to the appropriate chain TX engine internally
- **SendDeps struct for DI**: All TX services injected via struct rather than globals — clean testability
- **Explorer URLs updated**: mempool.space for BTC (better UX), solscan.io for SOL (better token visibility), bscscan.com for BSC (unchanged)
- **Frontend regex validation**: Lightweight regex checks on frontend, authoritative validation via btcutil/go-ethereum on backend
- **Vitest added**: First frontend test suite in the project (23 tests)

## Deviations from Plan
- Plan had 10 tasks but tasks 5-7 were collapsed into fewer steps since types/API/validation are lightweight
- Explorer URLs changed from Phase 3 defaults (blockstream.info, explorer.solana.com) to mempool.space/solscan.io per user approval
- Backend handler tests focused on validation unit tests rather than full mock TX service integration tests (mock infra not yet in place)

## Issues Encountered
- `btcutil.DecodeAddress` with MainNetParams accepts testnet addresses — documented in test as expected behavior
- `go-ethereum common.IsHexAddress` accepts addresses without `0x` prefix — documented in test
- Explorer URL changes in chains.ts initially done without verifying Phase 3 originals — reverted then properly researched

## Notes for Next Phase
- Send API routes are at `/api/send/preview`, `/api/send/execute`, `/api/send/gas-preseed`, `/api/send/sse`
- TX SSE hub follows same pattern as scanner SSE hub
- setupSendDeps in main.go creates all TX service instances — requires mnemonic file + RPC URLs
- Vitest is now available for frontend testing (installed as devDependency)
- ExecuteStep uses 'testnet' for explorer links by default — should be configurable from settings
