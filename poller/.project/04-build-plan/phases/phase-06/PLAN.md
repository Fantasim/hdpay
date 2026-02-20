# Phase 6: Frontend Setup & Auth (Outline)

> Will be expanded into a detailed plan before building.

<objective>
Scaffold the SvelteKit project at `web/poller/`, configure Tailwind + shadcn-svelte, port the design system from mockups (tokens.css, components.css), build the sidebar/header layout, implement the login page, and set up auth-gated routing. By the end, the dashboard shell is functional with login and navigation.
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
</constraints>

<tasks_outline>
1. SvelteKit project init at `web/poller/` (adapter-static for SPA mode, TypeScript strict, Tailwind, shadcn-svelte)
2. Port design system from `.project/03-mockups/` (tokens.css variables, components.css classes)
3. TypeScript types (`web/poller/src/lib/types.ts` — all interfaces from PROMPT.md)
4. Constants file (`web/poller/src/lib/constants.ts` — all frontend constants from PROMPT.md, mirror backend error codes)
5. API client utility (`web/poller/src/lib/utils/api.ts` — fetch wrapper with error handling, auto-redirect to /login on 401)
6. Auth store (`web/poller/src/lib/stores/auth.ts` — login/logout/check session state)
7. Login page (`routes/login/+page.svelte` — match login.html mockup exactly)
8. Layout components (Sidebar.svelte: 240px, 6 nav items + logout; Header.svelte: page title)
9. Root layout (`+layout.svelte` — auth check, redirect to /login if no session)
10. Formatting utilities (`web/poller/src/lib/utils/formatting.ts` — address truncation, number/USD formatting, date formatting, relative time)
</tasks_outline>

<research_needed>
- SvelteKit adapter-static configuration for SPA mode (all routes → index.html fallback)
- shadcn-svelte setup with Tailwind dark theme
- HDPay's frontend for reference patterns (if it has a web/ directory)
</research_needed>
