# Phase 11: Transaction History, Settings & Deployment

> **Status: Detailed** — Ready for implementation.

<objective>
Build the transaction history page (filterable table with explorer links), settings page (network mode, scan defaults, fee config, danger zone), build script for the final single binary with embedded frontend, static file server with SPA fallback, and end-to-end integration verification.
</objective>

## Tasks

### Task 1: Transaction History API — Extended DB Queries + Handler

**Goal**: Add filtered/paginated transaction listing with direction, token, and chain filters.

**Files to modify/create**:
- `internal/db/transactions.go` — Add `ListTransactionsFiltered` with direction, token, status filters
- `internal/api/handlers/transactions.go` — New: GET `/api/transactions`, GET `/api/transactions/:chain`
- `internal/api/router.go` — Wire transaction routes

**Details**:

The existing `ListTransactions(chain, page, pageSize)` only filters by chain. Extend with a new method:

```go
type TransactionFilter struct {
    Chain     *models.Chain
    Direction *string   // "in" or "out"
    Token     *models.Token
    Status    *string   // "pending", "confirmed", "failed"
}

func (d *DB) ListTransactionsFiltered(filter TransactionFilter, page, pageSize int) ([]models.Transaction, int64, error)
```

Handler parses query params: `?chain=BTC&direction=out&token=NATIVE&status=confirmed&page=1&pageSize=20`

Two routes:
- `GET /api/transactions` — all transactions, filtered by query params
- `GET /api/transactions/{chain}` — shorthand, chain from path + additional query param filters

Both return `APIResponse` with `data: []Transaction` and `meta: {page, pageSize, total, executionTime}`.

**Verification**:
- `go test ./internal/db/...` — test filtered queries
- `go test ./internal/api/handlers/...` — test handler param parsing
- `curl localhost:8080/api/transactions?chain=BTC&page=1&pageSize=5` returns valid JSON

---

### Task 2: Transaction History Frontend

**Goal**: Build the transactions page matching the mockup — filterable table with chain badges, direction icons, explorer links, pagination.

**Files to modify/create**:
- `web/src/lib/utils/api.ts` — Add `getTransactions` function
- `web/src/lib/types.ts` — Add `TransactionFilter` interface, `TransactionListParams`
- `web/src/lib/constants.ts` — Add `DEFAULT_TX_PAGE_SIZE`, `TX_DIRECTIONS`, `TX_STATUSES`
- `web/src/routes/transactions/+page.svelte` — Replace placeholder with full implementation

**Details**:

From the mockup, the page has:
1. **Header**: "Transactions" / "View all transaction history"
2. **Filter toolbar**: Chain (All/BTC/BSC/SOL), Direction (All/Incoming/Outgoing), Token (All/Native/USDC/USDT) — using filter-chip buttons
3. **Table**: Date (primary + secondary), Chain badge, Direction (icon + label), Token, Amount (mono right-aligned), From/To (truncated mono), Tx Hash (truncated + copy button), Status badge
4. **Pagination**: "Showing X — Y of Z" + page controls

Use the existing `getExplorerTxUrl` from `chains.ts` for linking tx hashes to explorers.

The page loads transactions on mount, re-fetches on filter or page change.

**Verification**:
- Page renders with mock data (or empty state if no transactions)
- Filter chips toggle correctly and trigger re-fetch
- Pagination controls work
- Copy-to-clipboard on tx hash
- Compare against `.project/03-mockups/screens/transactions.html`

---

### Task 3: Settings API — DB CRUD + Handler

**Goal**: Build settings CRUD backed by the existing `settings` key-value table.

**Files to create/modify**:
- `internal/db/settings.go` — New: `GetSetting`, `SetSetting`, `GetAllSettings`, `ResetBalances`, `ResetAll`
- `internal/api/handlers/settings.go` — New: `GET /api/settings`, `PUT /api/settings`, `POST /api/settings/reset-balances`, `POST /api/settings/reset-all`
- `internal/config/constants.go` — Add setting key constants
- `internal/config/errors.go` — Add settings error codes
- `internal/api/router.go` — Wire settings routes

**Details**:

Settings stored as key-value pairs in the `settings` table:
| Key | Default | Description |
|-----|---------|-------------|
| `max_scan_id` | `5000` | Default max scan ID per chain |
| `auto_resume_scans` | `true` | Auto-resume on startup |
| `resume_threshold_hours` | `24` | Hours before scan restarts |
| `btc_fee_rate` | `10` | sat/vB, 0 = dynamic |
| `bsc_gas_preseed_bnb` | `0.005` | BNB per address |
| `log_level` | `info` | debug/info/warn/error |

`GET /api/settings` returns all settings as a flat object.
`PUT /api/settings` accepts partial updates (only keys that are provided).

Danger zone endpoints:
- `POST /api/settings/reset-balances` — DELETE FROM balances + DELETE FROM scan_state + DELETE FROM transactions
- `POST /api/settings/reset-all` — above + DELETE FROM addresses

Both require a confirmation body: `{"confirm": true}`.

**Verification**:
- `go test ./internal/db/...` — settings CRUD tests
- `curl -X PUT localhost:8080/api/settings -d '{"max_scan_id": "10000"}'` works
- `curl localhost:8080/api/settings` returns all settings

---

### Task 4: Settings Frontend

**Goal**: Build the settings page matching the mockup — network mode, scanning config, transaction config, display, danger zone.

**Files to modify/create**:
- `web/src/lib/utils/api.ts` — Add `getSettings`, `updateSettings`, `resetBalances`, `resetAll`
- `web/src/lib/types.ts` — Add `SettingsData` interface (expanded)
- `web/src/routes/settings/+page.svelte` — Replace placeholder with full implementation

