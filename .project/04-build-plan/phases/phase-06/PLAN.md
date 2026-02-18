# Phase 6: Dashboard & Price Service

> **Status: Detailed** — Ready to build.

<objective>
Build the CoinGecko price service with caching, balance summary API, portfolio summary API, and the dashboard frontend with total portfolio value, quick stats, balance breakdown table, and ECharts portfolio distribution charts.
</objective>

## Dependencies

- Phase 4 (Scanner Engine): provides scanner & balance DB tables
- Phase 5 (Scan UI): provides SSE infrastructure, scan stores

## Research Results

### ECharts Integration
- Use `svelte-echarts` v1.0.0 (supports Svelte 5 runes)
- Install: `npm i -D svelte-echarts echarts`
- Tree-shake: import only `PieChart`, `BarChart`, `TooltipComponent`, `LegendComponent`, `CanvasRenderer`
- Use `$derived()` rune for reactive chart options
- Container needs explicit height (`h-64` or similar)

### CoinGecko Free API
- Endpoint: `GET https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,binancecoin,solana,usd-coin,tether&vs_currencies=usd`
- Response: `{"bitcoin":{"usd":97500},"binancecoin":{"usd":625},...}`
- Rate limit: ~5-15 req/min (public, no key) — 5-min cache is safe
- Coin ID mapping: bitcoin, binancecoin, solana, usd-coin, tether

---

## Tasks

### Task 1: CoinGecko Price Service

**Create** `internal/price/coingecko.go`

Implement a thread-safe price service with in-memory caching:

```
type PriceService struct {
    client    *http.Client
    cache     map[string]float64   // coin ID -> USD price
    cachedAt  time.Time
    mu        sync.RWMutex
}
```

Functions:
- `NewPriceService() *PriceService` — creates service with `http.Client` (timeout from `config.APITimeout`)
- `GetPrices(ctx context.Context) (map[string]float64, error)` — returns cached prices or fetches fresh if cache expired (`config.PriceCacheDuration`)
- `fetchPrices(ctx context.Context) (map[string]float64, error)` — calls CoinGecko, parses response

Add constants to `internal/config/constants.go`:
```go
// CoinGecko coin IDs
const (
    CoinGeckoIDs = "bitcoin,binancecoin,solana,usd-coin,tether"
)
```

Add to `internal/config/errors.go`:
```go
var ErrPriceFetchFailed = errors.New("price fetch failed")
```

Map CoinGecko IDs to our symbols: `bitcoin->BTC`, `binancecoin->BNB`, `solana->SOL`, `usd-coin->USDC`, `tether->USDT`

Log: every fetch attempt (INFO), cache hits (DEBUG), errors (ERROR).

<verification>
- Unit test: mock HTTP server returning known prices, verify parsing
- Unit test: cache expiry works (fetch once, read from cache, wait, re-fetch)
- `go vet ./internal/price/...`
</verification>

---

### Task 2: Balance Aggregation DB Queries

**Modify** `internal/db/balances.go`

Add a new method that returns aggregated balance totals per chain+token:

```go
type BalanceAggregate struct {
    Chain       models.Chain
    Token       models.Token
    TotalBalance string    // SUM as text (to avoid float precision loss)
    FundedCount  int
}

func (d *DB) GetBalanceAggregates() ([]BalanceAggregate, error)
```

SQL:
```sql
SELECT chain, token, CAST(SUM(CAST(balance AS REAL)) AS TEXT), COUNT(*)
FROM balances
WHERE balance != '0'
GROUP BY chain, token
ORDER BY chain, token
```

Also add a method to get the latest scan time across all chains:
```go
func (d *DB) GetLatestScanTime() (string, error)
```

SQL:
```sql
SELECT MAX(updated_at) FROM scan_state
```

<verification>
- Unit test: insert test balances, verify aggregation sums and counts
- `go vet ./internal/db/...`
</verification>

---

### Task 3: Dashboard API Handlers

**Create** `internal/api/handlers/dashboard.go`

Two handlers:

#### `GetPrices(ps *price.PriceService) http.HandlerFunc`
- `GET /api/dashboard/prices`
- Calls `ps.GetPrices(ctx)`, returns `map[string]float64` (keyed by symbol: BTC, BNB, SOL, USDC, USDT)
- Response: `{ "data": { "BTC": 97500, "BNB": 625, ... }, "meta": { "executionTime": 42 } }`

