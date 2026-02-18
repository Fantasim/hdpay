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

// Token Contract Addresses — BSC Testnet
const (
	BSCTestnetUSDCContract = "" // Set when available
	BSCTestnetUSDTContract = "" // Set when available
)

// Token Contract Addresses — SOL Devnet
const (
	SOLDevnetUSDCMint = "" // Set when available
	SOLDevnetUSDTMint = "" // Set when available
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

// Transaction
const (
	BTCFeeRateSatPerVByte = 10
	BSCGasLimitTransfer   = 21_000
	BSCGasLimitBEP20      = 65_000
	BSCGasPreSeedWei      = "5000000000000000" // 0.005 BNB
	SOLMaxInstructions    = 20
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

// Price
const (
	CoinGeckoBaseURL   = "https://api.coingecko.com/api/v3"
	CoinGeckoIDs       = "bitcoin,binancecoin,solana,usd-coin,tether"
	PriceCacheDuration = 5 * time.Minute
)
