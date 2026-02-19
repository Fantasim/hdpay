# Phase 4: TX Safety — Advanced

## Objective

Fix remaining TX safety issues: UTXO re-validation at execute time, BSC balance recheck, partial sweep resume, gas pre-seed idempotency, SOL ATA confirmation after creation, BSC gas re-estimation with cap, and BSC nonce gap handling.

## Audit IDs Covered

A6, A7, A8, A9, A10, A11, A12

## Tasks

### Task 1: BTC UTXO Re-Validation at Execute Time (A6)

**Problem**: BTC UTXOs can be spent externally between preview and execute. The user previews "5 UTXOs, 0.5 BTC" but by execute time, some UTXOs may be gone. Current code re-fetches UTXOs at execute but has no way to detect divergence from what the user saw.

**Solution**: Accept optional `expectedInputCount` and `expectedTotalSats` in the execute request. If provided, compare against re-fetched UTXOs at execute time. If UTXO count dropped by >20% or total value decreased by >10%, return an error requiring re-preview.

**Changes**:

1. **`internal/models/types.go`** — Add `ExpectedInputCount` and `ExpectedTotalSats` fields to `SendRequest`:
   ```go
   ExpectedInputCount int   `json:"expectedInputCount,omitempty"`
   ExpectedTotalSats  int64 `json:"expectedTotalSats,omitempty"`
   ```

2. **`internal/config/constants.go`** — Add UTXO divergence thresholds:
   ```go
   BTCUTXOCountDivergenceThreshold = 0.20  // 20% drop in UTXO count → error
   BTCUTXOValueDivergenceThreshold = 0.10  // 10% drop in total value → error
   ```

3. **`internal/config/errors.go`** — Add:
   ```go
   ErrUTXODiverged = errors.New("UTXO set diverged significantly since preview")
   ErrorUTXODiverged = "ERROR_UTXO_DIVERGED"
   ```

4. **`internal/tx/btc_tx.go`** — Add `ValidateUTXOsAgainstPreview` method to `BTCConsolidationService`:
   - Takes expected count + total sats from preview
   - Compares against freshly fetched UTXOs
   - Returns detailed error with old vs new values if diverged
   - Called from `Execute` after UTXOs are fetched, before building TX

5. **`internal/api/handlers/send.go`** — Pass expected values from `SendRequest` through to `executeBTCSweep` → `BTCConsolidationService.Execute`.

**Files Modified**: `internal/models/types.go`, `internal/config/constants.go`, `internal/config/errors.go`, `internal/tx/btc_tx.go`, `internal/api/handlers/send.go`

**Tests**: 3 tests
- UTXO count dropped below threshold → error
- UTXO value dropped below threshold → error
- UTXO set within tolerance → proceed

---

### Task 2: BSC Balance Recheck + Divergence Logging (A7)

**Problem**: Preview uses DB-stored balances, execute re-fetches real-time. BSC native sweeps already re-fetch (Phase 3), but token sweeps don't re-fetch the token balance from chain — they use the `addr.TokenBalances` from DB. Also, no logging of the discrepancy between preview amounts and execute amounts.

**Solution**:
- BSC native sweeps: already re-fetch. Add logging comparing DB balance vs real-time balance.
- BSC token sweeps: The token balance from `addr.TokenBalances` is used directly. Add a check using `eth_call` to read the on-chain token balance (via BEP-20 `balanceOf`). If on-chain balance is lower than expected, use the lower amount. Log the delta.

**Changes**:

1. **`internal/tx/bsc_tx.go`** — In `sweepNativeAddress`:
   - Log the old (DB-stored) vs new (real-time) balance before proceeding
   - Add constant `BSCMinSweepAmountWei` to skip addresses with negligible balance

2. **`internal/tx/bsc_tx.go`** — In `sweepTokenAddress`:
   - Add `EthClientWrapper.CallContract` or use a simple `balanceOf` eth_call to re-fetch token balance
   - Actually simpler: add `BalanceOfBEP20` method that encodes `balanceOf(address)` call and executes via `ethClient.CallContract`
   - Compare fetched balance with `addr.TokenBalances[token]`. Use the lower value. Log discrepancy.

3. **`internal/tx/bsc_tx.go`** — Add `EthCallWrapper` method to `EthClientWrapper` interface:
   ```go
   CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
   ```

4. **`internal/config/constants.go`** — Add:
   ```go
   BSCMinNativeSweepWei = "100000000000000" // 0.0001 BNB — below this, skip
   BEP20BalanceOfMethodID = "70a08231"       // keccak256("balanceOf(address)")[:4]
   ```

