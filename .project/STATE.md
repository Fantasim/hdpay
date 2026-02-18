# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building (Phase 3 of 11)
- **Status:** Phase 2 complete — ready to start Phase 3: Address Explorer
- **Last session:** Phase 2 completed (BIP-39 mnemonic, BTC/BSC/SOL derivation, init CLI, export, 37 tests at 83.3%)

## Build Progress

| # | Phase | Status |
|---|-------|--------|
| 1 | Project Foundation | **DONE** |
| 2 | HD Wallet & Address Generation | **DONE** |
| 3 | Address Explorer | **NEXT** |
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
- **BTC BIP-84**: Uses purpose=84 for Native SegWit bech32 (not BIP-44)
- **SOL SLIP-10**: Manual ed25519 implementation, zero extra deps
- **GitHub repo**: https://github.com/Fantasim/hdpay

## Next Actions
- Run `/cf-next` to start Phase 3: Address Explorer
- Phase 3 delivers: Address listing API endpoints, paginated address table UI, address detail view, chain filtering

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/04-build-plan/phases/phase-02/SUMMARY.md` | Phase 2 completion summary |
| `.project/04-build-plan/phases/phase-03/PLAN.md` | Phase 3 build plan |
| `CLAUDE.md` | Code conventions and project guidelines |
| `CHANGELOG.md` | Session changelog |
