package config

import "time"

// Address Generation
const (
	MaxAddressesPerChain = 500_000
	DefaultMaxScanID     = 5_000
)

// BIP-44 / BIP-84 Derivation Paths
const (
	BIP44Purpose    = 44  // Standard BIP-44 purpose
	BIP84Purpose    = 84  // BIP-84 purpose for Native SegWit (bech32)
	BTCCoinType     = 0   // m/84'/0'/0'/0/N (Native SegWit bech32)
	BSCCoinType     = 60  // m/44'/60'/0'/0/N (same as ETH)
	SOLCoinType     = 501 // m/44'/501'/N'/0'
	BTCTestCoinType = 1   // Testnet
)

// Token Contract Addresses — BSC Mainnet
const (
	BSCUSDCContract = "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"
	BSCUSDTContract = "0x55d398326f99059fF775485246999027B3197955"
)

// Token Contract Addresses — SOL Mainnet
const (
	SOLUSDCMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	SOLUSDTMint = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
)

// Token Contract Addresses — BSC Testnet (community-deployed test tokens)
const (
	BSCTestnetUSDCContract = "0x64544969ed7EBf5f083679233325356EbE738930" // 6 decimals
	BSCTestnetUSDTContract = "0x337610d27c682E347C9cD60BD4b3b107C9d34dDd" // 18 decimals
)

// Token Contract Addresses — SOL Testnet
// Note: Solana testnet does not have official Circle USDC/USDT deployments.
// These addresses are used as placeholders for test infrastructure; update to
// actual deployed mint addresses if/when available on testnet.
const (
	SOLTestnetUSDCMint = "CpMah17kQEL2wqyMKt3mZBdTnZbkbfx4nqmQMFDP5vwp" // placeholder — no official Circle testnet USDC
	SOLTestnetUSDTMint = ""                                                 // no testnet USDT exists
)

// Pagination
const (
	DefaultPage     = 1
	DefaultPageSize = 100
	MaxPageSize     = 1000
)

// Scanning
const (
	ScanBatchSizeBscScan   = 20  // BscScan balancemulti (native only)
	ScanBatchSizeSolanaRPC = 100 // getMultipleAccounts limit
	ScanResumeThreshold    = 24 * time.Hour
)

// Multicall3 — deployed on all EVM chains including BSC mainnet + testnet.
// Single eth_call reads hundreds of balances (native + BEP-20) in one request.
const (
	Multicall3Address   = "0xcA11bde05977b3631167028862bE2a173976CA11"
	Multicall3BatchSize = 200 // 200 addrs × 3 subcalls = 600 subcalls ≈ 2M gas (safe under 50M cap)
)

// JSON-RPC Batch — fallback when Multicall3 is unavailable.
const (
	BSCRPCBatchSize = 20 // eth_getBalance calls per JSON-RPC batch POST
)

// Scanner Chunk Sizes — how many addresses per scanner iteration (pool distributes across providers).
const (
	ScanChunkSizeBSC = 500 // Multicall3 providers handle the bulk
	ScanChunkSizeBTC = 100 // 3 providers × ~33 addrs each in parallel
	ScanChunkSizeSOL = 100 // matches getMultipleAccounts, already optimal
)

// Scanner Orchestrator
const (
	ScanProgressBroadcastInterval = 500 * time.Millisecond
	ProviderRequestTimeout        = 15 * time.Second
	ProviderMaxRetries            = 3
	ProviderRetryBaseDelay        = 1 * time.Second
	SSEHubChannelBuffer           = 64
	ScanContextTimeout            = 24 * time.Hour // upper bound on scan goroutine lifetime
)

// HTTP Client Connection Pool
const (
	HTTPMaxConnsPerHost     = 10  // max connections per provider host
	HTTPMaxIdleConnsPerHost = 5   // max idle connections per provider host
	HTTPMaxIdleConns        = 50  // max idle connections across all hosts
)

