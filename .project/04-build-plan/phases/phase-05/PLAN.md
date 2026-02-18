# Phase 5: Scan UI + SSE

> **Status: Detailed** — Ready to build.

<objective>
Build the scan control API endpoints, SSE streaming endpoint, and the frontend scan page with chain selector, max ID input, start/stop controls, real-time progress bars, provider status display, and scan history — matching the scan.html mockup.
</objective>

## Context

Phase 4 built the scanner engine: multi-provider BTC/BSC/SOL scanner with round-robin + failover, SSE hub for event fan-out, and a scanner orchestrator exposing `StartScan`, `StopScan`, `Status`, `IsRunning`. All of that needs to be wired through HTTP handlers to a Svelte frontend.

Key backend interfaces already in place:
- `scanner.Scanner` — `StartScan(ctx, chain, maxID)`, `StopScan(chain)`, `Status(chain)`, `IsRunning(chain)`
- `scanner.SSEHub` — `Subscribe() chan Event`, `Unsubscribe(ch)`, `Broadcast(event)`, `Run(ctx)`
- `scanner.Event` — `{Type: "scan_progress"|"scan_complete"|"scan_error", Data: ...}`
- `scanner.SetupScanner(db, cfg, hub) (*Scanner, error)` — factory

## Tasks

### Task 1: Wire Scanner into main.go and Router

**Goal**: Create SSE hub + scanner in `runServe()`, pass to router.

**Files to modify**:
- `cmd/server/main.go` — create hub, setup scanner, start `hub.Run()`, pass to router
- `internal/api/router.go` — accept `*scanner.Scanner` and `*scanner.SSEHub`, register scan routes

**Steps**:
1. In `runServe()`, after DB migrations:
   - Create `hub := scanner.NewSSEHub()`
   - Create root context with cancel for hub: `hubCtx, hubCancel := context.WithCancel(context.Background())`
   - `defer hubCancel()`
   - `go hub.Run(hubCtx)`
   - Call `sc, err := scanner.SetupScanner(database, cfg, hub)`
   - Pass `sc` and `hub` to `api.NewRouter(database, cfg, sc, hub)`
2. In `router.go`, change `NewRouter` signature to `NewRouter(database *db.DB, cfg *config.Config, sc *scanner.Scanner, hub *scanner.SSEHub) chi.Router`
3. Add scan routes inside the `/api` route group:
   ```
   POST /api/scan/start    → handlers.StartScan(sc)
   POST /api/scan/stop     → handlers.StopScan(sc)
   GET  /api/scan/status   → handlers.GetScanStatus(sc, database)
   GET  /api/scan/sse      → handlers.ScanSSE(hub)
   ```

**Verification**: `go build ./cmd/server/` compiles without errors.

---

### Task 2: Scan API Handlers

**Goal**: Implement the four scan HTTP handlers.

**File to create/modify**:
- `internal/api/handlers/scan.go`

**Handlers**:

1. **`StartScan(sc *scanner.Scanner) http.HandlerFunc`** — `POST /api/scan/start`
   - Parse JSON body: `{ "chain": "BTC", "maxId": 5000 }`
   - Validate chain with existing `isValidChain()`
   - Validate maxId: must be > 0 and <= `config.MaxAddressesPerChain`
   - Call `sc.StartScan(r.Context(), chain, maxID)`
   - If `errors.Is(err, config.ErrScanAlreadyRunning)` → 409 Conflict
   - If other error → 500
   - Success → 200 `{"data": {"message": "scan started", "chain": "BTC", "maxId": 5000}}`
   - Log: INFO on start, WARN on already running, ERROR on failure

2. **`StopScan(sc *scanner.Scanner) http.HandlerFunc`** — `POST /api/scan/stop`
   - Parse JSON body: `{ "chain": "BTC" }`
   - Validate chain
   - Call `sc.StopScan(chain)` — fire-and-forget
   - Return 200 `{"data": {"message": "scan stop requested", "chain": "BTC"}}`
   - Log: INFO on stop request

3. **`GetScanStatus(sc *scanner.Scanner, database *db.DB) http.HandlerFunc`** — `GET /api/scan/status`
   - Optional query param `?chain=BTC`
   - If chain provided: return single `ScanState` from `sc.Status(chain)`, augmented with `"isRunning"` from `sc.IsRunning(chain)`
   - If no chain: iterate `models.AllChains`, return map of chain → status
   - Return 200 with data
   - Log: DEBUG on status request

