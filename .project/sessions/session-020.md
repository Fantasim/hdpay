# Session 020 -- 2026-02-19

## Version: V2 (post-build)
## Phase: Post-V2 Comprehensive Audit Fixes
## Summary: Second audit pass fixing 14 issues across security, correctness, and infrastructure. Addressed CSRF bypass vulnerability, BSC private key zeroing, balance precision loss, log rotation, and config validation.

## What Was Done

### Security
- Fixed CSRF empty token bypass: `generateCSRFToken()` now returns error on `crypto/rand.Read` failure instead of empty string
- Added BSC private key zeroing: `ZeroECDSAKey()` helper with `defer` at all 3 BSC signing callsites

### Bug Fixes
- Fixed hard-coded testnet explorer links: now reads network from `GET /api/settings`
- Fixed `formatRawBalance` precision loss: rewrote to use string-based decimal placement instead of `parseFloat()` for values > 2^53
- Fixed `formatDate` invalid input: added `isNaN(date.getTime())` guard
- Fixed ScanProgress ETA div-zero: guarded `elapsedMs <= 0` case
- Fixed copy timeout cleanup: stored timeout ID and clear on new copy

### Infrastructure
- Added log rotation: `Setup()` returns `io.Closer`, `CleanOldLogs()` deletes files older than `LogMaxAgeDays`
- Added config validation: `Validate()` checks Network is "mainnet"|"testnet" and Port is 1-65535
- Added BTC fee estimation TTL cache (2-minute expiry) to avoid hammering mempool.space
- Removed misleading "All Chains" tab from addresses page (silently defaulted to BTC)
- Removed duplicate `TOKEN_DECIMALS` and unused `getTokenDecimals()` from `chains.ts`
- Settings API now includes `network` field for frontend to read server config
- Added `ErrInvalidConfig` error and `FeeCacheTTL` constant

## Decisions Made
- **String-based balance formatting**: Avoids JavaScript floating-point precision issues for large crypto values
- **Log rotation on startup**: Simpler than background goroutine, sufficient for daily-restart pattern
- **Config validation at boot**: Fail fast on invalid configuration rather than at runtime

## Issues / Blockers
- None

## Next Steps
- Fix send flow blocking issues
