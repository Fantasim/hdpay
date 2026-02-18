# Phase 6: Security Tests & Infrastructure (Outline)

> Expanded to detailed plan when Phase 5 is complete.

## Goal
Fill critical test gaps (security middleware, scanner providers, TX SSE hub) and harden server infrastructure (idle timeout, DB pool, price fallback, graceful shutdown).

## Audit IDs Covered
C1, C2, C3, D1, D3, D4, D5

## Key Changes

### Security Middleware Tests (C1)
Full test coverage for `internal/api/middleware/security.go`:

**HostCheck tests:**
- Allow `localhost`, `127.0.0.1`, `localhost:8080`, `127.0.0.1:8080`
- Block `evil.com`, `192.168.1.1`, empty host
- Block DNS rebinding attempts

**CORS tests:**
- Allow `http://localhost:8080`, `http://127.0.0.1:8080`
- Block `http://evil.com`, `null` origin
- Preflight OPTIONS returns correct headers
- Non-preflight requests get CORS headers

**CSRF tests:**
- Token generation returns non-empty hex string
- Token validation succeeds with matching token
- Token validation fails with missing/wrong token
- Cookie set correctly (SameSite=Strict, HttpOnly)

### Scanner Provider Tests (C2)
HTTP mock tests for untested providers:

**MempoolProvider (`btc_mempool.go`):**
- Success: Returns correct balance from valid JSON
- HTTP 429: Returns `ErrProviderRateLimit`
- HTTP 500: Returns `ErrProviderUnavailable`
- Malformed JSON: Returns error (not "0")
- Timeout: Returns error

**BSCRPCProvider (`bsc_rpc.go`):**
- Native balance: Correct hex parsing
- Token balance: Correct ABI decoding
- RPC error: Returns error
- Empty response: Returns error (not "0")

**SolanaRPCProvider (`sol_rpc.go`):**
- Native balance: Correct from `getMultipleAccounts`
- Token balance: Correct from parsed account data
- Partial results: Detected and logged
- RPC error: Returns error

### TX SSE Hub Tests (C3)
- Subscribe/unsubscribe lifecycle
- Broadcast to multiple clients
- Slow client event drop (non-blocking)
- Concurrent subscribe + broadcast
- Client cleanup after disconnect

### HTTP Server Hardening (D1)
```go
srv := &http.Server{
    Addr:           addr,
    Handler:        router,
    ReadTimeout:    config.ServerReadTimeout,
    WriteTimeout:   config.ServerWriteTimeout,
    IdleTimeout:    config.ServerIdleTimeout,     // NEW: 5 * time.Minute
    MaxHeaderBytes: config.ServerMaxHeaderBytes,   // NEW: 1 << 20 (1MB)
}
```

### DB Connection Pool (D3)
After `sql.Open()`:
```go
conn.SetMaxOpenConns(config.DBMaxOpenConns)         // 25
conn.SetMaxIdleConns(config.DBMaxIdleConns)         // 5
conn.SetConnMaxLifetime(config.DBConnMaxLifetime)   // 5 * time.Minute
```

### Price Service Stale-But-Serve (D4)
When CoinGecko fetch fails:
- If cache exists (even stale) → return with `stale: true` flag
- If no cache → return error as before
- Frontend shows "Prices may be outdated" indicator

### Graceful Shutdown (D5)
- Increase shutdown timeout to match `SendExecuteTimeout` (10 minutes)
- Signal running scans to stop via context cancellation
- Drain SSE clients (close all channels)
- Wait for in-flight sends to complete
- Then close DB

## Files Created
- `internal/api/middleware/security_test.go` — 15+ tests
- `internal/scanner/btc_mempool_test.go` — 6+ tests
- `internal/scanner/bsc_rpc_test.go` — 6+ tests
- `internal/scanner/sol_rpc_test.go` — 6+ tests
- `internal/tx/sse_test.go` — 6+ tests

## Files Modified
- `cmd/server/main.go` — Server hardening, graceful shutdown
- `internal/db/sqlite.go` — Connection pool config
- `internal/price/coingecko.go` — Stale-but-serve
- `internal/models/types.go` — PriceResponse with stale flag
- `internal/config/constants.go` — New server/DB constants

## Estimated Tests: ~45
