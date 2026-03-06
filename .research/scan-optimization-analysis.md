# Scanner Optimization Analysis — BSC & BTC

> Date: 2026-03-03
> Status: Research complete, ready for implementation planning

## Executive Summary

SOL scans 5,000 addresses in ~15-20 seconds. BSC takes ~25-30 minutes. BTC takes ~17-21 minutes.
The root cause is simple: SOL batches 100 addresses per API call; BSC and BTC fetch 1 address per call.

Four optimization strategies were researched against actual documentation and live APIs.
Combined, they can bring BSC and BTC scan times from 20-30 minutes down to **10-30 seconds**.

---

## Current Performance (5,000 addresses)

| Chain | Addrs/API call | Total API calls | Effective addr/sec | Scan time |
|-------|---------------|-----------------|-------------------|-----------|
| SOL   | 100           | 150 (native+tokens) | ~500-1000    | ~15-20s   |
| BSC   | 1             | 15,000 (native+tokens) | ~8-10     | ~25-30min |
| BTC   | 1             | 5,000 (native only) | ~4-5         | ~17-21min |

### Root Causes

1. **No batching** — BTC and BSC providers all use `MaxBatchSize() = 1`
2. **Sequential failover** — `pool.go` tries one provider at a time; others sit idle
3. **Sequential within provider** — For-loop processes addresses one-by-one, no pipelining
4. **Burst=1 rate limiter** — 10 rps provider enforces minimum 100ms gap between every call
5. **BSC tokens unbatched** — USDC + USDT add 10,000 more sequential `eth_call` requests

---

## Strategy 1: Multicall3 for BSC

### Feasibility: HIGH

### What It Is

Multicall3 is a contract deployed at `0xcA11bde05977b3631167028862bE2a173976CA11` on BSC mainnet
and testnet. A single `eth_call` to its `aggregate3` function can read **hundreds of balances**
(native BNB + BEP-20 tokens) in one HTTP request. It counts as **1 rate-limit token**.

### Evidence

- **Deployed and verified** on BSC mainnet (chainId 56) and testnet (chainId 97) — confirmed
  from mds1/multicall3 `deployments.json`
- **3.2M+ transactions** on BSC mainnet — this is one of the highest-traffic contracts on BSC
- **`getEthBalance(address)`** reads native BNB via EVM `BALANCE` opcode — works on all EVM chains
- **BSC default `RPCGasCap`**: 50,000,000 gas (50M) — confirmed from BSC node source code

### Gas Math

For 100 addresses querying BNB + USDC + USDT = 300 subcalls:

| Subcall type | Count | Gas/call | Subtotal |
|---|---|---|---|
| `getEthBalance` (cold BALANCE opcode) | 100 | ~2,600 | ~260,000 |
| BEP-20 `balanceOf` USDC (cold CALL) | 100 | ~3,500 | ~350,000 |
| BEP-20 `balanceOf` USDT (warm after USDC) | 100 | ~2,000 | ~200,000 |
| Multicall3 loop/memory overhead | — | — | ~100,000 |
| **Total** | **300** | | **~910,000 gas** |

That's ~1M gas — 1/50th of the 50M gas cap. **Safe batch size: 200 addresses per call**
(600 subcalls, ~2M gas). Could push to 500 addresses on reliable nodes.

### How to Call from Go

Two approaches:
- **`abigen`-generated bindings** (recommended): Generate typed Go code from Multicall3 ABI.
  `caller.Aggregate3(&bind.CallOpts{Context: ctx}, calls)` returns `[]Result{Success, ReturnData}`.
- **Raw `eth_call`** via `ethclient.CallContract`: Manual ABI encode/decode with `go-ethereum/accounts/abi`.

### Key Function Signatures

```solidity
function aggregate3(Call3[] calldata calls) returns (Result[] memory)
struct Call3 { address target; bool allowFailure; bytes callData; }
struct Result { bool success; bytes returnData; }
function getEthBalance(address addr) view returns (uint256 balance)
```

### Gotchas

- Use `allowFailure: true` on every subcall — if one address has no token account, the whole
  batch doesn't revert
- Check `result.Success` before parsing `returnData` — failed subcalls return revert data, not zero
- Some public nodes (Ankr free tier) may enforce lower gas caps during high load — test empirically
- `ethclient.Client` has no native batch method; use `rpc.Client` or `abigen` bindings

### Impact

| Metric | Before | After |
|--------|--------|-------|
| API calls for 5K (native+tokens) | 15,000 | **25** (200 addrs/call) |
| Time for 5K addresses | ~25-30 min | **~5-10 sec** |

---

## Strategy 2: BTC Batch Balance APIs

### Blockchain.info `/balance` — Feasibility: LOW for batching

**What the docs say**: `blockchain.info/balance?active=addr1|addr2|...` supports multiple addresses.
The CLAUDE.md claims 50 addresses/call.

**What actually happens** (tested live):
- Batches of 3+ addresses return HTTP 400 intermittently
- **bech32 (bc1q) addresses break `/multiaddr` entirely** — returns 400 even for valid addresses
- No testnet support (HTTP 404)
- No documented rate limits

