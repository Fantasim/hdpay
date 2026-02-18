# Phase 10: Send Interface

> **Status: Detailed** — Ready to build.

<objective>
Build the complete send/sweep interface: preview API (fees, addresses, gas assessment), execute API (sign + broadcast), gas pre-seed API, TX status SSE stream, and the wizard-style frontend with step progression, real-time TX progress, and receipt display with explorer links.
</objective>

## Architecture Overview

The send feature bridges three existing TX engines (BTC, BSC, SOL) into a unified API that the frontend wizard consumes. The handler layer dispatches to chain-specific services based on request parameters.

**Request flow:**
1. User selects chain + token + enters destination
2. `POST /api/send/preview` → handler queries funded addresses from DB, calls chain-specific Preview, returns unified response
3. (If BSC token) `POST /api/send/gas-preseed` → seeds BNB to addresses missing gas
4. `POST /api/send/execute` → handler calls chain-specific Execute, streams `tx_status` SSE events
5. Frontend displays per-TX progress and final receipt

**Key constraint:** TX services need dependencies (KeyService, RPC clients, DB) that are initialized in `main.go` and threaded through `NewRouter`.

---

<tasks>

## Task 1: Backend — Unified Send Types & DB Query

**Files:** `internal/models/types.go`, `internal/db/balances.go`

### 1a. Add unified send types to `models/types.go`

```go
// SendRequest is the common request body for preview and execute.
type SendRequest struct {
    Chain       Chain  `json:"chain"`
    Token       Token  `json:"token"`
    Destination string `json:"destination"`
}

// GasPreSeedRequest is the request body for gas pre-seeding.
type GasPreSeedRequest struct {
    SourceIndex     int      `json:"sourceIndex"`
    TargetAddresses []string `json:"targetAddresses"`
}

// FundedAddressInfo is a row in the preview's funded address table.
type FundedAddressInfo struct {
    AddressIndex int    `json:"addressIndex"`
    Address      string `json:"address"`
    Balance      string `json:"balance"`
    HasGas       bool   `json:"hasGas"`
}

// UnifiedSendPreview is the unified preview response for all chains.
type UnifiedSendPreview struct {
    Chain           Chain               `json:"chain"`
    Token           Token               `json:"token"`
    Destination     string              `json:"destination"`
    FundedCount     int                 `json:"fundedCount"`
    TotalAmount     string              `json:"totalAmount"`
    FeeEstimate     string              `json:"feeEstimate"`
    NetAmount       string              `json:"netAmount"`
    TxCount         int                 `json:"txCount"`
    NeedsGasPreSeed bool                `json:"needsGasPreSeed"`
    GasPreSeedCount int                 `json:"gasPreSeedCount"`
    FundedAddresses []FundedAddressInfo  `json:"fundedAddresses"`
}

// UnifiedSendResult is the unified execute response for all chains.
type UnifiedSendResult struct {
    Chain        Chain       `json:"chain"`
    Token        Token       `json:"token"`
    TxResults    []TxResult  `json:"txResults"`
    SuccessCount int         `json:"successCount"`
    FailCount    int         `json:"failCount"`
    TotalSwept   string      `json:"totalSwept"`
}

// TxResult is a single transaction result in a unified sweep.
type TxResult struct {
    AddressIndex int    `json:"addressIndex"`
    FromAddress  string `json:"fromAddress"`
    TxHash       string `json:"txHash"`
    Amount       string `json:"amount"`
    Status       string `json:"status"`
    Error        string `json:"error,omitempty"`
}
```

### 1b. Add DB method: `GetFundedAddressesJoined`

In `internal/db/balances.go`, add a method that joins `addresses` + `balances` tables to return funded `AddressWithBalance` records for a specific chain and token. This avoids N+1 queries in the handler.

```go
func (d *DB) GetFundedAddressesJoined(chain models.Chain, token models.Token) ([]models.AddressWithBalance, error)
```

The query: SELECT from addresses JOIN balances WHERE chain=? AND token=? AND balance!='0', then for each result also fetch all token balances for that address (needed for gas status check on BSC).

