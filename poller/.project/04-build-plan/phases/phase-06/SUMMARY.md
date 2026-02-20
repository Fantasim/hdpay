# Phase 6 Summary: Frontend Setup & Auth

## Completed: 2026-02-20

## What Was Built
- SvelteKit project scaffolded at `web/poller/` with adapter-static (SPA mode, fallback `index.html`)
- Tailwind CSS v4 via `@tailwindcss/vite` plugin (no config file needed)
- shadcn-svelte initialized with Slate base color, dark theme, components: button, card, input, label, badge, separator
- Full design system ported from mockup tokens into `app.css` (dark-only theme matching Linear-inspired mockups)
- TypeScript types matching all backend API models (25+ interfaces)
- Constants file mirroring all backend error codes, chain metadata, nav items, explorer URLs
- API client with fetch wrapper (auto 401→/login redirect, all 20 endpoints)
- Auth store (login/logout/checkSession with Svelte writable stores)
- Login page matching mockup (centered card, brand, network badge, error display)
- Sidebar component (240px fixed, 6 nav items with SVG icons, active state, logout, network badge)
- Header component (title + optional subtitle + actions snippet slot)
- Root layout with auth gating (session check on mount, redirect to /login, loading spinner)
- Formatting utilities (truncateAddress, formatUsd, formatPoints, formatDate, formatRelativeTime, etc.)
- 6 stub route pages (overview, transactions, watches, points, errors, settings)

## Files Created/Modified
- `web/poller/` — entire SvelteKit project scaffolded
- `web/poller/svelte.config.js` — adapter-static with fallback
- `web/poller/vite.config.ts` — Tailwind v4 plugin + `/api` proxy to localhost:8081
- `web/poller/src/app.css` — Design system: shadcn variables + Poller mockup tokens
- `web/poller/src/routes/+layout.ts` — SPA mode (prerender=true, ssr=false)
- `web/poller/src/routes/+layout.svelte` — Auth gating, sidebar, loading state, ModeWatcher
- `web/poller/src/routes/+page.svelte` — Overview stub
- `web/poller/src/routes/login/+page.svelte` — Login page (matches mockup)
- `web/poller/src/routes/transactions/+page.svelte` — Transactions stub
- `web/poller/src/routes/watches/+page.svelte` — Watches stub
- `web/poller/src/routes/points/+page.svelte` — Points stub
- `web/poller/src/routes/errors/+page.svelte` — Errors stub
- `web/poller/src/routes/settings/+page.svelte` — Settings stub
- `web/poller/src/lib/types.ts` — All TypeScript interfaces
- `web/poller/src/lib/constants.ts` — All frontend constants
- `web/poller/src/lib/utils/api.ts` — Fetch wrapper + all endpoint functions
- `web/poller/src/lib/utils/formatting.ts` — Number/address/date formatting
- `web/poller/src/lib/stores/auth.ts` — Auth state management
- `web/poller/src/lib/components/layout/Sidebar.svelte` — Navigation sidebar
- `web/poller/src/lib/components/layout/Header.svelte` — Page header
- `web/poller/src/lib/components/ui/` — shadcn-svelte components (button, card, input, label, badge, separator)

## Decisions Made
- **Independent frontend**: `web/poller/` is a fully standalone SvelteKit project, not sharing code with HDPay's `web/`. Shares visual design language only.
- **Dark-only app**: Both `:root` and `.dark` CSS scopes have identical values. ModeWatcher with `defaultMode="dark"`.
- **No responsive**: Desktop-only (per project decision). No mobile breakpoints.
- **shadcn-svelte over raw Tailwind**: Provides accessible, composable UI primitives (button, input, label, badge) out of the box.
- **Inline SVG icons**: Nav icons rendered as inline SVGs in Sidebar rather than importing an icon library. Keeps bundle small.
- **Vite proxy**: Dev server proxies `/api` to `localhost:8081` (Poller backend port).
- **Cookie-based auth**: `checkSession()` calls `GET /api/health` — if 401, user is not authenticated. Login sends POST `/api/admin/login`.

## Deviations from Plan
- None significant. All 12 tasks completed as planned.

## Issues Encountered
- shadcn-svelte init required `app.css` to exist first — created minimal file before running init
- shadcn-svelte `add` command prompted for confirmation — piped `echo "y"` to automate
- Missing peer dependencies after shadcn init — manually installed tailwind-variants, @lucide/svelte, tw-animate-css, clsx, tailwind-merge, bits-ui

## Notes for Next Phase
- All 6 route stubs are in place — Phase 7 will replace them with full dashboard pages
- API client has all endpoints wired — ready for data fetching in dashboard pages
- ECharts not yet installed — Phase 7 will add it for chart components
- Design tokens in app.css match mockup tokens — use `var(--color-*)` variables in all new components
- Header component accepts `actions` snippet for page-level action buttons
