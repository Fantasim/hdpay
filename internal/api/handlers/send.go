package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/tx"
)

// SendDeps holds all dependencies needed by send handlers.
type SendDeps struct {
	DB         *db.DB
	Config     *config.Config
	BTCService *tx.BTCConsolidationService
	BSCService *tx.BSCConsolidationService
	SOLService *tx.SOLConsolidationService
	GasPreSeed *tx.GasPreSeedService
	TxHub      *tx.TxSSEHub
	NetParams  *chaincfg.Params
	ChainLocks map[models.Chain]*sync.Mutex // per-chain mutex to prevent concurrent sweeps
}

// solBase58Regex matches valid Solana base58 addresses (32-44 chars, no 0OIl).
var solBase58Regex = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)

// validateDestination validates a destination address for a given chain.
func validateDestination(chain models.Chain, address string, netParams *chaincfg.Params) error {
	if address == "" {
		return fmt.Errorf("destination address is required")
	}

	switch chain {
	case models.ChainBTC:
		_, err := btcutil.DecodeAddress(address, netParams)
		if err != nil {
			return fmt.Errorf("invalid BTC address: %w", err)
		}

	case models.ChainBSC:
		if !common.IsHexAddress(address) {
			return fmt.Errorf("invalid BSC address: must be 0x-prefixed hex (42 chars)")
		}

	case models.ChainSOL:
		if !solBase58Regex.MatchString(address) {
			return fmt.Errorf("invalid SOL address: must be base58 encoded (32-44 chars)")
		}

	default:
		return fmt.Errorf("unsupported chain: %s", chain)
	}

	return nil
}

// isValidToken checks if a token is valid for a given chain.
func isValidToken(chain models.Chain, token models.Token) bool {
	switch chain {
	case models.ChainBTC:
		return token == models.TokenNative
	case models.ChainBSC:
		return token == models.TokenNative || token == models.TokenUSDC || token == models.TokenUSDT
	case models.ChainSOL:
		return token == models.TokenNative || token == models.TokenUSDC || token == models.TokenUSDT
	}
	return false
}

// getTokenContractAddress returns the contract/mint address for a token on a chain.
func getTokenContractAddress(chain models.Chain, token models.Token, network string) string {
	isTestnet := network == string(models.NetworkTestnet)

	switch chain {
	case models.ChainBSC:
		if token == models.TokenUSDC {
			if isTestnet {
				return config.BSCTestnetUSDCContract
			}
			return config.BSCUSDCContract
		}
		if token == models.TokenUSDT {
			if isTestnet {
				return config.BSCTestnetUSDTContract
			}
			return config.BSCUSDTContract
		}
	case models.ChainSOL:
		if token == models.TokenUSDC {
			if isTestnet {
				return config.SOLDevnetUSDCMint
			}
			return config.SOLUSDCMint
		}
		if token == models.TokenUSDT {
			if isTestnet {
				return config.SOLDevnetUSDTMint
			}
			return config.SOLUSDTMint
		}
	}
	return ""
}

// PreviewSend handles POST /api/send/preview.
func PreviewSend(deps *SendDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req models.SendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Warn("invalid send preview request body", "error", err)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidAddress, "invalid request body")
			return
		}

		req.Chain = models.Chain(strings.ToUpper(string(req.Chain)))
		req.Token = models.Token(strings.ToUpper(string(req.Token)))

		slog.Info("send preview requested",
			"chain", req.Chain,
			"token", req.Token,
			"destination", req.Destination,
		)

		// Validate chain.
		if !isValidChain(req.Chain) {
			slog.Warn("invalid chain for send preview", "chain", req.Chain)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidChain, "invalid chain: "+string(req.Chain))
			return
		}

		// Validate token.
		if !isValidToken(req.Chain, req.Token) {
			slog.Warn("invalid token for send preview", "chain", req.Chain, "token", req.Token)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidToken,
				fmt.Sprintf("invalid token %s for chain %s", req.Token, req.Chain))
			return
		}

		// Validate destination.
		if err := validateDestination(req.Chain, req.Destination, deps.NetParams); err != nil {
			slog.Warn("invalid destination address",
				"chain", req.Chain,
				"destination", req.Destination,
				"error", err,
			)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidDestination, err.Error())
			return
		}

		// Fetch funded addresses from DB.
		funded, err := deps.DB.GetFundedAddressesJoined(req.Chain, req.Token)
		if err != nil {
			slog.Error("failed to fetch funded addresses", "chain", req.Chain, "token", req.Token, "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch funded addresses")
			return
		}

		if len(funded) == 0 {
			slog.Info("no funded addresses for send preview", "chain", req.Chain, "token", req.Token)
			writeError(w, http.StatusBadRequest, config.ErrorNoFundedAddresses,
				fmt.Sprintf("no funded %s addresses found for %s", req.Token, req.Chain))
			return
		}

		slog.Info("funded addresses found for preview",
			"chain", req.Chain,
			"token", req.Token,
			"count", len(funded),
		)

		// Dispatch to chain-specific preview.
		preview, err := buildPreview(r, deps, req, funded)
		if err != nil {
			slog.Error("send preview failed",
				"chain", req.Chain,
				"token", req.Token,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, config.ErrorTxBuildFailed, err.Error())
			return
		}

		slog.Info("send preview generated",
			"chain", req.Chain,
			"token", req.Token,
			"fundedCount", preview.FundedCount,
			"totalAmount", preview.TotalAmount,
			"feeEstimate", preview.FeeEstimate,
			"needsGasPreSeed", preview.NeedsGasPreSeed,
			"duration", time.Since(start).Round(time.Millisecond),
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: preview,
			Meta: &models.APIMeta{ExecutionTime: time.Since(start).Milliseconds()},
		})
	}
}

