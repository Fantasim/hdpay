package tx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/mr-tron/base58"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/scanner"
)

// --- SOL RPC Client Interface ---

// SOLSignatureStatus represents the status of a Solana transaction signature.
type SOLSignatureStatus struct {
	Slot               uint64  `json:"slot"`
	Confirmations      *uint64 `json:"confirmations"`
	ConfirmationStatus *string `json:"confirmationStatus"`
	Err                interface{} `json:"err"`
}

// SOLRPCClient defines the minimal Solana RPC interface needed for transactions.
type SOLRPCClient interface {
	GetLatestBlockhash(ctx context.Context) (blockhash [32]byte, lastValidBlockHeight uint64, err error)
	SendTransaction(ctx context.Context, txBase64 string) (signature string, err error)
	GetSignatureStatuses(ctx context.Context, signatures []string) ([]SOLSignatureStatus, error)
	GetAccountInfo(ctx context.Context, address string) (exists bool, lamports uint64, err error)
	GetBalance(ctx context.Context, address string) (uint64, error)
}

// --- Default SOL RPC Client (JSON-RPC over HTTP) ---

// DefaultSOLRPCClient implements SOLRPCClient using Solana JSON-RPC.
type DefaultSOLRPCClient struct {
	httpClient *http.Client
	rpcURLs    []string
	currentIdx int
	mu         sync.Mutex
}

// NewDefaultSOLRPCClient creates a JSON-RPC client with round-robin URL selection.
func NewDefaultSOLRPCClient(httpClient *http.Client, rpcURLs []string) *DefaultSOLRPCClient {
	slog.Info("SOL RPC client created",
		"urlCount", len(rpcURLs),
		"urls", rpcURLs,
	)
	return &DefaultSOLRPCClient{
		httpClient: httpClient,
		rpcURLs:    rpcURLs,
	}
}

// solRPCRequest is a Solana JSON-RPC 2.0 request.
type solRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// solRPCGenericResponse is a generic JSON-RPC response with json.RawMessage result.
type solRPCGenericResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

// nextURL returns the next RPC URL in round-robin order.
func (c *DefaultSOLRPCClient) nextURL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	url := c.rpcURLs[c.currentIdx%len(c.rpcURLs)]
	c.currentIdx++
	return url
}

// doRPC sends a JSON-RPC request and returns the raw result.
func (c *DefaultSOLRPCClient) doRPC(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	url := c.nextURL()

	reqBody := solRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal RPC request: %w", err)
	}

	slog.Debug("SOL RPC request",
		"method", method,
		"url", url,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create RPC request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute RPC request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RPC HTTP %d from %s", resp.StatusCode, url)
	}

	var rpcResp solRPCGenericResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// GetLatestBlockhash fetches a recent blockhash for transaction building.
func (c *DefaultSOLRPCClient) GetLatestBlockhash(ctx context.Context) ([32]byte, uint64, error) {
	result, err := c.doRPC(ctx, "getLatestBlockhash", []interface{}{
		map[string]string{"commitment": "confirmed"},
	})
	if err != nil {
		return [32]byte{}, 0, fmt.Errorf("getLatestBlockhash: %w", err)
	}

	var parsed struct {
		Value struct {
			Blockhash            string `json:"blockhash"`
			LastValidBlockHeight uint64 `json:"lastValidBlockHeight"`
		} `json:"value"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return [32]byte{}, 0, fmt.Errorf("parse getLatestBlockhash: %w", err)
	}

	hashBytes, err := base58.Decode(parsed.Value.Blockhash)
	if err != nil {
		return [32]byte{}, 0, fmt.Errorf("decode blockhash: %w", err)
	}
	if len(hashBytes) != 32 {
		return [32]byte{}, 0, fmt.Errorf("invalid blockhash length: %d", len(hashBytes))
	}

	var blockhash [32]byte
	copy(blockhash[:], hashBytes)

	slog.Debug("fetched SOL blockhash",
		"blockhash", parsed.Value.Blockhash,
		"lastValidBlockHeight", parsed.Value.LastValidBlockHeight,
	)

	return blockhash, parsed.Value.LastValidBlockHeight, nil
}

// SendTransaction broadcasts a base64-encoded signed transaction.
func (c *DefaultSOLRPCClient) SendTransaction(ctx context.Context, txBase64 string) (string, error) {
	result, err := c.doRPC(ctx, "sendTransaction", []interface{}{
		txBase64,
		map[string]interface{}{
			"encoding":            "base64",
			"preflightCommitment": "confirmed",
		},
	})
	if err != nil {
		return "", fmt.Errorf("sendTransaction: %w", err)
	}

	var signature string
	if err := json.Unmarshal(result, &signature); err != nil {
		return "", fmt.Errorf("parse sendTransaction result: %w", err)
	}

	slog.Info("SOL transaction sent", "signature", signature)
	return signature, nil
}

// GetSignatureStatuses fetches the status of one or more transaction signatures.
func (c *DefaultSOLRPCClient) GetSignatureStatuses(ctx context.Context, signatures []string) ([]SOLSignatureStatus, error) {
	result, err := c.doRPC(ctx, "getSignatureStatuses", []interface{}{
		signatures,
		map[string]bool{"searchTransactionHistory": true},
	})
	if err != nil {
		return nil, fmt.Errorf("getSignatureStatuses: %w", err)
	}

	var parsed struct {
		Value []*SOLSignatureStatus `json:"value"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("parse getSignatureStatuses: %w", err)
	}

	statuses := make([]SOLSignatureStatus, len(parsed.Value))
	for i, s := range parsed.Value {
		if s != nil {
			statuses[i] = *s
		}
	}

	return statuses, nil
}

