package models

// Chain represents a supported blockchain.
type Chain string

const (
	ChainBTC Chain = "BTC"
	ChainBSC Chain = "BSC"
	ChainSOL Chain = "SOL"
)

// Token represents a supported token symbol.
type Token string

const (
	TokenNative Token = "NATIVE"
	TokenUSDC   Token = "USDC"
	TokenUSDT   Token = "USDT"
)

// Address represents a derived HD wallet address.
type Address struct {
	Chain        Chain  `json:"chain"`
	AddressIndex int    `json:"addressIndex"`
	Address      string `json:"address"`
	CreatedAt    string `json:"createdAt"`
}

// Balance represents the balance of an address for a specific token.
type Balance struct {
	Chain        Chain  `json:"chain"`
	AddressIndex int    `json:"addressIndex"`
	Token        Token  `json:"token"`
	Balance      string `json:"balance"`
	LastScanned  string `json:"lastScanned,omitempty"`
}

// ScanState represents the scanning progress for a chain.
type ScanState struct {
	Chain            Chain  `json:"chain"`
	LastScannedIndex int    `json:"lastScannedIndex"`
	MaxScanID        int    `json:"maxScanId"`
	Status           string `json:"status"`
	StartedAt        string `json:"startedAt,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
}

// Transaction represents a recorded transaction.
type Transaction struct {
	ID           int    `json:"id"`
	Chain        Chain  `json:"chain"`
	AddressIndex int    `json:"addressIndex"`
	TxHash       string `json:"txHash"`
	Direction    string `json:"direction"`
	Token        Token  `json:"token"`
	Amount       string `json:"amount"`
	FromAddress  string `json:"fromAddress"`
	ToAddress    string `json:"toAddress"`
	BlockNumber  *int   `json:"blockNumber,omitempty"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
	ConfirmedAt  string `json:"confirmedAt,omitempty"`
}

// APIResponse is the standard API response wrapper.
type APIResponse struct {
	Data interface{} `json:"data,omitempty"`
	Meta *APIMeta    `json:"meta,omitempty"`
}

// APIMeta contains pagination and execution metadata.
type APIMeta struct {
	Page          int   `json:"page,omitempty"`
	PageSize      int   `json:"pageSize,omitempty"`
	Total         int64 `json:"total,omitempty"`
	ExecutionTime int64 `json:"executionTime,omitempty"`
}

// APIError is the standard error response.
type APIError struct {
	Error APIErrorDetail `json:"error"`
}

// APIErrorDetail contains error code and message.
type APIErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
