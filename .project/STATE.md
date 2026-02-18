# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building (Phase 11 of 11)
- **Status:** Phase 10 complete — ready to start Phase 11: History, Settings & Deployment
- **Last session:** Phase 10 completed (Send Interface — unified preview/execute API, gas pre-seed, TX SSE hub, wizard UI, 47 tests)

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
| 11 | History, Settings & Deployment | **NEXT** |

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
- **BTC MultiPrevOutFetcher**: Critical for multi-input signing
- **BSC EIP-155**: Chain ID 56 (mainnet) / 97 (testnet), LegacyTx via go-ethereum v1.17.0
- **BSC manual ABI**: BEP-20 transfer uses 0xa9059cbb + LeftPadBytes (simpler than abi.Pack)
- **BSC 20% gas buffer**: SuggestGasPrice * 12/10 to prevent stuck TXs
- **BSC sequential nonce**: PendingNonceAt once + local increment for batch sends
- **SOL sequential sends**: Per-address sends (not multi-signer batch) due to 1232-byte TX limit
- **SOL zero SDK**: Raw binary serialization from scratch — only crypto/ed25519 + base58
- **SOL legacy TX**: Non-versioned transaction format for max validator compatibility
- **Explorer URLs**: mempool.space (BTC), bscscan.com (BSC), solscan.io (SOL)
- **Unified send API**: Single preview/execute endpoints dispatch to chain-specific TX engines
- **GitHub repo**: https://github.com/Fantasim/hdpay

## Next Actions
- Run `/cf-next` to start Phase 11: History, Settings & Deployment
- Phase 11 delivers: Transaction history page, settings page, build script, final integration
- This is the final phase — completing it finishes V1

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/04-build-plan/phases/phase-10/SUMMARY.md` | Phase 10 completion summary |
| `.project/04-build-plan/phases/phase-11/PLAN.md` | Phase 11 build plan (outline) |
| `CLAUDE.md` | Code conventions and project guidelines |
| `CHANGELOG.md` | Session changelog |