// GetAccountInfo checks if an account exists and returns its lamport balance.
func (c *DefaultSOLRPCClient) GetAccountInfo(ctx context.Context, address string) (bool, uint64, error) {
	result, err := c.doRPC(ctx, "getAccountInfo", []interface{}{
		address,
		map[string]string{"encoding": "base64"},
	})
	if err != nil {
		return false, 0, fmt.Errorf("getAccountInfo for %s: %w", address, err)
	}

	var parsed struct {
		Value *struct {
			Lamports uint64 `json:"lamports"`
		} `json:"value"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return false, 0, fmt.Errorf("parse getAccountInfo: %w", err)
	}

	if parsed.Value == nil {
		return false, 0, nil
	}

	return true, parsed.Value.Lamports, nil
}

// GetBalance fetches the SOL balance (lamports) for an address.
func (c *DefaultSOLRPCClient) GetBalance(ctx context.Context, address string) (uint64, error) {
	result, err := c.doRPC(ctx, "getBalance", []interface{}{
		address,
		map[string]string{"commitment": "confirmed"},
	})
	if err != nil {
		return 0, fmt.Errorf("getBalance for %s: %w", address, err)
	}

	var parsed struct {
		Value uint64 `json:"value"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return 0, fmt.Errorf("parse getBalance: %w", err)
	}

	slog.Debug("SOL balance fetched",
		"address", address,
		"lamports", parsed.Value,
	)

	return parsed.Value, nil
}

// --- Confirmation Polling ---

// WaitForSOLConfirmation polls getSignatureStatuses until the transaction is confirmed or fails.
func WaitForSOLConfirmation(ctx context.Context, client SOLRPCClient, signature string) (uint64, error) {
	slog.Debug("waiting for SOL confirmation", "signature", signature)

	pollCtx, cancel := context.WithTimeout(ctx, config.SOLConfirmationTimeout)
	defer cancel()

	for {
		statuses, err := client.GetSignatureStatuses(pollCtx, []string{signature})
		if err != nil {
			slog.Warn("SOL confirmation poll error", "signature", signature, "error", err)
			// Transient RPC error — wait and retry.
		} else if len(statuses) > 0 {
			status := statuses[0]

			// Check for on-chain error.
			if status.Err != nil {
				slog.Error("SOL transaction failed on-chain",
					"signature", signature,
					"error", status.Err,
				)
				return 0, fmt.Errorf("%w: %v", config.ErrSOLTxFailed, status.Err)
			}

			// Check confirmation status.
			if status.ConfirmationStatus != nil {
				cs := *status.ConfirmationStatus
				if cs == "confirmed" || cs == "finalized" {
					slog.Info("SOL transaction confirmed",
						"signature", signature,
						"slot", status.Slot,
						"confirmationStatus", cs,
					)
					return status.Slot, nil
				}
			}
		}

		// Not confirmed yet — wait and retry.
		select {
		case <-pollCtx.Done():
			return 0, fmt.Errorf("%w: signature %s", config.ErrSOLConfirmationTimeout, signature)
		case <-time.After(config.SOLConfirmationPollInterval):
			slog.Debug("SOL confirmation not ready, polling again", "signature", signature)
		}
	}
}

