# Phase 11: Transaction History, Settings & Deployment

> **Status: Outline** — Will be expanded with full task details when this phase is reached.

<objective>
Build the transaction history page (filterable table with explorer links), settings page (network mode, scan defaults, fee config), build script for the final single binary with embedded frontend, and end-to-end integration verification.
</objective>

## Key Deliverables

1. **Transaction History API**:
   - `GET /api/transactions` — all transactions, paginated
   - `GET /api/transactions/:chain` — chain-filtered
   - Query params: direction, token, dateRange, status
   - Explorer links per chain
2. **Transaction History Frontend** — matching `.project/03-mockups/screens/transactions.html`:
   - Filterable table: chain badges, direction arrows, token, amount, status badges
   - Explorer link buttons
   - Pagination
3. **Settings API**:
   - `GET /api/settings` — current settings
   - `PUT /api/settings` — update settings
   - Settings: max scan ID per chain, network mode, log level
4. **Settings Frontend** — matching `.project/03-mockups/screens/settings.html`:
   - Network mode toggle (mainnet/testnet)
   - Scan defaults
   - Fee override (optional — auto-detected by default, advanced users can override)
   - Display preferences
   - Danger zone (reset database, etc.)
5. **Build Script**:
   - `build.sh` — build frontend → embed in Go binary → produce `./hdpay`
   - `CGO_ENABLED=0 go build -ldflags="-s -w"` for smallest binary
   - `//go:embed web/build/*` for static file serving
6. **Static File Server** — serve embedded SvelteKit build, SPA fallback to index.html
7. **Final Integration** — verify full flow: init → serve → scan → dashboard → send → history

## Files to Create/Modify

- `internal/api/handlers/transactions.go` — transaction history endpoints
- `internal/api/handlers/settings.go` — settings endpoints
- `internal/db/transactions.go` — transaction queries (extend)
- `internal/db/settings.go` — settings CRUD
- `web/src/routes/transactions/+page.svelte`
- `web/src/routes/settings/+page.svelte`
- `web/src/lib/stores/transactions.ts`
- `web/src/lib/stores/settings.ts`
- `cmd/server/main.go` — embed static files, serve command finalization
- `build.sh` — build script
- `Makefile` — update with final build targets

## Reference Mockups
- `.project/03-mockups/screens/transactions.html`
- `.project/03-mockups/screens/settings.html`

<research_needed>
- Go embed FS: serving SvelteKit static build with SPA fallback routing
- adapter-static output structure: what files are generated and how to serve them
</research_needed>
