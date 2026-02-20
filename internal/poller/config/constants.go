package config

import "time"

// Polling Intervals
const (
	PollIntervalBTC = 60 * time.Second
	PollIntervalBSC = 5 * time.Second
	PollIntervalSOL = 5 * time.Second
)

// Confirmation Thresholds
const (
	ConfirmationsBTC = 1
	ConfirmationsBSC = 12
	SOLCommitment    = "finalized"
)

// Watch Defaults
const (
	DefaultWatchTimeoutMinutes = 30
	MaxWatchTimeoutMinutes     = 120
	MaxActiveWatches           = 100
)

// Session
const (
	SessionCookieName  = "poller_session"
	SessionTimeout     = 1 * time.Hour
	SessionTokenLength = 32 // bytes, hex-encoded = 64 chars
)

// Database
const (
	PollerDBPath      = "./data/poller.sqlite"
	PollerDBTestPath  = "./data/poller_test.sqlite"
	PollerDBWALMode   = true
	PollerDBBusyTimeout = 5000 // milliseconds
)

// Price
const (
	PriceCacheDuration = 60 * time.Second
	PriceRetryCount    = 3
	PriceRetryDelay    = 5 * time.Second
	StablecoinPrice    = 1.0 // USDC and USDT assumed $1.00
)

// CoinGecko IDs
const (
	CoinGeckoIDBTC = "bitcoin"
	CoinGeckoIDBNB = "binancecoin"
	CoinGeckoIDSOL = "solana"
)

// Logging
const (
	PollerLogFilePattern = "poller-%s-%s.log" // date (YYYY-MM-DD), level (info/warn/error/debug)
	PollerLogPrefix      = "poller-"
)

// Tiers
const (
	TiersConfigFile = "./tiers.json"
	MinTierCount    = 2 // At least one "ignore" tier + one earning tier
)

// Recovery
const (
	RecoveryPendingRetries  = 3
	RecoveryPendingInterval = 30 * time.Second
	StalePendingThreshold   = 24 * time.Hour
)

// Graceful Shutdown
const (
	ShutdownTimeout = 10 * time.Second
)

// Pagination
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// Server
const (
	PollerServerPort     = 8081
	ServerReadTimeout    = 30 * time.Second
	ServerWriteTimeout   = 60 * time.Second
	APITimeout           = 30 * time.Second
)

// Provider Error Categories (for system_errors table)
const (
	ErrorCategoryProvider = "provider"
	ErrorCategoryWatcher  = "watcher"
)

// Provider Error Severities
const (
	ErrorSeverityWarn     = "warn"
	ErrorSeverityError    = "error"
	ErrorSeverityCritical = "critical"
)
