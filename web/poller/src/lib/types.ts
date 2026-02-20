// Core domain types — matching backend models exactly

export type Chain = 'BTC' | 'BSC' | 'SOL';
export type Token = 'BTC' | 'BNB' | 'SOL' | 'USDC' | 'USDT';
export type WatchStatus = 'ACTIVE' | 'COMPLETED' | 'EXPIRED' | 'CANCELLED';
export type TxStatus = 'PENDING' | 'CONFIRMED';
export type TimeRange = 'today' | 'week' | 'month' | 'quarter' | 'all';

// API response wrappers
export interface APIResponse<T> {
	data: T;
}

export interface APIListResponse<T> {
	data: T[];
	meta: PaginationMeta;
}

export interface APIErrorResponse {
	error: {
		code: string;
		message: string;
	};
}

export interface PaginationMeta {
	page: number;
	pageSize: number;
	total: number;
}

// Watch
export interface Watch {
	id: string;
	chain: string;
	address: string;
	status: WatchStatus;
	started_at: string;
	expires_at: string;
	completed_at: string | null;
	poll_count: number;
	last_poll_at: string | null;
	last_poll_result: string | null;
	created_at: string;
}

export interface CreateWatchRequest {
	chain: Chain;
	address: string;
	timeout_minutes?: number;
}

export interface CreateWatchResponse {
	watch_id: string;
	chain: string;
	address: string;
	status: WatchStatus;
	started_at: string;
	expires_at: string;
	poll_interval_seconds: number;
}

// Transaction
export interface Transaction {
	id: number;
	watch_id: string;
	tx_hash: string;
	chain: string;
	address: string;
	token: string;
	amount_raw: string;
	amount_human: string;
	decimals: number;
	usd_value: number;
	usd_price: number;
	tier: number;
	multiplier: number;
	points: number;
	status: TxStatus;
	confirmations: number;
	block_number: number | null;
	detected_at: string;
	confirmed_at: string | null;
	created_at: string;
}

export interface TransactionFilters {
	chain?: string;
	token?: string;
	status?: TxStatus;
	tier?: number;
	min_usd?: number;
	max_usd?: number;
	date_from?: string;
	date_to?: string;
	page?: number;
	page_size?: number;
}

// Points
export interface PointsAccount {
	address: string;
	chain: string;
	unclaimed: number;
	pending: number;
	total: number;
	updated_at: string;
}

export interface PointsWithTransactions {
	address: string;
	chain: string;
	unclaimed: number;
	total: number;
	transactions: Transaction[];
}

export interface PendingPointsWithTransactions {
	address: string;
	chain: string;
	pending_points: number;
	transactions: Transaction[];
}

export interface ClaimRequest {
	addresses: string[];
}

export interface ClaimResult {
	claimed: ClaimedEntry[];
	skipped: string[];
	total_claimed: number;
}

export interface ClaimedEntry {
	address: string;
	chain: string;
	points_claimed: number;
}

// Dashboard
export interface DashboardStats {
	range: TimeRange;
	active_watches: number;
	total_watches: number;
	watches_completed: number;
	watches_expired: number;
	usd_received: number;
	points_awarded: number;
	pending_points: {
		accounts: number;
		total: number;
	};
	unique_addresses: number;
	avg_tx_usd: number;
	largest_tx_usd: number;
	by_day: DailyStatRow[];
}

export interface DailyStatRow {
	date: string;
	usd: number;
	points: number;
	txs: number;
}

export interface ChartData {
	usd_over_time: Array<{ date: string; usd: number }>;
	points_over_time: Array<{ date: string; points: number }>;
	tx_count_over_time: Array<{ date: string; count: number }>;
	by_chain: ChainBreakdown[];
	by_token: TokenBreakdown[];
	by_tier: TierBreakdown[];
	watches_over_time: DailyWatchStat[];
}

export interface ChainBreakdown {
	chain: string;
	usd: number;
	count: number;
}

export interface TokenBreakdown {
	token: string;
	usd: number;
	count: number;
}

export interface TierBreakdown {
	tier: number;
	count: number;
	total_points: number;
}

export interface DailyWatchStat {
	date: string;
	active: number;
	completed: number;
	expired: number;
}

// Dashboard Errors
export interface DashboardErrors {
	discrepancies: DiscrepancyRow[];
	errors: SystemError[];
	stale_pending: StalePendingRow[];
}

export interface DiscrepancyRow {
	type: string;
	address?: string;
	chain?: string;
	message: string;
	calculated?: number;
	stored?: number;
}

export interface SystemError {
	id: number;
	severity: string;
	category: string;
	message: string;
	details?: string;
	resolved: boolean;
	created_at: string;
}

export interface StalePendingRow {
	tx_hash: string;
	chain: string;
	address: string;
	detected_at: string;
	hours_pending: number;
}

// IP Allowlist
export interface IPAllowlistEntry {
	id: number;
	ip: string;
	description?: string;
	added_at: string;
}

// Tiers
export interface Tier {
	min_usd: number;
	max_usd: number | null;
	multiplier: number;
}

// Admin Settings
export interface AdminSettings {
	network: string;
	db_path: string;
	start_date: number;
	default_watch_timeout_min: number;
	max_active_watches: number;
	tiers: Tier[];
	tiers_file: string;
}

// Health
export interface HealthResponse {
	status: string;
	network: string;
	uptime: string;
	version: string;
}

// Watch Defaults
export interface WatchDefaults {
	default_watch_timeout_min: number;
	max_active_watches: number;
}