**Files Modified**: `internal/tx/bsc_tx.go`, `internal/config/constants.go`

**Tests**: 3 tests
- Native balance recheck: lower real-time balance uses new value
- Token balance recheck: on-chain lower than DB → uses on-chain
- Negligible balance skipped with warning

---

### Task 3: Partial Sweep Resume (A8)

**Problem**: BSC/SOL sweep fails at address #5 of 20 → addresses #1-4 are swept, no way to resume from #5. User must start over, but addresses #1-4 are now empty.

**Solution**: Add `POST /api/send/resume` endpoint that:
1. Accepts a `sweepID`
2. Queries `tx_state` for all TXs in that sweep
3. Skips addresses with `confirmed` or `dismissed` status
4. Retries addresses with `failed` status
5. Surfaces addresses with `uncertain` status for user decision
6. Re-uses the same sweepID

**Changes**:

1. **`internal/models/types.go`** — Add `ResumeRequest` and `ResumeSummary` types:
   ```go
   type ResumeRequest struct {
       SweepID     string `json:"sweepID"`
       Destination string `json:"destination"`
   }

   type ResumeSummary struct {
       SweepID       string `json:"sweepID"`
       Chain         string `json:"chain"`
       Token         string `json:"token"`
       TotalTxs      int    `json:"totalTxs"`
       Confirmed     int    `json:"confirmed"`
       Failed        int    `json:"failed"`
       Uncertain     int    `json:"uncertain"`
       Pending       int    `json:"pending"`
       ToRetry       int    `json:"toRetry"`
   }
   ```

2. **`internal/db/tx_state.go`** — Add:
   - `GetResumableTxStates(sweepID string) ([]TxStateRow, error)` — returns only failed/uncertain TXs for the sweep
   - `GetSweepChainToken(sweepID string) (chain, token string, err error)` — returns chain/token for a sweep

3. **`internal/api/handlers/send.go`** — Add `ResumeSweep` handler:
   - Validate sweepID exists
   - Get sweep summary (confirmed/failed/uncertain counts)
   - Re-acquire chain lock
   - For BSC/SOL: re-execute only the failed addresses
   - For BTC: BTC is a single TX, so resume = full re-execute
   - Return the same `UnifiedSendResult` format

4. **`internal/api/router.go`** — Add route: `POST /api/send/resume`

5. **`internal/tx/bsc_tx.go`** — Add `ResumeNativeSweep` / `ResumeTokenSweep` methods that accept specific address indices to retry

6. **`internal/tx/sol_tx.go`** — Add `ResumeNativeSweep` / `ResumeTokenSweep` methods

**Files Modified**: `internal/models/types.go`, `internal/db/tx_state.go`, `internal/api/handlers/send.go`, `internal/api/router.go`, `internal/tx/bsc_tx.go`, `internal/tx/sol_tx.go`

**Tests**: 4 tests
- Resume skips confirmed addresses
- Resume retries failed addresses
- Resume surfaces uncertain for user
- Resume with all confirmed → no-op

---

### Task 4: Gas Pre-Seed Idempotency (A9)

**Problem**: If gas pre-seed receipt poll times out, the TX may still land on-chain. User retries = duplicate gas sends. No deduplication based on existing in-flight TXs.

**Solution**: Before sending gas to an address, check `tx_state` for an existing TX from the same source to the same target with a non-terminal status. If found:
- `confirming`/`uncertain` → skip, return "already in flight"
- `confirmed` → skip, return "already funded"
- `failed` → proceed with retry (fresh nonce)

Also wire gas pre-seed TXs through `tx_state` for full lifecycle tracking.

**Changes**:

1. **`internal/tx/gas.go`** — In `Execute`:
   - Wire `tx_state` tracking (create → broadcasting → confirming → confirmed/failed)
   - Before each send, call `db.GetGasPreSeedState(fromAddr, toAddr)` to check for existing in-flight TX
   - Mark receipt-timeout TXs as `uncertain` instead of `failed`
   - Add `sweepID` parameter (gas pre-seed gets its own sweep ID)

2. **`internal/db/tx_state.go`** — Add:
   - `GetGasPreSeedState(fromAddress, toAddress string) (*TxStateRow, error)` — finds most recent non-dismissed TX between these addresses

