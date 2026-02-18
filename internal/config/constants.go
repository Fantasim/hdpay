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

// Token Contract Addresses — SOL Devnet
const (
	SOLDevnetUSDCMint = "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU" // Official Circle devnet USDC
	SOLDevnetUSDTMint = ""                                                // No official devnet USDT exists
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

// Scanner Orchestrator
const (
	ScanProgressBroadcastInterval = 500 * time.Millisecond
	ProviderRequestTimeout        = 15 * time.Second
	ProviderMaxRetries            = 3
	ProviderRetryBaseDelay        = 1 * time.Second
	SSEHubChannelBuffer           = 64
)

// Provider URLs — BTC Mainnet
const (
	BlockstreamMainnetURL = "https://blockstream.info/api"
	MempoolMainnetURL     = "https://mempool.space/api"
)

// Provider URLs — BTC Testnet
const (
	BlockstreamTestnetURL = "https://blockstream.info/testnet/api"
	MempoolTestnetURL     = "https://mempool.space/testnet/api"
)

// Provider URLs — BSC
const (
	BscScanAPIURL     = "https://api.bscscan.com/api"
	BscRPCMainnetURL  = "https://bsc-dataseed.binance.org"
	BscRPCMainnetURL2 = "https://rpc.ankr.com/bsc"
	BscRPCTestnetURL  = "https://data-seed-prebsc-1-s1.binance.org:8545"
	BscScanTestnetURL = "https://api-testnet.bscscan.com/api"
)

// Provider URLs — Solana
const (
	SolanaMainnetRPCURL = "https://api.mainnet-beta.solana.com"
	HeliusMainnetRPCURL = "https://mainnet.helius-rpc.com"
	SolanaDevnetRPCURL  = "https://api.devnet.solana.com"
)

// Solana Program IDs
const (
	SOLTokenProgramID           = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	SOLAssociatedTokenProgramID = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"
)

// Rate Limiting (requests per second unless noted)
const (
	RateLimitBscScan       = 5
	RateLimitBlockstream   = 10
	RateLimitMempool       = 10
	RateLimitBlockchainInfo = 5
	RateLimitSolanaRPC     = 10
	RateLimitHelius        = 10
	RateLimitCoinGecko     = 10 // requests per minute
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
	BTCDefaultFeeRate    = 10      // Fallback sat/vB if fee estimation fails
	BTCMinFeeRate        = 1       // Absolute minimum sat/vB
	BTCDustThresholdSats = 546     // Minimum P2WPKH output value
	BTCMaxTxWeight       = 400_000 // Max standard transaction weight units
	BTCMaxInputsPerTx    = 500     // Practical input limit before hitting size
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
)

// Server
const (
	ServerPort         = 8080
	ServerReadTimeout  = 30 * time.Second
	ServerWriteTimeout = 60 * time.Second
	APITimeout         = 30 * time.Second
	SSEKeepAliveInterval = 15 * time.Second
)

// Logging
const (
	LogDir         = "./logs"
	LogFilePattern = "hdpay-%s.log" // %s = YYYY-MM-DD
	LogMaxAgeDays  = 30
)

// Database
const (
	DBPath        = "./data/hdpay.sqlite"
	DBTestPath    = "./data/hdpay_test.sqlite"
	DBWALMode     = true
	DBBusyTimeout = 5000 // milliseconds
)

// Send / Execute
const (
	SendExecuteTimeout = 10 * time.Minute // max time for a full sweep execution
	TxSSEHubBuffer     = 64               // channel buffer for TX SSE events
)

// Explorer URLs
const (
	ExplorerBTCMainnet  = "https://mempool.space/tx/"
	ExplorerBTCTestnet  = "https://mempool.space/testnet/tx/"
	ExplorerBSCMainnet  = "https://bscscan.com/tx/"
	ExplorerBSCTestnet  = "https://testnet.bscscan.com/tx/"
	ExplorerSOLMainnet  = "https://solscan.io/tx/"
	ExplorerSOLDevnet   = "https://solscan.io/tx/" // append ?cluster=devnet
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

// TX State Statuses
const (
	TxStatePending      = "pending"
	TxStateBroadcasting = "broadcasting"
	TxStateConfirming   = "confirming"
	TxStateConfirmed    = "confirmed"
	TxStateFailed       = "failed"
	TxStateUncertain    = "uncertain"
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
