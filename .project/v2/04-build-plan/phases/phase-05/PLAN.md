# Phase 5: Provider Health & Broadcast Fallback (Outline)

> Expanded to detailed plan when Phase 4 is complete.

## Goal
Add provider health visibility for the user and broadcast redundancy for BSC/SOL.

## Audit IDs Covered
D2, plan #24 (health endpoint)

## Key Changes

### Provider Health Endpoint
`GET /api/health/providers` returns:
```json
{
  "data": {
    "BTC": [
      {"name": "blockstream", "status": "healthy", "circuitState": "closed", "lastSuccess": "...", "consecutiveFails": 0},
      {"name": "mempool", "status": "degraded", "circuitState": "half_open", "lastSuccess": "...", "consecutiveFails": 2}
    ],
    "BSC": [...],
    "SOL": [...]
  }
}
```

### Provider Health Tracking
On every provider call in the pool:
- Success → `RecordProviderSuccess(name)` → updates DB + resets circuit breaker
- Failure → `RecordProviderFailure(name, error)` → updates DB + may trip circuit breaker

### BSC Broadcast Fallback
Add secondary RPC endpoint for BSC TX broadcast:
- Primary: configured BSC RPC (e.g., `bsc-dataseed.binance.org`)
- Fallback: Ankr public (`rpc.ankr.com/bsc`)
- Try primary first, on failure try fallback
- Both testnet and mainnet fallback URLs

### SOL Broadcast Fallback
Add secondary RPC endpoint for SOL TX broadcast:
- Primary: configured Solana RPC
- Fallback: secondary RPC (e.g., Helius if available, or public RPC)
- Same pattern as BSC

### Frontend Health Indicators
Add provider status to scan page:
- Color-coded indicators per provider (green/yellow/red)
- Show circuit breaker state
- Auto-refresh on scan page load

## Files Modified/Created
- `internal/api/handlers/health.go` — Provider health handler
- `internal/scanner/pool.go` — Health recording on every call
- `internal/db/provider_health.go` — Used for persistence
- `internal/tx/bsc_tx.go` — Fallback RPC for broadcast
- `internal/tx/sol_tx.go` — Fallback RPC for broadcast
- `internal/config/constants.go` — Fallback RPC URLs
- `internal/api/router.go` — New route
- `web/src/routes/scan/+page.svelte` — Health indicators
- `web/src/lib/utils/api.ts` — Health API function
- `web/src/lib/types.ts` — ProviderHealth type

## Estimated Tests: ~15
