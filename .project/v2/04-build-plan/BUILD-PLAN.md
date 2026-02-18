# HDPay V2 — Build Plan (Robustness & Hardening)

> Fix 37 audit issues across 6 phases. No new user-facing features. Focus: money-loss prevention, scanner resilience, provider health, security tests, infrastructure hardening.

## Overview

HDPay V2 is built in **6 phases**, progressing from foundational types/schema through scanner fixes, transaction safety, provider health, and finally tests/infrastructure. Each phase is designed to be completable in a single coding session (3-5 hours) and produces a testable increment. All phases modify existing V1 code — no new pages or UI components.

## Phase Summary

| # | Phase | Audit IDs Covered | Complexity | Key Deliverables |
|---|-------|-------------------|------------|-----------------|
| 1 | Foundation: Schema, Error Types & Circuit Breaker | A13, B2, B5, B9, D6 | M | `tx_state` table, `provider_health` table, `BalanceResult` with error field, transient/permanent error wrapper, circuit breaker implementation, wired retry constants |
| 2 | Scanner Resilience | B1, B3, B4, B6, B7, B8, B10, B11 | L | Error collection (not early-return), partial result validation, atomic scan state, Retry-After parsing, token error visibility, decoupled native/token, SSE resync, exponential backoff |
| 3 | TX Safety — Core | A1, A2, A3, A4, A5 | L | Concurrent send mutex, BTC confirmation polling, SOL confirmation error handling, in-flight TX persistence, SOL blockhash refresh |
| 4 | TX Safety — Advanced | A6, A7, A8, A9, A10, A11, A12 | L | UTXO re-validation, BSC balance recheck, partial sweep resume, gas pre-seed idempotency, SOL ATA confirmation, BSC gas re-estimation |
| 5 | Provider Health & Broadcast Fallback | B5 wiring, D2, plan #24 | M | `/api/health/providers` endpoint, provider status tracking, BSC broadcast fallback (Ankr), SOL broadcast fallback, health in scan page |
| 6 | Security Tests & Infrastructure | C1, C2, C3, D1, D3, D4, D5 | M | Security middleware tests, scanner provider tests, TX SSE hub tests, HTTP idle timeout, DB pool limits, stale-price fallback, graceful shutdown |

## Dependency Graph

```
Phase 1: Foundation (schema + types + circuit breaker)
    │
    ├──────────────────────┐
    ▼                      ▼
Phase 2: Scanner         Phase 3: TX Safety Core
Resilience                 │
    │                      ▼
    │                    Phase 4: TX Safety Advanced
    │                      │
    └──────┬───────────────┘
           ▼
         Phase 5: Provider Health & Fallback
           │
           ▼
         Phase 6: Security Tests & Infrastructure
```

Phase 2 and Phase 3 can run in parallel (independent). Phases 4-6 are sequential.

## Session Model

- **Phase 1**: Detailed PLAN.md (ready to build immediately)
- **Phases 2-6**: Outline PLAN.md (expanded when reached, using context from prior phases)
- Each phase ends with a SUMMARY.md documenting what was built, decisions made, and notes for next phase
- Commit after each phase completion

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|-----------|
| In-flight TX persistence adds write latency to sends | Medium | Use async DB writes, batch where possible |
| Circuit breaker too aggressive = blocks healthy providers | Medium | Conservative defaults (3 failures, 30s cooldown), configurable |
| Partial result handling increases scanner complexity | Medium | Clean interfaces, thorough unit tests |
| BSC/SOL fallback RPC may have different behavior | Low | Test against both endpoints, normalize responses |
| Security middleware tests may reveal actual vulnerabilities | Low | Fix immediately if found |

---

## Phase 1: Foundation — Schema, Error Types & Circuit Breaker

### Goal
Lay the foundational infrastructure that all subsequent phases depend on: new DB tables, enhanced error types, circuit breaker pattern, and structured balance results.

### Tasks

#### 1.1 Database Migrations
Create new tables via numbered migration files.