### 1c. Add address validation function

In `internal/api/handlers/send.go` or a new `internal/api/handlers/validation.go`:

```go
func ValidateAddress(chain models.Chain, address string) error
```

- BTC: Use `btcutil.DecodeAddress` with appropriate net params
- BSC: Check `common.IsHexAddress`
- SOL: Check base58 decode, length 32 bytes

<verification>
- `go build ./...` compiles
- New types are importable from handlers package
- DB method returns correct results with test data
</verification>

---

## Task 2: Backend — TX SSE Hub

**Files:** `internal/tx/sse.go` (new)

Create a generic SSE hub for transaction status events, similar to `scanner.SSEHub` but in the `tx` package. This avoids coupling send handlers to the scanner package.

```go
// TxEvent represents a transaction status SSE event.
type TxEvent struct {
    Type string // "tx_status", "tx_complete", "tx_error"
    Data interface{}
}

// TxSSEHub manages SSE clients for transaction status updates.
type TxSSEHub struct { ... }

func NewTxSSEHub() *TxSSEHub
func (h *TxSSEHub) Run(ctx context.Context)
func (h *TxSSEHub) Subscribe() chan TxEvent
func (h *TxSSEHub) Unsubscribe(ch chan TxEvent)
func (h *TxSSEHub) Broadcast(event TxEvent)
```

The hub follows the exact same pattern as `scanner.SSEHub` (mutex-protected client map, non-blocking send).

<verification>
- `go build ./...` compiles
- Hub can be instantiated and broadcast events
</verification>

---

## Task 3: Backend — Send Handlers

**Files:** `internal/api/handlers/send.go`

### 3a. SendDeps struct

Create a dependency container that holds all TX service references:

```go
type SendDeps struct {
    DB           *db.DB
    Config       *config.Config
    BTCService   *tx.BTCConsolidationService
    BSCService   *tx.BSCConsolidationService
    SOLService   *tx.SOLConsolidationService
    GasPreSeed   *tx.GasPreSeedService
    TxHub        *tx.TxSSEHub
}
```

### 3b. PreviewSend handler

`POST /api/send/preview`

1. Parse `SendRequest` from body
2. Validate chain, token, destination address
3. Query `GetFundedAddressesJoined(chain, token)` from DB
4. If no funded addresses → return error
5. Dispatch to chain-specific preview:
   - **BTC native**: Call `BTCService.Preview(ctx, addresses, dest, feeRate)`
     - Convert result to `UnifiedSendPreview`
   - **BSC native**: Call `BSCService.PreviewNativeSweep(ctx, addresses, dest)`
     - Convert `BSCSendPreview` to unified
   - **BSC token**: Estimate gas (count × gasLimitBEP20 × bufferedGasPrice), check which addresses need gas via BNB balance check
     - Build `UnifiedSendPreview` with `needsGasPreSeed=true` if addresses lack BNB
   - **SOL native**: Call `SOLService.PreviewNativeSweep(ctx, addresses, dest)`
     - Convert to unified
   - **SOL token**: Call `SOLService.PreviewTokenSweep(ctx, addresses, dest, token, mint)`
     - Convert to unified
6. Return `APIResponse{Data: unifiedPreview}`

### 3c. ExecuteSend handler

`POST /api/send/execute`

1. Parse `SendRequest` from body
2. Validate (same as preview)
3. Re-fetch funded addresses (balances may have changed since preview)
4. Dispatch to chain-specific execute in a goroutine:
   - **BTC**: `BTCService.Execute` → single TX result
   - **BSC native**: `BSCService.ExecuteNativeSweep` → per-address results
   - **BSC token**: `BSCService.ExecuteTokenSweep` → per-address results
   - **SOL native**: `SOLService.ExecuteNativeSweep` → per-address results
   - **SOL token**: `SOLService.ExecuteTokenSweep` → per-address results
5. As each individual TX completes, broadcast `tx_status` event via `TxSSEHub`
6. Return unified result when all TXs complete

**Important:** The execute runs synchronously (blocking request) since the frontend needs the final result. SSE events provide real-time progress updates for the UI.

