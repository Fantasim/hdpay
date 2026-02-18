# Phase 1: Foundation — Schema, Error Types & Circuit Breaker

## Goal
Lay the foundational infrastructure that all subsequent V2 phases depend on: new DB tables for transaction state tracking and provider health, enhanced error types with transient/permanent classification, structured balance results with error annotations, and a reusable circuit breaker pattern.

## Prerequisites
- V1 fully built and working
- All V1 tests passing

## Tasks

### Task 1: Database Migrations

**1a. `tx_state` table** (`internal/db/migrations/005_tx_state.sql`)

Tracks individual transaction state through the broadcast lifecycle. Each row = one address sweep attempt.

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
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    error TEXT
);
CREATE INDEX IF NOT EXISTS idx_tx_state_status ON tx_state(status);
CREATE INDEX IF NOT EXISTS idx_tx_state_sweep ON tx_state(sweep_id, status);
CREATE INDEX IF NOT EXISTS idx_tx_state_chain ON tx_state(chain, status);
CREATE INDEX IF NOT EXISTS idx_tx_state_nonce ON tx_state(chain, from_address, nonce);
```

Valid statuses: `pending`, `broadcasting`, `confirming`, `confirmed`, `failed`, `uncertain`

**1b. `provider_health` table** (`internal/db/migrations/006_provider_health.sql`)

Tracks per-provider health state and circuit breaker status.

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
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

Valid circuit states: `closed`, `open`, `half_open`
Valid statuses: `healthy`, `degraded`, `down`
Valid provider types: `scan`, `broadcast`

### Task 2: TX State DB Methods

**File**: `internal/db/tx_state.go`

Methods:
- `CreateTxState(tx TxStateRow) error` — Insert new pending TX
- `UpdateTxStatus(id, status, txHash, error string) error` — Update status + optional txHash + error
- `GetPendingTxStates(chain string) ([]TxStateRow, error)` — Get all pending/broadcasting/confirming/uncertain TXs for a chain
- `GetTxStatesBySweepID(sweepID string) ([]TxStateRow, error)` — Get all TXs in a sweep operation
- `GetTxStateByNonce(chain, fromAddress string, nonce int64) (*TxStateRow, error)` — Check for existing TX at nonce (idempotency)
- `CountTxStatesByStatus(sweepID string) (map[string]int, error)` — Count per-status for a sweep

**Struct**:
```go
type TxStateRow struct {
    ID           string
    SweepID      string
    Chain        string
    Token        string
    AddressIndex uint32
    FromAddress  string
    ToAddress    string
    Amount       string
    TxHash       string
    Nonce        int64
    Status       string
    CreatedAt    string
    UpdatedAt    string
    Error        string
}
```

### Task 3: Provider Health DB Methods

**File**: `internal/db/provider_health.go`

Methods:
- `UpsertProviderHealth(ph ProviderHealthRow) error` — Insert or update provider health
- `GetProviderHealth(providerName string) (*ProviderHealthRow, error)` — Get single provider
- `GetProviderHealthByChain(chain string) ([]ProviderHealthRow, error)` — Get all providers for a chain
- `GetAllProviderHealth() ([]ProviderHealthRow, error)` — Get all providers
- `RecordProviderSuccess(providerName string) error` — Reset consecutive_fails, update last_success, set status=healthy
- `RecordProviderFailure(providerName, errorMsg string) error` — Increment consecutive_fails, update last_error

**Struct**:
```go
type ProviderHealthRow struct {
    ProviderName    string
    Chain           string
    ProviderType    string
    Status          string
    ConsecutiveFails int
    LastSuccess     string
    LastError       string
    LastErrorMsg    string
    CircuitState    string
    UpdatedAt       string
}
```

### Task 4: Enhanced Error Types

**File**: `internal/config/errors.go` — Add transient error wrapper

```go
// TransientError wraps an error that should be retried
type TransientError struct {
    Err        error
    RetryAfter time.Duration // 0 = use default backoff
}

func (e *TransientError) Error() string { return e.Err.Error() }
func (e *TransientError) Unwrap() error { return e.Err }

// NewTransientError wraps an error as transient (retriable)
func NewTransientError(err error) error {
    return &TransientError{Err: err}
}

// NewTransientErrorWithRetry wraps with explicit retry delay
func NewTransientErrorWithRetry(err error, retryAfter time.Duration) error {
    return &TransientError{Err: err, RetryAfter: retryAfter}
}

// IsTransient returns true if the error is transient (retriable)
func IsTransient(err error) bool {
    var te *TransientError
    return errors.As(err, &te)
}

// GetRetryAfter returns the retry delay if set, or 0
func GetRetryAfter(err error) time.Duration {
    var te *TransientError
    if errors.As(err, &te) {
        return te.RetryAfter
    }
    return 0
}
```

Mark existing errors as transient or permanent:
- Transient: `ErrProviderRateLimit`, `ErrProviderUnavailable`, `ErrProviderTimeout`
- Permanent: `ErrInvalidMnemonic`, `ErrInvalidAddress`, `ErrInsufficientBalance`

### Task 5: Enhanced BalanceResult

**File**: `internal/scanner/provider.go` — Update BalanceResult

```go
// Before:
type BalanceResult struct {
    Address      string
    AddressIndex uint32
    Balance      string
}

