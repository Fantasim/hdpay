package models

// Chain represents a supported blockchain.
type Chain string

const (
	ChainBTC Chain = "BTC"
	ChainBSC Chain = "BSC"
	ChainSOL Chain = "SOL"
)

// AllChains is the ordered list of supported chains.
var AllChains = []Chain{ChainBTC, ChainBSC, ChainSOL}

// NetworkMode represents mainnet or testnet operation.
type NetworkMode string

const (
	NetworkMainnet NetworkMode = "mainnet"
	NetworkTestnet NetworkMode = "testnet"
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

// AddressWithBalance represents an address with its balance data for API responses.
type AddressWithBalance struct {
	Chain         Chain              `json:"chain"`
	AddressIndex  int                `json:"addressIndex"`
	Address       string             `json:"address"`
	NativeBalance string             `json:"nativeBalance"`
	TokenBalances []TokenBalanceItem `json:"tokenBalances"`
	LastScanned   *string            `json:"lastScanned"`
}

// TokenBalanceItem represents a single token balance in an API response.
type TokenBalanceItem struct {
	Symbol          Token  `json:"symbol"`
	Balance         string `json:"balance"`
	ContractAddress string `json:"contractAddress"`
}

// AddressExport represents the JSON export format for a chain's addresses.
type AddressExport struct {
	Chain                  Chain               `json:"chain"`
	Network                string              `json:"network"`
	DerivationPathTemplate string              `json:"derivation_path_template"`
	GeneratedAt            string              `json:"generated_at"`
	Count                  int                 `json:"count"`
	Addresses              []AddressExportItem `json:"addresses"`
}

// AddressExportItem is a single address entry in the export file.
type AddressExportItem struct {
	Index   int    `json:"index"`
	Address string `json:"address"`
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

// UTXO represents an unspent transaction output.
type UTXO struct {
	TxID         string `json:"txid"`
	Vout         uint32 `json:"vout"`
	Value        int64  `json:"value"` // satoshis
	Confirmed    bool   `json:"confirmed"`
	BlockHeight  int64  `json:"blockHeight,omitempty"`
	Address      string `json:"address"`
	AddressIndex int    `json:"addressIndex"`
}

// FeeEstimate contains recommended fee rates from mempool.space.
type FeeEstimate struct {
	FastestFee  int64 `json:"fastestFee"`  // sat/vB, next block
	HalfHourFee int64 `json:"halfHourFee"` // sat/vB, ~3 blocks
	HourFee     int64 `json:"hourFee"`     // sat/vB, ~6 blocks
	EconomyFee  int64 `json:"economyFee"`  // sat/vB, several hours
	MinimumFee  int64 `json:"minimumFee"`  // sat/vB, eventually
}

// SendPreview contains the preview of a consolidation transaction.
type SendPreview struct {
	Chain          Chain  `json:"chain"`
	InputCount     int    `json:"inputCount"`
	TotalInputSats int64  `json:"totalInputSats"`
	OutputSats     int64  `json:"outputSats"`
	FeeSats        int64  `json:"feeSats"`
	FeeRate        int64  `json:"feeRate"`  // sat/vB
	EstimatedVsize int    `json:"estimatedVsize"`
	DestAddress    string `json:"destAddress"`
}

// SendResult contains the result of broadcasting a transaction.
type SendResult struct {
	TxHash string `json:"txHash"`
	Chain  Chain  `json:"chain"`
}

// BSCSendPreview contains the preview of a BSC consolidation transaction.
type BSCSendPreview struct {
	Chain       Chain  `json:"chain"`
	Token       Token  `json:"token"`
	InputCount  int    `json:"inputCount"`
	TotalAmount string `json:"totalAmount"` // wei or token smallest unit
	GasCostWei  string `json:"gasCostWei"`
	NetAmount   string `json:"netAmount"`  // totalAmount - gasCost (native only)
	DestAddress string `json:"destAddress"`
	GasPrice    string `json:"gasPrice"` // wei, for transparency
}

// BSCSendResult contains the result of a BSC consolidation sweep.
type BSCSendResult struct {
	Chain        Chain         `json:"chain"`
	Token        Token         `json:"token"`
	TxResults    []BSCTxResult `json:"txResults"`
	SuccessCount int           `json:"successCount"`
	FailCount    int           `json:"failCount"`
	TotalSwept   string        `json:"totalSwept"`
}

// BSCTxResult contains the result of a single BSC transaction within a sweep.
type BSCTxResult struct {
	AddressIndex int    `json:"addressIndex"`
	FromAddress  string `json:"fromAddress"`
	TxHash       string `json:"txHash"`
	Amount       string `json:"amount"`
	Status       string `json:"status"` // "confirmed", "reverted", "failed"
	Error        string `json:"error,omitempty"`
}

// GasPreSeedPreview contains the preview of a gas pre-seeding operation.
type GasPreSeedPreview struct {
	SourceIndex     int    `json:"sourceIndex"`
	SourceAddress   string `json:"sourceAddress"`
	SourceBalance   string `json:"sourceBalance"`   // wei
	TargetCount     int    `json:"targetCount"`
	AmountPerTarget string `json:"amountPerTarget"` // wei
	TotalNeeded     string `json:"totalNeeded"`     // wei (amount + gas)
	Sufficient      bool   `json:"sufficient"`
}

// GasPreSeedResult contains the result of a gas pre-seeding operation.
type GasPreSeedResult struct {
	TxResults    []BSCTxResult `json:"txResults"`
	SuccessCount int           `json:"successCount"`
	FailCount    int           `json:"failCount"`
	TotalSent    string        `json:"totalSent"` // wei
}

// SOLSendPreview contains the preview of a SOL consolidation sweep.
type SOLSendPreview struct {
	Chain           Chain  `json:"chain"`
	Token           Token  `json:"token"`
	InputCount      int    `json:"inputCount"`
	TotalAmount     string `json:"totalAmount"`     // lamports or token smallest unit
	TotalFee        string `json:"totalFee"`         // lamports
	NetAmount       string `json:"netAmount"`        // native only: totalAmount - totalFee
	DestAddress     string `json:"destAddress"`
	NeedATACreation bool   `json:"needATACreation"`  // SPL only
	ATARentCost     string `json:"ataRentCost"`      // lamports, if ATA creation needed
}

// SOLSendResult contains the result of a SOL consolidation sweep.
type SOLSendResult struct {
	Chain        Chain         `json:"chain"`
	Token        Token         `json:"token"`
	TxResults    []SOLTxResult `json:"txResults"`
	SuccessCount int           `json:"successCount"`
	FailCount    int           `json:"failCount"`
	TotalSwept   string        `json:"totalSwept"`
}

// SOLTxResult contains the result of a single SOL transaction within a sweep.
type SOLTxResult struct {
	AddressIndex int    `json:"addressIndex"`
	FromAddress  string `json:"fromAddress"`
	TxSignature  string `json:"txSignature"`
	Amount       string `json:"amount"`
	Status       string `json:"status"` // "confirmed", "failed"
	Slot         uint64 `json:"slot,omitempty"`
	Error        string `json:"error,omitempty"`
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