3. **`internal/config/constants.go`** — Add gas pre-seed token identifier:
   ```go
   TokenGasPreSeed = "GAS_PRESEED" // token identifier for gas pre-seed tx_state rows
   ```

4. **`internal/api/handlers/send.go`** — Pass sweepID to gas pre-seed execute

**Files Modified**: `internal/tx/gas.go`, `internal/db/tx_state.go`, `internal/config/constants.go`, `internal/api/handlers/send.go`

**Tests**: 3 tests
- Duplicate gas send detected and skipped (in-flight)
- Already confirmed target skipped
- Failed target allows retry

---

### Task 5: SOL ATA Confirmation After Creation (A10)

**Problem**: First TX creates destination ATA. After confirmation, the code marks `destATAExists = true` and stops including CreateATA instructions. But due to RPC lag, the next `GetAccountInfo` call might not see the ATA yet, causing the next TX to fail.

**Solution**: After the first TX that includes CreateATA confirms, explicitly poll `GetAccountInfo(destATA)` until the ATA exists (or timeout after 30s). Only then proceed with subsequent TXs.

**Changes**:

1. **`internal/tx/sol_tx.go`** — In `ExecuteTokenSweep`, after line 882 (`destATAExists = true`):
   - Call new `waitForATAExistence(ctx, destATAStr)` method
   - Poll `GetAccountInfo` every 2s for up to 30s
   - If ATA not visible after timeout, log warning but continue (optimistic)

2. **`internal/config/constants.go`** — Add:
   ```go
   SOLATAConfirmationTimeout      = 30 * time.Second
   SOLATAConfirmationPollInterval = 2 * time.Second
   ```

3. **`internal/config/errors.go`** — Add:
   ```go
   ErrSOLATANotVisible = errors.New("ATA not visible after creation confirmation")
   ```

**Files Modified**: `internal/tx/sol_tx.go`, `internal/config/constants.go`, `internal/config/errors.go`

**Tests**: 2 tests
- ATA becomes visible within timeout → proceed
- ATA not visible after timeout → warning logged, continues

---

### Task 6: BSC Gas Re-Estimation with Cap (A11)

**Problem**: Gas price estimated at preview, not re-checked at execute. Gas spike = stuck TX (underpay) or overpay. Current code does re-estimate at execute time, but there's no comparison or cap vs preview price.

**Solution**: Accept `expectedGasPrice` in the execute request (from preview). At execute time:
- Re-fetch gas price via `SuggestGasPrice()`
- If new price > 2x expected → return error, require re-preview
- If new price 1-2x expected → proceed with new price + 20% buffer
- Log the price change

**Changes**:

1. **`internal/models/types.go`** — Add `ExpectedGasPrice` to `SendRequest`:
   ```go
   ExpectedGasPrice string `json:"expectedGasPrice,omitempty"` // wei
   ```

2. **`internal/tx/bsc_tx.go`** — Add `ValidateGasPrice` function:
   - Takes expected gas price and current gas price
   - Returns error if current > 2x expected
   - Returns the buffered current price if within range
   - Logs the comparison

3. **`internal/config/constants.go`** — Add:
   ```go
   BSCGasPriceMaxIncreaseMultiplier = 2 // reject if gas price doubled since preview
   ```

4. **`internal/config/errors.go`** — Add:
   ```go
   ErrGasPriceSpiked = errors.New("gas price increased more than 2x since preview")
   ErrorGasPriceSpiked = "ERROR_GAS_PRICE_SPIKED"
   ```

5. **`internal/api/handlers/send.go`** — Pass `expectedGasPrice` to BSC sweep execution. In `executeBSCNativeSweep` and `executeBSCTokenSweep`, validate gas price before proceeding.

**Files Modified**: `internal/models/types.go`, `internal/tx/bsc_tx.go`, `internal/config/constants.go`, `internal/config/errors.go`, `internal/api/handlers/send.go`

**Tests**: 3 tests
- Gas price within range → proceed
- Gas price > 2x → error
- No expected gas price → skip validation (backward compat)

---

### Task 7: Gas Pre-Seed Nonce Gap Handling (A12)

**Problem**: Gas pre-seed uses `PendingNonceAt` once and increments locally. If a TX broadcast times out but actually lands on-chain, subsequent TXs at the same nonce will conflict. Also, if a TX fails, the locally-incremented nonce creates a gap.

**Solution**:
- Only increment nonce after broadcast succeeds (current code already does this via `if txResult.TxHash != "" { nonce++ }`)
- Mark timed-out TXs as `uncertain` instead of `failed` (from Task 4)
- On nonce-related errors (`nonce too low`, `already known`), re-fetch `PendingNonceAt` and retry once

