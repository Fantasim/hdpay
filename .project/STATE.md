# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building (Phase 1 of 11)
- **Status:** Build plan complete — ready to start Phase 1: Project Foundation
- **Last session:** Build plan generated (11 phases, 3 detailed + 8 outlines)

## Build Progress

| # | Phase | Status |
|---|-------|--------|
| 1 | Project Foundation | **NEXT** |
| 2 | HD Wallet & Address Generation | Pending |
| 3 | Address Explorer | Pending |
| 4 | Scanner Engine (Backend) | Pending (outline) |
| 5 | Scan UI + SSE | Pending (outline) |
| 6 | Dashboard & Price Service | Pending (outline) |
| 7 | BTC Transaction Engine | Pending (outline) |
| 8 | BSC Transaction Engine + Gas Pre-Seed | Pending (outline) |
| 9 | SOL Transaction Engine | Pending (outline) |
| 10 | Send Interface | Pending (outline) |
| 11 | History, Settings & Deployment | Pending (outline) |

## Key Decisions
- **Tech stack locked**: Go 1.22+ (Chi, SQLite/modernc, slog) + SvelteKit (adapter-static, TS strict, Tailwind+shadcn-svelte, ECharts, tanstack-virtual)
- **Feature scope**: 13 must-have, 6 should-have, 5 deferred to V2, 10 explicitly excluded
- **Auth**: None — localhost-only with CSRF/CORS/host validation
- **No responsive**: Desktop-only localhost tool
- **Setup phase skipped**: CLAUDE.md fully specified in prework
- **11 build phases**: Each 3-5h session-sized, detailed plans for phases 1-3, outlines for 4-11

## Design Decisions (Mockup Phase)
- **Theme**: Dark, Linear-inspired
- **Accent color**: #5e6ad2 (blue-violet), chain colors for chain-specific elements only
- **Sidebar**: Full (240px, icons + labels, always visible)
- **Dashboard**: Single summary value + detailed balance table
- **Scan page**: Unified panel with chain dropdown, scans list below
- **Send page**: Wizard-style single-page flow with sequential steps
- **Typography**: Inter (UI), JetBrains Mono (addresses, balances, data)
- **Density**: Moderate
- **Min font-size**: 12px

## Feature Summary (Must-Have)
1. Project Foundation (M)
2. HD Wallet Address Generation (L)
3. Address Explorer (S)
4. Scanner Engine (L)
5. Scan Control UI (M)
6. Price Service (S)
7. Dashboard (M)
8. BTC Transaction Engine (L)
9. BSC Transaction Engine (L)
10. SOL Transaction Engine (L)
11. Gas Pre-Seeding (M)
12. Send Interface (L)
13. Transaction History (M)

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+, Chi v5, SQLite/modernc, slog, envconfig |
| Crypto | btcsuite/btcd, go-ethereum, solana-go, go-bip39 |
| Frontend | SvelteKit, TypeScript strict, Tailwind+shadcn-svelte |
| Viz | ECharts, @tanstack/svelte-virtual |
| Testing | Go stdlib, Vitest+Testing Library |

## Prework Reference
- `.prework/AGENT_PROMPT.md` — Full build specification
- `.prework/TECHNICAL_REFERENCE.md` — Deep dive on all subsystems
- `CLAUDE.md` — Code conventions, constants, file structure
- `.project/02-plan.md` — Feature plan with tiers, user stories, data model
- `.project/03-mockups/` — HTML/CSS mockups (tokens, components, 6 screens)

## Next Actions
- Run `/cf-next` to start Phase 1: Project Foundation
- Phase 1 delivers: Go module, SQLite, logging, Chi router, security middleware, SvelteKit scaffold, sidebar layout

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state (do not edit manually) |
| `.project/STATE.md` | This file — Claude-readable resume context |
| `.project/02-plan.md` | Feature plan output |
| `.project/03-mockups/` | Design mockups (tokens.css, components.css, 6 screens) |
| `.project/04-build-plan/BUILD-PLAN.md` | Build plan overview |
| `.project/04-build-plan/phases/phase-NN/PLAN.md` | Per-phase build plans |
| `.prework/TECHNICAL_REFERENCE.md` | Deep technical reference |
| `CLAUDE.md` | Code conventions and project guidelines |
