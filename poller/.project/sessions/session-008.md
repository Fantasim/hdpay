# Session 008 — 2026-02-20

## Version: V1
## Phase: building (Phase 6: Frontend Setup & Auth)
## Summary: Phase 6 (Frontend Setup & Auth) completed. SvelteKit project at web/poller/ with adapter-static, Tailwind v4, shadcn-svelte. Design system ported from mockups. Login page, sidebar, header, auth store, API client (20 endpoints), types, constants, formatting utils, 6 stub route pages. Build passes.

## What Was Done
- Expanded Phase 6 outline PLAN.md into detailed 12-task plan
- Scaffolded SvelteKit project at `web/poller/` with adapter-static (SPA mode)
- Configured Tailwind CSS v4 via `@tailwindcss/vite` plugin (no tailwind.config.js)
- Configured Vite dev proxy: `/api` → `http://localhost:8081`
- Initialized shadcn-svelte with Slate base color, dark theme
- Installed shadcn components: button, card, input, label, badge, separator
- Ported design system from mockup tokens.css into app.css (dark-only, `:root` = `.dark`)
- Created TypeScript types matching all backend API models (25+ interfaces)
- Created constants file mirroring backend error codes, chain metadata, nav items, explorer URLs
- Built API client with fetch wrapper (auto 401 redirect) and 20 endpoint functions
- Built auth store with Svelte writable stores (login, logout, checkSession)
- Built login page matching mockup (centered card, brand icon, error, network badge)
- Built Sidebar component (240px fixed, 6 nav items with SVG icons, active state)
- Built Header component (title, subtitle, actions snippet)
- Built root layout with auth gating (session check, redirect, loading spinner, ModeWatcher)
- Created formatting utilities (truncateAddress, formatUsd, formatPoints, etc.)
- Created 6 stub route pages (overview, transactions, watches, points, errors, settings)
- Verified `npm run build` passes cleanly

## Decisions Made
- **Independent frontend**: `web/poller/` is standalone, not sharing code with HDPay's `web/`. Shares visual design language only.
- **Dark-only**: Both `:root` and `.dark` scopes have identical values. ModeWatcher defaultMode="dark".
- **Inline SVG icons**: Nav icons as inline SVGs in Sidebar rather than icon library to keep bundle small.
- **No responsive**: Desktop-only, no mobile breakpoints (per project decision).

## Issues / Blockers
- shadcn-svelte init required app.css to exist first — created minimal file before running init
- shadcn-svelte `add` command interactive — automated with `echo "y" |`
- Missing peer dependencies after shadcn init — manually installed tailwind-variants, @lucide/svelte, tw-animate-css, clsx, tailwind-merge, bits-ui

## Next Steps
- Phase 7: Dashboard Pages — replace all 6 stubs with real data-fetching pages
- Install Apache ECharts for chart components
- Build overview dashboard, transaction table, watch management, points ledger, error dashboard, settings panel
