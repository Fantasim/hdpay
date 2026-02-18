# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building (Phase 7 of 11)
- **Status:** Phase 6 complete — ready to start Phase 7: BTC Transaction Engine
- **Last session:** Phase 6 completed (Dashboard & Price Service — CoinGecko price service, portfolio API, dashboard page with charts, 9 tests)

## Build Progress

| # | Phase | Status |
|---|-------|--------|
| 1 | Project Foundation | **DONE** |
| 2 | HD Wallet & Address Generation | **DONE** |
| 3 | Address Explorer | **DONE** |
| 4 | Scanner Engine (Backend) | **DONE** |
| 5 | Scan UI + SSE | **DONE** |
| 6 | Dashboard & Price Service | **DONE** |
| 7 | BTC Transaction Engine | **NEXT** |
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
- **Blockchain.info dropped**: Does not support bech32 addresses
- **Manual ATA derivation**: ~160 lines custom PDA avoids full solana-go dependency
- **SSE store pattern**: `.svelte.ts` with runes, named event listeners, exponential backoff reconnect
- **ECharts**: svelte-echarts v1.0.0 with tree-shaking, $derived() rune for reactive options
- **CoinGecko**: Public API (no key), 5-min server-side cache
- **GitHub repo**: https://github.com/Fantasim/hdpay

## Next Actions
- Run `/cf-next` to start Phase 7: BTC Transaction Engine
- Phase 7 delivers: UTXO fetching, multi-input TX building, P2WPKH signing, dynamic fee estimation, broadcast

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/04-build-plan/phases/phase-06/SUMMARY.md` | Phase 6 completion summary |
| `.project/04-build-plan/phases/phase-07/PLAN.md` | Phase 7 build plan |
| `CLAUDE.md` | Code conventions and project guidelines |
| `CHANGELOG.md` | Session changelog |