Note: To broadcast per-TX events, we need callback hooks in the consolidation services. Since modifying the services is invasive, an alternative approach: the handler runs the execute and returns the full result. The frontend polls or uses SSE for intermediate status. Simplest approach: the execute endpoint returns the full result (blocking), and the frontend shows a loading state. For V1 this is acceptable since sweeps are sequential and each TX takes ~5-30 seconds.

**Revised approach for V1:** Execute is a blocking endpoint. No intermediate SSE for individual TXs. The full result is returned when complete. The TX SSE hub can be added later if needed. This keeps the implementation simple and avoids modifying all three TX engines.

### 3d. GasPreSeed handler

`POST /api/send/gas-preseed`

1. Parse `GasPreSeedRequest` from body
2. Call `GasPreSeedService.Preview(ctx, sourceIndex, targetAddresses)` first
3. If preview shows insufficient balance → return error with preview data
4. Call `GasPreSeedService.Execute(ctx, sourceIndex, targetAddresses)`
5. Return `GasPreSeedResult`

### 3e. SendSSE handler

`GET /api/send/sse`

SSE endpoint for TX status events. Same pattern as scan SSE. For V1, this primarily serves keepalive pings. Per-TX events can be added in V2 by hooking into the consolidation service callbacks.

<verification>
- `go build ./...` compiles
- Handlers return correct error responses for invalid input
- Preview returns correct data shape for each chain
</verification>

---

## Task 4: Backend — Service Initialization & Router

**Files:** `cmd/server/main.go`, `internal/api/router.go`

### 4a. Initialize TX services in `main.go`

In `runServe()`, after database and scanner setup:

1. Create `KeyService` (requires mnemonic file path from config)
2. Create chain-specific clients:
   - BTC: `BTCUTXOFetcher`, `BTCFeeEstimator`, `BTCBroadcaster` (using Esplora/Mempool URLs based on network)
   - BSC: `ethclient.Dial()` to BSC RPC URL (based on network)
   - SOL: `DefaultSOLRPCClient` with RPC URLs (based on network)
3. Create consolidation services: `BTCConsolidationService`, `BSCConsolidationService`, `SOLConsolidationService`, `GasPreSeedService`
4. Create `TxSSEHub`, start in background goroutine
5. Pass services to `NewRouter`

### 4b. Update `NewRouter` signature

Add send deps parameter:

```go
func NewRouter(database *db.DB, cfg *config.Config, sc *scanner.Scanner, hub *scanner.SSEHub, ps *price.PriceService, sendDeps *handlers.SendDeps) chi.Router
```

Register routes:

```go
// Send / Transaction
r.Post("/send/preview", handlers.PreviewSend(sendDeps))
r.Post("/send/execute", handlers.ExecuteSend(sendDeps))
r.Post("/send/gas-preseed", handlers.GasPreSeed(sendDeps))
r.Get("/send/sse", handlers.SendSSE(sendDeps.TxHub))
```

### 4c. Network-aware URL selection

Use `cfg.Network` to pick correct RPC URLs:
- BTC: `blockstream.info/api` vs `blockstream.info/testnet/api`
- BSC: `bsc-dataseed.binance.org` vs `data-seed-prebsc-1-s1.binance.org:8545`
- SOL: `api.mainnet-beta.solana.com` vs `api.devnet.solana.com`

Add URL constants to `internal/config/constants.go`.

<verification>
- `go build ./...` compiles
- Server starts without errors
- `/api/health` still works
- Send routes are registered (check via curl or logs)
</verification>

---

## Task 5: Frontend — TypeScript Types

**Files:** `web/src/lib/types.ts`

Add types matching the backend unified models:

```typescript
// Send-related types
export interface SendRequest {
    chain: Chain;
    token: string;
    destination: string;
}

export interface FundedAddressInfo {
    addressIndex: number;
    address: string;
    balance: string;
    hasGas: boolean;
}

export interface UnifiedSendPreview {
    chain: Chain;
    token: string;
    destination: string;
    fundedCount: number;
    totalAmount: string;
    feeEstimate: string;
    netAmount: string;
    txCount: number;
    needsGasPreSeed: boolean;
    gasPreSeedCount: number;
    fundedAddresses: FundedAddressInfo[];
}

export interface TxResult {
    addressIndex: number;
    fromAddress: string;
    txHash: string;
    amount: string;
    status: string;
    error?: string;
}

export interface UnifiedSendResult {
    chain: Chain;
    token: string;
    txResults: TxResult[];
    successCount: number;
    failCount: number;
    totalSwept: string;
}

export interface GasPreSeedRequest {
    sourceIndex: number;
    targetAddresses: string[];
}

export interface GasPreSeedPreview {
    sourceIndex: number;
    sourceAddress: string;
    sourceBalance: string;
    targetCount: number;
    amountPerTarget: string;
    totalNeeded: string;
    sufficient: boolean;
}

export interface GasPreSeedResult {
    txResults: TxResult[];
    successCount: number;
    failCount: number;
    totalSent: string;
}
```

<verification>
- TypeScript compiles without errors
- Types match backend JSON structure
</verification>

---

## Task 6: Frontend — API Functions

**Files:** `web/src/lib/utils/api.ts`

Add send API functions:

```typescript
// Send API
export function previewSend(req: SendRequest): Promise<APIResponse<UnifiedSendPreview>> {
    return api.post<UnifiedSendPreview>('/send/preview', req);
}

export function executeSend(req: SendRequest): Promise<APIResponse<UnifiedSendResult>> {
    return api.post<UnifiedSendResult>('/send/execute', req);
}

export function gasPreSeed(req: GasPreSeedRequest): Promise<APIResponse<GasPreSeedResult>> {
    return api.post<GasPreSeedResult>('/send/gas-preseed', req);
}
```

<verification>
- TypeScript compiles
- Functions accept correct parameter types
</verification>

---

## Task 7: Frontend — Address Validation

**Files:** `web/src/lib/utils/validation.ts` (new)

Implement chain-specific address validators:

```typescript
export function validateAddress(chain: Chain, address: string): string | null
```

Returns null if valid, error message string if invalid.

**BTC validation:**
- Mainnet: starts with `bc1` (bech32) or `1` / `3` (legacy)
- Testnet: starts with `tb1` (bech32) or `m` / `n` / `2` (legacy)
- Regex for bech32: `/^(bc1|tb1)[a-zA-HJ-NP-Z0-9]{25,62}$/i` (simplified)
- Note: full bech32 checksum validation is server-side; frontend does format check only

**BSC validation:**
- Must start with `0x`
- Must be 42 characters total
- Must be valid hex: `/^0x[0-9a-fA-F]{40}$/`

**SOL validation:**
- Base58 characters only: `/^[1-9A-HJ-NP-Za-km-z]{32,44}$/`

Also add:
```typescript
export function validateAmount(amount: string): string | null
export function isValidDestination(chain: Chain, address: string): boolean
```

<verification>
- Unit tests pass for each chain's valid/invalid addresses
- TypeScript compiles
</verification>

---

## Task 8: Frontend — Send Store

**Files:** `web/src/lib/stores/send.svelte.ts` (new)

Wizard state management using Svelte 5 runes:

```typescript
type SendStep = 'select' | 'preview' | 'gas-preseed' | 'execute' | 'complete';

interface SendStoreState {
    step: SendStep;
    chain: Chain | null;
    token: string | null;
    destination: string;
    preview: UnifiedSendPreview | null;
    gasPreSeedResult: GasPreSeedResult | null;
    executeResult: UnifiedSendResult | null;
    loading: boolean;
    error: string | null;
}
```

Methods:
- `setSelection(chain, token, destination)` — validates and moves to preview step
- `fetchPreview()` — calls API, stores result, advances to preview step
- `fetchGasPreSeed(sourceIndex)` — calls API, stores result
- `executeSwept()` — calls execute API, stores result, advances to complete
- `reset()` — returns to initial state
- `goBack()` — returns to previous step

<verification>
- Store compiles
- State transitions work correctly
</verification>

---

## Task 9: Frontend — Send Page & Components