// Provider URLs — BTC Mainnet
const (
	BlockstreamMainnetURL = "https://blockstream.info/api"
	MempoolMainnetURL     = "https://mempool.space/api"
	BitapsMainnetURL      = "https://api.bitaps.com/btc/v1/blockchain"
)

// Provider URLs — BTC Testnet
const (
	BlockstreamTestnetURL = "https://blockstream.info/testnet/api"
	MempoolTestnetURL     = "https://mempool.space/testnet/api"
	BitapsTestnetURL      = "https://api.bitaps.com/btc/testnet/v1/blockchain"
)

// Provider URLs — BSC
const (
	// BscScanAPIURL is kept for reference only — api.bscscan.com was shut down Dec 18, 2025.
	// Do NOT use this URL; all BSC scanning now goes through public RPC nodes.
	BscScanAPIURL     = "https://api.bscscan.com/api" // DEPRECATED
	BscRPCMainnetURL  = "https://bsc-dataseed.binance.org"
	BscRPCMainnetURL2 = "https://rpc.ankr.com/bsc"
	BscRPCMainnetURL3 = "https://bsc-dataseed.nariox.org"
	BscRPCMainnetURL4 = "https://bsc-dataseed.defibit.io"
	BscRPCMainnetURL5 = "https://bsc-dataseed.ninicoin.io"
	BscRPCMainnetURL6 = "https://bsc-dataseed-public.bnbchain.org"
	LlamaNodesBSCURL  = "https://bsc.llamarpc.com"
	DRPCBSCURL        = "https://bsc.drpc.org"
	NodeRealBSCRPCURL = "https://bsc.nodereal.io"
	BscRPCTestnetURL  = "https://data-seed-prebsc-1-s1.binance.org:8545"
	BscScanTestnetURL = "https://api-testnet.bscscan.com/api" // DEPRECATED
	// NodeReal BSCTrace — Etherscan-compatible API (official replacement for deprecated BscScan).
	// Requires free API key from nodereal.io. API key is embedded in the URL path.
	// Full URL format: NodeRealBSCTraceBaseURL + "/" + apiKey + "/bsctrace/api"
	NodeRealBSCTraceBaseURL = "https://open-platform.nodereal.io"
)

// Provider URLs — Solana
const (
	SolanaMainnetRPCURL      = "https://api.mainnet-beta.solana.com"
	HeliusMainnetRPCURL      = "https://mainnet.helius-rpc.com"
	SolanaTestnetRPCURL      = "https://api.testnet.solana.com"
	AnkrSolanaMainnetURL     = "https://rpc.ankr.com/solana"
	AnkrSolanaTestnetURL     = "https://rpc.ankr.com/solana_testnet"
	DRPCSolanaURL            = "https://solana.drpc.org"
	OnFinalitySolanaURL      = "https://solana.api.onfinality.io/public"
	// Alchemy Solana RPC — requires API key from alchemy.com.
	// API key is embedded in the URL path: /v2/{api_key}
	AlchemySolanaMainnetURLFmt = "https://solana-mainnet.g.alchemy.com/v2/%s"
	AlchemySolanaTestnetURLFmt = "https://solana-testnet.g.alchemy.com/v2/%s"
)

// Solana Program IDs
const (
	SOLTokenProgramID           = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	SOLAssociatedTokenProgramID = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"
)

// Rate Limiting (requests per second unless noted)
// Values verified against provider nginx configs and official documentation (Feb 2026).
const (
	RateLimitBscScan        = 0  // Deprecated — api.bscscan.com shut down Dec 18, 2025
	RateLimitBlockstream    = 5  // nginx heavylimitzone = 5r/s (was incorrectly set to 10)
	RateLimitMempool        = 3  // nginx 200r/m ≈ 3.3r/s (was incorrectly set to 10)
	RateLimitBlockchainInfo = 5
	RateLimitBitaps         = 5  // community estimate; no official rate limit docs
	RateLimitBSCRPC         = 10 // conservative for public BSC JSON-RPC nodes
	RateLimitLlamaNodes     = 50 // 50 req/s; no key, no registration required
	RateLimitDRPC           = 50 // conservative; actual free tier capacity is ~2,100 CUPS
	RateLimitNodeReal       = 10 // 150 CUPS free tier ≈ 10 req/s for non-trivial calls
	RateLimitSolanaRPC      = 10
	RateLimitHelius         = 10
	RateLimitAnkrSOL        = 30 // ~1,800 req/min guaranteed = 30 req/s
	RateLimitAlchemy        = 25 // 25 RPS on free tier
	RateLimitOnFinality     = 10 // conservative; no official docs
	RateLimitCoinGecko      = 30 // requests per minute — Demo plan = 30 RPM (was 10)
)

