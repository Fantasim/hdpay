package tx

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// --- Mock EthClient ---

type mockEthClient struct {
	pendingNonce    uint64
	pendingNonceErr error
	gasPrice        *big.Int
	gasPriceErr     error
	sendTxErr       error
	receipt         *types.Receipt
	receiptErr      error
	balance         *big.Int
	balanceErr      error
	callResult      []byte
	callErr         error

	// Track calls for assertions
	sentTxs []*types.Transaction
}

func (m *mockEthClient) PendingNonceAt(_ context.Context, _ common.Address) (uint64, error) {
	return m.pendingNonce, m.pendingNonceErr
}

func (m *mockEthClient) SuggestGasPrice(_ context.Context) (*big.Int, error) {
	if m.gasPriceErr != nil {
		return nil, m.gasPriceErr
	}
	return new(big.Int).Set(m.gasPrice), nil
}

func (m *mockEthClient) SendTransaction(_ context.Context, tx *types.Transaction) error {
	m.sentTxs = append(m.sentTxs, tx)
	return m.sendTxErr
}

func (m *mockEthClient) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	if m.receiptErr != nil {
		return nil, m.receiptErr
	}
	return m.receipt, nil
}

func (m *mockEthClient) BalanceAt(_ context.Context, _ common.Address, _ *big.Int) (*big.Int, error) {
	if m.balanceErr != nil {
		return nil, m.balanceErr
	}
	return new(big.Int).Set(m.balance), nil
}

func (m *mockEthClient) CallContract(_ context.Context, _ ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if m.callErr != nil {
		return nil, m.callErr
	}
	return m.callResult, nil
}

// --- Tests ---

func TestEncodeBEP20Transfer(t *testing.T) {
	to := common.HexToAddress("0x1234567890AbcdEF1234567890aBcdef12345678")
	amount := big.NewInt(1_000_000) // 1 USDC (6 decimals)

	data := EncodeBEP20Transfer(to, amount)

	// Should be exactly 68 bytes: 4 selector + 32 address + 32 amount
	if len(data) != 68 {
		t.Fatalf("expected 68 bytes, got %d", len(data))
	}

	// Check function selector (first 4 bytes)
	if data[0] != 0xa9 || data[1] != 0x05 || data[2] != 0x9c || data[3] != 0xbb {
		t.Errorf("wrong function selector: %x", data[:4])
	}

	// Check address is in bytes 4-36 (left-padded to 32 bytes)
	// The address should be in the last 20 bytes of the 32-byte field
	for i := 4; i < 16; i++ {
		if data[i] != 0 {
			t.Errorf("address padding byte %d should be 0, got %d", i, data[i])
		}
	}

	// Check amount is in bytes 36-68
	// Amount 1_000_000 = 0xF4240
	amountBytes := data[36:68]
	parsedAmount := new(big.Int).SetBytes(amountBytes)
	if parsedAmount.Cmp(amount) != 0 {
		t.Errorf("amount mismatch: expected %s, got %s", amount, parsedAmount)
	}
}

func TestEncodeBEP20Transfer_LargeAmount(t *testing.T) {
	to := common.HexToAddress("0xdead000000000000000000000000000000000000")
	// 1 USDT (18 decimals) = 10^18
	amount, _ := new(big.Int).SetString("1000000000000000000", 10)

	data := EncodeBEP20Transfer(to, amount)

	if len(data) != 68 {
		t.Fatalf("expected 68 bytes, got %d", len(data))
	}

	parsedAmount := new(big.Int).SetBytes(data[36:68])
	if parsedAmount.Cmp(amount) != 0 {
		t.Errorf("amount mismatch: expected %s, got %s", amount, parsedAmount)
	}
}

func TestBuildBSCNativeTransfer(t *testing.T) {
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	amount := big.NewInt(5_000_000_000_000_000) // 0.005 BNB
	gasPrice := big.NewInt(3_000_000_000)        // 3 Gwei
	nonce := uint64(42)

	tx := BuildBSCNativeTransfer(nonce, to, amount, gasPrice)

	if tx.Nonce() != nonce {
		t.Errorf("nonce: expected %d, got %d", nonce, tx.Nonce())
	}
	if tx.To() == nil || *tx.To() != to {
		t.Errorf("to: expected %s, got %v", to.Hex(), tx.To())
	}
	if tx.Value().Cmp(amount) != 0 {
		t.Errorf("value: expected %s, got %s", amount, tx.Value())
	}
	if tx.GasPrice().Cmp(gasPrice) != 0 {
		t.Errorf("gasPrice: expected %s, got %s", gasPrice, tx.GasPrice())
	}
	if tx.Gas() != config.BSCGasLimitTransfer {
		t.Errorf("gas: expected %d, got %d", config.BSCGasLimitTransfer, tx.Gas())
	}
	if len(tx.Data()) != 0 {
		t.Errorf("data should be nil for native transfer, got %d bytes", len(tx.Data()))
	}
}

