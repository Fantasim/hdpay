import { API_BASE } from '$lib/constants';
import type { AddressWithBalance, APIErrorResponse, APIResponse, Chain } from '$lib/types';

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