**Verdict**: Not viable as a batch scanning workhorse for HDPay's bech32-only addresses.

### Blockchair `/addresses/balances` — Feasibility: HIGH (requires free API key)

**Major finding**: Blockchair has an endpoint not currently in our provider list:

```
GET https://api.blockchair.com/bitcoin/addresses/balances?addresses=addr1,addr2,...,addrN
```

- **Up to 25,000 addresses per single request**
- Under **1 second** response time for 25,000 addresses
- Returns confirmed balance only (no tx history overhead)
- Requires **free API key** (non-commercial use)
- **No testnet equivalent**

This would scan 500K BTC addresses in **20 requests** (~20 seconds total).

**Trade-off**: Requires registering for a free API key at blockchair.com. This adds an external
dependency but costs nothing. The key is for rate-limit identification, not payment.

### Blockstream + Mempool (single-address, existing)

Already implemented. No batch endpoint exists for either. Confirmed working with bech32 and
testnet. These remain the no-key-required fallback.

### Impact (with Blockchair)

| Metric | Before | After |
|--------|--------|-------|
| API calls for 5K addresses | 5,000 | **5** (1000 addrs/call, conservative) |
| Time for 5K addresses | ~17-21 min | **~5-10 sec** |

### Impact (without Blockchair, parallel-only)

| Metric | Before | After |
|--------|--------|-------|
| API calls for 5K addresses | 5,000 | 5,000 (same) |
| Effective addr/sec | ~4-5 | ~12-15 (3x from parallel) |
| Time for 5K addresses | ~17-21 min | **~5-7 min** |

---

## Strategy 3: JSON-RPC Batch Requests (BSC fallback)

### Feasibility: HIGH (but Multicall3 is superior)

### What It Is

JSON-RPC 2.0 natively supports sending an array of requests in one HTTP POST.
go-ethereum has `rpc.Client.BatchCallContext(ctx, []BatchElem)` built in.

### Evidence

- **BSC node defaults**: `BatchRequestLimit: 1000`, `BatchResponseMaxSize: 25MB`
  (confirmed from bnb-chain/bsc `node/defaults.go`)
- **go-ethereum native support**: `rpc.BatchElem` struct + `BatchCallContext` method
- All BSC public RPC endpoints run geth forks that inherit batch support

### Why Multicall3 Is Better

| | JSON-RPC Batch | Multicall3 |
|---|---|---|
| Rate limit cost | N tokens (1 per sub-call, most providers) | **1 token** (single eth_call) |
| Native + tokens in 1 call | No (separate methods) | **Yes** (heterogeneous subcalls) |
| Atomic block snapshot | No (may span blocks) | **Yes** (single eth_call = single block) |
| Go support | `rpc.Client.BatchCallContext` | `abigen` bindings or raw `eth_call` |

### Role in HDPay

Use JSON-RPC batch as **fallback** when Multicall3 fails or provider doesn't support complex eth_call.
Batch 20-50 `eth_getBalance` calls per HTTP request via `rpc.BatchCallContext`.

### Gotchas

- Free-tier providers often count each item in a batch as a separate request for rate limiting
- Some providers silently truncate batches — check all `BatchElem.Error` fields
- `ethclient.Client` does NOT expose batching — must use underlying `rpc.Client` directly

---

## Strategy 4: Parallel Provider Fan-Out

### Feasibility: HIGH

### What It Is

Currently `pool.go` tries providers sequentially (round-robin failover). Instead, split a
batch of addresses across all healthy providers and fetch concurrently.

### Go Pattern

```go
g, _ := errgroup.WithContext(ctx)
work := partitionAddresses(addresses, healthyProviders)
for i, w := range work {
    g.Go(func() error {
        w.provider.RateLimiter().Wait(ctx)
        results[i], err = w.provider.FetchBalances(ctx, w.addrs)
        return err
    })
}
g.Wait()
mergeResults(results)
```

### Evidence

- `golang.org/x/time/rate.Limiter` is goroutine-safe (uses `sync.Mutex` internally)
- `errgroup` is the idiomatic Go fan-out pattern (confirmed from Go blog + sync package)
- Each provider has its own `RateLimiter` instance — independent throttling is already in place

### Burst Parameter

**Keep `burst=1`** — the current implementation is correct:
- `burst=1` at 10 rps: requests spaced exactly 100ms apart. No initial spike.
- `burst=3` at 10 rps: first 3 requests fire instantly, then 100ms spacing. **Steady-state identical**.
- Free-tier providers use sliding-window counters that penalize bursts. `burst=1` is safest.

### Partition by Batch Size

Weight address distribution by provider capability:
- Provider A (`MaxBatchSize=100`): gets 5x more addresses
- Provider B (`MaxBatchSize=20`): gets 1x
- Provider C (`MaxBatchSize=1`): gets 0.05x (or skip for batch work)

### Error Handling

