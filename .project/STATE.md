# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building (Phase 4 of 11)
- **Status:** Phase 3 complete — ready to start Phase 4: Scanner Engine (Backend)
- **Last session:** Phase 3 completed (Address Explorer — REST API, paginated table, chain tabs, filters, 11 new tests)

## Build Progress

| # | Phase | Status |
|---|-------|--------|
| 1 | Project Foundation | **DONE** |
| 2 | HD Wallet & Address Generation | **DONE** |
| 3 | Address Explorer | **DONE** |
| 4 | Scanner Engine (Backend) | **NEXT** |
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
- **BTC BIP-84**: Uses purpose=84 for Native SegWit bech32 (not BIP-44)
- **SOL SLIP-10**: Manual ed25519 implementation, zero extra deps
- **GitHub repo**: https://github.com/Fantasim/hdpay

## Next Actions
- Run `/cf-next` to start Phase 4: Scanner Engine (Backend)
- Phase 4 delivers: Provider interface, round-robin rotation, rate limiting, BTC/BSC/SOL scanner providers, scan orchestrator

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/04-build-plan/phases/phase-03/SUMMARY.md` | Phase 3 completion summary |
| `.project/04-build-plan/phases/phase-04/PLAN.md` | Phase 4 build plan |
| `CLAUDE.md` | Code conventions and project guidelines |
| `CHANGELOG.md` | Session changelog |
