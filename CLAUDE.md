# HDPay Development Guidelines

## Project Overview

HDPay is a self-hosted cryptocurrency payment tool that derives addresses from a BIP-44 HD wallet (24-word mnemonic), scans balances across multiple blockchains using free-tier APIs, tracks transaction history locally, and enables batch fund consolidation — all controlled via a Svelte dashboard.

**Supported Chains**: BTC, BSC, SOL
**Supported Tokens**: Native (BTC, BNB, SOL) + USDC and USDT on BSC and SOL
**Core Philosophy**: Self-hosted, zero external paid services, localhost-only, single binary deployment.

---

## Tech Stack

### Backend (Go 1.22+)
- **Router**: Chi (`github.com/go-chi/chi/v5`)
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **HD Wallet**: `github.com/tyler-smith/go-bip39` (mnemonic/seed), `github.com/btcsuite/btcd/btcutil/hdkeychain` (BIP-32 key derivation)
- **BTC**: `github.com/btcsuite/btcd` (transaction building, signing, address generation — Native SegWit bech32)
- **BSC/EVM**: `github.com/ethereum/go-ethereum` (ethclient, token transfers, BEP-20 ABI)
- **SOL**: `github.com/gagliardetto/solana-go` (RPC client, transaction building, SPL token support)
- **Solana HD**: `github.com/dmitrymomot/solana` or manual ed25519 derivation from BIP-44 path `m/44'/501'/N'/0'`
- **Logging**: `log/slog` (structured, multi-output: stdout + daily rotated files)
- **Config**: `github.com/kelseyhightower/envconfig`
- **HTTP Client**: Standard `net/http` with rate-limiting middleware for API calls
- **Price Feed**: CoinGecko free API (`/api/v3/simple/price`)

### Frontend (SvelteKit)
- **Framework**: SvelteKit with `adapter-static`
- **Language**: TypeScript (strict mode, zero `any`)
- **Styling**: Tailwind CSS + shadcn-svelte
- **Charts**: Apache ECharts (portfolio visualization)
- **State**: Svelte stores (built-in)
- **Table Virtualization**: `@tanstack/svelte-virtual` (address lists up to 500K rows)

### Deployment
- **Build**: Single Go binary serves embedded static SvelteKit build
- **Database**: SQLite file persisted locally
- **Network**: Localhost-only binding, CORS locked to `127.0.0.1` / `localhost`
- **Security**: CSRF protection, no external inbound access

---

## Blockchain API Providers (Free Tier)

All scanning uses free-tier APIs with round-robin rotation to stay within limits.

### BTC
| Provider | Endpoint | Rate Limit | Batch |
|----------|----------|------------|-------|
| Blockstream Esplora | `blockstream.info/api` | ~10 req/s | 1 address/call |
| Mempool.space | `mempool.space/api` | ~10 req/s | 1 address/call |
| Blockchain.info | `blockchain.info/multiaddr` | ~5 req/s | Multiple addresses/call |

### BSC
| Provider | Endpoint | Rate Limit | Batch |
|----------|----------|------------|-------|
| BscScan API | `api.bscscan.com/api` | 5 req/s | Up to 20 addresses for balance |
| BSC Public RPC | `bsc-dataseed.binance.org` | ~10 req/s | Single via ethclient |
| Ankr Public | `rpc.ankr.com/bsc` | ~30 req/s | Single via ethclient |

### SOL
| Provider | Endpoint | Rate Limit | Batch |
|----------|----------|------------|-------|
| Solana Public RPC | `api.mainnet-beta.solana.com` | ~10 req/s | `getMultipleAccounts` up to 100/call |
| Helius Free | `mainnet.helius-rpc.com` | ~10 req/s | `getMultipleAccounts` up to 100/call |

### Testnet
| Chain | Provider |
|-------|----------|
| BTC Testnet | `blockstream.info/testnet/api`, `mempool.space/testnet/api` |
| BSC Testnet | `data-seed-prebsc-1-s1.binance.org:8545`, BscScan testnet API |
| SOL Devnet | `api.devnet.solana.com` |

---

## Code Conventions

### Go Backend

