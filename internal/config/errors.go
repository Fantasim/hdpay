package config

import "errors"

// Sentinel errors for internal use.
var (
	ErrInvalidMnemonic    = errors.New("invalid mnemonic")
	ErrProviderRateLimit  = errors.New("provider rate limit exceeded")
	ErrProviderUnavailable = errors.New("provider unavailable")
	ErrTokensNotSupported = errors.New("tokens not supported by this provider")
	ErrScanAlreadyRunning = errors.New("scan already running for this chain")
	ErrScanInterrupted    = errors.New("scan interrupted")
	ErrInsufficientGas    = errors.New("insufficient gas for transaction")
	ErrTransactionFailed  = errors.New("transaction broadcast failed")
)

// Error codes — shared with frontend via API responses.
const (
	ErrorInvalidMnemonic    = "ERROR_INVALID_MNEMONIC"
	ErrorAddressGeneration  = "ERROR_ADDRESS_GENERATION"
	ErrorDatabase           = "ERROR_DATABASE"
	ErrorScanFailed         = "ERROR_SCAN_FAILED"
	ErrorScanInterrupted    = "ERROR_SCAN_INTERRUPTED"
	ErrorProviderRateLimit  = "ERROR_PROVIDER_RATE_LIMIT"
	ErrorProviderUnavailable = "ERROR_PROVIDER_UNAVAILABLE"
	ErrorInsufficientBalance = "ERROR_INSUFFICIENT_BALANCE"
	ErrorInsufficientGas    = "ERROR_INSUFFICIENT_GAS"
	ErrorTxBuildFailed      = "ERROR_TX_BUILD_FAILED"
	ErrorTxSignFailed       = "ERROR_TX_SIGN_FAILED"
	ErrorTxBroadcastFailed  = "ERROR_TX_BROADCAST_FAILED"
	ErrorInvalidAddress     = "ERROR_INVALID_ADDRESS"
	ErrorInvalidChain       = "ERROR_INVALID_CHAIN"
	ErrorInvalidToken       = "ERROR_INVALID_TOKEN"
	ErrorExportFailed       = "ERROR_EXPORT_FAILED"
	ErrorPriceFetchFailed   = "ERROR_PRICE_FETCH_FAILED"
	ErrorInvalidConfig      = "ERROR_INVALID_CONFIG"
)
