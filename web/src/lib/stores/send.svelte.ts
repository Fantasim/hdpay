import { browser } from '$app/environment';
import {
	API_BASE,
	SSE_RECONNECT_DELAY_MS,
	SSE_MAX_RECONNECT_DELAY_MS,
	SSE_BACKOFF_MULTIPLIER,
	SEND_POLL_INTERVAL_MS
} from '$lib/constants';
import {
	previewSend as apiPreviewSend,
	executeSend as apiExecuteSend,
	gasPreSeed as apiGasPreSeed,
	getSweepStatus as apiGetSweepStatus
} from '$lib/utils/api';
import { validateAddress } from '$lib/utils/validation';
import type {
	Chain,
	GasPreSeedResult,
	SendRequest,
	SendStep,
	SendToken,
	TxResult,
	UnifiedSendPreview,
	UnifiedSendResult
} from '$lib/types';

type SSEConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error';

interface SendStoreState {
	step: SendStep;
	chain: Chain | null;
	token: SendToken | null;
	destination: string;
	destinationError: string | null;
	preview: UnifiedSendPreview | null;
	gasPreSeedResult: GasPreSeedResult | null;
	feePayerIndex: number | null;
	sweepID: string | null;
	executeResult: UnifiedSendResult | null;
	txProgress: TxResult[];
	loading: boolean;
	error: string | null;
	sseStatus: SSEConnectionStatus;
}

const INITIAL_STATE: SendStoreState = {
	step: 'select',
	chain: null,
	token: null,
	destination: '',
	destinationError: null,
	preview: null,
	gasPreSeedResult: null,
	feePayerIndex: null,
	sweepID: null,
	executeResult: null,
	txProgress: [],
	loading: false,
	error: null,
	sseStatus: 'disconnected'
};

