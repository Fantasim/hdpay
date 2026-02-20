import type { ProviderHealth } from '$lib/types';

/**
 * Returns the CSS class for a provider's status indicator dot.
 */
export function statusDotClass(provider: ProviderHealth): string {
	if (provider.status === 'healthy' && provider.circuitState === 'closed') {
		return 'provider-dot-healthy';
	}
	if (provider.status === 'degraded' || provider.circuitState === 'half_open') {
		return 'provider-dot-degraded';
	}
	return 'provider-dot-down';
}

/**
 * Returns a human-readable status label for a provider.
 */
export function statusLabel(provider: ProviderHealth): string {
	if (provider.circuitState === 'open') return 'Down';
	if (provider.circuitState === 'half_open') return 'Degraded';
	if (provider.consecutiveFails > 0) return `${provider.consecutiveFails} fails`;
	return 'Healthy';
}
