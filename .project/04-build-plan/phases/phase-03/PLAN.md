# Phase 3: Address Explorer

<objective>
Build the address explorer: paginated REST API for listing addresses per chain (with balance filters), JSON export endpoint, and the frontend address page with chain tabs, filter chips, paginated table with copy-to-clipboard, and virtual scrolling for large result sets.
</objective>

<tasks>

## Task 1: Address API Endpoints

Implement the address listing and export API endpoints.

**Files:**
- `internal/api/handlers/address.go`:
  - `GET /api/addresses/:chain` — list addresses for a chain
    - Query params: `page` (default 1), `pageSize` (default 100, max 1000), `hasBalance` (bool), `token` (filter)
    - Response: `{ "data": [...], "meta": { "page", "pageSize", "total" } }`
    - Joins with balances table when `hasBalance=true`
  - `GET /api/addresses/:chain/export` — export all addresses as JSON download
    - Streams JSON response (Content-Type: application/json, Content-Disposition: attachment)
    - Same format as file export from Phase 2
  - Validate chain parameter (BTC, BSC, SOL only)
- `internal/db/addresses.go` — add query methods:
  - `GetAddressesPaginated(chain, page, pageSize, hasBalance, token) ([]AddressWithBalance, total int, error)`
  - `StreamAddresses(chain string, fn func(Address) error) error` — for export streaming
- Wire endpoints in `internal/api/router.go`

**Verification:**
- `curl localhost:8080/api/addresses/BTC?page=1&pageSize=10` → returns 10 addresses
- `curl localhost:8080/api/addresses/BTC?hasBalance=true` → returns only funded addresses
- `curl localhost:8080/api/addresses/INVALID` → returns error with ERROR_INVALID_CHAIN
- Export endpoint returns downloadable JSON

## Task 2: Address Response Types

Define API response types for address endpoints.

**Files:**
- `internal/models/types.go` — add:
  - `AddressWithBalance` struct: Chain, Index, Address, NativeBalance, TokenBalances []TokenBalance, LastScanned
  - `TokenBalance` struct: Token, Balance, ContractAddress
  - `APIResponse` struct: Data, Meta, Error
  - `PaginationMeta` struct: Page, PageSize, Total
  - `APIError` struct: Code, Message

**Verification:**
- Types serialize to JSON matching the API response format in CLAUDE.md

## Task 3: Frontend API Client

Set up the API client with CSRF token handling.

**Files:**
- `web/src/lib/utils/api.ts`:
  - `fetchAPI<T>(path, options)` — base fetch wrapper
    - Reads CSRF token from `csrf_token` cookie
    - Sets `X-CSRF-Token` header on mutations
    - Parses JSON response
    - Handles errors (throws typed errors)
  - `getAddresses(chain, params)` — GET /api/addresses/:chain
  - `exportAddresses(chain)` — GET /api/addresses/:chain/export (triggers download)
- `web/src/lib/types.ts` — add:
  - `AddressWithBalance` interface
  - `TokenBalance` interface
  - `APIResponse<T>` interface
  - `PaginationMeta` interface

**Verification:**
- API client correctly reads CSRF cookie and sends header
- Type-safe responses (no `any`)

## Task 4: Address Explorer Page

Build the frontend address explorer page matching the mockup.

**Reference mockup:** `.project/03-mockups/screens/addresses.html`

**Files:**
- `web/src/routes/addresses/+page.svelte` — main page:
  - Page header: "Addresses" title + "Export JSON" button
  - Chain tabs: All Chains, BTC, BSC, SOL (with chain-color dots)
  - Filter chips: All, Has Balance, Native, USDC, USDT
  - Address table (see Task 5)
  - Pagination controls
- `web/src/lib/components/address/AddressTable.svelte`:
  - Columns: #, Chain, Address (truncated + copy btn), Native Balance, Token Balances, Last Scanned
  - Chain badge with chain-specific colors
  - Monospace font for addresses and balances
  - Copy-to-clipboard button per address
  - Zero balances shown in disabled/muted color
  - Token balances shown as stacked rows (USDC, USDT) per mockup
  - "Never" shown for unscanned addresses
- `web/src/lib/stores/addresses.ts`:
  - Address store with reactive pagination state
  - Chain filter, balance filter, token filter
  - Fetch on filter/page change

**Verification:**
- Page renders matching mockup layout
- Chain tabs switch between chains
- Filter chips filter the table
- Pagination works (first, prev, page numbers, next, last)
- Copy button copies full address to clipboard
- Loading state shown while fetching

## Task 5: Virtual Scrolling

Integrate @tanstack/svelte-virtual for large address lists.

**Files:**
- `web/src/lib/components/address/AddressTable.svelte` — enhance with:
  - Virtual scrolling when total > MAX_TABLE_ROWS_DISPLAY (1000)
  - Fixed header row
  - Smooth scrolling
  - Row height estimation
  - Falls back to regular table for small result sets

**Verification:**
- Table handles 500K addresses without freezing (virtual scroll)
- Scroll position maintained when re-rendering
- Row heights consistent

## Task 6: Formatting Utilities

Build shared formatting functions used across the app.

**Files:**
- `web/src/lib/utils/formatting.ts`:
  - `truncateAddress(address, length)` — truncate middle: `bc1qxy2k...x0wlh`
  - `formatBalance(rawBalance, decimals)` — convert smallest unit to display: "100000000" → "1.00000000"
  - `formatUSD(amount)` — format as USD: "$1,234.56"
  - `formatNumber(n)` — format with commas: "1,500,000"
  - `formatRelativeTime(date)` — "2 min ago", "1 hour ago", "Never"
  - `copyToClipboard(text)` — navigator.clipboard.writeText wrapper
- `web/src/lib/utils/chains.ts`:
  - `getChainColor(chain)` — return chain-specific CSS color
  - `getChainLabel(chain)` — "Bitcoin", "BNB Chain", "Solana"
  - `getExplorerURL(chain, type, hash, network)` — explorer link builder
  - `getTokenDecimals(chain, token)` — decimals lookup

**Verification:**
- All formatting functions have correct output for edge cases
- Address truncation shows first N and last N characters
- Balance formatting handles large numbers and different decimal counts

</tasks>

<success_criteria>
- [ ] `GET /api/addresses/BTC` returns paginated address list
- [ ] `GET /api/addresses/BTC?hasBalance=true` filters to funded addresses only
- [ ] Export endpoint streams JSON download
- [ ] Frontend address page matches mockup layout
- [ ] Chain tabs switch data source
- [ ] Filter chips (All, Has Balance, Native, USDC, USDT) work correctly
- [ ] Pagination controls (first/prev/pages/next/last) navigate correctly
- [ ] Copy button copies full address to clipboard
- [ ] Virtual scrolling handles large datasets without performance issues
- [ ] Monospace font for addresses and balances
- [ ] Zero balances shown in muted color
- [ ] "Never" shown for unscanned addresses
- [ ] All formatting utilities tested
- [ ] CSRF token sent correctly on API calls
</success_criteria>

<verification>
1. Start backend + frontend dev servers
2. Navigate to /addresses
3. Verify chain tabs switch correctly
4. Click "Has Balance" filter → shows only funded addresses (may be empty if no scans yet)
5. Click copy button → address copied to clipboard
6. Click "Export JSON" → triggers file download
7. Navigate pages → data updates correctly
8. API test: `curl "localhost:8080/api/addresses/BTC?page=2&pageSize=50"` → correct pagination meta
9. Run frontend tests: `cd web && npm test`
</verification>
