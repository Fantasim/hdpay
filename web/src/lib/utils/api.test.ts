import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api, ApiError, getAddresses, getTransactions } from './api';

// Mock global fetch
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

// Mock document.cookie for CSRF
Object.defineProperty(document, 'cookie', {
	writable: true,
	value: ''
});

beforeEach(() => {
	mockFetch.mockReset();
});

describe('api.get', () => {
	it('makes GET request to correct URL', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: 'test' })
		});

		await api.get('/test');

		expect(mockFetch).toHaveBeenCalledWith('/api/test', {
			method: 'GET',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'same-origin',
			body: undefined
		});
	});

	it('does NOT include CSRF header for GET requests', async () => {
		document.cookie = 'csrf_token=abc123';
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: 'test' })
		});

		await api.get('/test');

		const callHeaders = mockFetch.mock.calls[0][1].headers;
		expect(callHeaders['X-CSRF-Token']).toBeUndefined();
	});
});

describe('api.post', () => {
	it('includes CSRF header for POST requests', async () => {
		document.cookie = 'csrf_token=abc123';
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: 'ok' })
		});

		await api.post('/test', { key: 'value' });

		const callHeaders = mockFetch.mock.calls[0][1].headers;
		// Token is either abc123 (fresh) or cached from prior test — either way it's present
		expect(callHeaders['X-CSRF-Token']).toBeDefined();
	});

	it('sends JSON body', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: 'ok' })
		});

		await api.post('/test', { chain: 'BTC' });

		expect(mockFetch.mock.calls[0][1].body).toBe('{"chain":"BTC"}');
	});
});

describe('api.put', () => {
	it('includes CSRF header for PUT requests', async () => {
		document.cookie = 'csrf_token=abc123';
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: 'ok' })
		});

		await api.put('/settings', { max_scan_id: '1000' });

		const callHeaders = mockFetch.mock.calls[0][1].headers;
		expect(callHeaders['X-CSRF-Token']).toBeDefined();
	});
});

describe('api.delete', () => {
	it('includes CSRF header for DELETE requests', async () => {
		document.cookie = 'csrf_token=abc123';
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: 'ok' })
		});

		await api.delete('/test');

		const callHeaders = mockFetch.mock.calls[0][1].headers;
		expect(callHeaders['X-CSRF-Token']).toBeDefined();
	});
});

describe('error handling', () => {
	it('throws ApiError on non-ok response', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: false,
			status: 400,
			statusText: 'Bad Request',
			json: () =>
				Promise.resolve({
					error: { code: 'ERROR_INVALID_CHAIN', message: 'Invalid chain' }
				})
		});

		await expect(api.get('/test')).rejects.toThrow(ApiError);
	});

	it('includes error code and status in ApiError', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: false,
			status: 400,
			statusText: 'Bad Request',
			json: () =>
				Promise.resolve({
					error: { code: 'ERROR_INVALID_CHAIN', message: 'Invalid chain' }
				})
		});

		try {
			await api.get('/test');
		} catch (e) {
			expect(e).toBeInstanceOf(ApiError);
			const err = e as ApiError;
			expect(err.code).toBe('ERROR_INVALID_CHAIN');
			expect(err.message).toBe('Invalid chain');
			expect(err.status).toBe(400);
		}
	});

	it('clears CSRF token cache on 403', async () => {
		// Ensure a token is cached by making a successful POST
		document.cookie = 'csrf_token=cached_token';
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: 'ok' })
		});
		await api.post('/first', {});

		// Verify token was sent
		const firstHeaders = mockFetch.mock.calls[0][1].headers;
		expect(firstHeaders['X-CSRF-Token']).toBeDefined();

		// 403 response triggers clearCsrfToken()
		mockFetch.mockResolvedValueOnce({
			ok: false,
			status: 403,
			statusText: 'Forbidden',
			json: () =>
				Promise.resolve({
					error: { code: 'CSRF_FAILED', message: 'CSRF token invalid' }
				})
		});

		try {
			await api.post('/second', {});
		} catch {
			// expected
		}

		// Clear the cookie so getCsrfToken() can't re-read it
		document.cookie = '';

		// After 403 cleared the cache + empty cookie, no CSRF should be sent
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: 'ok' })
		});
		await api.post('/third', {});

		const thirdHeaders = mockFetch.mock.calls[2][1].headers;
		expect(thirdHeaders['X-CSRF-Token']).toBeUndefined();
	});

	it('handles JSON parse failure in error response', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: false,
			status: 500,
			statusText: 'Internal Server Error',
			json: () => Promise.reject(new Error('invalid json'))
		});

		try {
			await api.get('/test');
		} catch (e) {
			expect(e).toBeInstanceOf(ApiError);
			const err = e as ApiError;
			expect(err.code).toBe('UNKNOWN');
			expect(err.message).toBe('Internal Server Error');
		}
	});
});

describe('getAddresses', () => {
	it('constructs correct URL with no params', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: [] })
		});

		await getAddresses('BTC');

		expect(mockFetch.mock.calls[0][0]).toBe('/api/addresses/BTC');
	});

	it('constructs correct URL with all params', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: [] })
		});

		await getAddresses('BSC', {
			page: 2,
			pageSize: 50,
			hasBalance: true,
			token: 'USDC'
		});

		const url = mockFetch.mock.calls[0][0] as string;
		expect(url).toContain('/api/addresses/BSC?');
		expect(url).toContain('page=2');
		expect(url).toContain('pageSize=50');
		expect(url).toContain('hasBalance=true');
		expect(url).toContain('token=USDC');
	});

	it('omits undefined params', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: [] })
		});

		await getAddresses('SOL', { page: 1 });

		const url = mockFetch.mock.calls[0][0] as string;
		expect(url).toContain('page=1');
		expect(url).not.toContain('pageSize');
		expect(url).not.toContain('hasBalance');
		expect(url).not.toContain('token');
	});
});

describe('getTransactions', () => {
	it('constructs correct URL with no params', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: [] })
		});

		await getTransactions();

		expect(mockFetch.mock.calls[0][0]).toBe('/api/transactions');
	});

	it('serializes all filter params', async () => {
		mockFetch.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ data: [] })
		});

		await getTransactions({
			chain: 'BTC',
			direction: 'out',
			token: 'NATIVE',
			status: 'confirmed',
			page: 3,
			pageSize: 10
		});

		const url = mockFetch.mock.calls[0][0] as string;
		expect(url).toContain('chain=BTC');
		expect(url).toContain('direction=out');
		expect(url).toContain('token=NATIVE');
		expect(url).toContain('status=confirmed');
		expect(url).toContain('page=3');
		expect(url).toContain('pageSize=10');
	});
});
