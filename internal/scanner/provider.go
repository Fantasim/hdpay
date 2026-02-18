package scanner

import (
	"context"

	"github.com/Fantasim/hdpay/internal/models"
)

// BalanceResult holds a balance value for a single address.
type BalanceResult struct {
	Address      string
	AddressIndex int
	Balance      string // raw balance string (satoshis, wei, lamports)
	Error        string // non-empty if balance is unreliable
	Source       string // provider name that returned this result
}

// Provider fetches balance data from an external blockchain API.
type Provider interface {
	// Name returns the provider's display name (e.g. "Blockstream", "BscScan").
	Name() string

	// Chain returns which blockchain this provider serves.
	Chain() models.Chain

	// MaxBatchSize returns the maximum number of addresses per API call.
	MaxBatchSize() int

	// FetchNativeBalances returns native token balances for a batch of addresses.
	// The returned slice may be shorter than input if some addresses fail.
	FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error)

	// FetchTokenBalances returns token balances for a batch of addresses.
	// contractOrMint is the token contract (BSC) or mint address (SOL).
	// BTC providers should return config.ErrTokensNotSupported.
	FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractOrMint string) ([]BalanceResult, error)
}
