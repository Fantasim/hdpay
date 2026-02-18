# Phase 6: Dashboard & Price Service

> **Status: Outline** — Will be expanded with full task details when this phase is reached.

<objective>
Build the CoinGecko price service with caching, balance summary API, and the dashboard frontend with total portfolio value, quick stats, balance breakdown table, and ECharts portfolio distribution charts.
</objective>

## Key Deliverables

1. **CoinGecko Price Service** — fetch BTC/BNB/SOL/USDC/USDT prices in USD, 5-minute cache
2. **Price API** — `GET /api/dashboard/prices`
3. **Portfolio Summary API** — `GET /api/dashboard/portfolio` — aggregate balances × prices
4. **Balance Summary API** — `GET /api/balances/summary` — per chain+token totals
5. **Dashboard Frontend** — matching `.project/03-mockups/screens/dashboard.html`
6. **Total Portfolio Value** — large USD display
7. **Quick Stats** — total addresses, funded count, chains, last scan time
8. **Balance Breakdown Table** — chain, token, balance, USD value, funded addresses
9. **ECharts Charts** — pie chart (chain distribution), bar chart (token distribution)

## Files to Create/Modify

- `internal/price/coingecko.go` — price fetcher + cache
- `internal/api/handlers/dashboard.go` — dashboard endpoints
- `internal/db/balances.go` — add aggregation queries
- `web/src/routes/+page.svelte` — dashboard page (replace placeholder)
- `web/src/lib/components/dashboard/PortfolioOverview.svelte`
- `web/src/lib/components/dashboard/ChainBreakdown.svelte`
- `web/src/lib/stores/balances.ts`
- `web/src/lib/stores/prices.ts`

## Reference Mockup
- `.project/03-mockups/screens/dashboard.html`

<research_needed>
- ECharts Svelte integration: best approach (svelte-echarts wrapper vs direct init)
- CoinGecko free API: rate limits, exact endpoint for multiple coins
</research_needed>