// --- SOL Consolidation Service ---

// SOLConsolidationService orchestrates SOL native and SPL token sweeps.
type SOLConsolidationService struct {
	keyService *KeyService
	rpcClient  SOLRPCClient
	database   *db.DB
	network    string
}

// NewSOLConsolidationService creates the SOL consolidation orchestrator.
func NewSOLConsolidationService(
	keyService *KeyService,
	rpcClient SOLRPCClient,
	database *db.DB,
	network string,
) *SOLConsolidationService {
	slog.Info("SOL consolidation service created", "network", network)
	return &SOLConsolidationService{
		keyService: keyService,
		rpcClient:  rpcClient,
		database:   database,
		network:    network,
	}
}

// PreviewNativeSweep calculates the expected result of a native SOL consolidation.
func (s *SOLConsolidationService) PreviewNativeSweep(
	ctx context.Context,
	addresses []models.AddressWithBalance,
	destAddress string,
) (*models.SOLSendPreview, error) {
	slog.Info("SOL native sweep preview",
		"addressCount", len(addresses),
		"destAddress", destAddress,
	)

	feePerTx := uint64(config.SOLBaseTransactionFee)
	var totalAmount, totalFee uint64
	inputCount := 0

	for _, addr := range addresses {
		bal, err := strconv.ParseUint(addr.NativeBalance, 10, 64)
		if err != nil || bal == 0 {
			continue
		}
		if bal <= feePerTx {
			slog.Debug("SOL preview: skipping address with insufficient balance",
				"address", addr.Address,
				"balance", bal,
				"fee", feePerTx,
			)
			continue
		}
		sweepable := bal - feePerTx
		totalAmount += sweepable
		totalFee += feePerTx
		inputCount++
	}

	preview := &models.SOLSendPreview{
		Chain:       models.ChainSOL,
		Token:       models.TokenNative,
		InputCount:  inputCount,
		TotalAmount: strconv.FormatUint(totalAmount+totalFee, 10), // gross balance
		TotalFee:    strconv.FormatUint(totalFee, 10),
		NetAmount:   strconv.FormatUint(totalAmount, 10),
		DestAddress: destAddress,
	}

	slog.Info("SOL native sweep preview complete",
		"inputCount", preview.InputCount,
		"totalAmount", preview.TotalAmount,
		"totalFee", preview.TotalFee,
		"netAmount", preview.NetAmount,
	)

	return preview, nil
}

