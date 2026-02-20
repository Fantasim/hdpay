import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// Mock SvelteKit $app/environment
vi.mock('$app/environment', () => ({
	browser: true,
	dev: true,
	building: false,
	version: 'test'
}));

// Mock SvelteKit $app/navigation
vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn(),
	invalidate: vi.fn(),
	invalidateAll: vi.fn(),
	preloadData: vi.fn(),
	preloadCode: vi.fn()
}));

// Mock SvelteKit $app/state
vi.mock('$app/state', () => ({
	page: {
		url: new URL('http://localhost:5173'),
		params: {},
		route: { id: '/' },
		status: 200,
		error: null,
		data: {},
		form: null
	}
}));