**`tx_state` table:**
```sql
CREATE TABLE IF NOT EXISTS tx_state (
    id TEXT PRIMARY KEY,
    sweep_id TEXT NOT NULL,
    chain TEXT NOT NULL,
    token TEXT NOT NULL,
    address_index INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    amount TEXT NOT NULL,
    tx_hash TEXT,
    nonce INTEGER,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    error TEXT
);
CREATE INDEX IF NOT EXISTS idx_tx_state_status ON tx_state(status);
CREATE INDEX IF NOT EXISTS idx_tx_state_sweep ON tx_state(sweep_id, status);
CREATE INDEX IF NOT EXISTS idx_tx_state_chain ON tx_state(chain, status);
```

**`provider_health` table:**
```sql
CREATE TABLE IF NOT EXISTS provider_health (
    provider_name TEXT PRIMARY KEY,
    chain TEXT NOT NULL,
    provider_type TEXT NOT NULL DEFAULT 'scan',
    status TEXT NOT NULL DEFAULT 'healthy',
    consecutive_fails INTEGER NOT NULL DEFAULT 0,
    last_success TEXT,
    last_error TEXT,
    last_error_msg TEXT,
    circuit_state TEXT NOT NULL DEFAULT 'closed',
    updated_at TEXT NOT NULL
);
```

**Files:**
- `internal/db/migrations/005_tx_state.sql`
- `internal/db/migrations/006_provider_health.sql`
- `internal/db/tx_state.go` — CRUD for tx_state (Create, UpdateStatus, GetPending, GetBySweepID)
- `internal/db/provider_health.go` — CRUD for provider_health
- `internal/db/tx_state_test.go`
- `internal/db/provider_health_test.go`

#### 1.2 Enhanced Error Types
Add transient/permanent error classification and structured balance results.

**Files:**
- `internal/config/errors.go` — Add `TransientError` wrapper type, `IsTransient()` helper
- `internal/scanner/provider.go` — Update `BalanceResult` struct to include `Error` and `Source` fields
- `internal/config/constants.go` — Wire `ProviderMaxRetries`, `ProviderRetryBaseDelay`, add circuit breaker constants

**New constants:**
```go
// Circuit Breaker
CircuitBreakerThreshold = 3           // consecutive failures to trip
CircuitBreakerCooldown  = 30 * time.Second  // time before half-open test
CircuitBreakerHalfOpenMax = 1         // max requests in half-open state

// TX State
TxStatePending      = "pending"
TxStateBroadcasting = "broadcasting"
TxStateConfirming   = "confirming"
TxStateConfirmed    = "confirmed"
TxStateFailed       = "failed"
TxStateUncertain    = "uncertain"
```

#### 1.3 Circuit Breaker Implementation
Implement the circuit breaker pattern as a reusable component.

**Files:**
- `internal/scanner/circuit_breaker.go` — `CircuitBreaker` struct with `Allow()`, `RecordSuccess()`, `RecordFailure()`, `State()` methods
- `internal/scanner/circuit_breaker_test.go` — Tests for all state transitions (closed→open→half-open→closed)

#### 1.4 Sweep ID Generator
Add UUID generation for sweep operations (groups of TX states).

**Files:**
- `internal/tx/sweep.go` — `GenerateSweepID()` using `crypto/rand`

### Tests (Phase 1)
- tx_state CRUD: 8 tests (create, update status, get pending, get by sweep, concurrent access)
- provider_health CRUD: 6 tests (create, update, get by chain, reset)
- circuit breaker: 8 tests (closed→open on failures, open→half-open on cooldown, half-open→closed on success, half-open→open on failure, concurrent access)
- error types: 4 tests (wrap transient, wrap permanent, IsTransient, unwrap)

### Acceptance Criteria
- [ ] Migrations apply cleanly on existing V1 database
- [ ] `BalanceResult` carries optional error per address
- [ ] Circuit breaker transitions through all states correctly
- [ ] All new code has tests, `go test ./...` passes

---

## Phase 2: Scanner Resilience (Outline)

### Goal
Fix all scanner/provider issues: error collection, partial result handling, atomic state, circuit breaker wiring, Retry-After, token error visibility.

