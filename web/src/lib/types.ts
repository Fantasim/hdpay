// Chain represents a supported blockchain.
export type Chain = 'BTC' | 'BSC' | 'SOL';

// Token represents a supported token symbol.
export type TokenSymbol = 'BTC' | 'BNB' | 'SOL' | 'USDC' | 'USDT';

// Network represents mainnet or testnet.
export type Network = 'mainnet' | 'testnet';

// ScanStatus represents the state of a scan.
export type ScanStatus = 'idle' | 'scanning' | 'paused' | 'completed' | 'failed' | 'error';

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

// AddressWithBalance represents an address with its balances (matches backend API).
export interface AddressWithBalance {
	chain: Chain;
	addressIndex: number;
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

// ScanCompleteEvent is the SSE payload for scan_complete.
export interface ScanCompleteEvent {
	chain: Chain;
	scanned: number;
	found: number;
	duration: string;
}

// ScanErrorEvent is the SSE payload for scan_error.
export interface ScanErrorEvent {
	chain: Chain;
	error: string;
	message: string;
}

// ScanStateWithRunning augments ScanState with live running flag from backend.
export interface ScanStateWithRunning extends ScanState {
	isRunning: boolean;
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

// PriceData represents current USD prices keyed by symbol.
export interface PriceData {
	BTC: number;
	BNB: number;
	SOL: number;
	USDC: number;
	USDT: number;
}

// PortfolioResponse represents the GET /api/dashboard/portfolio response data.
export interface PortfolioResponse {
	totalUsd: number;
	lastScan: string | null;
	chains: ChainPortfolio[];
}

// ChainPortfolio represents a single chain's portfolio data.
export interface ChainPortfolio {
	chain: Chain;
	addressCount: number;
	fundedCount: number;
	tokens: TokenPortfolioItem[];
}

// TokenPortfolioItem represents a token balance within a chain's portfolio.
export interface TokenPortfolioItem {
	symbol: TokenSymbol;
	balance: string;
	usd: number;
	fundedCount: number;
}

// Settings represents user settings.
export interface Settings {
	maxScanId: number;
	network: Network;
}
