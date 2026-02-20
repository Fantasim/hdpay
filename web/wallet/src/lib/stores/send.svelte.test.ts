import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the API functions before importing the store.
vi.mock('$lib/utils/api', () => ({
	previewSend: vi.fn(),
	executeSend: vi.fn(),
	gasPreSeed: vi.fn(),
	getSweepStatus: vi.fn()
}));

// Mock EventSource globally as a proper class.
class MockEventSource {
	static readonly CONNECTING = 0;
	static readonly OPEN = 1;
	static readonly CLOSED = 2;

	onopen: ((ev: Event) => void) | null = null;
	onerror: ((ev: Event) => void) | null = null;
	readyState = MockEventSource.OPEN;

	close = vi.fn();
	addEventListener = vi.fn();

	constructor(public url: string) {}
}
vi.stubGlobal('EventSource', MockEventSource);

import { sendStore } from './send.svelte';
import { previewSend, executeSend } from '$lib/utils/api';
import type { UnifiedSendPreview } from '$lib/types';

beforeEach(() => {
	sendStore.reset();
	vi.clearAllMocks();
});

describe('sendStore - setChain', () => {
	it('sets the chain and resets token', () => {
		sendStore.setToken('USDC');
		sendStore.setChain('BTC');
		expect(sendStore.state.chain).toBe('BTC');
		expect(sendStore.state.token).toBeNull();
	});

	it('clears error when setting chain', () => {
		// Trigger an error state
		sendStore.setChain('BSC');
		expect(sendStore.state.error).toBeNull();
	});
});

describe('sendStore - setToken', () => {
	it('sets the token', () => {
		sendStore.setToken('USDC');
		expect(sendStore.state.token).toBe('USDC');
	});

	it('clears error when setting token', () => {
		sendStore.setToken('NATIVE');
		expect(sendStore.state.error).toBeNull();
	});
});

describe('sendStore - setDestination', () => {
	it('clears error for empty destination', () => {
		sendStore.setChain('BTC');
		sendStore.setDestination('');
		expect(sendStore.state.destinationError).toBeNull();
	});

	it('clears error when chain is not set', () => {
		sendStore.setDestination('bc1qtest');
		expect(sendStore.state.destinationError).toBeNull();
	});

	it('validates BTC address format', () => {
		sendStore.setChain('BTC');
		sendStore.setDestination('invalid_address');
		expect(sendStore.state.destinationError).not.toBeNull();
	});

	it('accepts valid BTC address', () => {
		sendStore.setChain('BTC');
		sendStore.setDestination('bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu');
		expect(sendStore.state.destinationError).toBeNull();
	});

	it('validates BSC address format', () => {
		sendStore.setChain('BSC');
		sendStore.setDestination('not-an-address');
		expect(sendStore.state.destinationError).not.toBeNull();
	});

	it('accepts valid BSC address', () => {
		sendStore.setChain('BSC');
		sendStore.setDestination('0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb');
		expect(sendStore.state.destinationError).toBeNull();
	});

	it('validates SOL address format', () => {
		sendStore.setChain('SOL');
		sendStore.setDestination('bad');
		expect(sendStore.state.destinationError).not.toBeNull();
	});

	it('accepts valid SOL address', () => {
		sendStore.setChain('SOL');
		sendStore.setDestination('3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx');
		expect(sendStore.state.destinationError).toBeNull();
	});
});

describe('sendStore - buildRequest (via fetchPreview)', () => {
	it('returns error when required fields are missing', async () => {
		// No chain, token, or destination
		await sendStore.fetchPreview();
		expect(sendStore.state.error).toBe(
			'Please select chain, token, and enter a valid destination address.'
		);
	});

	it('returns error when destination has validation error', async () => {
		sendStore.setChain('BTC');
		sendStore.setToken('NATIVE');
		sendStore.setDestination('invalid_btc_addr');
		await sendStore.fetchPreview();
		expect(sendStore.state.error).not.toBeNull();
		expect(sendStore.state.error).toContain('Invalid BTC address');
	});

	it('calls previewSend with correct request', async () => {
		const mockPreview: UnifiedSendPreview = {
			chain: 'BTC',
			token: 'NATIVE',
			destination: 'bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu',
			fundedCount: 1,
			totalAmount: '100000',
			feeEstimate: '1000',
			netAmount: '99000',
			txCount: 1,
			needsGasPreSeed: false,
			gasPreSeedCount: 0,
			fundedAddresses: []
		};
		vi.mocked(previewSend).mockResolvedValueOnce({ data: mockPreview });

		sendStore.setChain('BTC');
		sendStore.setToken('NATIVE');
		sendStore.setDestination('bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu');
		await sendStore.fetchPreview();

		expect(previewSend).toHaveBeenCalledWith({
			chain: 'BTC',
			token: 'NATIVE',
			destination: 'bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu'
		});
		expect(sendStore.state.step).toBe('preview');
		expect(sendStore.state.preview).toEqual(mockPreview);
	});

	it('includes feePayerIndex when set', async () => {
		const mockPreview: UnifiedSendPreview = {
			chain: 'SOL',
			token: 'USDC',
			destination: '3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx',
			fundedCount: 1,
			totalAmount: '20000000',
			feeEstimate: '5000',
			netAmount: '19995000',
			txCount: 1,
			needsGasPreSeed: true,
			gasPreSeedCount: 1,
			fundedAddresses: []
		};
		vi.mocked(previewSend).mockResolvedValueOnce({ data: mockPreview });

		sendStore.setChain('SOL');
		sendStore.setToken('USDC');
		sendStore.setDestination('3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx');
		sendStore.confirmFeePayerIndex(5);
		await sendStore.fetchPreview();

		expect(previewSend).toHaveBeenCalledWith({
			chain: 'SOL',
			token: 'USDC',
			destination: '3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx',
			feePayerIndex: 5
		});
	});
});