### Key Changes
- **All providers**: Replace early-return-on-error with error collection. Continue batch, annotate failed addresses.
- **Pool**: Wire circuit breaker. Return all errors (not just last). Add exponential backoff between retries.
- **Scanner**: Atomic scan state + balance updates in DB transaction. Decouple native/token failures. Surface token errors.
- **Solana RPC**: Validate `getMultipleAccounts` result count vs requested count.
- **Rate limiter**: Parse `Retry-After` header on 429 responses.
- **SSE hub**: Add `scan_state` event for client resync after reconnect.

### Files Modified
- `internal/scanner/btc_blockstream.go`, `btc_mempool.go` — Error collection
- `internal/scanner/bsc_bscscan.go`, `bsc_rpc.go` — Error collection
- `internal/scanner/sol_rpc.go` — Error collection + partial result validation
- `internal/scanner/pool.go` — Circuit breaker wiring, error aggregation, backoff
- `internal/scanner/scanner.go` — Atomic state, decoupled native/token, token error events
- `internal/scanner/ratelimiter.go` — Retry-After support
- `internal/scanner/sse.go` — Resync event

### Tests
- Per-provider error collection tests (partial batch failures)
- Pool with circuit breaker integration tests
- Scanner atomic state tests (simulate DB failure mid-scan)
- Retry-After header parsing tests

---

## Phase 3: TX Safety — Core (Outline)

### Goal
Fix the 5 most critical TX safety issues: concurrent access, BTC confirmation, SOL confirmation, in-flight persistence, SOL blockhash.

### Key Changes
- **Concurrent send mutex**: Per-chain `sync.Mutex` in send handler. Return 409 if chain is busy.
- **BTC confirmation**: Add `WaitForBTCConfirmation()` — poll mempool for TX existence, then poll for 1+ confirmation. Add to BTC broadcaster.
- **SOL confirmation fix**: Distinguish RPC errors from TX failures. Return `uncertain` status on RPC errors during polling.
- **In-flight TX persistence**: Before each address sweep, write `tx_state` row with status `pending`. Update to `broadcasting`→`confirming`→`confirmed`/`failed`/`uncertain`. Add `GET /api/send/pending` endpoint.
- **SOL blockhash refresh**: Fetch blockhash per-TX (with 20s cache to avoid unnecessary fetches).

### Files Modified
- `internal/api/handlers/send.go` — Per-chain mutex, pending TX endpoint
- `internal/tx/btc_tx.go` — Add confirmation polling
- `internal/tx/sol_tx.go` — Fix confirmation, add blockhash refresh
- `internal/tx/bsc_tx.go` — Wire tx_state persistence
- `internal/db/tx_state.go` — Used by all TX engines
- `internal/api/router.go` — New routes

### Tests
- Concurrent send mutex: HTTP test with parallel requests
- BTC confirmation polling: Mock mempool API responses
- SOL confirmation: Simulate RPC errors during polling
- TX state lifecycle: pending→broadcasting→confirmed/failed/uncertain

---

## Phase 4: TX Safety — Advanced (Outline)

### Goal
Fix remaining TX safety issues: re-validation, partial resume, idempotency, ATA confirmation.

### Key Changes
- **BTC UTXO re-validation**: Re-fetch UTXOs at execute time. Compare with preview count/total. If diverged significantly, return error with diff.
- **BSC balance recheck**: Call `BalanceAt()` before each address sweep. Skip if below minimum.
- **Partial sweep resume**: Use `tx_state` table — on resume, skip addresses with `confirmed` status, retry `failed`/`uncertain`.
- **Gas pre-seed idempotency**: Before sending gas, check if a pending TX already exists for that nonce. If so, wait for it instead of re-sending.
- **SOL ATA confirmation**: After ATA-creating TX confirms, verify ATA exists via `GetAccountInfo` before building next TX.
- **BSC gas re-estimation**: Re-fetch gas price at execute time. Apply max-increase cap (2x preview price).

