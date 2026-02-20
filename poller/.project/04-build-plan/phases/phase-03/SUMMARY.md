# Phase 3 Summary: Blockchain Providers

## Completed: 2026-02-20

## What Was Built
- Provider interface tailored to transaction detection (different from HDPay's balance-scanning interface)
- ProviderSet with thread-safe round-robin rotation, rate limiting, and circuit breaker integration
- BTC provider: Blockstream + Mempool.space (shared Esplora API, pagination at 25/page)
- BSC provider: BscScan (normal txlist + tokentx, block-number-based confirmation counting)
- SOL provider: Solana JSON-RPC (getSignaturesForAddress + getTransaction, native SOL + SPL token detection, composite tx_hash)
- Comprehensive test suite: 49 tests, 82.5% coverage

## Files Created/Modified
- `internal/poller/provider/provider.go` — Provider interface, RawTransaction, ProviderSet (round-robin + rate limit + circuit breaker), NewHTTPClient
- `internal/poller/provider/btc.go` — BlockstreamProvider, MempoolProvider (shared Esplora API format, pagination, satoshi→human conversion)
- `internal/poller/provider/bsc.go` — BscScanProvider (normal BNB + token USDC/USDT detection, confirmation counting via block numbers, weiToHuman converter)
- `internal/poller/provider/sol.go` — SolanaRPCProvider (JSON-RPC, native SOL via pre/postBalances, SPL tokens via pre/postTokenBalances, composite tx_hash, extractBaseSignature, lamportsToHuman)
- `internal/poller/provider/provider_test.go` — ProviderSet round-robin, rotation, circuit breaker, context cancellation tests
- `internal/poller/provider/btc_test.go` — BTC response parsing, cutoff, pagination, multi-output, error handling tests
- `internal/poller/provider/bsc_test.go` — BSC normal + token parsing, cutoff, confirmation counting, error handling tests
- `internal/poller/provider/sol_test.go` — SOL native + SPL parsing, composite tx_hash, confirmation status, error handling tests
- `internal/poller/config/constants.go` — Added ErrorCategoryProvider/Watcher, ErrorSeverity constants

## Decisions Made
- **Embedded MempoolProvider**: MempoolProvider embeds BlockstreamProvider (same Esplora API format, only Name() and baseURL differ) — avoids code duplication
- **Composite tx_hash for SOL**: `"signature:SOL"`, `"signature:USDC"`, etc. — allows a single Solana tx to produce multiple RawTransaction entries (native + token in same tx)
- **BTC confirmation**: Uses simple `confirmed=true/false` from Esplora API (1-conf threshold), no block number tracking needed
- **BSC confirmation**: Uses `currentBlock - txBlock >= 12` (pollerconfig.ConfirmationsBSC), fetches block via BscScan proxy `eth_blockNumber`
- **SOL confirmation**: Uses `confirmationStatus == "finalized"` (pollerconfig.SOLCommitment), no block counting
- **Helius provider**: Falls back to Solana devnet URL in testnet mode (no separate Helius devnet endpoint in constants)
- **weiToHuman shared**: BSC's `weiToHuman()` reused by SOL's `tokenAmountToHuman()` (same big.Float math)

## Deviations from Plan
- **No separate Helius provider type**: HeliusProvider is just a SolanaRPCProvider with a different URL (same JSON-RPC format), created via `NewHeliusProvider()` factory function
- **Token decimals for BSC testnet**: BSC testnet USDC uses 6 decimals (not 18 like mainnet) — handled in `tokenDecimals()` method

## Issues Encountered
- **Go generic methods**: Go doesn't allow type parameters on methods — refactored `execute[T]` from a generic method to concrete `executeFetch` using `[]RawTransaction` as the return type, with closures for confirmation/block queries
- **BscScan "No transactions found"**: BscScan returns `status: "0"` with `message: "No transactions found"` for empty results (not an error) — handled as a special case

## Notes for Next Phase
- Phase 4 (Watch Engine) will use `ProviderSet.ExecuteFetch()` to poll addresses and `ExecuteConfirmation()` to track pending transactions
- Provider failures should be logged to `system_errors` table using the new `ErrorCategoryProvider` constant
- The `RawTransaction` struct maps directly to the `transactions` table columns via the watcher layer
