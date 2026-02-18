# Phase 4 Summary: Scanner Engine (Backend)

## Completed: 2026-02-18

## What Was Built
- Multi-provider blockchain scanner with round-robin rotation and automatic failover
- BTC providers: Blockstream Esplora + Mempool.space
- BSC providers: BscScan REST API (batch 20 native) + ethclient JSON-RPC (native + BEP-20 token)
- SOL provider: Solana JSON-RPC (`getMultipleAccounts` batch 100, native + SPL token via ATA derivation)
- Per-provider rate limiting using `golang.org/x/time/rate`
- Provider pool with round-robin + failover on rate limit / unavailable errors
- Scanner orchestrator with goroutine-based scanning, context cancellation, and DB resume support
- SSE hub for fan-out event broadcasting to connected clients
- Balance DB operations: upsert single/batch, funded query, summary aggregation
- Scan state DB operations: get/upsert/resume with 24h threshold
- Manual Solana ATA (Associated Token Account) derivation via PDA + Edwards25519 on-curve check
- 56 tests covering scanner, SSE, pool, DB balances, DB scans, providers

## Files Created/Modified

### Created
- `internal/scanner/provider.go` — Provider interface + BalanceResult type
- `internal/scanner/ratelimiter.go` — Rate limiter wrapping golang.org/x/time/rate
- `internal/scanner/btc_blockstream.go` — Blockstream Esplora BTC provider
- `internal/scanner/btc_mempool.go` — Mempool.space BTC provider
- `internal/scanner/bsc_bscscan.go` — BscScan REST API (native batch + single token)
- `internal/scanner/bsc_rpc.go` — BSC ethclient provider (native + BEP-20 token via balanceOf)
- `internal/scanner/sol_ata.go` — Solana ATA derivation (PDA, ed25519 on-curve check)
- `internal/scanner/sol_rpc.go` — Solana JSON-RPC provider (native + SPL token)
- `internal/scanner/pool.go` — Provider pool (round-robin, failover)
- `internal/scanner/sse.go` — SSE event hub (subscribe/unsubscribe/broadcast)
- `internal/scanner/scanner.go` — Scanner orchestrator (StartScan/StopScan/resume)
- `internal/scanner/setup.go` — SetupScanner factory function + provider wiring
- `internal/scanner/sol_ata_test.go` — 5 ATA tests
- `internal/scanner/btc_blockstream_test.go` — 5 Blockstream tests
- `internal/scanner/bsc_bscscan_test.go` — 4 BscScan tests
- `internal/scanner/pool_test.go` — 4 pool tests
- `internal/scanner/sse_test.go` — 6 SSE hub tests
- `internal/scanner/scanner_test.go` — 6 scanner orchestrator tests

### Modified
- `internal/db/balances.go` — Balance CRUD operations (UpsertBalance, UpsertBalanceBatch, GetFundedAddresses, GetBalanceSummary, GetAddressesBatch)
- `internal/db/scans.go` — Scan state operations (GetScanState, UpsertScanState, ShouldResume)
- `internal/config/constants.go` — Provider URLs, scanner constants, Solana program IDs
- `internal/config/errors.go` — ErrProviderUnavailable, ErrTokensNotSupported, ErrScanAlreadyRunning
- `go.mod` / `go.sum` — Added golang.org/x/time + go-ethereum dependencies
- `internal/db/balances_test.go` — 10 balance/batch DB tests
- `internal/db/scans_test.go` — 8 scan state DB tests

## Decisions Made
- **Dropped Blockchain.info**: Does not support bech32 (bc1) addresses for balance queries
- **BscScan tokens single-address only**: No batch token balance API; use individual `tokenbalance` calls
- **Manual ATA derivation**: ~160 lines of custom PDA derivation avoids pulling full solana-go dependency
- **Non-blocking SSE broadcast**: Slow clients get events dropped rather than blocking other clients
- **Token failures non-fatal**: Token balance fetch errors are logged but don't fail the scan
- **NULLIF fix for scan state**: Used `NULLIF(excluded.started_at, '')` to properly preserve started_at on updates

## Deviations from Plan
- None significant — all 13 tasks completed as planned

## Issues Encountered
- **go-ethereum transitive deps**: Adding ethclient import pulled ~60 transitive dependencies; resolved with `go mod tidy`
- **sol_ata.go type error**: Initial version used non-existent `bigInt` type; rewrote with `math/big.Int`
- **DB setupTestDB collision**: Existing `addresses_test.go` already defined `setupTestDB`; reused shared helpers
- **scan state COALESCE bug**: Empty string `""` treated as non-NULL by `COALESCE`; fixed with `NULLIF`

## Notes for Next Phase
- Phase 5 (Scan UI + SSE) should wire SSE hub to API handler and connect frontend via EventSource
- Scanner orchestrator exposes `StartScan`, `StopScan`, `Status`, `IsRunning` — ready for handler integration
- `SetupScanner` factory creates all providers and pools; needs to be called from main.go
- Token scanning works for BSC (USDC/USDT via BscScan or RPC) and SOL (USDC/USDT via ATA derivation + RPC)