#### File Organization
```
cmd/
  server/
    main.go                # Entry point, minimal
internal/
  api/
    router.go              # Chi router setup, middleware
    handlers/
      address.go           # Address generation, listing, export
      scan.go              # Balance scanning endpoints
      send.go              # Transaction building, signing, broadcasting
      dashboard.go         # Portfolio overview, stats
      settings.go          # Configuration endpoints
    middleware/
      security.go          # CORS, CSRF, localhost-only
      logging.go           # Request/response logging
  wallet/
    hd.go                  # BIP-44 HD key derivation (master)
    btc.go                 # BTC address generation (Native SegWit bech32)
    bsc.go                 # BSC/EVM address generation (same as ETH, coin type 60)
    sol.go                 # SOL address generation (ed25519, m/44'/501'/N'/0')
  scanner/
    scanner.go             # Scanner orchestrator, resume logic
    provider.go            # Provider interface + round-robin rotation
    btc_provider.go        # Blockstream, Mempool.space, Blockchain.info
    bsc_provider.go        # BscScan, public RPCs
    sol_provider.go        # Solana RPC, Helius
    ratelimiter.go         # Per-provider rate limiting
  tx/
    btc_tx.go              # BTC multi-input UTXO transaction building
    bsc_tx.go              # BSC sequential transfers (native + BEP-20)
    sol_tx.go              # SOL multi-instruction transaction building (native + SPL)
    gas.go                 # BSC gas pre-seeding logic
    broadcaster.go         # Transaction broadcast + confirmation tracking
  db/
    sqlite.go              # SQLite connection, migrations
    addresses.go           # Address CRUD
    balances.go            # Balance state
    transactions.go        # Transaction history
    scans.go               # Scan state (resume support)
  price/
    coingecko.go           # USD price fetching
  config/
    config.go              # Configuration struct (envconfig)
    constants.go           # ALL numeric/string constants
    errors.go              # ALL error types and codes
  models/
    types.go               # Shared domain types
  logging/
    logger.go              # slog setup: stdout + daily file rotation
```

#### Naming Conventions
- **Files**: lowercase with underscores (`btc_tx.go`, `sol_provider.go`)
- **Packages**: short, lowercase, no underscores (`wallet`, `scanner`, `tx`)
- **Exported**: PascalCase (`GenerateAddresses`, `ScanResult`)
- **Unexported**: camelCase (`deriveKey`, `buildTxInput`)
- **Constants**: PascalCase (`MaxAddressesPerChain`, `DefaultScanBatchSize`)
- **Errors**: `Err` prefix (`ErrInvalidMnemonic`, `ErrProviderRateLimit`)

#### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to derive BTC address at index %d: %w", index, err)
}

// All errors defined in config/errors.go — NEVER inline error strings
var (
    ErrInvalidMnemonic    = errors.New("invalid mnemonic")
    ErrProviderRateLimit  = errors.New("provider rate limit exceeded")
    ErrScanInterrupted    = errors.New("scan interrupted")
    ErrInsufficientGas    = errors.New("insufficient gas for transaction")
    ErrTransactionFailed  = errors.New("transaction broadcast failed")
)
```

#### Logging

## EVERY SINGLE ACTION MUST BE LOGGED. WE NEVER HAVE ENOUGH LOGS.

Use structured logging with slog at appropriate levels:
```go
// DEBUG: Internal state, derivation details, API request/response bodies
slog.Debug("deriving address",
    "chain", "BTC",
    "index", index,
    "path", path,
)

// INFO: User-facing actions, milestones
slog.Info("scan started",
    "chain", chain,
    "maxID", maxID,
    "providerCount", len(providers),
)

// WARN: Recoverable issues, fallbacks
slog.Warn("provider rate limited, rotating",
    "provider", provider.Name(),
    "retryAfter", retryAfter,
)

