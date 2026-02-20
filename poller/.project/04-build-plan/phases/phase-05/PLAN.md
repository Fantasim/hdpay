# Phase 5: API Layer

<objective>
Build the full HTTP API layer: Chi router with middleware stack (IP allowlist, session auth, request logging), all REST endpoints for watches, points, admin, and dashboard, plus the health check. Response formats match PROMPT.md's exact JSON schemas. The API should be fully curl-testable.
</objective>

<tasks>

## Task 1: Response helpers

**Create** `internal/poller/api/response.go`

Standard response functions used by all handlers:

```go
// JSON writes a success response: {"data": payload}
func JSON(w http.ResponseWriter, status int, data interface{})

// JSONList writes a paginated list response: {"data": [...], "meta": {page, pageSize, total}}
func JSONList(w http.ResponseWriter, data interface{}, page, pageSize int, total int64)

// Error writes an error response: {"error": {"code": "...", "message": "..."}}
func Error(w http.ResponseWriter, status int, code, message string)
```

All handlers use these — no inline `json.NewEncoder` calls.

**Verification**: Compiles. Unit test round-trips JSON encoding.

---

## Task 2: IP allowlist middleware

**Create** `internal/poller/api/middleware/ipallow.go`

Logic (per PROMPT.md):
1. Extract client IP from `r.RemoteAddr` (Chi's `RealIP` middleware already resolves X-Forwarded-For into RemoteAddr)
2. If IP is localhost (`127.0.0.1`, `::1`) → ALLOW
3. If IP is private network (`10.x`, `172.16-31.x`, `192.168.x`) → ALLOW
4. Look up IP in in-memory cache (`map[string]bool` protected by `sync.RWMutex`) → ALLOW if found
5. Otherwise → 403 with `ERROR_IP_NOT_ALLOWED`

The cache is loaded at startup via `db.LoadAllIPsIntoMap()` and refreshed when IPs are added/removed from the admin dashboard.

Struct:

```go
type IPAllowlist struct {
    mu    sync.RWMutex
    cache map[string]bool
}

func NewIPAllowlist(initial map[string]bool) *IPAllowlist
func (al *IPAllowlist) Middleware(next http.Handler) http.Handler
func (al *IPAllowlist) Refresh(ips map[string]bool)
func (al *IPAllowlist) IsAllowed(ip string) bool
```

Helper: `isPrivateIP(ip string) bool` — parses IP, checks against RFC 1918 ranges + loopback.

**Verification**: Unit test with localhost, private IPs, unknown IPs, allowlisted IPs, refresh behavior.

---

## Task 3: Session auth store

**Create** `internal/poller/api/middleware/session.go`

In-memory session store (lost on restart, acceptable per PROMPT.md):

```go
type Session struct {
    Token     string
    CreatedAt time.Time
    ExpiresAt time.Time
}

type SessionStore struct {
    mu       sync.RWMutex
    sessions map[string]*Session // token -> session
    passHash []byte              // bcrypt hash of admin password
    username string
}

func NewSessionStore(username, password string) (*SessionStore, error)
func (s *SessionStore) Login(username, password string) (string, error)  // returns token
func (s *SessionStore) Validate(token string) bool
func (s *SessionStore) Logout(token string)
func (s *SessionStore) Middleware(next http.Handler) http.Handler
```

- `NewSessionStore` bcrypt-hashes the plaintext password at startup
- `Login` compares username (exact match) + bcrypt.CompareHashAndPassword, generates 32-byte random token (crypto/rand, hex-encoded), stores session with 1h expiry
- `Validate` checks token exists and not expired; removes expired sessions lazily
- `Middleware` reads `poller_session` cookie → validates → 401 if invalid/expired; attaches session info to context if needed

Cookie settings: `HttpOnly; SameSite=Strict; Path=/`

**Verification**: Unit tests for login success, wrong password, wrong username, session timeout, logout, cookie middleware.

---

## Task 4: CORS middleware

**Create** `internal/poller/api/middleware/cors.go`

Per PROMPT.md: `Access-Control-Allow-Origin: *` (safe because IP allowlist is the security boundary).

```go
func CORS(next http.Handler) http.Handler
```

Sets:
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`
- Handles OPTIONS preflight with 204

**Verification**: Unit test preflight and regular requests.

---

## Task 5: Chi router setup

**Create** `internal/poller/api/router.go`

Dependencies struct passed to the router constructor:

```go
type Dependencies struct {
    DB         *pollerdb.DB
    Watcher    *watcher.Watcher
    Calculator *points.PointsCalculator
    Allowlist  *IPAllowlist
    Sessions   *SessionStore
    Config     *pollerconfig.Config
    Pricer     *points.Pricer
}

func NewRouter(deps *Dependencies) chi.Router
```

Middleware order (on root router):
1. `middleware.RealIP` (Chi built-in — resolves X-Forwarded-For)
2. `middleware.Recoverer` (Chi built-in — panic recovery)
3. HDPay's `RequestLogging` (imported from `internal/api/middleware`)
4. `CORS`

Route groups with middleware isolation:

```
/api/health                    → no IP check, no auth (mounted BEFORE IP middleware)

r.Route("/api", func(r chi.Router) {
    r.Use(ipAllowlist)         → all /api/* except exemptions

    // Exempt: login
    r.Post("/admin/login", loginHandler)

    // Watch & Points — IP-restricted only (no session)
    r.Post("/watch", createWatchHandler)
    r.Delete("/watch/{id}", cancelWatchHandler)
    r.Get("/watches", listWatchesHandler)
    r.Get("/points", getPointsHandler)
    r.Get("/points/pending", getPendingPointsHandler)
    r.Post("/points/claim", claimPointsHandler)

    // Admin & Dashboard — session required
    r.Route("/admin", func(r chi.Router) {
        r.Use(sessions.Middleware)

        r.Post("/logout", logoutHandler)
        r.Get("/allowlist", getAllowlistHandler)
        r.Post("/allowlist", addAllowlistHandler)
        r.Delete("/allowlist/{id}", removeAllowlistHandler)
        r.Get("/settings", getSettingsHandler)
        r.Put("/tiers", updateTiersHandler)
        r.Put("/watch-defaults", updateWatchDefaultsHandler)
    })

    r.Route("/dashboard", func(r chi.Router) {
        r.Use(sessions.Middleware)

        r.Get("/stats", dashboardStatsHandler)
        r.Get("/transactions", dashboardTransactionsHandler)
        r.Get("/charts", dashboardChartsHandler)
        r.Get("/errors", dashboardErrorsHandler)
    })
})
```

Note: `/api/admin/login` is inside the IP-restricted group but NOT inside the session-auth group. This matches the PROMPT.md requirement: "login and health are exempt from IP allowlist". Wait — actually re-reading: "POST /api/admin/login and GET /api/health are exempt from IP allowlist checks." So login needs to be OUTSIDE the IP-restricted group too.

**Corrected structure:**

```
// No middleware — fully open
r.Get("/api/health", healthHandler)
r.Post("/api/admin/login", loginHandler)

r.Route("/api", func(r chi.Router) {
    r.Use(ipAllowlist.Middleware)

    // Watch & Points — IP-restricted, no session
    r.Post("/watch", ...)
    r.Delete("/watch/{id}", ...)
    r.Get("/watches", ...)
    r.Get("/points", ...)
    r.Get("/points/pending", ...)
    r.Post("/points/claim", ...)

    // Admin — IP-restricted + session
    r.Route("/admin", func(r chi.Router) {
        r.Use(sessions.Middleware)
        r.Post("/logout", ...)
        r.Get("/allowlist", ...)
        ...
    })

    // Dashboard — IP-restricted + session
    r.Route("/dashboard", func(r chi.Router) {
        r.Use(sessions.Middleware)
        ...
    })
})
```

**Update** `cmd/poller/main.go` to:
- Create `IPAllowlist` from `db.LoadAllIPsIntoMap()`
- Create `SessionStore` with admin credentials from config
- Create `Dependencies` struct
- Call `api.NewRouter(deps)` instead of inline router
- Keep existing watcher/recovery/shutdown logic

**Verification**: Server starts, `/api/health` responds 200. Routes are reachable.

---

## Task 6: Health check handler

**Create** `internal/poller/api/handlers/health.go`

```
GET /api/health → 200
{
  "data": {
    "status": "ok",
    "network": "testnet",
    "active_watches": 5
  }
}
```

No auth, no IP check (mounted outside middleware groups).

**Verification**: `curl localhost:8081/api/health` returns 200.

---

## Task 7: Auth handlers (login/logout)

**Create** `internal/poller/api/handlers/auth.go`

### POST /api/admin/login
- Parse JSON body `{"username": "...", "password": "..."}`
- Call `sessions.Login(username, password)`
- On success: set `poller_session` cookie, return 200
- On failure: 401 with `ERROR_INVALID_CREDENTIALS`

### POST /api/admin/logout
- Read `poller_session` cookie
- Call `sessions.Logout(token)`
- Clear cookie (set MaxAge=-1)
- Return 200

**Verification**: Login with correct creds → cookie set. Login with bad creds → 401. Logout → cookie cleared. Subsequent request → 401.

---

## Task 8: Watch handlers

**Create** `internal/poller/api/handlers/watch.go`

### POST /api/watch
- Parse JSON body `{"chain": "BTC", "address": "bc1q...", "timeout_minutes": 30}`
- If `timeout_minutes` is 0/missing, use `watcher.DefaultWatchTimeout()`
- Validate address format via `validate.Address(chain, address, cfg.Network)`
- Call `watcher.CreateWatch(chain, address, timeoutMinutes)`
- Parse error string to determine status code:
  - Contains `ERROR_ALREADY_WATCHING` → 409
  - Contains `ERROR_MAX_WATCHES` → 429
  - Contains `ERROR_INVALID_CHAIN` → 400
  - Contains `ERROR_PROVIDER_UNAVAILABLE` → 500
  - Default → 500
- On success: 201 with watch data + `poll_interval_seconds`

### DELETE /api/watch/{id}
- Extract `{id}` from URL
- Call `watcher.CancelWatch(id)`
- Parse error:
  - Contains `ERROR_WATCH_NOT_FOUND` → 404
  - Contains `ERROR_WATCH_EXPIRED` → 409 (already terminal)
  - Default → 500
- On success: 200 with `{"data": {"watch_id": "...", "status": "CANCELLED"}}`

### GET /api/watches
- Query params: `status` (optional), `chain` (optional), `page` (default 1), `page_size` (default 50)
- Build `models.WatchFilters` from query params
- Call `db.ListWatches(filters)` (note: needs pagination support — add if missing)
- Return with pagination meta

**Verification**: Create watch via curl → 201. Cancel → 200. List → paginated results. Duplicate → 409. Max watches → 429.

---

## Task 9: Points handlers

**Create** `internal/poller/api/handlers/points.go`

### GET /api/points
- Call `db.ListWithUnclaimed()` → accounts with `unclaimed > 0`
- For each account, fetch recent confirmed transactions via `db.ListByAddress(address)` and filter to confirmed + unclaimed
- Return per PROMPT.md format with `transactions` array nested

### GET /api/points/pending
- Call `db.ListWithPending()` → accounts with `pending > 0`
- For each account, fetch PENDING transactions via `db.ListByAddress(address)` and filter to PENDING
- Add `confirmations_required` based on chain (use constants)
- Return per PROMPT.md format

### POST /api/points/claim
- Parse JSON body `{"addresses": ["bc1q...", "0xF278..."]}`
- Validate: non-empty array
- For each address:
  - Look up the points account(s) — an address may exist on multiple chains
  - Call `db.ClaimPoints(address, chain)` for each chain where unclaimed > 0
  - If unclaimed was 0: add to `skipped` list
  - If unclaimed > 0: add to `claimed` list with points_claimed amount
- Return `{"data": {"claimed": [...], "skipped": [...], "total_claimed": N}}`
- Skip unknown addresses silently (per PROMPT.md)

**Verification**: Points endpoint returns only unclaimed accounts. Claim resets unclaimed, keeps pending. Claim unknown address → skipped silently.

---

## Task 10: Admin handlers

**Create** `internal/poller/api/handlers/admin.go`

### GET /api/admin/allowlist
- Call `db.ListAllowedIPs()`
- Return `{"data": [...]}`

### POST /api/admin/allowlist
- Parse JSON body `{"ip": "1.2.3.4", "description": "game server"}`
- Validate IP format (net.ParseIP)
- Call `db.AddIP(ip, description)`
- Refresh in-memory cache: `allowlist.Refresh(db.LoadAllIPsIntoMap())`
- Return 201 with the new entry

### DELETE /api/admin/allowlist/{id}
- Parse `{id}` as int
- Call `db.RemoveIP(id)`
- Refresh cache
- Return 200

### GET /api/admin/settings
- Return current settings:
  - `max_active_watches`: from `watcher.MaxActiveWatches()`
  - `default_watch_timeout`: from `watcher.DefaultWatchTimeout()`
  - `tiers`: from `calculator.Tiers()`
  - `network`: from config
  - `start_date`: from config (formatted)
  - `active_watches`: from `watcher.ActiveCount()`

### PUT /api/admin/tiers
- Parse JSON body: array of tier objects
- Validate via `points.ValidateTiers(tiers)`
- Write to `cfg.TiersFile` (tiers.json)
- Call `calculator.Reload(tiers)`
- Return 200

### PUT /api/admin/watch-defaults
- Parse JSON body `{"max_active_watches": 200, "default_watch_timeout": 60}`
- Validate ranges (max_active ≥ 1, timeout 1-120)
- Call `watcher.SetMaxActiveWatches(n)` and/or `watcher.SetDefaultWatchTimeout(n)`
- Return 200

**Verification**: CRUD allowlist via curl. Update tiers → calculator reloads. Update watch defaults → watcher reflects new values.

---

## Task 11: Dashboard handlers

**Create** `internal/poller/api/handlers/dashboard.go`

### GET /api/dashboard/stats?range=week
- Parse `range` query param (default: "all")
- Validate: must be one of `today|week|month|quarter|all`
- Calculate date range boundaries (UTC)
- Run aggregate queries against transactions table:
  - `usd_received`: SUM(usd_value) WHERE status=CONFIRMED AND confirmed_at in range
  - `points_awarded`: SUM(points) WHERE status=CONFIRMED AND confirmed_at in range
  - `total_watches` / `watches_completed` / `watches_expired`: COUNT from watches table in range
  - `active_watches`: from `watcher.ActiveCount()`
  - `unique_addresses`: COUNT(DISTINCT address) from transactions in range
  - `avg_tx_usd`: AVG(usd_value) WHERE status=CONFIRMED in range
  - `largest_tx_usd`: MAX(usd_value) WHERE status=CONFIRMED in range
  - `pending_points`: from points table (not time-filtered — global state)
  - `by_day`: GROUP BY date(confirmed_at) with daily usd/points/txs
- Return per PROMPT.md schema

**New DB methods needed** (add to `pollerdb`):
- `func (d *DB) DashboardStats(dateFrom, dateTo string) (*DashboardStatsResult, error)` — single method with aggregated queries
- `func (d *DB) DailyStats(dateFrom, dateTo string) ([]DailyStatRow, error)` — by_day array
- `func (d *DB) WatchStats(dateFrom, dateTo string) (*WatchStatsResult, error)` — watch counts by status in range
- `func (d *DB) PendingPointsSummary() (accounts int, total int, error)` — global pending

### GET /api/dashboard/transactions
- Query params: page, page_size, chain, token, status, tier, min_usd, max_usd, date_from, date_to
- Build `models.TransactionFilters` + `models.Pagination`
- Call `db.ListAll(filters, pagination)` (already exists — returns []Transaction, total, error)
- Return with pagination meta

### GET /api/dashboard/charts
- Shares the same `range` param as stats (but affects all chart data)
- Compute 7 datasets:
  - `usd_over_time`: daily SUM(usd_value) grouped by date
  - `points_over_time`: daily SUM(points)
  - `tx_count_over_time`: daily COUNT(*)
  - `by_chain`: aggregate per chain
  - `by_token`: aggregate per token
  - `by_tier`: aggregate per tier
  - `watches_over_time`: daily watch counts by status

**New DB methods needed**:
- `func (d *DB) ChartByChain(dateFrom, dateTo string) ([]ChainBreakdown, error)`
- `func (d *DB) ChartByToken(dateFrom, dateTo string) ([]TokenBreakdown, error)`
- `func (d *DB) ChartByTier(dateFrom, dateTo string) ([]TierBreakdown, error)`
- `func (d *DB) ChartWatchesByDay(dateFrom, dateTo string) ([]DailyWatchStat, error)`

### GET /api/dashboard/errors
- Run 5 discrepancy checks (SQL from PROMPT.md):
  1. Points sum mismatch (SUM tx points vs stored total)
  2. Unclaimed exceeds total
  3. Orphaned transactions (no matching watch)
  4. Stale pending txs (>24h)
  5. Unresolved system errors by category
- Return `{"data": {"discrepancies": [...], "errors": [...], "stale_pending": [...]}}`

**New DB methods needed**:
- `func (d *DB) CheckPointsMismatch() ([]DiscrepancyRow, error)`
- `func (d *DB) CheckUnclaimedExceedsTotal() ([]DiscrepancyRow, error)`
- `func (d *DB) CheckOrphanedTransactions() ([]DiscrepancyRow, error)`
- `func (d *DB) CheckStalePending() ([]StalePendingRow, error)`

**Verification**: Stats endpoint returns correct aggregates. Charts return 7 datasets. Errors endpoint runs all 5 checks.

---

## Task 12: New DB methods + models

**Modify** `internal/poller/pollerdb/` — add new files:

- `internal/poller/pollerdb/dashboard.go` — all dashboard aggregate queries
- `internal/poller/pollerdb/discrepancy.go` — all 5 discrepancy check queries

**Modify** `internal/poller/models/types.go` — add result types:

```go
type DashboardStatsResult struct { ... }
type DailyStatRow struct { Date string; USD float64; Points int; TxCount int }
type WatchStatsResult struct { Total, Completed, Expired, Cancelled int }
type ChainBreakdown struct { Chain string; USD float64; Count int }
type TokenBreakdown struct { Token string; USD float64; Count int }
type TierBreakdown struct { Tier int; Count int; TotalPoints int }
type DailyWatchStat struct { Date string; Active, Completed, Expired int }
type DiscrepancyRow struct { Type, Address, Chain, Message string; Calculated, Stored int }
type StalePendingRow struct { TxHash, Chain, Address, DetectedAt string; HoursPending float64 }
```

**Verification**: Each DB method returns correct results against test data.

---

## Task 13: Update main.go

**Modify** `cmd/poller/main.go`:
- Import the new `api` package
- Create `IPAllowlist` from DB cache
- Create `SessionStore` with admin credentials
- Build `api.Dependencies`
- Replace inline router with `api.NewRouter(deps)`
- Remove the inline health handler (now in the router)

**Verification**: `make build && ./bin/poller` starts. Health endpoint works. Login works.

---

## Task 14: Add new constants

**Modify** `internal/poller/config/constants.go`:
- Add `CORSAllowedMethods`, `CORSAllowedHeaders` strings
- Add `AdminCookieMaxAge` (for cookie expiry alignment)
- Add error category constants for discrepancies: `ErrorCategoryPoints`, `ErrorCategoryDiscrepancy`, `ErrorCategoryRecovery`, `ErrorCategoryPrice`

**Modify** `internal/poller/config/errors.go`:
- Verify all error codes used by handlers exist (all present from Phase 1)

**Verification**: Constants compile and are referenced in new code.

---

## Task 15: API layer tests

**Create** `internal/poller/api/middleware/ipallow_test.go`:
- TestLocalhost → allowed
- TestPrivateIPs → allowed (10.x, 172.16.x, 192.168.x)
- TestUnknownIP → 403
- TestAllowlistedIP → allowed
- TestRefresh → new IP becomes allowed

**Create** `internal/poller/api/middleware/session_test.go`:
- TestLoginSuccess
- TestLoginWrongPassword → error
- TestLoginWrongUsername → error
- TestSessionExpiry → validate returns false
- TestLogout → token invalidated
- TestMiddleware401 → missing cookie
- TestMiddlewareValid → passes through

**Create** `internal/poller/api/handlers/handlers_test.go`:
- Integration-style tests: create a test router with mock/real DB
- TestCreateWatch → 201
- TestCreateWatchDuplicate → 409
- TestCancelWatch → 200
- TestCancelWatchNotFound → 404
- TestListWatches → paginated
- TestGetPoints → unclaimed only
- TestClaimPoints → resets unclaimed, skips unknown
- TestClaimPointsEdgeCases → pending unaffected
- TestLogin → cookie set
- TestLogout → cookie cleared
- TestDashboardStats → correct aggregates
- TestDashboardErrors → discrepancy checks
- TestHealthNoAuth → 200 without any cookies

**Verification**: `go test ./internal/poller/api/... -v` — all pass. Coverage ≥ 70%.

</tasks>

<success_criteria>
1. Server starts cleanly with all middleware and routes registered
2. `GET /api/health` returns 200 with no auth required
3. `POST /api/admin/login` with correct credentials returns session cookie
4. `POST /api/admin/login` with wrong credentials returns 401
5. All `/api/*` endpoints (except health and login) reject non-allowlisted IPs with 403
6. All `/api/admin/*` and `/api/dashboard/*` endpoints require valid session cookie (401 without)
7. `POST /api/watch` creates a watch and returns 201
8. `DELETE /api/watch/:id` cancels a watch and returns 200
9. `GET /api/points` returns accounts with unclaimed > 0 (with tx details)
10. `POST /api/points/claim` resets unclaimed, skips unknown addresses
11. Dashboard stats return correct aggregates for different time ranges
12. Dashboard errors run all 5 discrepancy checks
13. All response formats match PROMPT.md's exact JSON schemas
14. All tests pass with ≥ 70% coverage on api/ package
15. No hardcoded values — all constants from config package
</success_criteria>
