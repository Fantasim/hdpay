import { describe, it, expect } from 'vitest';
import { statusDotClass, statusLabel } from './providers';
import type { ProviderHealth } from '$lib/types';

function makeProvider(overrides: Partial<ProviderHealth> = {}): ProviderHealth {
	return {
		name: 'TestProvider',
		chain: 'BTC',
		type: 'http',
		status: 'healthy',
		circuitState: 'closed',
		consecutiveFails: 0,
		lastSuccess: '',
		lastError: '',
		lastErrorMsg: '',
		...overrides
	};
}

describe('statusDotClass', () => {
	it('returns healthy class when healthy + closed circuit', () => {
		const provider = makeProvider({ status: 'healthy', circuitState: 'closed' });
		expect(statusDotClass(provider)).toBe('provider-dot-healthy');
	});

	it('returns degraded class when status is degraded', () => {
		const provider = makeProvider({ status: 'degraded', circuitState: 'closed' });
		expect(statusDotClass(provider)).toBe('provider-dot-degraded');
	});

	it('returns degraded class when circuit is half_open', () => {
		const provider = makeProvider({ status: 'healthy', circuitState: 'half_open' });
		expect(statusDotClass(provider)).toBe('provider-dot-degraded');
	});

	it('returns down class when status is down + open circuit', () => {
		const provider = makeProvider({ status: 'down', circuitState: 'open' });
		expect(statusDotClass(provider)).toBe('provider-dot-down');
	});

	it('returns down class when healthy but circuit open', () => {
		const provider = makeProvider({ status: 'healthy', circuitState: 'open' });
		expect(statusDotClass(provider)).toBe('provider-dot-down');
	});
});

describe('statusLabel', () => {
	it('returns Healthy for healthy provider', () => {
		const provider = makeProvider({ circuitState: 'closed', consecutiveFails: 0 });
		expect(statusLabel(provider)).toBe('Healthy');
	});

	it('returns Down when circuit is open', () => {
		const provider = makeProvider({ circuitState: 'open' });
		expect(statusLabel(provider)).toBe('Down');
	});

	it('returns Degraded when circuit is half_open', () => {
		const provider = makeProvider({ circuitState: 'half_open' });
		expect(statusLabel(provider)).toBe('Degraded');
	});

	it('returns fail count when consecutiveFails > 0', () => {
		const provider = makeProvider({ circuitState: 'closed', consecutiveFails: 5 });
		expect(statusLabel(provider)).toBe('5 fails');
	});

	it('returns 1 fails for single failure', () => {
		const provider = makeProvider({ circuitState: 'closed', consecutiveFails: 1 });
		expect(statusLabel(provider)).toBe('1 fails');
	});
});