#### `GetPortfolio(database *db.DB, ps *price.PriceService) http.HandlerFunc`
- `GET /api/dashboard/portfolio`
- Calls `database.GetBalanceAggregates()` + `ps.GetPrices(ctx)`
- Computes USD values per chain+token: `balance * price`
- Calls `database.CountAddresses(chain)` for each chain to get total address counts
- Calls `database.GetLatestScanTime()` for last scan
- Returns:
```json
{
  "data": {
    "totalUsd": 124892.47,
    "lastScan": "2026-02-18T10:30:00Z",
    "chains": [
      {
        "chain": "BTC",
        "addressCount": 500000,
        "fundedCount": 12,
        "tokens": [
          { "symbol": "BTC", "balance": "0.45210000", "usd": 45210.00, "fundedCount": 12 }
        ]
      }
    ]
  },
  "meta": { "executionTime": 85 }
}
```

Wire both into `internal/api/router.go` under a new `r.Route("/dashboard", ...)` group.

<verification>
- Unit test: mock DB + mock PriceService, verify JSON response shapes
- Test with empty balances (returns zero totals)
- `go vet ./internal/api/...`
</verification>

---

### Task 4: Wire Price Service in main.go

**Modify** `cmd/server/main.go`

- Import `price` package
- Create `ps := price.NewPriceService()` in `runServe()`
- Pass `ps` to `api.NewRouter(...)`
- Update `api.NewRouter` signature to accept `*price.PriceService`

**Modify** `internal/api/router.go`

- Accept `ps *price.PriceService` parameter
- Add dashboard route group:
```go
r.Route("/dashboard", func(r chi.Router) {
    r.Get("/prices", handlers.GetPrices(ps))
    r.Get("/portfolio", handlers.GetPortfolio(database, ps))
})
```

<verification>
- `go build ./cmd/server/...` — compiles
- Start server, `curl http://127.0.0.1:8080/api/dashboard/prices` returns prices
- `curl http://127.0.0.1:8080/api/dashboard/portfolio` returns portfolio structure
</verification>

---

### Task 5: Frontend Types & Constants

**Modify** `web/src/lib/types.ts`

Update `PriceData` to include all tokens:
```typescript
export interface PriceData {
    BTC: number;
    BNB: number;
    SOL: number;
    USDC: number;
    USDT: number;
}
```

Add dashboard-specific types:
```typescript
export interface PortfolioResponse {
    totalUsd: number;
    lastScan: string | null;
    chains: ChainPortfolio[];
}

export interface ChainPortfolio {
    chain: Chain;
    addressCount: number;
    fundedCount: number;
    tokens: TokenPortfolioItem[];
}

export interface TokenPortfolioItem {
    symbol: TokenSymbol;
    balance: string;
    usd: number;
    fundedCount: number;
}
```

**Modify** `web/src/lib/constants.ts`

Add:
```typescript
export const PRICE_REFRESH_INTERVAL_MS = 5 * 60 * 1000; // 5 minutes
export const PORTFOLIO_REFRESH_INTERVAL_MS = 60 * 1000; // 1 minute
```

<verification>
- `npm run check` — TypeScript compiles with no errors
</verification>

---

### Task 6: Dashboard API Functions

**Modify** `web/src/lib/utils/api.ts`

Add:
```typescript
export function getPrices(): Promise<APIResponse<PriceData>> {
    return api.get<PriceData>('/dashboard/prices');
}

export function getPortfolio(): Promise<APIResponse<PortfolioResponse>> {
    return api.get<PortfolioResponse>('/dashboard/portfolio');
}
```

<verification>
- TypeScript compiles with `npm run check`
</verification>

---

### Task 7: Install ECharts + svelte-echarts

Run in `web/`:
```bash
npm i -D echarts svelte-echarts
```

<verification>
- `npm ls echarts svelte-echarts` — installed
- `npm run build` — no errors
</verification>

---

### Task 8: Dashboard Page & Components

**Create** `web/src/lib/components/dashboard/PortfolioOverview.svelte`

Displays:
- Total portfolio value (large `$124,892.47` styled with mono font)
- 4 stat cards in a grid: Total Addresses, Funded, Chains (3), Last Scan (relative time)
- Props: `portfolio: PortfolioResponse, loading: boolean`
- Match mockup styling: `.stats-grid` 4-column grid, `.card` with `.stat` inside