// buildPreview dispatches to chain-specific preview logic and returns a unified preview.
func buildPreview(r *http.Request, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance) (*models.UnifiedSendPreview, error) {
	ctx := r.Context()

	switch req.Chain {
	case models.ChainBTC:
		return buildBTCPreview(ctx, deps, req, funded)

	case models.ChainBSC:
		if req.Token == models.TokenNative {
			return buildBSCNativePreview(ctx, deps, req, funded)
		}
		return buildBSCTokenPreview(ctx, deps, req, funded)

	case models.ChainSOL:
		if req.Token == models.TokenNative {
			return buildSOLNativePreview(ctx, deps, req, funded)
		}
		return buildSOLTokenPreview(ctx, deps, req, funded)

	default:
		return nil, fmt.Errorf("unsupported chain: %s", req.Chain)
	}
}

// buildBTCPreview generates a unified preview for BTC consolidation.
func buildBTCPreview(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance) (*models.UnifiedSendPreview, error) {
	// Convert to Address slice for BTC service.
	addresses := make([]models.Address, len(funded))
	for i, f := range funded {
		addresses[i] = models.Address{
			Chain:        f.Chain,
			AddressIndex: f.AddressIndex,
			Address:      f.Address,
		}
	}

	// Use default fee rate; the BTC service will try to estimate dynamically.
	btcPreview, err := deps.BTCService.Preview(ctx, addresses, req.Destination, 0)
	if err != nil {
		return nil, fmt.Errorf("BTC preview failed: %w", err)
	}

	// Build funded address info.
	fundedInfos := make([]models.FundedAddressInfo, len(funded))
	for i, f := range funded {
		fundedInfos[i] = models.FundedAddressInfo{
			AddressIndex: f.AddressIndex,
			Address:      f.Address,
			Balance:      f.NativeBalance,
			HasGas:       true, // BTC: UTXOs cover fees implicitly
		}
	}

	return &models.UnifiedSendPreview{
		Chain:           req.Chain,
		Token:           req.Token,
		Destination:     req.Destination,
		FundedCount:     btcPreview.InputCount,
		TotalAmount:     fmt.Sprintf("%d", btcPreview.TotalInputSats),
		FeeEstimate:     fmt.Sprintf("%d", btcPreview.FeeSats),
		NetAmount:       fmt.Sprintf("%d", btcPreview.OutputSats),
		TxCount:         1, // BTC is always a single transaction.
		NeedsGasPreSeed: false,
		GasPreSeedCount: 0,
		FundedAddresses: fundedInfos,
	}, nil
}

// buildBSCNativePreview generates a unified preview for BSC native (BNB) sweep.
func buildBSCNativePreview(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance) (*models.UnifiedSendPreview, error) {
	bscPreview, err := deps.BSCService.PreviewNativeSweep(ctx, funded, req.Destination)
	if err != nil {
		return nil, fmt.Errorf("BSC native preview failed: %w", err)
	}

	fundedInfos := make([]models.FundedAddressInfo, len(funded))
	for i, f := range funded {
		fundedInfos[i] = models.FundedAddressInfo{
			AddressIndex: f.AddressIndex,
			Address:      f.Address,
			Balance:      f.NativeBalance,
			HasGas:       true, // Native BNB sweeps: gas is deducted from balance.
		}
	}

	return &models.UnifiedSendPreview{
		Chain:           req.Chain,
		Token:           req.Token,
		Destination:     req.Destination,
		FundedCount:     bscPreview.InputCount,
		TotalAmount:     bscPreview.TotalAmount,
		FeeEstimate:     bscPreview.GasCostWei,
		NetAmount:       bscPreview.NetAmount,
		TxCount:         bscPreview.InputCount, // One TX per address.
		NeedsGasPreSeed: false,
		GasPreSeedCount: 0,
		FundedAddresses: fundedInfos,
	}, nil
}

