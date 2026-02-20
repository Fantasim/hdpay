# Phase 2: Core Services

<objective>
Implement the points calculation engine (tier system with tiers.json), wire HDPay's existing CoinGecko price service, and build address validation for all three chains. These are the stateless domain services that the watch engine will depend on. All must be fully tested.
</objective>

<features>
F10 — Points Calculation (cents * multiplier, flat tiers, rounded integers)
F11 — Tier Configuration (tiers.json loading, validation, default creation)
F13 — CoinGecko Price Fetching (reuse HDPay's PriceService — 60s cache, stablecoins hardcoded $1.00)
F24 — Address Format Validation (BTC bech32, BSC hex, SOL base58)
</features>

<tasks>

## Task 1: Tier Configuration & Loading

**Create** the tier system that loads from tiers.json.

**Files to create:**
- `internal/poller/points/tiers.go` — LoadTiers(), ValidateTiers(), CreateDefaultTiers(), Tier struct

**Details:**
- Tier struct: `MinUSD float64`, `MaxUSD *float64` (nil = unbounded), `Multiplier float64`
- `LoadTiers(path string)`: read JSON file, parse, validate, return `[]Tier`
- `CreateDefaultTiers(path string)`: write default 9-tier config from PROMPT.md, log INFO
- On startup: try LoadTiers → if file not found → CreateDefaultTiers → LoadTiers
- Validation rules (from PROMPT.md):
  - `min_usd >= 0`
  - Each tier's `min_usd` must equal previous tier's `max_usd` (no gaps)
  - `multiplier >= 0`
  - Last tier must have `max_usd: null`
  - At least `MinTierCount` (2) tiers
  - Sorted by `min_usd` ascending
- Return descriptive errors for each validation failure

**Verification:**
- Default tiers.json matches PROMPT.md exactly (9 tiers)
- File is created if missing
- Invalid configurations are rejected with clear error messages

## Task 2: Points Calculator

**Create** the points calculation engine.

**Files to create:**
- `internal/poller/points/calculator.go` — PointsCalculator struct, Calculate method

**Details:**
- `PointsCalculator` holds loaded tiers (refreshable via `Reload()` for dashboard saves)
- `Calculate(usdValue float64) (points int, tier int, multiplier float64)`:
  - Find matching tier for usdValue (min_usd <= value < max_usd, or min_usd <= value for last unbounded tier)
  - `cents = floor(usdValue * 100)`
  - `points = round(cents * multiplier)` using `math.Round()`
  - Return points, tier index, multiplier used
- Thread-safe: tiers protected by `sync.RWMutex` for hot-reload from dashboard
- Log DEBUG on each calculation with all inputs/outputs

**Verification:**
- $0.50 → 0 points (tier 0, multiplier 0.0)
- $1.00 → 100 points (tier 1, multiplier 1.0)
- $5.00 → 500 points (tier 1, multiplier 1.0)
- $11.99 → 1199 points (tier 1, multiplier 1.0)
- $12.00 → 1320 points (tier 2, multiplier 1.1)
- $20.00 → 2200 points (tier 2, multiplier 1.1)
- $50.00 → 6000 points (tier 3, multiplier 1.2)
- $100.00 → 13000 points (tier 4, multiplier 1.3)
- $200.00 → 28000 points (tier 5, multiplier 1.4)
- $500.00 → 75000 points (tier 6, multiplier 1.5)
- $1000.00 → 200000 points (tier 7, multiplier 2.0)
- $2000.00 → 600000 points (tier 8, multiplier 3.0)

## Task 3: Price Service (Reuse HDPay)

**Wire** HDPay's existing CoinGecko PriceService with Poller-specific behavior.

**Files to create:**
- `internal/poller/points/pricer.go` — Poller price wrapper (thin layer over HDPay's PriceService)

**What to import from HDPay (not rewrite):**
- `internal/price` — `PriceService`, `NewPriceService()`, `GetPrices(ctx)`

**Details:**
- HDPay's `PriceService` already handles: CoinGecko API, caching (5min TTL), stale fallback, BTC/BNB/SOL/USDC/USDT prices
- Poller needs a thin wrapper that:
  - Calls `GetPrices(ctx)` and extracts the price for a specific token
  - Implements Poller-specific retry logic: 3 retries with 5s delay (on price fetch failure, leave tx PENDING)
  - Hardcodes stablecoins to $1.00 (short-circuit — don't even call GetPrices for USDC/USDT)
- `GetTokenPrice(ctx context.Context, token models.Token) (float64, error)`:
  - If token is USDC or USDT → return `StablecoinPrice` (1.0) immediately
  - Map token to price key: BTC→"BTC", BNB→"BNB", SOL→"SOL"
  - Retry up to `PriceRetryCount` with `PriceRetryDelay` between
  - On all retries failed: return error

**Verification:**
- Stablecoins return $1.00 without API call
- Native tokens fetch from CoinGecko via HDPay's service
- Retry logic works on failure
- Cache prevents duplicate API calls

## Task 4: Address Validation

**Create** address format validators for all three chains.

**Files to create:**
- `internal/poller/validate/address.go` — ValidateAddress(chain, address, network) error

**What to import from HDPay:**
- `internal/models` — `Chain`, `ChainBTC`, `ChainBSC`, `ChainSOL`
- HDPay's `btcutil` dependency (already in go.mod) for bech32 validation

**Details:**
- `ValidateAddress(chain models.Chain, address string, network string) error`

- **BTC validation**:
  - Mainnet: starts with `bc1` (bech32), `1` (P2PKH), or `3` (P2SH)
  - Testnet: starts with `tb1` (bech32), `m`/`n` (P2PKH), or `2` (P2SH)
  - Use `btcd/btcutil` for bech32 decoding and checksum validation (already a dependency)
  - Reject if checksum invalid

- **BSC validation**:
  - Must match regex `^0x[0-9a-fA-F]{40}$`
  - Same format for mainnet and testnet

- **SOL validation**:
  - Base58 encoded string
  - Must decode to exactly 32 bytes
  - Use `btcd/btcutil/base58` for decoding (already a dependency via btcsuite)

**Verification:**
- Known valid testnet addresses from MEMORY.md pass validation
- Invalid formats fail with descriptive errors
- Chain/network mismatch detected

## Task 5: Core Services Tests

**Create** comprehensive tests for all services.

**Files to create:**
- `internal/poller/points/tiers_test.go` — Tier loading, validation, defaults
- `internal/poller/points/calculator_test.go` — All tier boundaries, edge cases
- `internal/poller/points/pricer_test.go` — Retry, stablecoins, cache
- `internal/poller/validate/address_test.go` — All chains, valid + invalid

**Details:**
- Points calculator: test EVERY tier boundary from Task 2 verification list
- Tiers: test validation rules (gaps, unsorted, missing unbounded, too few tiers)
- Price: use httptest.Server to mock CoinGecko responses for retry testing
- Address validation: use known addresses from MEMORY.md as test vectors:
  - BTC testnet: `tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr` (valid)
  - BSC testnet: `0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb` (valid)
  - SOL devnet: `3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx` (valid)

**Verification:**
- `go test ./internal/poller/points/... ./internal/poller/validate/...` passes
- Coverage > 80% on points package (critical path)
- Coverage > 70% on validate package

</tasks>

<success_criteria>
- Points calculator returns correct values for ALL tier boundaries
- Tiers.json created with defaults if missing, loaded and validated correctly
- Price service reuses HDPay's CoinGecko client, adds Poller retry logic, hardcodes stablecoins
- Address validation works for BTC (bech32), BSC (hex), SOL (base58) on both networks
- All tests pass with > 70% coverage on core packages
- No hardcoded values — all from constants
</success_criteria>

<research_needed>
- Confirm btcutil/base58 package works for SOL address decoding (32-byte ed25519 pubkeys)
</research_needed>
