import { browser } from '$app/environment';
import { API_BASE, SSE_RECONNECT_DELAY_MS, SSE_MAX_RECONNECT_DELAY_MS, SSE_BACKOFF_MULTIPLIER } from '$lib/constants';
import {
	previewSend as apiPreviewSend,
	executeSend as apiExecuteSend,
	gasPreSeed as apiGasPreSeed
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

		// Connect SSE before execution for real-time updates.
		connectSSE();

		try {
			const response = await apiExecuteSend(req);
			state.executeResult = response.data;
			state.step = 'complete';
		} catch (err) {
			state.error = err instanceof Error ? err.message : 'Send execution failed';
		} finally {
			state.loading = false;
			disconnectSSE();
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
		Object.assign(state, { ...INITIAL_STATE });
	}

	// SSE for real-time TX status updates.
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
				const data = JSON.parse(e.data) as UnifiedSendResult;
				state.executeResult = data;
				state.step = 'complete';
			} catch {
				// Malformed payload — ignore.
			}
		});

		es.addEventListener('tx_error', (e: MessageEvent<string>) => {
			try {
				const data = JSON.parse(e.data) as { error: string; message: string };
				state.error = data.message || data.error;
			} catch {
				// Malformed payload — ignore.
			}
		});

		es.onerror = () => {
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