// buildBSCTokenPreview generates a unified preview for BSC token (USDC/USDT) sweep.
func buildBSCTokenPreview(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance) (*models.UnifiedSendPreview, error) {
	// Estimate gas cost: count × gasLimit × bufferedGasPrice.
	gasPrice, err := deps.BSCService.EstimateGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("BSC gas price estimation failed: %w", err)
	}

	gasCostPerTx := new(big.Int).Mul(big.NewInt(int64(config.BSCGasLimitBEP20)), gasPrice)
	totalGasCost := new(big.Int).Mul(gasCostPerTx, big.NewInt(int64(len(funded))))

	// Calculate total token amount.
	totalAmount := new(big.Int)
	var gasPreSeedCount int
	fundedInfos := make([]models.FundedAddressInfo, len(funded))

	for i, f := range funded {
		// Find token balance.
		tokenBal := "0"
		for _, tb := range f.TokenBalances {
			if tb.Symbol == req.Token {
				tokenBal = tb.Balance
				break
			}
		}

		bal, ok := new(big.Int).SetString(tokenBal, 10)
		if ok {
			totalAmount.Add(totalAmount, bal)
		}

		// Check if address has enough BNB for gas.
		nativeBal, _ := new(big.Int).SetString(f.NativeBalance, 10)
		if nativeBal == nil {
			nativeBal = new(big.Int)
		}
		hasGas := nativeBal.Cmp(gasCostPerTx) >= 0
		if !hasGas {
			gasPreSeedCount++
		}

		fundedInfos[i] = models.FundedAddressInfo{
			AddressIndex: f.AddressIndex,
			Address:      f.Address,
			Balance:      tokenBal,
			HasGas:       hasGas,
		}
	}

	return &models.UnifiedSendPreview{
		Chain:           req.Chain,
		Token:           req.Token,
		Destination:     req.Destination,
		FundedCount:     len(funded),
		TotalAmount:     totalAmount.String(),
		FeeEstimate:     totalGasCost.String(),
		NetAmount:       totalAmount.String(), // Token sweep: no fee deducted from token amount.
		TxCount:         len(funded),
		NeedsGasPreSeed: gasPreSeedCount > 0,
		GasPreSeedCount: gasPreSeedCount,
		FundedAddresses: fundedInfos,
	}, nil
}

// buildSOLNativePreview generates a unified preview for SOL native sweep.
func buildSOLNativePreview(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance) (*models.UnifiedSendPreview, error) {
	solPreview, err := deps.SOLService.PreviewNativeSweep(ctx, funded, req.Destination)
	if err != nil {
		return nil, fmt.Errorf("SOL native preview failed: %w", err)
	}

	fundedInfos := make([]models.FundedAddressInfo, len(funded))
	for i, f := range funded {
		fundedInfos[i] = models.FundedAddressInfo{
			AddressIndex: f.AddressIndex,
			Address:      f.Address,
			Balance:      f.NativeBalance,
			HasGas:       true, // SOL: fee deducted from balance.
		}
	}

	return &models.UnifiedSendPreview{
		Chain:           req.Chain,
		Token:           req.Token,
		Destination:     req.Destination,
		FundedCount:     solPreview.InputCount,
		TotalAmount:     solPreview.TotalAmount,
		FeeEstimate:     solPreview.TotalFee,
		NetAmount:       solPreview.NetAmount,
		TxCount:         solPreview.InputCount,
		NeedsGasPreSeed: false,
		GasPreSeedCount: 0,
		FundedAddresses: fundedInfos,
	}, nil
}

// buildSOLTokenPreview generates a unified preview for SOL token (USDC/USDT) sweep.
func buildSOLTokenPreview(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance) (*models.UnifiedSendPreview, error) {
	mint := getTokenContractAddress(req.Chain, req.Token, deps.Config.Network)

	solPreview, err := deps.SOLService.PreviewTokenSweep(ctx, funded, req.Destination, req.Token, mint)
	if err != nil {
		return nil, fmt.Errorf("SOL token preview failed: %w", err)
	}

	fundedInfos := make([]models.FundedAddressInfo, len(funded))
	for i, f := range funded {
		tokenBal := "0"
		for _, tb := range f.TokenBalances {
			if tb.Symbol == req.Token {
				tokenBal = tb.Balance
				break
			}
		}
		fundedInfos[i] = models.FundedAddressInfo{
			AddressIndex: f.AddressIndex,
			Address:      f.Address,
			Balance:      tokenBal,
			HasGas:       true, // SOL: fee deducted from SOL balance, not token.
		}
	}

	return &models.UnifiedSendPreview{
		Chain:           req.Chain,
		Token:           req.Token,
		Destination:     req.Destination,
		FundedCount:     solPreview.InputCount,
		TotalAmount:     solPreview.TotalAmount,
		FeeEstimate:     solPreview.TotalFee,
		NetAmount:       solPreview.NetAmount,
		TxCount:         solPreview.InputCount,
		NeedsGasPreSeed: false,
		GasPreSeedCount: 0,
		FundedAddresses: fundedInfos,
	}, nil
}

