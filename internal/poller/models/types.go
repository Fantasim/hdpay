package models

// WatchStatus represents the state of a watch.
type WatchStatus string

const (
	WatchStatusActive    WatchStatus = "ACTIVE"
	WatchStatusCompleted WatchStatus = "COMPLETED"
	WatchStatusExpired   WatchStatus = "EXPIRED"
	WatchStatusCancelled WatchStatus = "CANCELLED"
)

// TxStatus represents the confirmation state of a transaction.
type TxStatus string

const (
	TxStatusPending   TxStatus = "PENDING"
	TxStatusConfirmed TxStatus = "CONFIRMED"
)

// Watch represents an active or historical address watch.
type Watch struct {
	ID             string      `json:"id"`
	Chain          string      `json:"chain"`
	Address        string      `json:"address"`
	Status         WatchStatus `json:"status"`
	StartedAt      string      `json:"started_at"`
	ExpiresAt      string      `json:"expires_at"`
	CompletedAt    *string     `json:"completed_at,omitempty"`
	PollCount      int         `json:"poll_count"`
	LastPollAt     *string     `json:"last_poll_at,omitempty"`
	LastPollResult *string     `json:"last_poll_result,omitempty"`
	CreatedAt      string      `json:"created_at"`
}

// Transaction represents a detected blockchain transaction.
type Transaction struct {
	ID            int      `json:"id"`
	WatchID       string   `json:"watch_id"`
	TxHash        string   `json:"tx_hash"`
	Chain         string   `json:"chain"`
	Address       string   `json:"address"`
	Token         string   `json:"token"`
	AmountRaw     string   `json:"amount_raw"`
	AmountHuman   string   `json:"amount_human"`
	Decimals      int      `json:"decimals"`
	USDValue      float64  `json:"usd_value"`
	USDPrice      float64  `json:"usd_price"`
	Tier          int      `json:"tier"`
	Multiplier    float64  `json:"multiplier"`
	Points        int      `json:"points"`
	Status        TxStatus `json:"status"`
	Confirmations int      `json:"confirmations"`
	BlockNumber   *int64   `json:"block_number,omitempty"`
	DetectedAt    string   `json:"detected_at"`
	ConfirmedAt   *string  `json:"confirmed_at,omitempty"`
	CreatedAt     string   `json:"created_at"`
}

// PointsAccount represents the points ledger for an address on a chain.
type PointsAccount struct {
	Address   string `json:"address"`
	Chain     string `json:"chain"`
	Unclaimed int    `json:"unclaimed"`
	Pending   int    `json:"pending"`
	Total     int    `json:"total"`
	UpdatedAt string `json:"updated_at"`
}

// SystemError represents a system error or discrepancy.
type SystemError struct {
	ID        int    `json:"id"`
	Severity  string `json:"severity"`
	Category  string `json:"category"`
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
	Resolved  bool   `json:"resolved"`
	CreatedAt string `json:"created_at"`
}

// IPAllowEntry represents an allowed IP in the allowlist.
type IPAllowEntry struct {
	ID          int    `json:"id"`
	IP          string `json:"ip"`
	Description string `json:"description,omitempty"`
	AddedAt     string `json:"added_at"`
}

// Tier represents a points multiplier tier.
type Tier struct {
	MinUSD     float64  `json:"min_usd"`
	MaxUSD     *float64 `json:"max_usd"` // nil = unbounded
	Multiplier float64  `json:"multiplier"`
}

// WatchFilters contains filter parameters for listing watches.
type WatchFilters struct {
	Status *WatchStatus
	Chain  *string
}

// TransactionFilters contains filter parameters for listing transactions.
type TransactionFilters struct {
	Chain   *string
	Token   *string
	Status  *TxStatus
	Tier    *int
	MinUSD  *float64
	MaxUSD  *float64
	DateFrom *string
	DateTo   *string
}

// Pagination contains pagination parameters.
type Pagination struct {
	Page     int
	PageSize int
}

// DailyStatRow contains aggregated transaction data for a single day.
type DailyStatRow struct {
	Date    string  `json:"date"`
	USD     float64 `json:"usd"`
	Points  int     `json:"points"`
	TxCount int     `json:"txs"`
}

// ChainBreakdown contains aggregated data for a single chain.
type ChainBreakdown struct {
	Chain string  `json:"chain"`
	USD   float64 `json:"usd"`
	Count int     `json:"count"`
}

// TokenBreakdown contains aggregated data for a single token.
type TokenBreakdown struct {
	Token string  `json:"token"`
	USD   float64 `json:"usd"`
	Count int     `json:"count"`
}

// TierBreakdown contains aggregated data for a single tier.
type TierBreakdown struct {
	Tier        int `json:"tier"`
	Count       int `json:"count"`
	TotalPoints int `json:"total_points"`
}

// DailyWatchStat contains watch counts for a single day.
type DailyWatchStat struct {
	Date      string `json:"date"`
	Active    int    `json:"active"`
	Completed int    `json:"completed"`
	Expired   int    `json:"expired"`
}

// DiscrepancyRow represents a detected data discrepancy.
type DiscrepancyRow struct {
	Type       string `json:"type"`
	Address    string `json:"address,omitempty"`
	Chain      string `json:"chain,omitempty"`
	Message    string `json:"message"`
	Calculated int    `json:"calculated,omitempty"`
	Stored     int    `json:"stored,omitempty"`
}

// StalePendingRow represents a transaction stuck in PENDING state.
type StalePendingRow struct {
	TxHash       string  `json:"tx_hash"`
	Chain        string  `json:"chain"`
	Address      string  `json:"address"`
	DetectedAt   string  `json:"detected_at"`
	HoursPending float64 `json:"hours_pending"`
}
