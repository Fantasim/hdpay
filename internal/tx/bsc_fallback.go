package tx

import (
	"context"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// FallbackEthClient wraps a primary and fallback EthClientWrapper.
// For SendTransaction, it tries the primary first, then falls back to the secondary.
// All other methods delegate to the primary client only.
type FallbackEthClient struct {
	primary  EthClientWrapper
	fallback EthClientWrapper
}

// NewFallbackEthClient creates a client that falls back on broadcast failure.
// If fallback is nil, behaves identically to primary.
func NewFallbackEthClient(primary, fallback EthClientWrapper) *FallbackEthClient {
	slog.Info("BSC fallback eth client created",
		"hasFallback", fallback != nil,
	)
	return &FallbackEthClient{
		primary:  primary,
		fallback: fallback,
	}
}

// SendTransaction tries the primary RPC first, falls back on failure.
func (f *FallbackEthClient) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	err := f.primary.SendTransaction(ctx, tx)
	if err == nil {
		return nil
	}

	if f.fallback == nil {
		return err
	}

	slog.Warn("BSC primary broadcast failed, trying fallback RPC",
		"txHash", tx.Hash().Hex(),
		"primaryError", err,
	)

	fallbackErr := f.fallback.SendTransaction(ctx, tx)
	if fallbackErr == nil {
		slog.Info("BSC fallback broadcast succeeded",
			"txHash", tx.Hash().Hex(),
		)
		return nil
	}

	slog.Error("BSC fallback broadcast also failed",
		"txHash", tx.Hash().Hex(),
		"primaryError", err,
		"fallbackError", fallbackErr,
	)

	// Return the primary error since it's more likely to be informative.
	return err
}

// PendingNonceAt delegates to the primary client.
func (f *FallbackEthClient) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return f.primary.PendingNonceAt(ctx, account)
}

// SuggestGasPrice delegates to the primary client.
func (f *FallbackEthClient) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return f.primary.SuggestGasPrice(ctx)
}

// TransactionReceipt delegates to the primary client.
func (f *FallbackEthClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return f.primary.TransactionReceipt(ctx, txHash)
}

// BalanceAt delegates to the primary client.
func (f *FallbackEthClient) BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return f.primary.BalanceAt(ctx, account, blockNumber)
}

// CallContract delegates to the primary client.
func (f *FallbackEthClient) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return f.primary.CallContract(ctx, msg, blockNumber)
}