// ExecuteNativeSweep performs sequential SOL transfers from funded addresses to a destination.
func (s *SOLConsolidationService) ExecuteNativeSweep(
	ctx context.Context,
	addresses []models.AddressWithBalance,
	destAddress string,
) (*models.SOLSendResult, error) {
	slog.Info("SOL native sweep execute",
		"addressCount", len(addresses),
		"destAddress", destAddress,
	)
	start := time.Now()

	destPubKey, err := SolPublicKeyFromBase58(destAddress)
	if err != nil {
		return nil, fmt.Errorf("parse destination address: %w", err)
	}

	feePerTx := uint64(config.SOLBaseTransactionFee)

	result := &models.SOLSendResult{
		Chain: models.ChainSOL,
		Token: models.TokenNative,
	}
	var totalSwept uint64

	for _, addr := range addresses {
		if err := ctx.Err(); err != nil {
			slog.Warn("SOL native sweep cancelled", "error", err)
			break
		}

		txResult := s.sweepNativeAddress(ctx, addr, destPubKey, feePerTx)
		result.TxResults = append(result.TxResults, txResult)

		if txResult.Status == "confirmed" {
			result.SuccessCount++
			amount, _ := strconv.ParseUint(txResult.Amount, 10, 64)
			totalSwept += amount
		} else {
			result.FailCount++
		}
	}

	result.TotalSwept = strconv.FormatUint(totalSwept, 10)

	slog.Info("SOL native sweep complete",
		"successCount", result.SuccessCount,
		"failCount", result.FailCount,
		"totalSwept", result.TotalSwept,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return result, nil
}

// sweepNativeAddress sends SOL from a single address to the destination.
func (s *SOLConsolidationService) sweepNativeAddress(
	ctx context.Context,
	addr models.AddressWithBalance,
	dest SolPublicKey,
	feePerTx uint64,
) models.SOLTxResult {
	txResult := models.SOLTxResult{
		AddressIndex: addr.AddressIndex,
		FromAddress:  addr.Address,
	}

	// Get real-time balance.
	balance, err := s.rpcClient.GetBalance(ctx, addr.Address)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("get balance: %s", err)
		slog.Error("SOL sweep: failed to get balance", "address", addr.Address, "error", err)
		return txResult
	}

	if balance <= feePerTx {
		txResult.Status = "failed"
		txResult.Error = "balance too low to cover fee"
		slog.Warn("SOL sweep: insufficient balance for fee",
			"address", addr.Address,
			"balance", balance,
			"fee", feePerTx,
		)
		return txResult
	}

	sendAmount := balance - feePerTx

	// Derive private key.
	privKey, err := s.keyService.DeriveSOLPrivateKey(ctx, uint32(addr.AddressIndex))
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("derive key: %s", err)
		slog.Error("SOL sweep: key derivation failed", "index", addr.AddressIndex, "error", err)
		return txResult
	}

	// Verify derived address matches.
	derivedPubKey := privKey.Public().(ed25519.PublicKey)
	derivedAddr := base58.Encode(derivedPubKey)
	if derivedAddr != addr.Address {
		txResult.Status = "failed"
		txResult.Error = "derived address mismatch"
		slog.Error("SOL sweep: address mismatch",
			"expected", addr.Address,
			"derived", derivedAddr,
			"index", addr.AddressIndex,
		)
		return txResult
	}

	// Fetch recent blockhash.
	blockhash, _, err := s.rpcClient.GetLatestBlockhash(ctx)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("get blockhash: %s", err)
		slog.Error("SOL sweep: blockhash fetch failed", "error", err)
		return txResult
	}

	// Build transaction.
	var fromPubKey SolPublicKey
	copy(fromPubKey[:], derivedPubKey)

	ix := BuildSystemTransferInstruction(fromPubKey, dest, sendAmount)
	signers := map[SolPublicKey]ed25519.PrivateKey{
		fromPubKey: privKey,
	}

	txBytes, txSig, err := BuildAndSerializeTransaction(fromPubKey, []SolInstruction{ix}, blockhash, signers)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("build tx: %s", err)
		slog.Error("SOL sweep: build transaction failed", "address", addr.Address, "error", err)
		return txResult
	}

	slog.Info("SOL sweep: broadcasting native transfer",
		"from", addr.Address,
		"to", dest.ToBase58(),
		"amount", sendAmount,
		"txSize", len(txBytes),
	)

	// Broadcast.
	txBase64 := base64.StdEncoding.EncodeToString(txBytes)
	signature, err := s.rpcClient.SendTransaction(ctx, txBase64)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("broadcast: %s", err)
		slog.Error("SOL sweep: broadcast failed", "address", addr.Address, "error", err)
		return txResult
	}

	// Use the returned signature (should match txSig, but trust the RPC).
	if signature != "" {
		txResult.TxSignature = signature
	} else {
		txResult.TxSignature = txSig
	}
	txResult.Amount = strconv.FormatUint(sendAmount, 10)

	slog.Info("SOL sweep: TX broadcast, waiting for confirmation",
		"signature", txResult.TxSignature,
		"from", addr.Address,
	)

	// Wait for confirmation.
	slot, err := WaitForSOLConfirmation(ctx, s.rpcClient, txResult.TxSignature)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("confirmation: %s", err)
		slog.Error("SOL sweep: confirmation failed", "signature", txResult.TxSignature, "error", err)
		// TX was broadcast — record as pending.
		s.recordSOLTransaction(addr, txResult.TxSignature, txResult.Amount, dest.ToBase58(), models.TokenNative, "pending")
		return txResult
	}

	txResult.Status = "confirmed"
	txResult.Slot = slot

	s.recordSOLTransaction(addr, txResult.TxSignature, txResult.Amount, dest.ToBase58(), models.TokenNative, "confirmed")

	slog.Info("SOL sweep: native transfer confirmed",
		"signature", txResult.TxSignature,
		"from", addr.Address,
		"amount", sendAmount,
		"slot", slot,
	)

	return txResult
}