Use **reassignment, not fail-fast**: if provider B fails, redistribute its addresses to A and C.
Don't cancel successful in-flight work.

### Impact

| Chain | Providers | Before (sequential) | After (parallel) | Speedup |
|-------|-----------|--------------------|--------------------|---------|
| BSC   | 8 RPC     | ~8-10 addr/s       | ~60-80 addr/s      | ~8x     |
| BTC   | 3         | ~4-5 addr/s        | ~12-15 addr/s      | ~3x     |
| SOL   | 4-6       | ~500-1000 addr/s   | ~2000-3000 addr/s  | ~3x     |

---

## Combined Impact Projection

### BSC (5,000 addresses, native + USDC + USDT)

| Scenario | API calls | Time |
|----------|-----------|------|
| Current | 15,000 | ~25-30 min |
| + Multicall3 (200 addrs/batch) | 25 | ~5-10 sec |
| + Parallel providers | 25 (split across 8) | ~2-5 sec |
| + JSON-RPC batch fallback | 250-750 | ~30-60 sec (degraded) |

### BTC (5,000 addresses, native only)

| Scenario | API calls | Time |
|----------|-----------|------|
| Current | 5,000 | ~17-21 min |
| + Blockchair (1000/batch) | 5 | ~5-10 sec |
| + Parallel providers (no Blockchair) | 5,000 (split across 3) | ~5-7 min |
| + Both | 5 (Blockchair primary) | ~5-10 sec |

### Full scan (500,000 addresses per chain)

| Chain | Current | Optimized |
|-------|---------|-----------|
| BSC   | ~42 hours | **~2-5 min** (Multicall3) |
| BTC   | ~28 hours | **~10 min** (Blockchair) or ~90 min (parallel only) |
| SOL   | ~25 min  | ~8 min (parallel, already fast) |

---

## Implementation Priority

### Phase 1: Multicall3 for BSC (biggest single win)

- Add `abigen`-generated Multicall3 bindings
- New `BSCMulticallProvider` implementing `BalanceProvider` interface
- Batch 200 addresses per `eth_call` (native + USDC + USDT = 600 subcalls)
- Falls back to existing providers on failure
- **Eliminates ~99.8% of BSC API calls**

### Phase 2: Parallel provider fan-out

- Refactor `Pool.FetchNativeBalances` and `Pool.FetchTokenBalances`
- Split addresses across all healthy providers using weighted partitioning
- Use `errgroup.Group` (zero value) for non-cancelling concurrency
- Reassign failed addresses to remaining providers
- **3-8x throughput improvement for all chains**

### Phase 3: Blockchair for BTC (if free API key acceptable)

- New `BlockchairProvider` with 1,000 addresses/batch (conservative, max is 25K)
- Requires `HDPAY_BLOCKCHAIR_API_KEY` env var (free, non-commercial)
- No testnet — use Blockstream/Mempool for testnet
- **Eliminates ~99.8% of BTC API calls on mainnet**

### Phase 4: JSON-RPC batch as BSC fallback

- When Multicall3 provider is unavailable, use `rpc.Client.BatchCallContext`
- Batch 20-50 `eth_getBalance` calls per HTTP request
- Rate limiter needs `ConsumeN(n)` method for batch-aware token consumption
- **20-50x fewer HTTP connections even in degraded mode**

---

## Open Questions

1. **Blockchair API key**: Requires registration. User comfortable with this external dependency?
   If not, BTC optimization is limited to parallel fan-out (~3x improvement).

2. **Multicall3 gas cap on Ankr free tier**: Need empirical testing. If Ankr enforces a lower
   gas cap than 50M, may need to reduce batch size to 100 addresses.

3. **Rate limiter batch awareness**: JSON-RPC batch fallback needs the rate limiter to consume
   N tokens per batch call. Current `Wait()` only consumes 1. Need `WaitN(ctx, n)` support.

---

## Sources

- [mds1/multicall3 deployments.json](https://github.com/mds1/multicall3) — BSC deployment confirmed
- [mds1/multicall3 Multicall3.sol](https://github.com/mds1/multicall3/blob/main/src/Multicall3.sol) — Full ABI
- [BSC ethconfig/config.go](https://github.com/bnb-chain/bsc) — `RPCGasCap: 50000000`
- [BSC protocol_params.go](https://github.com/bnb-chain/bsc) — EIP-2929 gas costs
- [BSC node/defaults.go](https://github.com/bnb-chain/bsc) — `BatchRequestLimit: 1000`
- [Blockchain.info API docs](https://www.blockchain.com/explorer/api/blockchain_api)
- [Blockchair API v2 changelog](https://github.com/Blockchair/Blockchair.Support/blob/master/API.md) — `/addresses/balances` 25K limit
- [Blockstream Esplora API](https://github.com/Blockstream/esplora/blob/master/API.md) — No batch endpoint
- [go-ethereum rpc/client.go](https://github.com/ethereum/go-ethereum) — `BatchCallContext`, `BatchElem`
- [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) — Token bucket, burst semantics
- [golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) — Fan-out pattern
