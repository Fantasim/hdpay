# Phase 7: Dashboard Pages (Outline)

> Will be expanded into a detailed plan before building. This is the largest phase and may be split into sub-sessions.

<objective>
Build all 6 dashboard pages matching their HTML mockups: Overview (stats + charts), Transactions (table + filters + pagination), Watches (table with status badges), Pending Points, Errors (discrepancies + system errors), and Settings (tier editor, IP allowlist, system info, watch defaults). Desktop-only, manual refresh.
</objective>

<features>
F28 — Overview Page (8 stat cards + 7 charts + time range selector)
F29 — Transactions Page (full history table, filters, server-side pagination 25/50/100)
F30 — Watches Page (table with countdown timers, status badges, filters)
F31 — Pending Points Page (unclaimed + pending points per address)
F32 — Errors Page (3 sections: discrepancies, stale pending >24h, system errors)
F33 — Settings: Tier Editor (editable table, save to tiers.json, client+server validation)
F34 — Settings: IP Allowlist (table, add with IP+label, remove by ID, immediate cache refresh)
F35 — Settings: System Info & Watch Defaults (read-only info + runtime-editable defaults, lost on restart)
S1 — Discrepancy Auto-Detection (5 SQL checks triggered on errors page load)
S2 — Block Explorer Links (chain-aware + network-aware: mainnet→blockstream/bscscan/solscan, testnet→testnet explorers)
</features>

<tasks_outline>
1. Svelte stores for each data domain (stats, transactions, watches, points, errors, settings)
2. Overview: StatsCards (8 cards) + TimeRangeSelector (today/week/month/quarter/all)
3. Overview: 7 ECharts components (USD over time bar, points over time line, tx count bar, chain pie, token pie, tier distribution bar, watches over time line)
4. Transactions: TransactionTable (all columns from PROMPT.md, block explorer links per chain+network S2) + TransactionFilters (chain, token, date range, tier, status, min/max USD)
5. Transactions: Server-side pagination with page size selector (25/50/100)
6. Watches: WatchesTable (countdown timers for ACTIVE via setInterval, status color badges, last_poll_result JSON display, filters by status+chain)
7. Points: PendingPointsTable (address, chain, unclaimed, pending, tx count, last tx, all-time total)
8. Errors: DiscrepancyDetector (triggers GET /api/dashboard/errors which runs 5 SQL checks S1) + ErrorsList + StalePendingList (>24h pending txs)
9. Settings: TierEditor (editable rows with min_usd/max_usd/multiplier, client-side validation matching server rules, save button → PUT /api/admin/tiers)
10. Settings: IPAllowlist (table with IP+description+added_at, add form with IP+optional label, remove button → DELETE /api/admin/allowlist/:id)
11. Settings: SystemInfo (read-only: uptime, version, network, active watches, DB size, START_DATE) + WatchDefaults (editable timeout+max watches, note: lost on restart)
12. Mockup compliance check (compare each page against `.project/03-mockups/screens/` mockup)
</tasks_outline>

<research_needed>
- Apache ECharts integration with SvelteKit (svelte-echarts or direct echarts import)
- shadcn-svelte data table component for sortable/filterable tables
</research_needed>
