# Poller — V1 Build Plan

> Crypto-to-Points microservice: 8 phases from empty directory to deployable single binary.

## Overview

38 features (35 must-have + 3 should-have) organized into 8 build phases. Each phase produces a working, testable increment. Backend-first approach — the entire Go backend is functional before any frontend work begins.

## Project Structure Decision

**Poller lives inside HDPay's Go module** as `cmd/poller/` with its own `internal/poller/` packages. This gives full access to HDPay's `internal/` packages (logging, config, models, scanner, price, db, middleware). Two independent binaries from one module:
- `go build ./cmd/server/` → HDPay binary
- `go build ./cmd/poller/` → Poller binary

**No separate `go.mod`** — avoids Go's `internal/` cross-module visibility restriction.

## HDPay Code Reuse Strategy

Poller imports HDPay packages directly instead of rewriting them:

| HDPay Package | What Poller uses | Adaptation needed |
|---|---|---|
| `internal/logging/` | `Setup()` — slog split by level, daily rotation | None — use as-is |
| `internal/config/constants.go` | Provider URLs, rate limits, token contracts, decimals | Extend with Poller-specific constants |
| `internal/config/errors.go` | Sentinel errors, `TransientError`, error codes | Extend with Poller-specific errors |
| `internal/models/types.go` | `Chain`, `Token`, `NetworkMode` types | Extend with Poller domain types |
| `internal/price/coingecko.go` | `PriceService` with cache + stale fallback | None — use as-is |
| `internal/scanner/ratelimiter.go` | Token bucket rate limiter | None — use as-is |
| `internal/scanner/circuit_breaker.go` | Circuit breaker for providers | None — use as-is |
| `internal/db/sqlite.go` | SQLite setup (WAL, busy_timeout, migration runner) | Own migration files |
| `internal/api/middleware/logging.go` | Request/response logging | None — use as-is |

**Poller writes its own code for**: watch engine, poll loop, tx detection algorithms, points calculator, session auth, IP allowlist, all HTTP handlers, all DB CRUD, frontend.

## Design Decisions

- **Login exempt from IP allowlist** — `/api/admin/login` and `/api/health` bypass IP checks so admins can log in from anywhere
- **Watch defaults are runtime-only** — loaded from env vars on boot, editable from dashboard but lost on restart. No settings table needed.
- **BSC transactions store `block_number`** — added to transactions table for confirmation counting without re-fetching from BscScan

## Architecture Summary

```
HDPay module (github.com/Fantasim/hdpay)
    ├── cmd/server/main.go          # HDPay binary (existing)
    ├── cmd/poller/main.go          # Poller binary (new)
    ├── internal/                   # Shared HDPay packages (reused by Poller)
    │   ├── logging/                # slog setup
    │   ├── config/                 # constants, errors
    │   ├── models/                 # Chain, Token types
    │   ├── price/                  # CoinGecko service
    │   ├── scanner/                # rate limiter, circuit breaker
    │   └── ...
    ├── internal/poller/            # Poller-specific packages (new)
    │   ├── config/                 # Poller config struct (envconfig)
    │   ├── pollerdb/               # Poller SQLite (own tables, migrations)
    │   ├── watcher/                # Watch orchestrator, poll loop
    │   ├── provider/               # Poller-specific Provider interface, BTC/BSC/SOL detection
    │   ├── points/                 # Points calculator, tiers.json
    │   ├── validate/               # Address validation
    │   ├── api/                    # Chi router, handlers, middleware
    │   │   ├── handlers/
    │   │   └── middleware/         # IP allowlist, session auth
    │   └── models/                 # Poller domain types (Watch, PointsAccount, etc.)
    └── web/poller/                 # Poller SvelteKit dashboard (new)
```

## Phase Overview

| Phase | Name | Features | Session Estimate | Key Deliverable |
|-------|------|----------|-----------------|-----------------|
| 1 | Foundation | F16, F19, F20 | 1 session | Poller scaffold, config, DB, models — compiles and runs |
| 2 | Core Services | F10, F11, F13, F24 | 1 session | Points calculator, price (reuse HDPay), address validation — all tested |
| 3 | Blockchain Providers | F7-F9, F14, F15 | 1-2 sessions | Provider interface, BTC/BSC/SOL detection, rate limiting — tested with testnet |
| 4 | Watch Engine | F1-F6, F25, F21 | 2 sessions | Full watch lifecycle: create→poll→detect→confirm→complete, recovery, shutdown |
| 5 | API Layer | F12, F17, F18, F22, S3 | 1-2 sessions | All HTTP endpoints, middleware, auth — curl-testable API |
| 6 | Frontend Setup & Auth | F23, F26, F27 | 1 session | SvelteKit scaffold, design system, layout, login page |
| 7 | Dashboard Pages | F28-F35, S1, S2 | 2-3 sessions | All 6 dashboard pages with charts, filters, editors |
| 8 | Embedding & Polish | F23 (embed) | 1 session | go:embed, SPA routing, startup sequence, integration tests |

**Total: 10-13 sessions**