// ERROR: Failures that need attention
slog.Error("transaction broadcast failed",
    "chain", "BSC",
    "txHash", hash,
    "error", err,
)
```

**Log output**: Dual output — stdout (for terminal) + daily rotated files in `./logs/hdpay-YYYY-MM-DD.log`.

#### Context Usage
- Pass `context.Context` as first parameter everywhere
- Use for cancellation (scan interruption), timeouts (API calls), request-scoped values
- Never store context in structs

#### Database Conventions
- Use `modernc.org/sqlite` (pure Go, no CGO dependency)
- All migrations in `internal/db/migrations/` as numbered SQL files
- WAL mode enabled for concurrent reads
- Prepared statements for all queries
- All table/column names in snake_case

---

### Frontend (SvelteKit)

#### File Organization
```
web/
  src/
    lib/
      components/
        ui/                 # shadcn-svelte components
        layout/
          Sidebar.svelte
          Header.svelte
        address/
          AddressTable.svelte
          AddressExport.svelte
        scan/
          ScanControl.svelte
          ScanProgress.svelte
        send/
          SendPanel.svelte
          GasPreSeed.svelte
          TransactionConfirm.svelte
        dashboard/
          PortfolioOverview.svelte
          ChainBreakdown.svelte
          TokenBalances.svelte
        settings/
          SettingsPanel.svelte
      stores/
        addresses.ts        # Address state
        scan.ts             # Scan state + SSE connection
        balances.ts         # Balance aggregation
        transactions.ts     # Transaction history
        settings.ts         # User settings (max ID, network mode)
        prices.ts           # USD price cache
      utils/
        api.ts              # API client (single source of truth)
        formatting.ts       # Number/address/date formatting
        validation.ts       # Input validation
        chains.ts           # Chain metadata helpers
      constants.ts          # ALL frontend constants, error codes
      types.ts              # ALL TypeScript interfaces
    routes/
      +layout.svelte
      +page.svelte          # Dashboard (landing)
      addresses/
        +page.svelte
      scan/
        +page.svelte
      send/
        +page.svelte
      transactions/
        +page.svelte
      settings/
        +page.svelte
    app.css
    app.d.ts
  static/
```

#### TypeScript Rules
- **Strict mode**: enabled in `tsconfig.json`
- **No `any`**: use `unknown` and type guards instead — NO EXCEPTIONS
- **Explicit return types**: on ALL exported functions
- **Interface over type**: for object shapes
- **All types in `types.ts`**: never define types inline in components

```typescript
// Good — in types.ts
export interface AddressBalance {
  chain: Chain;
  index: number;
  address: string;
  nativeBalance: string;
  tokenBalances: TokenBalance[];
  lastScanned: string | null;
}

export interface TokenBalance {
  symbol: TokenSymbol;
  balance: string;
  contractAddress: string;
}

// Good — explicit return type
export function formatBalance(wei: string, decimals: number): string {
  // ...
}

// Bad — NEVER do this
type AddressBalance = any;
export const formatBalance = (wei, decimals) => { ... }
```

#### Component Conventions
```svelte
<script lang="ts">
  // 1. Imports
  import { onMount } from 'svelte';
  import { scanStore } from '$lib/stores/scan';
  import type { ScanStatus } from '$lib/types';

  // 2. Props
  export let chain: Chain;

  // 3. Local state
  let isScanning = false;

  // 4. Reactive statements
  $: progress = $scanStore.progress;

  // 5. Functions
  async function startScan(): Promise<void> {
    // ...
  }

  // 6. Lifecycle
  onMount(() => {
    // ...
  });
</script>

<!-- Template -->
<div class="container">
  <!-- Content -->
</div>

<style>
  /* Scoped styles if needed (prefer Tailwind) */
</style>
```

---

## Constants and Configuration

### NO Hardcoded Values — THIS IS NON-NEGOTIABLE

Every single number, string, error code, URL, timeout, limit, and configuration value goes in dedicated constant files. The constants package is the compass of this project.

**Go** (`internal/config/constants.go`):
```go
package config

import "time"

// Address Generation
const (
    MaxAddressesPerChain = 500_000
    DefaultMaxScanID     = 5_000
)

// BIP-44 Derivation Paths
const (
    BIP44Purpose     = 44
    BTC_CoinType     = 0     // m/44'/0'/0'/0/N
    BSC_CoinType     = 60    // m/44'/60'/0'/0/N (same as ETH)
    SOL_CoinType     = 501   // m/44'/501'/N'/0'
    BTC_TestCoinType = 1     // Testnet
)

