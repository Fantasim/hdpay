# Session 023 -- 2026-02-19

## Version: V2 (post-build)
## Phase: Network Mainnet/Testnet Coexistence
## Summary: Added network column to all core tables so mainnet and testnet data coexist in the same database. Changed default network to testnet. Made network env-only (not editable from UI). All queries automatically filter by active network.

## What Was Done

### Network Column Migration
- Migration 007: adds `network` column to addresses, balances, scan_state, transactions, tx_state tables
- Auto-detects network for existing data using BTC address prefix (`bc1` = mainnet, `tb1` = testnet)
- `db.New(path)` signature changed to `db.New(path, network)` — all callers updated
- Model structs (Address, Balance, ScanState, Transaction, AddressWithBalance) now include `Network` field
- All queries automatically filter by active network via `DB.network` field

### Network Settings
- Default network changed from `mainnet` to `testnet`
- Network is env-only (`HDPAY_NETWORK`), not editable from Settings UI
- Settings page shows current network as read-only badge with env var hint
- Removed interactive network toggle (radio buttons) from Settings page

### Tests
- `TestNetworkIsolation`: two DB instances (testnet + mainnet) on same file see only their own data
- `TestResetBalances_NetworkScoped` and `TestResetAll_NetworkScoped`: reset operations only affect active network

## Decisions Made
- **Network as env-only**: Prevents accidental network switches that could cause data confusion or fund loss
- **Default testnet**: Safer default — users must explicitly opt into mainnet
- **Same database, network column**: Simpler than separate DB files, supports easy data inspection
- **Auto-detect on migration**: Existing data gets correct network label without user intervention

## Issues / Blockers
- None

## Next Steps
- Project is in solid post-V2 state with comprehensive hardening
- Consider V3 planning for new features
