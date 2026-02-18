# Session 013 — 2026-02-18

## Version: V2
## Phase: Building (Phase 1 of 6)
## Summary: V2 Build Phase 1 complete — Foundation: Schema, Error Types & Circuit Breaker

## What Was Done
- Created `tx_state` table migration (005) for tracking individual TX lifecycle through broadcast
- Created `provider_health` table migration (006) for per-provider health and circuit breaker state
- Implemented TX state DB methods: Create, UpdateStatus, GetPending, GetBySweep, GetByNonce, CountByStatus
- Implemented provider health DB methods: Upsert, Get, GetByChain, GetAll, RecordSuccess, RecordFailure
- Added `TransientError` type with `IsTransient()` and `GetRetryAfter()` helpers to errors.go
- Added `ErrCircuitOpen`, `ErrProviderTimeout` sentinel errors
- Added constants: TX state statuses, provider health statuses, provider types, circuit states, circuit breaker config
- Implemented circuit breaker state machine (closed/open/half-open) in scanner package
- Enhanced `BalanceResult` with `Error` and `Source` fields
- Updated all 6 provider implementations to populate `Source` field on BalanceResult
- Implemented sweep ID generator using crypto/rand
- Fixed pre-existing `TestRunMigrationsIdempotent` to be migration-count-agnostic
- Wrote 24 new tests across 4 test files: tx_state (8), provider_health (6), circuit_breaker (8), errors (4)

## Files Created
- `internal/db/migrations/005_tx_state.sql`
- `internal/db/migrations/006_provider_health.sql`
- `internal/db/tx_state.go` + `tx_state_test.go`
- `internal/db/provider_health.go` + `provider_health_test.go`
- `internal/scanner/circuit_breaker.go` + `circuit_breaker_test.go`
- `internal/config/errors_test.go`
- `internal/tx/sweep.go`

## Files Modified
- `internal/config/constants.go` — Added circuit breaker, TX state, provider health, circuit state constants
- `internal/config/errors.go` — Added TransientError type, ErrCircuitOpen, ErrProviderTimeout
- `internal/scanner/provider.go` — Enhanced BalanceResult with Error + Source fields
- `internal/scanner/btc_blockstream.go` — Added Source to BalanceResult
- `internal/scanner/btc_mempool.go` — Added Source to BalanceResult
- `internal/scanner/bsc_bscscan.go` — Added Source to BalanceResult
- `internal/scanner/bsc_rpc.go` — Added Source to BalanceResult
- `internal/scanner/sol_rpc.go` — Added Source to BalanceResult
- `internal/scanner/setup.go` — Added Source to test provider BalanceResult
- `internal/db/sqlite_test.go` — Fixed TestRunMigrationsIdempotent

## Decisions Made
- Circuit breaker: 3-failure threshold, 30s cooldown, 1 half-open request (matching plan exactly)
- TransientError uses errors.As for unwrap chain support
- BalanceResult.Error stays empty for now — will be populated in Phase 2

## Issues / Blockers
- None

## Next Steps
- Start V2 Build Phase 2: Scanner Resilience
- Read `.project/v2/04-build-plan/phases/phase-02/PLAN.md`