// ExecuteSend handles POST /api/send/execute.
func ExecuteSend(deps *SendDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req models.SendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Warn("invalid send execute request body", "error", err)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidAddress, "invalid request body")
			return
		}

		req.Chain = models.Chain(strings.ToUpper(string(req.Chain)))
		req.Token = models.Token(strings.ToUpper(string(req.Token)))

		slog.Info("send execute requested",
			"chain", req.Chain,
			"token", req.Token,
			"destination", req.Destination,
		)

		// Validate chain, token, destination.
		if !isValidChain(req.Chain) {
			writeError(w, http.StatusBadRequest, config.ErrorInvalidChain, "invalid chain")
			return
		}
		if !isValidToken(req.Chain, req.Token) {
			writeError(w, http.StatusBadRequest, config.ErrorInvalidToken, "invalid token for chain")
			return
		}
		if err := validateDestination(req.Chain, req.Destination, deps.NetParams); err != nil {
			writeError(w, http.StatusBadRequest, config.ErrorInvalidDestination, err.Error())
			return
		}

		// Re-fetch funded addresses (may have changed since preview).
		funded, err := deps.DB.GetFundedAddressesJoined(req.Chain, req.Token)
		if err != nil {
			slog.Error("failed to fetch funded addresses for execute", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch funded addresses")
			return
		}

		if len(funded) == 0 {
			writeError(w, http.StatusBadRequest, config.ErrorNoFundedAddresses, "no funded addresses found")
			return
		}

		// Acquire per-chain mutex to prevent concurrent sweeps.
		mu := deps.ChainLocks[req.Chain]
		if mu == nil {
			slog.Error("no chain lock configured", "chain", req.Chain)
			writeError(w, http.StatusInternalServerError, config.ErrorTxBroadcastFailed, "internal configuration error")
			return
		}
		if !mu.TryLock() {
			slog.Warn("send already in progress for chain", "chain", req.Chain)
			writeError(w, http.StatusConflict, config.ErrorSendBusy,
				fmt.Sprintf("send operation already in progress for %s", req.Chain))
			return
		}
		defer mu.Unlock()

		// Generate sweep ID for grouping all TX states in this sweep.
		sweepID := tx.GenerateSweepID()

		slog.Info("executing send sweep",
			"chain", req.Chain,
			"token", req.Token,
			"fundedCount", len(funded),
			"destination", req.Destination,
			"sweepID", sweepID,
		)

		// Dispatch to chain-specific execute.
		result, err := executeSweep(r, deps, req, funded, sweepID)
		if err != nil {
			slog.Error("send execute failed",
				"chain", req.Chain,
				"token", req.Token,
				"error", err,
				"duration", time.Since(start).Round(time.Millisecond),
			)
			writeError(w, http.StatusInternalServerError, config.ErrorTxBroadcastFailed, err.Error())
			return
		}

		slog.Info("send execute completed",
			"chain", req.Chain,
			"token", req.Token,
			"successCount", result.SuccessCount,
			"failCount", result.FailCount,
			"totalSwept", result.TotalSwept,
			"duration", time.Since(start).Round(time.Millisecond),
		)

		// Broadcast completion event via SSE.
		if deps.TxHub != nil {
			deps.TxHub.Broadcast(tx.TxEvent{
				Type: "tx_complete",
				Data: tx.TxCompleteData{
					Chain:        string(req.Chain),
					Token:        string(req.Token),
					SuccessCount: result.SuccessCount,
					FailCount:    result.FailCount,
					TotalSwept:   result.TotalSwept,
				},
			})
		}

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: result,
			Meta: &models.APIMeta{ExecutionTime: time.Since(start).Milliseconds()},
		})
	}
}