## Dependency Graph

```
Phase 1 (Foundation)
    └── Phase 2 (Core Services)
        └── Phase 3 (Providers)
            └── Phase 4 (Watch Engine)
                └── Phase 5 (API Layer)
                    ├── Phase 6 (Frontend Setup)
                    │   └── Phase 7 (Dashboard Pages)
                    └── Phase 8 (Embedding & Polish)
```

Phases are strictly sequential — each depends on the previous.

## Feature-to-Phase Mapping

| Feature | Phase | Description |
|---------|-------|-------------|
| F16 | 1 | SQLite database (5 tables, WAL mode, migrations) |
| F19 | 1 | Config & validation (envconfig struct) |
| F20 | 1 | Logging (reuse HDPay's slog setup) |
| F10 | 2 | Points calculation (cents * multiplier) |
| F11 | 2 | Tier configuration (tiers.json, validation) |
| F13 | 2 | CoinGecko price fetching (reuse HDPay's PriceService) |
| F24 | 2 | Address format validation (BTC/BSC/SOL) |
| F7 | 3 | BTC detection (Blockstream/Mempool tx parsing) |
| F8 | 3 | BSC detection (BscScan txlist + tokentx) |
| F9 | 3 | SOL detection (getSignaturesForAddress + getTransaction) |
| F14 | 3 | Provider round-robin (per-chain, rate-limited) |
| F15 | 3 | Provider failure logging (system_errors table) |
| F1 | 4 | Watch management (create, cancel, list) |
| F2 | 4 | Watch lifecycle engine (goroutine per watch, states) |
| F3 | 4 | Smart cutoff detection (last tx or START_DATE) |
| F4 | 4 | Poll loop (per-chain intervals, tick processing) |
| F5 | 4 | Transaction deduplication (tx_hash unique, SOL composite) |
| F6 | 4 | Confirmation tracking (PENDING→CONFIRMED) |
| F25 | 4 | Recovery on boot (expire active, re-check pending) |
| F21 | 4 | Startup & shutdown (full sequence, graceful) |
| F12 | 5 | Points API (GET /points, /points/pending, POST /points/claim) |
| F17 | 5 | IP allowlist middleware (hot-reloadable cache) |
| F18 | 5 | Session auth (bcrypt, cookies, 1h expiry) |
| F22 | 5 | Chi router (middleware stack, CORS) |
| S3 | 5 | Health check endpoint |
| F23 | 6+8 | Single binary + embedded SPA (scaffold in 6, embed in 8) |
| F26 | 6 | Login page |
| F27 | 6 | Layout (sidebar, header, auth-gated routing) |
| F28 | 7 | Overview page (8 stats cards + 7 charts) |
| F29 | 7 | Transactions page (table + filters + pagination) |
| F30 | 7 | Watches page (table with countdown timers) |
| F31 | 7 | Pending points page |
| F32 | 7 | Errors page (discrepancies + stale + system errors) |
| F33 | 7 | Settings: tier editor |
| F34 | 7 | Settings: IP allowlist |
| F35 | 7 | Settings: system info & watch defaults |
| S1 | 7 | Discrepancy auto-detection (5 SQL checks) |
| S2 | 7 | Block explorer links |

## Testing Strategy

- **Phase 1**: DB migration tests, config validation tests
- **Phase 2**: Points calculator tests (all tier boundaries), address validation tests (all chains)
- **Phase 3**: Provider mock tests, rate limiter tests (HDPay's rate limiter already tested)
- **Phase 4**: Watch lifecycle tests, dedup tests, recovery tests
- **Phase 5**: Middleware tests (IP, session), handler tests (all endpoints), points claim edge cases
- **Phase 6-7**: Frontend component tests (Vitest)
- **Phase 8**: Integration tests (full startup → API calls → verify DB state)

Coverage target: 70% on core packages (`watcher`, `points`, `provider`, `pollerdb`, `validate`).

## API Implementation Notes

- Response formats must match PROMPT.md's exact JSON schemas (POST /watch, GET /points, etc.)
- HTTP status codes: 200, 201, 400, 401, 403, 404, 409, 429, 500 as specified
- Points claim edge cases: claim while pending exists (only reset unclaimed), new funds after claim, skip unknown addresses silently
- Session cookies: `HttpOnly; SameSite=Strict; Path=/`
- `DELETE /api/admin/allowlist/:id` — deletion by route parameter, not request body

## Conventions Reminder

- Poller-specific constants → `internal/poller/config/constants.go`
- Shared constants (provider URLs, rates, decimals) → import from `internal/config/constants.go`
- Poller-specific errors → `internal/poller/config/errors.go`
- Shared errors → import from `internal/config/errors.go`
- Poller types → `internal/poller/models/types.go`
- Shared types (Chain, Token) → import from `internal/models/types.go`
- Frontend constants → `web/poller/src/lib/constants.ts`
- Frontend types → `web/poller/src/lib/types.ts`
- Check HDPay packages before writing new code — import, don't rewrite
- Log every action (slog structured logging)
- No hardcoded values — ever
