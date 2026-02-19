# Phase 6: Security Tests & Infrastructure

<objective>
Fill critical test gaps (security middleware, scanner providers, TX SSE hub) and harden server infrastructure (idle timeout, DB pool, price fallback, graceful shutdown).
</objective>

## Audit IDs Covered
C1, C2, C3, D1, D3, D4, D5

---

## Tasks

### Task 1: Security Middleware Tests (C1)

**File created:** `internal/api/middleware/security_test.go`

Write comprehensive tests for the three middleware functions in `internal/api/middleware/security.go`.

#### HostCheck Tests
1. **Allow localhost** — `Host: localhost` returns 200
2. **Allow 127.0.0.1** — `Host: 127.0.0.1` returns 200
3. **Allow localhost with port** — `Host: localhost:8080` returns 200
4. **Allow 127.0.0.1 with port** — `Host: 127.0.0.1:8080` returns 200
5. **Block external host** — `Host: evil.com` returns 403
6. **Block private IP** — `Host: 192.168.1.1` returns 403
7. **Block empty host** — empty `Host` header returns 403

#### CORS Tests
8. **Allow localhost origin** — `Origin: http://localhost:8080` sets `Access-Control-Allow-Origin`
9. **Allow 127.0.0.1 origin** — `Origin: http://127.0.0.1:3000` sets CORS headers
10. **Block external origin** — `Origin: http://evil.com` does NOT set `Access-Control-Allow-Origin`
11. **Block null origin** — `Origin: null` does NOT set CORS headers
12. **Block empty origin** — no `Origin` header does NOT set CORS headers
13. **Preflight OPTIONS** — returns 204 with correct `Access-Control-Allow-Methods` and `Access-Control-Allow-Headers`
14. **Non-preflight with valid origin** — `GET` with valid origin passes through to handler and includes CORS headers

#### CSRF Tests
15. **GET sets cookie** — `GET` request without cookie sets `csrf_token` cookie
16. **GET preserves existing cookie** — `GET` with existing cookie does NOT overwrite it
17. **POST with valid token** — `POST` with matching cookie+header succeeds (200)
18. **POST without cookie** — `POST` missing `csrf_token` cookie returns 403
19. **POST without header** — `POST` with cookie but no `X-CSRF-Token` header returns 403
20. **POST with mismatched token** — `POST` with cookie and different header value returns 403
21. **Token format** — `generateCSRFToken()` returns 64-char hex string (32 bytes)

**Implementation pattern:** Use `httptest.NewRecorder()` and `httptest.NewRequest()`. Wrap a simple `200 OK` handler with each middleware. Assert status codes, response headers, and cookies.

<verification>
- `go test ./internal/api/middleware/ -v -run TestHostCheck` — all 7 pass
- `go test ./internal/api/middleware/ -v -run TestCORS` — all 7 pass
- `go test ./internal/api/middleware/ -v -run TestCSRF` — all 7 pass
</verification>

---

### Task 2: Scanner Provider Tests — Mempool (C2)

**File created:** `internal/scanner/btc_mempool_test.go`

Mirror the existing `btc_blockstream_test.go` pattern — use `httptest.NewServer` to mock Mempool.space API responses.

#### Tests
1. **Success** — Valid JSON response returns correct balance (funded - spent + mempool)
2. **Rate limited (429)** — Returns transient error wrapping `ErrProviderRateLimit`
3. **Server error (500)** — Returns transient error wrapping `ErrProviderUnavailable`
4. **Malformed JSON** — Returns error (not silently returning "0")
5. **Partial failure** — 3 addresses, middle one fails: returns all 3 results with error annotation on failed one, no top-level error
6. **All fail** — All addresses return error: top-level error returned
7. **Context cancellation** — Cancelled context stops iteration immediately
8. **Token not supported** — `FetchTokenBalances()` returns `ErrTokensNotSupported`
9. **Metadata** — `Name()` returns "Mempool", `Chain()` returns BTC, `MaxBatchSize()` returns 1

**Implementation:** Create `MempoolProvider` directly with `baseURL: server.URL` pointing at httptest server, custom `http.Client`, and a fast rate limiter.

<verification>
- `go test ./internal/scanner/ -v -run TestMempoolProvider` — all 9 pass
</verification>

---

### Task 3: Scanner Provider Tests — BSC RPC (C2)

**File created:** `internal/scanner/bsc_rpc_test.go`

BSCRPCProvider uses `ethclient.Client` which communicates via JSON-RPC over HTTP. Use `httptest.NewServer` to mock the JSON-RPC endpoint, then dial with `ethclient.Dial(server.URL)`.