// executeSweep dispatches to chain-specific execute logic and returns a unified result.
func executeSweep(r *http.Request, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance, sweepID string) (*models.UnifiedSendResult, error) {
	ctx := r.Context()

	switch req.Chain {
	case models.ChainBTC:
		return executeBTCSweep(ctx, deps, req, funded, sweepID)

	case models.ChainBSC:
		if req.Token == models.TokenNative {
			return executeBSCNativeSweep(ctx, deps, req, funded, sweepID)
		}
		return executeBSCTokenSweep(ctx, deps, req, funded, sweepID)

	case models.ChainSOL:
		if req.Token == models.TokenNative {
			return executeSOLNativeSweep(ctx, deps, req, funded, sweepID)
		}
		return executeSOLTokenSweep(ctx, deps, req, funded, sweepID)

	default:
		return nil, fmt.Errorf("unsupported chain: %s", req.Chain)
	}
}

// executeBTCSweep executes a BTC consolidation sweep.
func executeBTCSweep(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance, sweepID string) (*models.UnifiedSendResult, error) {
	addresses := make([]models.Address, len(funded))
	for i, f := range funded {
		addresses[i] = models.Address{
			Chain:        f.Chain,
			AddressIndex: f.AddressIndex,
			Address:      f.Address,
		}
	}

	btcResult, err := deps.BTCService.Execute(ctx, addresses, req.Destination, 0, sweepID, req.ExpectedInputCount, req.ExpectedTotalSats)
	if err != nil {
		return nil, fmt.Errorf("BTC execute failed: %w", err)
	}

	// Build per-input results for display.
	txResults := make([]models.TxResult, len(funded))
	for i, f := range funded {
		txResults[i] = models.TxResult{
			AddressIndex: f.AddressIndex,
			FromAddress:  f.Address,
			TxHash:       btcResult.TxHash,
			Amount:       f.NativeBalance,
			Status:       "success",
		}
	}

	return &models.UnifiedSendResult{
		Chain:        req.Chain,
		Token:        req.Token,
		TxResults:    txResults,
		SuccessCount: len(funded),
		FailCount:    0,
		TotalSwept:   strconv.FormatInt(btcResult.OutputSats, 10),
	}, nil
}

// executeBSCNativeSweep executes a BSC native BNB sweep.
func executeBSCNativeSweep(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance, sweepID string) (*models.UnifiedSendResult, error) {
	bscResult, err := deps.BSCService.ExecuteNativeSweep(ctx, funded, req.Destination, sweepID, req.ExpectedGasPrice)
	if err != nil {
		return nil, fmt.Errorf("BSC native sweep failed: %w", err)
	}

	txResults := make([]models.TxResult, len(bscResult.TxResults))
	for i, r := range bscResult.TxResults {
		txResults[i] = models.TxResult{
			AddressIndex: r.AddressIndex,
			FromAddress:  r.FromAddress,
			TxHash:       r.TxHash,
			Amount:       r.Amount,
			Status:       r.Status,
			Error:        r.Error,
		}
	}

	return &models.UnifiedSendResult{
		Chain:        req.Chain,
		Token:        req.Token,
		TxResults:    txResults,
		SuccessCount: bscResult.SuccessCount,
		FailCount:    bscResult.FailCount,
		TotalSwept:   bscResult.TotalSwept,
	}, nil
}

// executeBSCTokenSweep executes a BSC token (USDC/USDT) sweep.
func executeBSCTokenSweep(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance, sweepID string) (*models.UnifiedSendResult, error) {
	contractAddr := getTokenContractAddress(req.Chain, req.Token, deps.Config.Network)

	bscResult, err := deps.BSCService.ExecuteTokenSweep(ctx, funded, req.Destination, req.Token, contractAddr, sweepID, req.ExpectedGasPrice)
	if err != nil {
		return nil, fmt.Errorf("BSC token sweep failed: %w", err)
	}

	txResults := make([]models.TxResult, len(bscResult.TxResults))
	for i, r := range bscResult.TxResults {
		txResults[i] = models.TxResult{
			AddressIndex: r.AddressIndex,
			FromAddress:  r.FromAddress,
			TxHash:       r.TxHash,
			Amount:       r.Amount,
			Status:       r.Status,
			Error:        r.Error,
		}
	}

	return &models.UnifiedSendResult{
		Chain:        req.Chain,
		Token:        req.Token,
		TxResults:    txResults,
		SuccessCount: bscResult.SuccessCount,
		FailCount:    bscResult.FailCount,
		TotalSwept:   bscResult.TotalSwept,
	}, nil
}

