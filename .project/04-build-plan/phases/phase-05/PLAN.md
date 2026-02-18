# Phase 5: Scan UI + SSE

> **Status: Outline** — Will be expanded with full task details when this phase is reached.

<objective>
Build the scan control API endpoints, SSE streaming endpoint, and the frontend scan page with chain dropdown, max ID input, start/stop controls, real-time progress bars, provider status display, and scan history.
</objective>

## Key Deliverables

1. **Scan API Endpoints**:
   - `POST /api/scan/start` — start scan with `{chain, maxID}`
   - `POST /api/scan/stop` — stop current scan for a chain
   - `GET /api/scan/status` — current status of all chains
   - `GET /api/scan/sse` — SSE stream for real-time progress
2. **SSE Stream Handler** — EventSource-compatible endpoint, keepalive, reconnection support
3. **Frontend Scan Page** — matching `.project/03-mockups/screens/scan.html`
4. **Scan Control Panel** — chain dropdown, max ID input, start/stop buttons, info alert with estimate
5. **Active & Recent Scans** — per-chain scan cards with progress bars, status badges, stats
6. **Provider Status** — grid showing provider health indicators
7. **Scan Store** — Svelte store managing scan state, SSE connection, progress updates

## Files to Create/Modify

- `internal/api/handlers/scan.go` — scan endpoints
- `internal/api/handlers/sse.go` — SSE endpoint
- `web/src/routes/scan/+page.svelte`
- `web/src/lib/components/scan/ScanControl.svelte`
- `web/src/lib/components/scan/ScanProgress.svelte`
- `web/src/lib/stores/scan.ts`

## Reference Mockup
- `.project/03-mockups/screens/scan.html`

<research_needed>
- SvelteKit EventSource integration patterns — auto-reconnect handling in stores
</research_needed>
