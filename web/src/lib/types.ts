// Chain represents a supported blockchain.
export type Chain = 'BTC' | 'BSC' | 'SOL';

// Token represents a supported token symbol.
export type TokenSymbol = 'BTC' | 'BNB' | 'SOL' | 'USDC' | 'USDT';

// Network represents mainnet or testnet.
export type Network = 'mainnet' | 'testnet';

// ScanStatus represents the state of a scan.
export type ScanStatus = 'idle' | 'scanning' | 'paused' | 'completed' | 'error';

// TransactionDirection represents inbound or outbound.
export type TransactionDirection = 'in' | 'out';

// TransactionStatus represents the state of a transaction.
export type TransactionStatus = 'pending' | 'confirmed' | 'failed';

// Address represents a derived HD wallet address.
export interface Address {
	chain: Chain;
	addressIndex: number;
	address: string;
	createdAt: string;
}

// AddressBalance represents an address with its balances.
export interface AddressBalance {
	chain: Chain;
	index: number;
	address: string;
	nativeBalance: string;
	tokenBalances: TokenBalance[];
	lastScanned: string | null;
}

// TokenBalance represents the balance of a specific token.
export interface TokenBalance {
	symbol: TokenSymbol;
	balance: string;
	contractAddress: string;
}

// ScanState represents the scanning progress for a chain.
export interface ScanState {
	chain: Chain;
	lastScannedIndex: number;
	maxScanId: number;
	status: ScanStatus;
	startedAt: string | null;
	updatedAt: string | null;
}

// ScanProgress represents SSE progress data.
export interface ScanProgress {
	chain: Chain;
	scanned: number;
	total: number;
	found: number;
	elapsed: string;
}

// Transaction represents a recorded transaction.
export interface Transaction {
	id: number;
	chain: Chain;
	addressIndex: number;
	txHash: string;
	direction: TransactionDirection;
	token: string;
	amount: string;
	fromAddress: string;
	toAddress: string;
	blockNumber: number | null;
	status: TransactionStatus;
	createdAt: string;
	confirmedAt: string | null;
}

// PortfolioSummary represents the dashboard portfolio overview.
export interface PortfolioSummary {
	totalUsd: number;
	chains: ChainSummary[];
}

// ChainSummary represents balance summary for a single chain.
export interface ChainSummary {
	chain: Chain;
	nativeBalance: string;
	nativeUsd: number;
	tokens: TokenSummaryItem[];
	addressCount: number;
	fundedCount: number;
}

// TokenSummaryItem represents a token balance summary.
export interface TokenSummaryItem {
	symbol: TokenSymbol;
	balance: string;
	usd: number;
}

// HealthResponse represents the /api/health response.
export interface HealthResponse {
	status: string;
	version: string;
	network: Network;
	dbPath: string;
}

// APIResponse is the standard API response wrapper.
export interface APIResponse<T> {
	data: T;
	meta?: APIMeta;
}

// APIMeta contains pagination and execution metadata.
export interface APIMeta {
	page?: number;
	pageSize?: number;
	total?: number;
	executionTime?: number;
}

// APIError is the standard error response.
export interface APIErrorResponse {
	error: {
		code: string;
		message: string;
	};
}

// PriceData represents current prices.
export interface PriceData {
	btc: number;
	bnb: number;
	sol: number;
}

// Settings represents user settings.
export interface Settings {
	maxScanId: number;
	network: Network;
}
