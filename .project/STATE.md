# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building (Phase 8 of 11)
- **Status:** Phase 7 complete — ready to start Phase 8: BSC Transaction Engine + Gas Pre-Seed
- **Last session:** Phase 7 completed (BTC Transaction Engine — key derivation, UTXO fetching, fee estimation, multi-input P2WPKH TX building+signing, broadcasting, transaction DB CRUD, 38 tests)

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
| 8 | BSC Transaction Engine + Gas Pre-Seed | **NEXT** |
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
- **MultiPrevOutFetcher**: Critical for multi-input BTC signing (CannedPrevOutputFetcher would produce wrong sigs)
- **Confirmed UTXOs only**: Unconfirmed UTXOs filtered out to avoid spending unconfirmed outputs
- **No mnemonic caching**: Mnemonic read from file each derivation call for security
- **text/plain broadcast**: Blockstream/Mempool expect raw hex as text/plain, not JSON
- **No retry on 400**: Bad TX (400) means TX itself is invalid, don't retry on other providers
- **GitHub repo**: https://github.com/Fantasim/hdpay

## Next Actions
- Run `/cf-next` to start Phase 8: BSC Transaction Engine + Gas Pre-Seed
- Phase 8 delivers: BSC native+BEP-20 transfers, gas pre-seeding, sequential automated consolidation
- `Broadcaster` interface and `KeyService` are ready for BSC extension

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/04-build-plan/phases/phase-07/SUMMARY.md` | Phase 7 completion summary |
| `.project/04-build-plan/phases/phase-08/PLAN.md` | Phase 8 build plan (outline) |
| `CLAUDE.md` | Code conventions and project guidelines |
| `CHANGELOG.md` | Session changelog |
