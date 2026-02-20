import type { Chain, TimeRange, TxStatus, WatchStatus } from './types';

// API
export const API_BASE = '/api';

// Display
export const BALANCE_DECIMAL_PLACES = 6;
export const ADDRESS_TRUNCATE_LENGTH = 8;

// Chains
export const SUPPORTED_CHAINS: readonly Chain[] = ['BTC', 'BSC', 'SOL'] as const;
export const CHAIN_COLORS: Record<Chain, string> = {
	BTC: '#f7931a',
	BSC: '#F0B90B',
	SOL: '#9945FF'
} as const;
export const CHAIN_NATIVE_SYMBOLS: Record<Chain, string> = {
	BTC: 'BTC',
	BSC: 'BNB',
	SOL: 'SOL'
} as const;
export const CHAIN_TOKENS: Record<Chain, readonly string[]> = {
	BTC: ['BTC'],
	BSC: ['BNB', 'USDC', 'USDT'],
	SOL: ['SOL', 'USDC', 'USDT']
} as const;

// Watch statuses
export const WATCH_STATUSES: readonly WatchStatus[] = [
	'ACTIVE',
	'COMPLETED',
	'EXPIRED',
	'CANCELLED'
] as const;
export const STATUS_COLORS: Record<WatchStatus, string> = {
	ACTIVE: '#3b82f6',
	COMPLETED: '#10b981',
	EXPIRED: '#ef4444',
	CANCELLED: '#6b7280'
} as const;

// Transaction statuses
export const TX_STATUSES: readonly TxStatus[] = ['PENDING', 'CONFIRMED'] as const;
export const TX_STATUS_COLORS: Record<TxStatus, string> = {
	PENDING: '#f59e0b',
	CONFIRMED: '#10b981'
} as const;

// Time ranges for dashboard
export const TIME_RANGES: readonly TimeRange[] = [
	'today',
	'week',
	'month',
	'quarter',
	'all'
] as const;
export const TIME_RANGE_LABELS: Record<TimeRange, string> = {
	today: 'Today',
	week: 'This Week',
	month: 'This Month',
	quarter: 'This Quarter',
	all: 'All Time'
} as const;

// Chart colors
export const CHART_COLORS: readonly string[] = [
	'#f7931a',
	'#F0B90B',
	'#9945FF',
	'#3b82f6',
	'#10b981'
] as const;

// Block explorer URLs (mainnet)
export const EXPLORER_TX_URL: Record<Chain, string> = {
	BTC: 'https://blockstream.info/tx/',
	BSC: 'https://bscscan.com/tx/',
	SOL: 'https://solscan.io/tx/'
} as const;

// Block explorer URLs (testnet)
export const EXPLORER_TX_URL_TESTNET: Record<Chain, string> = {
	BTC: 'https://blockstream.info/testnet/tx/',
	BSC: 'https://testnet.bscscan.com/tx/',
	SOL: 'https://solscan.io/tx/?cluster=devnet/'
} as const;

// Confirmation thresholds
export const CONFIRMATIONS_REQUIRED: Record<Chain, number | string> = {
	BTC: 1,
	BSC: 12,
	SOL: 'finalized'
} as const;

// Sidebar navigation items
export const NAV_ITEMS = [
	{ label: 'Overview', path: '/', icon: 'chart' },
	{ label: 'Transactions', path: '/transactions', icon: 'list' },
	{ label: 'Watches', path: '/watches', icon: 'eye' },
	{ label: 'Pending Points', path: '/points', icon: 'coins' },
	{ label: 'Errors', path: '/errors', icon: 'alert' },
	{ label: 'Settings', path: '/settings', icon: 'settings' }
] as const;

// Error codes (mirror backend config/errors.go)
export const ERROR_ALREADY_WATCHING = 'ERROR_ALREADY_WATCHING';
export const ERROR_WATCH_NOT_FOUND = 'ERROR_WATCH_NOT_FOUND';
export const ERROR_WATCH_EXPIRED = 'ERROR_WATCH_EXPIRED';
export const ERROR_ADDRESS_NOT_FOUND = 'ERROR_ADDRESS_NOT_FOUND';
export const ERROR_ADDRESS_INVALID = 'ERROR_ADDRESS_INVALID';
export const ERROR_INVALID_CHAIN = 'ERROR_INVALID_CHAIN';
export const ERROR_INVALID_TOKEN = 'ERROR_INVALID_TOKEN';
export const ERROR_INVALID_TIMEOUT = 'ERROR_INVALID_TIMEOUT';
export const ERROR_MAX_WATCHES = 'ERROR_MAX_WATCHES';
export const ERROR_TX_ALREADY_RECORDED = 'ERROR_TX_ALREADY_RECORDED';
export const ERROR_NOTHING_TO_CLAIM = 'ERROR_NOTHING_TO_CLAIM';
export const ERROR_PROVIDER_UNAVAILABLE = 'ERROR_PROVIDER_UNAVAILABLE';
export const ERROR_PROVIDER_RATE_LIMIT = 'ERROR_PROVIDER_RATE_LIMIT';
export const ERROR_PRICE_FETCH_FAILED = 'ERROR_PRICE_FETCH_FAILED';
export const ERROR_DATABASE = 'ERROR_DATABASE';
export const ERROR_TIERS_INVALID = 'ERROR_TIERS_INVALID';
export const ERROR_TIERS_FILE = 'ERROR_TIERS_FILE';
export const ERROR_UNAUTHORIZED = 'ERROR_UNAUTHORIZED';
export const ERROR_FORBIDDEN = 'ERROR_FORBIDDEN';
export const ERROR_SESSION_EXPIRED = 'ERROR_SESSION_EXPIRED';
export const ERROR_IP_NOT_ALLOWED = 'ERROR_IP_NOT_ALLOWED';
export const ERROR_INVALID_CREDENTIALS = 'ERROR_INVALID_CREDENTIALS';
export const ERROR_DISCREPANCY = 'ERROR_DISCREPANCY';
export const ERROR_INVALID_REQUEST = 'ERROR_INVALID_REQUEST';
export const ERROR_INTERNAL = 'ERROR_INTERNAL';
