# Phase 4: Watch Engine

<objective>
Implement the watcher orchestrator that manages the full watch lifecycle: one goroutine per active watch, smart cutoff detection, transaction deduplication, confirmation tracking (PENDING->CONFIRMED with price fetch + points award), watch stop conditions, startup recovery, and graceful shutdown.
</objective>

<tasks>

## Task 1: Watcher Orchestrator Struct + Start/Stop

**File**: `internal/poller/watcher/watcher.go`

Create the `Watcher` struct — the central orchestrator for all watch activity.

```go
type Watcher struct {
    db         *pollerdb.DB
    providers  map[string]*provider.ProviderSet  // keyed by chain: "BTC", "BSC", "SOL"
    pricer     *points.Pricer
    calculator *points.PointsCalculator
    cfg        *config.Config

    mu      sync.Mutex
    watches map[string]context.CancelFunc  // watchID -> cancel
    wg      sync.WaitGroup                 // tracks active goroutines for graceful shutdown

    // Runtime-mutable settings (loaded from config, editable from dashboard)
    settingsMu          sync.RWMutex
    maxActiveWatches    int
    defaultWatchTimeout int  // minutes
}
```

Functions:
- `NewWatcher(db, providers, pricer, calculator, cfg) *Watcher` — constructor, logs creation
- `ActiveCount() int` — returns number of active watch goroutines (reads mu-protected map)
- `MaxActiveWatches() int` / `SetMaxActiveWatches(n int)` — get/set runtime setting
- `DefaultWatchTimeout() int` / `SetDefaultWatchTimeout(n int)` — get/set runtime setting
- `Stop()` — graceful shutdown: cancel all watch contexts, WaitGroup.Wait with ShutdownTimeout, expire remaining active watches in DB

**Design decisions** (from HDPay scanner research):
- Use `context.Background()` with per-watch timeout — NOT request context
- Store `context.CancelFunc` in map, keyed by watch ID
- Use `sync.WaitGroup` (unlike HDPay scanner) because we have multiple concurrent goroutines that must all finish before shutdown completes
- `Stop()` order: (1) cancel all contexts, (2) wait with timeout, (3) expire remaining in DB

<verification>
- `NewWatcher` compiles with all dependencies
- `Stop()` handles empty watcher (no active watches)
- Settings getters/setters are thread-safe
</verification>

---

## Task 2: Watch Creation

**File**: `internal/poller/watcher/watcher.go` (additional methods)

Method `CreateWatch(chain, address string, timeoutMinutes int) (*models.Watch, error)`:

1. Validate chain is supported (`BTC`, `BSC`, `SOL`)
2. Check provider set exists for chain
3. Check active watch limit (`ActiveCount() >= maxActiveWatches` -> return `ErrorMaxWatches`)
4. Check duplicate: `db.GetActiveWatchByAddress(address)` — if exists, return `ErrorAlreadyWatching`
5. Generate watch ID: `uuid.New().String()` (add `github.com/google/uuid` — already in go.sum via hdpay deps)
6. Calculate `started_at` (now) and `expires_at` (now + timeoutMinutes)
7. Create `models.Watch` struct, insert via `db.CreateWatch()`
8. Spawn poll goroutine via `go w.runWatch(watchCtx, cancel, watch)`
9. Store cancel func in `w.watches[watch.ID]`
10. Return the watch

**Important**: Create context from `context.Background()` with timeout = `timeoutMinutes * time.Minute`. The context timeout is the authoritative stop signal for the goroutine.

<verification>
- Creating a watch returns a valid Watch struct with correct fields
- Duplicate address returns ErrorAlreadyWatching
- Exceeding max watches returns ErrorMaxWatches
- Unknown chain returns ErrorInvalidChain
</verification>

---

## Task 3: Watch Cancellation

**File**: `internal/poller/watcher/watcher.go` (additional method)

Method `CancelWatch(watchID string) error`:

1. Lock `mu`, check if watchID exists in `watches` map
2. If not in map: check DB — if status != ACTIVE, return appropriate error (not found or already stopped)
3. Call cancel func (triggers context cancellation in goroutine)
4. The goroutine's cleanup path handles the DB status update to CANCELLED (see Task 5)
5. Return nil

<verification>
- Cancelling an active watch triggers goroutine shutdown
- Cancelling a non-existent watch returns error
- Cancelling an already-completed watch returns error
</verification>

---

## Task 4: Smart Cutoff Resolution

**File**: `internal/poller/watcher/poll.go`

