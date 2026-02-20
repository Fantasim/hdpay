# Session 005 — 2026-02-20

## Version: V1
## Phase: building
## Summary: Phase 3 (Blockchain Providers) completed. Provider interface for tx detection, ProviderSet round-robin with HDPay RateLimiter+CircuitBreaker. BTC (Blockstream+Mempool, pagination), BSC (BscScan txlist+tokentx, block confirmations), SOL (JSON-RPC, native+SPL, composite tx_hash). 49 tests, 82.5% coverage.

## What Was Done
- Created Provider interface tailored to transaction detection (FetchTransactions, CheckConfirmation, GetCurrentBlock)
- Built RawTransaction struct to represent detected incoming transactions
- Implemented ProviderSet with thread-safe round-robin rotation, importing HDPay's RateLimiter and CircuitBreaker
- Built BlockstreamProvider for BTC tx detection (Esplora API, pagination at 25/page, multi-output aggregation)
- Built MempoolProvider embedding BlockstreamProvider (same API format, different URL)
- Built BscScanProvider for BSC tx detection (txlist for BNB + tokentx for USDC/USDT, block-number confirmation counting)
- Built SolanaRPCProvider for SOL tx detection (getSignaturesForAddress + getTransaction, native SOL via balance deltas, SPL tokens via postTokenBalances)
- Implemented composite tx_hash format for SOL ("signature:TOKEN") to handle multi-transfer transactions
- Added NewHTTPClient using HDPay's connection pool constants
- Added provider error category and severity constants to poller config
- Wrote 49 tests with httptest mock servers, 82.5% coverage
- Research: Solana getTransaction response structure (pre/postBalances, pre/postTokenBalances, loadedAddresses)
- Research: BscScan testnet API URL confirmed as api-testnet.bscscan.com/api

## Decisions Made
- **MempoolProvider embeds BlockstreamProvider**: Same Esplora API format, only Name() and baseURL differ — avoids code duplication
- **Composite tx_hash for SOL**: "signature:SOL", "signature:USDC", etc. — one Solana tx can produce multiple RawTransaction entries
- **BTC confirmation**: Simple confirmed=true/false from Esplora (1-conf threshold), no block number tracking
- **BSC confirmation**: currentBlock - txBlock >= 12, fetches block via BscScan proxy eth_blockNumber
- **SOL confirmation**: confirmationStatus == "finalized", no block counting needed
- **Helius**: Falls back to Solana devnet URL in testnet mode (no separate Helius devnet endpoint)
- **weiToHuman shared**: BSC and SOL token amounts use the same big.Float conversion function
- **Generic methods**: Go doesn't allow type parameters on methods — refactored to concrete types with closures

## Issues / Blockers
- Go generic methods not allowed — had to restructure execute() from a generic method to concrete executeFetch() using closures for confirmation/block queries
- BSC testnet USDC uses 6 decimals (not 18 like mainnet) — handled in tokenDecimals() method

## Next Steps
- Start Phase 4: Watch Engine — the core polling loop that ties providers + points + DB together
- Watcher service: poll active watches on chain-specific intervals
- Transaction detection using ProviderSet.ExecuteFetch()
- Confirmation tracking using ProviderSet.ExecuteConfirmation()
- Points calculation integration with PointsCalculator + Pricer