// Token Contract Addresses — BSC Mainnet
const (
    BSC_USDC_Contract = "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"
    BSC_USDT_Contract = "0x55d398326f99059fF775485246999027B3197955"
)

// Token Contract Addresses — SOL Mainnet
const (
    SOL_USDC_Mint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
    SOL_USDT_Mint = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
)

// Token Contract Addresses — BSC Testnet
const (
    BSC_Testnet_USDC_Contract = "" // Set when available
    BSC_Testnet_USDT_Contract = "" // Set when available
)

// Token Contract Addresses — SOL Devnet
const (
    SOL_Devnet_USDC_Mint = "" // Set when available
    SOL_Devnet_USDT_Mint = "" // Set when available
)

// Scanning
const (
    ScanBatchSize_BscScan    = 20   // BscScan multi-address balance
    ScanBatchSize_SolanaRPC  = 100  // getMultipleAccounts limit
    ScanBatchSize_Blockchain = 50   // Blockchain.info multiaddr
    ScanResumeThreshold      = 24 * time.Hour // Re-scan if older than this
)

// Rate Limiting (per provider)
const (
    RateLimit_BscScan      = 5  // requests/second
    RateLimit_Blockstream  = 10
    RateLimit_Mempool      = 10
    RateLimit_BlockchainInfo = 5
    RateLimit_SolanaRPC    = 10
    RateLimit_Helius       = 10
    RateLimit_CoinGecko    = 10 // requests/minute
)

// Transaction
const (
    BTC_FeeRate_SatPerVByte = 10   // Conservative default, should be fetched dynamically
    BSC_GasLimit_Transfer   = 21_000
    BSC_GasLimit_BEP20      = 65_000
    BSC_GasPreSeed_Wei      = "5000000000000000" // 0.005 BNB per address
    SOL_MaxInstructions     = 20 // Max instructions per SOL transaction
)

// Server
const (
    ServerPort              = 8080
    ServerReadTimeout       = 30 * time.Second
    ServerWriteTimeout      = 60 * time.Second
    APITimeout              = 30 * time.Second
    SSEKeepAliveInterval    = 15 * time.Second
)

// Logging
const (
    LogDir          = "./logs"
    LogFilePattern  = "hdpay-%s.log" // %s = YYYY-MM-DD
    LogMaxAgeDays   = 30
)

// Database
const (
    DBPath         = "./data/hdpay.sqlite"
    DBTestPath     = "./data/hdpay_test.sqlite"
    DBWALMode      = true
    DBBusyTimeout  = 5000 // milliseconds
)

// Price
const (
    CoinGeckoBaseURL    = "https://api.coingecko.com/api/v3"
    PriceCacheDuration  = 5 * time.Minute
)
```

**Go** (`internal/config/errors.go`):
```go
package config

// Error codes — shared with frontend via API responses
const (
    ERROR_INVALID_MNEMONIC      = "ERROR_INVALID_MNEMONIC"
    ERROR_ADDRESS_GENERATION    = "ERROR_ADDRESS_GENERATION"
    ERROR_DATABASE              = "ERROR_DATABASE"
    ERROR_SCAN_FAILED           = "ERROR_SCAN_FAILED"
    ERROR_SCAN_INTERRUPTED      = "ERROR_SCAN_INTERRUPTED"
    ERROR_PROVIDER_RATE_LIMIT   = "ERROR_PROVIDER_RATE_LIMIT"
    ERROR_PROVIDER_UNAVAILABLE  = "ERROR_PROVIDER_UNAVAILABLE"
    ERROR_INSUFFICIENT_BALANCE  = "ERROR_INSUFFICIENT_BALANCE"
    ERROR_INSUFFICIENT_GAS      = "ERROR_INSUFFICIENT_GAS"
    ERROR_TX_BUILD_FAILED       = "ERROR_TX_BUILD_FAILED"
    ERROR_TX_SIGN_FAILED        = "ERROR_TX_SIGN_FAILED"
    ERROR_TX_BROADCAST_FAILED   = "ERROR_TX_BROADCAST_FAILED"
    ERROR_INVALID_ADDRESS       = "ERROR_INVALID_ADDRESS"
    ERROR_INVALID_CHAIN         = "ERROR_INVALID_CHAIN"
    ERROR_INVALID_TOKEN         = "ERROR_INVALID_TOKEN"
    ERROR_EXPORT_FAILED         = "ERROR_EXPORT_FAILED"
    ERROR_PRICE_FETCH_FAILED    = "ERROR_PRICE_FETCH_FAILED"
    ERROR_INVALID_CONFIG        = "ERROR_INVALID_CONFIG"
)
```

**TypeScript** (`web/src/lib/constants.ts`):
```typescript
// API
export const API_BASE = '/api';
export const SSE_RECONNECT_DELAY_MS = 1000;

