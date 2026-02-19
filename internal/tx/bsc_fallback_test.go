package tx

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// mockBroadcastClient is a minimal mock for testing FallbackEthClient.
type mockBroadcastClient struct {
	sendErr       error
	sendCallCount int
	nonceResult   uint64
	nonceErr      error
	gasPrice      *big.Int
	gasPriceErr   error
}

func (m *mockBroadcastClient) SendTransaction(_ context.Context, _ *types.Transaction) error {
	m.sendCallCount++
	return m.sendErr
}

func (m *mockBroadcastClient) PendingNonceAt(_ context.Context, _ common.Address) (uint64, error) {
	return m.nonceResult, m.nonceErr
}

func (m *mockBroadcastClient) SuggestGasPrice(_ context.Context) (*big.Int, error) {
	if m.gasPrice != nil {
		return m.gasPrice, nil
	}
	return nil, m.gasPriceErr
}

func (m *mockBroadcastClient) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	return nil, ethereum.NotFound
}

func (m *mockBroadcastClient) BalanceAt(_ context.Context, _ common.Address, _ *big.Int) (*big.Int, error) {
	return big.NewInt(0), nil
}

func (m *mockBroadcastClient) CallContract(_ context.Context, _ ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	return nil, nil
}

func TestFallbackEthClient_PrimarySucceeds(t *testing.T) {
	primary := &mockBroadcastClient{sendErr: nil}
	fallback := &mockBroadcastClient{sendErr: nil}
	client := NewFallbackEthClient(primary, fallback)

	tx := types.NewTx(&types.LegacyTx{
		Nonce: 0,
		To:    &common.Address{},
		Value: big.NewInt(100),
		Gas:   21000,
	})

	err := client.SendTransaction(context.Background(), tx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if primary.sendCallCount != 1 {
		t.Errorf("expected primary called once, got %d", primary.sendCallCount)
	}
	if fallback.sendCallCount != 0 {
		t.Errorf("expected fallback not called, got %d", fallback.sendCallCount)
	}
}

func TestFallbackEthClient_PrimaryFailsFallbackSucceeds(t *testing.T) {
	primaryErr := errors.New("primary RPC down")
	primary := &mockBroadcastClient{sendErr: primaryErr}
	fallback := &mockBroadcastClient{sendErr: nil}
	client := NewFallbackEthClient(primary, fallback)

	tx := types.NewTx(&types.LegacyTx{
		Nonce: 0,
		To:    &common.Address{},
		Value: big.NewInt(100),
		Gas:   21000,
	})

	err := client.SendTransaction(context.Background(), tx)
	if err != nil {
		t.Fatalf("expected no error (fallback succeeds), got %v", err)
	}
	if primary.sendCallCount != 1 {
		t.Errorf("expected primary called once, got %d", primary.sendCallCount)
	}
	if fallback.sendCallCount != 1 {
		t.Errorf("expected fallback called once, got %d", fallback.sendCallCount)
	}
}

func TestFallbackEthClient_BothFail(t *testing.T) {
	primaryErr := errors.New("primary RPC down")
	fallbackErr := errors.New("fallback RPC down")
	primary := &mockBroadcastClient{sendErr: primaryErr}
	fallback := &mockBroadcastClient{sendErr: fallbackErr}
	client := NewFallbackEthClient(primary, fallback)

	tx := types.NewTx(&types.LegacyTx{
		Nonce: 0,
		To:    &common.Address{},
		Value: big.NewInt(100),
		Gas:   21000,
	})

	err := client.SendTransaction(context.Background(), tx)
	if err == nil {
		t.Fatal("expected error when both fail")
	}
	// Should return primary error.
	if err.Error() != primaryErr.Error() {
		t.Errorf("expected primary error %q, got %q", primaryErr, err)
	}
	if primary.sendCallCount != 1 {
		t.Errorf("expected primary called once, got %d", primary.sendCallCount)
	}
	if fallback.sendCallCount != 1 {
		t.Errorf("expected fallback called once, got %d", fallback.sendCallCount)
	}
}

func TestFallbackEthClient_NilFallback(t *testing.T) {
	primaryErr := errors.New("primary RPC down")
	primary := &mockBroadcastClient{sendErr: primaryErr}
	client := NewFallbackEthClient(primary, nil)

	tx := types.NewTx(&types.LegacyTx{
		Nonce: 0,
		To:    &common.Address{},
		Value: big.NewInt(100),
		Gas:   21000,
	})

	err := client.SendTransaction(context.Background(), tx)
	if err == nil {
		t.Fatal("expected error with nil fallback")
	}
	if err.Error() != primaryErr.Error() {
		t.Errorf("expected primary error %q, got %q", primaryErr, err)
	}
}

func TestFallbackEthClient_DelegatesToPrimary(t *testing.T) {
	primary := &mockBroadcastClient{
		nonceResult: 42,
		gasPrice:    big.NewInt(5000000000),
	}
	fallback := &mockBroadcastClient{
		nonceResult: 99, // should not be used
		gasPrice:    big.NewInt(9999),
	}
	client := NewFallbackEthClient(primary, fallback)

	// PendingNonceAt should delegate to primary.
	nonce, err := client.PendingNonceAt(context.Background(), common.Address{})
	if err != nil {
		t.Fatalf("PendingNonceAt error: %v", err)
	}
	if nonce != 42 {
		t.Errorf("expected nonce 42, got %d", nonce)
	}

	// SuggestGasPrice should delegate to primary.
	gp, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		t.Fatalf("SuggestGasPrice error: %v", err)
	}
	if gp.Cmp(big.NewInt(5000000000)) != 0 {
		t.Errorf("expected gas price 5000000000, got %s", gp)
	}
}