### Files Modified
- `internal/tx/btc_tx.go` — UTXO diff check in Execute
- `internal/tx/btc_utxo.go` — UTXO comparison helper
- `internal/tx/bsc_tx.go` — Balance recheck, gas re-estimation
- `internal/tx/sol_tx.go` — ATA confirmation, resume logic
- `internal/tx/gas.go` — Nonce idempotency
- `internal/api/handlers/send.go` — Resume endpoint

### Tests
- UTXO divergence detection
- BSC balance recheck (balance dropped below minimum)
- Partial resume (mock 3/5 confirmed, 2 failed → resume only failed)
- Gas pre-seed idempotency (duplicate nonce detection)

---

## Phase 5: Provider Health & Broadcast Fallback (Outline)

### Goal
Add provider health visibility and broadcast redundancy.

### Key Changes
- **Health endpoint**: `GET /api/health/providers` — returns per-provider status, circuit state, last success/error.
- **Provider status tracking**: Update `provider_health` table on every provider call (success/failure). Surface in health endpoint.
- **BSC broadcast fallback**: Add Ankr public RPC as secondary BSC broadcast endpoint.
- **SOL broadcast fallback**: Add secondary Solana RPC URL for broadcast.
- **Frontend**: Add provider health indicators to scan page (green/yellow/red per provider).

### Files Modified/Created
- `internal/api/handlers/health.go` — New health provider handler
- `internal/scanner/pool.go` — Record health on every call
- `internal/db/provider_health.go` — Read/write health state
- `internal/tx/bsc_tx.go` — Add fallback RPC for broadcast
- `internal/tx/sol_tx.go` — Add fallback RPC for broadcast
- `internal/config/constants.go` — Fallback RPC URLs
- `web/src/routes/scan/+page.svelte` — Provider health indicators
- `web/src/lib/utils/api.ts` — Health API function
- `web/src/lib/types.ts` — ProviderHealth type

### Tests
- Health endpoint returns correct provider statuses
- BSC broadcast falls back to Ankr on primary failure
- SOL broadcast falls back on primary failure

---

## Phase 6: Security Tests & Infrastructure (Outline)

### Goal
Fill critical test gaps and harden server infrastructure.

### Key Changes

**Tests:**
- **Security middleware**: HostCheck (localhost/127.0.0.1/blocked hosts), CORS (allowed/blocked origins, preflight), CSRF (token generation, validation, cookie flow)
- **Scanner providers**: Mempool.space, BSC RPC, SOL RPC — with HTTP mocks for success/error/timeout/malformed responses
- **TX SSE hub**: Concurrent subscribe/broadcast/unsubscribe, buffer overflow, client cleanup

**Infrastructure:**
- **HTTP server**: Add `IdleTimeout: 5*time.Minute`, `MaxHeaderBytes: 1<<20`
- **DB pool**: Set `MaxOpenConns(25)`, `MaxIdleConns(5)`, `ConnMaxLifetime(5*time.Minute)`
- **Price fallback**: Return stale cached prices with `stale: true` when CoinGecko fetch fails
- **Graceful shutdown**: Increase timeout to match `SendExecuteTimeout`. Signal scan/SSE goroutines to stop. Close DB after drain.

### Files Modified
- `internal/api/middleware/security_test.go` — NEW: 15+ tests
- `internal/scanner/btc_mempool_test.go` — NEW: 6+ tests
- `internal/scanner/bsc_rpc_test.go` — NEW: 6+ tests
- `internal/scanner/sol_rpc_test.go` — NEW: 6+ tests
- `internal/tx/sse_test.go` — NEW: 6+ tests
- `cmd/server/main.go` — Server hardening, graceful shutdown
- `internal/db/sqlite.go` — Connection pool
- `internal/price/coingecko.go` — Stale-but-serve
- `internal/models/types.go` — PriceResponse with stale flag

### Tests
- All security middleware paths tested
- Provider mock tests with error/timeout scenarios
- SSE hub concurrent access tests
- Price service stale-but-serve test

---

## Phase Completion Checklist

After each phase:
1. `go test ./...` passes
2. `cd web && npm run build` succeeds
3. SUMMARY.md written
4. Commit with descriptive message
5. Update state.json phase status
