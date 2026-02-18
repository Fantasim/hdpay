import { browser } from '$app/environment';
import {
	API_BASE,
	SSE_RECONNECT_DELAY_MS,
	SSE_MAX_RECONNECT_DELAY_MS,
	SSE_BACKOFF_MULTIPLIER,
	SUPPORTED_CHAINS
} from '$lib/constants';
import {
	startScan as apiStartScan,
	stopScan as apiStopScan,
	getScanStatus
} from '$lib/utils/api';
import type {
	Chain,
	ScanCompleteEvent,
	ScanErrorEvent,
	ScanProgress,
	ScanStateWithRunning
} from '$lib/types';

type SSEConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error';

interface ScanStoreState {
	statuses: Record<string, ScanStateWithRunning | null>;
	progress: Record<string, ScanProgress | null>;
	lastComplete: Record<string, ScanCompleteEvent | null>;
	lastError: Record<string, ScanErrorEvent | null>;
	sseStatus: SSEConnectionStatus;
	loading: boolean;
	error: string | null;
}

function createScanStore() {
	let state = $state<ScanStoreState>({
		statuses: {},
		progress: {},
		lastComplete: {},
		lastError: {},
		sseStatus: 'disconnected',
		loading: false,
		error: null
	});

	let eventSource: EventSource | null = null;
	let retryCount = 0;
	let retryTimer: ReturnType<typeof setTimeout> | null = null;

	// Fetch scan status for all chains from the REST API.
	async function fetchStatus(): Promise<void> {
		state.loading = true;
		state.error = null;

		try {
			const response = await getScanStatus();
			state.statuses = response.data ?? {};
		} catch (err) {
			state.error = err instanceof Error ? err.message : 'Failed to fetch scan status';
		} finally {
			state.loading = false;
		}
	}

	// Start a scan for a chain.
	async function startScan(chain: Chain, maxId: number): Promise<void> {
		state.error = null;

		try {
			await apiStartScan(chain, maxId);
			// Refresh status to reflect the new scan.
			await fetchStatus();
		} catch (err) {
			state.error = err instanceof Error ? err.message : 'Failed to start scan';
			throw err;
		}
	}

	// Stop a scan for a chain.
	async function stopScan(chain: Chain): Promise<void> {
		state.error = null;

		try {
			await apiStopScan(chain);
			// Refresh status after stop request.
			await fetchStatus();
		} catch (err) {
			state.error = err instanceof Error ? err.message : 'Failed to stop scan';
		}
	}

	// Connect to the SSE endpoint.
	function connectSSE(): void {
		if (!browser) return;
		if (eventSource) return; // Already connected.

		retryCount = 0;
		openEventSource();
	}

	// Disconnect from the SSE endpoint.
	function disconnectSSE(): void {
		clearRetryTimer();
		closeEventSource();
		state.sseStatus = 'disconnected';
	}

	function openEventSource(): void {
		closeEventSource();

		state.sseStatus = 'connecting';
		const es = new EventSource(`${API_BASE}/scan/sse`);
		eventSource = es;

		es.onopen = () => {
			state.sseStatus = 'connected';
			retryCount = 0;
		};

		// Named event listeners — onmessage does NOT fire for named events.
		es.addEventListener('scan_progress', (e: MessageEvent<string>) => {
			try {
				const data = JSON.parse(e.data) as ScanProgress;
				state.progress = { ...state.progress, [data.chain]: data };
			} catch {
				// Malformed payload — ignore.
			}
		});

		es.addEventListener('scan_complete', (e: MessageEvent<string>) => {
			try {
				const data = JSON.parse(e.data) as ScanCompleteEvent;
				state.lastComplete = { ...state.lastComplete, [data.chain]: data };
				state.progress = { ...state.progress, [data.chain]: null };
				// Refresh status from API to get updated DB state.
				fetchStatus();
			} catch {
				// Malformed payload — ignore.
			}
		});

		es.addEventListener('scan_error', (e: MessageEvent<string>) => {
			try {
				const data = JSON.parse(e.data) as ScanErrorEvent;
				state.lastError = { ...state.lastError, [data.chain]: data };
				state.progress = { ...state.progress, [data.chain]: null };
				// Refresh status from API.
				fetchStatus();
			} catch {
				// Malformed payload — ignore.
			}
		});

		es.onerror = () => {
			// readyState CLOSED (2) = fatal error, browser will NOT auto-retry.
			// readyState CONNECTING (0) = browser IS auto-retrying — do not double-reconnect.
			if (es.readyState === EventSource.CLOSED) {
				state.sseStatus = 'error';
				closeEventSource();
				scheduleReconnect();
			}
		};
	}

	function scheduleReconnect(): void {
		clearRetryTimer();

		const delay = Math.min(
			SSE_RECONNECT_DELAY_MS * Math.pow(SSE_BACKOFF_MULTIPLIER, retryCount),
			SSE_MAX_RECONNECT_DELAY_MS
		);
		retryCount++;

		retryTimer = setTimeout(() => {
			retryTimer = null;
			// Only reconnect if we haven't been explicitly disconnected.
			if (state.sseStatus !== 'disconnected') {
				openEventSource();
			}
		}, delay);
	}

	function clearRetryTimer(): void {
		if (retryTimer !== null) {
			clearTimeout(retryTimer);
			retryTimer = null;
		}
	}

	function closeEventSource(): void {
		if (eventSource !== null) {
			eventSource.onopen = null;
			eventSource.onerror = null;
			eventSource.close();
			eventSource = null;
		}
	}

	// Helper: check if any chain is currently scanning.
	function isAnyScanning(): boolean {
		return SUPPORTED_CHAINS.some((chain) => {
			const status = state.statuses[chain];
			return status?.isRunning === true;
		});
	}

	return {
		get state() {
			return state;
		},
		fetchStatus,
		startScan,
		stopScan,
		connectSSE,
		disconnectSSE,
		isAnyScanning
	};
}

export const scanStore = createScanStore();