**Changes**:

1. **`internal/tx/gas.go`** — In `sendGasPreSeed`:
   - Detect nonce-related broadcast errors (strings contain "nonce too low" or "already known")
   - On nonce error: re-fetch PendingNonceAt, rebuild+resign TX, retry broadcast once
   - Mark receipt timeout as `uncertain` instead of `failed`
   - Return the new nonce to caller so the local counter stays correct

2. **`internal/tx/gas.go`** — Change `Execute` to accept nonce recovery:
   - If sendGasPreSeed returns a nonce correction, update the local counter

3. **`internal/config/errors.go`** — Add:
   ```go
   ErrNonceConflict = errors.New("nonce conflict detected, re-fetched")
   ```

**Files Modified**: `internal/tx/gas.go`, `internal/config/errors.go`

**Tests**: 3 tests
- Normal gas pre-seed works unchanged
- Nonce-too-low error → re-fetch + retry succeeds
- Receipt timeout → status `uncertain` (not `failed`)

---

## New Constants Summary

```go
// constants.go additions
BTCUTXOCountDivergenceThreshold  = 0.20
BTCUTXOValueDivergenceThreshold  = 0.10
BSCMinNativeSweepWei             = "100000000000000"  // 0.0001 BNB
BEP20BalanceOfMethodID           = "70a08231"
SOLATAConfirmationTimeout        = 30 * time.Second
SOLATAConfirmationPollInterval   = 2 * time.Second
BSCGasPriceMaxIncreaseMultiplier = 2
TokenGasPreSeed                  = "GAS_PRESEED"
```

## New Errors Summary

```go
// errors.go additions
ErrUTXODiverged     = errors.New("UTXO set diverged significantly since preview")
ErrGasPriceSpiked   = errors.New("gas price increased more than 2x since preview")
ErrSOLATANotVisible = errors.New("ATA not visible after creation confirmation")
ErrNonceConflict    = errors.New("nonce conflict detected")

ErrorUTXODiverged   = "ERROR_UTXO_DIVERGED"
ErrorGasPriceSpiked = "ERROR_GAS_PRICE_SPIKED"
```

## Files Modified (All Tasks)

| File | Tasks |
|------|-------|
| `internal/config/constants.go` | 1, 2, 5, 6 |
| `internal/config/errors.go` | 1, 5, 6, 7 |
| `internal/models/types.go` | 1, 3, 6 |
| `internal/tx/btc_tx.go` | 1 |
| `internal/tx/bsc_tx.go` | 2, 3, 6 |
| `internal/tx/sol_tx.go` | 3, 5 |
| `internal/tx/gas.go` | 4, 7 |
| `internal/db/tx_state.go` | 3, 4 |
| `internal/api/handlers/send.go` | 1, 3, 4, 6 |
| `internal/api/router.go` | 3 |

## Estimated Tests: ~21

| Task | Tests |
|------|-------|
| 1: UTXO re-validation | 3 |
| 2: BSC balance recheck | 3 |
| 3: Partial sweep resume | 4 |
| 4: Gas pre-seed idempotency | 3 |
| 5: SOL ATA confirmation | 2 |
| 6: BSC gas re-estimation | 3 |
| 7: Nonce gap handling | 3 |

## Success Criteria

- [ ] BTC execute rejects when UTXO set diverged >20% count or >10% value from preview
- [ ] BSC sweeps log old-vs-new balance at execute time
- [ ] BSC token sweeps re-fetch on-chain token balance
- [ ] `POST /api/send/resume` resumes failed BSC/SOL sweeps, skipping confirmed addresses
- [ ] Gas pre-seed skips already-funded or in-flight targets
- [ ] SOL token sweep waits for ATA visibility after creation TX confirms
- [ ] BSC execute rejects when gas price >2x preview price
- [ ] Gas pre-seed handles nonce conflicts via re-fetch + retry
- [ ] All new code has tests, `go test ./...` passes
- [ ] No hardcoded values — all thresholds/timeouts in constants.go

## Verification

1. Run `go test ./internal/tx/... -v` — all tests pass
2. Run `go test ./internal/db/... -v` — all tests pass
3. Run `go test ./internal/api/... -v` — all tests pass
4. Run `go test ./... -count=1` — full suite passes
5. Run `go vet ./...` — no issues
6. Verify no hardcoded strings/numbers outside constants.go and errors.go
