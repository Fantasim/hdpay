# Plan: HDPay V2 — Robustness & Hardening

> Fix all money-loss risks, improve scanner resilience, add provider health, fill security test gaps, harden infrastructure. No new user-facing features.

## Scope Summary

HDPay V2 is a hardening release. V1 delivered a complete scan→send pipeline across 3 chains, but a robustness audit revealed 37 issues (8 CRITICAL, 13 HIGH, 12 MEDIUM, 4 LOW) across transaction safety, scanner resilience, provider health, security testing, and infrastructure. V2 fixes all of these without adding new features or changing the tech stack.

**Input**: [V1 Robustness Audit](00-robustness-audit.md) — 37 issues across 4 categories.

## Feature Tiers

### Must-Have (V2 Core — CRITICAL + HIGH fixes)

| # | Feature | Description | Complexity | Audit IDs |
|---|---------|-------------|------------|-----------|
| 1 | Concurrent send mutex | Per-chain mutex preventing simultaneous send/execute requests | S | A1 |
| 2 | BTC confirmation polling | Poll mempool after broadcast (like BSC/SOL already do), with timeout and retry | M | A2 |
| 3 | SOL confirmation error handling | Distinguish "RPC error during polling" from "TX actually failed on-chain". Return uncertain status instead of false "failed" | M | A3 |
| 4 | In-flight TX persistence | Write TX to DB with status "pending" BEFORE broadcast. On restart, surface pending TXs. Prevents double-send on crash recovery | L | A4 |
| 5 | SOL blockhash refresh per-TX | Fetch fresh blockhash before each individual TX broadcast (not once per batch). Cache for 20s to avoid unnecessary fetches | S | A5 |
| 6 | BTC UTXO re-validation at execute | Re-fetch UTXOs at execute time, compare with preview. If changed, return error with diff — don't silently proceed | M | A6 |
| 7 | BSC balance re-check at execute | Re-fetch real-time balance before each address sweep. Skip addresses whose balance dropped below minimum | S | A7 |
| 8 | Partial sweep resume | Track per-address sweep status in DB. On failure/restart, resume from last incomplete address | L | A8 |
| 9 | Gas pre-seed idempotency | Check nonce before retry. If TX with that nonce already exists on-chain, skip instead of re-sending | M | A9 |
| 10 | SOL ATA creation confirmation | After first TX creates ATA, confirm ATA exists via RPC before building subsequent TXs | S | A10 |
| 11 | Provider error collection (not early-return) | Collect errors per-address, continue batch, return partial results with error annotations | M | B1 |
| 12 | Error vs zero balance distinction | Return structured result with `{balance, error}` per address. Never silently return "0" on error | M | B2 |
| 13 | Partial RPC result validation | Compare returned count vs requested count. If fewer, re-fetch missing indices | S | B3 |
| 14 | Scan state atomic updates | Use DB transaction for scan state + balance updates. If either fails, both roll back | M | B4 |
| 15 | Circuit breaker for providers | After N consecutive failures (default 3), skip provider for cooldown period (default 30s). Auto-reset on success | M | B5 |
| 16 | Retry-After header parsing | Extract `Retry-After` from 429 responses, use as minimum wait before next attempt | S | B6 |
| 17 | Token scan failure visibility | Return token scan errors to frontend. Show "USDC scan failed" not "0 USDC" | S | B7 |
| 18 | Decouple native/token scan failures | Native balance failure should not abort token scans. Each runs independently | S | B8 |
| 19 | Security middleware tests | Full test coverage for HostCheck, CORS, CSRF middleware | M | C1 |
| 20 | BSC/SOL broadcast fallback | Add secondary RPC endpoint for TX broadcast. Ankr for BSC, backup RPC for SOL | M | D2 |
| 21 | HTTP server hardening | Add `IdleTimeout`, `MaxHeaderBytes` to http.Server | S | D1 |

### Should-Have (V2 Enhanced — MEDIUM fixes)

| # | Feature | Description | Complexity | Audit IDs |
|---|---------|-------------|------------|-----------|
| 22 | BSC gas re-estimation at execute | Re-fetch gas price at execute time with configurable max-increase cap | S | A11 |
| 23 | Transient vs permanent error classification | Error wrapper type that marks errors as retriable or not. Smart retry only on transient | M | A13 |
| 24 | Provider health endpoint | `GET /api/health/providers` — ping each provider, report status, last success, error count | M | — |
| 25 | Exponential backoff in provider pool | When all providers fail for a batch, wait with exponential backoff before next batch attempt | S | B11 |
| 26 | Pool error aggregation | Return all provider errors (not just last) when all fail | S | B9 |
| 27 | SSE client resync | Add `scan_state` event type that sends full current state. Client requests resync after reconnect | M | B10 |
| 28 | DB connection pool limits | Set `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime` | S | D3 |
| 29 | Price service stale-but-serve | When CoinGecko fetch fails, return cached prices with `stale: true` flag instead of error | S | D4 |
| 30 | Graceful shutdown for in-flight TXs | Increase shutdown timeout to match `SendExecuteTimeout`. Drain SSE clients. Close DB after all goroutines | M | D5 |
| 31 | Scanner provider tests | Add tests for Mempool, BSC RPC, SOL RPC providers with HTTP mocks | M | C2 |
| 32 | TX SSE hub tests | Test concurrent subscribe/broadcast/unsubscribe | S | C3 |

### Nice-to-Have (Defer to V3)

