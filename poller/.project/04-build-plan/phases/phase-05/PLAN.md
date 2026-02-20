# Phase 5: API Layer (Outline)

> Will be expanded into a detailed plan before building.

<objective>
Build the full HTTP API layer: Chi router with middleware stack (IP allowlist, session auth, request logging), all REST endpoints for watches, points, admin, and dashboard, plus the health check. Response formats must match PROMPT.md's exact JSON schemas. The API should be fully curl-testable.
</objective>

<features>
F12 — Points API (GET /points, GET /points/pending, POST /points/claim with edge cases)
F17 — IP Allowlist Middleware (X-Forwarded-For, localhost/private always allowed, DB-backed cache, login+health exempt)
F18 — Session Auth (bcrypt, random 32-byte token, HttpOnly SameSite=Strict Path=/ cookie, 1h expiry, in-memory store)
F22 — Chi Router (middleware stack, CORS: Access-Control-Allow-Origin: *)
S3 — Health Check (GET /api/health, no auth, no IP check)
</features>

<hdpay_reuse>
- Import `internal/api/middleware/logging.go` — request/response logging middleware (use as-is)
- Reference HDPay's Chi router setup (`internal/api/router.go`) for middleware ordering pattern
- Import Chi from HDPay's existing go.mod dependency
</hdpay_reuse>

<tasks_outline>
1. IP allowlist middleware (extract IP from X-Forwarded-For/RemoteAddr, localhost/private always allowed, cache from DB, exempt: /api/health and /api/admin/login)
2. Session auth store (in-memory map + RWMutex, create/validate/expire, bcrypt password hash at startup)
3. Session middleware (read poller_session cookie, validate token, attach to context, 401 on invalid/expired)
4. Chi router setup (middleware order: HDPay's logging → IP allowlist → route-specific session auth)
5. CORS configuration (Access-Control-Allow-Origin: *, standard headers)
6. Standard response helpers (success JSON, error JSON with code+message, paginated response with meta)
7. Watch handlers (POST /api/watch → 201, DELETE /api/watch/:id, GET /api/watches with filters)
8. Points handlers (GET /api/points with tx details, GET /api/points/pending with confirmations info, POST /api/points/claim with edge cases: skip unknown silently, only reset unclaimed not pending)
9. Admin auth handlers (POST /api/admin/login → set cookie, POST /api/admin/logout → clear session)
10. Admin settings handlers (GET /api/admin/allowlist, POST /api/admin/allowlist, DELETE /api/admin/allowlist/:id, GET /api/admin/settings, PUT /api/admin/tiers → write tiers.json + reload calculator, PUT /api/admin/watch-defaults → update runtime values)
11. Dashboard handlers (GET /api/dashboard/stats?range=today|week|month|quarter|all with by_day array, GET /api/dashboard/transactions with pagination+filters, GET /api/dashboard/charts with all 7 datasets, GET /api/dashboard/errors → run 5 discrepancy SQL checks + list system errors + stale pending)
12. Health check handler (GET /api/health, no middleware, returns 200 with basic status)
13. HTTP status codes: 200/201/400/401/403/404/409/429/500 per PROMPT.md
14. API layer tests (middleware unit tests, handler tests with mock DB/watcher, points claim edge case tests)
</tasks_outline>

<research_needed>
- Review HDPay's middleware ordering pattern
- Confirm Chi route group middleware isolation (session auth only on /api/admin/* and /api/dashboard/*)
</research_needed>