**Create** `web/src/lib/components/dashboard/BalanceBreakdown.svelte`

Displays:
- Section title "Balance Breakdown"
- Table matching mockup: Chain (badge), Token, Balance (mono right-aligned), USD Value (mono right-aligned), Funded Addresses (right-aligned)
- Props: `chains: ChainPortfolio[]`
- Chain badges use existing `.badge-btc`, `.badge-bsc`, `.badge-sol` classes

**Create** `web/src/lib/components/dashboard/PortfolioCharts.svelte`

Displays:
- Section title "Portfolio Distribution"
- Pie chart showing USD distribution by chain (BTC/BSC/SOL)
- Use `svelte-echarts` `<Chart>` component with tree-shaking
- Colors from `CHAIN_COLORS` constant
- Props: `chains: ChainPortfolio[]`
- Reactive via `$derived()` rune
- Show "No data" placeholder when no balances exist

**Modify** `web/src/routes/+page.svelte`

Replace placeholder with full dashboard:
- `onMount`: fetch portfolio + prices
- Interval refresh for prices (5 min) + portfolio (1 min)
- Loading state with skeleton
- Error handling
- Wire PortfolioOverview, BalanceBreakdown, PortfolioCharts

<verification>
- `npm run build` — compiles
- Visual check: page matches mockup layout (total value -> stats grid -> breakdown table -> chart)
- Handles empty state (no balances scanned yet)
- Components properly cleanup intervals on destroy
</verification>

---

### Task 9: Tests

**Create** `internal/price/coingecko_test.go`
- Test price parsing from mock HTTP response
- Test cache hit (second call within duration returns cached)
- Test error handling (bad HTTP response, malformed JSON)

**Create** `internal/api/handlers/dashboard_test.go`
- Test `GET /api/dashboard/prices` returns correct shape
- Test `GET /api/dashboard/portfolio` with mock data
- Test portfolio with empty balances

<verification>
- `go test ./internal/price/... -v`
- `go test ./internal/api/handlers/... -v`
- All pass
</verification>

---

## Files to Create

| File | Purpose |
|------|---------|
| `internal/price/coingecko.go` | CoinGecko price fetcher + 5-min cache |
| `internal/price/coingecko_test.go` | Price service tests |
| `internal/api/handlers/dashboard.go` | Portfolio + prices API handlers |
| `internal/api/handlers/dashboard_test.go` | Dashboard handler tests |
| `web/src/lib/components/dashboard/PortfolioOverview.svelte` | Total value + stats grid |
| `web/src/lib/components/dashboard/BalanceBreakdown.svelte` | Balance table by chain+token |
| `web/src/lib/components/dashboard/PortfolioCharts.svelte` | ECharts pie chart |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/config/constants.go` | Add `CoinGeckoIDs` |
| `internal/config/errors.go` | Add `ErrPriceFetchFailed` |
| `internal/db/balances.go` | Add `GetBalanceAggregates`, `GetLatestScanTime` |
| `internal/api/router.go` | Add dashboard route group, accept `PriceService` |
| `cmd/server/main.go` | Create `PriceService`, pass to router |
| `web/src/lib/types.ts` | Update `PriceData`, add portfolio types |
| `web/src/lib/constants.ts` | Add refresh interval constants |
| `web/src/lib/utils/api.ts` | Add `getPrices`, `getPortfolio` |
| `web/src/routes/+page.svelte` | Replace placeholder with dashboard |

## Reference Mockup
- `.project/03-mockups/screens/dashboard.html`

<success_criteria>
- [ ] CoinGecko price service fetches and caches prices (5-min TTL)
- [ ] `GET /api/dashboard/prices` returns symbol-keyed USD prices
- [ ] `GET /api/dashboard/portfolio` returns aggregated balances with USD values
- [ ] Dashboard page shows total portfolio value, 4 stat cards, breakdown table
- [ ] ECharts pie chart renders portfolio distribution by chain
- [ ] Empty state handled gracefully (no balances -> $0.00, empty table, no chart)
- [ ] All backend tests pass (`go test ./...`)
- [ ] Frontend builds without errors (`npm run build`)
- [ ] No hardcoded constants — all values in constants files
</success_criteria>
