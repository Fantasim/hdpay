package tx

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// gasPreSeedAmountWei is the amount of BNB (in wei) sent to each target during gas pre-seeding.
var gasPreSeedAmountWei = func() *big.Int {
	val, _ := new(big.Int).SetString(config.BSCGasPreSeedWei, 10)
	return val
}()

// GasPreSeedService distributes BNB from a source address to targets that need gas for token transfers.
type GasPreSeedService struct {
	keyService *KeyService
	ethClient  EthClientWrapper
	database   *db.DB
	chainID    *big.Int
}

// NewGasPreSeedService creates a gas pre-seeding service.
func NewGasPreSeedService(
	keyService *KeyService,
	ethClient EthClientWrapper,
	database *db.DB,
	chainID *big.Int,
) *GasPreSeedService {
	slog.Info("gas pre-seed service created", "chainID", chainID)
	return &GasPreSeedService{
		keyService: keyService,
		ethClient:  ethClient,
		database:   database,
		chainID:    chainID,
	}
}

// Preview calculates what the gas pre-seeding operation would require.
func (s *GasPreSeedService) Preview(
	ctx context.Context,
	sourceIndex int,
	targetAddresses []string,
) (*models.GasPreSeedPreview, error) {
	slog.Info("gas pre-seed preview",
		"sourceIndex", sourceIndex,
		"targetCount", len(targetAddresses),
	)

	// Derive the source address.
	_, sourceAddr, err := s.keyService.DeriveBSCPrivateKey(ctx, uint32(sourceIndex))
	if err != nil {
		return nil, fmt.Errorf("derive source address at index %d: %w", sourceIndex, err)
	}

	// Get source balance.
	sourceBalance, err := s.ethClient.BalanceAt(ctx, sourceAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("get source balance: %w", err)
	}

	// Get gas price for cost estimation.
	gasPrice, err := s.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	gasPrice = BufferedGasPrice(gasPrice)

	targetCount := int64(len(targetAddresses))
	gasCostPerSend := new(big.Int).Mul(gasPrice, big.NewInt(int64(config.BSCGasLimitTransfer)))
	totalAmountToSend := new(big.Int).Mul(gasPreSeedAmountWei, big.NewInt(targetCount))
	totalGasCost := new(big.Int).Mul(gasCostPerSend, big.NewInt(targetCount))
	totalNeeded := new(big.Int).Add(totalAmountToSend, totalGasCost)

	sufficient := sourceBalance.Cmp(totalNeeded) >= 0

	preview := &models.GasPreSeedPreview{
		SourceIndex:     sourceIndex,
		SourceAddress:   sourceAddr.Hex(),
		SourceBalance:   sourceBalance.String(),
		TargetCount:     len(targetAddresses),
		AmountPerTarget: gasPreSeedAmountWei.String(),
		TotalNeeded:     totalNeeded.String(),
		Sufficient:      sufficient,
	}

	slog.Info("gas pre-seed preview complete",
		"sourceAddress", sourceAddr.Hex(),
		"sourceBalance", sourceBalance,
		"targetCount", len(targetAddresses),
		"totalNeeded", totalNeeded,
		"sufficient", sufficient,
	)

	return preview, nil
}

