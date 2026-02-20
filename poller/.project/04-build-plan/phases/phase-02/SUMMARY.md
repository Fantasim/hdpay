# Phase 2 Summary: Core Services

## Completed: 2026-02-20

## What Was Built
- Tier configuration system (load from JSON, validate, create defaults, auto-create on first run)
- Points calculator with flat tier matching (USD → tier → cents × multiplier → rounded integer points)
- Price service wrapper over HDPay's CoinGecko PriceService (stablecoin short-circuit, retry logic)
- Address validation for BTC (bech32 via btcutil + network check), BSC (hex regex), SOL (base58 → 32 bytes)

## Files Created/Modified
- `internal/poller/points/tiers.go` — LoadTiers, ValidateTiers, CreateDefaultTiers, LoadOrCreateTiers
- `internal/poller/points/calculator.go` — PointsCalculator (thread-safe, hot-reloadable), Calculate method
- `internal/poller/points/pricer.go` — Pricer wrapper over HDPay PriceService, GetTokenPrice with retry
- `internal/poller/validate/address.go` — Address(chain, addr, network) validator
- `internal/poller/points/tiers_test.go` — 13 test cases (loading, validation rules, defaults, file creation)
- `internal/poller/points/calculator_test.go` — 12 tier boundary tests + edge cases (zero, large, fractional, reload, copy safety)
- `internal/poller/points/pricer_test.go` — Stablecoin, native tokens, unknown token, retry, all-fail, context cancel
- `internal/poller/validate/address_test.go` — Known testnet addresses (BTC/BSC/SOL), invalid formats, network independence

## Decisions Made
- **mr-tron/base58 for SOL**: Used existing `mr-tron/base58` (already in go.mod) instead of btcutil/base58 because it returns errors on invalid input
- **btcutil.DecodeAddress + IsForNet for BTC**: DecodeAddress alone doesn't reject cross-network addresses; added explicit IsForNet check
- **errors.Is for file detection**: Used `errors.Is(err, fs.ErrNotExist)` instead of `os.IsNotExist` because LoadTiers wraps the underlying error with fmt.Errorf
- **Token as string in Pricer**: Used plain `string` for token parameter (not models.Token) since Poller's token values come from various sources and the pricer maps them internally

## Deviations from Plan
- None — all tasks completed as specified

## Issues Encountered
- `os.IsNotExist` doesn't unwrap `fmt.Errorf("%w")` errors — fixed by using `errors.Is(err, fs.ErrNotExist)`
- `btcutil.DecodeAddress` accepts testnet addresses with mainnet params — fixed by adding `decoded.IsForNet(params)` check

## Notes for Next Phase
- Phase 3 (Blockchain Providers) will build tx detection providers that use the address validation from this phase
- The Pricer will be used by the watch engine (Phase 4) when transactions are confirmed
- The PointsCalculator will be called by the watch engine to compute points on confirmation