func TestBuildBSCTokenTransfer(t *testing.T) {
	contract := common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d")
	recipient := common.HexToAddress("0x2222222222222222222222222222222222222222")
	amount := big.NewInt(1_000_000) // 1 USDC
	gasPrice := big.NewInt(3_000_000_000)
	nonce := uint64(0)

	tx := BuildBSCTokenTransfer(nonce, contract, recipient, amount, gasPrice)

	if tx.To() == nil || *tx.To() != contract {
		t.Errorf("to should be contract address %s, got %v", contract.Hex(), tx.To())
	}
	if tx.Value().Sign() != 0 {
		t.Errorf("value should be 0 for token transfer, got %s", tx.Value())
	}
	if tx.Gas() != config.BSCGasLimitBEP20 {
		t.Errorf("gas: expected %d, got %d", config.BSCGasLimitBEP20, tx.Gas())
	}
	if len(tx.Data()) != 68 {
		t.Errorf("data should be 68 bytes, got %d", len(tx.Data()))
	}
}

func TestSignBSCTx(t *testing.T) {
	// Generate a random key for testing.
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	to := common.HexToAddress("0x3333333333333333333333333333333333333333")
	unsignedTx := BuildBSCNativeTransfer(0, to, big.NewInt(1000), big.NewInt(3_000_000_000))

	chainID := big.NewInt(config.BSCTestnetChainID)
	signedTx, err := SignBSCTx(unsignedTx, chainID, privKey)
	if err != nil {
		t.Fatalf("SignBSCTx: %v", err)
	}

	// Recover the signer address to verify.
	signer := types.NewEIP155Signer(chainID)
	sender, err := types.Sender(signer, signedTx)
	if err != nil {
		t.Fatalf("recover sender: %v", err)
	}

	expectedAddr := crypto.PubkeyToAddress(privKey.PublicKey)
	if sender != expectedAddr {
		t.Errorf("sender mismatch: expected %s, got %s", expectedAddr.Hex(), sender.Hex())
	}

	// Verify the chain ID is embedded (EIP-155).
	v, _, _ := signedTx.RawSignatureValues()
	// For EIP-155: v = {0,1} + chainID*2 + 35
	minV := new(big.Int).Mul(chainID, big.NewInt(2))
	minV.Add(minV, big.NewInt(35))
	if v.Cmp(minV) < 0 {
		t.Errorf("v value %s too low for chain ID %s (expected >= %s)", v, chainID, minV)
	}
}

func TestSignBSCTx_MainnetChainID(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	to := common.HexToAddress("0x4444444444444444444444444444444444444444")
	unsignedTx := BuildBSCNativeTransfer(0, to, big.NewInt(1000), big.NewInt(5_000_000_000))

	chainID := big.NewInt(config.BSCMainnetChainID)
	signedTx, err := SignBSCTx(unsignedTx, chainID, privKey)
	if err != nil {
		t.Fatalf("SignBSCTx: %v", err)
	}

	signer := types.NewEIP155Signer(chainID)
	sender, err := types.Sender(signer, signedTx)
	if err != nil {
		t.Fatalf("recover sender: %v", err)
	}

	expectedAddr := crypto.PubkeyToAddress(privKey.PublicKey)
	if sender != expectedAddr {
		t.Errorf("sender mismatch: expected %s, got %s", expectedAddr.Hex(), sender.Hex())
	}
}

func TestBufferedGasPrice(t *testing.T) {
	suggested := big.NewInt(3_000_000_000) // 3 Gwei
	buffered := BufferedGasPrice(suggested)

	// 3 Gwei * 12/10 = 3.6 Gwei
	expected := big.NewInt(3_600_000_000)
	if buffered.Cmp(expected) != 0 {
		t.Errorf("buffered gas price: expected %s, got %s", expected, buffered)
	}
}

