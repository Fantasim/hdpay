# Session 022 -- 2026-02-19

## Version: V2 (post-build)
## Phase: SOL Fee Payer & Security Audit
## Summary: Implemented SOL fee payer mechanism for token sweeps (eliminating need for gas pre-seeding on SOL). Conducted comprehensive security audit with 21 fixes across scanning, fund movement, and UX safety — including 5 critical, 6 high, and 5 medium severity issues.

## What Was Done

### SOL Fee Payer Mechanism
- SOL token sweeps (USDC/USDT) now use Solana's native fee payer mechanism
- Single address pays all TX fees instead of each token holder paying their own
- No gas pre-seeding required on SOL (unlike BSC)
- Added `FeePayerIndex` field on `SendRequest`
- Chain-aware GasPreSeedStep: "Fee Payer Selection" for SOL vs "Gas Pre-Seed" for BSC
- Fee payer balance validation: checks sufficient SOL for all estimated fees upfront

### Security Audit — Critical
- SOL blockhash staleness (TX-1): now tracks `lastValidBlockHeight`, estimates block consumption rate, forces refresh when nearing expiry. Cache TTL reduced from 20s to 10s
- Confirmation modal for irreversible sends (UX-2): typed "CONFIRM" + 3-second countdown
- Full destination address at execute step (UX-1): no longer truncated to 10 chars, copy button added
- Network badge at execution (UX-3): prominent MAINNET (red) / TESTNET (yellow) badge

### Security Audit — High
- BTC fee safety margin (TX-2): 2% safety margin to prevent underestimation from vsize rounding
- BTC UTXO divergence thresholds tightened (TX-3): from 20%/10% to 5%/3%
- BSC per-TX gas check (TX-5): checks gas balance before EACH transfer, skips gas-less addresses
- Scanner completion race (SC-2): `removeScan()` moved into `finishScan()` after all writes
- Double-click protection (UX-6): synchronous click guard + `pointer-events: none`
- Gas pre-seed skip warning (UX-7): warning modal explaining token transfers will fail without gas

### Security Audit — Medium
- Scan context timeout (SC-6): 24h upper bound via `context.WithTimeout`
- Token failure backoff (SC-5): exponential backoff like native failures
- Non-atomic scan state retry (SC-3): single retry with 100ms pause
- Rate limiter burst control (SC-7): `Burst(1)` for even request distribution
- HTTP connection pool limits (SC-8): `MaxConnsPerHost=10`, `MaxIdleConnsPerHost=5`

### Additional UX
- USD value display on execute step
- Fee estimate display on execute step
- TX count explanation for multi-TX chains
- SSE connection status indicator during execution
- Copy buttons for destination address and TX hashes

## Decisions Made
- **SOL fee payer vs gas pre-seeding**: SOL natively supports fee payers — simpler and cheaper than BSC's approach
- **CONFIRM + countdown**: Defense-in-depth against accidental sends on mainnet
- **SOL blockhash cache TTL = 10s**: 20s was too aggressive given ~400ms block times
- **BTC UTXO thresholds 5%/3%**: Tighter than initial 20%/10% — crypto values are high, small % matters
- **Burst(1)**: More predictable request distribution, prevents burst spikes hitting rate limits

## Issues / Blockers
- None

## Next Steps
- Network mainnet/testnet coexistence support
