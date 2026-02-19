import { API_BASE } from '$lib/constants';
import type {
	AddressWithBalance, APIErrorResponse, APIResponse, Chain,
	GasPreSeedRequest, GasPreSeedResult,
	PortfolioResponse, PriceResponse, ProviderHealthMap, ScanStateWithRunning,
	SendRequest, Settings, SweepStarted, Transaction, TransactionListParams, TxResult,
	UnifiedSendPreview
} from '$lib/types';

let csrfToken: string | null = null;

function getCsrfToken(): string | null {
	if (csrfToken) return csrfToken;

	const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/);
	if (match) {
		csrfToken = match[1];
	}
	return csrfToken;
}

function clearCsrfToken(): void {
	csrfToken = null;
}

async function request<T>(
	method: string,
	path: string,
	body?: unknown
): Promise<APIResponse<T>> {
	const url = `${API_BASE}${path}`;
	const headers: Record<string, string> = {
		'Content-Type': 'application/json'
	};

	// Add CSRF token for mutating requests
	if (method !== 'GET' && method !== 'HEAD') {
		const token = getCsrfToken();
		if (token) {
			headers['X-CSRF-Token'] = token;
		}
	}

	const res = await fetch(url, {
		method,
		headers,
		credentials: 'same-origin',
		body: body ? JSON.stringify(body) : undefined
	});

	if (!res.ok) {
		// Refresh CSRF token on 403
		if (res.status === 403) {
			clearCsrfToken();
		}

		const errorBody = (await res.json().catch(() => ({
			error: { code: 'UNKNOWN', message: res.statusText }
		}))) as APIErrorResponse;

		throw new ApiError(
			errorBody.error.code,
			errorBody.error.message,
			res.status
		);
	}

	return (await res.json()) as APIResponse<T>;
}

export class ApiError extends Error {
	constructor(
		public readonly code: string,
		message: string,
		public readonly status: number
	) {
		super(message);
		this.name = 'ApiError';
	}
}

// API client — single source of truth for all backend calls
export const api = {
	get<T>(path: string): Promise<APIResponse<T>> {
		return request<T>('GET', path);
	},

	post<T>(path: string, body?: unknown): Promise<APIResponse<T>> {
		return request<T>('POST', path, body);
	},

	put<T>(path: string, body?: unknown): Promise<APIResponse<T>> {
		return request<T>('PUT', path, body);
	},

	delete<T>(path: string): Promise<APIResponse<T>> {
		return request<T>('DELETE', path);
	}
};

// Address API

export interface AddressListParams {
	page?: number;
	pageSize?: number;
	hasBalance?: boolean;
	token?: string;
}

export function getAddresses(
	chain: Chain,
	params: AddressListParams = {}
): Promise<APIResponse<AddressWithBalance[]>> {
	const searchParams = new URLSearchParams();
	if (params.page !== undefined) searchParams.set('page', String(params.page));
	if (params.pageSize !== undefined) searchParams.set('pageSize', String(params.pageSize));
	if (params.hasBalance) searchParams.set('hasBalance', 'true');
	if (params.token) searchParams.set('token', params.token);

	const qs = searchParams.toString();
	const path = `/addresses/${chain}${qs ? '?' + qs : ''}`;
	return api.get<AddressWithBalance[]>(path);
}

export function exportAddresses(chain: Chain): void {
	const url = `${API_BASE}/addresses/${chain}/export`;
	window.open(url, '_blank');
}

// Scan API

export interface StartScanResponse {
	message: string;
	chain: Chain;
	maxId: number;
}

export interface StopScanResponse {
	message: string;
	chain: Chain;
}

export function startScan(chain: Chain, maxId: number): Promise<APIResponse<StartScanResponse>> {
	return api.post<StartScanResponse>('/scan/start', { chain, maxId });
}

export function stopScan(chain: Chain): Promise<APIResponse<StopScanResponse>> {
	return api.post<StopScanResponse>('/scan/stop', { chain });
}

export function getScanStatus(): Promise<APIResponse<Record<string, ScanStateWithRunning | null>>> {
	return api.get<Record<string, ScanStateWithRunning | null>>('/scan/status');
}

export function getScanStatusForChain(chain: Chain): Promise<APIResponse<ScanStateWithRunning>> {
	return api.get<ScanStateWithRunning>(`/scan/status?chain=${chain}`);
}

// Dashboard API

export function getPrices(): Promise<APIResponse<PriceResponse>> {
	return api.get<PriceResponse>('/dashboard/prices');
}

export function getPortfolio(): Promise<APIResponse<PortfolioResponse>> {
	return api.get<PortfolioResponse>('/dashboard/portfolio');
}

// Send API

export function previewSend(req: SendRequest): Promise<APIResponse<UnifiedSendPreview>> {
	return api.post<UnifiedSendPreview>('/send/preview', req);
}

export function executeSend(req: SendRequest): Promise<APIResponse<SweepStarted>> {
	return api.post<SweepStarted>('/send/execute', req);
}

export function getSweepStatus(sweepID: string): Promise<APIResponse<TxResult[]>> {
	return api.get<TxResult[]>(`/send/sweep/${sweepID}`);
}

export function gasPreSeed(req: GasPreSeedRequest): Promise<APIResponse<GasPreSeedResult>> {
	return api.post<GasPreSeedResult>('/send/gas-preseed', req);
}

// Transaction History API

export function getTransactions(
	params: TransactionListParams = {}
): Promise<APIResponse<Transaction[]>> {
	const searchParams = new URLSearchParams();
	if (params.chain) searchParams.set('chain', params.chain);
	if (params.direction) searchParams.set('direction', params.direction);
	if (params.token) searchParams.set('token', params.token);
	if (params.status) searchParams.set('status', params.status);
	if (params.page !== undefined) searchParams.set('page', String(params.page));
	if (params.pageSize !== undefined) searchParams.set('pageSize', String(params.pageSize));

	const qs = searchParams.toString();
	return api.get<Transaction[]>(`/transactions${qs ? '?' + qs : ''}`);
}

// Settings API

export function getSettings(): Promise<APIResponse<Settings>> {
	return api.get<Settings>('/settings');
}

export function updateSettings(settings: Partial<Settings>): Promise<APIResponse<Settings>> {
	return api.put<Settings>('/settings', settings);
}

export function resetBalances(): Promise<APIResponse<{ message: string }>> {
	return api.post<{ message: string }>('/settings/reset-balances', { confirm: true });
}

export function resetAll(): Promise<APIResponse<{ message: string }>> {
	return api.post<{ message: string }>('/settings/reset-all', { confirm: true });
}

// Provider Health API

export function getProviderHealth(): Promise<APIResponse<ProviderHealthMap>> {
	return api.get<ProviderHealthMap>('/health/providers');
}