// PreviewTokenSweep calculates the expected result of a SPL token consolidation.
func (s *SOLConsolidationService) PreviewTokenSweep(
	ctx context.Context,
	addresses []models.AddressWithBalance,
	destAddress string,
	token models.Token,
	mint string,
) (*models.SOLSendPreview, error) {
	slog.Info("SOL token sweep preview",
		"addressCount", len(addresses),
		"destAddress", destAddress,
		"token", token,
		"mint", mint,
	)

	feePerTx := uint64(config.SOLBaseTransactionFee)
	var totalAmount uint64
	var totalFee uint64
	inputCount := 0

	for _, addr := range addresses {
		tokenBal := findTokenBalance(addr, token)
		if tokenBal == 0 {
			continue
		}

		// Check that the address has enough SOL for the TX fee.
		nativeBal, err := strconv.ParseUint(addr.NativeBalance, 10, 64)
		if err != nil || nativeBal < feePerTx {
			slog.Debug("SOL token preview: skipping address with insufficient SOL for fee",
				"address", addr.Address,
				"nativeBalance", addr.NativeBalance,
				"fee", feePerTx,
			)
			continue
		}

		totalAmount += tokenBal
		totalFee += feePerTx
		inputCount++
	}

	// Check if destination ATA exists.
	destATA, err := scanner.DeriveATA(destAddress, mint)
	if err != nil {
		return nil, fmt.Errorf("derive destination ATA: %w", err)
	}

	needATA := false
	ataRent := uint64(0)
	exists, _, err := s.rpcClient.GetAccountInfo(ctx, destATA)
	if err != nil {
		slog.Warn("SOL token preview: failed to check dest ATA existence",
			"destATA", destATA,
			"error", err,
		)
	} else if !exists {
		needATA = true
		ataRent = config.SOLATARentLamports
		totalFee += ataRent // First TX pays rent for ATA creation.
	}

	preview := &models.SOLSendPreview{
		Chain:           models.ChainSOL,
		Token:           token,
		InputCount:      inputCount,
		TotalAmount:     strconv.FormatUint(totalAmount, 10),
		TotalFee:        strconv.FormatUint(totalFee, 10),
		NetAmount:       strconv.FormatUint(totalAmount, 10), // tokens not reduced by fee
		DestAddress:     destAddress,
		NeedATACreation: needATA,
		ATARentCost:     strconv.FormatUint(ataRent, 10),
	}

	slog.Info("SOL token sweep preview complete",
		"inputCount", preview.InputCount,
		"totalAmount", preview.TotalAmount,
		"totalFee", preview.TotalFee,
		"needATACreation", preview.NeedATACreation,
	)

	return preview, nil
}

