# Phase 5 Summary: Provider Health & Broadcast Fallback

## Completed: 2026-02-19

## What Was Built
- DB-backed provider health recording wired into scanner Pool
- Provider Health API endpoint (`GET /api/health/providers`)
- BSC broadcast fallback via `FallbackEthClient` wrapper (primary + Ankr RPC)
- SOL broadcast fallback via `doRPCAllURLs` (tries all configured URLs before failing)
- Frontend live provider health component with color-coded status indicators
- Automatic provider health row initialization on startup

## Files Created/Modified
- `internal/scanner/pool.go` — Added `database *db.DB` field, `SetDB()` for DB injection + initial health row upsert, `recordHealthSuccess()` / `recordHealthFailure()` helpers, health recording after circuit breaker state changes
- `internal/db/provider_health.go` — Added `UpdateProviderCircuitState()` method deriving status from circuit state
- `internal/scanner/setup.go` — Added `pool.SetDB(database)` calls for BTC, BSC, SOL pools
- `internal/api/handlers/provider_health.go` — NEW: `GetProviderHealth()` handler grouping rows by chain
- `internal/api/router.go` — Added `GET /health/providers` route
- `internal/tx/bsc_fallback.go` — NEW: `FallbackEthClient` implementing `EthClientWrapper` with SendTransaction fallback
- `internal/tx/bsc_fallback_test.go` — NEW: 5 tests covering primary success, fallback, both fail, nil fallback, delegation
- `cmd/server/main.go` — BSC fallback client wiring for mainnet (Ankr as secondary)
- `internal/tx/sol_tx.go` — Refactored `doRPC` → `doRPCToURL`, added `doRPCAllURLs` for broadcast fallback
- `web/src/lib/types.ts` — Added `ProviderHealthStatus`, `CircuitState`, `ProviderHealth`, `ProviderHealthMap` types
- `web/src/lib/utils/api.ts` — Added `getProviderHealth()` function
- `web/src/lib/components/scan/ProviderStatus.svelte` — Rewrote: live API fetch, color-coded dots, loading/error states

## Decisions Made
- **Non-blocking DB writes**: Health recording uses goroutines to avoid adding latency to scanner hot paths
- **SetDB() pattern**: Optional setter keeps backward compatibility with tests that don't need a DB
- **FallbackEthClient wrapper**: Clean interface-preserving pattern for BSC; only SendTransaction falls back, other methods delegate to primary
- **SOL round-robin fallback**: Leverages existing multi-URL support via `doRPCAllURLs` — tries all URLs before returning first error
- **Startup health init**: `SetDB()` upserts initial health rows for all providers, ensuring the health API always has data

## Deviations from Plan
- Task 6 (startup initialization) was folded into Task 1's `SetDB()` method rather than being a separate change in `main.go` — cleaner because initialization happens automatically when DB is set

## Issues Encountered
- None significant — all implementations were straightforward

## Notes for Next Phase
- Provider health is now visible in the UI and persisted to DB
- Circuit breaker state changes are reflected in both DB and API responses
- BSC and SOL broadcast paths now have fallback redundancy
- Phase 6 (Security Tests & Infrastructure) can build on this health infrastructure for monitoring