// Display
export const MAX_TABLE_ROWS_DISPLAY = 1000;
export const ADDRESS_TRUNCATE_LENGTH = 8;
export const BALANCE_DECIMAL_PLACES = 6;

// Chains
export const SUPPORTED_CHAINS = ['BTC', 'BSC', 'SOL'] as const;
export const CHAIN_NATIVE_SYMBOLS = { BTC: 'BTC', BSC: 'BNB', SOL: 'SOL' } as const;
export const CHAIN_TOKENS = {
  BSC: ['USDC', 'USDT'],
  SOL: ['USDC', 'USDT'],
  BTC: [],
} as const;

// Chart Colors
export const CHART_COLORS = ['#f7931a', '#F0B90B', '#9945FF', '#3b82f6', '#10b981'] as const;

// Error Codes (mirror backend)
export const ERROR_INVALID_MNEMONIC = 'ERROR_INVALID_MNEMONIC';
export const ERROR_SCAN_FAILED = 'ERROR_SCAN_FAILED';
export const ERROR_SCAN_INTERRUPTED = 'ERROR_SCAN_INTERRUPTED';
export const ERROR_TX_BUILD_FAILED = 'ERROR_TX_BUILD_FAILED';
export const ERROR_TX_BROADCAST_FAILED = 'ERROR_TX_BROADCAST_FAILED';
export const ERROR_INSUFFICIENT_BALANCE = 'ERROR_INSUFFICIENT_BALANCE';
export const ERROR_INSUFFICIENT_GAS = 'ERROR_INSUFFICIENT_GAS';
// ... mirror all backend error codes
```

### Environment Variables

```env
# .env.example (commit this)
HDPAY_MNEMONIC_FILE=/path/to/mnemonic.txt    # Path to file containing 24-word mnemonic
HDPAY_DB_PATH=./data/hdpay.sqlite
HDPAY_PORT=8080
HDPAY_LOG_LEVEL=info                          # debug, info, warn, error
HDPAY_LOG_DIR=./logs
HDPAY_NETWORK=mainnet                         # mainnet, testnet

# API Keys (free tier)
HDPAY_BSCSCAN_API_KEY=
HDPAY_HELIUS_API_KEY=

# Optional overrides
HDPAY_BTC_FEE_RATE=10
HDPAY_BSC_GAS_PRESEED_WEI=5000000000000000
```

Load with envconfig:
```go
type Config struct {
    MnemonicFile   string `envconfig:"HDPAY_MNEMONIC_FILE" required:"true"`
    DBPath         string `envconfig:"HDPAY_DB_PATH" default:"./data/hdpay.sqlite"`
    Port           int    `envconfig:"HDPAY_PORT" default:"8080"`
    LogLevel       string `envconfig:"HDPAY_LOG_LEVEL" default:"info"`
    LogDir         string `envconfig:"HDPAY_LOG_DIR" default:"./logs"`
    Network        string `envconfig:"HDPAY_NETWORK" default:"mainnet"`
    BscScanAPIKey  string `envconfig:"HDPAY_BSCSCAN_API_KEY"`
    HeliusAPIKey   string `envconfig:"HDPAY_HELIUS_API_KEY"`
}
```

---

## Testing Requirements

### Backend Testing

Every feature MUST have tests. No exceptions.

```
internal/
  wallet/
    hd.go
    hd_test.go
    btc.go
    btc_test.go
  scanner/
    scanner.go
    scanner_test.go
  tx/
    btc_tx.go
    btc_tx_test.go
