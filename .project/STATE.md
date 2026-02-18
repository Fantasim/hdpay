# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building (Phase 2 of 11)
- **Status:** Phase 1 complete — ready to start Phase 2: HD Wallet & Address Generation
- **Last session:** Phase 1 completed (Go module, SQLite, logging, Chi router, security middleware, SvelteKit scaffold, sidebar layout)

## Build Progress

| # | Phase | Status |
|---|-------|--------|
| 1 | Project Foundation | **DONE** |
| 2 | HD Wallet & Address Generation | **NEXT** |
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
- **Tech stack locked**: Go 1.22+ (Chi, SQLite/modernc, slog) + SvelteKit (adapter-static, TS strict, Tailwind v4)
- **Svelte 5**: Using runes syntax ($props, $state, $derived)
- **Go PATH**: `/usr/local/go/bin` must be added to PATH
- **Tailwind v4**: Using @tailwindcss/vite plugin, @theme directive in app.css
- **GitHub repo**: https://github.com/Fantasim/hdpay

## Next Actions
- Run `/cf-next` to start Phase 2: HD Wallet & Address Generation
- Phase 2 delivers: BIP-44 key derivation, BTC/BSC/SOL address generation, `init` CLI command, SQLite address storage

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/04-build-plan/phases/phase-01/SUMMARY.md` | Phase 1 completion summary |
| `.project/04-build-plan/phases/phase-02/PLAN.md` | Phase 2 build plan |
| `CLAUDE.md` | Code conventions and project guidelines |
| `CHANGELOG.md` | Session changelog |
