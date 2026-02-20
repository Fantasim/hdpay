# Phase 6: Frontend Setup & Auth

<objective>
Scaffold the SvelteKit project at `web/poller/`, configure Tailwind v4 + shadcn-svelte, port the design system from mockups, build the sidebar/header layout, implement the login page, and set up auth-gated routing. By the end, the dashboard shell is functional with login and navigation.
</objective>

<features>
F23 — Single Binary + Embedded SPA (scaffold only — embedding in Phase 8)
F26 — Login Page (username + password form, error display, redirect on success)
F27 — Layout (sidebar with 6 pages + logout, header with page title, auth-gated routing)
</features>

<constraints>
- Desktop-only — no responsive design (explicitly deferred)
- Manual refresh only — no auto-refresh or SSE
- Match HDPay's dashboard theme exactly (dark, Linear-inspired)
- Dark-only theme — no light mode toggle needed
</constraints>

<research_completed>
- SvelteKit adapter-static: use `fallback: 'index.html'`, `+layout.ts` with `prerender=true, ssr=false` (matches HDPay's web/)
- Tailwind v4: use `@tailwindcss/vite` plugin, no `tailwind.config.js` needed, `@import "tailwindcss"` in app.css
- shadcn-svelte: `npx shadcn-svelte@latest init`, dark mode via `mode-watcher` with `defaultMode="dark"`
- HDPay's `web/` exists and uses the exact same stack — reference for patterns
</research_completed>

## Tasks

### Task 1: SvelteKit Project Scaffolding
**Files**: `web/poller/` (new directory tree)

1. Run `npx sv create web/poller` with TypeScript, SvelteKit minimal template
2. Configure `svelte.config.js`:
   - adapter-static with `fallback: 'index.html'`, `pages: 'build'`, `assets: 'build'`
3. Configure `vite.config.ts`:
   - Add `@tailwindcss/vite` plugin (before sveltekit plugin)
   - Add dev server proxy: `/api` → `http://localhost:8081` (Poller backend port)
4. Create `src/routes/+layout.ts`:
   ```ts
   export const prerender = true;
   export const ssr = false;
   ```
5. Install dependencies:
   - `tailwindcss @tailwindcss/vite` (Tailwind v4)
   - `mode-watcher` (dark mode)

**Verification**: `npm run build` produces `build/index.html`, `npm run dev` starts on port 5173.

---

### Task 2: shadcn-svelte Initialization
**Files**: `web/poller/` (modifies app.css, creates components.json)

1. Run `npx shadcn-svelte@latest init` with Slate base color, dark mode
2. Add needed components: `button`, `card`, `input`, `label`, `badge`, `select`, `separator`
3. Verify `components.json` is created and `$lib/components/ui/` has the component files

**Verification**: Import a Button component in a test page, confirm it renders with dark styling.

---

### Task 3: Design System — app.css with Tokens
**Files**: `web/poller/src/app.css`

Port design tokens from `.project/03-mockups/styles/tokens.css` into Tailwind v4's `@theme` block. Match HDPay's `web/src/app.css` approach exactly:

1. `@import "tailwindcss"` at the top
2. `@theme { ... }` block with all color tokens, typography, spacing, layout variables
3. Base body styles (background, color, font, antialiasing)
4. Scrollbar styles
5. Keep shadcn-svelte's injected CSS variables intact (from init step)

Key tokens to port from `tokens.css`:
- Background colors (bg-primary through bg-input)
- Sidebar colors
- Text colors (primary, secondary, muted, disabled)
- Accent colors
- Border colors
- Chain colors (BTC, BSC, SOL)
- Status colors (success, error, warning, info)
- Font families (Inter, JetBrains Mono)
- Font sizes (xs through 4xl)
- Spacing scale
- Layout vars (sidebar-width: 240px, header-height: 56px)
- Border radii, shadows, transitions, z-index

**Verification**: Variables available in dev tools, body has correct dark background.

---

### Task 4: TypeScript Types
**Files**: `web/poller/src/lib/types.ts` (new)

Define all TypeScript interfaces matching the backend API responses from PROMPT.md:

```ts
// Core types
type Chain = 'BTC' | 'BSC' | 'SOL'
type Token = 'BTC' | 'BNB' | 'SOL' | 'USDC' | 'USDT'
type WatchStatus = 'ACTIVE' | 'COMPLETED' | 'EXPIRED' | 'CANCELLED'
type TxStatus = 'PENDING' | 'CONFIRMED'
type TimeRange = 'today' | 'week' | 'month' | 'quarter' | 'all'

// API response wrappers
interface APIResponse<T> { data: T }
interface APIListResponse<T> { data: T[]; meta: PaginationMeta }
interface APIErrorResponse { error: { code: string; message: string } }
interface PaginationMeta { page: number; pageSize: number; total: number }

// Domain types matching backend models
interface Watch { ... }
interface Transaction { ... }
interface PointsAccount { ... }
interface PendingPointsAccount { ... }
interface ClaimResult { ... }
interface DashboardStats { ... }
interface ChartData { ... }
interface IPAllowlistEntry { ... }
interface SystemError { ... }
interface DiscrepancyResult { ... }
interface Tier { ... }
interface AdminSettings { ... }
```

All field names must match the JSON keys from backend API responses exactly.

**Verification**: No TypeScript errors when importing types.

---

### Task 5: Constants File
**Files**: `web/poller/src/lib/constants.ts` (new)

Port all frontend constants from PROMPT.md `constants.ts` section:

- `API_BASE = '/api'`
- Display constants (decimal places, truncate length)
- Chain arrays, colors, native symbols, tokens
- Watch/TX status arrays and colors
- Time range options and labels
- Chart colors
- Block explorer URLs (mainnet + testnet)
- All error codes mirroring backend `config/errors.go`

**Verification**: All constants importable, no hardcoded values remain in components.

---

### Task 6: API Client Utility
**Files**: `web/poller/src/lib/utils/api.ts` (new)

Build a fetch wrapper inspired by HDPay's `web/src/lib/utils/api.ts` but adapted for Poller:

1. Core `request<T>(method, path, body?)` function
2. Auto-redirect to `/login` on 401 response (session expired)
3. No CSRF token needed (Poller uses IP allowlist + session cookie, not CSRF)
4. `ApiError` class with `code`, `message`, `status` fields
5. `api` object with `.get()`, `.post()`, `.put()`, `.delete()` methods
6. Specific API functions:
   - Auth: `login(username, password)`, `logout()`, `checkSession()`
   - Watch: `createWatch(chain, address, timeoutMin?)`, `cancelWatch(id)`, `listWatches(filters?)`
   - Points: `getPoints()`, `getPendingPoints()`, `claimPoints(addresses)`
   - Admin: `getSettings()`, `updateTiers(tiers)`, `updateWatchDefaults(defaults)`, `getAllowlist()`, `addAllowlistIP(ip, desc)`, `removeAllowlistIP(id)`
   - Dashboard: `getDashboardStats(range)`, `getDashboardTransactions(params)`, `getDashboardCharts(range)`, `getDashboardErrors()`

**Verification**: Functions have correct return types, import works.

---

### Task 7: Auth Store
**Files**: `web/poller/src/lib/stores/auth.ts` (new)

Create a Svelte store for auth state:

1. `isAuthenticated` writable store (boolean)
2. `isLoading` writable store (boolean) — for initial session check
3. `authError` writable store (string | null) — for login error display
4. `login(username, password)` — calls API, sets isAuthenticated on success
5. `logout()` — calls API, clears isAuthenticated, redirects to /login
6. `checkSession()` — calls GET /api/admin/settings (or /api/health with auth), sets isAuthenticated
7. On 401 anywhere: auto-set isAuthenticated=false

**Verification**: Store updates correctly on login/logout flows.

---

### Task 8: Login Page
**Files**: `web/poller/src/routes/login/+page.svelte` (new)

Match the `login.html` mockup exactly:

1. Full-page centered layout with gradient background
2. Login card (400px wide, surface background, rounded corners, shadow)
3. Brand section: "P" icon with accent background + "Poller" text
4. Subtitle: "Crypto-to-Points Dashboard"
5. Error alert (hidden by default, shown on invalid credentials)
6. Form: username input, password input, "Sign In" button (full width, primary style)
7. Network badge at bottom (TESTNET/MAINNET based on /api/health response)
8. Slide-up animation on card

On successful login → redirect to `/` (overview page).
Use shadcn-svelte Button and Input components where appropriate.

**Verification**: Page matches mockup visually. Login works against running backend.

---

### Task 9: Layout Components
**Files**:
- `web/poller/src/lib/components/layout/Sidebar.svelte` (new)
- `web/poller/src/lib/components/layout/Header.svelte` (new)

**Sidebar.svelte** (match mockup's sidebar exactly):
1. Fixed position, 240px wide, full height
2. Brand: "P" icon + "Poller" text at top
3. "DASHBOARD" section label
4. Nav items with icons (use inline SVG or lucide-svelte):
   - Overview (`/`)
   - Transactions (`/transactions`)
   - Watches (`/watches`)
   - Pending Points (`/points`)
   - Errors (`/errors`)
   - Settings (`/settings`)
5. Active state highlighting based on current route (`$page.url.pathname`)
6. Footer: Logout button + network badge (TESTNET/MAINNET)

**Header.svelte**:
1. Page title (passed as prop)
2. Optional subtitle
3. Optional right-side actions slot
4. Bottom border
5. Min-height matching header-height token

**Verification**: Sidebar shows correct active item per route, logout works.

---

### Task 10: Root Layout with Auth Gating
**Files**: `web/poller/src/routes/+layout.svelte` (modify)

1. Import app.css
2. Import ModeWatcher from mode-watcher, set defaultMode="dark"
3. On mount: call `checkSession()` from auth store
4. While loading: show nothing (or minimal spinner)
5. If authenticated: render sidebar + header + page content
6. If NOT authenticated AND not on `/login`: redirect to `/login`
7. If on `/login`: render just the page content (no sidebar/header)

**Verification**: Unauthenticated → redirected to login. Authenticated → see dashboard shell.

---

### Task 11: Formatting Utilities
**Files**: `web/poller/src/lib/utils/formatting.ts` (new)

Port relevant formatting functions (adapted from HDPay's `web/src/lib/utils/formatting.ts`):

1. `truncateAddress(address, length?)` — show start...end
2. `formatUsd(amount)` — "$1,234.56"
3. `formatNumber(n)` — comma-separated integers
4. `formatPoints(n)` — comma-separated, no decimals
5. `formatDate(dateStr)` — "Feb 20, 2026, 10:30 AM"
6. `formatRelativeTime(dateStr)` — "2 min ago", "1 hour ago"
7. `formatCountdown(expiresAt)` — "12m 34s" remaining (for active watches)
8. `copyToClipboard(text)` — clipboard utility

**Verification**: Functions produce expected output for sample inputs.

---

### Task 12: Stub Pages for All Routes
**Files**: `web/poller/src/routes/*/+page.svelte` (new)

Create minimal stub pages for each route so navigation works:

1. `/` (overview) — "Overview" title, "Coming in Phase 7" placeholder
2. `/transactions` — "Transactions" title, placeholder
3. `/watches` — "Watches" title, placeholder
4. `/points` — "Pending Points" title, placeholder
5. `/errors` — "Errors" title, placeholder
6. `/settings` — "Settings" title, placeholder

Each page should use the Header component with the correct title.

**Verification**: All 6 sidebar links navigate to the correct page with the correct title.

---

<success_criteria>
1. `npm run dev` starts the SvelteKit dev server without errors
2. `npm run build` produces `build/index.html` (SPA fallback)
3. Navigating to any page without a session redirects to `/login`
4. Login page matches the `login.html` mockup visually (dark theme, centered card, brand, form)
5. Successful login with correct credentials redirects to overview page
6. Failed login shows error alert
7. Dashboard layout shows 240px sidebar with 6 nav items + logout
8. Clicking each nav item navigates to the correct page with correct header title
9. Active sidebar item is highlighted
10. Logout clears session and redirects to login
11. Network badge shows TESTNET/MAINNET based on backend
12. All TypeScript types compile without errors
13. No hardcoded values — all constants in `constants.ts`
14. No `any` types anywhere
</success_criteria>

<estimated_sessions>1</estimated_sessions>
