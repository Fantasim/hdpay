package tx

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// EthClientWrapper defines the minimal ethclient interface needed for BSC transactions.
// This allows mocking in tests.
type EthClientWrapper interface {
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
}

// bep20TransferSelector is the 4-byte function selector for transfer(address,uint256).
var bep20TransferSelector = func() []byte {
	b, _ := hex.DecodeString(config.BEP20TransferMethodID)
	return b
}()

// EncodeBEP20Transfer encodes a BEP-20 transfer(address,uint256) call.
// Returns 68 bytes: 4-byte selector + 32-byte padded address + 32-byte padded amount.
func EncodeBEP20Transfer(to common.Address, amount *big.Int) []byte {
	data := make([]byte, 0, 68)
	data = append(data, bep20TransferSelector...)
	data = append(data, common.LeftPadBytes(to.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(amount.Bytes(), 32)...)
	return data
}

// BufferedGasPrice applies the BSC gas price buffer (20% increase) to a suggested gas price.
func BufferedGasPrice(suggested *big.Int) *big.Int {
	buffered := new(big.Int).Mul(suggested, big.NewInt(int64(config.BSCGasPriceBufferNumerator)))
	buffered.Div(buffered, big.NewInt(int64(config.BSCGasPriceBufferDenominator)))
	return buffered
}

// BSCChainID returns the correct chain ID for the given network.
func BSCChainID(network string) *big.Int {
	if network == string(models.NetworkTestnet) {
		return big.NewInt(config.BSCTestnetChainID)
	}
	return big.NewInt(config.BSCMainnetChainID)
}

// BuildBSCNativeTransfer builds an unsigned native BNB transfer transaction.
func BuildBSCNativeTransfer(nonce uint64, to common.Address, amount *big.Int, gasPrice *big.Int) *types.Transaction {
	return types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    amount,
		Gas:      config.BSCGasLimitTransfer,
		GasPrice: gasPrice,
		Data:     nil,
	})
}

// BuildBSCTokenTransfer builds an unsigned BEP-20 token transfer transaction.
// The To address is the token contract. Value is 0 (not sending BNB).
func BuildBSCTokenTransfer(nonce uint64, contractAddr common.Address, recipient common.Address, amount *big.Int, gasPrice *big.Int) *types.Transaction {
	data := EncodeBEP20Transfer(recipient, amount)

	toAddr := contractAddr
	return types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &toAddr,
		Value:    big.NewInt(0),
		Gas:      config.BSCGasLimitBEP20,
		GasPrice: gasPrice,
		Data:     data,
	})
}

// SignBSCTx signs a BSC transaction with EIP-155 replay protection.
func SignBSCTx(tx *types.Transaction, chainID *big.Int, privKey *ecdsa.PrivateKey) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(chainID)
	signed, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		return nil, fmt.Errorf("sign BSC transaction: %w", err)
	}
	return signed, nil
}

// WaitForReceipt polls for a transaction receipt until mined, reverted, or timeout.
func WaitForReceipt(ctx context.Context, client EthClientWrapper, txHash common.Hash) (*types.Receipt, error) {
	slog.Debug("waiting for receipt", "txHash", txHash.Hex())

	pollCtx, cancel := context.WithTimeout(ctx, config.BSCReceiptPollTimeout)
	defer cancel()

	for {
		receipt, err := client.TransactionReceipt(pollCtx, txHash)
		if err == nil {
			slog.Info("receipt received",
				"txHash", txHash.Hex(),
				"status", receipt.Status,
				"blockNumber", receipt.BlockNumber,
				"gasUsed", receipt.GasUsed,
			)

			if receipt.Status == types.ReceiptStatusFailed {
				return receipt, fmt.Errorf("%w: tx %s reverted in block %d",
					config.ErrTxReverted, txHash.Hex(), receipt.BlockNumber.Uint64())
			}

			return receipt, nil
		}

		if !errors.Is(err, ethereum.NotFound) {
			return nil, fmt.Errorf("query receipt for %s: %w", txHash.Hex(), err)
		}

		// Not mined yet — wait and retry.
		select {
		case <-pollCtx.Done():
			return nil, fmt.Errorf("%w: tx %s not mined within timeout", config.ErrReceiptTimeout, txHash.Hex())
		case <-time.After(config.BSCReceiptPollInterval):
			slog.Debug("receipt not ready, polling again", "txHash", txHash.Hex())
		}
	}
}

