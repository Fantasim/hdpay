# Phase 6 Summary: Security Tests & Infrastructure

## Completed: 2026-02-19

## What Was Built

### Security Middleware Tests (C1) -- 21 tests
- 7 HostCheck tests: localhost allow, 127.0.0.1 allow, private IP block, external host block, port variations, missing host header, forwarded header ignore
- 7 CORS tests: localhost origin, 127.0.0.1 origin, external origin block, preflight with correct headers, null origin block, no origin pass-through, port variation
- 7 CSRF tests: GET bypass, POST requires token, valid token, invalid token rejection, cookie set on first GET, missing cookie rejection, token refresh on 403

### Scanner Provider Tests (C2) -- 26 tests
- Mempool Provider (9): success path, rate limit (429), server error (500), malformed JSON, partial failure, all fail, context cancellation, token not supported, metadata
- BSC RPC Provider (8): native balance success, zero balance, all fail, partial failure, token balance, null token account, context cancellation, metadata
- Solana RPC Provider (9): native balance success, null account, partial results, RPC error, nil result, rate limited, token balance, null ATA, metadata

### TX SSE Hub Tests (C3) -- 8 tests
- Subscribe/unsubscribe lifecycle, idempotent unsubscribe, single broadcast, multi-client broadcast, slow client drop, concurrent subscribe+broadcast (race-safe), Run cancellation closes clients

### Server Hardening (D1)
- Added `IdleTimeout` (5 min) and `MaxHeaderBytes` (1 MB) to http.Server config
- Server config logged at startup

### DB Connection Pool (D3)
- `SetMaxOpenConns(25)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5 min)` from centralized constants
- Pool config logged at startup

### Price Stale-but-Serve (D4)
- `GetPrices()` returns stale cache when live fetch fails (within 30-min tolerance)
- `IsStale()` method on PriceService for consumer inspection
- Dashboard handler returns `{ prices: {...}, stale: bool }` response shape
- Frontend `PriceResponse` type and API function updated
- 3 new price tests: stale cache on error, no cache returns error, expired cache returns error

### Graceful Shutdown (D5)
- Shutdown timeout uses `config.ShutdownTimeout` (10 min, matching longest send operation)
- Ordered drain: cancel hub context -> HTTP server shutdown -> DB close
- All shutdown steps logged

## Files Created/Modified

### Created
- `internal/api/middleware/security_test.go` -- 21 security middleware tests
- `internal/scanner/btc_mempool_test.go` -- 9 Mempool provider tests
- `internal/scanner/bsc_rpc_test.go` -- 8 BSC RPC provider tests
- `internal/scanner/sol_rpc_test.go` -- 9 Solana RPC provider tests
- `internal/tx/sse_test.go` -- 8 TX SSE hub tests

### Modified
- `internal/config/constants.go` -- Added 7 constants: ServerIdleTimeout, ServerMaxHeaderBytes, DBMaxOpenConns, DBMaxIdleConns, DBConnMaxLifetime, ShutdownTimeout, PriceStaleTolerance
- `cmd/server/main.go` -- IdleTimeout, MaxHeaderBytes, shutdown timeout, ordered drain, logging
- `internal/db/sqlite.go` -- Connection pool configuration from config constants
- `internal/price/coingecko.go` -- Stale cache field, stale-but-serve logic, IsStale() method
- `internal/api/handlers/dashboard.go` -- PriceResponse shape with stale field
- `internal/api/handlers/dashboard_test.go` -- Updated for new price response shape
- `internal/price/coingecko_test.go` -- 3 new stale-but-serve tests
- `web/src/lib/types.ts` -- PriceResponse interface
- `web/src/lib/utils/api.ts` -- Updated getPrices return type

## Decisions Made
- **Stale-but-serve tolerance**: 30 minutes -- serves stale cached prices if live fetch fails within this window
- **Connection pool sizing**: 25 max open / 5 idle / 5 min lifetime -- tuned for SQLite WAL mode
- **Shutdown timeout**: Matches SendExecuteTimeout (10 min) to allow in-flight sweeps to complete
- **Test approach**: httptest mock servers for all provider tests; ethclient.Dial(server.URL) for BSC RPC mocking
- **Price response shape change**: `{ prices: {...}, stale: bool }` wrapping instead of flat prices -- minimal API signature change

## Deviations from Plan
- None -- all 10 tasks implemented as planned

## Issues Encountered
- **Mempool rate limit test**: Top-level error wraps per-address errors with "all N addresses failed", so `config.IsTransient(err)` didn't match. Fixed by checking per-address error annotation string.
- **Dashboard handler test**: After changing price response shape, existing test expected flat `data["BTC"]` instead of `data["prices"]["BTC"]`. Fixed immediately.
- **Frontend type error**: After importing PriceResponse, function body still referenced old PriceData type. Fixed immediately.

## Notes for Next Phase
- All V2 build phases are now complete (6 of 6)
- Total new test count across V2: ~130 tests added
- All Go tests pass, frontend builds clean
- No V3 planning items were deferred from this phase
