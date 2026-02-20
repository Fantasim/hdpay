# Phase 8: Embedding & Polish (Detailed)

<objective>
Wire everything into a deployable single binary: embed the SvelteKit build via go:embed, configure SPA fallback routing, verify the full startup sequence works end-to-end, and perform final quality audits (constants, dedup, security, coverage).
</objective>

<features>
F23 — Single Binary + Embedded SPA (go:embed web/poller/build, SPA fallback, immutable cache for _app/)
</features>

<hdpay_reuse>
- Import HDPay's `internal/api/handlers.SPAHandler(fs.FS)` — proven SPA fallback handler with immutable cache headers
- Pattern: `web/embed.go` for `//go:embed all:build` directive
- Pattern: `fs.Sub()` to strip the `build/` prefix from the embedded FS
</hdpay_reuse>

<tasks>

## Task 1: Create `web/poller/embed.go`

Create the embed directive file that makes the SvelteKit build available at compile time.

**File**: `web/poller/embed.go`

```go
package poller

import "embed"

//go:embed all:build
var StaticFiles embed.FS
```

The `all:` prefix ensures `_app/` (underscore-prefixed) directories are included.

**Verification**:
- File compiles: `go build ./web/poller/`

---

## Task 2: Update Dependencies struct and router to accept static FS

Modify `internal/poller/api/router.go`:

1. Add `StaticFS io/fs.FS` field to `Dependencies` struct
2. Import HDPay's `SPAHandler` from `github.com/Fantasim/hdpay/internal/api/handlers`
3. At end of `NewRouter()`, if `deps.StaticFS != nil`, register `r.NotFound(handlers.SPAHandler(deps.StaticFS))`

**Verification**:
- Compiles with `go build ./internal/poller/api/`
- When StaticFS is nil, no SPA handler is registered (dev mode works as before)

---

## Task 3: Update `cmd/poller/main.go` to wire embedded FS

1. Import `web/poller` package (the embed package)
2. Use `fs.Sub(webpoller.StaticFiles, "build")` to strip the `build/` prefix
3. Pass the resulting `fs.FS` to `pollerapi.Dependencies.StaticFS`
4. Add logging: `slog.Info("embedded SPA loaded")`

**Verification**:
- `go build ./cmd/poller/` succeeds with the embedded SPA

---

## Task 4: Update Makefile

Add `build-poller-frontend` target and chain it into `build-poller`:

```makefile
build-poller-frontend:
	cd web/poller && npm run build

build-poller: build-poller-frontend
	$(GO) build $(LDFLAGS) -o bin/poller ./cmd/poller
```

Also add `web/poller/build` and `web/poller/.svelte-kit` to the `clean` target.

**Verification**:
- `make build-poller` builds frontend then compiles binary in one command

---

## Task 5: Build and verify binary

1. Run `make build-poller`
2. Verify binary exists at `bin/poller`
3. Check binary size is reasonable (includes embedded SPA assets)

**Verification**:
- `ls -lh bin/poller` shows the binary
- Binary is larger than a pure Go binary (SPA assets embedded)

---

## Task 6: Smoke test the full binary

1. Create a minimal `.env` file with required vars
2. Run `bin/poller` (or `make dev-poller`)
3. Verify:
   - Health endpoint: `curl http://localhost:8081/api/health`
   - SPA serves: `curl -s http://localhost:8081/ | head -5` (should contain HTML)
   - SPA fallback: `curl -s http://localhost:8081/transactions` (should contain index.html content)
   - Immutable cache: `curl -sI http://localhost:8081/_app/immutable/` (should have max-age)
   - API still works: `curl http://localhost:8081/api/watches`
   - Login works: `curl -X POST http://localhost:8081/api/admin/login -d '{"username":"admin","password":"changeme"}'`

**Verification**:
- All curl commands return expected responses
- No errors in logs

---

## Task 7: Constants audit

Grep all Poller code for hardcoded values that should be constants:

- Magic numbers (timeouts, sizes, limits)
- Hardcoded strings (URLs, error messages, paths)
- Duplicate constant definitions

Scan: `internal/poller/`, `cmd/poller/`, `web/poller/src/`

Move any findings to `internal/poller/config/constants.go` or `web/poller/src/lib/constants.ts`.

**Verification**:
- No hardcoded values remain outside constant files (excluding test fixtures)

---

## Task 8: Utility dedup audit

Scan for duplicated functions across `internal/poller/` packages:

- Similar formatting functions
- Repeated error handling patterns
- Duplicate validation logic

Extract shared code to appropriate packages.

**Verification**:
- No significant function duplication across packages

---

## Task 9: Security audit

Verify:
1. No private keys stored or logged anywhere
2. IP allowlist enforced on all API routes (except /health and /login)
3. Session cookies have HttpOnly, SameSite=Strict, Path=/
4. Admin password bcrypt-hashed in memory, plaintext only in .env
5. No SQL injection vectors (all queries use prepared statements)
6. CORS configured correctly

**Verification**:
- Grep for sensitive patterns (private key, mnemonic, password in logs)
- Review middleware chain ordering

---

## Task 10: Test coverage report

Run coverage on core packages:
```
go test -cover ./internal/poller/watcher/...
go test -cover ./internal/poller/points/...
go test -cover ./internal/poller/provider/...
go test -cover ./internal/poller/pollerdb/...
go test -cover ./internal/poller/validate/...
```

Target: >70% on each core package.

**Verification**:
- Coverage report shows ≥70% on core packages

---

## Task 11: Fix any issues found in audits

Address findings from tasks 7-10:
- Move hardcoded values to constants
- Extract duplicated code
- Fix security issues
- Add tests to reach coverage targets

**Verification**:
- All audit findings resolved

---

## Task 12: Final build and tag

1. `make clean && make build-poller`
2. Verify binary works end-to-end
3. Update CHANGELOG.md

**Verification**:
- Clean build succeeds
- Binary serves both API and SPA correctly

</tasks>

<success_criteria>
1. `make build-poller` produces a single binary with embedded SPA
2. Binary serves SvelteKit dashboard at `/` and all routes
3. SPA fallback works (unknown routes serve index.html)
4. `/_app/immutable/` assets get `Cache-Control: public, max-age=31536000, immutable`
5. All API endpoints still work correctly
6. No hardcoded constants outside central files
7. No duplicated utility functions
8. Security audit passes (no leaked secrets, middleware enforced)
9. Core package test coverage ≥70%
10. Full startup sequence (14 steps) works
</success_criteria>

<session_estimate>1 session</session_estimate>