function createSendStore() {
	let state = $state<SendStoreState>({ ...INITIAL_STATE });

	let eventSource: EventSource | null = null;
	let retryCount = 0;
	let retryTimer: ReturnType<typeof setTimeout> | null = null;
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	// Set the selected chain.
	function setChain(chain: Chain): void {
		state.chain = chain;
		// Reset token when chain changes (BTC only has NATIVE).
		state.token = null;
		state.error = null;
	}

	// Set the selected token.
	function setToken(token: SendToken): void {
		state.token = token;
		state.error = null;
	}

	// Set and validate the destination address.
	function setDestination(destination: string): void {
		state.destination = destination;

		if (!state.chain || destination.trim().length === 0) {
			state.destinationError = null;
			return;
		}

		state.destinationError = validateAddress(state.chain, destination);
	}

	// Build the SendRequest from the current selection.
	function buildRequest(): SendRequest | null {
		if (!state.chain || !state.token || !state.destination.trim()) {
			return null;
		}
		const req: SendRequest = {
			chain: state.chain,
			token: state.token,
			destination: state.destination.trim()
		};
		if (state.feePayerIndex !== null) {
			req.feePayerIndex = state.feePayerIndex;
		}
		return req;
	}

	// Validate the current selection and fetch preview.
	async function fetchPreview(): Promise<void> {
		const req = buildRequest();
		if (!req) {
			state.error = 'Please select chain, token, and enter a valid destination address.';
			return;
		}

		if (state.destinationError) {
			state.error = state.destinationError;
			return;
		}

		state.loading = true;
		state.error = null;
		state.preview = null;

		try {
			const response = await apiPreviewSend(req);
			state.preview = response.data;
			state.step = 'preview';
		} catch (err) {
			state.error = err instanceof Error ? err.message : 'Failed to fetch send preview';
		} finally {
			state.loading = false;
		}
	}

	// Execute gas pre-seeding for BSC token sweeps.
	async function executeGasPreSeed(sourceIndex: number): Promise<void> {
		if (!state.preview) {
			state.error = 'No preview data available.';
			return;
		}

		const targetAddresses = state.preview.fundedAddresses
			.filter((a) => !a.hasGas)
			.map((a) => a.address);

		if (targetAddresses.length === 0) {
			state.error = 'No addresses need gas pre-seeding.';
			return;
		}

		state.loading = true;
		state.error = null;
		state.gasPreSeedResult = null;

		try {
			const response = await apiGasPreSeed({
				sourceIndex,
				targetAddresses
			});
			state.gasPreSeedResult = response.data;
			state.step = 'execute';
		} catch (err) {
			state.error = err instanceof Error ? err.message : 'Gas pre-seed failed';
		} finally {
			state.loading = false;
		}
	}

	// Execute the sweep transaction(s).
	// Returns 202 immediately — progress is driven by SSE events (tx_status, tx_complete, tx_error).
	// If SSE drops, falls back to polling GET /api/send/sweep/{sweepID}.
	async function executeSweep(): Promise<void> {
		const req = buildRequest();
		if (!req) {
			state.error = 'Invalid send request.';
			return;
		}

		state.loading = true;
		state.error = null;
		state.executeResult = null;
		state.txProgress = [];
		state.sweepID = null;

		// Connect SSE before POST so we catch events immediately.
		connectSSE();

		try {
			const response = await apiExecuteSend(req);
			// 202 Accepted — sweep is running in background.
			state.sweepID = response.data.sweepID;
			// Do NOT set loading=false — SSE events drive completion.
		} catch (err) {
			state.loading = false;
			state.error = err instanceof Error ? err.message : 'Send execution failed';
			disconnectSSE();
			stopPollingFallback();
		}
	}

	// Navigate to a specific step.
	function goToStep(step: SendStep): void {
		state.step = step;
		state.error = null;
	}

	// Go back to the previous step.
	function goBack(): void {
		switch (state.step) {
			case 'preview':
				state.step = 'select';
				break;
			case 'gas-preseed':
				state.step = 'preview';
				state.feePayerIndex = null;
				break;
			case 'execute':
				// If gas pre-seed was used, go back to it; otherwise go to preview.
				state.step = state.preview?.needsGasPreSeed ? 'gas-preseed' : 'preview';
				break;
			case 'complete':
				// No going back from complete — reset instead.
				break;
		}
		state.error = null;
	}

	// Advance from preview step based on whether gas pre-seed is needed.
	function advanceFromPreview(): void {
		if (state.preview?.needsGasPreSeed) {
			state.step = 'gas-preseed';
		} else {
			state.step = 'execute';
		}
		state.error = null;
	}

	// Skip gas pre-seed and go directly to execute.
	function skipGasPreSeed(): void {
		state.step = 'execute';
		state.error = null;
	}

	// Confirm SOL fee payer index and advance to execute step.
	// Unlike BSC gas pre-seed, this requires no API call — the fee payer
	// index is passed to the execute request instead.
	function confirmFeePayerIndex(index: number): void {
		state.feePayerIndex = index;
		state.step = 'execute';
		state.error = null;
	}

	// Reset the store to initial state.
	function reset(): void {
		disconnectSSE();
		stopPollingFallback();
		Object.assign(state, { ...INITIAL_STATE });
	}

	// --- SSE for real-time TX status updates ---

	function connectSSE(): void {
		if (!browser) return;
		if (eventSource) return;

		retryCount = 0;
		openEventSource();
	}

	function disconnectSSE(): void {
		clearRetryTimer();
		closeEventSource();
		state.sseStatus = 'disconnected';
	}

	function openEventSource(): void {
		closeEventSource();

		state.sseStatus = 'connecting';
		const es = new EventSource(`${API_BASE}/send/sse`);
		eventSource = es;

		es.onopen = () => {
			state.sseStatus = 'connected';
			retryCount = 0;
			// SSE is connected — stop polling fallback if active.
			stopPollingFallback();
		};

		es.addEventListener('tx_status', (e: MessageEvent<string>) => {
			try {
				const data = JSON.parse(e.data) as TxResult;
				// Update or add the TX result.
				const idx = state.txProgress.findIndex(
					(t) => t.addressIndex === data.addressIndex
				);
				if (idx >= 0) {
					state.txProgress = [
						...state.txProgress.slice(0, idx),
						data,
						...state.txProgress.slice(idx + 1)
					];
				} else {
					state.txProgress = [...state.txProgress, data];
				}
			} catch {
				// Malformed payload — ignore.
			}
		});

		es.addEventListener('tx_complete', (e: MessageEvent<string>) => {
			try {
				const data = JSON.parse(e.data) as {
					chain: Chain;
					token: string;
					successCount: number;
					failCount: number;
					totalSwept: string;
					txResults?: TxResult[];
				};
				state.executeResult = {
					chain: data.chain,
					token: (data.token as SendToken) ?? state.token ?? 'NATIVE',
					txResults: data.txResults ?? [...state.txProgress],
					successCount: data.successCount,
					failCount: data.failCount,
					totalSwept: data.totalSwept
				};
				state.step = 'complete';
				state.loading = false;
				disconnectSSE();
				stopPollingFallback();
			} catch {
				// Malformed payload — ignore.
			}
		});

		es.addEventListener('tx_error', (e: MessageEvent<string>) => {
			try {
				const data = JSON.parse(e.data) as { error: string; message: string };
				state.error = data.message || data.error;
				state.loading = false;
				disconnectSSE();
				stopPollingFallback();
			} catch {
				// Malformed payload — ignore.
			}
		});

		es.onerror = () => {
			if (es.readyState === EventSource.CLOSED) {
				state.sseStatus = 'error';
				closeEventSource();
				scheduleReconnect();
				// If we're actively waiting for a sweep, start polling fallback.
				if (state.loading && state.sweepID) {
					startPollingFallback();
				}
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

	// --- Polling fallback when SSE disconnects during active sweep ---

	function startPollingFallback(): void {
		if (pollTimer || !state.sweepID) return;

		pollTimer = setInterval(async () => {
			if (!state.sweepID || state.step === 'complete') {
				stopPollingFallback();
				return;
			}

			try {
				const res = await apiGetSweepStatus(state.sweepID);
				const txStates = res.data;

				// Update txProgress from DB state.
				state.txProgress = txStates.map((s) => ({
					addressIndex: s.addressIndex,
					fromAddress: s.fromAddress,
					txHash: s.txHash,
					amount: s.amount,
					status: s.status,
					error: s.error
				}));

				// Check if all are terminal -> build executeResult, set complete.
				const terminalStatuses = ['confirmed', 'failed', 'uncertain', 'success'];
				const allTerminal =
					txStates.length > 0 &&
					txStates.every((s) => terminalStatuses.includes(s.status));

				if (allTerminal) {
					const successCount = txStates.filter(
						(s) => s.status === 'success' || s.status === 'confirmed'
					).length;

					state.executeResult = {
						chain: state.chain ?? 'BTC',
						token: state.token ?? 'NATIVE',
						txResults: state.txProgress,
						successCount,
						failCount: txStates.length - successCount,
						totalSwept: ''
					};
					state.step = 'complete';
					state.loading = false;
					stopPollingFallback();
					disconnectSSE();
				}
			} catch {
				// Polling error — ignore, will retry next interval.
			}
		}, SEND_POLL_INTERVAL_MS);
	}

	function stopPollingFallback(): void {
		if (pollTimer) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	return {
		get state() {
			return state;
		},
		setChain,
		setToken,
		setDestination,
		fetchPreview,
		executeGasPreSeed,
		executeSweep,
		goToStep,
		goBack,
		advanceFromPreview,
		skipGasPreSeed,
		confirmFeePayerIndex,
		reset
	};
}

export const sendStore = createSendStore();
