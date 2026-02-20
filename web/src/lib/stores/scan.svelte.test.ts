import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the API functions.
const mockGetScanStatus = vi.fn();
const mockStartScan = vi.fn();
const mockStopScan = vi.fn();

vi.mock('$lib/utils/api', () => ({
	getScanStatus: (...args: unknown[]) => mockGetScanStatus(...args),
	startScan: (...args: unknown[]) => mockStartScan(...args),
	stopScan: (...args: unknown[]) => mockStopScan(...args)
}));

// Mock EventSource as a proper class.
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

import { scanStore } from './scan.svelte';

beforeEach(() => {
	mockGetScanStatus.mockReset();
	mockStartScan.mockReset();
	mockStopScan.mockReset();
	scanStore.disconnectSSE();
});

describe('scanStore - initial state', () => {
	it('starts with empty statuses', () => {
		expect(scanStore.state.statuses).toEqual({});
	});

	it('starts disconnected', () => {
		expect(scanStore.state.sseStatus).toBe('disconnected');
	});

	it('is not loading initially', () => {
		expect(scanStore.state.loading).toBe(false);
	});
});

describe('scanStore - fetchStatus', () => {
	it('fetches and stores scan status', async () => {
		const mockStatuses = {
			BTC: {
				chain: 'BTC',
				lastScannedIndex: 100,
				maxScanId: 5000,
				status: 'completed',
				startedAt: null,
				updatedAt: null,
				isRunning: false,
				fundedCount: 3
			},
			BSC: null,
			SOL: null
		};
		mockGetScanStatus.mockResolvedValueOnce({ data: mockStatuses });

		await scanStore.fetchStatus();

		expect(scanStore.state.statuses).toEqual(mockStatuses);
		expect(scanStore.state.loading).toBe(false);
	});

	it('sets error on failure', async () => {
		mockGetScanStatus.mockRejectedValueOnce(new Error('Fetch failed'));

		await scanStore.fetchStatus();

		expect(scanStore.state.error).toBe('Fetch failed');
		expect(scanStore.state.loading).toBe(false);
	});
});

describe('scanStore - startScan', () => {
	it('calls API and refreshes status', async () => {
		mockStartScan.mockResolvedValueOnce({ data: { message: 'ok' } });
		mockGetScanStatus.mockResolvedValueOnce({ data: {} });

		await scanStore.startScan('BTC', 5000);

		expect(mockStartScan).toHaveBeenCalledWith('BTC', 5000);
		expect(mockGetScanStatus).toHaveBeenCalled();
	});

	it('sets error on failure', async () => {
		mockStartScan.mockRejectedValueOnce(new Error('Scan start failed'));

		await expect(scanStore.startScan('BTC', 5000)).rejects.toThrow('Scan start failed');
		expect(scanStore.state.error).toBe('Scan start failed');
	});
});

describe('scanStore - stopScan', () => {
	it('calls API and refreshes status', async () => {
		mockStopScan.mockResolvedValueOnce({ data: { message: 'ok' } });
		mockGetScanStatus.mockResolvedValueOnce({ data: {} });

		await scanStore.stopScan('BSC');

		expect(mockStopScan).toHaveBeenCalledWith('BSC');
		expect(mockGetScanStatus).toHaveBeenCalled();
	});

	it('sets error on failure', async () => {
		mockStopScan.mockRejectedValueOnce(new Error('Stop failed'));

		await scanStore.stopScan('SOL');

		expect(scanStore.state.error).toBe('Stop failed');
	});
});

describe('scanStore - isAnyScanning', () => {
	it('returns false when no chains are scanning', () => {
		expect(scanStore.isAnyScanning()).toBe(false);
	});

	it('returns true when a chain is running', async () => {
		const mockStatuses = {
			BTC: {
				chain: 'BTC' as const,
				lastScannedIndex: 50,
				maxScanId: 5000,
				status: 'scanning' as const,
				startedAt: null,
				updatedAt: null,
				isRunning: true,
				fundedCount: 0
			}
		};
		mockGetScanStatus.mockResolvedValueOnce({ data: mockStatuses });
		await scanStore.fetchStatus();

		expect(scanStore.isAnyScanning()).toBe(true);
	});
});

describe('scanStore - SSE connection', () => {
	it('sets status to connecting when connectSSE is called', () => {
		scanStore.connectSSE();
		expect(scanStore.state.sseStatus).toBe('connecting');
	});

	it('disconnectSSE sets status to disconnected', () => {
		scanStore.connectSSE();
		scanStore.disconnectSSE();
		expect(scanStore.state.sseStatus).toBe('disconnected');
	});
});