// Provider known monthly call limits (0 = no documented monthly cap).
// Used by the provider usage dashboard to show "X / Y this month" progress.
const (
	KnownMonthlyLimitBlockstream = int64(500_000)   // free unauthenticated tier
	KnownMonthlyLimitMempool     = int64(0)          // no monthly cap, only per-minute rate
	KnownMonthlyLimitBitaps      = int64(0)          // no documented monthly cap
	KnownMonthlyLimitHelius      = int64(1_000_000)  // credits/month (1 credit per std RPC call)
	KnownMonthlyLimitSolanaRPC   = int64(0)          // no monthly cap
	KnownMonthlyLimitCoinGecko   = int64(10_000)     // Demo plan: 10K calls/month
	KnownMonthlyLimitBSCRPC      = int64(0)          // public RPC — no monthly cap
	KnownMonthlyLimitLlamaNodes  = int64(0)          // no documented monthly cap
	KnownMonthlyLimitDRPC        = int64(0)          // no cap on no-key tier
	KnownMonthlyLimitNodeReal    = int64(10_000_000) // 10M CU/month free tier
	KnownMonthlyLimitAnkrSOL     = int64(0)          // no documented monthly cap
	KnownMonthlyLimitAlchemy     = int64(30_000_000) // 30M CU/month free tier
	KnownMonthlyLimitOnFinality  = int64(0)          // no documented monthly cap
)

// Transaction — General
const (
	BSCGasLimitTransfer = 21_000
	BSCGasLimitBEP20    = 65_000
	BSCGasPreSeedWei    = "5000000000000000" // 0.005 BNB
	SOLMaxInstructions  = 20
)

// SOL Transaction
const (
	SOLLamportsPerSOL           = 1_000_000_000       // 1 SOL = 10^9 lamports
	SOLBaseTransactionFee       = 5_000                // lamports per signature
	SOLMaxTxSize                = 1232                 // bytes — hard limit on Solana TX wire format
	SOLConfirmationTimeout      = 60 * time.Second     // max wait for tx confirmation
	SOLConfirmationPollInterval = 2 * time.Second      // poll interval for getSignatureStatuses
	SOLATARentLamports          = 2_039_280            // rent-exempt minimum for ATA creation (~0.00204 SOL)
	SOLSystemProgramID          = "11111111111111111111111111111111"
	SOLRentSysvarID             = "SysvarRent111111111111111111111111111111111"
)

// BSC Chain IDs
const (
	BSCMainnetChainID = 56
	BSCTestnetChainID = 97
)

// BSC Transaction
const (
	BSCGasPriceBufferNumerator   = 12 // Multiply gas price by 12/10 = 20% buffer
	BSCGasPriceBufferDenominator = 10
	BSCReceiptPollInterval       = 3 * time.Second
	BSCReceiptPollTimeout        = 120 * time.Second
)

// BEP-20 Transfer
const (
	BEP20TransferMethodID = "a9059cbb" // keccak256("transfer(address,uint256)")[:4]
)