**Details**:

From the mockup, the page has 5 cards (max-width: 680px):
1. **Network**: Radio card selection (Mainnet/Testnet) + warning alert about restart
2. **Scanning**: Max scan ID input, auto-resume toggle, resume threshold dropdown
3. **Transaction**: BTC fee rate (input + "sat/vB" suffix), BSC gas pre-seed amount (input + "BNB" suffix)
4. **Display**: Log level dropdown, Price currency dropdown (disabled, "Coming soon")
5. **Danger Zone**: Red-bordered card with Reset Database + Re-generate Addresses buttons (with confirmation dialogs)

Bottom: Save Settings button.

The page loads current settings on mount, tracks changes locally, saves on button click.
Danger zone actions show a confirmation dialog before executing.

Network mode change should show the warning that it requires restart + re-scan.

**Verification**:
- Page renders all 5 cards correctly
- Save button sends PUT to `/api/settings`
- Danger zone buttons show confirmation before acting
- Compare against `.project/03-mockups/screens/settings.html`

---

### Task 5: Embedded Static File Server (SPA Fallback)

**Goal**: Serve the SvelteKit build from the Go binary using `embed.FS` with SPA fallback routing.

**Files to create/modify**:
- `web/embed.go` — New: `//go:embed all:build` with subFS helper
- `internal/api/handlers/spa.go` — New: SPA handler with 404 interception fallback to `index.html`
- `internal/api/router.go` — Add catch-all `/*` route for SPA serving (after `/api` routes)
- `cmd/server/main.go` — Pass embedded FS to router

**Details**:

The current SvelteKit config uses `adapter-static` with `fallback: 'index.html'` and `ssr = false`. The build outputs to `web/build/`.

Embed pattern:
```go
// web/embed.go
package web

import "embed"

//go:embed all:build
var BuildFS embed.FS
```

SPA handler with `hookedResponseWriter` that intercepts 404 from `http.FileServer` and serves `index.html` instead. Cache headers: `_app/immutable/**` gets `Cache-Control: public, max-age=31536000, immutable`; everything else gets `no-cache`.

Router ordering: `/api/*` routes first, then `/*` catch-all for SPA. This ensures API calls never hit the file server.

**Verification**:
- `go build ./cmd/server/` compiles with embedded frontend
- `curl localhost:8080/` returns HTML
- `curl localhost:8080/addresses` returns HTML (SPA fallback)
- `curl localhost:8080/api/health` still returns JSON (not HTML)
- `curl localhost:8080/_app/immutable/...` returns cache headers

---

### Task 6: Build Script

**Goal**: Create a build script that builds frontend + embeds into Go binary.

**Files to create**:
- `build.sh` — Build script
- `Makefile` — Update with build targets

**Details**:

`build.sh`:
1. `cd web && npm ci && npm run build` — build SvelteKit
2. `cd .. && CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" -o ./hdpay ./cmd/server/` — build Go binary

`Makefile` targets:
- `make build` — full build (frontend + backend)
- `make build-frontend` — SvelteKit only
- `make build-backend` — Go only (assumes frontend already built)
- `make dev` — run with `go run` (dev mode, no embed — serve frontend via Vite proxy)
- `make test` — run all tests
- `make clean` — remove build artifacts

**Verification**:
- `bash build.sh` produces `./hdpay` binary
- `./hdpay version` prints version
- `./hdpay serve` serves both API and frontend from single binary

---

### Task 7: Tests

**Goal**: Add tests for all new functionality.

**Files to create**:
- `internal/db/transactions_test.go` — Test filtered listing
- `internal/db/settings_test.go` — Test settings CRUD
- `internal/api/handlers/transactions_test.go` — Test handler param parsing
- `internal/api/handlers/settings_test.go` — Test settings handlers

**Details**:
- DB tests: CRUD operations, filter combinations, pagination edge cases, reset operations
- Handler tests: Query param parsing, validation, error responses

**Verification**:
- `go test ./internal/db/... -v` passes
- `go test ./internal/api/handlers/... -v` passes
- `go test ./... -count=1` — all tests pass

---

### Task 8: Final Integration & Cleanup

**Goal**: Verify the full flow works end-to-end, fix any remaining issues.

**Checklist**:
1. Build binary with `build.sh`
2. Run `./hdpay init --mnemonic-file ...` to generate addresses
3. Run `./hdpay serve` — verify dashboard, addresses, scan, send, transactions, settings all load
4. Run a testnet scan and verify it appears in transaction history
5. Verify settings persist across restarts
6. Verify danger zone reset works
7. Check for any hardcoded constants — move to constants files
8. Check for duplicated utility functions — extract to shared
9. Update `CHANGELOG.md`

**Verification**:
- Full application works as a single binary
- All pages match their mockups
- No console errors in browser
- `go test ./... -count=1` passes
- `npm run check` (TypeScript) passes in web/

---

## Success Criteria

- [ ] Transaction history page renders with filters (chain, direction, token) and pagination
- [ ] Settings page renders all 5 cards and persists changes
- [ ] Danger zone reset operations work with confirmation
- [ ] `build.sh` produces a single binary that serves both API and frontend
- [ ] SPA routing works (direct navigation to `/addresses` serves the app, not 404)
- [ ] `_app/immutable/` assets served with long-term cache headers
- [ ] All new code has tests, `go test ./...` passes
- [ ] Frontend TypeScript check passes with zero errors
- [ ] Pages match their respective mockups

## Reference Mockups
- `.project/03-mockups/screens/transactions.html`
- `.project/03-mockups/screens/settings.html`
