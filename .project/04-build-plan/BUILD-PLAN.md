# HDPay — Build Plan

> Phased build plan from project foundation to V1 deployment.

## Overview

HDPay V1 is built in **11 phases**, progressing from infrastructure through wallet generation, scanning, dashboard, transaction engines, and finally the send interface with history/settings. Each phase is designed to be completable in a single coding session (3-5 hours) and produces a working, testable increment.

## Phase Summary

| # | Phase | Features Covered | Complexity | Key Deliverables |
|---|-------|-----------------|------------|-----------------|
| 1 | Project Foundation | #1 | M | Go module, SQLite, slog, Chi router, security middleware, SvelteKit scaffold, sidebar layout |
| 2 | HD Wallet & Address Generation | #2 | L | BIP-44 derivation (BTC/BSC/SOL), init CLI, batch DB insert, JSON export, test vectors |
| 3 | Address Explorer | #3, #16 | S | Address API (paginated), export endpoint, frontend page, virtual scroll, chain tabs, filters |
| 4 | Scanner Engine (Backend) | #4 | L | Provider interface, 7 providers, rate limiter, round-robin pool, scanner orchestrator, resume, token scanning, SSE hub |
| 5 | Scan UI + SSE | #5, #17, #18 | M | Scan API endpoints, SSE stream, frontend scan page, progress bars, provider status, scan history |
| 6 | Dashboard & Price Service | #6, #7, #14 | M | CoinGecko price API, balance summary API, dashboard frontend, ECharts charts |
| 7 | BTC Transaction Engine | #8, #19 | L | UTXO fetching, multi-input TX building, P2WPKH signing, dynamic fee estimation, broadcast |
| 8 | BSC Transaction Engine + Gas Pre-Seed | #9, #11 | L | BNB transfers, BEP-20 transfers, nonce management, gas pre-seeding |
| 9 | SOL Transaction Engine | #10 | L | Multi-instruction batch, SPL token transfers, ATA derivation, batch splitting |
| 10 | Send Interface | #12 | L | Send preview/execute/gas-preseed APIs, wizard-style frontend, TX progress SSE, receipt |
| 11 | History, Settings & Deployment | #13, #15 | M | Transaction history API+UI, settings API+UI, build script, embedded binary, final integration |

## Dependency Graph

```
Phase 1: Foundation
    │
    ▼
Phase 2: Wallet & Addresses
    │
    ├──────────────────┐
    ▼                  ▼
Phase 3: Explorer    Phase 4: Scanner Engine
                       │
                       ▼
                     Phase 5: Scan UI
                       │
                       ▼
                     Phase 6: Dashboard & Prices
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
        Phase 7:     Phase 8:     Phase 9:
        BTC TX       BSC TX+Gas   SOL TX
          │            │            │
          └────────────┼────────────┘
                       ▼
                     Phase 10: Send Interface
                       │
                       ▼
                     Phase 11: History, Settings & Deploy
```

## Session Model

- **Phases 1-3**: Detailed PLAN.md (ready to build immediately)
- **Phases 4-11**: Outline PLAN.md (expanded when reached, using context from prior phases)
- Each phase ends with a SUMMARY.md documenting what was built, decisions made, and notes for next phase
- Commit after each phase completion

## Tech Stack (Locked)

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+, Chi v5, SQLite/modernc, slog, envconfig |
| Crypto | btcsuite/btcd, go-ethereum, solana-go, go-bip39 |
| Frontend | SvelteKit (adapter-static), TypeScript strict, Tailwind+shadcn-svelte |
| Viz | ECharts, @tanstack/svelte-virtual |
| Testing | Go stdlib, Vitest+Testing Library |

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|-----------|
| SOL SLIP-10 derivation mismatch | High | Test against Phantom wallet addresses with known mnemonic |
| Provider rate limits too restrictive | Medium | Round-robin rotation + exponential backoff + multiple providers per chain |
| BTC UTXO transaction too large | Medium | Split into multiple transactions if > 100KB |
| BSC nonce management race conditions | Medium | Sequential processing with nonce locking |
| SOL transaction size limit | Medium | Batch splitting at ~20 instructions |
| SQLite performance with 1.5M rows | Low | Indexed columns, WAL mode, batched inserts |
