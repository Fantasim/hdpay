# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building — ALL 11 PHASES COMPLETE
- **Status:** V1 fully built. All build phases done.
- **Last session:** Phase 11 completed (History, Settings & Deployment — transaction history, settings, embedded SPA, 22MB binary)

## Build Progress

| # | Phase | Status |
|---|-------|--------|
| 1 | Project Foundation | **DONE** |
| 2 | HD Wallet & Address Generation | **DONE** |
| 3 | Address Explorer | **DONE** |
| 4 | Scanner Engine (Backend) | **DONE** |
| 5 | Scan UI + SSE | **DONE** |
| 6 | Dashboard & Price Service | **DONE** |
| 7 | BTC Transaction Engine | **DONE** |
| 8 | BSC Transaction Engine + Gas Pre-Seed | **DONE** |
| 9 | SOL Transaction Engine | **DONE** |
| 10 | Send Interface | **DONE** |
| 11 | History, Settings & Deployment | **DONE** |

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
- **Single binary**: 22MB Go binary with embedded SvelteKit SPA via `go:embed`
- **SPA fallback**: Chi `NotFound` handler serves index.html for client-side routing
- **GitHub repo**: https://github.com/Fantasim/hdpay

## V1 Complete
All 11 build phases are done. Options:
- Run `/cf-save` to save final session state
- Run `/cf-new-version` to start planning V2

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/04-build-plan/phases/phase-11/SUMMARY.md` | Phase 11 completion summary |
| `CLAUDE.md` | Code conventions and project guidelines |
| `CHANGELOG.md` | Session changelog |
