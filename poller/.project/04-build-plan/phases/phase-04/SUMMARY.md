# Phase 4 Summary: Watch Engine

## Completed: 2026-02-20

## What Was Built
- Watcher orchestrator: central `Watcher` struct with goroutine-per-watch model, WaitGroup for shutdown, context.Background() with per-watch timeout
- Watch creation with validation (chain, duplicate address, max limit), UUID generation, goroutine spawn
- Watch cancellation via context cancel, with goroutine cleanup handling DB status update
- Smart cutoff resolution: MAX(last recorded tx detected_at, POLLER_START_DATE)
- Poll loop goroutine with per-chain ticker intervals (BTC:60s, BSC:5s, SOL:5s)
- Transaction processing pipeline: dedup by tx_hash, confirmed = price + points + unclaimed ledger, pending = estimate pending points
- Confirmation tracking: re-check PENDING txs each tick, on confirm: price → points → MovePendingToUnclaimed
- Watch stop conditions: EXPIRED, COMPLETED (all txs confirmed + ≥1 tx), CANCELLED
- Startup recovery: expire stale ACTIVE watches, re-check PENDING txs with 3 retries at 30s intervals
- Runtime-mutable MaxActiveWatches and DefaultWatchTimeout
- main.go fully wired: config → logging → DB → tiers → PriceService → Pricer → Calculator → Providers → Watcher → Recovery → HTTP → shutdown

## Files Created/Modified
- `internal/poller/watcher/watcher.go` — Watcher struct, NewWatcher, CreateWatch, CancelWatch, Stop, ActiveCount, runtime settings
- `internal/poller/watcher/poll.go` — runWatch (poll loop), resolveCutoff, processTransaction, recheckPending, checkStopConditions, determineStopStatus, estimatePendingPoints, helpers
- `internal/poller/watcher/recovery.go` — RunRecovery (expire active, re-check pending with retries)
- `internal/poller/watcher/watcher_test.go` — 31 tests: lifecycle, creation, cancellation, dedup, SOL composite, cutoff, stop conditions, recovery, concurrent, shutdown
- `internal/poller/watcher/testhelpers_test.go` — Mock price server, test helpers
- `internal/poller/pollerdb/transactions.go` — Added ListPendingByWatchID, CountByWatchID
- `internal/poller/config/constants.go` — Added WatchContextGracePeriod, RecoveryTimeout
- `cmd/poller/main.go` — Full startup/shutdown sequence with all services wired, initProviderSets helper

## Decisions Made
- **WaitGroup for watch goroutines**: Unlike HDPay scanner (which doesn't use WaitGroup), Poller uses WaitGroup because it has multiple concurrent goroutines that must all finish before shutdown completes
- **context.Background() for watches**: Watch goroutines use context.Background() with timeout, not HTTP request context, so they survive handler return
- **Price fetch failure on confirmed tx**: Falls back to PENDING status — will be confirmed on next recheck tick
- **Recovery is synchronous**: Blocks startup until complete (expire watches + re-check pending txs)
- **Graceful shutdown order**: watcher.Stop() BEFORE HTTP server shutdown, ensuring all watch goroutines finish their DB writes
- **Stablecoin short-circuit**: USDC/USDT use $1.00 via Pricer, no CoinGecko call
- **Shared test price server**: Single httptest.Server for all test cases (init'd once in init())

## Deviations from Plan
- None — all 12 tasks completed as specified

## Issues Encountered
- **Mock price server URL mismatch**: PriceService uses `baseURL + "/simple/price"` but initial mock was registered at `/api/v3/simple/price` — fixed to match the relative path format
- **Slow recovery test**: TestRecovery_UnresolvablePending takes 60s (3 retries × 30s interval) — added `testing.Short()` skip

## Notes for Next Phase
- Phase 5 (API Layer) will use `Watcher.CreateWatch()`, `CancelWatch()`, `ActiveCount()` directly from HTTP handlers
- Runtime settings (`SetMaxActiveWatches`, `SetDefaultWatchTimeout`) will be called from settings handlers
- `Watcher` struct should be passed to API handlers as a dependency
- The `db` passed to the Watcher is the same instance used by handlers — shared SQLite connection