// After:
type BalanceResult struct {
    Address      string
    AddressIndex uint32
    Balance      string
    Error        string // non-empty if balance is unreliable
    Source       string // provider name that returned this result
}
```

Update all provider `FetchNativeBalances` and `FetchTokenBalances` to populate `Source` field. `Error` stays empty on success (will be populated in Phase 2 when error collection is implemented).

### Task 6: Circuit Breaker

**File**: `internal/scanner/circuit_breaker.go`

```go
type CircuitBreaker struct {
    mu              sync.Mutex
    state           string // "closed", "open", "half_open"
    consecutiveFails int
    threshold       int
    cooldown        time.Duration
    lastFailure     time.Time
    halfOpenAllowed int
    halfOpenCount   int
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker

// Allow returns true if a request should be allowed through
func (cb *CircuitBreaker) Allow() bool

// RecordSuccess records a successful call — resets to closed state
func (cb *CircuitBreaker) RecordSuccess()

// RecordFailure records a failed call — may trip to open state
func (cb *CircuitBreaker) RecordFailure()

// State returns the current circuit state
func (cb *CircuitBreaker) State() string

// ConsecutiveFailures returns the current failure count
func (cb *CircuitBreaker) ConsecutiveFailures() int
```

State machine:
- **Closed** (normal): All requests pass. On failure, increment counter. If counter >= threshold → Open.
- **Open** (tripped): All requests blocked (return `ErrCircuitOpen`). After cooldown elapsed → Half-Open.
- **Half-Open** (testing): Allow 1 request. If success → Closed (reset counter). If failure → Open (restart cooldown).

### Task 7: Constants Updates

**File**: `internal/config/constants.go`

```go
// Circuit Breaker
const (
    CircuitBreakerThreshold   = 3               // consecutive failures to trip
    CircuitBreakerCooldown    = 30 * time.Second // time before half-open test
    CircuitBreakerHalfOpenMax = 1                // max requests in half-open
)

// TX State Statuses
const (
    TxStatePending      = "pending"
    TxStateBroadcasting = "broadcasting"
    TxStateConfirming   = "confirming"
    TxStateConfirmed    = "confirmed"
    TxStateFailed       = "failed"
    TxStateUncertain    = "uncertain"
)

// Provider Health Statuses
const (
    ProviderStatusHealthy  = "healthy"
    ProviderStatusDegraded = "degraded"
    ProviderStatusDown     = "down"
)

// Circuit States
const (
    CircuitClosed   = "closed"
    CircuitOpen     = "open"
    CircuitHalfOpen = "half_open"
)
```

Add new error:
```go
var ErrCircuitOpen = errors.New("circuit breaker is open")
const ERROR_CIRCUIT_OPEN = "ERROR_CIRCUIT_OPEN"
```

### Task 8: Sweep ID Generator

**File**: `internal/tx/sweep.go`

```go
func GenerateSweepID() string {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        // Fallback to timestamp-based ID
        return fmt.Sprintf("sweep-%d", time.Now().UnixNano())
    }
    return hex.EncodeToString(b)
}
```

## Tests

### `internal/db/tx_state_test.go`
1. TestCreateTxState — Insert and retrieve
2. TestUpdateTxStatus — Status transitions
3. TestUpdateTxStatusWithHash — Sets txHash on confirming
4. TestGetPendingTxStates — Filters by chain and pending statuses
5. TestGetTxStatesBySweepID — Groups by sweep
6. TestGetTxStateByNonce — Idempotency lookup
7. TestCountTxStatesByStatus — Aggregate counts
8. TestTxStateNotFound — Returns nil for missing

### `internal/db/provider_health_test.go`
1. TestUpsertProviderHealth — Insert and update
2. TestGetProviderHealthByChain — Filter by chain
3. TestGetAllProviderHealth — Returns all
4. TestRecordProviderSuccess — Resets fails, updates timestamp
5. TestRecordProviderFailure — Increments fails
6. TestProviderHealthNotFound — Returns nil for missing

### `internal/scanner/circuit_breaker_test.go`
1. TestCircuitBreaker_ClosedAllowsRequests
2. TestCircuitBreaker_OpensAfterThreshold
3. TestCircuitBreaker_OpenBlocksRequests
4. TestCircuitBreaker_HalfOpenAfterCooldown
5. TestCircuitBreaker_HalfOpenSuccessCloses
6. TestCircuitBreaker_HalfOpenFailureReopens
7. TestCircuitBreaker_SuccessResetsClosed
8. TestCircuitBreaker_ConcurrentAccess

### `internal/config/errors_test.go`
1. TestTransientError_Wrap
2. TestTransientError_IsTransient
3. TestTransientError_WithRetryAfter
4. TestPermanentError_NotTransient

## Acceptance Criteria
- [ ] Migrations 005 + 006 apply cleanly on existing V1 database
- [ ] tx_state CRUD works with all status transitions
- [ ] provider_health CRUD works with success/failure recording
- [ ] Circuit breaker transitions through all states correctly
- [ ] `BalanceResult` has `Error` and `Source` fields (populated in Phase 2)
- [ ] `IsTransient()` correctly classifies errors
- [ ] All new code has tests
- [ ] `go test ./...` passes
- [ ] No changes to existing V1 behavior (purely additive)

## Files Created
- `internal/db/migrations/005_tx_state.sql`
- `internal/db/migrations/006_provider_health.sql`
- `internal/db/tx_state.go`
- `internal/db/tx_state_test.go`
- `internal/db/provider_health.go`
- `internal/db/provider_health_test.go`
- `internal/scanner/circuit_breaker.go`
- `internal/scanner/circuit_breaker_test.go`
- `internal/tx/sweep.go`

## Files Modified
- `internal/config/constants.go` — New constants
- `internal/config/errors.go` — TransientError type, new errors
- `internal/scanner/provider.go` — BalanceResult fields
- `internal/db/sqlite.go` — Run new migrations