**Files:**
- `web/src/routes/send/+page.svelte` — main page (wizard container)
- `web/src/lib/components/send/SelectStep.svelte` — Step 1
- `web/src/lib/components/send/PreviewStep.svelte` — Step 2
- `web/src/lib/components/send/GasPreSeedStep.svelte` — Step 3
- `web/src/lib/components/send/ExecuteStep.svelte` — Step 4

### 9a. Wizard Stepper

The page renders a 4-step stepper at the top (matching mockup: Select → Preview → Gas Pre-Seed → Execute), with completed steps collapsed into summary bars.

### 9b. Step 1: Select

- Chain selector (BTC / BSC / SOL buttons or radio)
- Token selector (Native / USDC / USDT — filtered by chain)
- Destination address input with validation feedback
- "Continue" button (disabled until valid)

### 9c. Step 2: Preview

After step 1, auto-fetch preview from API. Display:
- Transaction summary card (chain, token, funded count, total amount, destination)
- Fee estimate card (estimated fee, gas pre-seed warning if applicable)
- Funded addresses table (index, truncated address, balance, gas status badge)
- Back / Continue buttons

### 9d. Step 3: Gas Pre-Seed (conditional)

Only shown for BSC token sweeps when addresses need gas. Display:
- Source address selector (dropdown of BNB-funded addresses)
- Cost summary (amount per target, total needed, source balance)
- Execute pre-seed button with loading state
- Results table after completion
- Skip to Step 4 if no gas pre-seed needed

### 9e. Step 4: Execute

- Confirmation prompt ("This will send X USDT from Y addresses to Z")
- Execute button with loading spinner
- Results table: per-TX row with address, amount, status badge, TX hash link
- Explorer links: chain-specific block explorer URLs
- Summary: success/fail counts, total swept

### 9f. Explorer URL helpers

Add to `web/src/lib/utils/chains.ts`:
```typescript
export function getExplorerTxUrl(chain: Chain, txHash: string, network: Network): string
```

- BTC mainnet: `https://mempool.space/tx/{hash}`
- BTC testnet: `https://mempool.space/testnet/tx/{hash}`
- BSC mainnet: `https://bscscan.com/tx/{hash}`
- BSC testnet: `https://testnet.bscscan.com/tx/{hash}`
- SOL mainnet: `https://solscan.io/tx/{hash}`
- SOL devnet: `https://solscan.io/tx/{hash}?cluster=devnet`

<verification>
- Page renders without errors
- Wizard steps transition correctly
- Preview data displays properly
- Address validation shows feedback
- Compare against mockup `.project/03-mockups/screens/send.html`
</verification>

---

## Task 10: Backend Tests

**Files:** `internal/api/handlers/send_test.go`, `web/src/lib/utils/validation.test.ts`

### 10a. Handler tests

Test with mock TX services:
- Preview returns correct response for each chain/token combo
- Preview returns error for invalid chain/token
- Preview returns error for invalid destination address
- Preview returns error when no funded addresses
- Execute returns correct response shape
- GasPreSeed validates inputs

### 10b. Frontend validation tests

- BTC address validation (valid bech32, valid legacy, invalid)
- BSC address validation (valid checksummed, valid lowercase, invalid)
- SOL address validation (valid, invalid characters, wrong length)

<verification>
- `go test ./internal/api/handlers/...` passes
- `npm run test` in web/ passes (or vitest)
</verification>

</tasks>

---

<success_criteria>
1. `POST /api/send/preview` returns funded addresses, amounts, fees for all chain/token combos
2. `POST /api/send/execute` dispatches to correct TX engine and returns results
3. `POST /api/send/gas-preseed` distributes BNB to target addresses
4. Frontend wizard completes full flow: select → preview → (gas pre-seed) → execute → receipt
5. Address validation works on both frontend (format) and backend (decode)
6. Explorer links open correct URLs for each chain
7. All tests pass
8. No hardcoded constants — all values in constants files
9. Server starts without errors with TX services initialized
</success_criteria>

<research_needed>
None — all required patterns exist in the codebase already.
</research_needed>
