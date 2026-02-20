# Phase 7: Dashboard Pages

<objective>
Build all 6 dashboard pages matching their HTML mockups: Overview (stats + charts), Transactions (table + filters + server-side pagination), Watches (table with countdown timers), Points (summary cards + accounts table), Errors (3 sections: discrepancies, stale pending, system errors), and Settings (tier editor, IP allowlist, watch defaults, system info). Desktop-only, manual refresh, dark theme.
</objective>

<research_completed>
- **Apache ECharts**: Use `svelte-echarts@1.0.0` + `echarts@5.x`. Tree-shake via `echarts/core`. Pass `theme="dark"`. Every chart container needs explicit Tailwind height class.
- **Data tables**: Use shadcn-svelte table primitives + `@tanstack/table-core` (NOT `@tanstack/svelte-table`). shadcn provides `createSvelteTable` bridge for Svelte 5. Use `manualPagination: true` + `manualSorting: true` for server-side behavior. Keep filter state in plain `$state` variables.
</research_completed>

<tasks>

## Task 1: Install Dependencies & Add Constants

**Install npm packages:**
```bash
cd web/poller && npm install echarts svelte-echarts @tanstack/table-core
```

**Add shadcn components:**
```bash
npx shadcn-svelte@latest add select
```

**Add to `web/poller/src/lib/constants.ts`:**
```typescript
// Pagination
export const TABLE_PAGE_SIZES = [25, 50, 100] as const;
export const TABLE_DEFAULT_PAGE_SIZE = 25;

// Token colors (for pie charts)
export const TOKEN_COLORS: Record<string, string> = {
  BTC: '#f7931a',
  BNB: '#F0B90B',
  SOL: '#9945FF',
  USDC: '#3b82f6',
  USDT: '#10b981',
} as const;
```

**Files:**
- `web/poller/package.json` (updated)
- `web/poller/src/lib/constants.ts` (updated)
- `web/poller/src/lib/components/ui/select/` (new shadcn component)

<verification>
- `cd web/poller && npm run build` passes
- `echarts`, `svelte-echarts`, `@tanstack/table-core` in package.json
- Select component importable
</verification>

---

## Task 2: ECharts Wrapper Component

Create a reusable chart wrapper that registers needed chart types and provides the dark theme.

**File: `web/poller/src/lib/components/charts/ChartWrapper.svelte`**

- Import `Chart` from `svelte-echarts`
- Import and `use()`: BarChart, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, TitleComponent, CanvasRenderer from `echarts/core` subpaths
- Props: `options: EChartsOption`, `height: string` (default `'20rem'`)
- Render `<Chart {init} {options} theme="dark" />` inside a div with dynamic height class
- Set `backgroundColor: 'transparent'` in all chart options so the card background shows through

<verification>
- Component renders without errors
- Charts use dark palette automatically
- Build passes
</verification>

---

## Task 3: Overview Page — Stats Cards + Time Range Selector

Replace the overview stub at `web/poller/src/routes/+page.svelte`.

**Layout from mockup:**
- Header: "Overview" with time range selector buttons in the actions slot
- Stats section: 2 rows of 4 cards each (8 cards total), 4-column grid
- Charts section: "Charts" label, then 2-column grid of 7 chart cards

**Components to create:**

### 3a: `TimeRangeSelector.svelte`
- `web/poller/src/lib/components/dashboard/TimeRangeSelector.svelte`
- Props: `selected: TimeRange`, event: `onchange: (range: TimeRange) => void`
- Renders 5 buttons (Today, This Week, This Month, This Quarter, All Time) using `TIME_RANGES` and `TIME_RANGE_LABELS` from constants
- Active button has accent background (matching mockup `.time-range-btn.active`)

### 3b: `StatsCard.svelte`
- `web/poller/src/lib/components/dashboard/StatsCard.svelte`
- Props: `label: string`, `value: string`, `hint: string`
- Renders card matching mockup structure: stat-label, stat-value (mono font), stat-hint

### 3c: Overview page
- `web/poller/src/routes/+page.svelte`
- On mount: call `getDashboardStats(selectedRange)` and `getDashboardCharts(selectedRange)`
- When time range changes: re-fetch both endpoints
- Render 8 StatsCard components in a 4-column grid:
  - Row 1: Active Watches, Total Watches, USD Received, Points Awarded
  - Row 2: Pending Points, Unique Addresses, Avg TX Size, Largest TX