Function `resolveCutoff(db *pollerdb.DB, address string, startDate int64) int64`:

1. Query `db.LastDetectedAt(address)` — gets the latest `detected_at` timestamp for this address across all past watches
2. If a result exists: parse it to unix timestamp, return `MAX(lastDetected, startDate)`
3. If no previous transactions: return `startDate` (from config)

This ensures:
- First-ever watch for an address scans from `POLLER_START_DATE`
- Subsequent watches only look for txs newer than the last known one

<verification>
- Returns startDate when no prior transactions exist
- Returns max(lastDetected, startDate) when prior txs exist
- Handles edge case: lastDetected < startDate (returns startDate)
</verification>

---

## Task 5: Poll Loop Goroutine

**File**: `internal/poller/watcher/poll.go`

Method `(w *Watcher) runWatch(ctx context.Context, cancel context.CancelFunc, watch *models.Watch)`:

This is the core loop — one goroutine per active watch.

```
defer w.wg.Done()
defer w.removeWatch(watch.ID)  // remove from active map LAST

1. Resolve cutoff (Task 4)
2. Determine poll interval from chain (PollIntervalBTC/BSC/SOL)
3. Create ticker
4. Log "watch goroutine started"

MAIN LOOP:
for {
    select {
    case <-ctx.Done():
        // Context cancelled (timeout expired or manual cancel)
        // Determine reason: ctx.Err() == context.DeadlineExceeded -> EXPIRED, else CANCELLED
        // Update DB status
        // Log, return
    case <-ticker.C:
        // === TICK PROCESSING ===

        A. Fetch new transactions
           - ps.ExecuteFetch(ctx, address, cutoff)
           - On error: log to system_errors, update poll result, continue (never stop a watch for provider failure)
           - On success: process each RawTransaction

        B. Process each new transaction (Task 6)
           - Dedup check: db.GetByTxHash(txHash)
           - If exists: skip (already recorded)
           - If new: insert into DB, handle points (Task 6)

        C. Re-check pending transactions (Task 7)
           - For each PENDING tx in DB for this watch: check confirmation

        D. Update watch poll metadata
           - Increment poll_count
           - Update last_poll_at, last_poll_result

        E. Check stop conditions (Task 8)
           - If watch expired (time.Now() > expiresAt) -> EXPIRED
           - If all txs confirmed AND at least 1 tx -> COMPLETED

        F. Update cutoff to most recent tx blocktime (so next tick only finds newer txs)
    }
}
```

Helper method `removeWatch(watchID string)`:
- Lock mu, delete from watches map, unlock
- Log removal

<verification>
- Goroutine starts and ticks at correct interval per chain
- Context cancellation causes clean exit with correct status
- Provider failures are logged but don't stop the watch
- Poll count increments each tick
</verification>

---

## Task 6: Transaction Processing Pipeline

**File**: `internal/poller/watcher/poll.go` (method on Watcher)

Method `processTransaction(ctx context.Context, watch *models.Watch, raw provider.RawTransaction) error`:

1. **Dedup check**: `db.GetByTxHash(raw.TxHash)` — if exists, return nil (skip silently)

2. **Ensure points account**: `db.GetOrCreatePoints(watch.Address, watch.Chain)`

3. **Determine status and points**:
   - If `raw.Confirmed`:
     - Fetch price: `pricer.GetTokenPrice(ctx, raw.Token)`
     - If price fetch fails: insert as PENDING (will be confirmed later with price)
     - If price succeeds: calculate `usdValue = amountHuman * price`, then `calculator.Calculate(usdValue)` -> tier, multiplier, points
     - Insert tx as CONFIRMED with all price/points data
     - `db.AddUnclaimed(address, chain, points)` — add to points ledger
   - If NOT `raw.Confirmed`:
     - Insert tx as PENDING (no price, no points yet — will be filled on confirmation)
     - Optionally estimate pending points: fetch price, calculate, store as pending
     - `db.AddPending(address, chain, estimatedPoints)` — add to pending ledger

4. **Insert transaction**: `db.InsertTransaction(tx)` with all fields mapped from RawTransaction + calculations

**Amount parsing**: `raw.AmountHuman` is already a human-readable string (e.g., "0.001"). Parse to float64 with `strconv.ParseFloat`.

<verification>
- Duplicate tx_hash is skipped without error
- Confirmed tx: price fetched, points calculated, added to unclaimed
- Pending tx: inserted with PENDING status, pending points estimated
- Price fetch failure on confirmed tx: falls back to PENDING
</verification>

---

## Task 7: Confirmation Tracking

