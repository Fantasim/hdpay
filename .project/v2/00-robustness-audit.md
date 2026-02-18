# V1 Robustness Audit — Consolidated Findings

> Input for V2 planning. Three audits performed: TX engines, scanner/providers, API/infrastructure.

## A. Transaction Safety (Money-Loss Prevention)

| ID | Issue | Severity | Where | Detail |
|----|-------|----------|-------|--------|
| A1 | No concurrent send mutex | CRITICAL | send.go:452 | Two tabs can hit `/send/execute` simultaneously → nonce collisions (BSC), double-broadcasts (BTC/SOL) |
| A2 | BTC: No post-broadcast confirmation | CRITICAL | btc_tx.go | Broadcast returns txHash but never verifies it landed in mempool. BSC and SOL both poll — BTC doesn't |
| A3 | SOL: Confirmation ignores RPC errors | CRITICAL | sol_tx.go:283-328 | RPC down during WaitForSOLConfirmation → logs error, continues polling → TX confirmed on-chain but user sees "failed" → may re-send |
| A4 | No in-flight TX persistence | CRITICAL | all TX engines | Server crash mid-sweep = no record of which addresses sent. User retries = double-send |
| A5 | SOL blockhash expires mid-batch | CRITICAL | sol_tx.go:522 | Blockhash valid ~30s; sequential sweeps exceed this → silent TX rejection |
| A6 | UTXO race between preview/execute | HIGH | btc_tx.go:276-425 | BTC UTXOs can be spent externally between preview and execute — no locking/versioning |
| A7 | BSC balance mismatch preview→execute | HIGH | bsc_tx.go:180-318 | Preview uses DB balance (stale), execute uses real-time — can diverge significantly |
| A8 | Partial sweep = no resume | HIGH | bsc_tx.go, sol_tx.go | BSC/SOL sweep fails at address #5 of 20 → addresses #1-4 swept, no way to resume from #5 |
| A9 | Gas pre-seed timeout = possible double-spend | HIGH | gas.go:203-270 | Receipt poll timeout marks TX "failed" but TX may land on-chain; user retries = duplicate gas sends |
| A10 | SOL ATA creation race | HIGH | sol_tx.go:724-763 | First TX creates dest ATA, second TX assumes it exists — RPC lag means it might not see it yet |
| A11 | BSC gas price spike preview→execute | MEDIUM | bsc_tx.go | Gas estimated at preview, not re-estimated at execute. Spike = underpay (stuck) or overpay |
| A12 | BSC nonce gap on gas pre-seed timeout | MEDIUM | gas.go:151-188 | Sequential nonce increments; if a TX broadcast times out but lands, subsequent nonces conflict |
| A13 | No error classification (transient vs permanent) | MEDIUM | all TX engines | All errors treated the same — no smart retry vs fast-fail |

## B. Scanner Resilience

| ID | Issue | Severity | Where | Detail |
|----|-------|----------|-------|--------|
| B1 | Providers return early on first error | CRITICAL | btc_blockstream.go:64, btc_mempool.go:54, bsc_bscscan.go:159 | Address #5 of 20 fails → #6-20 never queried. All providers have this bug |
| B2 | Silent "0" on decode/RPC errors | CRITICAL | btc_blockstream.go:126, bsc_rpc.go:159, sol_rpc.go:274 | Malformed API responses return "0" balance — indistinguishable from true zero |
| B3 | Partial RPC results undetected (SOL) | CRITICAL | sol_rpc.go:142-177 | `getMultipleAccounts` returns 50 of 100 → code silently accepts, never fetches remaining 50 |
| B4 | Scan state write failure = address gap | CRITICAL | scanner.go:322-332 | If `UpsertScanState` fails mid-scan → crash → addresses skipped permanently on resume |
| B5 | No circuit breaker | HIGH | pool.go:58-86 | Failed provider keeps getting hit every batch — no cooldown period |
| B6 | No Retry-After header parsing | HIGH | all providers | 429 responses always retry at base rate — ignores server's backoff hint |
| B7 | Token scan failure is silent | HIGH | scanner.go:283-317 | USDC fetch fails for 5000 addresses → one WARN line → user sees "0 USDC" |
| B8 | Native balance failure stops all tokens | HIGH | scanner.go:234-255 | Native provider fails → entire batch aborted → token scans never run |
| B9 | Pool returns only last error | MEDIUM | pool.go:85 | When all providers fail, only last error returned — can't diagnose multi-provider outage |
| B10 | SSE dropped events = stale frontend | MEDIUM | sse.go:99-119 | Slow clients miss progress updates, no recovery/resync mechanism |
| B11 | No exponential backoff in pool retry | MEDIUM | pool.go:62-86 | Pool retries immediately with round-robin — hammers recovering provider |

## C. Security & Testing

| ID | Issue | Severity | Where | Detail |
|----|-------|----------|-------|--------|
| C1 | Security middleware 0% tested | HIGH | middleware/security.go | HostCheck, CORS, CSRF — entire security boundary has 0 tests |
| C2 | Scanner providers partially untested | MEDIUM | btc_mempool.go, bsc_rpc.go, sol_rpc.go | Mempool, BSC RPC, SOL RPC providers have 0 tests each |
| C3 | TX SSE hub untested | MEDIUM | tx/sse.go | Concurrent subscribe/broadcast/unsubscribe untested |

## D. Infrastructure

| ID | Issue | Severity | Where | Detail |
|----|-------|----------|-------|--------|
| D1 | No HTTP idle timeout | HIGH | main.go:141-146 | SSE connections can exhaust file descriptors. Missing `IdleTimeout` and `MaxHeaderBytes` |
| D2 | BSC/SOL broadcast = single point of failure | HIGH | bsc_tx.go, sol_tx.go | Single RPC endpoint for TX broadcast — if down, cannot send |
| D3 | No DB connection pool limits | MEDIUM | sqlite.go:33 | Unlimited connections under concurrent load |
| D4 | Price service: no stale-but-serve | MEDIUM | coingecko.go:81 | CoinGecko down = dashboard shows nothing, no fallback to stale cached prices |
| D5 | No graceful shutdown for in-flight TXs | MEDIUM | main.go:160-168 | Shutdown timeout 10s but sends take up to 10 minutes |
| D6 | Unused retry constants | LOW | constants.go | `ProviderMaxRetries` and `ProviderRetryBaseDelay` defined but never used |
| D7 | Error messages leak internals | LOW | various handlers | Raw `err.Error()` strings go to frontend |
| D8 | CSRF token gen can return empty string | LOW | security.go:116 | `rand.Read` failure = all CSRF validation breaks |
| D9 | No IPv6 loopback | LOW | security.go:20 | `[::1]` blocked by host check |
| D10 | No request body size limit | LOW | router.go | Theoretical OOM (localhost-only, low risk) |

## Totals

| Severity | Count |
|----------|-------|
| CRITICAL | 8 |
| HIGH | 13 |
| MEDIUM | 12 |
| LOW | 4 |
| **Total** | **37** |