- Below stats: "Charts" section title + 7 chart cards (Task 4)

**Files:**
- `web/poller/src/lib/components/dashboard/TimeRangeSelector.svelte` (new)
- `web/poller/src/lib/components/dashboard/StatsCard.svelte` (new)
- `web/poller/src/routes/+page.svelte` (replace stub)

<verification>
- Page renders with loading state, then shows stats cards
- Time range buttons switch and re-fetch data
- 8 stats cards in 2 rows of 4
- Build passes
</verification>

---

## Task 4: Overview Page — 7 Chart Components

Create individual chart components, each accepting their data slice from `ChartData`:

### 4a: `UsdOverTimeChart.svelte`
- `web/poller/src/lib/components/charts/UsdOverTimeChart.svelte`
- Props: `data: Array<{ date: string; usd: number }>`
- Bar chart: x=dates, y=USD. Accent color bars. Tooltip shows "$X,XXX.XX"

### 4b: `PointsOverTimeChart.svelte`
- `web/poller/src/lib/components/charts/PointsOverTimeChart.svelte`
- Props: `data: Array<{ date: string; points: number }>`
- Line chart: x=dates, y=points. Accent color line. Tooltip shows "XX,XXX pts"

### 4c: `TxCountChart.svelte`
- `web/poller/src/lib/components/charts/TxCountChart.svelte`
- Props: `data: Array<{ date: string; count: number }>`
- Bar chart: x=dates, y=count. Blue (#3b82f6) bars.

### 4d: `ChainBreakdownChart.svelte`
- `web/poller/src/lib/components/charts/ChainBreakdownChart.svelte`
- Props: `data: ChainBreakdown[]`
- Pie/donut chart: BTC/BSC/SOL with CHAIN_COLORS. Legend below with percentages.

### 4e: `TokenBreakdownChart.svelte`
- `web/poller/src/lib/components/charts/TokenBreakdownChart.svelte`
- Props: `data: TokenBreakdown[]`
- Pie/donut chart: BTC/BNB/SOL/USDC/USDT with TOKEN_COLORS. Legend below.

### 4f: `TierDistributionChart.svelte`
- `web/poller/src/lib/components/charts/TierDistributionChart.svelte`
- Props: `data: TierBreakdown[]`
- Bar chart: x=tier labels (T0, T1, ...), y=count. Green bars.

### 4g: `WatchesOverTimeChart.svelte`
- `web/poller/src/lib/components/charts/WatchesOverTimeChart.svelte`
- Props: `data: DailyWatchStat[]`
- Line chart: 3 lines (active=blue, completed=green, expired=red). Legend.

### Wire into overview page:
- Each chart in a `<div class="card">` with `card-title`, same layout as mockup
- 2-column grid below the stats

<verification>
- All 7 charts render with mock/fetched data
- Colors match mockup (chain colors, token colors)
- Pie charts show donut style with legend
- Bar/line charts have proper axes and tooltips
- Build passes
</verification>

---

## Task 5: Transactions Page — Table, Filters, Pagination

Replace the transactions stub at `web/poller/src/routes/transactions/+page.svelte`.

**Layout from mockup:**
- Header: "Transactions" / "Full transaction history"
- Filter bar: Chain dropdown, Token dropdown, Status dropdown, Tier dropdown, separator, Min $ input, Max $ input, Page size segmented control (25/50/100) pushed to the right
- Table: 11 columns (Timestamp, Address, Chain, Token, Amount, USD Value, Tier, Points, TX Hash, Watch, Status)
- Pagination: "Showing X–Y of Z transactions" + page number buttons

**Components to create:**

### 5a: `TransactionFilters.svelte`
- `web/poller/src/lib/components/transactions/TransactionFilters.svelte`
- Props: `filters: TransactionFilters`, event: `onchange: (filters: TransactionFilters) => void`
- Renders: 4 select dropdowns (chain, token, status, tier) + 2 number inputs (min/max USD)
- Use native `<select>` styled with `form-select` class (matching mockup) rather than shadcn Select for simplicity — the mockup uses native selects
- Each change calls onchange with updated filters

### 5b: `PageSizeSelector.svelte`
- `web/poller/src/lib/components/ui/PageSizeSelector.svelte`
- Props: `selected: number`, event: `onchange: (size: number) => void`
- Segmented button group: 25 | 50 | 100 (matching mockup `.page-size-control`)

### 5c: `TransactionTable.svelte`
- `web/poller/src/lib/components/transactions/TransactionTable.svelte`
- Props: `transactions: Transaction[]`, `network: string`
- Renders table with all 11 columns from the mockup
- Address: truncated, mono font, full address in title attribute
- Chain: colored badge (badge-btc, badge-bsc, badge-sol)
- Amount: right-aligned, mono font, `amount_human`
- USD: right-aligned, mono font, `formatUsd(usd_value)`
- Tier: tier badge (centered number in rounded square)
- Points: right-aligned, accent color, mono font, `formatPoints(points)`
- TX Hash: truncated, clickable link to block explorer (use `EXPLORER_TX_URL` or `EXPLORER_TX_URL_TESTNET` based on network). For SOL composite hashes (`sig:TOKEN`), strip the `:TOKEN` suffix for the explorer link.
- Status: CONFIRMED (green badge) or PENDING (amber badge)

### 5d: Pagination component
- `web/poller/src/lib/components/ui/Pagination.svelte`
- Props: `page: number`, `pageSize: number`, `total: number`, event: `onPageChange: (page: number) => void`
- Shows "Showing X–Y of Z transactions" + numbered page buttons + next arrow
- Matching mockup `.pagination` styles

### 5e: Wire into page
- `web/poller/src/routes/transactions/+page.svelte`
- Local state: `filters`, `page`, `pageSize`, `transactions`, `total`, `loading`
- On mount + on filter/page change: call `getDashboardTransactions(filters)` with page/pageSize
- Layout: Header → FilterBar + PageSizeSelector → Table → Pagination

<verification>
- Table renders with all 11 columns
- Filter dropdowns filter the data (server-side)
- Page size selector changes row count
- Pagination navigates between pages
- TX hash links open correct block explorer
- SOL composite hashes handled correctly
- Build passes
</verification>

---

## Task 6: Watches Page — Table with Countdown Timers

Replace the watches stub at `web/poller/src/routes/watches/+page.svelte`.

**Layout from mockup:**
- Header: "Watches" / "Active and historical address watches"
- Filter bar: Status filter chips (All, Active, Completed, Expired, Cancelled) + Chain filter chips (All, BTC, BSC, SOL)
- Table: 7 columns (Address, Chain, Status, Started, Time Remaining, Polls, Last Poll Result)

**Components to create:**

### 6a: `FilterChips.svelte`
- `web/poller/src/lib/components/ui/FilterChips.svelte`
- Props: `label: string`, `options: string[]`, `selected: string`, event: `onchange: (val: string) => void`
- Renders a label + row of chip buttons, active chip highlighted
- Reusable for both status and chain filters

### 6b: Watches page
- `web/poller/src/routes/watches/+page.svelte`
- On mount: call `listWatches(params)` with status/chain filters
- Table columns from mockup:
  - Address: truncated mono with muted suffix
  - Chain: colored badge
  - Status: colored badge (ACTIVE=blue, COMPLETED=green, EXPIRED=red, CANCELLED=gray)
  - Started: formatted date
  - Time Remaining: for ACTIVE watches, live countdown via `setInterval(1000)` using `formatCountdown(expires_at)`. For others: "—"
  - Polls: poll_count
  - Last Poll Result: last_poll_result string (from JSON)
- Countdown timer: on component mount, start a 1-second interval that updates all active watch countdowns. Clear on destroy.
- Filter changes re-fetch from API

<verification>
- Table shows watches with all 7 columns
- Active watches show live countdown updating every second
- Non-active watches show "—" for time remaining
- Status badges have correct colors
- Filter chips work for both status and chain
- Build passes
</verification>

---

## Task 7: Points Page — Summary Cards + Table

Replace the points stub at `web/poller/src/routes/points/+page.svelte`.

**Layout from mockup:**
- Header: "Points" / "Accounts with unclaimed and pending points"
- 3 summary cards in a row: Total Unclaimed, Total Pending, All-Time Awarded
- Table: 7 columns (Address, Chain, Unclaimed, Pending, TX Count, Last TX, All-Time Total)

**Implementation:**
- On mount: call `getPoints()` to get accounts with unclaimed points, also call `getDashboardStats('all')` for aggregate numbers (total pending, total unclaimed, all-time)
- Summary cards use the same `StatsCard` component from Task 3b (or a slightly adapted version matching the points mockup which has larger values)
- Table:
  - Address: mono font, truncated
  - Chain: colored badge
  - Unclaimed: right-aligned, green if > 0, muted if 0
  - Pending: right-aligned, warning/amber if > 0, muted if 0
  - TX Count: centered
  - Last TX: formatted date
  - All-Time Total: right-aligned, mono font

**Note:** The API returns `PointsWithTransactions[]` which has `unclaimed` and `total`. We also need `pending` from `PointsAccount` — may need to merge data from `getPoints()` (unclaimed) + `getPendingPoints()` (pending) + potentially a full points list. Check what the backend returns and adjust.

Actually, looking at the types more carefully:
- `getPoints()` returns accounts with `unclaimed > 0` plus their transactions
- For the points page, we need ALL accounts (not just unclaimed > 0) — need to use `getDashboardStats` for summary numbers, and present the table from the merged data

For simplicity: fetch both `getPoints()` and `getPendingPoints()`, merge by address+chain to build the table. The summary card totals come from aggregating the merged data.

<verification>
- 3 summary cards show correct aggregated numbers
- Table shows all accounts with points
- Unclaimed values are green when > 0
- Pending values are amber when > 0
- Build passes
</verification>

---

## Task 8: Errors Page — 3 Sections

Replace the errors stub at `web/poller/src/routes/errors/+page.svelte`.

**Layout from mockup:**
- Header: "Errors" / "System health, discrepancies, and error log"
- 3 card sections stacked vertically:

### Section 1: Discrepancies
- Card title: "Discrepancies" / "Auto-detected integrity issues"
- Table: 5 columns (Type, Address, Chain, Details, Detected)
- Type: colored badge (POINTS_MISMATCH=error/red, UNCLAIMED_EXCEEDS_TOTAL=warning/amber, etc.)
- Address: mono font
- Chain: colored badge

### Section 2: Stale Pending Transactions
- Card title: "Stale Pending Transactions" / "Pending for more than 24 hours"
- Table: 5 columns (TX Hash, Chain, Address, Detected At, Hours Pending)
- TX Hash: clickable link to block explorer
- Hours Pending: red text with "Xh" format

### Section 3: System Errors
- Card title: "System Errors" / "Provider failures and runtime issues"
- Table: 5 columns (Severity, Category, Message, Timestamp, Resolved)
- Severity: dot indicator + text (ERROR=red dot, WARNING=amber dot)
- Category: default badge (PROVIDER, PRICE, RECOVERY, etc.)
- Resolved: "Yes" in green or "No" in red

**Implementation:**
- On mount: call `getDashboardErrors()` — triggers 5 SQL discrepancy checks on the backend
- Render each section from the response `discrepancies`, `stale_pending`, `errors` arrays
- Show "No issues found" message in empty tables

<verification>
- All 3 sections render
- Empty state shows "No issues found" message
- Severity dots have correct colors
- TX hash links work
- Build passes
</verification>

---

## Task 9: Settings Page — 4 Sections

Replace the settings stub at `web/poller/src/routes/settings/+page.svelte`.

**Layout from mockup:**
- Header: "Settings" / "Configuration and system management"
- 4 sections separated by dividers:

### Section 1: Points Tier Editor
- `web/poller/src/lib/components/settings/TierEditor.svelte`
- Props: `tiers: Tier[]`, event: `onsave: (tiers: Tier[]) => void`
- Table: 5 columns (Tier, Min USD, Max USD, Multiplier, Example)
- Each row has inline number inputs for min_usd, max_usd, multiplier
- Tier 0 row visually dimmed (opacity 0.55)
- Last tier has max_usd disabled showing "∞"
- Example column: computed from values, e.g., "$5.00 → 500 pts"
- "Save Tiers" button calls `updateTiers(tiers)` API
- Client-side validation before save:
  - min_usd >= 0
  - Each tier's min_usd == previous tier's max_usd (no gaps)
  - multiplier >= 0
  - Last tier max_usd is null
  - At least 2 tiers
  - Sorted by min_usd ascending
- Show validation errors inline

### Section 2: IP Allowlist
- `web/poller/src/lib/components/settings/IPAllowlist.svelte`
- Props: none (fetches own data)
- On mount: call `getAllowlist()`
- Table: 4 columns (IP Address, Description, Added, Actions)
- IP: mono font, accent color
- Actions: "Remove" danger button → calls `removeAllowlistIP(id)`, refreshes list
- Add form below table: IP input (mono font) + optional label input + "Add" button
- On add: calls `addAllowlistIP(ip, description)`, refreshes list
- Handles errors (invalid IP, duplicate)

### Section 3: Watch Defaults
- `web/poller/src/lib/components/settings/WatchDefaults.svelte`
- Props: `defaults: WatchDefaults`, event: `onsave: (defaults: WatchDefaults) => void`
- 2-column grid with two form groups:
  - Default Timeout (minutes): number input, 1-120 range
  - Max Active Watches: number input, min 1
- "Save Defaults" button → calls `updateWatchDefaults(defaults)` API
- Note below: "Changes take effect immediately but are lost on restart"

### Section 4: System Information
- `web/poller/src/lib/components/settings/SystemInfo.svelte`
- Props: `settings: AdminSettings`, `health: HealthResponse`
- 2-column grid of info items:
  - Uptime: from health response
  - Version: from health response
  - Network: badge (TESTNET/MAINNET)
  - Active Watches: from stats
  - Database Size: from settings (or health)
  - Start Date: formatted from settings.start_date (unix timestamp)
  - Total Transactions: from stats
  - Total Points Awarded: from stats

### Settings page
- `web/poller/src/routes/settings/+page.svelte`
- On mount: fetch `getSettings()` and `getHealth()`
- Render 4 sections with dividers between them
- Each section has title + description + component

<verification>
- Tier editor shows all tiers with editable inputs
- Tier 0 dimmed, last tier max_usd disabled
- Example column updates reactively
- Save validation catches gaps in tiers
- IP allowlist loads, add/remove works
- Watch defaults saves correctly
- System info shows all 8 items
- Build passes
</verification>

---

## Task 10: Block Explorer Link Helper

Create a shared utility for constructing block explorer URLs.

**File: `web/poller/src/lib/utils/explorer.ts`**

```typescript
export function getTxExplorerUrl(chain: string, txHash: string, network: string): string
```

- Uses `EXPLORER_TX_URL` for mainnet, `EXPLORER_TX_URL_TESTNET` for testnet
- For SOL composite hashes (`sig:TOKEN`): strips the `:TOKEN` suffix before building URL
- Used by TransactionTable, Errors page (stale pending TX links)

<verification>
- Correct URLs for all chains on both networks
- SOL composite hash stripping works
</verification>

---

## Task 11: Mockup Compliance Check & Polish

After all pages are built:

1. **Open each mockup** and compare against the built page:
   - `overview.html` → `/` page
   - `transactions.html` → `/transactions` page
   - `watches.html` → `/watches` page
   - `points.html` → `/points` page
   - `errors.html` → `/errors` page
   - `settings.html` → `/settings` page

2. **Check:**
   - Card styles match (border, background, radius, padding)
   - Table styles match (header bg, cell padding, borders, hover)
   - Badge styles match (chain badges, status badges)
   - Typography matches (mono font for numbers/addresses, correct sizes)
   - Spacing matches (grid gaps, margins)
   - Colors match (accent, muted, error, warning, success)

3. **Fix any drift** from mockups

4. **Final build check**: `npm run build` succeeds

<verification>
- All 6 pages visually match their mockup counterparts
- No console errors
- Build passes cleanly
</verification>

</tasks>

<success_criteria>
- All 6 dashboard pages fully implemented and rendering
- Overview: 8 stats cards + time range selector + 7 ECharts charts
- Transactions: table with 11 columns, 6 filter dropdowns/inputs, server-side pagination with page size selector (25/50/100)
- Watches: table with live countdown timers for active watches, filter chips by status and chain
- Points: 3 summary cards + accounts table with colored point values
- Errors: 3 sections (discrepancies, stale pending, system errors) with proper badges and severity indicators
- Settings: tier editor with validation, IP allowlist with add/remove, watch defaults form, system info grid
- Block explorer links work for all chains (mainnet + testnet, SOL composite hash handled)
- All pages match their HTML mockup designs (colors, layout, typography)
- `npm run build` passes with zero errors
- No `any` types, all functions have explicit return types
- All constants in `constants.ts`, no hardcoded values
</success_criteria>

<session_estimate>
2-3 sessions. Suggested split:
- **Session A**: Tasks 1-4 (dependencies, chart wrapper, overview page with all charts)
- **Session B**: Tasks 5-7 (transactions page with full filtering/pagination, watches page, points page)
- **Session C**: Tasks 8-11 (errors page, settings page with all 4 sections, explorer utility, mockup compliance)
</session_estimate>
