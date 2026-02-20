import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the API function.
const mockGetAddresses = vi.fn();
vi.mock('$lib/utils/api', () => ({
	getAddresses: (...args: unknown[]) => mockGetAddresses(...args)
}));

import { addressStore } from './addresses.svelte';

beforeEach(() => {
	mockGetAddresses.mockReset();
	// Default: return empty data
	mockGetAddresses.mockResolvedValue({ data: [], meta: null });
});

describe('addressStore - initial state', () => {
	it('starts with BTC chain', () => {
		expect(addressStore.state.chain).toBe('BTC');
	});

	it('starts with page 1', () => {
		expect(addressStore.state.page).toBe(1);
	});

	it('starts with no balance filter', () => {
		expect(addressStore.state.hasBalance).toBe(false);
	});

	it('starts with empty token filter', () => {
		expect(addressStore.state.token).toBe('');
	});
});

describe('addressStore - fetchAddresses', () => {
	it('calls getAddresses with current state params', async () => {
		mockGetAddresses.mockResolvedValueOnce({
			data: [{ chain: 'BTC', addressIndex: 0, address: 'bc1q...', nativeBalance: '0', tokenBalances: [], lastScanned: null }],
			meta: { page: 1, pageSize: 100, total: 1 }
		});

		await addressStore.fetchAddresses();

		expect(mockGetAddresses).toHaveBeenCalledWith('BTC', {
			page: 1,
			pageSize: 100
		});
	});

	it('includes hasBalance param when filter is active', async () => {
		addressStore.setFilter({ hasBalance: true });
		// Wait for the implicit fetchAddresses call
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		const lastCall = mockGetAddresses.mock.calls[mockGetAddresses.mock.calls.length - 1];
		expect(lastCall[1]).toMatchObject({ hasBalance: true });
	});

	it('includes token param when filter is set', async () => {
		mockGetAddresses.mockReset();
		mockGetAddresses.mockResolvedValue({ data: [], meta: null });
		addressStore.setFilter({ token: 'USDC' });
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		const lastCall = mockGetAddresses.mock.calls[mockGetAddresses.mock.calls.length - 1];
		expect(lastCall[1]).toMatchObject({ token: 'USDC' });
	});

	it('stores response data', async () => {
		const mockData = [
			{
				chain: 'BTC' as const,
				addressIndex: 0,
				address: 'bc1qtest',
				nativeBalance: '100000',
				tokenBalances: [],
				lastScanned: '2026-02-18T10:00:00Z'
			}
		];
		mockGetAddresses.mockResolvedValueOnce({
			data: mockData,
			meta: { page: 1, pageSize: 100, total: 1 }
		});

		await addressStore.fetchAddresses();

		expect(addressStore.state.addresses).toEqual(mockData);
		expect(addressStore.state.meta?.total).toBe(1);
	});

	it('sets error on API failure', async () => {
		mockGetAddresses.mockRejectedValueOnce(new Error('Network error'));

		await addressStore.fetchAddresses();

		expect(addressStore.state.error).toBe('Network error');
		expect(addressStore.state.addresses).toEqual([]);
	});
});

describe('addressStore - setChain', () => {
	it('resets page to 1 when changing chain', async () => {
		// Set to a later page first
		addressStore.setPage(3);
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		mockGetAddresses.mockReset();
		mockGetAddresses.mockResolvedValue({ data: [], meta: null });

		addressStore.setChain('SOL');
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		expect(addressStore.state.chain).toBe('SOL');
		expect(addressStore.state.page).toBe(1);
	});

	it('triggers fetchAddresses', async () => {
		mockGetAddresses.mockReset();
		mockGetAddresses.mockResolvedValue({ data: [], meta: null });

		addressStore.setChain('BSC');
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		expect(mockGetAddresses).toHaveBeenCalledWith('BSC', expect.any(Object));
	});
});

describe('addressStore - setPage', () => {
	it('updates page and triggers fetch', async () => {
		mockGetAddresses.mockReset();
		mockGetAddresses.mockResolvedValue({ data: [], meta: null });

		addressStore.setPage(5);
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		expect(addressStore.state.page).toBe(5);
	});
});

describe('addressStore - setFilter', () => {
	it('resets page to 1 when changing filter', async () => {
		addressStore.setPage(3);
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		mockGetAddresses.mockReset();
		mockGetAddresses.mockResolvedValue({ data: [], meta: null });

		addressStore.setFilter({ hasBalance: true });
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		expect(addressStore.state.page).toBe(1);
	});

	it('can set hasBalance filter', async () => {
		mockGetAddresses.mockReset();
		mockGetAddresses.mockResolvedValue({ data: [], meta: null });

		addressStore.setFilter({ hasBalance: true });
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		expect(addressStore.state.hasBalance).toBe(true);
	});

	it('can set token filter', async () => {
		mockGetAddresses.mockReset();
		mockGetAddresses.mockResolvedValue({ data: [], meta: null });

		addressStore.setFilter({ token: 'USDT' });
		await vi.waitFor(() => expect(mockGetAddresses).toHaveBeenCalled());

		expect(addressStore.state.token).toBe('USDT');
	});
});
