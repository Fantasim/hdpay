import { writable } from 'svelte/store';
import { goto } from '$app/navigation';
import * as api from '$lib/utils/api';

export const isAuthenticated = writable<boolean>(false);
export const isLoading = writable<boolean>(true);
export const authError = writable<string | null>(null);

/**
 * Attempt login with username and password.
 * On success: sets isAuthenticated=true, redirects to '/'.
 * On failure: sets authError with the error message.
 */
export async function login(username: string, password: string): Promise<void> {
	authError.set(null);

	try {
		await api.login(username, password);
		isAuthenticated.set(true);
		goto('/');
	} catch (err) {
		if (err instanceof api.ApiError) {
			authError.set(err.message);
		} else {
			authError.set('An unexpected error occurred');
		}
	}
}

/**
 * Logout: clears session and redirects to login page.
 */
export async function logout(): Promise<void> {
	try {
		await api.logout();
	} catch {
		// Ignore logout errors — clear local state regardless
	}
	isAuthenticated.set(false);
	goto('/login');
}

/**
 * Check if current session is valid by calling a protected endpoint.
 * Sets isAuthenticated accordingly and clears isLoading.
 */
export async function checkSession(): Promise<void> {
	isLoading.set(true);
	try {
		await api.getSettings();
		isAuthenticated.set(true);
	} catch {
		isAuthenticated.set(false);
	} finally {
		isLoading.set(false);
	}
}