// BSCConsolidationService orchestrates BSC native and token sweeps.
type BSCConsolidationService struct {
	keyService *KeyService
	ethClient  EthClientWrapper
	database   *db.DB
	chainID    *big.Int
}

// NewBSCConsolidationService creates the BSC consolidation orchestrator.
func NewBSCConsolidationService(
	keyService *KeyService,
	ethClient EthClientWrapper,
	database *db.DB,
	chainID *big.Int,
) *BSCConsolidationService {
	slog.Info("BSC consolidation service created", "chainID", chainID)
	return &BSCConsolidationService{
		keyService: keyService,
		ethClient:  ethClient,
		database:   database,
		chainID:    chainID,
	}
}

// EstimateGasPrice returns the buffered gas price for BSC transactions.
func (s *BSCConsolidationService) EstimateGasPrice(ctx context.Context) (*big.Int, error) {
	suggested, err := s.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	buffered := BufferedGasPrice(suggested)
	slog.Debug("BSC gas price estimated",
		"suggested", suggested.String(),
		"buffered", buffered.String(),
	)
	return buffered, nil
}

// PreviewNativeSweep calculates the expected result of a native BNB consolidation.
func (s *BSCConsolidationService) PreviewNativeSweep(ctx context.Context, addresses []models.AddressWithBalance, destAddr string) (*models.BSCSendPreview, error) {
	slog.Info("BSC native sweep preview",
		"addressCount", len(addresses),
		"destAddress", destAddr,
	)

	gasPrice, err := s.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	gasPrice = BufferedGasPrice(gasPrice)

	gasCostPerTx := new(big.Int).Mul(gasPrice, big.NewInt(int64(config.BSCGasLimitTransfer)))
	totalGasCost := new(big.Int).Mul(gasCostPerTx, big.NewInt(int64(len(addresses))))

	totalBalance := new(big.Int)
	inputCount := 0
	for _, addr := range addresses {
		bal, ok := new(big.Int).SetString(addr.NativeBalance, 10)
		if !ok || bal.Sign() <= 0 {
			continue
		}
		// Only count addresses where balance > gasCost
		if bal.Cmp(gasCostPerTx) > 0 {
			totalBalance.Add(totalBalance, bal)
			inputCount++
		}
	}

	netAmount := new(big.Int).Sub(totalBalance, new(big.Int).Mul(gasCostPerTx, big.NewInt(int64(inputCount))))
	if netAmount.Sign() < 0 {
		netAmount = big.NewInt(0)
	}

	preview := &models.BSCSendPreview{
		Chain:       models.ChainBSC,
		Token:       models.TokenNative,
		InputCount:  inputCount,
		TotalAmount: totalBalance.String(),
		GasCostWei:  totalGasCost.String(),
		NetAmount:   netAmount.String(),
		DestAddress: destAddr,
		GasPrice:    gasPrice.String(),
	}

	slog.Info("BSC native sweep preview complete",
		"inputCount", preview.InputCount,
		"totalAmount", preview.TotalAmount,
		"gasCost", preview.GasCostWei,
		"netAmount", preview.NetAmount,
	)

	return preview, nil
}

