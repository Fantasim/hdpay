# Project State: HDPay

> A self-hosted cryptocurrency payment tool that derives HD wallet addresses (BTC, BSC, SOL), scans balances via free-tier APIs, tracks transactions locally, and enables batch fund consolidation — all via a localhost Svelte dashboard.

## Current Position
- **Phase:** building (Phase 9 of 11)
- **Status:** Phase 8 complete — ready to start Phase 9: SOL Transaction Engine
- **Last session:** Phase 8 completed (BSC Transaction Engine + Gas Pre-Seed — BSC key derivation, native BNB + BEP-20 transfers, EIP-155 signing, receipt polling, consolidation service, gas pre-seeding, 22 tests)

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
| 9 | SOL Transaction Engine | **NEXT** |
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
- **BTC MultiPrevOutFetcher**: Critical for multi-input signing
- **BSC EIP-155**: Chain ID 56 (mainnet) / 97 (testnet), LegacyTx via go-ethereum v1.17.0
- **BSC manual ABI**: BEP-20 transfer uses 0xa9059cbb + LeftPadBytes (simpler than abi.Pack)
- **BSC 20% gas buffer**: SuggestGasPrice * 12/10 to prevent stuck TXs
- **BSC sequential nonce**: PendingNonceAt once + local increment for batch sends
- **GitHub repo**: https://github.com/Fantasim/hdpay

## Next Actions
- Run `/cf-next` to start Phase 9: SOL Transaction Engine
- Phase 9 delivers: SOL native + SPL token transfers, multi-instruction consolidation TXs
- `KeyService` needs `DeriveSOLPrivateKey` (SLIP-10 ed25519 path m/44'/501'/N'/0')
- SOL uses different confirmation model (not receipt polling — uses commitment levels)

## Files Reference
| File | Purpose |
|------|---------|
| `.project/state.json` | Machine-readable state |
| `.project/STATE.md` | This file — resume context |
| `.project/04-build-plan/phases/phase-08/SUMMARY.md` | Phase 8 completion summary |
| `.project/04-build-plan/phases/phase-09/PLAN.md` | Phase 9 build plan (outline) |
| `CLAUDE.md` | Code conventions and project guidelines |
| `CHANGELOG.md` | Session changelog |
