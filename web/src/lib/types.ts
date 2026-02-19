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

// ScanTokenErrorEvent is the SSE payload for scan_token_error (B7).
export interface ScanTokenErrorEvent {
	chain: Chain;
	token: string;
	error: string;
	message: string;
}

// ScanStateSnapshot is the SSE payload for scan_state (B10 resync).
export interface ScanStateSnapshot {
	chain: Chain;
	lastScannedIndex: number;
	maxScanId: number;
	status: string;
	isRunning: boolean;
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

// Settings represents all user settings from the key-value store.
export interface Settings {
	max_scan_id: string;
	auto_resume_scans: string;
	resume_threshold_hours: string;
	btc_fee_rate: string;
	bsc_gas_preseed_bnb: string;
	log_level: string;
}

// TransactionListParams for the transactions API.
export interface TransactionListParams {
	chain?: Chain;
	direction?: TransactionDirection;
	token?: string;
	status?: TransactionStatus;
	page?: number;
	pageSize?: number;
}

// --- Send Types ---

// SendToken represents valid token values for the send API.
export type SendToken = 'NATIVE' | 'USDC' | 'USDT';

// SendRequest is the request body for preview and execute.
export interface SendRequest {
	chain: Chain;
	token: SendToken;
	destination: string;
}

// FundedAddressInfo is a row in the preview's funded address table.
export interface FundedAddressInfo {
	addressIndex: number;
	address: string;
	balance: string;
	hasGas: boolean;
}

// UnifiedSendPreview is the unified preview response for all chains.
export interface UnifiedSendPreview {
	chain: Chain;
	token: SendToken;
	destination: string;
	fundedCount: number;
	totalAmount: string;
	feeEstimate: string;
	netAmount: string;
	txCount: number;
	needsGasPreSeed: boolean;
	gasPreSeedCount: number;
	fundedAddresses: FundedAddressInfo[];
}

// TxResult is a single transaction result in a unified sweep.
export interface TxResult {
	addressIndex: number;
	fromAddress: string;
	txHash: string;
	amount: string;
	status: string;
	error?: string;
}

// UnifiedSendResult is the unified execute response for all chains.
export interface UnifiedSendResult {
	chain: Chain;
	token: SendToken;
	txResults: TxResult[];
	successCount: number;
	failCount: number;
	totalSwept: string;
}

// GasPreSeedRequest is the request body for gas pre-seeding.
export interface GasPreSeedRequest {
	sourceIndex: number;
	targetAddresses: string[];
}

// GasPreSeedPreview contains the preview of a gas pre-seeding operation.
export interface GasPreSeedPreview {
	sourceIndex: number;
	sourceAddress: string;
	sourceBalance: string;
	targetCount: number;
	amountPerTarget: string;
	totalNeeded: string;
	sufficient: boolean;
}

// GasPreSeedResult contains the result of a gas pre-seeding operation.
export interface GasPreSeedResult {
	txResults: TxResult[];
	successCount: number;
	failCount: number;
	totalSent: string;
}

// SendStep represents the current step in the send wizard.
export type SendStep = 'select' | 'preview' | 'gas-preseed' | 'execute' | 'complete';

// --- Provider Health Types ---

// ProviderHealthStatus represents the health status of a provider.
export type ProviderHealthStatus = 'healthy' | 'degraded' | 'down';

// CircuitState represents the circuit breaker state.
export type CircuitState = 'closed' | 'open' | 'half_open';

// ProviderHealth represents a single provider's health from GET /api/health/providers.
export interface ProviderHealth {
	name: string;
	chain: Chain;
	type: string;
	status: ProviderHealthStatus;
	circuitState: CircuitState;
	consecutiveFails: number;
	lastSuccess: string;
	lastError: string;
	lastErrorMsg: string;
}

// ProviderHealthMap is the response data from GET /api/health/providers.
export type ProviderHealthMap = Record<Chain, ProviderHealth[]>;