// ExecuteNativeSweep performs sequential BNB transfers from funded addresses to a destination.
func (s *BSCConsolidationService) ExecuteNativeSweep(ctx context.Context, addresses []models.AddressWithBalance, destAddr string) (*models.BSCSendResult, error) {
	slog.Info("BSC native sweep execute",
		"addressCount", len(addresses),
		"destAddress", destAddr,
	)
	start := time.Now()

	dest := common.HexToAddress(destAddr)

	// Get gas price once for all transactions.
	gasPrice, err := s.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	gasPrice = BufferedGasPrice(gasPrice)
	gasCostPerTx := new(big.Int).Mul(gasPrice, big.NewInt(int64(config.BSCGasLimitTransfer)))

	slog.Info("BSC sweep gas price",
		"gasPrice", gasPrice,
		"gasCostPerTx", gasCostPerTx,
	)

	result := &models.BSCSendResult{
		Chain:   models.ChainBSC,
		Token:   models.TokenNative,
	}
	totalSwept := new(big.Int)

	for _, addr := range addresses {
		if err := ctx.Err(); err != nil {
			slog.Warn("BSC native sweep cancelled", "error", err)
			break
		}

		txResult := s.sweepNativeAddress(ctx, addr, dest, gasPrice, gasCostPerTx)
		result.TxResults = append(result.TxResults, txResult)

		if txResult.Status == "confirmed" {
			result.SuccessCount++
			amount, _ := new(big.Int).SetString(txResult.Amount, 10)
			if amount != nil {
				totalSwept.Add(totalSwept, amount)
			}
		} else {
			result.FailCount++
		}
	}

	result.TotalSwept = totalSwept.String()

	slog.Info("BSC native sweep complete",
		"successCount", result.SuccessCount,
		"failCount", result.FailCount,
		"totalSwept", result.TotalSwept,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return result, nil
}

// sweepNativeAddress sends BNB from a single address to the destination.
func (s *BSCConsolidationService) sweepNativeAddress(
	ctx context.Context,
	addr models.AddressWithBalance,
	dest common.Address,
	gasPrice *big.Int,
	gasCostPerTx *big.Int,
) models.BSCTxResult {
	txResult := models.BSCTxResult{
		AddressIndex: addr.AddressIndex,
		FromAddress:  addr.Address,
	}

	fromAddr := common.HexToAddress(addr.Address)

	// Get real-time balance.
	balance, err := s.ethClient.BalanceAt(ctx, fromAddr, nil)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("get balance: %s", err)
		slog.Error("BSC sweep: failed to get balance", "address", addr.Address, "error", err)
		return txResult
	}

	sendAmount := new(big.Int).Sub(balance, gasCostPerTx)
	if sendAmount.Sign() <= 0 {
		txResult.Status = "failed"
		txResult.Error = "balance too low to cover gas"
		slog.Warn("BSC sweep: insufficient balance for gas",
			"address", addr.Address,
			"balance", balance,
			"gasCost", gasCostPerTx,
		)
		return txResult
	}

	// Derive private key.
	privKey, derivedAddr, err := s.keyService.DeriveBSCPrivateKey(ctx, uint32(addr.AddressIndex))
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("derive key: %s", err)
		slog.Error("BSC sweep: key derivation failed", "index", addr.AddressIndex, "error", err)
		return txResult
	}

	// Verify derived address matches.
	if derivedAddr != fromAddr {
		txResult.Status = "failed"
		txResult.Error = "derived address mismatch"
		slog.Error("BSC sweep: address mismatch",
			"expected", addr.Address,
			"derived", derivedAddr.Hex(),
			"index", addr.AddressIndex,
		)
		return txResult
	}

	// Get nonce.
	nonce, err := s.ethClient.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("get nonce: %s", err)
		slog.Error("BSC sweep: failed to get nonce", "address", addr.Address, "error", err)
		return txResult
	}

	// Build and sign.
	unsignedTx := BuildBSCNativeTransfer(nonce, dest, sendAmount, gasPrice)
	signedTx, err := SignBSCTx(unsignedTx, s.chainID, privKey)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("sign tx: %s", err)
		slog.Error("BSC sweep: sign failed", "address", addr.Address, "error", err)
		return txResult
	}

	slog.Info("BSC sweep: broadcasting native transfer",
		"from", addr.Address,
		"to", dest.Hex(),
		"amount", sendAmount,
		"nonce", nonce,
	)

	// Broadcast.
	if err := s.ethClient.SendTransaction(ctx, signedTx); err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("broadcast: %s", err)
		slog.Error("BSC sweep: broadcast failed", "address", addr.Address, "error", err)
		return txResult
	}

	txHash := signedTx.Hash()
	txResult.TxHash = txHash.Hex()
	txResult.Amount = sendAmount.String()

	slog.Info("BSC sweep: TX broadcast, waiting for receipt",
		"txHash", txHash.Hex(),
		"from", addr.Address,
	)

	// Wait for receipt.
	receipt, err := WaitForReceipt(ctx, s.ethClient, txHash)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("receipt: %s", err)
		slog.Error("BSC sweep: receipt failed", "txHash", txHash.Hex(), "error", err)
		// TX was broadcast — record it even if receipt polling fails.
		s.recordBSCTransaction(addr, txHash.Hex(), sendAmount.String(), dest.Hex(), models.TokenNative, "pending")
		return txResult
	}

	txResult.Status = "confirmed"

	// Record in DB.
	s.recordBSCTransaction(addr, txHash.Hex(), sendAmount.String(), dest.Hex(), models.TokenNative, "confirmed")

	slog.Info("BSC sweep: native transfer confirmed",
		"txHash", txHash.Hex(),
		"from", addr.Address,
		"amount", sendAmount,
		"block", receipt.BlockNumber,
	)

	return txResult
}

