# Phase 1 Summary: Foundation — Schema, Error Types & Circuit Breaker

## Completed: 2026-02-18

## What Was Built
- `tx_state` DB table for tracking individual TX lifecycle (pending→broadcasting→confirming→confirmed|failed|uncertain)
- `provider_health` DB table for per-provider health and circuit breaker state persistence
- TX state CRUD: 6 methods (Create, UpdateStatus, GetPending, GetBySweep, GetByNonce, CountByStatus)
- Provider health CRUD: 6 methods (Upsert, Get, GetByChain, GetAll, RecordSuccess, RecordFailure)
- `TransientError` type with `IsTransient()` and `GetRetryAfter()` for retry classification
- Circuit breaker state machine (closed/open/half-open) with configurable threshold and cooldown
- Enhanced `BalanceResult` with `Error` and `Source` fields across all providers
- Sweep ID generator using crypto/rand
- 24 new tests across 4 test files

## Files Created/Modified
- `internal/db/migrations/005_tx_state.sql` — TX state tracking table
- `internal/db/migrations/006_provider_health.sql` — Provider health table
- `internal/db/tx_state.go` — TX state CRUD methods
- `internal/db/tx_state_test.go` — 8 tests
- `internal/db/provider_health.go` — Provider health CRUD methods
- `internal/db/provider_health_test.go` — 6 tests
- `internal/scanner/circuit_breaker.go` — Circuit breaker state machine
- `internal/scanner/circuit_breaker_test.go` — 8 tests
- `internal/config/errors.go` — TransientError type + new sentinel errors
- `internal/config/errors_test.go` — 4 tests
- `internal/config/constants.go` — New constant groups
- `internal/tx/sweep.go` — Sweep ID generator
- `internal/scanner/provider.go` — BalanceResult enhanced
- 6 provider files — Source field populated

## Decisions Made
- Circuit breaker: 3 consecutive fails threshold, 30s cooldown, 1 half-open request
- TransientError uses errors.As for full unwrap chain support
- BalanceResult.Error deferred to Phase 2 (field exists, not yet populated)

## Deviations from Plan
- None — implemented exactly as planned

## Issues Encountered
- `TestRunMigrationsIdempotent` broke due to hardcoded migration count — fixed to be count-agnostic

## Notes for Next Phase
- Circuit breaker is ready but not yet wired into providers (Phase 2 will integrate it)
- BalanceResult.Error field exists but is empty — Phase 2 will populate it during error collection
- tx_state table ready for Phase 3 (TX Safety Core) to use
- provider_health table ready for Phase 5 (Provider Health) to use
