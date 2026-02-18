# Phase 6 Summary: Dashboard & Price Service

## Completed: 2026-02-18

## What Was Built
- CoinGecko price service with 5-minute in-memory cache, thread-safe
- Balance aggregation DB queries (per chain+token SUM, latest scan time)
- Dashboard API: `GET /api/dashboard/prices` and `GET /api/dashboard/portfolio`
- Dashboard frontend page with total portfolio value, 4 quick-stat cards, balance breakdown table, and ECharts pie chart
- 3 dashboard components: PortfolioOverview, BalanceBreakdown, PortfolioCharts
- 9 new tests (6 price service, 3 dashboard handlers)

## Files Created/Modified
- `internal/price/coingecko.go` — CoinGecko price fetcher with cache
- `internal/price/coingecko_test.go` — 6 tests (success, cache hit, cache expiry, HTTP error, malformed JSON, partial response)
- `internal/api/handlers/dashboard.go` — GetPrices + GetPortfolio handlers
- `internal/api/handlers/dashboard_test.go` — 3 tests (prices, portfolio with balances, portfolio empty)
- `internal/db/balances.go` — Added GetBalanceAggregates, GetLatestScanTime
- `internal/config/constants.go` — Added CoinGeckoIDs
- `internal/config/errors.go` — Added ErrPriceFetchFailed
- `internal/api/router.go` — Added dashboard route group, PriceService parameter
- `cmd/server/main.go` — Created PriceService, passed to router
- `web/src/lib/types.ts` — Updated PriceData, added PortfolioResponse/ChainPortfolio/TokenPortfolioItem
- `web/src/lib/constants.ts` — Added PRICE_REFRESH_INTERVAL_MS, PORTFOLIO_REFRESH_INTERVAL_MS
- `web/src/lib/utils/api.ts` — Added getPrices, getPortfolio
- `web/src/lib/components/dashboard/PortfolioOverview.svelte` — Total value + stats grid
- `web/src/lib/components/dashboard/BalanceBreakdown.svelte` — Balance table by chain+token
- `web/src/lib/components/dashboard/PortfolioCharts.svelte` — ECharts pie chart
- `web/src/routes/+page.svelte` — Full dashboard page with auto-refresh

## Decisions Made
- **svelte-echarts v1.0.0**: Used wrapper library for ECharts (Svelte 5 compatible), tree-shaking with PieChart only
- **Direct ECharts init**: Passed `init` from `echarts/core` to `<Chart>` component for proper tree-shaking
- **CoinGecko public API**: No API key needed; 5-min cache safely under rate limits
- **NATIVE token mapping**: Backend maps NATIVE to BTC/BNB/SOL symbols for price lookup via `tokenToSymbol()` helper
- **Portfolio auto-refresh**: 1-minute interval for portfolio, keeping price cache at 5-min server-side
- **Donut chart style**: Used `radius: ['40%', '70%']` for cleaner look vs solid pie

## Deviations from Plan
- Removed pre-existing PortfolioSummary/ChainSummary/TokenSummaryItem types from types.ts (replaced by actual API-matching types)

## Issues Encountered
- None

## Notes for Next Phase
- Price service is instantiated once in main.go and shared across all handlers
- ECharts is only imported with PieChart; future phases can add BarChart if needed
- Dashboard auto-refreshes but relies on scan having been run to show data
