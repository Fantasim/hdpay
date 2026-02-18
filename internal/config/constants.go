package config

import "time"

// Address Generation
const (
	MaxAddressesPerChain = 500_000
	DefaultMaxScanID     = 5_000
)

// BIP-44 Derivation Paths
const (
	BIP44Purpose    = 44
	BTCCoinType     = 0   // m/44'/0'/0'/0/N
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

// Scanning
const (
	ScanBatchSizeBscScan    = 20  // BscScan multi-address balance
	ScanBatchSizeSolanaRPC  = 100 // getMultipleAccounts limit
	ScanBatchSizeBlockchain = 50  // Blockchain.info multiaddr
	ScanResumeThreshold     = 24 * time.Hour
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
	PriceCacheDuration = 5 * time.Minute
)