| # | Feature | Description | Why Defer |
|---|---------|-------------|-----------|
| 33 | BSC nonce gap detection | Detect and resolve nonce gaps from timed-out TXs | Complex nonce management, low frequency |
| 34 | BTC RBF (Replace-By-Fee) | Allow fee bumping for stuck BTC transactions | Adds TX complexity, rare scenario |
| 35 | Dead letter queue for failed TXs | Persistent queue for failed TXs with retry scheduling | Over-engineering for V2 |
| 36 | Error message sanitization | Strip internal details from API error responses | Low risk (localhost-only) |
| 37 | CSRF token generation hardening | Panic on `rand.Read` failure instead of returning empty | Low risk (crypto failure = system broken) |
| 38 | IPv6 loopback support | Allow `[::1]` in host check | Low demand |
| 39 | Request body size limits | Add `http.MaxBytesHandler` wrapper | Low risk (localhost-only) |
| 40 | Unused retry constants cleanup | Wire `ProviderMaxRetries`/`ProviderRetryBaseDelay` or remove | Cleanup, not functional |

## Explicitly NOT in V2

- **New features** — No new chains, tokens, UI pages, or user-facing functionality
- **Tech stack changes** — No library upgrades, framework changes, or new dependencies (except maybe a secondary RPC URL)
- **UI redesign** — No layout changes, theme changes, or component rewrites
- **Cloud deployment** — Still localhost-only
- **Authentication** — Still single-user, no auth

## Architecture Changes

### New: In-Flight Transaction State Machine

```
PREVIEW → PENDING → BROADCASTING → CONFIRMING → CONFIRMED/FAILED
                                                     ↓
                                                  UNCERTAIN
```

- `PENDING`: Written to DB, not yet broadcast
- `BROADCASTING`: Broadcast started, awaiting response
- `CONFIRMING`: Broadcast succeeded, polling for confirmation
- `CONFIRMED`: On-chain confirmation received
- `FAILED`: Permanent failure (insufficient funds, invalid address)
- `UNCERTAIN`: RPC error during confirmation — TX may or may not be on-chain

### New: Circuit Breaker State Machine

```
CLOSED (normal) → OPEN (tripped after N failures) → HALF_OPEN (testing)
     ↑                                                      ↓
     ← ← ← ← ← ← ← (success resets) ← ← ← ← ← ← ← ← ←
```

- Each provider gets its own circuit breaker
- Configurable: `CircuitBreakerThreshold` (3), `CircuitBreakerCooldown` (30s)
- Half-open: allow one request through after cooldown, if it succeeds → close

### Modified: Provider Pool with Health Tracking

```go
type ProviderHealth struct {
    Name           string
    Status         string    // "healthy", "degraded", "down"
    ConsecutiveFails int
    LastSuccess    time.Time
    LastError      time.Time
    LastErrorMsg   string
    CircuitState   string    // "closed", "open", "half_open"
}
```

### Modified: Balance Result with Error Annotation

```go
// Before (V1):
type BalanceResult struct {
    Address      string
    AddressIndex uint32
    Balance      string  // "0" on error — ambiguous!
}

// After (V2):
type BalanceResult struct {
    Address      string
    AddressIndex uint32
    Balance      string
    Error        string  // non-empty if balance is unreliable
    Source       string  // which provider returned this result
}
```

## Data Model Changes

### New Table: `tx_state`

```sql
CREATE TABLE tx_state (
    id TEXT PRIMARY KEY,              -- UUID
    chain TEXT NOT NULL,
    token TEXT NOT NULL,              -- "NATIVE", "USDC", "USDT"
    address_index INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    amount TEXT NOT NULL,
    tx_hash TEXT,
    status TEXT NOT NULL,             -- pending/broadcasting/confirming/confirmed/failed/uncertain
    nonce INTEGER,                    -- BSC nonce for idempotency
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    error TEXT
);
CREATE INDEX idx_tx_state_status ON tx_state(status);
CREATE INDEX idx_tx_state_chain ON tx_state(chain, status);
```

### New Table: `provider_health`

```sql
CREATE TABLE provider_health (
    provider_name TEXT PRIMARY KEY,
    chain TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'healthy',
    consecutive_fails INTEGER NOT NULL DEFAULT 0,
    last_success TEXT,
    last_error TEXT,
    last_error_msg TEXT,
    circuit_state TEXT NOT NULL DEFAULT 'closed'
);
```

## API Changes

### New Endpoints

```
GET  /api/health/providers           # Provider health status per chain
GET  /api/send/pending               # List pending/uncertain in-flight TXs
POST /api/send/retry/{id}            # Retry a failed/uncertain TX
POST /api/send/dismiss/{id}          # Mark uncertain TX as resolved
```

### Modified Endpoints

```
GET  /api/scan/status                # Now includes per-token error states
GET  /api/balances/:chain            # Now includes stale/error annotations
GET  /api/dashboard/prices           # Now returns stale prices with flag
POST /api/send/execute               # Now returns immediately with tx_state IDs, progress via SSE
```

### Modified SSE Events

```
event: tx_status
data: {"id":"uuid","chain":"BSC","status":"confirming","address":"0x...","txHash":"0x..."}

event: tx_uncertain
data: {"id":"uuid","chain":"SOL","status":"uncertain","address":"...","error":"RPC timeout"}

event: scan_token_error
data: {"chain":"BSC","token":"USDC","error":"all providers failed","scanned":0}
```

## Testing Requirements

### V2 Test Targets

| Area | Current Coverage | V2 Target |
|------|-----------------|-----------|
| Security middleware | 0% | 90%+ |
| TX engines (unit) | ~90% | 95%+ |
| TX state machine | 0% (new) | 90%+ |
| Circuit breaker | 0% (new) | 95%+ |
| Scanner providers | ~70% | 90%+ |
| Provider health | 0% (new) | 90%+ |

## User Answers Archive

### V2 Direction
Robustness audit performed by automated agent. 37 issues found. V2 focuses on: preventing money loss, improving reliability when 3rd-party providers are down or rate-limited, adding backup providers, and filling critical test gaps. No new features.
