package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strings"

	hdconfig "github.com/Fantasim/hdpay/internal/config"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
)

// Solana JSON-RPC response types.

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Error   *rpcError       `json:"error,omitempty"`
	Result  json.RawMessage `json:"result"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// getSignaturesForAddress response
type signatureInfo struct {
	Signature          string      `json:"signature"`
	Slot               uint64      `json:"slot"`
	BlockTime          *int64      `json:"blockTime"`
	ConfirmationStatus string      `json:"confirmationStatus"`
	Err                interface{} `json:"err"`
}

// getTransaction response
type transactionResult struct {
	Slot        uint64          `json:"slot"`
	BlockTime   *int64          `json:"blockTime"`
	Transaction txEnvelope      `json:"transaction"`
	Meta        txMeta          `json:"meta"`
	Version     json.RawMessage `json:"version"`
}

type txEnvelope struct {
	Message    txMessage `json:"message"`
	Signatures []string  `json:"signatures"`
}

type txMessage struct {
	AccountKeys []string `json:"accountKeys"`
}

type txMeta struct {
	Err               interface{}      `json:"err"`
	Fee               uint64           `json:"fee"`
	PreBalances       []int64          `json:"preBalances"`
	PostBalances      []int64          `json:"postBalances"`
	PreTokenBalances  []tokenBalance   `json:"preTokenBalances"`
	PostTokenBalances []tokenBalance   `json:"postTokenBalances"`
	LoadedAddresses   *loadedAddresses `json:"loadedAddresses"`
}

type loadedAddresses struct {
	Writable []string `json:"writable"`
	Readonly []string `json:"readonly"`
}

type tokenBalance struct {
	AccountIndex  int           `json:"accountIndex"`
	Mint          string        `json:"mint"`
	Owner         string        `json:"owner"`
	UITokenAmount uiTokenAmount `json:"uiTokenAmount"`
}

type uiTokenAmount struct {
	Amount         string  `json:"amount"` // raw integer string
	Decimals       int     `json:"decimals"`
	UIAmount       *float64 `json:"uiAmount"`
	UIAmountString string  `json:"uiAmountString"`
}

// getSignatureStatuses response
type signatureStatusesResult struct {
	Context struct {
		Slot uint64 `json:"slot"`
	} `json:"context"`
	Value []*signatureStatus `json:"value"` // nil entries for unknown signatures
}

type signatureStatus struct {
	Slot               uint64      `json:"slot"`
	Confirmations      *int        `json:"confirmations"` // nil = finalized
	Err                interface{} `json:"err"`
	ConfirmationStatus string      `json:"confirmationStatus"`
}

// SOL signature page size for getSignaturesForAddress.
const solSignaturePageSize = 20

// SolanaRPCProvider detects SOL transactions via Solana JSON-RPC.
type SolanaRPCProvider struct {
	client  *http.Client
	rpcURL  string
	network string
}

// NewSolanaRPCProvider creates a Solana RPC provider for the given network.
func NewSolanaRPCProvider(client *http.Client, network string) *SolanaRPCProvider {
	rpcURL := hdconfig.SolanaMainnetRPCURL
	if network == "testnet" {
		rpcURL = hdconfig.SolanaDevnetRPCURL
	}

	slog.Info("solana rpc provider created",
		"network", network,
		"rpcURL", rpcURL,
	)

	return &SolanaRPCProvider{
		client:  client,
		rpcURL:  rpcURL,
		network: network,
	}
}

// NewHeliusProvider creates a Helius-backed Solana RPC provider.
func NewHeliusProvider(client *http.Client, network, apiKey string) *SolanaRPCProvider {
	rpcURL := hdconfig.HeliusMainnetRPCURL
	if apiKey != "" {
		rpcURL += "?api-key=" + apiKey
	}
	// Helius doesn't have a separate devnet endpoint in our constants,
	// so fall back to Solana devnet for testnet mode.
	if network == "testnet" {
		rpcURL = hdconfig.SolanaDevnetRPCURL
	}

	slog.Info("helius provider created",
		"network", network,
		"rpcURL", rpcURL,
	)

	return &SolanaRPCProvider{
		client:  client,
		rpcURL:  rpcURL,
		network: network,
	}
}

func (p *SolanaRPCProvider) Name() string { return "solana-rpc" }
func (p *SolanaRPCProvider) Chain() string { return "SOL" }

// FetchTransactions detects incoming SOL and SPL token transfers for an address since cutoffUnix.
// A single Solana transaction can contain both native SOL and SPL token transfers,
// producing separate RawTransaction entries with composite tx_hash: "signature:TOKEN".
func (p *SolanaRPCProvider) FetchTransactions(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error) {
	// Step 1: Get recent signatures for the address
	sigs, err := p.getSignaturesForAddress(ctx, address, cutoffUnix)
	if err != nil {
		return nil, fmt.Errorf("solana get signatures: %w", err)
	}

	slog.Debug("SOL signatures fetched",
		"address", address,
		"count", len(sigs),
	)

	// Step 2: For each signature, fetch the full transaction and parse transfers
	var result []RawTransaction
	for _, sig := range sigs {
		txs, err := p.parseTransaction(ctx, sig.Signature, address)
		if err != nil {
			slog.Warn("SOL failed to parse transaction, skipping",
				"signature", sig.Signature,
				"error", err,
			)
			continue
		}

		// Set confirmation status from the signature info
		for i := range txs {
			if sig.BlockTime != nil {
				txs[i].BlockTime = *sig.BlockTime
			}
			txs[i].Confirmed = sig.ConfirmationStatus == pollerconfig.SOLCommitment
		}

		result = append(result, txs...)
	}

	slog.Info("SOL transactions fetched",
		"provider", p.Name(),
		"address", address,
		"count", len(result),
	)

	return result, nil
}

// CheckConfirmation checks whether a SOL transaction is finalized.
// txHash may be a composite hash ("signature:TOKEN") — the base signature is extracted.
func (p *SolanaRPCProvider) CheckConfirmation(ctx context.Context, txHash string, _ int64) (bool, int, error) {
	// Extract base signature from composite tx_hash
	signature := extractBaseSignature(txHash)

	slog.Debug("checking SOL tx confirmation",
		"provider", p.Name(),
		"signature", signature,
		"compositeTxHash", txHash,
	)

	status, err := p.getSignatureStatus(ctx, signature)
	if err != nil {
		return false, 0, fmt.Errorf("solana check confirmation: %w", err)
	}

	if status == nil {
		slog.Warn("SOL signature status not found",
			"signature", signature,
		)
		return false, 0, nil
	}

	confirmed := status.ConfirmationStatus == pollerconfig.SOLCommitment
	confirmations := 0
	if status.Confirmations != nil {
		confirmations = *status.Confirmations
	} else if confirmed {
		confirmations = 1 // finalized = at least 1
	}

	slog.Debug("SOL tx confirmation status",
		"signature", signature,
		"confirmationStatus", status.ConfirmationStatus,
		"confirmed", confirmed,
	)

	return confirmed, confirmations, nil
}

// GetCurrentBlock is not used for SOL confirmation (SOL uses finalization status).
func (p *SolanaRPCProvider) GetCurrentBlock(_ context.Context) (uint64, error) {
	return 0, nil
}

// getSignaturesForAddress fetches recent signatures for an address, filtering by cutoff.
func (p *SolanaRPCProvider) getSignaturesForAddress(ctx context.Context, address string, cutoffUnix int64) ([]signatureInfo, error) {
	params := map[string]interface{}{
		"limit":      solSignaturePageSize,
		"commitment": "confirmed",
	}

	body, err := p.rpcCall(ctx, "getSignaturesForAddress", []interface{}{address, params})
	if err != nil {
		return nil, err
	}

	var sigs []signatureInfo
	if err := json.Unmarshal(body, &sigs); err != nil {
		return nil, fmt.Errorf("parse getSignaturesForAddress: %w", err)
	}

	// Filter by cutoff and skip failed transactions
	var result []signatureInfo
	for _, sig := range sigs {
		// Skip if older than cutoff
		if sig.BlockTime != nil && *sig.BlockTime < cutoffUnix {
			break // ordered newest first
		}

		// Skip failed transactions
		if sig.Err != nil {
			slog.Debug("skipping failed SOL tx",
				"signature", sig.Signature,
				"err", sig.Err,
			)
			continue
		}

		result = append(result, sig)
	}

	return result, nil
}

// parseTransaction fetches a full transaction and extracts incoming transfers to the address.
// Returns separate RawTransaction entries for native SOL and each SPL token transfer.
func (p *SolanaRPCProvider) parseTransaction(ctx context.Context, signature, address string) ([]RawTransaction, error) {
	params := map[string]interface{}{
		"commitment":                     "confirmed",
		"maxSupportedTransactionVersion": 0,
	}

	body, err := p.rpcCall(ctx, "getTransaction", []interface{}{signature, params})
	if err != nil {
		return nil, err
	}

	// Handle null result (transaction not found)
	if string(body) == "null" {
		return nil, fmt.Errorf("transaction %s not found", signature)
	}

	var tx transactionResult
	if err := json.Unmarshal(body, &tx); err != nil {
		return nil, fmt.Errorf("parse getTransaction: %w", err)
	}

	// Skip failed transactions
	if tx.Meta.Err != nil {
		return nil, nil
	}

	// Build full account list (accountKeys + loadedAddresses)
	allAccounts := make([]string, len(tx.Transaction.Message.AccountKeys))
	copy(allAccounts, tx.Transaction.Message.AccountKeys)
	if tx.Meta.LoadedAddresses != nil {
		allAccounts = append(allAccounts, tx.Meta.LoadedAddresses.Writable...)
		allAccounts = append(allAccounts, tx.Meta.LoadedAddresses.Readonly...)
	}

	var result []RawTransaction

	// 1. Detect native SOL transfer
	addrIdx := -1
	for i, acc := range allAccounts {
		if acc == address {
			addrIdx = i
			break
		}
	}

	if addrIdx >= 0 && addrIdx < len(tx.Meta.PreBalances) && addrIdx < len(tx.Meta.PostBalances) {
		delta := tx.Meta.PostBalances[addrIdx] - tx.Meta.PreBalances[addrIdx]
		if delta > 0 {
			amountRaw := fmt.Sprintf("%d", delta)
			amountHuman := lamportsToHuman(delta)

			slog.Debug("SOL native transfer detected",
				"signature", signature,
				"address", address,
				"lamports", delta,
				"amountSOL", amountHuman,
			)

			result = append(result, RawTransaction{
				TxHash:      signature + ":SOL",
				Token:       "SOL",
				AmountRaw:   amountRaw,
				AmountHuman: amountHuman,
				Decimals:    hdconfig.SOLDecimals,
			})
		}
	}

	// 2. Detect SPL token transfers (USDC, USDT)
	usdcMint, usdtMint := p.tokenMints()
	tokenTransfers := p.detectTokenTransfers(tx, address, usdcMint, usdtMint, signature)
	result = append(result, tokenTransfers...)

	return result, nil
}

// detectTokenTransfers parses pre/postTokenBalances for incoming SPL token transfers.
func (p *SolanaRPCProvider) detectTokenTransfers(tx transactionResult, address, usdcMint, usdtMint, signature string) []RawTransaction {
	var result []RawTransaction

	// Build a map of pre-token balances by (accountIndex, mint) for delta calculation
	type balanceKey struct {
		accountIndex int
		mint         string
	}
	preBalances := make(map[balanceKey]string)
	for _, tb := range tx.Meta.PreTokenBalances {
		preBalances[balanceKey{tb.AccountIndex, tb.Mint}] = tb.UITokenAmount.Amount
	}

	// Check postTokenBalances for transfers to our address
	for _, tb := range tx.Meta.PostTokenBalances {
		// Must be owned by our address
		if tb.Owner != address {
			continue
		}

		// Must be a tracked token
		token, decimals := identifySPLToken(tb.Mint, usdcMint, usdtMint)
		if token == "" {
			continue
		}

		// Calculate delta
		postAmount := new(big.Int)
		postAmount.SetString(tb.UITokenAmount.Amount, 10)

		preAmount := new(big.Int)
		if pre, ok := preBalances[balanceKey{tb.AccountIndex, tb.Mint}]; ok {
			preAmount.SetString(pre, 10)
		}

		delta := new(big.Int).Sub(postAmount, preAmount)
		if delta.Sign() <= 0 {
			continue // no incoming transfer
		}

		amountRaw := delta.String()
		amountHuman := tokenAmountToHuman(amountRaw, decimals)

		slog.Debug("SOL SPL token transfer detected",
			"signature", signature,
			"address", address,
			"token", token,
			"mint", tb.Mint,
			"amount", amountHuman,
		)

		result = append(result, RawTransaction{
			TxHash:      signature + ":" + token,
			Token:       token,
			AmountRaw:   amountRaw,
			AmountHuman: amountHuman,
			Decimals:    decimals,
		})
	}

	return result
}

// getSignatureStatus checks the confirmation status of a signature.
func (p *SolanaRPCProvider) getSignatureStatus(ctx context.Context, signature string) (*signatureStatus, error) {
	params := map[string]interface{}{
		"searchTransactionHistory": true,
	}

	body, err := p.rpcCall(ctx, "getSignatureStatuses", []interface{}{
		[]string{signature},
		params,
	})
	if err != nil {
		return nil, err
	}

	var result signatureStatusesResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse getSignatureStatuses: %w", err)
	}

	if len(result.Value) == 0 || result.Value[0] == nil {
		return nil, nil // signature not found
	}

	return result.Value[0], nil
}

// rpcCall sends a JSON-RPC request and returns the result field.
func (p *SolanaRPCProvider) rpcCall(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	reqBody := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal rpc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.rpcURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rpc request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read rpc response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited (HTTP 429) from solana rpc")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rpc status %d: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parse rpc response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// tokenMints returns the USDC and USDT mint addresses for the current network.
func (p *SolanaRPCProvider) tokenMints() (usdc, usdt string) {
	if p.network == "testnet" {
		return hdconfig.SOLDevnetUSDCMint, hdconfig.SOLDevnetUSDTMint
	}
	return hdconfig.SOLUSDCMint, hdconfig.SOLUSDTMint
}

// identifySPLToken determines the token name and decimals from a mint address.
func identifySPLToken(mint, usdcMint, usdtMint string) (string, int) {
	switch {
	case mint == usdcMint && usdcMint != "":
		return "USDC", hdconfig.SOLUSDCDecimals
	case mint == usdtMint && usdtMint != "":
		return "USDT", hdconfig.SOLUSDTDecimals
	default:
		return "", 0
	}
}

// extractBaseSignature removes the ":TOKEN" suffix from a composite tx_hash.
func extractBaseSignature(txHash string) string {
	if idx := strings.LastIndex(txHash, ":"); idx > 0 {
		suffix := txHash[idx+1:]
		if suffix == "SOL" || suffix == "USDC" || suffix == "USDT" {
			return txHash[:idx]
		}
	}
	return txHash
}

// lamportsToHuman converts lamports to a human-readable SOL string.
func lamportsToHuman(lamports int64) string {
	lamportsBig := new(big.Float).SetInt64(lamports)
	divisor := new(big.Float).SetInt64(int64(hdconfig.SOLLamportsPerSOL))
	result := new(big.Float).Quo(lamportsBig, divisor)
	return result.Text('f', hdconfig.SOLDecimals)
}

// tokenAmountToHuman converts a raw token amount string to human-readable.
func tokenAmountToHuman(rawAmount string, decimals int) string {
	return weiToHuman(rawAmount, decimals) // reuse BSC's converter — same math
}