// ExecuteTokenSweep performs sequential SPL token transfers from funded addresses.
func (s *SOLConsolidationService) ExecuteTokenSweep(
	ctx context.Context,
	addresses []models.AddressWithBalance,
	destAddress string,
	token models.Token,
	mint string,
) (*models.SOLSendResult, error) {
	slog.Info("SOL token sweep execute",
		"addressCount", len(addresses),
		"destAddress", destAddress,
		"token", token,
		"mint", mint,
	)
	start := time.Now()

	mintPubKey, err := SolPublicKeyFromBase58(mint)
	if err != nil {
		return nil, fmt.Errorf("parse mint address: %w", err)
	}

	destPubKey, err := SolPublicKeyFromBase58(destAddress)
	if err != nil {
		return nil, fmt.Errorf("parse destination address: %w", err)
	}

	// Derive destination ATA.
	destATAStr, err := scanner.DeriveATA(destAddress, mint)
	if err != nil {
		return nil, fmt.Errorf("derive destination ATA: %w", err)
	}
	destATAPubKey, err := SolPublicKeyFromBase58(destATAStr)
	if err != nil {
		return nil, fmt.Errorf("parse destination ATA: %w", err)
	}

	// Check if destination ATA exists.
	destATAExists, _, err := s.rpcClient.GetAccountInfo(ctx, destATAStr)
	if err != nil {
		return nil, fmt.Errorf("check destination ATA: %w", err)
	}

	slog.Info("SOL token sweep: destination ATA",
		"destATA", destATAStr,
		"exists", destATAExists,
	)

	feePerTx := uint64(config.SOLBaseTransactionFee)

	result := &models.SOLSendResult{
		Chain: models.ChainSOL,
		Token: token,
	}
	var totalSwept uint64

	for _, addr := range addresses {
		if err := ctx.Err(); err != nil {
			slog.Warn("SOL token sweep cancelled", "error", err)
			break
		}

		tokenBal := findTokenBalance(addr, token)
		if tokenBal == 0 {
			continue
		}

		txResult := s.sweepTokenAddress(ctx, addr, destPubKey, destATAPubKey, mintPubKey, tokenBal, feePerTx, token, mint, !destATAExists)
		result.TxResults = append(result.TxResults, txResult)

		if txResult.Status == "confirmed" {
			result.SuccessCount++
			amount, _ := strconv.ParseUint(txResult.Amount, 10, 64)
			totalSwept += amount
			// After first successful tx with ATA creation, mark it as existing.
			if !destATAExists {
				destATAExists = true
				slog.Info("SOL token sweep: destination ATA created", "destATA", destATAStr)
			}
		} else {
			result.FailCount++
		}
	}

	result.TotalSwept = strconv.FormatUint(totalSwept, 10)

	slog.Info("SOL token sweep complete",
		"token", token,
		"successCount", result.SuccessCount,
		"failCount", result.FailCount,
		"totalSwept", result.TotalSwept,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return result, nil
}

// sweepTokenAddress sends SPL tokens from a single address to the destination.
func (s *SOLConsolidationService) sweepTokenAddress(
	ctx context.Context,
	addr models.AddressWithBalance,
	destPubKey SolPublicKey,
	destATAPubKey SolPublicKey,
	mintPubKey SolPublicKey,
	tokenAmount uint64,
	feePerTx uint64,
	token models.Token,
	mint string,
	needCreateATA bool,
) models.SOLTxResult {
	txResult := models.SOLTxResult{
		AddressIndex: addr.AddressIndex,
		FromAddress:  addr.Address,
	}

	// Check native SOL balance for fee.
	nativeBal, err := s.rpcClient.GetBalance(ctx, addr.Address)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("get balance: %s", err)
		slog.Error("SOL token sweep: failed to get balance", "address", addr.Address, "error", err)
		return txResult
	}

	minRequired := feePerTx
	if needCreateATA {
		minRequired += config.SOLATARentLamports
	}

	if nativeBal < minRequired {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("insufficient SOL for fee: have %d, need %d", nativeBal, minRequired)
		slog.Warn("SOL token sweep: insufficient SOL for fee",
			"address", addr.Address,
			"balance", nativeBal,
			"required", minRequired,
		)
		return txResult
	}

	// Derive private key.
	privKey, err := s.keyService.DeriveSOLPrivateKey(ctx, uint32(addr.AddressIndex))
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("derive key: %s", err)
		slog.Error("SOL token sweep: key derivation failed", "index", addr.AddressIndex, "error", err)
		return txResult
	}

	derivedPubKey := privKey.Public().(ed25519.PublicKey)
	derivedAddr := base58.Encode(derivedPubKey)
	if derivedAddr != addr.Address {
		txResult.Status = "failed"
		txResult.Error = "derived address mismatch"
		slog.Error("SOL token sweep: address mismatch",
			"expected", addr.Address,
			"derived", derivedAddr,
		)
		return txResult
	}

	var fromPubKey SolPublicKey
	copy(fromPubKey[:], derivedPubKey)

	// Derive source ATA.
	sourceATAStr, err := scanner.DeriveATA(addr.Address, mint)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("derive source ATA: %s", err)
		slog.Error("SOL token sweep: source ATA derivation failed", "address", addr.Address, "error", err)
		return txResult
	}
	sourceATAPubKey, err := SolPublicKeyFromBase58(sourceATAStr)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("parse source ATA: %s", err)
		return txResult
	}

	// Fetch recent blockhash.
	blockhash, _, err := s.rpcClient.GetLatestBlockhash(ctx)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("get blockhash: %s", err)
		slog.Error("SOL token sweep: blockhash fetch failed", "error", err)
		return txResult
	}

	// Build instructions.
	var instructions []SolInstruction

	if needCreateATA {
		createATAIx := BuildCreateATAInstruction(fromPubKey, destATAPubKey, destPubKey, mintPubKey)
		instructions = append(instructions, createATAIx)
		slog.Info("SOL token sweep: including CreateATA instruction",
			"payer", addr.Address,
			"destATA", destATAPubKey.ToBase58(),
		)
	}

	transferIx := BuildSPLTransferInstruction(sourceATAPubKey, destATAPubKey, fromPubKey, tokenAmount)
	instructions = append(instructions, transferIx)

	signers := map[SolPublicKey]ed25519.PrivateKey{
		fromPubKey: privKey,
	}

	txBytes, txSig, err := BuildAndSerializeTransaction(fromPubKey, instructions, blockhash, signers)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("build tx: %s", err)
		slog.Error("SOL token sweep: build transaction failed", "address", addr.Address, "error", err)
		return txResult
	}

	slog.Info("SOL token sweep: broadcasting transfer",
		"from", addr.Address,
		"token", token,
		"amount", tokenAmount,
		"txSize", len(txBytes),
		"includesCreateATA", needCreateATA,
	)

	// Broadcast.
	txBase64 := base64.StdEncoding.EncodeToString(txBytes)
	signature, err := s.rpcClient.SendTransaction(ctx, txBase64)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("broadcast: %s", err)
		slog.Error("SOL token sweep: broadcast failed", "address", addr.Address, "error", err)
		return txResult
	}

	if signature != "" {
		txResult.TxSignature = signature
	} else {
		txResult.TxSignature = txSig
	}
	txResult.Amount = strconv.FormatUint(tokenAmount, 10)

	slog.Info("SOL token sweep: TX broadcast, waiting for confirmation",
		"signature", txResult.TxSignature,
		"from", addr.Address,
	)

	// Wait for confirmation.
	slot, err := WaitForSOLConfirmation(ctx, s.rpcClient, txResult.TxSignature)
	if err != nil {
		txResult.Status = "failed"
		txResult.Error = fmt.Sprintf("confirmation: %s", err)
		slog.Error("SOL token sweep: confirmation failed", "signature", txResult.TxSignature, "error", err)
		s.recordSOLTransaction(addr, txResult.TxSignature, txResult.Amount, destPubKey.ToBase58(), token, "pending")
		return txResult
	}

	txResult.Status = "confirmed"
	txResult.Slot = slot

	s.recordSOLTransaction(addr, txResult.TxSignature, txResult.Amount, destPubKey.ToBase58(), token, "confirmed")

	slog.Info("SOL token sweep: transfer confirmed",
		"signature", txResult.TxSignature,
		"from", addr.Address,
		"token", token,
		"amount", tokenAmount,
		"slot", slot,
	)

	return txResult
}