// BTC Transaction Building
const (
	BTCDefaultFeeRate       = 10      // Fallback sat/vB if fee estimation fails
	BTCMinFeeRate           = 1       // Absolute minimum sat/vB
	BTCDustThresholdSats    = 546     // Minimum P2WPKH output value
	BTCMaxTxWeight          = 400_000 // Max standard transaction weight units
	BTCMaxInputsPerTx       = 500     // Practical input limit before hitting size
	BTCFeeSafetyMarginPct   = 2       // Percentage added to estimated fee to prevent underestimation
)

// BTC Transaction vsize estimation (weight units per BIP-141)
const (
	BTCTxOverheadWU        = 42  // version(16) + marker(1) + flag(1) + vinCount(4) + voutCount(4) + locktime(16)
	BTCP2WPKHInputNonWitWU = 164 // (outpoint(36) + scriptLen(1) + sequence(4)) × 4
	BTCP2WPKHInputWitWU    = 108 // stackCount(1) + sigLen(1) + sig(72) + pkLen(1) + pk(33)
	BTCP2WPKHOutputWU      = 124 // (value(8) + scriptLen(1) + script(22)) × 4
)

// BTC Fee Estimation
const (
	MempoolFeeEstimatePath = "/v1/fees/recommended"
	FeeEstimateTimeout     = 5 * time.Second
	FeeCacheTTL            = 2 * time.Minute
)

// Server
const (
	ServerPort           = 8080
	ServerReadTimeout    = 30 * time.Second
	ServerWriteTimeout   = 60 * time.Second
	ServerIdleTimeout    = 5 * time.Minute
	ServerMaxHeaderBytes = 1 << 20 // 1MB
	APITimeout           = 30 * time.Second
	SSEKeepAliveInterval = 15 * time.Second
	HealthCheckTimeout   = 10 * time.Second
)

// Logging
const (
	LogDir         = "./logs"
	LogFilePattern = "hdpay-%s-%s.log" // date (YYYY-MM-DD), level (info/warn/error/debug)
	LogMaxAgeDays  = 30
)

// Database
const (
	DBPath            = "./data/hdpay.sqlite"
	DBTestPath        = "./data/hdpay_test.sqlite"
	DBWALMode         = true
	DBBusyTimeout     = 5000 // milliseconds
	DBMaxOpenConns    = 25
	DBMaxIdleConns    = 5
	DBConnMaxLifetime = 5 * time.Minute
)

// Send / Execute
const (
	SendExecuteTimeout = 10 * time.Minute // max time for a full sweep execution
	TxSSEHubBuffer     = 64               // channel buffer for TX SSE events
)

// Graceful Shutdown
const (
	ShutdownTimeout = SendExecuteTimeout // match longest operation (10 min)
)

// Price Staleness
const (
	PriceStaleTolerance = 30 * time.Minute // max age for stale-but-serve
)

// Explorer URLs
const (
	ExplorerBTCMainnet  = "https://mempool.space/tx/"
	ExplorerBTCTestnet  = "https://mempool.space/testnet/tx/"
	ExplorerBSCMainnet  = "https://bscscan.com/tx/"
	ExplorerBSCTestnet  = "https://testnet.bscscan.com/tx/"
	ExplorerSOLMainnet  = "https://solscan.io/tx/"
	ExplorerSOLTestnet  = "https://solscan.io/tx/" // append ?cluster=testnet
)

// Circuit Breaker
const (
	CircuitBreakerThreshold   = 3               // consecutive failures to trip
	CircuitBreakerCooldown    = 30 * time.Second // time before half-open test
	CircuitBreakerHalfOpenMax = 1                // max requests in half-open
)

// Scanner Resilience
const (
	ExponentialBackoffBase = 1 * time.Second  // base delay for backoff when all providers fail
	ExponentialBackoffMax  = 30 * time.Second // max backoff cap
	MaxConsecutivePoolFails = 5               // max consecutive all-provider failures before stopping scan
)

// BTC Confirmation Polling
const (
	BTCConfirmationTimeout      = 10 * time.Minute // max wait for BTC TX to get 1 confirmation
	BTCConfirmationPollInterval = 15 * time.Second  // poll interval for Esplora /tx/{txid}/status
	BTCTxStatusPath             = "/tx/%s/status"   // Esplora endpoint format for TX status
)