// executeSOLNativeSweep executes a SOL native sweep.
func executeSOLNativeSweep(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance, sweepID string) (*models.UnifiedSendResult, error) {
	solResult, err := deps.SOLService.ExecuteNativeSweep(ctx, funded, req.Destination, sweepID)
	if err != nil {
		return nil, fmt.Errorf("SOL native sweep failed: %w", err)
	}

	txResults := make([]models.TxResult, len(solResult.TxResults))
	for i, r := range solResult.TxResults {
		txResults[i] = models.TxResult{
			AddressIndex: r.AddressIndex,
			FromAddress:  r.FromAddress,
			TxHash:       r.TxSignature,
			Amount:       r.Amount,
			Status:       r.Status,
			Error:        r.Error,
		}
	}

	return &models.UnifiedSendResult{
		Chain:        req.Chain,
		Token:        req.Token,
		TxResults:    txResults,
		SuccessCount: solResult.SuccessCount,
		FailCount:    solResult.FailCount,
		TotalSwept:   solResult.TotalSwept,
	}, nil
}

// executeSOLTokenSweep executes a SOL token (USDC/USDT) sweep.
func executeSOLTokenSweep(ctx context.Context, deps *SendDeps, req models.SendRequest, funded []models.AddressWithBalance, sweepID string) (*models.UnifiedSendResult, error) {
	mint := getTokenContractAddress(req.Chain, req.Token, deps.Config.Network)

	solResult, err := deps.SOLService.ExecuteTokenSweep(ctx, funded, req.Destination, req.Token, mint, sweepID)
	if err != nil {
		return nil, fmt.Errorf("SOL token sweep failed: %w", err)
	}

	txResults := make([]models.TxResult, len(solResult.TxResults))
	for i, r := range solResult.TxResults {
		txResults[i] = models.TxResult{
			AddressIndex: r.AddressIndex,
			FromAddress:  r.FromAddress,
			TxHash:       r.TxSignature,
			Amount:       r.Amount,
			Status:       r.Status,
			Error:        r.Error,
		}
	}

	return &models.UnifiedSendResult{
		Chain:        req.Chain,
		Token:        req.Token,
		TxResults:    txResults,
		SuccessCount: solResult.SuccessCount,
		FailCount:    solResult.FailCount,
		TotalSwept:   solResult.TotalSwept,
	}, nil
}

// GasPreSeedHandler handles POST /api/send/gas-preseed.
func GasPreSeedHandler(deps *SendDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req models.GasPreSeedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Warn("invalid gas pre-seed request body", "error", err)
			writeError(w, http.StatusBadRequest, config.ErrorGasPreSeedFailed, "invalid request body")
			return
		}

		slog.Info("gas pre-seed requested",
			"sourceIndex", req.SourceIndex,
			"targetCount", len(req.TargetAddresses),
		)

		if len(req.TargetAddresses) == 0 {
			writeError(w, http.StatusBadRequest, config.ErrorGasPreSeedFailed, "no target addresses provided")
			return
		}

		// Validate target addresses.
		for _, addr := range req.TargetAddresses {
			if !common.IsHexAddress(addr) {
				slog.Warn("invalid target address in gas pre-seed", "address", addr)
				writeError(w, http.StatusBadRequest, config.ErrorInvalidAddress,
					fmt.Sprintf("invalid BSC address: %s", addr))
				return
			}
		}

		// Preview first to check feasibility.
		preview, err := deps.GasPreSeed.Preview(r.Context(), req.SourceIndex, req.TargetAddresses)
		if err != nil {
			slog.Error("gas pre-seed preview failed", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorGasPreSeedFailed, err.Error())
			return
		}

		if !preview.Sufficient {
			slog.Warn("insufficient balance for gas pre-seed",
				"sourceIndex", req.SourceIndex,
				"sourceBalance", preview.SourceBalance,
				"totalNeeded", preview.TotalNeeded,
			)
			writeError(w, http.StatusBadRequest, config.ErrorInsufficientBalance,
				fmt.Sprintf("source address has %s wei but needs %s wei", preview.SourceBalance, preview.TotalNeeded))
			return
		}

		// Execute the gas pre-seed.
		result, err := deps.GasPreSeed.Execute(r.Context(), req.SourceIndex, req.TargetAddresses)
		if err != nil {
			slog.Error("gas pre-seed execute failed", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorGasPreSeedFailed, err.Error())
			return
		}

		slog.Info("gas pre-seed completed",
			"successCount", result.SuccessCount,
			"failCount", result.FailCount,
			"totalSent", result.TotalSent,
			"duration", time.Since(start).Round(time.Millisecond),
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: result,
			Meta: &models.APIMeta{ExecutionTime: time.Since(start).Milliseconds()},
		})
	}
}

