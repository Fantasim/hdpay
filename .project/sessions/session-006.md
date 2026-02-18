# Session 006 — 2026-02-18

## Version: V1
## Phase: building (Phase 6 of 11)
## Summary: Phase 6 complete — Dashboard & Price Service

## What Was Done
- Created CoinGecko price service with 5-minute in-memory cache, thread-safe (`internal/price/coingecko.go`)
- Added balance aggregation DB queries: `GetBalanceAggregates` (SUM per chain+token), `GetLatestScanTime`
- Built dashboard API handlers: `GET /api/dashboard/prices`, `GET /api/dashboard/portfolio`
- Wired PriceService into main.go and router.go with dependency injection
- Installed ECharts + svelte-echarts v1.0.0 with tree-shaking
- Created 3 dashboard Svelte 5 components:
  - PortfolioOverview: total USD value + 4 stat cards
  - BalanceBreakdown: table with chain badges, balance, USD, funded count
  - PortfolioCharts: ECharts donut pie chart for chain distribution
- Built dashboard page with 1-minute auto-refresh and empty state handling
- Updated frontend types (PriceData, PortfolioResponse, ChainPortfolio, TokenPortfolioItem)
- Added API functions (getPrices, getPortfolio) and refresh interval constants
- Wrote 9 tests: 6 price service tests, 3 dashboard handler tests
- All existing tests continue to pass

## Decisions Made
- **svelte-echarts v1.0.0**: Wrapper library for ECharts with Svelte 5 support, tree-shaking via PieChart-only imports
- **CoinGecko public API**: No API key needed; 5-min cache safely under rate limits (~5-15 req/min public tier)
- **NATIVE token mapping**: Backend `tokenToSymbol()` maps NATIVE → BTC/BNB/SOL for price lookup
- **Donut chart**: Used `radius: ['40%', '70%']` for cleaner look vs solid pie
- **Portfolio auto-refresh**: 1-minute interval client-side, 5-minute price cache server-side

## Issues / Blockers
- None

## Next Steps
- Run `/cf-next` to start Phase 7: BTC Transaction Engine
- Phase 7 delivers: UTXO fetching, multi-input TX building, P2WPKH signing, dynamic fee estimation, broadcast