// SOL Blockhash Cache
const (
	SOLBlockhashCacheTTL = 10 * time.Second // max age before fetching fresh blockhash (reduced from 20s for safety)
)

// SOL Block Production
var (
	// SOLBlocksPerSecondEstimate is the average Solana block production rate.
	// Used to estimate whether a cached blockhash has likely expired.
	SOLBlocksPerSecondEstimate float64 = 2.5
)

const (
	// SOLBlockhashSafetyMarginBlocks is the safety margin in blocks.
	// If estimated consumed blocks exceeds this, force a blockhash refresh.
	// Solana blockhashes are valid for ~150 blocks; we refresh well before that.
	SOLBlockhashSafetyMarginBlocks uint64 = 100
)

// SOL ATA Confirmation
const (
	SOLATAConfirmationTimeout      = 30 * time.Second // max wait for ATA to be visible after creation
	SOLATAConfirmationPollInterval = 2 * time.Second  // poll interval for GetAccountInfo(destATA)
)

// BTC UTXO Re-Validation (preview->execute divergence thresholds)
// Tightened from 20%/10% to 5%/3% to prevent silent value slippage.
const (
	BTCUTXOCountDivergenceThreshold = 0.05 // reject if UTXO count dropped >5%
	BTCUTXOValueDivergenceThreshold = 0.03 // reject if total value dropped >3%
)

// BSC Balance Recheck
const (
	BSCMinNativeSweepWei           = "100000000000000" // 0.0001 BNB — below this, skip address
	BEP20BalanceOfMethodID         = "70a08231"         // keccak256("balanceOf(address)")[:4]
	BSCTokenBalanceDivergenceRatio = 0.05               // skip address if on-chain balance dropped >5% from DB
)

// BSC Gas Price Re-Estimation
const (
	BSCGasPriceMaxIncreaseMultiplier = 2 // reject if gas price more than 2x preview
)

// Gas Pre-Seed Token Identifier
const (
	TokenGasPreSeed = "GAS_PRESEED" // token field in tx_state for gas pre-seed rows
)

// SOL Confirmation
const (
	SOLMaxConfirmationRPCErrors = 3 // consecutive RPC errors before marking TX as uncertain
)

// TX State Statuses
const (
	TxStatePending      = "pending"
	TxStateBroadcasting = "broadcasting"
	TxStateConfirming   = "confirming"
	TxStateConfirmed    = "confirmed"
	TxStateFailed       = "failed"
	TxStateUncertain    = "uncertain"
	TxStateDismissed    = "dismissed"
)

// TX Reconciler (startup reconciliation of pending transactions)
const (
	ReconcileMaxAge       = 1 * time.Hour    // Pending TXs older than this are marked uncertain
	ReconcileCheckTimeout = 10 * time.Second // Timeout per on-chain status check
)

// Provider Health Statuses
const (
	ProviderStatusHealthy  = "healthy"
	ProviderStatusDegraded = "degraded"
	ProviderStatusDown     = "down"
)

// Provider Types
const (
	ProviderTypeScan      = "scan"
	ProviderTypeBroadcast = "broadcast"
)

// Circuit States
const (
	CircuitClosed   = "closed"
	CircuitOpen     = "open"
	CircuitHalfOpen = "half_open"
)

// Token Decimals
const (
	BTCDecimals      = 8
	BNBDecimals      = 18
	SOLDecimals      = 9
	BSCUSDCDecimals  = 18 // BSC USDC is 18 decimals (not 6 like Ethereum)
	BSCUSDTDecimals  = 18
	SOLUSDCDecimals  = 6
	SOLUSDTDecimals  = 6
)

// Price
const (
	CoinGeckoBaseURL   = "https://api.coingecko.com/api/v3"
	CoinGeckoIDs       = "bitcoin,binancecoin,solana,usd-coin,tether"
	PriceCacheDuration = 5 * time.Minute
)
