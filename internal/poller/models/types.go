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