describe('sendStore - goBack', () => {
	it('goes from preview to select', () => {
		sendStore.goToStep('preview');
		sendStore.goBack();
		expect(sendStore.state.step).toBe('select');
	});

	it('goes from gas-preseed to preview and resets feePayerIndex', () => {
		sendStore.confirmFeePayerIndex(3);
		sendStore.goToStep('gas-preseed');
		sendStore.goBack();
		expect(sendStore.state.step).toBe('preview');
		expect(sendStore.state.feePayerIndex).toBeNull();
	});

	it('goes from execute to gas-preseed when needsGasPreSeed', async () => {
		// Set up preview with needsGasPreSeed
		const mockPreview: UnifiedSendPreview = {
			chain: 'BSC',
			token: 'USDC',
			destination: '0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb',
			fundedCount: 1,
			totalAmount: '100',
			feeEstimate: '10',
			netAmount: '90',
			txCount: 1,
			needsGasPreSeed: true,
			gasPreSeedCount: 1,
			fundedAddresses: []
		};
		vi.mocked(previewSend).mockResolvedValueOnce({ data: mockPreview });

		sendStore.setChain('BSC');
		sendStore.setToken('USDC');
		sendStore.setDestination('0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb');
		await sendStore.fetchPreview();

		sendStore.goToStep('execute');
		sendStore.goBack();
		expect(sendStore.state.step).toBe('gas-preseed');
	});

	it('goes from execute to preview when no gas pre-seed', async () => {
		const mockPreview: UnifiedSendPreview = {
			chain: 'BTC',
			token: 'NATIVE',
			destination: 'bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu',
			fundedCount: 1,
			totalAmount: '100',
			feeEstimate: '10',
			netAmount: '90',
			txCount: 1,
			needsGasPreSeed: false,
			gasPreSeedCount: 0,
			fundedAddresses: []
		};
		vi.mocked(previewSend).mockResolvedValueOnce({ data: mockPreview });

		sendStore.setChain('BTC');
		sendStore.setToken('NATIVE');
		sendStore.setDestination('bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu');
		await sendStore.fetchPreview();

		sendStore.goToStep('execute');
		sendStore.goBack();
		expect(sendStore.state.step).toBe('preview');
	});

	it('does not go back from complete', () => {
		sendStore.goToStep('complete');
		sendStore.goBack();
		expect(sendStore.state.step).toBe('complete');
	});
});

describe('sendStore - advanceFromPreview', () => {
	it('goes to gas-preseed when preview.needsGasPreSeed is true', async () => {
		const mockPreview: UnifiedSendPreview = {
			chain: 'BSC',
			token: 'USDC',
			destination: '0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb',
			fundedCount: 1,
			totalAmount: '100',
			feeEstimate: '10',
			netAmount: '90',
			txCount: 1,
			needsGasPreSeed: true,
			gasPreSeedCount: 3,
			fundedAddresses: []
		};
		vi.mocked(previewSend).mockResolvedValueOnce({ data: mockPreview });

		sendStore.setChain('BSC');
		sendStore.setToken('USDC');
		sendStore.setDestination('0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb');
		await sendStore.fetchPreview();

		sendStore.advanceFromPreview();
		expect(sendStore.state.step).toBe('gas-preseed');
	});

	it('goes to execute when no gas pre-seed needed', async () => {
		const mockPreview: UnifiedSendPreview = {
			chain: 'BTC',
			token: 'NATIVE',
			destination: 'bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu',
			fundedCount: 1,
			totalAmount: '100',
			feeEstimate: '10',
			netAmount: '90',
			txCount: 1,
			needsGasPreSeed: false,
			gasPreSeedCount: 0,
			fundedAddresses: []
		};
		vi.mocked(previewSend).mockResolvedValueOnce({ data: mockPreview });

		sendStore.setChain('BTC');
		sendStore.setToken('NATIVE');
		sendStore.setDestination('bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu');
		await sendStore.fetchPreview();

		sendStore.advanceFromPreview();
		expect(sendStore.state.step).toBe('execute');
	});
});

describe('sendStore - confirmFeePayerIndex', () => {
	it('sets feePayerIndex and advances to execute', () => {
		sendStore.confirmFeePayerIndex(7);
		expect(sendStore.state.feePayerIndex).toBe(7);
		expect(sendStore.state.step).toBe('execute');
	});
});

describe('sendStore - skipGasPreSeed', () => {
	it('goes to execute step', () => {
		sendStore.goToStep('gas-preseed');
		sendStore.skipGasPreSeed();
		expect(sendStore.state.step).toBe('execute');
	});
});

describe('sendStore - reset', () => {
	it('returns to initial state', () => {
		sendStore.setChain('BTC');
		sendStore.setToken('NATIVE');
		sendStore.setDestination('bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu');
		sendStore.goToStep('preview');

		sendStore.reset();

		expect(sendStore.state.step).toBe('select');
		expect(sendStore.state.chain).toBeNull();
		expect(sendStore.state.token).toBeNull();
		expect(sendStore.state.destination).toBe('');
		expect(sendStore.state.loading).toBe(false);
		expect(sendStore.state.error).toBeNull();
	});
});

describe('sendStore - executeSweep error', () => {
	it('sets error on API failure', async () => {
		sendStore.setChain('BTC');
		sendStore.setToken('NATIVE');
		sendStore.setDestination('bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu');

		vi.mocked(executeSend).mockRejectedValueOnce(new Error('Network error'));

		await sendStore.executeSweep();

		expect(sendStore.state.error).toBe('Network error');
		expect(sendStore.state.loading).toBe(false);
	});
});