4. **`ScanSSE(hub *scanner.SSEHub) http.HandlerFunc`** — `GET /api/scan/sse`
   - Set headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no`
   - Subscribe to hub: `ch := hub.Subscribe()`, `defer hub.Unsubscribe(ch)`
   - Flush headers immediately
   - Loop with `select`:
     - `case event := <-ch`: serialize as `event: <type>\ndata: <json>\n\n`, flush
     - `case <-time.After(config.SSEKeepAliveInterval)`: write `: keepalive\n\n`, flush
     - `case <-r.Context().Done()`: return (client disconnected)
   - Log: INFO on connect/disconnect, DEBUG on events sent

**Request body type** (define in scan.go):
```go
type startScanRequest struct {
    Chain string `json:"chain"`
    MaxID int    `json:"maxId"`
}

type stopScanRequest struct {
    Chain string `json:"chain"`
}
```

**Response type for augmented status** (define in scan.go):
```go
type scanStatusResponse struct {
    models.ScanState
    IsRunning bool `json:"isRunning"`
}
```

**Verification**: `go build ./...` compiles. Unit tests in Task 3.

---

### Task 3: Scan Handler Tests

**Goal**: Test all four scan handlers.

**File to create**:
- `internal/api/handlers/scan_test.go`

**Tests**:
1. `TestStartScan_Success` — valid chain + maxId → 200
2. `TestStartScan_InvalidChain` — bad chain → 400
3. `TestStartScan_InvalidMaxId` — 0 or negative → 400
4. `TestStartScan_AlreadyRunning` — mock scanner returns ErrScanAlreadyRunning → 409
5. `TestStopScan_Success` — valid chain → 200
6. `TestStopScan_InvalidChain` — bad chain → 400
7. `TestGetScanStatus_SingleChain` — ?chain=BTC → returns status
8. `TestGetScanStatus_AllChains` — no query param → returns map of 3 chains
9. `TestScanSSE_Connect` — verify SSE headers set, keepalive sent

**Approach**: Create a mock scanner interface for testing, or use the real scanner with a test DB. The handlers take concrete `*scanner.Scanner`, so either:
- Create a real scanner with mock providers for testing, or
- Extract a `ScanService` interface for handlers (if warranted)

Given the scanner is already constructed with mock-friendly setup, use a real scanner with a test DB.

**Verification**: `go test ./internal/api/handlers/ -v -run TestS` passes.

---

### Task 4: Add Scan API Functions to Frontend

**Goal**: Add scan-related API functions and missing types.

**Files to modify**:
- `web/src/lib/utils/api.ts` — add scan API functions
- `web/src/lib/types.ts` — add SSE event types
- `web/src/lib/constants.ts` — add scan-related constants

**api.ts additions**:
```typescript
// Scan API
export interface StartScanParams {
    chain: Chain;
    maxId: number;
}

export function startScan(params: StartScanParams): Promise<APIResponse<{ message: string; chain: Chain; maxId: number }>> {
    return api.post('/scan/start', params);
}

export function stopScan(chain: Chain): Promise<APIResponse<{ message: string; chain: Chain }>> {
    return api.post('/scan/stop', { chain });
}

export function getScanStatus(chain?: Chain): Promise<APIResponse<ScanStatusResponse>> {
    const path = chain ? `/scan/status?chain=${chain}` : '/scan/status';
    return api.get(path);
}
```

**types.ts additions**:
```typescript
// SSE event payloads
export interface ScanCompleteEvent {
    chain: Chain;
    scanned: number;
    found: number;
    duration: string;
}

export interface ScanErrorEvent {
    chain: Chain;
    error: string;
    message: string;
}

// Augmented scan status from API
export interface ScanStatusResponse {
    [key: string]: ScanStateWithRunning | null;
}