// Execute performs the gas pre-seeding: sends BNB from the source to each target address.
// Nonce is managed locally — fetched once and incremented per TX.
func (s *GasPreSeedService) Execute(
	ctx context.Context,
	sourceIndex int,
	targetAddresses []string,
) (*models.GasPreSeedResult, error) {
	slog.Info("gas pre-seed execute",
		"sourceIndex", sourceIndex,
		"targetCount", len(targetAddresses),
	)
	start := time.Now()

	// Derive source private key.
	privKey, sourceAddr, err := s.keyService.DeriveBSCPrivateKey(ctx, uint32(sourceIndex))
	if err != nil {
		return nil, fmt.Errorf("derive source key at index %d: %w", sourceIndex, err)
	}

	// Get source balance.
	sourceBalance, err := s.ethClient.BalanceAt(ctx, sourceAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("get source balance: %w", err)
	}

	// Get gas price.
	gasPrice, err := s.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %w", err)
	}
	gasPrice = BufferedGasPrice(gasPrice)

	// Validate source has sufficient balance.
	targetCount := int64(len(targetAddresses))
	gasCostPerSend := new(big.Int).Mul(gasPrice, big.NewInt(int64(config.BSCGasLimitTransfer)))
	totalAmountToSend := new(big.Int).Mul(gasPreSeedAmountWei, big.NewInt(targetCount))
	totalGasCost := new(big.Int).Mul(gasCostPerSend, big.NewInt(targetCount))
	totalNeeded := new(big.Int).Add(totalAmountToSend, totalGasCost)

	if sourceBalance.Cmp(totalNeeded) < 0 {
		return nil, fmt.Errorf("%w: source balance %s < total needed %s (%d targets)",
			config.ErrInsufficientBNBForGas, sourceBalance, totalNeeded, targetCount)
	}

	// Get initial nonce — then increment locally per TX.
	nonce, err := s.ethClient.PendingNonceAt(ctx, sourceAddr)
	if err != nil {
		return nil, fmt.Errorf("get source nonce: %w", err)
	}

	slog.Info("gas pre-seed: starting distribution",
		"sourceAddress", sourceAddr.Hex(),
		"sourceBalance", sourceBalance,
		"targetCount", len(targetAddresses),
		"amountPerTarget", gasPreSeedAmountWei,
		"gasPrice", gasPrice,
		"startNonce", nonce,
	)

	result := &models.GasPreSeedResult{}
	totalSent := new(big.Int)

	for i, targetAddr := range targetAddresses {
		if err := ctx.Err(); err != nil {
			slog.Warn("gas pre-seed cancelled", "error", err, "sent", i)
			break
		}

		txResult := s.sendGasPreSeed(ctx, privKey, sourceAddr, targetAddr, nonce, gasPrice, i)
		result.TxResults = append(result.TxResults, txResult)

		if txResult.Status == "confirmed" {
			result.SuccessCount++
			totalSent.Add(totalSent, gasPreSeedAmountWei)
		} else {
			result.FailCount++
		}

		// Increment nonce if the TX was broadcast (even if receipt failed).
		if txResult.TxHash != "" {
			nonce++
		}
	}

	result.TotalSent = totalSent.String()

	slog.Info("gas pre-seed complete",
		"successCount", result.SuccessCount,
		"failCount", result.FailCount,
		"totalSent", result.TotalSent,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return result, nil
}

// sendGasPreSeed sends a single gas pre-seed transaction.
func (s *GasPreSeedService) sendGasPreSeed(
	ctx context.Context,
	privKey *ecdsa.PrivateKey,
	sourceAddr common.Address,
	targetAddr string,
	nonce uint64,
	gasPrice *big.Int,
	targetIndex int,
) models.BSCTxResult {
	txResult := models.BSCTxResult{
		AddressIndex: targetIndex,
		FromAddress:  sourceAddr.Hex(),
	}

	target := common.HexToAddress(targetAddr)

	// Build native transfer.
	unsignedTx := BuildBSCNativeTransfer(nonce, target, gasPreSeedAmountWei, gasPrice)

	signedTx, err := SignBSCTx(unsignedTx, s.chainID, privKey)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("sign tx: %s", err)
		slog.Error("gas pre-seed: sign failed", "target", targetAddr, "error", err)
		return txResult
	}

	slog.Info("gas pre-seed: broadcasting",
		"target", targetAddr,
		"amount", gasPreSeedAmountWei,
		"nonce", nonce,
		"targetNum", targetIndex+1,
	)

	if err := s.ethClient.SendTransaction(ctx, signedTx); err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("broadcast: %s", err)
		slog.Error("gas pre-seed: broadcast failed", "target", targetAddr, "error", err)
		return txResult
	}

	txHash := signedTx.Hash()
	txResult.TxHash = txHash.Hex()
	txResult.Amount = gasPreSeedAmountWei.String()

	// Wait for receipt.
	receipt, err := WaitForReceipt(ctx, s.ethClient, txHash)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("receipt: %s", err)
		slog.Error("gas pre-seed: receipt failed", "txHash", txHash.Hex(), "error", err)
		s.recordGasPreSeedTx(sourceAddr.Hex(), targetAddr, txHash.Hex(), "pending")
		return txResult
	}

	txResult.Status = "confirmed"

	s.recordGasPreSeedTx(sourceAddr.Hex(), targetAddr, txHash.Hex(), "confirmed")

	slog.Info("gas pre-seed: transfer confirmed",
		"txHash", txHash.Hex(),
		"target", targetAddr,
		"block", receipt.BlockNumber,
	)

	return txResult
}

// recordGasPreSeedTx stores a gas pre-seed transaction in the database.
func (s *GasPreSeedService) recordGasPreSeedTx(fromAddr, toAddr, txHash, status string) {
	txRecord := models.Transaction{
		Chain:       models.ChainBSC,
		TxHash:      txHash,
		Direction:   "gas-preseed",
		Token:       models.TokenNative,
		Amount:      gasPreSeedAmountWei.String(),
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Status:      status,
	}

	if _, err := s.database.InsertTransaction(txRecord); err != nil {
		slog.Error("failed to record gas pre-seed TX in DB",
			"txHash", txHash,
			"error", err,
		)
	}
}
