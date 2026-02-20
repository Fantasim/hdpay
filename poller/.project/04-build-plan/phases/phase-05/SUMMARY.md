# Phase 5 Summary: API Layer

## Completed: 2026-02-20

## What Was Built
- Chi v5 router with layered middleware (RealIP, Recoverer, RequestLogging, CORS, IP Allowlist, Session Auth)
- IP allowlist middleware with in-memory cache, private IP bypass, and DB-backed refresh
- Session auth store with bcrypt password hashing, crypto/rand tokens, HttpOnly cookies, 1h expiry
- CORS middleware (Access-Control-Allow-Origin: *, preflight 204)
- Response helpers package (httputil): JSON envelope, paginated list, error responses
- 17 API endpoints across 5 handler groups (health, auth, watch, points, admin, dashboard)
- Dashboard DB methods: aggregate stats, daily stats, watch stats, charts (by chain/token/tier), discrepancy checks
- 21 handler integration tests + 19 middleware unit tests (40 total new tests)

## Files Created
- `internal/poller/httputil/response.go` — JSON/JSONList/Error response helpers
- `internal/poller/api/router.go` — Chi router setup, Dependencies struct, route groups
- `internal/poller/api/middleware/ipallow.go` — IP allowlist middleware with cache
- `internal/poller/api/middleware/session.go` — Session auth store + middleware
- `internal/poller/api/middleware/cors.go` — CORS middleware
- `internal/poller/api/handlers/health.go` — GET /api/health (no auth, no IP check)
- `internal/poller/api/handlers/auth.go` — POST /api/admin/login, POST /api/admin/logout
- `internal/poller/api/handlers/watch.go` — POST /api/watch, DELETE /api/watch/{id}, GET /api/watches
- `internal/poller/api/handlers/points.go` — GET /api/points, GET /api/points/pending, POST /api/points/claim
- `internal/poller/api/handlers/admin.go` — GET/POST/DELETE /api/admin/allowlist, GET /api/admin/settings, PUT /api/admin/tiers, PUT /api/admin/watch-defaults
- `internal/poller/api/handlers/dashboard.go` — GET /api/dashboard/stats, GET /api/dashboard/transactions, GET /api/dashboard/charts, GET /api/dashboard/errors
- `internal/poller/pollerdb/dashboard.go` — DashboardStats, DailyStats, WatchStats, PendingPointsSummary, ChartByChain/Token/Tier, ChartWatchesByDay
- `internal/poller/pollerdb/discrepancy.go` — CheckPointsMismatch, CheckUnclaimedExceedsTotal, CheckOrphanedTransactions, CheckStalePending
- `internal/poller/api/middleware/ipallow_test.go` — 9 tests
- `internal/poller/api/middleware/session_test.go` — 10 tests
- `internal/poller/api/handlers/handlers_test.go` — 21 integration tests

## Files Modified
- `internal/poller/models/types.go` — Added DashboardStatsResult, WatchStatsResult, DailyStatRow, ChainBreakdown, TokenBreakdown, TierBreakdown, DailyWatchStat, DiscrepancyRow, StalePendingRow, TransactionFilters, Pagination
- `cmd/poller/main.go` — Replaced inline router with pollerapi.NewRouter(deps), added IP allowlist init, session store init, Dependencies struct
- `internal/poller/config/constants.go` — Added SessionCookieName, SessionExpiry, DefaultPageSize, MaxPageSize
- `internal/poller/config/errors.go` — Added ErrorInvalidCredentials, ErrorIPNotAllowed, ErrorSessionRequired, ErrorNotFound

## Decisions Made
- **httputil package**: Response helpers placed in `internal/poller/httputil/` (not `api/`) to avoid import cycle between `api/` and `handlers/`
- **CORS: Allow-Origin: ***: IP allowlist is the security boundary, not CORS. Simplifies development and dashboard access.
- **Login IP-exempt**: POST /api/admin/login and GET /api/health bypass both IP allowlist and session auth
- **External test package**: Handler tests use `package handlers_test` to break import cycle (test imports both `api` and `handlers`)
- **Nil watcher safe**: Test setup creates a real Watcher with nil providers (ActiveCount works, no actual polling)

## Deviations from Plan
- Response helpers moved from `internal/poller/api/response.go` to `internal/poller/httputil/response.go` due to import cycle
- Handler tests use external test package (`handlers_test`) instead of internal package

## Issues Encountered
- **Import cycle**: `handlers` → `api` (response.go) → `handlers` (router.go). Resolved by creating separate `httputil` package.
- **Nil pointer panic**: Tests had nil Watcher, causing panic on `w.ActiveCount()`. Fixed by creating real Watcher with nil providers.
- **Nil slice serialization**: Go marshals `nil` slices as `null` not `[]`. Added explicit nil-to-empty-slice guards for `sysErrors` and `stalePending` in dashboard errors handler.

## Notes for Next Phase
- Frontend can target all 17 endpoints (auth, watch, points, admin, dashboard)
- Session auth uses HttpOnly cookie `poller_session` — frontend just needs to call POST /api/admin/login and the cookie is set automatically
- Dashboard stats/charts accept `?range=today|week|month|quarter|all` query parameter
- Dashboard transactions support pagination (`page`, `page_size`) and filters (`chain`, `token`, `status`, `tier`, `min_usd`, `max_usd`, `date_from`, `date_to`)