// --- Helpers ---

// findTokenBalance extracts the token balance for a specific token from an AddressWithBalance.
func findTokenBalance(addr models.AddressWithBalance, token models.Token) uint64 {
	for _, tb := range addr.TokenBalances {
		if tb.Symbol == token {
			bal, err := strconv.ParseUint(tb.Balance, 10, 64)
			if err != nil || bal == 0 {
				return 0
			}
			return bal
		}
	}
	return 0
}

// recordSOLTransaction stores a SOL transaction in the database.
func (s *SOLConsolidationService) recordSOLTransaction(
	addr models.AddressWithBalance,
	txSignature string,
	amount string,
	destAddr string,
	token models.Token,
	status string,
) {
	if s.database == nil {
		slog.Warn("SOL transaction not recorded: database not configured",
			"txSignature", txSignature,
			"addressIndex", addr.AddressIndex,
		)
		return
	}

	txRecord := models.Transaction{
		Chain:        models.ChainSOL,
		AddressIndex: addr.AddressIndex,
		TxHash:       txSignature,
		Direction:    "send",
		Token:        token,
		Amount:       amount,
		FromAddress:  addr.Address,
		ToAddress:    destAddr,
		Status:       status,
	}

	if _, err := s.database.InsertTransaction(txRecord); err != nil {
		slog.Error("failed to record SOL transaction in DB",
			"txSignature", txSignature,
			"addressIndex", addr.AddressIndex,
			"error", err,
		)
	}
}