// ExecuteTokenSweep performs sequential BEP-20 token transfers from funded addresses.
func (s *BSCConsolidationService) ExecuteTokenSweep(
	ctx context.Context,
	addresses []models.AddressWithBalance,
	destAddr string,
	token models.Token,
	contractAddr string,
) (*models.BSCSendResult, error) {
	slog.Info("BSC token sweep execute",
		"addressCount", len(addresses),
		"destAddress", destAddr,
		"token", token,
		"contract", contractAddr,
	)
	start := time.Now()

	dest := common.HexToAddress(destAddr)
	contract := common.HexToAddress(contractAddr)

	// Get gas price once.
	gasPrice, err := s.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	gasPrice = BufferedGasPrice(gasPrice)
	gasCostPerTx := new(big.Int).Mul(gasPrice, big.NewInt(int64(config.BSCGasLimitBEP20)))

	slog.Info("BSC token sweep gas price",
		"gasPrice", gasPrice,
		"gasCostPerTx", gasCostPerTx,
		"token", token,
	)

	// Pre-check: all addresses must have enough BNB for gas.
	needsGas, err := s.checkGasForTokenSweep(ctx, addresses, gasCostPerTx)
	if err != nil {
		return nil, err
	}
	if len(needsGas) > 0 {
		slog.Error("BSC token sweep: addresses need gas pre-seeding",
			"count", len(needsGas),
		)
		return nil, fmt.Errorf("%w: %d addresses need gas pre-seeding", config.ErrInsufficientBNBForGas, len(needsGas))
	}

	result := &models.BSCSendResult{
		Chain: models.ChainBSC,
		Token: token,
	}
	totalSwept := new(big.Int)

	for _, addr := range addresses {
		if err := ctx.Err(); err != nil {
			slog.Warn("BSC token sweep cancelled", "error", err)
			break
		}

		txResult := s.sweepTokenAddress(ctx, addr, dest, contract, token, gasPrice)
		result.TxResults = append(result.TxResults, txResult)

		if txResult.Status == "confirmed" {
			result.SuccessCount++
			amount, _ := new(big.Int).SetString(txResult.Amount, 10)
			if amount != nil {
				totalSwept.Add(totalSwept, amount)
			}
		} else {
			result.FailCount++
		}
	}

	result.TotalSwept = totalSwept.String()

	slog.Info("BSC token sweep complete",
		"token", token,
		"successCount", result.SuccessCount,
		"failCount", result.FailCount,
		"totalSwept", result.TotalSwept,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return result, nil
}

// checkGasForTokenSweep verifies that all addresses have enough BNB for BEP-20 gas.
// Returns the list of address indices that need gas pre-seeding.
func (s *BSCConsolidationService) checkGasForTokenSweep(
	ctx context.Context,
	addresses []models.AddressWithBalance,
	gasCostPerTx *big.Int,
) ([]int, error) {
	var needsGas []int

	for _, addr := range addresses {
		fromAddr := common.HexToAddress(addr.Address)
		balance, err := s.ethClient.BalanceAt(ctx, fromAddr, nil)
		if err != nil {
			return nil, fmt.Errorf("get BNB balance for gas check on %s: %w", addr.Address, err)
		}

		if balance.Cmp(gasCostPerTx) < 0 {
			needsGas = append(needsGas, addr.AddressIndex)
			slog.Warn("BSC token sweep: address needs gas",
				"address", addr.Address,
				"index", addr.AddressIndex,
				"bnbBalance", balance,
				"gasNeeded", gasCostPerTx,
			)
		}
	}

	return needsGas, nil
}

// sweepTokenAddress sends BEP-20 tokens from a single address to the destination.
func (s *BSCConsolidationService) sweepTokenAddress(
	ctx context.Context,
	addr models.AddressWithBalance,
	dest common.Address,
	contract common.Address,
	token models.Token,
	gasPrice *big.Int,
) models.BSCTxResult {
	txResult := models.BSCTxResult{
		AddressIndex: addr.AddressIndex,
		FromAddress:  addr.Address,
	}

	// Find the token balance for this address.
	var tokenBalance *big.Int
	for _, tb := range addr.TokenBalances {
		if tb.Symbol == token {
			bal, ok := new(big.Int).SetString(tb.Balance, 10)
			if ok && bal.Sign() > 0 {
				tokenBalance = bal
			}
			break
		}
	}

	if tokenBalance == nil || tokenBalance.Sign() <= 0 {
		txResult.Status = "failed"
		txResult.Error = "no token balance"
		return txResult
	}

	fromAddr := common.HexToAddress(addr.Address)

	// Derive private key.
	privKey, derivedAddr, err := s.keyService.DeriveBSCPrivateKey(ctx, uint32(addr.AddressIndex))
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("derive key: %s", err)
		slog.Error("BSC token sweep: key derivation failed", "index", addr.AddressIndex, "error", err)
		return txResult
	}

	if derivedAddr != fromAddr {
		txResult.Status = "failed"
		txResult.Error = "derived address mismatch"
		slog.Error("BSC token sweep: address mismatch",
			"expected", addr.Address,
			"derived", derivedAddr.Hex(),
		)
		return txResult
	}

	// Get nonce.
	nonce, err := s.ethClient.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("get nonce: %s", err)
		slog.Error("BSC token sweep: nonce failed", "address", addr.Address, "error", err)
		return txResult
	}

	// Build and sign BEP-20 transfer.
	unsignedTx := BuildBSCTokenTransfer(nonce, contract, dest, tokenBalance, gasPrice)
	signedTx, err := SignBSCTx(unsignedTx, s.chainID, privKey)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("sign tx: %s", err)
		slog.Error("BSC token sweep: sign failed", "address", addr.Address, "error", err)
		return txResult
	}

	slog.Info("BSC token sweep: broadcasting transfer",
		"from", addr.Address,
		"to", dest.Hex(),
		"contract", contract.Hex(),
		"token", token,
		"amount", tokenBalance,
		"nonce", nonce,
	)

	// Broadcast.
	if err := s.ethClient.SendTransaction(ctx, signedTx); err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("broadcast: %s", err)
		slog.Error("BSC token sweep: broadcast failed", "address", addr.Address, "error", err)
		return txResult
	}

	txHash := signedTx.Hash()
	txResult.TxHash = txHash.Hex()
	txResult.Amount = tokenBalance.String()

	slog.Info("BSC token sweep: TX broadcast, waiting for receipt",
		"txHash", txHash.Hex(),
		"from", addr.Address,
	)

	// Wait for receipt.
	receipt, err := WaitForReceipt(ctx, s.ethClient, txHash)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("receipt: %s", err)
		slog.Error("BSC token sweep: receipt failed", "txHash", txHash.Hex(), "error", err)
		s.recordBSCTransaction(addr, txHash.Hex(), tokenBalance.String(), dest.Hex(), token, "pending")
		return txResult
	}

	txResult.Status = "confirmed"

	s.recordBSCTransaction(addr, txHash.Hex(), tokenBalance.String(), dest.Hex(), token, "confirmed")

	slog.Info("BSC token sweep: transfer confirmed",
		"txHash", txHash.Hex(),
		"from", addr.Address,
		"token", token,
		"amount", tokenBalance,
		"block", receipt.BlockNumber,
	)

	return txResult
}

// recordBSCTransaction stores a BSC transaction in the database.
func (s *BSCConsolidationService) recordBSCTransaction(
	addr models.AddressWithBalance,
	txHash string,
	amount string,
	destAddr string,
	token models.Token,
	status string,
) {
	txRecord := models.Transaction{
		Chain:        models.ChainBSC,
		AddressIndex: addr.AddressIndex,
		TxHash:       txHash,
		Direction:    "send",
		Token:        token,
		Amount:       amount,
		FromAddress:  addr.Address,
		ToAddress:    destAddr,
		Status:       status,
	}

	if _, err := s.database.InsertTransaction(txRecord); err != nil {
		slog.Error("failed to record BSC transaction in DB",
			"txHash", txHash,
			"addressIndex", addr.AddressIndex,
			"error", err,
		)
	}
}

// VerifyBSCAddress verifies that a private key at the given index produces the expected address.
func VerifyBSCAddress(privKey *ecdsa.PrivateKey, expectedAddr string) bool {
	pubKey := privKey.Public().(*ecdsa.PublicKey)
	addr := crypto.PubkeyToAddress(*pubKey)
	return addr.Hex() == expectedAddr
}
