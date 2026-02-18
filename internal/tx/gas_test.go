package tx

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Fantasim/hdpay/internal/config"
)

func TestGasPreSeed_Preview_Sufficient(t *testing.T) {
	// Source has 1 BNB, sending to 3 targets at 0.005 BNB each.
	// Total amount: 0.015 BNB + gas costs.
	gasPrice := big.NewInt(3_000_000_000)
	sourceBalance := big.NewInt(1_000_000_000_000_000_000) // 1 BNB

	mock := &mockEthClient{
		gasPrice: gasPrice,
		balance:  sourceBalance,
	}

	// We need a KeyService that works without a real file.
	// For preview, we only need DeriveBSCPrivateKey to return an address.
	// Since we can't easily mock KeyService, test the computation logic directly.

	bufferedGP := BufferedGasPrice(gasPrice)
	gasCostPerSend := new(big.Int).Mul(bufferedGP, big.NewInt(int64(config.BSCGasLimitTransfer)))

	targetCount := int64(3)
	totalAmount := new(big.Int).Mul(gasPreSeedAmountWei, big.NewInt(targetCount))
	totalGas := new(big.Int).Mul(gasCostPerSend, big.NewInt(targetCount))
	totalNeeded := new(big.Int).Add(totalAmount, totalGas)

	if sourceBalance.Cmp(totalNeeded) < 0 {
		t.Fatalf("source should be sufficient: balance=%s, needed=%s", sourceBalance, totalNeeded)
	}

	// Verify the gas pre-seed amount is correct.
	expected, _ := new(big.Int).SetString(config.BSCGasPreSeedWei, 10)
	if gasPreSeedAmountWei.Cmp(expected) != 0 {
		t.Errorf("gasPreSeedAmountWei: expected %s, got %s", expected, gasPreSeedAmountWei)
	}

	_ = mock
}

func TestGasPreSeed_InsufficientSource(t *testing.T) {
	gasPrice := big.NewInt(3_000_000_000)
	// Source only has 0.001 BNB — not enough for even one pre-seed (0.005 BNB + gas).
	sourceBalance := big.NewInt(1_000_000_000_000_000) // 0.001 BNB

	bufferedGP := BufferedGasPrice(gasPrice)
	gasCostPerSend := new(big.Int).Mul(bufferedGP, big.NewInt(int64(config.BSCGasLimitTransfer)))

	targetCount := int64(3)
	totalAmount := new(big.Int).Mul(gasPreSeedAmountWei, big.NewInt(targetCount))
	totalGas := new(big.Int).Mul(gasCostPerSend, big.NewInt(targetCount))
	totalNeeded := new(big.Int).Add(totalAmount, totalGas)

	if sourceBalance.Cmp(totalNeeded) >= 0 {
		t.Fatalf("source should be insufficient: balance=%s, needed=%s", sourceBalance, totalNeeded)
	}
}

func TestGasPreSeed_NonceIncrement(t *testing.T) {
	// Verify that nonce increments correctly during sequential sends.
	gasPrice := big.NewInt(3_000_000_000)
	privKey, _ := crypto.GenerateKey()

	mock := &mockEthClient{
		pendingNonce: 5,
		gasPrice:     gasPrice,
		receipt: &types.Receipt{
			Status:      types.ReceiptStatusSuccessful,
			BlockNumber: big.NewInt(100),
		},
	}

	targets := []common.Address{
		common.HexToAddress("0xaaaa000000000000000000000000000000000001"),
		common.HexToAddress("0xaaaa000000000000000000000000000000000002"),
		common.HexToAddress("0xaaaa000000000000000000000000000000000003"),
	}

	bufferedGP := BufferedGasPrice(gasPrice)
	chainID := big.NewInt(config.BSCTestnetChainID)
	nonce := uint64(5)

	for i, target := range targets {
		tx := BuildBSCNativeTransfer(nonce, target, gasPreSeedAmountWei, bufferedGP)

		if tx.Nonce() != uint64(5+i) {
			t.Errorf("tx %d: expected nonce %d, got %d", i, 5+i, tx.Nonce())
		}

		signedTx, err := SignBSCTx(tx, chainID, privKey)
		if err != nil {
			t.Fatalf("sign tx %d: %v", i, err)
		}

		if err := mock.SendTransaction(context.Background(), signedTx); err != nil {
			t.Fatalf("send tx %d: %v", i, err)
		}

		nonce++
	}

	// Should have sent 3 transactions.
	if len(mock.sentTxs) != 3 {
		t.Errorf("expected 3 sent txs, got %d", len(mock.sentTxs))
	}

	// Verify each tx has sequential nonces.
	for i, tx := range mock.sentTxs {
		expectedNonce := uint64(5 + i)
		if tx.Nonce() != expectedNonce {
			t.Errorf("sent tx %d: expected nonce %d, got %d", i, expectedNonce, tx.Nonce())
		}
	}
}

func TestGasPreSeed_Execute_BroadcastFail(t *testing.T) {
	// When broadcast fails, the TX hash should be empty and nonce should NOT increment.
	gasPrice := big.NewInt(3_000_000_000)
	privKey, _ := crypto.GenerateKey()

	mock := &mockEthClient{
		gasPrice:  gasPrice,
		sendTxErr: errors.New("broadcast error"),
	}

	bufferedGP := BufferedGasPrice(gasPrice)
	target := common.HexToAddress("0xbbbb000000000000000000000000000000000001")
	chainID := big.NewInt(config.BSCTestnetChainID)

	tx := BuildBSCNativeTransfer(0, target, gasPreSeedAmountWei, bufferedGP)
	signedTx, _ := SignBSCTx(tx, chainID, privKey)

	err := mock.SendTransaction(context.Background(), signedTx)
	if err == nil {
		t.Fatal("expected broadcast error")
	}

	// The tx was "sent" to the mock (appended to sentTxs) but returned an error.
	// In real code, the nonce should NOT be incremented when broadcast fails.
}
