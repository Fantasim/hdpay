# Phase 4: Watch Engine (Outline)

> Will be expanded into a detailed plan before building.

<objective>
Implement the watcher orchestrator that manages the full watch lifecycle: one goroutine per active watch, smart cutoff detection, transaction deduplication, confirmation tracking (PENDING→CONFIRMED with price fetch + points award), watch stop conditions, startup recovery, and graceful shutdown.
</objective>

<features>
F1 — Watch Management (create, cancel, list — watcher-side logic)
F2 — Watch Lifecycle Engine (goroutine per watch, states: ACTIVE→COMPLETED/EXPIRED/CANCELLED)
F3 — Smart Cutoff Detection (MAX(last recorded tx, START_DATE))
F4 — Poll Loop (per-chain intervals, tick processing)
F5 — Transaction Deduplication (tx_hash unique in DB, SOL composite key sig:TOKEN)
F6 — Confirmation Tracking (PENDING→CONFIRMED with chain-specific thresholds)
F25 — Recovery on Boot (expire active watches, re-check orphaned pending txs with 3 retries)
F21 — Startup & Shutdown (full sequence, graceful with 10s timeout)
</features>

<hdpay_reuse>
- Import `internal/scanner/ratelimiter` and `internal/scanner/circuit_breaker` (already used in Phase 3)
- Import `internal/models` for Chain/Token types
- Reference HDPay's scanner orchestrator pattern (`internal/scanner/scanner.go`) for goroutine management
</hdpay_reuse>

<tasks_outline>
1. Watcher orchestrator struct (map of active watches, WaitGroup, dependencies: DB, providers, pricer, calculator)
2. Watch creation logic (validate address, check duplicate via GetActiveWatchByAddress, create DB record, spawn goroutine)
3. Poll loop goroutine (ticker at chain-specific interval, detect new txs, process, re-check pending, stop conditions)
4. Smart cutoff resolution (query DB for last tx detected_at for address, fall back to config.StartDate)
5. Transaction processing pipeline (dedup check by tx_hash → insert DB → if CONFIRMED: fetch price, calculate points, update points ledger; if PENDING: calculate pending_points, update points.pending)
6. Confirmation tracking (each tick: re-check all PENDING txs via provider.CheckConfirmation, on confirmed: fetch price via pricer, calculate points, move pending→unclaimed in points table)
7. Watch stop conditions (timeout→EXPIRED, cancel→CANCELLED, all confirmed + at least one tx→COMPLETED)
8. Watch cancellation (DELETE /watch/:id → cancel context → goroutine exits → update status)
9. Startup recovery (expire all ACTIVE watches in DB, re-check all PENDING txs with 3 retries at 30s intervals, log system error if still pending)
10. Graceful shutdown (cancel all watch contexts, WaitGroup.Wait with ShutdownTimeout, expire remaining, close DB)
11. Runtime watch defaults (MaxActiveWatches, DefaultWatchTimeout loaded from config, mutable in memory)
12. Watch engine tests (lifecycle, dedup, cutoff resolution, recovery, shutdown, concurrent watches)
</tasks_outline>

<research_needed>
- Review HDPay's scanner orchestrator for goroutine lifecycle patterns
</research_needed>