#### Tests
1. **Native balance success** — Mock `eth_getBalance` returns hex value, verify correct decimal string
2. **Native balance zero** — Account with 0 balance returns "0"
3. **Native all fail** — RPC error on every address, returns top-level error
4. **Token balance success** — Mock `eth_call` for `balanceOf`, returns correct decoded value
5. **Token balance — empty contract address** — Returns error immediately
6. **Token all fail** — All token fetches fail, returns top-level error
7. **Token balance — malformed response** — `eth_call` returns <32 bytes, error reported
8. **Metadata** — Name, Chain, MaxBatchSize correct

**Implementation:** The httptest server parses JSON-RPC request body, switches on `method`, and returns appropriate JSON-RPC response. `ethclient.Dial(server.URL)` connects to the mock.

<verification>
- `go test ./internal/scanner/ -v -run TestBSCRPCProvider` — all 8 pass
</verification>

---

### Task 4: Scanner Provider Tests — Solana RPC (C2)

**File created:** `internal/scanner/sol_rpc_test.go`

SolanaRPCProvider uses `http.Client` with manual JSON-RPC, same as Mempool — use httptest directly.

#### Tests
1. **Native balance success** — Mock `getMultipleAccounts` with base64 encoding, verify lamports conversion
2. **Native balance — null account** — Account doesn't exist (null in array), returns "0"
3. **Native balance — partial results** — Request 3, receive 2: third address gets error annotation "not returned by RPC"
4. **Native balance — RPC error** — JSON-RPC error response, returns `ErrProviderUnavailable`
5. **Native balance — nil result** — Missing `result` field, returns error
6. **Rate limited (429)** — Returns transient error
7. **Token balance success** — Mock with `jsonParsed` encoding, correct amount extraction
8. **Token balance — null ATA** — ATA doesn't exist, returns "0"
9. **Metadata** — Name, Chain, MaxBatchSize correct

**Implementation:** Create `SolanaRPCProvider` directly with `rpcURL: server.URL`, mock HTTP server parses JSON-RPC body and returns appropriate response.

<verification>
- `go test ./internal/scanner/ -v -run TestSolanaRPCProvider` — all 9 pass
</verification>

---

### Task 5: TX SSE Hub Tests (C3)

**File created:** `internal/tx/sse_test.go`

Test the `TxSSEHub` from `internal/tx/sse.go`.

#### Tests
1. **Subscribe returns channel** — `Subscribe()` returns non-nil channel, `ClientCount()` increments
2. **Unsubscribe removes client** — After `Unsubscribe()`, `ClientCount()` decrements, channel is closed
3. **Unsubscribe idempotent** — Calling `Unsubscribe()` twice doesn't panic
4. **Broadcast to single client** — Send event, receive on client channel
5. **Broadcast to multiple clients** — 3 clients all receive the same event
6. **Slow client drop** — Fill a client's buffer, broadcast another event: event is dropped for that client, others still receive
7. **Concurrent subscribe/broadcast** — Launch goroutines subscribing, unsubscribing, and broadcasting concurrently: no panic, no deadlock (run with `-race`)
8. **Run cancellation closes clients** — Cancel hub context, verify all client channels are closed

<verification>
- `go test ./internal/tx/ -v -run TestTxSSEHub -race` — all 8 pass
</verification>

---

### Task 6: New Constants (D1, D3)

**File modified:** `internal/config/constants.go`

Add new constants:

```go
// Server (additions)
const (
    ServerIdleTimeout    = 5 * time.Minute
    ServerMaxHeaderBytes = 1 << 20 // 1MB
)

// Database (additions)
const (
    DBMaxOpenConns    = 25
    DBMaxIdleConns    = 5
    DBConnMaxLifetime = 5 * time.Minute
)

// Price (additions)
const (
    PriceStaleTolerance = 30 * time.Minute // max age for stale-but-serve
)

// Graceful Shutdown
const (
    ShutdownTimeout = SendExecuteTimeout // match longest operation (10 min)
)
```

<verification>
- `go build ./...` — compiles cleanly
</verification>

---

### Task 7: HTTP Server Hardening (D1)

**File modified:** `cmd/server/main.go`

In `runServe()`, add `IdleTimeout` and `MaxHeaderBytes` to the `http.Server`:

```go
srv := &http.Server{
    Addr:           addr,
    Handler:        router,
    ReadTimeout:    config.ServerReadTimeout,
    WriteTimeout:   config.ServerWriteTimeout,
    IdleTimeout:    config.ServerIdleTimeout,
    MaxHeaderBytes: config.ServerMaxHeaderBytes,
}
```

Add logging for the new values.

<verification>
- `go build ./cmd/server/` — compiles
- Verify constants are used (no hardcoded values)
</verification>

---

### Task 8: DB Connection Pool (D3)

**File modified:** `internal/db/sqlite.go`

In `New()`, after `conn.Ping()` and before returning, add connection pool configuration:

```go
conn.SetMaxOpenConns(config.DBMaxOpenConns)
conn.SetMaxIdleConns(config.DBMaxIdleConns)
conn.SetConnMaxLifetime(config.DBConnMaxLifetime)

slog.Info("database connection pool configured",
    "maxOpenConns", config.DBMaxOpenConns,
    "maxIdleConns", config.DBMaxIdleConns,
    "connMaxLifetime", config.DBConnMaxLifetime,
)
```

<verification>
- `go test ./internal/db/ -v` — all existing DB tests still pass
- `go build ./...` — compiles
</verification>

---

### Task 9: Price Service Stale-But-Serve (D4)

**Files modified:**
- `internal/price/coingecko.go` — Return stale cache on fetch error
- `internal/models/types.go` — Add `PriceResponse` type with `Stale` flag
- `internal/api/handlers/dashboard.go` — Surface stale flag in API response

#### Changes to `coingecko.go`

Modify `GetPrices()`:
1. Cache hit (fresh) — return as before
2. Cache miss → fetch. If fetch succeeds → update cache, return
3. **If fetch fails AND stale cache exists** → return stale cache + `stale: true` flag
4. If fetch fails AND no cache → return error

Return type changes from `(map[string]float64, error)` to `(*PriceResult, error)` where:

```go
// PriceResult holds prices with staleness metadata.
type PriceResult struct {
    Prices map[string]float64
    Stale  bool
}
```

Add this type in `internal/price/coingecko.go` (internal to the price package).

#### Changes to dashboard handler

Update `GetPrices` and `GetPortfolio` handlers to check for staleness and include it in the response. The `GetPrices` handler returns `{"data": {"prices": {...}, "stale": false}}`. The `GetPortfolio` handler continues working with stale prices (better to show approximate portfolio than error).

#### Changes to frontend

In `web/src/lib/types.ts`, add stale flag to price response type.
In the dashboard, show a subtle "Prices may be outdated" indicator when stale is true.

#### Test

Add test in `internal/price/coingecko_test.go`:
- **Stale-but-serve** — Populate cache, make CoinGecko return 500, verify stale prices returned with `Stale: true`
- **No cache + error** — Empty cache, CoinGecko returns 500, verify error returned

<verification>
- `go test ./internal/price/ -v` — all tests pass (including new stale tests)
- `go test ./internal/api/handlers/ -v` — dashboard tests still pass
</verification>

---

### Task 10: Graceful Shutdown (D5)

**File modified:** `cmd/server/main.go`

Enhance the shutdown sequence in `runServe()`:

1. **Increase shutdown timeout** to `config.ShutdownTimeout` (10 min, matching `SendExecuteTimeout`)
2. **Cancel scan context** — The scanner uses `hubCtx` from the SSE hub. Cancel `hubCancel()` to signal scan goroutines to stop
3. **Log shutdown progress** — Log each shutdown step for observability
4. **Ordered shutdown**: signal → cancel scanner context → drain SSE hubs → HTTP server shutdown → close DB

Current code:
```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
```

New code:
```go
slog.Info("initiating graceful shutdown",
    "timeout", config.ShutdownTimeout,
)

// 1. Cancel scanner/SSE hub context — stops scans and drains SSE clients.
hubCancel()
slog.Info("scanner and SSE contexts cancelled")

// 2. Shut down HTTP server with generous timeout for in-flight sends.
ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
    return fmt.Errorf("server shutdown error: %w", err)
}

slog.Info("server stopped gracefully")
```

<verification>
- `go build ./cmd/server/` — compiles
- Review shutdown order: hub cancel → server shutdown → defer DB close
</verification>

---

## Success Criteria

- [ ] `go test ./internal/api/middleware/ -v` — 21 security middleware tests pass
- [ ] `go test ./internal/scanner/ -v -run "TestMempoolProvider|TestBSCRPCProvider|TestSolanaRPCProvider"` — 26 scanner provider tests pass
- [ ] `go test ./internal/tx/ -v -run TestTxSSEHub -race` — 8 SSE hub tests pass
- [ ] `go test ./internal/price/ -v` — stale-but-serve tests pass
- [ ] `go test ./...` — ALL existing + new tests pass
- [ ] `go build ./...` — clean compilation
- [ ] No hardcoded constants — all new values in `config/constants.go`
- [ ] Server has `IdleTimeout` and `MaxHeaderBytes` configured
- [ ] DB has connection pool limits configured
- [ ] Graceful shutdown timeout matches `SendExecuteTimeout` (10 min)
- [ ] Price service returns stale cache on fetch failure (not error)

## Estimated Tests: ~47

| Area | Count |
|------|-------|
| Security middleware (HostCheck + CORS + CSRF) | 21 |
| Mempool provider | 9 |
| BSC RPC provider | 8 |
| Solana RPC provider | 9 |
| TX SSE hub | 8 |
| Price stale-but-serve | 2 |
| **Total** | **57** |