export interface ScanStateWithRunning extends ScanState {
    isRunning: boolean;
}
```

**constants.ts additions**:
```typescript
export const DEFAULT_MAX_SCAN_ID = 5000;
export const SSE_MAX_RECONNECT_DELAY_MS = 30_000;
export const SSE_BACKOFF_MULTIPLIER = 2;
```

**Verification**: `npm run check` passes in `web/` directory.

---

### Task 5: Scan Store with SSE Connection

**Goal**: Build the scan store managing scan state, SSE connection, and progress updates.

**File to create**:
- `web/src/lib/stores/scan.svelte.ts` (`.svelte.ts` required for runes to work in a store)

**Store design**:
- Uses Svelte 5 runes: `$state`, `$derived`
- Factory function pattern (matching `addresses.ts` pattern)
- Manages EventSource connection lifecycle: connect, disconnect, reconnect with exponential backoff
- Per-chain state tracking: stores `ScanStateWithRunning` for each chain
- Live progress from SSE: stores latest `ScanProgress` per chain
- Named event listeners: `addEventListener('scan_progress', ...)` etc. (NOT `onmessage` — named events require `addEventListener`)

**State shape**:
```typescript
interface ScanStoreState {
    statuses: Record<Chain, ScanStateWithRunning | null>;
    progress: Record<Chain, ScanProgress | null>;
    lastComplete: Record<Chain, ScanCompleteEvent | null>;
    lastError: Record<Chain, ScanErrorEvent | null>;
    sseConnected: boolean;
    sseStatus: 'disconnected' | 'connecting' | 'connected' | 'error';
    loading: boolean;
    error: string | null;
}
```

**Methods**:
- `fetchStatus()` — GET /api/scan/status, populates `statuses`
- `startScan(chain, maxId)` — POST start, then fetches status
- `stopScan(chain)` — POST stop
- `connectSSE()` — creates EventSource at `/api/scan/sse`, adds named event listeners
- `disconnectSSE()` — closes EventSource, clears retry timer
- Internal: `scheduleReconnect()` with exponential backoff using `SSE_RECONNECT_DELAY_MS` and `SSE_MAX_RECONNECT_DELAY_MS`

**SSE event handling**:
- `scan_progress` → update `progress[chain]`
- `scan_complete` → update `lastComplete[chain]`, clear `progress[chain]`, refresh status via API
- `scan_error` → update `lastError[chain]`, clear `progress[chain]`, refresh status via API

**Lifecycle**: SSE is connected in page `onMount`, disconnected in return cleanup.

**Verification**: Store compiles, `npm run check` passes.

---

### Task 6: ScanControl Component

**Goal**: Build the scan control panel matching the mockup.

**File to create**:
- `web/src/lib/components/scan/ScanControl.svelte`

**Props**: None (uses store directly)

**Layout** (from mockup):
- Card with "Scan Control" header
- Grid with 4 columns: Chain select, Max ID input, Start button, Stop button
- Info alert below showing estimate: "Scan will check addresses 0 through {maxId}. Estimated time: ~{estimate} minutes."

**Behavior**:
- Chain select: dropdown with BTC, BSC, SOL options
- Max ID input: number input, default `DEFAULT_MAX_SCAN_ID`, min 1, max 500000
- Start Scan button: primary style, calls `scanStore.startScan(chain, maxId)`, disabled when scan running for selected chain
- Stop button: danger style, calls `scanStore.stopScan(chain)`, disabled when no scan running for selected chain
- Estimate calculation: `Math.ceil(maxId / 1000)` minutes (rough heuristic)

**Verification**: Component renders in scan page.

---

### Task 7: ScanProgress Component

**Goal**: Build scan progress cards for each chain, matching the mockup's "Active & Recent Scans" section.

**File to create**:
- `web/src/lib/components/scan/ScanProgress.svelte`

**Props**:
```typescript
interface Props {
    chain: Chain;
    status: ScanStateWithRunning | null;
    progress: ScanProgress | null;
    lastComplete: ScanCompleteEvent | null;
    lastError: ScanErrorEvent | null;
}
```

**Layout** (from mockup):
- Each chain gets a scan-item card:
  - Header: chain dot (colored) + chain name + chain badge (e.g. "Bitcoin") + status badge
  - Progress bar (if scanning): `scanned / total` width percentage, chain-colored
  - Details line:
    - If scanning: `{scanned} / {total} scanned — {found} funded — ETA {eta}`
    - If completed: `{total} scanned — {found} funded — {duration}`
    - If idle: `Last scan: {time ago} — {found} funded`
    - If error: error message in red

**Status badges**:
- `scanning` → "Scanning" accent badge
- `completed` → "Completed" success badge
- `failed` → "Error" error badge
- `idle`/null → "Idle" default badge

**ETA calculation**: `(total - scanned) * elapsed / scanned` — format as minutes/seconds.

**Verification**: Component renders with mock data.

---

### Task 8: ProviderStatus Component

**Goal**: Display provider health grid matching the mockup.

**File to create**:
- `web/src/lib/components/scan/ProviderStatus.svelte`

**Design decision**: Provider status is currently not exposed by the backend scanner API. For V1, display static provider names per chain with a "Healthy" default status. This is a display-only component; live health tracking is a future enhancement.

**Layout** (from mockup):
- Card with "Provider Status" header
- 4-column grid of provider items
- Each item: colored dot (green=healthy) + provider name + status text

**Static data**:
- Blockstream (BTC), Mempool.space (BTC), BscScan (BSC), Solana RPC (SOL)
- All show as "Healthy" by default
- Can optionally show "Rate limited" (warning) if SSE reports provider issues

**Verification**: Component renders.

---

### Task 9: Scan Page Assembly

**Goal**: Wire all components together in the scan page.

**File to modify**:
- `web/src/routes/scan/+page.svelte` — replace placeholder

**Layout**:
1. `Header` with title "Scan"
2. `ScanControl` component (card)
3. "Active & Recent Scans" section title
4. Three `ScanProgress` cards (one per chain: BTC, BSC, SOL)
5. `ProviderStatus` card

**Lifecycle**:
- `onMount`: call `scanStore.fetchStatus()`, then `scanStore.connectSSE()`
- `onMount` return: call `scanStore.disconnectSSE()`

**Verification**: Page loads, displays scan controls, shows chain progress cards, SSE connects.

---

### Task 10: Integration Test — Full Flow

**Goal**: Verify the entire scan flow works end-to-end.

**Manual verification steps**:
1. `go build ./cmd/server/` — compiles
2. `go test ./...` — all tests pass
3. `cd web && npm run check` — TypeScript check passes
4. `cd web && npm run build` — production build succeeds
5. Verify scan page renders with all three chain cards
6. Verify SSE endpoint returns `text/event-stream` content type
7. Verify start/stop/status endpoints return correct responses

**Automated**:
- Run `go test ./internal/api/handlers/ -v`
- Run `cd web && npm run check`

---

## Success Criteria

- [ ] Scanner is wired in `main.go` — hub, scanner created and passed to router
- [ ] `POST /api/scan/start` starts a scan, returns 200 (or 409 if already running)
- [ ] `POST /api/scan/stop` stops a running scan
- [ ] `GET /api/scan/status` returns scan state for all chains (with `isRunning` flag)
- [ ] `GET /api/scan/sse` streams SSE events with proper headers, keepalive, and named events
- [ ] Frontend scan store manages SSE connection with auto-reconnect
- [ ] Scan page matches mockup layout: control panel, chain progress cards, provider grid
- [ ] Progress bars update in real-time via SSE
- [ ] Start/Stop buttons enable/disable based on scan state
- [ ] All Go tests pass (`go test ./...`)
- [ ] TypeScript check passes (`npm run check`)
- [ ] Frontend builds (`npm run build`)

## Files to Create

| File | Purpose |
|------|---------|
| `web/src/lib/stores/scan.svelte.ts` | Scan store with SSE connection management |
| `web/src/lib/components/scan/ScanControl.svelte` | Scan control panel (chain, maxId, start/stop) |
| `web/src/lib/components/scan/ScanProgress.svelte` | Per-chain progress card with bar and stats |
| `web/src/lib/components/scan/ProviderStatus.svelte` | Provider health grid |
| `internal/api/handlers/scan_test.go` | Handler unit tests |

## Files to Modify

| File | Changes |
|------|---------|
| `cmd/server/main.go` | Wire SSEHub + Scanner + hub.Run() |
| `internal/api/router.go` | Accept scanner/hub, add scan routes |
| `internal/api/handlers/scan.go` | 4 handlers: start, stop, status, SSE |
| `web/src/lib/utils/api.ts` | Add startScan, stopScan, getScanStatus |
| `web/src/lib/types.ts` | Add ScanCompleteEvent, ScanErrorEvent, ScanStateWithRunning |
| `web/src/lib/constants.ts` | Add DEFAULT_MAX_SCAN_ID, SSE backoff constants |
| `web/src/routes/scan/+page.svelte` | Replace placeholder with full scan page |
