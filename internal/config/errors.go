package config

import (
	"errors"
	"time"
)

// Sentinel errors for internal use.
var (
	ErrInvalidMnemonic     = errors.New("invalid mnemonic")
	ErrProviderRateLimit   = errors.New("provider rate limit exceeded")
	ErrProviderUnavailable = errors.New("provider unavailable")
	ErrTokensNotSupported  = errors.New("tokens not supported by this provider")
	ErrScanAlreadyRunning  = errors.New("scan already running for this chain")
	ErrScanInterrupted     = errors.New("scan interrupted")
	ErrInsufficientGas     = errors.New("insufficient gas for transaction")
	ErrTransactionFailed   = errors.New("transaction broadcast failed")
	ErrPriceFetchFailed    = errors.New("price fetch failed")
	ErrUTXOFetchFailed     = errors.New("UTXO fetch failed")
	ErrFeeEstimateFailed   = errors.New("fee estimation failed")
	ErrInsufficientUTXO    = errors.New("insufficient UTXO value to cover fee")
	ErrTxTooLarge          = errors.New("transaction exceeds maximum weight")
	ErrDustOutput          = errors.New("output below dust threshold")
	ErrMnemonicFileNotSet   = errors.New("mnemonic file path not configured")
	ErrKeyDerivation        = errors.New("key derivation failed")
	ErrNonceTooLow          = errors.New("nonce too low")
	ErrTxReverted           = errors.New("transaction reverted")
	ErrInsufficientBNBForGas = errors.New("insufficient BNB for gas")
	ErrGasPreSeedFailed     = errors.New("gas pre-seed failed")
	ErrReceiptTimeout       = errors.New("receipt polling timeout")

	// BTC Confirmation
	ErrBTCConfirmationTimeout = errors.New("BTC transaction confirmation timeout")

	// SOL
	ErrSOLTxTooLarge            = errors.New("SOL transaction exceeds 1232 byte limit")
	ErrSOLConfirmationTimeout   = errors.New("SOL transaction confirmation timeout")
	ErrSOLConfirmationUncertain = errors.New("SOL confirmation uncertain due to RPC errors")
	ErrSOLTxFailed              = errors.New("SOL transaction failed on-chain")
	ErrSOLInsufficientLamports  = errors.New("insufficient lamports to cover transaction fee")
	ErrSOLATACreationFailed     = errors.New("failed to create associated token account")
	ErrSOLBlockhashExpired      = errors.New("recent blockhash expired")

	// Send
	ErrNoFundedAddresses = errors.New("no funded addresses found")
	ErrInvalidDestination = errors.New("invalid destination address")
	ErrSendInProgress    = errors.New("send operation already in progress")

	// Circuit Breaker
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// Provider
	ErrProviderTimeout    = errors.New("provider request timeout")
	ErrPartialResults     = errors.New("partial results returned")
	ErrAllProvidersFailed = errors.New("all providers failed")
)

// TransientError wraps an error that should be retried.
type TransientError struct {
	Err        error
	RetryAfter time.Duration // 0 = use default backoff
}

func (e *TransientError) Error() string { return e.Err.Error() }
func (e *TransientError) Unwrap() error { return e.Err }

// NewTransientError wraps an error as transient (retriable).
func NewTransientError(err error) error {
	return &TransientError{Err: err}
}

// NewTransientErrorWithRetry wraps with explicit retry delay.
func NewTransientErrorWithRetry(err error, retryAfter time.Duration) error {
	return &TransientError{Err: err, RetryAfter: retryAfter}
}

// IsTransient returns true if the error is transient (retriable).
func IsTransient(err error) bool {
	var te *TransientError
	return errors.As(err, &te)
}

// GetRetryAfter returns the retry delay if set, or 0.
func GetRetryAfter(err error) time.Duration {
	var te *TransientError
	if errors.As(err, &te) {
		return te.RetryAfter
	}
	return 0
}

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
	ErrorUTXOFetchFailed    = "ERROR_UTXO_FETCH_FAILED"
	ErrorFeeEstimateFailed  = "ERROR_FEE_ESTIMATE_FAILED"
	ErrorInsufficientUTXO   = "ERROR_INSUFFICIENT_UTXO"
	ErrorTxTooLarge         = "ERROR_TX_TOO_LARGE"
	ErrorNonceTooLow        = "ERROR_NONCE_TOO_LOW"
	ErrorTxReverted         = "ERROR_TX_REVERTED"
	ErrorReceiptTimeout     = "ERROR_RECEIPT_TIMEOUT"
	ErrorGasPreSeedFailed   = "ERROR_GAS_PRESEED_FAILED"

	// BTC
	ErrorBTCConfirmationTimeout = "ERROR_BTC_CONFIRMATION_TIMEOUT"

	// SOL
	ErrorSOLTxTooLarge            = "ERROR_SOL_TX_TOO_LARGE"
	ErrorSOLConfirmationTimeout   = "ERROR_SOL_CONFIRMATION_TIMEOUT"
	ErrorSOLConfirmationUncertain = "ERROR_SOL_CONFIRMATION_UNCERTAIN"
	ErrorSOLTxFailed              = "ERROR_SOL_TX_FAILED"
	ErrorSOLInsufficientLamports  = "ERROR_SOL_INSUFFICIENT_LAMPORTS"
	ErrorSOLATACreationFailed     = "ERROR_SOL_ATA_CREATION_FAILED"

	// Send
	ErrorNoFundedAddresses  = "ERROR_NO_FUNDED_ADDRESSES"
	ErrorInvalidDestination = "ERROR_INVALID_DESTINATION"
	ErrorSendInProgress     = "ERROR_SEND_IN_PROGRESS"
	ErrorSendBusy           = "ERROR_SEND_BUSY"

	// Circuit Breaker
	ErrorCircuitOpen = "ERROR_CIRCUIT_OPEN"

	// Provider
	ErrorProviderTimeout    = "ERROR_PROVIDER_TIMEOUT"
	ErrorPartialResults     = "ERROR_PARTIAL_RESULTS"
	ErrorAllProvidersFailed = "ERROR_ALL_PROVIDERS_FAILED"
	ErrorTokenScanFailed    = "ERROR_TOKEN_SCAN_FAILED"
)