**File**: `internal/poller/watcher/poll.go` (method on Watcher)

Method `recheckPending(ctx context.Context, watch *models.Watch) error`:

1. Query pending transactions for this watch: `db.ListPendingByWatchID(watch.ID)` (new DB method needed)
2. For each pending tx:
   a. Call `providerSet.ExecuteConfirmation(ctx, tx.TxHash, tx.BlockNumber)`
   b. If error: log warning, continue to next tx (don't fail the whole batch)
   c. If confirmed:
      - Fetch price: `pricer.GetTokenPrice(ctx, tx.Token)`
      - If price fails: log warning, leave PENDING (retry next tick)
      - Calculate points: `calculator.Calculate(usdValue)`
      - Update tx in DB: `db.UpdateToConfirmed(txHash, confirmations, blockNumber, confirmedAt, usdValue, price, tier, multiplier, points)`
      - Move points: `db.MovePendingToUnclaimed(address, chain, oldPendingPoints, newConfirmedPoints)`
   d. If not yet confirmed: log debug, continue

**New DB method needed**: `ListPendingByWatchID(watchID string) ([]Transaction, error)` in `pollerdb/transactions.go`

**SOL tx_hash handling**: SOL uses composite tx_hash like `"sig:SOL"`. The `extractBaseSignature` function from `provider/sol.go` should be used when calling `CheckConfirmation`. Add a helper: if chain is SOL, extract base signature from composite hash.

<verification>
- Pending txs get re-checked each tick
- Confirmed tx: price fetched, points calculated, status updated, points moved
- Price failure leaves tx PENDING for next tick
- Provider failure on one tx doesn't block others
</verification>

---

## Task 8: Watch Stop Conditions

**File**: `internal/poller/watcher/poll.go` (within the main loop)

After each tick's processing, check stop conditions:

1. **Timeout/Expiry**: `time.Now().After(expiresAt)` -> set status EXPIRED, cancel context
   - Note: context.WithTimeout should also handle this, but explicit check is belt-and-suspenders

2. **Completion**: All conditions must be true:
   - At least 1 transaction detected for this watch
   - All transactions are CONFIRMED (no PENDING remaining)
   - Then: set status COMPLETED, cancel context

3. **Already cancelled**: Handled by `ctx.Done()` select case

Helper method `checkStopConditions(watch, txCount, pendingCount int) (shouldStop bool, newStatus WatchStatus)`:
- Returns `(true, EXPIRED)` if past expiry
- Returns `(true, COMPLETED)` if txCount > 0 && pendingCount == 0
- Returns `(false, "")` otherwise

When stopping:
- Update DB: `db.UpdateWatchStatus(watch.ID, newStatus, &completedAt)`
- Log the reason
- Cancel context (return from goroutine)

<verification>
- Watch expires when past timeout
- Watch completes when all txs confirmed and at least one exists
- Watch with 0 txs never auto-completes (only expires or is cancelled)
</verification>

---

## Task 9: Startup Recovery

**File**: `internal/poller/watcher/recovery.go`

Method `(w *Watcher) RunRecovery(ctx context.Context) error`:

Called once at startup, before accepting new watches.

1. **Expire active watches**: `db.ExpireAllActiveWatches()` — marks any ACTIVE watches from a previous crash as EXPIRED
   - Log count of expired watches

2. **Re-check orphaned pending transactions**: `db.ListPending()` — get ALL pending txs globally
   - For each pending tx, retry confirmation check up to `RecoveryPendingRetries` (3) times
   - Between retries: sleep `RecoveryPendingInterval` (30s) with ctx.Done() check
   - If confirmed after retries: update tx, move points
   - If still pending after all retries: log system error via `db.InsertError(ErrorSeverityWarn, ErrorCategoryWatcher, message, details)`

**Important**: Recovery runs synchronously (blocks startup). It must complete before the watcher accepts new watches. Use the provided context for cancellation.

<verification>
- All ACTIVE watches from previous run are expired
- Pending transactions are re-checked with retries
- Unresolvable pending txs are logged as system errors
- Recovery respects context cancellation
</verification>

---

## Task 10: main.go Integration

**File**: `cmd/poller/main.go`

Update the startup sequence to wire everything together:

```
1. Load config (existing)
2. Init logging (existing)
3. Open DB + migrations (existing)
4. Load tiers (existing tier loading logic from Phase 2)
5. NEW: Init PriceService (HDPay's price.PriceService)
6. NEW: Init Pricer (points.NewPricer)
7. NEW: Init PointsCalculator (points.NewPointsCalculator)
8. NEW: Init ProviderSets (one per chain, using config for API keys + network mode)
9. NEW: Init Watcher (watcher.NewWatcher)
10. NEW: Run recovery (watcher.RunRecovery)
11. Setup router (existing + future API endpoints)
12. Start HTTP server (existing)
13. Wait for shutdown signal (existing)
14. NEW: Stop watcher (watcher.Stop) BEFORE HTTP server shutdown
15. Shutdown HTTP server (existing)
16. Close DB (existing)
```

Provider initialization helper (in `cmd/poller/main.go` or a setup helper):
- BTC: BlockstreamProvider + MempoolProvider (testnet URLs if network=testnet)
- BSC: BscScanProvider (with API key from config, testnet if network=testnet)
- SOL: SolanaRPCProvider + HeliusProvider (devnet if network=testnet, helius API key)

<verification>
- Binary compiles and starts with all services wired
- Recovery runs before HTTP server accepts requests
- Shutdown stops watcher before closing DB
- Provider URLs match network mode (mainnet vs testnet)
</verification>

---

## Task 11: New Constants + Errors

**File**: `internal/poller/config/constants.go` (additions)

Add any missing constants discovered during implementation:
- `WatchContextGracePeriod = 5 * time.Second` — extra time added to watch context timeout beyond expiry

**File**: `internal/poller/config/errors.go` (additions)

Check for any missing error codes needed by the watcher.

---

## Task 12: Tests

**File**: `internal/poller/watcher/watcher_test.go`

Test categories:

1. **Watch lifecycle tests**:
   - Create watch -> verify DB record and goroutine started
   - Cancel watch -> verify status CANCELLED
   - Watch expires -> verify status EXPIRED
   - Watch completes -> verify status COMPLETED (all txs confirmed)

2. **Dedup tests**:
   - Same tx_hash processed twice -> only one DB record
   - SOL composite tx_hash dedup (sig:SOL vs sig:USDC are different)

3. **Cutoff resolution tests**:
   - No prior txs -> returns config.StartDate
   - Prior txs exist -> returns max(lastDetected, startDate)

4. **Transaction processing tests**:
   - Confirmed tx -> price fetched, points calculated, unclaimed updated
   - Pending tx -> inserted with PENDING, pending points estimated
   - Price fetch failure -> tx stays PENDING

5. **Confirmation tracking tests**:
   - PENDING tx confirmed -> status updated, points moved
   - PENDING tx still pending -> no change
   - Provider failure during confirmation -> tx stays PENDING

6. **Recovery tests**:
   - Active watches expired on startup
   - Pending txs re-checked with retries
   - Unresolvable pending -> system error logged

7. **Concurrent watch tests**:
   - Multiple watches on different chains run simultaneously
   - Max active watches limit enforced

8. **Shutdown tests**:
   - Graceful stop cancels all watches
   - WaitGroup wait with timeout

**Testing approach**: Use a real SQLite DB (in-memory or temp file). Mock the providers with a `MockProvider` that implements the `Provider` interface. Mock the pricer by creating a test PriceService with predetermined responses.

<verification>
- All tests pass
- Coverage >= 70% on watcher package
- No race conditions (run with -race flag)
</verification>

</tasks>

<success_criteria>
- Watcher can create watches, poll for transactions, and detect new txs
- Transactions are deduplicated by tx_hash
- Pending transactions are re-checked each tick and confirmed when chain confirms
- Confirmed transactions trigger price fetch + points calculation + points ledger update
- Watches stop correctly: EXPIRED (timeout), CANCELLED (manual), COMPLETED (all confirmed)
- Startup recovery expires stale watches and re-checks orphaned pending txs
- Graceful shutdown cancels all watches and waits for cleanup
- main.go wires everything together and binary starts successfully
- Tests pass with >= 70% coverage, no race conditions
</success_criteria>

<dependencies>
- Phase 1: DB, config, models (complete)
- Phase 2: PointsCalculator, Pricer, address validation (complete)
- Phase 3: Provider interface, ProviderSet, BTC/BSC/SOL providers (complete)
</dependencies>

<new_db_methods>
- `ListPendingByWatchID(watchID string) ([]Transaction, error)` — needed for per-watch pending tx re-check
- `CountByWatchID(watchID string) (total int, pending int, error)` — needed for stop condition checks
</new_db_methods>

<estimated_effort>
~2 sessions. This is the most complex phase — the watch engine is the core of the entire Poller product.
</estimated_effort>
