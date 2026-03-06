package config

import "time"

// Version
const Version = "1.0.0"

// Polling Intervals — BSC/SOL at 15s balances API efficiency against detection latency.
// BSC block time ~3s, SOL ~0.4s — polling faster than 10s wastes free-tier API quota.
const (
	PollIntervalBTC = 60 * time.Second
	PollIntervalBSC = 15 * time.Second
	PollIntervalSOL = 15 * time.Second
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
	RecoveryTimeout         = 5 * time.Minute // max time for entire recovery process
	StalePendingThreshold   = 24 * time.Hour
)

// Graceful Shutdown
const (
	ShutdownTimeout          = 10 * time.Second
	WatchContextGracePeriod  = 5 * time.Second // extra time beyond watch expiry for goroutine cleanup
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

// Dashboard Date Ranges (in days, negative for past)
const (
	DateRangeWeekDays    = -7
	DateRangeMonthDays   = -30
	DateRangeQuarterDays = -90
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

// Supported Chains — single source of truth for chain iteration.
var SupportedChains = []string{"BTC", "BSC", "SOL"}

// Claim
const (
	MaxClaimBatchSize = 500 // max addresses per claim request
)

// BSC Provider
const (
	BSCBlockTimeSeconds = 3      // BSC produces a block ~every 3 seconds
	BSCMaxLogBlockRange = 50_000 // cap for eth_getLogs fromBlock range (~41h at 3s blocks)
	BNBSyntheticHashFmt = "bnb-%s-block-%d" // format for synthetic BNB balance-delta tx hashes
)

// SOL Provider
const (
	SOLSignaturePageSize = 20          // getSignaturesForAddress page size
	SOLMaxSignaturePages = 10          // max pages to paginate (200 sigs safety cap)
	SOLFetchCommitment   = "confirmed" // commitment level for fetching (not confirmation)
	SOLMaxTxVersion      = 0           // maxSupportedTransactionVersion for getTransaction
)

// Adaptive Polling — reduces API waste on idle watches by backing off the poll interval.
const (
	AdaptiveEmptyThreshold = 5 // consecutive empty ticks before first backoff (2x)
	AdaptiveMaxMultiplier  = 4 // max interval multiplier (4x base interval)
)

// Smart Confirmation Scheduling — minimum elapsed time before first confirmation
// recheck, based on chain block time × required confirmations.
// Slightly less than theoretical minimum to avoid missing fast blocks.
const (
	ConfirmationMinWaitBTC = 8 * time.Minute  // ~10 min block time × 1 conf, check 2 min early
	ConfirmationMinWaitBSC = 30 * time.Second  // ~3s block time × 12 conf = 36s, check 6s early
	ConfirmationMinWaitSOL = 10 * time.Second  // finalization ~13s, check 3s early
)

// Per-Watch Error Tracking
const (
	WatchMaxConsecutiveErrors = 10 // consecutive fetch failures before logging system error
)

// Dashboard
const (
	MaxDashboardDailyRows = 366 // max rows for daily-aggregation queries (prevents unbounded results)
)

// Recovery
const (
	RecoveryPerTxTimeout = 2 * time.Minute // max time per pending tx during recovery
)

// Orphan Recovery — periodic background check for PENDING txs from expired/completed watches.
const (
	OrphanRecoveryInterval = 5 * time.Minute // how often to check for orphaned pending txs
)

// Provider Usage — DB-backed API call tracking with daily granularity.
const (
	ProviderUsageDateFormat = "2006-01-02" // Go date format for YYYY-MM-DD
	ProviderUsageMonthDays  = 30           // days to aggregate for monthly stats
)