// SendSSE handles GET /api/send/sse for transaction status streaming.
func SendSSE(hub *tx.TxSSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			slog.Error("TX SSE not supported: response writer does not implement http.Flusher")
			writeError(w, http.StatusInternalServerError, config.ErrorTxBroadcastFailed, "streaming not supported")
			return
		}

		slog.Info("TX SSE client connecting", "remoteAddr", r.RemoteAddr)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := hub.Subscribe()
		defer func() {
			hub.Unsubscribe(ch)
			slog.Info("TX SSE client disconnected", "remoteAddr", r.RemoteAddr)
		}()

		slog.Info("TX SSE client connected",
			"remoteAddr", r.RemoteAddr,
			"totalClients", hub.ClientCount(),
		)

		keepAlive := time.NewTicker(config.SSEKeepAliveInterval)
		defer keepAlive.Stop()

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					slog.Info("TX SSE channel closed", "remoteAddr", r.RemoteAddr)
					return
				}

				data, err := json.Marshal(event.Data)
				if err != nil {
					slog.Error("failed to marshal TX SSE event", "type", event.Type, "error", err)
					continue
				}

				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, string(data))
				flusher.Flush()

			case <-keepAlive.C:
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()

			case <-r.Context().Done():
				slog.Info("TX SSE client context cancelled", "remoteAddr", r.RemoteAddr)
				return
			}
		}
	}
}

// GetPendingTxStates handles GET /api/send/pending.
// Returns all non-terminal transaction states (pending, broadcasting, confirming, uncertain).
// Optional query param: ?chain=BTC to filter by chain.
func GetPendingTxStates(deps *SendDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		chain := r.URL.Query().Get("chain")

		slog.Info("fetching pending tx states", "chainFilter", chain)

		var allStates []db.TxStateRow
		var err error

		if chain != "" {
			chain = strings.ToUpper(chain)
			allStates, err = deps.DB.GetPendingTxStates(chain)
		} else {
			allStates, err = deps.DB.GetAllPendingTxStates()
		}

		if err != nil {
			slog.Error("failed to fetch pending tx states", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch pending transactions")
			return
		}

		slog.Info("pending tx states fetched",
			"count", len(allStates),
			"chainFilter", chain,
			"duration", time.Since(start).Round(time.Millisecond),
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: allStates,
			Meta: &models.APIMeta{ExecutionTime: time.Since(start).Milliseconds()},
		})
	}
}

// GetResumeSummary handles GET /api/send/resume/{sweepID}.
// Returns the state of a sweep for the resume UI.
func GetResumeSummary(deps *SendDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sweepID := chi.URLParam(r, "sweepID")
		if sweepID == "" {
			writeError(w, http.StatusBadRequest, config.ErrorSweepNotFound, "sweep ID is required")
			return
		}

		slog.Info("resume summary requested", "sweepID", sweepID)

		// Check sweep exists.
		chain, token, _, err := deps.DB.GetSweepMeta(sweepID)
		if err != nil {
			slog.Error("failed to get sweep meta", "sweepID", sweepID, "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch sweep metadata")
			return
		}
		if chain == "" {
			slog.Warn("sweep not found", "sweepID", sweepID)
			writeError(w, http.StatusNotFound, config.ErrorSweepNotFound, "sweep not found")
			return
		}

		// Count by status.
		counts, err := deps.DB.CountTxStatesByStatus(sweepID)
		if err != nil {
			slog.Error("failed to count tx states", "sweepID", sweepID, "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to count transaction states")
			return
		}

		totalTxs := 0
		for _, c := range counts {
			totalTxs += c
		}

		summary := models.ResumeSummary{
			SweepID:   sweepID,
			Chain:     chain,
			Token:     token,
			TotalTxs:  totalTxs,
			Confirmed: counts[config.TxStateConfirmed],
			Failed:    counts[config.TxStateFailed],
			Uncertain: counts[config.TxStateUncertain],
			Pending:   counts[config.TxStatePending] + counts[config.TxStateBroadcasting] + counts[config.TxStateConfirming],
			ToRetry:   counts[config.TxStateFailed] + counts[config.TxStateUncertain],
		}

		slog.Info("resume summary generated",
			"sweepID", sweepID,
			"totalTxs", summary.TotalTxs,
			"confirmed", summary.Confirmed,
			"failed", summary.Failed,
			"uncertain", summary.Uncertain,
			"toRetry", summary.ToRetry,
		)

		writeJSON(w, http.StatusOK, models.APIResponse{Data: summary})
	}
}

