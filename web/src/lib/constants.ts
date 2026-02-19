// API
export const API_BASE = '/api';
export const SSE_RECONNECT_DELAY_MS = 1000;
export const SSE_MAX_RECONNECT_DELAY_MS = 30_000;
export const SSE_BACKOFF_MULTIPLIER = 2;

// Scan
export const DEFAULT_MAX_SCAN_ID = 5000;
export const MAX_SCAN_ID = 500_000;

// Display
export const MAX_TABLE_ROWS_DISPLAY = 1000;
export const ADDRESS_TRUNCATE_LENGTH = 8;
export const BALANCE_DECIMAL_PLACES = 6;

// Chains
export const SUPPORTED_CHAINS = ['BTC', 'BSC', 'SOL'] as const;
export const CHAIN_NATIVE_SYMBOLS = { BTC: 'BTC', BSC: 'BNB', SOL: 'SOL' } as const;
export const CHAIN_TOKENS = {
	BSC: ['USDC', 'USDT'],
	SOL: ['USDC', 'USDT'],
	BTC: []
} as const;

// Token Decimals — raw balance units to human-readable conversion
// Mirrors Go config/constants.go Token Decimals section
export const TOKEN_DECIMALS: Record<string, Record<string, number>> = {
	BTC: { NATIVE: 8 },
	BSC: { NATIVE: 18, USDC: 18, USDT: 18 },
	SOL: { NATIVE: 9, USDC: 6, USDT: 6 }
} as const;

// Dashboard Refresh
export const PRICE_REFRESH_INTERVAL_MS = 5 * 60 * 1000; // 5 minutes
export const PORTFOLIO_REFRESH_INTERVAL_MS = 60 * 1000; // 1 minute

// Chart Colors
export const CHART_COLORS = ['#f7931a', '#F0B90B', '#9945FF', '#3b82f6', '#10b981'] as const;

// Chain Colors
export const CHAIN_COLORS = {
	BTC: '#f7931a',
	BSC: '#F0B90B',
	SOL: '#9945FF'
} as const;

// Transactions
export const DEFAULT_TX_PAGE_SIZE = 20;
export const TX_DIRECTIONS = ['in', 'out'] as const;
export const TX_STATUSES = ['pending', 'confirmed', 'failed'] as const;

// Settings
export const RESUME_THRESHOLD_OPTIONS = [1, 6, 12, 24, 48] as const;
export const LOG_LEVELS = ['debug', 'info', 'warn', 'error'] as const;

// Send polling fallback
export const SEND_POLL_INTERVAL_MS = 3000;

// Error Codes (mirror backend)
export const ERROR_INVALID_MNEMONIC = 'ERROR_INVALID_MNEMONIC';
export const ERROR_ADDRESS_GENERATION = 'ERROR_ADDRESS_GENERATION';
export const ERROR_DATABASE = 'ERROR_DATABASE';
export const ERROR_SCAN_FAILED = 'ERROR_SCAN_FAILED';
export const ERROR_SCAN_INTERRUPTED = 'ERROR_SCAN_INTERRUPTED';
export const ERROR_PROVIDER_RATE_LIMIT = 'ERROR_PROVIDER_RATE_LIMIT';
export const ERROR_PROVIDER_UNAVAILABLE = 'ERROR_PROVIDER_UNAVAILABLE';
export const ERROR_INSUFFICIENT_BALANCE = 'ERROR_INSUFFICIENT_BALANCE';
export const ERROR_INSUFFICIENT_GAS = 'ERROR_INSUFFICIENT_GAS';
export const ERROR_TX_BUILD_FAILED = 'ERROR_TX_BUILD_FAILED';
export const ERROR_TX_SIGN_FAILED = 'ERROR_TX_SIGN_FAILED';
export const ERROR_TX_BROADCAST_FAILED = 'ERROR_TX_BROADCAST_FAILED';
export const ERROR_INVALID_ADDRESS = 'ERROR_INVALID_ADDRESS';
export const ERROR_INVALID_CHAIN = 'ERROR_INVALID_CHAIN';
export const ERROR_INVALID_TOKEN = 'ERROR_INVALID_TOKEN';
export const ERROR_EXPORT_FAILED = 'ERROR_EXPORT_FAILED';
export const ERROR_PRICE_FETCH_FAILED = 'ERROR_PRICE_FETCH_FAILED';
export const ERROR_INVALID_CONFIG = 'ERROR_INVALID_CONFIG';