```

#### Test Patterns
```go
func TestDeriveBTCAddress(t *testing.T) {
    tests := []struct {
        name     string
        mnemonic string
        index    uint32
        network  string
        want     string
        wantErr  bool
    }{
        {
            name:     "index 0 mainnet",
            mnemonic: "abandon abandon ... about",
            index:    0,
            network:  "mainnet",
            want:     "bc1q...",
        },
        // More cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := DeriveBTCAddress(tt.mnemonic, tt.index, tt.network)
            if (err != nil) != tt.wantErr {
                t.Errorf("DeriveBTCAddress() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("DeriveBTCAddress() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### Coverage Target
- Minimum 70% for core packages (`wallet`, `scanner`, `tx`, `db`)
- `go test -cover ./...`

### Frontend Testing

Use Vitest + Testing Library:
```typescript
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ScanControl from './ScanControl.svelte';

describe('ScanControl', () => {
  it('starts scan with correct parameters', async () => {
    const { getByRole } = render(ScanControl, { props: { chain: 'BTC' } });
    // ...
  });
});
```

---

## API Design

### RESTful Endpoints

```
# Address Management
POST   /api/addresses/generate          # Generate addresses (one-time init)
GET    /api/addresses/:chain            # List addresses for chain (paginated)
GET    /api/addresses/:chain/export     # Export as JSON

# Scanning
POST   /api/scan/start                  # Start scan { chain, maxID }
POST   /api/scan/stop                   # Stop current scan
GET    /api/scan/status                 # Current scan status
GET    /api/scan/sse                    # SSE stream for scan progress

# Balances
GET    /api/balances/summary            # Portfolio summary (all chains)
GET    /api/balances/:chain             # Balances for chain (with filters: hasBalance, token)
GET    /api/balances/:chain/:token      # Token-specific balances

# Transactions — Sending
POST   /api/send/preview                # Preview send (fees, addresses involved)
POST   /api/send/execute                # Execute send (sign + broadcast)
POST   /api/send/gas-preseed            # Pre-seed gas to BSC addresses

# Transaction History
GET    /api/transactions                # All transaction history
GET    /api/transactions/:chain         # Chain-specific history

# Dashboard
GET    /api/dashboard/portfolio         # Portfolio with USD values
GET    /api/dashboard/prices            # Current prices

# Settings
GET    /api/settings                    # Current settings
PUT    /api/settings                    # Update settings
GET    /api/health                      # Health check
```

### Response Format

```json
{
  "data": { ... },
  "meta": {
    "page": 1,
    "pageSize": 100,
    "total": 5000,
    "executionTime": 42
  }
}

// Error
{
  "error": {
    "code": "ERROR_SCAN_FAILED",
    "message": "Provider rate limit exceeded, retrying..."
  }
}
```

### SSE Events (Server-Sent Events)

```
event: scan_progress
data: {"chain":"BTC","scanned":1500,"total":5000,"found":3,"elapsed":"2m30s"}

event: scan_complete
data: {"chain":"BTC","scanned":5000,"found":12,"duration":"8m15s"}

event: scan_error
data: {"chain":"BTC","error":"ERROR_PROVIDER_UNAVAILABLE","message":"All providers down"}

event: tx_status
data: {"txHash":"0x...","status":"confirmed","chain":"BSC","confirmations":3}
```

---

## Key Architecture Decisions

### Address Generation
- One-time CLI command: `./hdpay init --mnemonic-file /path/to/mnemonic.txt`
- Generates 500K addresses per chain (1.5M total), stores in SQLite
- Mnemonic is read, used, then the file reference is stored (not the mnemonic itself)
- Private keys are NEVER stored — derived on-demand only for transaction signing
- Mnemonic file path stored in env var, read only when signing transactions

### Scanning Strategy
- Round-robin across multiple providers per chain
- Per-provider rate limiters (token bucket)
- Batch where possible: Blockchain.info multi-addr, Solana getMultipleAccounts(100), BscScan multi-balance(20)
- Resumable: store `last_scanned_index` per chain in DB
- Resume threshold: if last scan < 24h ago, resume from last index; otherwise restart

### Transaction Consolidation
- **BTC**: Single UTXO transaction with multiple inputs (one per funded address), one output to destination
- **SOL**: Single transaction with multiple transfer instructions (up to ~20 per tx due to size limits), multiple txs if more
- **BSC**: Sequential automated transactions (one per address), with gas pre-seeding step first for token transfers
- All signing happens on-demand: derive private key → sign → discard key

### Security
- Localhost-only binding (`127.0.0.1`)
- CORS: only `http://localhost:*` and `http://127.0.0.1:*`
- CSRF token on all mutating endpoints
- Mnemonic never in memory longer than needed for signing
- Private keys never stored, never logged
- SQL injection prevention via prepared statements
- No external inbound network access

---

## Commit Strategy

### Granular Commits

Commit after every meaningful change:
- Added new endpoint
- Fixed bug
- Added test
- Refactored function

**NOT**: "implemented scanning" (too big)

### Commit Message Format

```
<type>: <short description>

[optional body]
```

Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

Examples:
```
feat: add BTC address generation via BIP-44

fix: handle Blockstream API 429 with exponential backoff

test: add SOL derivation path test vectors

refactor: extract provider rotation into shared middleware
```

---

## Session Changelog

After EVERY coding session, update `CHANGELOG.md`:

```markdown
# Changelog

## [Unreleased]

### 2026-02-18

#### Added
- BTC address generation with Native SegWit (bech32)
- SQLite schema for addresses table

#### Changed
- Increased scan batch size for Solana to 100

#### Fixed
- Rate limiter not resetting between providers
```

---

## Component Reusability

### No Duplicated Logic

Before creating any new component or utility:
1. `grep -r "functionName" web/src/lib/`
2. If exists → import from existing location
3. If new but reusable → create in `web/src/lib/utils/` immediately

### Shared Utilities
- Address formatting → `utils/formatting.ts`
- API calls → `utils/api.ts` (SINGLE source of truth for all backend calls)
- Validation → `utils/validation.ts`
- Chain helpers → `utils/chains.ts`

---

## AI Agent Instructions

When working on this codebase:

### ⚠️ CRITICAL: Always Changelog + Commit After Every Task

**YOU MUST UPDATE CHANGELOG.md AND COMMIT BEFORE ENDING ANY SESSION OR TASK. NO EXCEPTIONS.**

1. Update `CHANGELOG.md` with what changed
2. Commit all modified files with a proper commit message
3. Never leave uncommitted work
4. Do this after EVERY task, not just at the end of a session

### ⚠️ Before Adding Any Function

**ALWAYS check before creating helpers:**

1. `grep -r "functionName" web/src/lib/` (frontend)
2. `grep -r "functionName" internal/` (backend)
3. If exists → import from existing location
4. If new but reusable → put in proper utils package immediately

**Never duplicate utility functions across components or packages.**

### ⚠️ Constants Are Sacred

- NEVER hardcode a number, string, URL, timeout, error message, or limit
- ALL values go in `internal/config/constants.go` or `web/src/lib/constants.ts`
- Error codes go in `internal/config/errors.go`
- If you're typing a literal value that isn't a variable name, it probably needs to be a constant

### Workflow Checklist

1. **Read before writing**: Always read existing code before modifying
2. **Follow patterns**: Match existing code style exactly
3. **Test everything**: Every feature needs tests
4. **Commit frequently**: After each meaningful change
5. **Update changelog**: Before committing, update CHANGELOG.md
6. **No hardcoding**: Use constants files — EVERY TIME
7. **Check reusability**: Don't duplicate components or utilities
8. **Type everything**: No `any`, explicit return types (TypeScript)
9. **Handle errors**: Wrap with context, log at appropriate level
10. **Security first**: Never log private keys or mnemonics, localhost-only
11. **Log everything**: Every action, state change, API call, and error must be logged
