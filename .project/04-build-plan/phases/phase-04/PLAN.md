# Phase 4: Scanner Engine (Backend)

> **Status: Outline** — Will be expanded with full task details when this phase is reached.

<objective>
Build the complete scanning engine backend: provider interface with round-robin rotation, per-provider rate limiting, 7 provider implementations (3 BTC, 2 BSC, 2 SOL), the scanner orchestrator with resume logic, token scanning (native + USDC/USDT), and the SSE hub for broadcasting progress events.
</objective>

## Key Deliverables

1. **Provider Interface** — `Provider` interface with `GetBalances(ctx, addresses)` and `Name()`, `MaxBatchSize()`, `Chain()` methods
2. **Rate Limiter** — Per-provider token bucket rate limiter using `golang.org/x/time/rate`
3. **BTC Providers** — Blockstream Esplora (single address), Mempool.space (single), Blockchain.info (batch up to 50)
4. **BSC Providers** — BscScan API (batch 20 for native, single for tokens), BSC public RPC (ethclient)
5. **SOL Providers** — Solana public RPC (getMultipleAccounts batch 100), Helius free tier
6. **Provider Pool** — Round-robin rotation with automatic failover on rate limit/error
7. **Scanner Orchestrator** — Per-chain scan with goroutine, context cancellation, batch processing
8. **Resume Logic** — Persist `scan_state` in DB, resume from `last_scanned_index`, 24h threshold
9. **Token Scanning** — BTC (native only), BSC (BNB + USDC + USDT via separate passes), SOL (SOL + USDC/USDT via ATA derivation)
10. **SSE Hub** — Fan-out broadcaster, client registration/unregistration, keepalive pings
11. **Balance DB Operations** — Upsert balances, query funded addresses, aggregate summaries

## Files to Create/Modify

- `internal/scanner/provider.go` — Provider interface
- `internal/scanner/ratelimiter.go` — Rate limiter wrapper
- `internal/scanner/btc_provider.go` — 3 BTC providers
- `internal/scanner/bsc_provider.go` — 2 BSC providers
- `internal/scanner/sol_provider.go` — 2 SOL providers
- `internal/scanner/pool.go` — Provider pool with rotation
- `internal/scanner/scanner.go` — Scanner orchestrator
- `internal/scanner/sse.go` — SSE hub
- `internal/db/balances.go` — Balance CRUD
- `internal/db/scans.go` — Scan state CRUD
- Tests for each provider (mock HTTP), scanner orchestrator, SSE hub

## Estimated Complexity
- **High** — Most complex backend phase
- 7 different API integrations
- Token scanning adds 3x complexity for BSC/SOL
- Resume logic requires careful state management

<research_needed>
- Blockchain.info multiaddr API: exact query limits and response format for bech32 addresses
- BscScan tokenbalance endpoint: batch capability or single-address only
- Solana getMultipleAccounts: response format for SPL token accounts (ATA data layout)
- Helius free tier: exact rate limits and endpoint differences from public RPC
</research_needed>
