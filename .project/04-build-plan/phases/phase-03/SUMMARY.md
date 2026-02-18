# Phase 3 Summary: Address Explorer

## Completed: 2026-02-18

## What Was Built
- Paginated address listing API with chain/balance/token filter support
- Streaming JSON export endpoint for address downloads
- Address explorer frontend page matching mockup: chain tabs, filter chips, paginated table
- Copy-to-clipboard on address cells with visual feedback
- Chain badge components with chain-specific colors
- Formatting utilities: relative time, clipboard, chain helpers

## Files Created/Modified
- `internal/models/types.go` — Added `AddressWithBalance`, `TokenBalanceItem` types
- `internal/config/constants.go` — Added `DefaultPage`, `DefaultPageSize`, `MaxPageSize` pagination constants
- `internal/db/addresses.go` — Added `AddressFilter`, `GetAddressesWithBalances`, `hydrateBalances` methods
- `internal/api/handlers/address.go` — `ListAddresses` and `ExportAddresses` handlers with validation/logging
- `internal/api/router.go` — Wired `/api/addresses/{chain}` and `/api/addresses/{chain}/export` routes
- `internal/db/addresses_test.go` — 5 tests: pagination, hasBalance, token filter, hydration, empty
- `internal/api/handlers/address_test.go` — 6 tests: pagination, invalid chain, defaults, case insensitive, export
- `web/src/lib/types.ts` — Renamed `AddressBalance` → `AddressWithBalance`, fixed `index` → `addressIndex`
- `web/src/lib/utils/api.ts` — Added `getAddresses`, `exportAddresses` functions
- `web/src/lib/utils/formatting.ts` — Added `formatRelativeTime`, `copyToClipboard`
- `web/src/lib/utils/chains.ts` — New: `getChainColor`, `getChainLabel`, `getTokenDecimals`, `getExplorerUrl`
- `web/src/lib/stores/addresses.ts` — New: reactive address store with chain/page/filter state
- `web/src/lib/components/address/AddressTable.svelte` — New: table with badges, copy, token rows
- `web/src/routes/addresses/+page.svelte` — Full address explorer page

## Decisions Made
- **Server-side pagination over virtual scrolling**: With 100 rows/page, the table never needs virtual scrolling. `@tanstack/svelte-virtual` is installed for future use if needed.
- **Chain-scoped API**: API requires a chain parameter (no "all chains" endpoint) — keeps queries simple against 1.5M addresses.
- **Streaming export**: Export uses `StreamAddresses` callback pattern to avoid loading 500K addresses into memory.

## Deviations from Plan
- Virtual scrolling not integrated into AddressTable — server-side pagination makes it unnecessary at 100 rows/page.
- "All Chains" tab defaults to BTC since the API requires a chain parameter (one chain at a time).

## Issues Encountered
- `last_scanned` column is nullable — needed `sql.NullString` in balance hydration scan.

## Notes for Next Phase
- Address API is at `/api/addresses/{chain}` with pagination, `hasBalance`, and `token` query params.
- Balance hydration uses a batch query with `(chain, address_index) IN (...)` for efficiency.
- `writeJSON` and `writeError` helper functions are available in the handlers package for reuse.
- `isValidChain` and `parseIntParam` are also reusable helpers in handlers package.
