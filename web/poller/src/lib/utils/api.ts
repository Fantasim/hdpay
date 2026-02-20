import { goto } from '$app/navigation';
import { API_BASE } from '$lib/constants';
import type {
	AdminSettings,
	APIListResponse,
	APIResponse,
	ChartData,
	ClaimRequest,
	ClaimResult,
	CreateWatchRequest,
	CreateWatchResponse,
	DashboardErrors,
	DashboardStats,
	HealthResponse,
	IPAllowlistEntry,
	PendingPointsWithTransactions,
	PointsWithTransactions,
	Tier,
	TimeRange,
	Transaction,
	TransactionFilters,
	Watch,
	WatchDefaults,
	WatchStatus
} from '$lib/types';

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

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
	const url = `${API_BASE}${path}`;
	const headers: Record<string, string> = {
		'Content-Type': 'application/json'
	};

	const res = await fetch(url, {
		method,
		headers,
		credentials: 'same-origin',
		body: body ? JSON.stringify(body) : undefined
	});

	if (!res.ok) {
		// Auto-redirect to login on 401
		if (res.status === 401) {
			goto('/login');
			throw new ApiError('ERROR_SESSION_EXPIRED', 'Session expired', 401);
		}

		const errorBody = await res.json().catch(() => ({
			error: { code: 'UNKNOWN', message: res.statusText }
		}));

		throw new ApiError(
			errorBody.error?.code ?? 'UNKNOWN',
			errorBody.error?.message ?? res.statusText,
			res.status
		);
	}

	return (await res.json()) as T;
}

// API client — single source of truth
const api = {
	get<T>(path: string): Promise<T> {
		return request<T>('GET', path);
	},
	post<T>(path: string, body?: unknown): Promise<T> {
		return request<T>('POST', path, body);
	},
	put<T>(path: string, body?: unknown): Promise<T> {
		return request<T>('PUT', path, body);
	},
	delete<T>(path: string): Promise<T> {
		return request<T>('DELETE', path);
	}
};

// ===== Auth =====

export function login(
	username: string,
	password: string
): Promise<APIResponse<{ message: string }>> {
	return api.post('/admin/login', { username, password });
}

export function logout(): Promise<APIResponse<{ message: string }>> {
	return api.post('/admin/logout');
}

// ===== Health =====

export function getHealth(): Promise<APIResponse<HealthResponse>> {
	return api.get('/health');
}

// ===== Watch =====

export function createWatch(req: CreateWatchRequest): Promise<APIResponse<CreateWatchResponse>> {
	return api.post('/watch', req);
}

export function cancelWatch(id: string): Promise<APIResponse<{ message: string }>> {
	return api.delete(`/watch/${id}`);
}

export function listWatches(params?: {
	status?: WatchStatus;
	chain?: string;
}): Promise<APIResponse<Watch[]>> {
	const searchParams = new URLSearchParams();
	if (params?.status) searchParams.set('status', params.status);
	if (params?.chain) searchParams.set('chain', params.chain);
	const qs = searchParams.toString();
	return api.get(`/watches${qs ? '?' + qs : ''}`);
}

// ===== Points =====

export function getPoints(): Promise<APIResponse<PointsWithTransactions[]>> {
	return api.get('/points');
}

export function getPendingPoints(): Promise<APIResponse<PendingPointsWithTransactions[]>> {
	return api.get('/points/pending');
}

export function claimPoints(req: ClaimRequest): Promise<APIResponse<ClaimResult>> {
	return api.post('/points/claim', req);
}

// ===== Admin =====

export function getSettings(): Promise<APIResponse<AdminSettings>> {
	return api.get('/admin/settings');
}

export function updateTiers(tiers: Tier[]): Promise<APIResponse<{ message: string }>> {
	return api.put('/admin/tiers', { tiers });
}

export function updateWatchDefaults(
	defaults: WatchDefaults
): Promise<APIResponse<{ message: string }>> {
	return api.put('/admin/watch-defaults', defaults);
}

export function getAllowlist(): Promise<APIResponse<IPAllowlistEntry[]>> {
	return api.get('/admin/allowlist');
}

export function addAllowlistIP(
	ip: string,
	description?: string
): Promise<APIResponse<IPAllowlistEntry>> {
	return api.post('/admin/allowlist', { ip, description });
}

export function removeAllowlistIP(id: number): Promise<APIResponse<{ message: string }>> {
	return api.delete(`/admin/allowlist/${id}`);
}

// ===== Dashboard =====

export function getDashboardStats(range: TimeRange): Promise<APIResponse<DashboardStats>> {
	return api.get(`/dashboard/stats?range=${range}`);
}

export function getDashboardTransactions(
	filters: TransactionFilters = {}
): Promise<APIListResponse<Transaction>> {
	const params = new URLSearchParams();
	if (filters.chain) params.set('chain', filters.chain);
	if (filters.token) params.set('token', filters.token);
	if (filters.status) params.set('status', filters.status);
	if (filters.tier !== undefined) params.set('tier', String(filters.tier));
	if (filters.min_usd !== undefined) params.set('min_usd', String(filters.min_usd));
	if (filters.max_usd !== undefined) params.set('max_usd', String(filters.max_usd));
	if (filters.date_from) params.set('date_from', filters.date_from);
	if (filters.date_to) params.set('date_to', filters.date_to);
	if (filters.page !== undefined) params.set('page', String(filters.page));
	if (filters.page_size !== undefined) params.set('page_size', String(filters.page_size));
	const qs = params.toString();
	return api.get(`/dashboard/transactions${qs ? '?' + qs : ''}`);
}

export function getDashboardCharts(range: TimeRange): Promise<APIResponse<ChartData>> {
	return api.get(`/dashboard/charts?range=${range}`);
}

export function getDashboardErrors(): Promise<APIResponse<DashboardErrors>> {
	return api.get('/dashboard/errors');
}
