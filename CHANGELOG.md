# Changelog

## [Unreleased]

### 2026-02-18 (Phase 3)

#### Added
- Address listing API: `GET /api/addresses/{chain}` with pagination, hasBalance/token filters
- Address export API: `GET /api/addresses/{chain}/export` with streaming JSON download
- DB method `GetAddressesWithBalances` — paginated address+balance query with filter support
- DB method `hydrateBalances` — batch loads balance data for a page of addresses
- `AddressWithBalance` and `TokenBalanceItem` Go response types (`internal/models/types.go`)
- Pagination constants: DefaultPage, DefaultPageSize, MaxPageSize (`internal/config/constants.go`)
- Handler tests: 6 tests covering pagination, invalid chain, export, case insensitivity
- DB tests: 5 tests covering pagination, hasBalance filter, token filter, balance hydration, empty chain
- Frontend address explorer page with chain tabs, filter chips, paginated table
- AddressTable component: chain badges, truncated addresses, copy-to-clipboard, token balance rows
- Address store with reactive state: chain/page/filter switching triggers API refetch
- `getAddresses` and `exportAddresses` API client functions (`lib/utils/api.ts`)
- `formatRelativeTime` and `copyToClipboard` utilities (`lib/utils/formatting.ts`)
- Chain utilities: `getChainColor`, `getChainLabel`, `getTokenDecimals`, `getExplorerUrl` (`lib/utils/chains.ts`)

#### Changed
- Renamed `AddressBalance` to `AddressWithBalance` in frontend types (field `index` → `addressIndex` to match backend)

### 2026-02-18 (Phase 2)

#### Added
- BIP-39 mnemonic validation, seed derivation, file reading (`internal/wallet/hd.go`)
- BTC Native SegWit (bech32) address derivation via BIP-84: `m/84'/0'/0'/0/N` (`internal/wallet/btc.go`)
- BSC/EVM address derivation via BIP-44: `m/44'/60'/0'/0/N` with EIP-55 checksum (`internal/wallet/bsc.go`)
- SOL address derivation via SLIP-10 ed25519: `m/44'/501'/N'/0'` — manual implementation, zero extra deps (`internal/wallet/sol.go`)
- Address generator for bulk generation with progress callbacks (`internal/wallet/generator.go`)
- JSON export with streaming (no OOM on 500K addresses) (`internal/wallet/export.go`)
- `init` CLI command: generates 500K addresses per chain, batch inserts (10K/tx), idempotent
- `export` CLI command: exports addresses to `./data/export/*.json`
- DB address methods: InsertAddressBatch, CountAddresses, GetAddresses, StreamAddresses (`internal/db/addresses.go`)
- Domain types: NetworkMode, AllChains, AddressExport, AddressExportItem (`internal/models/types.go`)
- BIP84Purpose constant for Native SegWit derivation (`internal/config/constants.go`)
- 37 unit tests for wallet package with 83.3% coverage
- SLIP-10 spec test vectors verified (master key + child derivation)
- Known test vectors: BTC `bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu`, BSC `0x9858EfFD232B4033E47d90003D41EC34EcaEda94`, SOL `HAgk14JpMQLgt6rVgv7cBQFJWFto5Dqxi472uT3DKpqk`

#### Changed
- BTC derivation uses BIP-84 (purpose=84) instead of BIP-44 (purpose=44) for correct bech32 standard
- `cmd/server/main.go` now supports `init` and `export` subcommands

### 2026-02-18 (Phase 1)

#### Added
- Go module with full directory structure (cmd/, internal/)
- Central constants (`internal/config/constants.go`) — all numeric/string values
- Central error codes (`internal/config/errors.go`) — all error types
- Config loading via envconfig (`internal/config/config.go`)
- Structured logging with dual output: stdout + daily rotated file (`internal/logging/logger.go`)
- SQLite database with WAL mode, migration system, initial schema with 5 tables (`internal/db/`)
- Chi router with middleware stack: request logging, host check, CORS, CSRF (`internal/api/`)
- Health endpoint (`GET /api/health`)
- Server entry point with graceful shutdown (`cmd/server/main.go`)
- SvelteKit frontend with adapter-static, TypeScript strict mode, Tailwind CSS v4
- Design tokens from mockup phase ported to `app.css`
- Sidebar layout component matching mockup (240px, icons, active states, network badge)
- All route placeholders (Dashboard, Addresses, Scan, Send, Transactions, Settings)
- Frontend TypeScript types (`lib/types.ts`) and constants (`lib/constants.ts`)
- API client with CSRF token handling (`lib/utils/api.ts`)
- Formatting utilities (`lib/utils/formatting.ts`)
- Header component with title and optional actions slot
- Makefile with dev, build, test, lint targets
- `.env.example` with documented environment variables
- `.gitignore` for Go binary, data, logs, node_modules
- Unit tests for logging and database modules