// ExecuteResume handles POST /api/send/resume.
// Retries failed and uncertain transactions from a previous sweep.
func ExecuteResume(deps *SendDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req models.ResumeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Warn("invalid resume request body", "error", err)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidAddress, "invalid request body")
			return
		}

		slog.Info("resume execute requested",
			"sweepID", req.SweepID,
			"destination", req.Destination,
		)

		if req.SweepID == "" {
			writeError(w, http.StatusBadRequest, config.ErrorSweepNotFound, "sweep ID is required")
			return
		}

		// Get sweep metadata.
		chain, token, origDest, err := deps.DB.GetSweepMeta(req.SweepID)
		if err != nil {
			slog.Error("failed to get sweep meta", "sweepID", req.SweepID, "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch sweep metadata")
			return
		}
		if chain == "" {
			writeError(w, http.StatusNotFound, config.ErrorSweepNotFound, "sweep not found")
			return
		}

		// Use original destination if not overridden.
		dest := req.Destination
		if dest == "" {
			dest = origDest
		}

		chainModel := models.Chain(chain)
		tokenModel := models.Token(token)

		// Validate destination.
		if err := validateDestination(chainModel, dest, deps.NetParams); err != nil {
			writeError(w, http.StatusBadRequest, config.ErrorInvalidDestination, err.Error())
			return
		}

		// Get retryable tx states.
		retryable, err := deps.DB.GetRetryableTxStates(req.SweepID)
		if err != nil {
			slog.Error("failed to get retryable tx states", "sweepID", req.SweepID, "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch retryable transactions")
			return
		}

		if len(retryable) == 0 {
			slog.Info("no retryable transactions found", "sweepID", req.SweepID)
			writeJSON(w, http.StatusOK, models.APIResponse{
				Data: &models.UnifiedSendResult{
					Chain:      chainModel,
					Token:      tokenModel,
					TotalSwept: "0",
				},
			})
			return
		}

		// Acquire per-chain lock.
		mu := deps.ChainLocks[chainModel]
		if mu == nil {
			writeError(w, http.StatusInternalServerError, config.ErrorTxBroadcastFailed, "internal configuration error")
			return
		}
		if !mu.TryLock() {
			writeError(w, http.StatusConflict, config.ErrorSendBusy,
				fmt.Sprintf("send operation already in progress for %s", chain))
			return
		}
		defer mu.Unlock()

		// Build AddressWithBalance slice from retryable states.
		// Re-fetch current balances from DB.
		funded, err := deps.DB.GetFundedAddressesJoined(chainModel, tokenModel)
		if err != nil {
			slog.Error("failed to fetch funded addresses for resume", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch funded addresses")
			return
		}

		// Filter funded to only include addresses that need retry.
		retryIndices := make(map[int]bool, len(retryable))
		for _, rs := range retryable {
			retryIndices[rs.AddressIndex] = true
		}

		var retryFunded []models.AddressWithBalance
		for _, f := range funded {
			if retryIndices[f.AddressIndex] {
				retryFunded = append(retryFunded, f)
			}
		}

		slog.Info("resuming sweep",
			"sweepID", req.SweepID,
			"chain", chain,
			"token", token,
			"retryableCount", len(retryable),
			"fundedRetryCount", len(retryFunded),
			"destination", dest,
		)

		// Generate a new sweep ID for the retry batch.
		retrySweepID := tx.GenerateSweepID()

		// Build a SendRequest for dispatching.
		sendReq := models.SendRequest{
			Chain:       chainModel,
			Token:       tokenModel,
			Destination: dest,
		}

		result, err := executeSweep(r, deps, sendReq, retryFunded, retrySweepID)
		if err != nil {
			slog.Error("resume execute failed",
				"sweepID", req.SweepID,
				"retrySweepID", retrySweepID,
				"error", err,
				"duration", time.Since(start).Round(time.Millisecond),
			)
			writeError(w, http.StatusInternalServerError, config.ErrorTxBroadcastFailed, err.Error())
			return
		}

		slog.Info("resume execute completed",
			"sweepID", req.SweepID,
			"retrySweepID", retrySweepID,
			"successCount", result.SuccessCount,
			"failCount", result.FailCount,
			"totalSwept", result.TotalSwept,
			"duration", time.Since(start).Round(time.Millisecond),
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: result,
			Meta: &models.APIMeta{ExecutionTime: time.Since(start).Milliseconds()},
		})
	}
}

// DismissTxState handles POST /api/send/dismiss/{id}.
// Marks an uncertain TX as dismissed (user verified in explorer).
func DismissTxState(deps *SendDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, config.ErrorInvalidAddress, "transaction state ID is required")
			return
		}

		slog.Info("dismissing tx state", "id", id)

		if err := deps.DB.UpdateTxStatus(id, config.TxStateDismissed, "", ""); err != nil {
			slog.Error("failed to dismiss tx state", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to dismiss transaction")
			return
		}

		slog.Info("tx state dismissed", "id", id)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: map[string]string{"id": id, "status": config.TxStateDismissed},
		})
	}
}
