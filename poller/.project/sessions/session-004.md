# Session 004 — 2026-02-20

## Version: V1
## Phase: building (Phase 2: Core Services)
## Summary: Phase 2 (Core Services) completed — tier system, points calculator, price wrapper, address validation with comprehensive tests.

## What Was Done
- Created tier configuration system (`internal/poller/points/tiers.go`):
  - LoadTiers, ValidateTiers, CreateDefaultTiers, LoadOrCreateTiers
  - 9-tier default config auto-created if tiers.json missing
  - Full validation: sorted, no gaps, unbounded last tier, min >= 0, multiplier >= 0
- Created points calculator (`internal/poller/points/calculator.go`):
  - Flat tier matching (entire amount uses single tier multiplier)
  - Formula: cents = floor(USD * 100), points = round(cents * multiplier)
  - Thread-safe via RWMutex, hot-reloadable via Reload() for dashboard saves
- Created price service wrapper (`internal/poller/points/pricer.go`):
  - Thin wrapper over HDPay's CoinGecko PriceService
  - Stablecoins (USDC/USDT) short-circuit to $1.00 without API call
  - 3× retry with 5s delay for native token price fetches
  - Context-aware cancellation during retries
- Created address validation (`internal/poller/validate/address.go`):
  - BTC: btcutil.DecodeAddress + IsForNet for network-aware validation
  - BSC: regex ^0x[0-9a-fA-F]{40}$ (network-independent)
  - SOL: mr-tron/base58 decode to exactly 32 bytes (network-independent)
- Wrote comprehensive tests (4 test files):
  - All 12 tier boundary values from plan verified
  - Edge cases: zero, large amounts, fractional cents, reload, copy safety
  - Price retry, all-fail, context cancel, stablecoin, unknown token
  - Known testnet addresses from MEMORY.md as test vectors

## Decisions Made
- Used `mr-tron/base58` for SOL validation (already in go.mod, returns errors unlike btcutil/base58)
- Added `IsForNet()` check after `DecodeAddress` because btcutil accepts cross-network addresses
- Used `errors.Is(err, fs.ErrNotExist)` instead of `os.IsNotExist` for wrapped error detection
- Used plain `string` for Pricer token param (not models.Token) for flexibility

## Issues / Blockers
- `os.IsNotExist` doesn't unwrap `fmt.Errorf("%w")` — fixed with `errors.Is(err, fs.ErrNotExist)`
- `btcutil.DecodeAddress` accepts testnet addresses on mainnet params — fixed with `IsForNet()` check

## Next Steps
- Start Phase 3: Blockchain Providers
- Build BTC/BSC/SOL tx detection providers (different from HDPay's balance scanning)
- Implement Provider interface with round-robin rotation
- Integrate rate limiting and circuit breaker from HDPay