func TestBSCChainID(t *testing.T) {
	mainnet := BSCChainID("mainnet")
	if mainnet.Int64() != config.BSCMainnetChainID {
		t.Errorf("mainnet chain ID: expected %d, got %d", config.BSCMainnetChainID, mainnet.Int64())
	}

	testnet := BSCChainID("testnet")
	if testnet.Int64() != config.BSCTestnetChainID {
		t.Errorf("testnet chain ID: expected %d, got %d", config.BSCTestnetChainID, testnet.Int64())
	}
}

func TestWaitForReceipt_Success(t *testing.T) {
	mock := &mockEthClient{
		receipt: &types.Receipt{
			Status:      types.ReceiptStatusSuccessful,
			BlockNumber: big.NewInt(12345),
			GasUsed:     21000,
		},
	}

	ctx := context.Background()
	txHash := common.HexToHash("0xabc123")

	receipt, err := WaitForReceipt(ctx, mock, txHash)
	if err != nil {
		t.Fatalf("WaitForReceipt: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Errorf("expected successful receipt, got status %d", receipt.Status)
	}
}

func TestWaitForReceipt_Reverted(t *testing.T) {
	mock := &mockEthClient{
		receipt: &types.Receipt{
			Status:      types.ReceiptStatusFailed,
			BlockNumber: big.NewInt(12345),
		},
	}

	ctx := context.Background()
	txHash := common.HexToHash("0xdef456")

	_, err := WaitForReceipt(ctx, mock, txHash)
	if err == nil {
		t.Fatal("expected error for reverted TX")
	}
	if !errors.Is(err, config.ErrTxReverted) {
		t.Errorf("expected ErrTxReverted, got: %v", err)
	}
}

func TestWaitForReceipt_Timeout(t *testing.T) {
	mock := &mockEthClient{
		receiptErr: ethereum.NotFound,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	txHash := common.HexToHash("0x789abc")

	_, err := WaitForReceipt(ctx, mock, txHash)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, config.ErrReceiptTimeout) {
		t.Errorf("expected ErrReceiptTimeout, got: %v", err)
	}
}

func TestVerifyBSCAddress(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(privKey.PublicKey)

	if !VerifyBSCAddress(privKey, addr.Hex()) {
		t.Error("expected address verification to pass")
	}

	if VerifyBSCAddress(privKey, "0x0000000000000000000000000000000000000000") {
		t.Error("expected address verification to fail for wrong address")
	}
}

func TestBSCConsolidation_NativeSweep(t *testing.T) {
	balance := big.NewInt(1_000_000_000_000_000_000) // 1 BNB
	gasPrice := big.NewInt(3_000_000_000)             // 3 Gwei

	// Generate a test private key.
	privKey, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(privKey.PublicKey)

	mock := &mockEthClient{
		pendingNonce: 0,
		gasPrice:     gasPrice,
		balance:      balance,
		receipt: &types.Receipt{
			Status:      types.ReceiptStatusSuccessful,
			BlockNumber: big.NewInt(100),
		},
	}

	// We can't easily test with KeyService (needs mnemonic file).
	// Instead, test the building + signing flow directly.
	bufferedGP := BufferedGasPrice(gasPrice)
	gasCost := new(big.Int).Mul(bufferedGP, big.NewInt(int64(config.BSCGasLimitTransfer)))
	sendAmount := new(big.Int).Sub(balance, gasCost)

	dest := common.HexToAddress("0x5555555555555555555555555555555555555555")
	unsignedTx := BuildBSCNativeTransfer(0, dest, sendAmount, bufferedGP)

	chainID := big.NewInt(config.BSCTestnetChainID)
	signedTx, err := SignBSCTx(unsignedTx, chainID, privKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Verify the transaction properties.
	if signedTx.Value().Cmp(sendAmount) != 0 {
		t.Errorf("value: expected %s, got %s", sendAmount, signedTx.Value())
	}

	// Broadcast and check receipt.
	if err := mock.SendTransaction(context.Background(), signedTx); err != nil {
		t.Fatalf("send: %v", err)
	}

	receipt, err := WaitForReceipt(context.Background(), mock, signedTx.Hash())
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Errorf("expected successful receipt")
	}

	// Verify sender.
	signer := types.NewEIP155Signer(chainID)
	sender, err := types.Sender(signer, signedTx)
	if err != nil {
		t.Fatalf("recover sender: %v", err)
	}
	if sender != addr {
		t.Errorf("sender: expected %s, got %s", addr.Hex(), sender.Hex())
	}

	_ = mock // mock was used
}

func TestBSCConsolidation_TokenSweep_Flow(t *testing.T) {
	gasPrice := big.NewInt(3_000_000_000)
	privKey, _ := crypto.GenerateKey()

	// Build a BEP-20 transfer.
	contract := common.HexToAddress(config.BSCUSDCContract)
	dest := common.HexToAddress("0x6666666666666666666666666666666666666666")
	amount := big.NewInt(1_000_000) // 1 USDC

	bufferedGP := BufferedGasPrice(gasPrice)
	unsignedTx := BuildBSCTokenTransfer(0, contract, dest, amount, bufferedGP)

	chainID := big.NewInt(config.BSCMainnetChainID)
	signedTx, err := SignBSCTx(unsignedTx, chainID, privKey)
	if err != nil {
		t.Fatalf("sign token tx: %v", err)
	}

	// Verify tx properties.
	if *signedTx.To() != contract {
		t.Errorf("to: expected contract %s, got %s", contract.Hex(), signedTx.To().Hex())
	}
	if signedTx.Value().Sign() != 0 {
		t.Errorf("value should be 0 for token transfer")
	}
	if signedTx.Gas() != config.BSCGasLimitBEP20 {
		t.Errorf("gas: expected %d, got %d", config.BSCGasLimitBEP20, signedTx.Gas())
	}
	if len(signedTx.Data()) != 68 {
		t.Errorf("data: expected 68 bytes, got %d", len(signedTx.Data()))
	}
}

func TestCheckGasForTokenSweep_AllHaveGas(t *testing.T) {
	gasPrice := big.NewInt(3_000_000_000)
	bufferedGP := BufferedGasPrice(gasPrice)
	gasCostPerTx := new(big.Int).Mul(bufferedGP, big.NewInt(int64(config.BSCGasLimitBEP20)))

	// Balance higher than gas cost.
	mock := &mockEthClient{
		balance: new(big.Int).Add(gasCostPerTx, big.NewInt(1)),
	}

	svc := &BSCConsolidationService{ethClient: mock}

	addresses := []models.AddressWithBalance{
		{Address: "0x1111111111111111111111111111111111111111", AddressIndex: 0},
		{Address: "0x2222222222222222222222222222222222222222", AddressIndex: 1},
	}

	needsGas, err := svc.checkGasForTokenSweep(context.Background(), addresses, gasCostPerTx)
	if err != nil {
		t.Fatalf("checkGasForTokenSweep: %v", err)
	}
	if len(needsGas) != 0 {
		t.Errorf("expected 0 addresses needing gas, got %d", len(needsGas))
	}
}

func TestCheckGasForTokenSweep_SomeNeedGas(t *testing.T) {
	gasCostPerTx := big.NewInt(234_000_000_000_000) // ~0.000234 BNB

	mock := &mockEthClient{
		balance: big.NewInt(0), // All have zero BNB
	}

	svc := &BSCConsolidationService{ethClient: mock}

	addresses := []models.AddressWithBalance{
		{Address: "0x1111111111111111111111111111111111111111", AddressIndex: 0},
		{Address: "0x2222222222222222222222222222222222222222", AddressIndex: 1},
	}

	needsGas, err := svc.checkGasForTokenSweep(context.Background(), addresses, gasCostPerTx)
	if err != nil {
		t.Fatalf("checkGasForTokenSweep: %v", err)
	}
	if len(needsGas) != 2 {
		t.Errorf("expected 2 addresses needing gas, got %d", len(needsGas))
	}
}

// --- Task 2: BSC Balance Recheck Tests (A7) ---

func TestBalanceOfBEP20_Success(t *testing.T) {
	// Simulate a balanceOf call returning 1,000,000 (1 USDC, 6 decimals).
	expected := big.NewInt(1_000_000)
	result := common.LeftPadBytes(expected.Bytes(), 32)

	mock := &mockEthClient{
		callResult: result,
	}

	contract := common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d")
	owner := common.HexToAddress("0x1111111111111111111111111111111111111111")

	balance, err := BalanceOfBEP20(context.Background(), mock, contract, owner)
	if err != nil {
		t.Fatalf("BalanceOfBEP20: %v", err)
	}
	if balance.Cmp(expected) != 0 {
		t.Errorf("expected %s, got %s", expected, balance)
	}
}

func TestBalanceOfBEP20_CallError(t *testing.T) {
	mock := &mockEthClient{
		callErr: errors.New("rpc error"),
	}

	contract := common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d")
	owner := common.HexToAddress("0x1111111111111111111111111111111111111111")

	_, err := BalanceOfBEP20(context.Background(), mock, contract, owner)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBalanceOfBEP20_ShortResult(t *testing.T) {
	mock := &mockEthClient{
		callResult: []byte{0x01, 0x02}, // Only 2 bytes, not 32
	}

	contract := common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d")
	owner := common.HexToAddress("0x1111111111111111111111111111111111111111")

	_, err := BalanceOfBEP20(context.Background(), mock, contract, owner)
	if err == nil {
		t.Fatal("expected error for short result")
	}
}

func TestBalanceOfBEP20_ZeroBalance(t *testing.T) {
	result := make([]byte, 32) // All zeros = 0 balance

	mock := &mockEthClient{
		callResult: result,
	}

	contract := common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d")
	owner := common.HexToAddress("0x1111111111111111111111111111111111111111")

	balance, err := BalanceOfBEP20(context.Background(), mock, contract, owner)
	if err != nil {
		t.Fatalf("BalanceOfBEP20: %v", err)
	}
	if balance.Sign() != 0 {
		t.Errorf("expected zero balance, got %s", balance)
	}
}

// --- Task 6: BSC Gas Re-Estimation Tests (A11) ---

func TestValidateGasPriceAgainstPreview_WithinTolerance(t *testing.T) {
	current := big.NewInt(3_600_000_000)   // 3.6 Gwei
	expected := "3000000000"                // 3 Gwei — current is 1.2x, within 2x

	err := ValidateGasPriceAgainstPreview(current, expected)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateGasPriceAgainstPreview_Spiked(t *testing.T) {
	current := big.NewInt(7_000_000_000)   // 7 Gwei
	expected := "3000000000"                // 3 Gwei — current is 2.33x, exceeds 2x

	err := ValidateGasPriceAgainstPreview(current, expected)
	if err == nil {
		t.Fatal("expected gas price spike error")
	}
	if !errors.Is(err, config.ErrGasPriceSpiked) {
		t.Errorf("expected ErrGasPriceSpiked, got: %v", err)
	}
}

func TestValidateGasPriceAgainstPreview_ExactlyDouble(t *testing.T) {
	current := big.NewInt(6_000_000_000)   // 6 Gwei
	expected := "3000000000"                // 3 Gwei — exactly 2x, should be OK

	err := ValidateGasPriceAgainstPreview(current, expected)
	if err != nil {
		t.Errorf("expected no error at exactly 2x, got: %v", err)
	}
}

func TestValidateGasPriceAgainstPreview_EmptyExpected(t *testing.T) {
	current := big.NewInt(100_000_000_000) // Huge gas price

	// Empty expected should skip validation.
	if err := ValidateGasPriceAgainstPreview(current, ""); err != nil {
		t.Errorf("expected no error with empty expected, got: %v", err)
	}

	// Zero expected should skip validation.
	if err := ValidateGasPriceAgainstPreview(current, "0"); err != nil {
		t.Errorf("expected no error with zero expected, got: %v", err)
	}
}

func TestBSCMinNativeSweepThreshold(t *testing.T) {
	// Verify the minimum sweep threshold is correctly parsed.
	expected, _ := new(big.Int).SetString(config.BSCMinNativeSweepWei, 10)
	if bscMinNativeSweepWei.Cmp(expected) != 0 {
		t.Errorf("bscMinNativeSweepWei: expected %s, got %s", expected, bscMinNativeSweepWei)
	}

	// Should be 0.0001 BNB = 100,000,000,000,000 wei
	if bscMinNativeSweepWei.Sign() <= 0 {
		t.Error("bscMinNativeSweepWei should be positive")
	}
}

// verifyPrivKeyMatchesExpected is a helper that generates a private key and returns it
// along with its address — used to simulate known key derivation in tests.
func generateTestKey(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key, crypto.PubkeyToAddress(key.PublicKey)
}
