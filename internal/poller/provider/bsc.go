package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	hdconfig "github.com/Fantasim/hdpay/internal/config"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
)

// bscScanResponse is the top-level BscScan API response envelope.
type bscScanResponse struct {
	Status  string          `json:"status"` // "1" = success, "0" = error
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

// bscNormalTx represents a normal (BNB) transaction from BscScan txlist.
type bscNormalTx struct {
	Hash      string `json:"hash"`
	From      string `json:"from"`
	To        string `json:"to"`
	Value     string `json:"value"` // wei
	IsError   string `json:"isError"`
	TimeStamp string `json:"timeStamp"` // unix seconds
}

// bscTokenTx represents a token transfer from BscScan tokentx.
type bscTokenTx struct {
	Hash            string `json:"hash"`
	From            string `json:"from"`
	To              string `json:"to"`
	Value           string `json:"value"`
	ContractAddress string `json:"contractAddress"`
	TokenDecimal    string `json:"tokenDecimal"`
	TimeStamp       string `json:"timeStamp"`
}

// bscBlockNumberResult represents the eth_blockNumber proxy response.
type bscBlockNumberResult struct {
	Result string `json:"result"` // hex-encoded block number
}

// BscScanProvider detects BSC transactions via BscScan API.
type BscScanProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	network string
}

// NewBscScanProvider creates a BscScan provider for the given network.
func NewBscScanProvider(client *http.Client, network, apiKey string) *BscScanProvider {
	baseURL := hdconfig.BscScanAPIURL
	if network == "testnet" {
		baseURL = hdconfig.BscScanTestnetURL
	}

	slog.Info("bscscan provider created",
		"network", network,
		"baseURL", baseURL,
		"hasAPIKey", apiKey != "",
	)

	return &BscScanProvider{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		network: network,
	}
}

func (p *BscScanProvider) Name() string  { return "bscscan" }
func (p *BscScanProvider) Chain() string  { return "BSC" }

// FetchTransactions returns incoming BSC transactions (BNB + USDC + USDT) for an address since cutoffUnix.
func (p *BscScanProvider) FetchTransactions(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error) {
	var result []RawTransaction

	// 1. Normal transactions (BNB)
	normalTxs, err := p.fetchNormalTxs(ctx, address, cutoffUnix)
	if err != nil {
		return nil, fmt.Errorf("bscscan fetch normal txs: %w", err)
	}
	result = append(result, normalTxs...)

	// 2. Token transactions (USDC + USDT)
	tokenTxs, err := p.fetchTokenTxs(ctx, address, cutoffUnix)
	if err != nil {
		return nil, fmt.Errorf("bscscan fetch token txs: %w", err)
	}
	result = append(result, tokenTxs...)

	slog.Info("BSC transactions fetched",
		"provider", p.Name(),
		"address", address,
		"normalCount", len(normalTxs),
		"tokenCount", len(tokenTxs),
		"totalCount", len(result),
	)

	return result, nil
}

// fetchNormalTxs fetches normal (BNB) transactions from BscScan.
func (p *BscScanProvider) fetchNormalTxs(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error) {
	url := fmt.Sprintf("%s?module=account&action=txlist&address=%s&sort=desc&apikey=%s",
		p.baseURL, address, p.apiKey)

	slog.Debug("fetching BSC normal txs",
		"provider", p.Name(),
		"address", address,
	)

	body, err := p.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var resp bscScanResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bscscan parse response: %w", err)
	}

	// BscScan returns status "0" with message "No transactions found" when empty
	if resp.Status == "0" && resp.Message == "No transactions found" {
		return nil, nil
	}
	if resp.Status == "0" {
		return nil, fmt.Errorf("bscscan API error: %s", resp.Message)
	}

	var txs []bscNormalTx
	if err := json.Unmarshal(resp.Result, &txs); err != nil {
		return nil, fmt.Errorf("bscscan parse normal txs: %w", err)
	}

	var result []RawTransaction
	for _, tx := range txs {
		ts, err := strconv.ParseInt(tx.TimeStamp, 10, 64)
		if err != nil {
			slog.Warn("bscscan invalid timestamp", "txHash", tx.Hash, "timestamp", tx.TimeStamp)
			continue
		}

		// Skip transactions older than cutoff
		if ts < cutoffUnix {
			break // sorted desc, so we can stop
		}

		// Skip outgoing transactions
		if !strings.EqualFold(tx.To, address) {
			continue
		}

		// Skip failed transactions
		if tx.IsError == "1" {
			slog.Debug("skipping failed BSC tx", "txHash", tx.Hash)
			continue
		}

		// Skip zero-value transactions
		if tx.Value == "0" {
			continue
		}

		amountHuman := weiToHuman(tx.Value, hdconfig.BNBDecimals)

		slog.Debug("BSC BNB incoming tx detected",
			"txHash", tx.Hash,
			"address", address,
			"wei", tx.Value,
			"amountBNB", amountHuman,
		)

		result = append(result, RawTransaction{
			TxHash:      tx.Hash,
			Token:       "BNB",
			AmountRaw:   tx.Value,
			AmountHuman: amountHuman,
			Decimals:    hdconfig.BNBDecimals,
			BlockTime:   ts,
			Confirmed:   true, // txlist only returns mined txs
			BlockNumber: 0,    // will be populated below if needed
		})
	}

	return result, nil
}

// fetchTokenTxs fetches token (USDC + USDT) transactions from BscScan.
func (p *BscScanProvider) fetchTokenTxs(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error) {
	url := fmt.Sprintf("%s?module=account&action=tokentx&address=%s&sort=desc&apikey=%s",
		p.baseURL, address, p.apiKey)

	slog.Debug("fetching BSC token txs",
		"provider", p.Name(),
		"address", address,
	)

	body, err := p.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var resp bscScanResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bscscan parse response: %w", err)
	}

	if resp.Status == "0" && resp.Message == "No transactions found" {
		return nil, nil
	}
	if resp.Status == "0" {
		return nil, fmt.Errorf("bscscan API error: %s", resp.Message)
	}

	var txs []bscTokenTx
	if err := json.Unmarshal(resp.Result, &txs); err != nil {
		return nil, fmt.Errorf("bscscan parse token txs: %w", err)
	}

	// Resolve contract addresses based on network
	usdcContract, usdtContract := p.tokenContracts()

	var result []RawTransaction
	for _, tx := range txs {
		ts, err := strconv.ParseInt(tx.TimeStamp, 10, 64)
		if err != nil {
			slog.Warn("bscscan invalid token tx timestamp", "txHash", tx.Hash, "timestamp", tx.TimeStamp)
			continue
		}

		if ts < cutoffUnix {
			break
		}

		// Skip outgoing
		if !strings.EqualFold(tx.To, address) {
			continue
		}

		// Determine token from contract address
		token, decimals := p.identifyToken(tx.ContractAddress, usdcContract, usdtContract)
		if token == "" {
			continue // not a tracked token
		}

		amountHuman := weiToHuman(tx.Value, decimals)

		slog.Debug("BSC token incoming tx detected",
			"txHash", tx.Hash,
			"address", address,
			"token", token,
			"amount", amountHuman,
			"contract", tx.ContractAddress,
		)

		result = append(result, RawTransaction{
			TxHash:      tx.Hash,
			Token:       token,
			AmountRaw:   tx.Value,
			AmountHuman: amountHuman,
			Decimals:    decimals,
			BlockTime:   ts,
			Confirmed:   true,
			BlockNumber: 0,
		})
	}

	return result, nil
}

// CheckConfirmation checks BSC transaction confirmation by comparing block numbers.
func (p *BscScanProvider) CheckConfirmation(ctx context.Context, _ string, blockNumber int64) (bool, int, error) {
	currentBlock, err := p.GetCurrentBlock(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("bscscan get current block: %w", err)
	}

	if blockNumber <= 0 {
		return false, 0, nil
	}

	confirmations := int(currentBlock) - int(blockNumber)
	confirmed := confirmations >= pollerconfig.ConfirmationsBSC

	slog.Debug("BSC confirmation check",
		"blockNumber", blockNumber,
		"currentBlock", currentBlock,
		"confirmations", confirmations,
		"confirmed", confirmed,
		"threshold", pollerconfig.ConfirmationsBSC,
	)

	return confirmed, confirmations, nil
}

// GetCurrentBlock returns the latest BSC block number via BscScan proxy.
func (p *BscScanProvider) GetCurrentBlock(ctx context.Context) (uint64, error) {
	url := fmt.Sprintf("%s?module=proxy&action=eth_blockNumber&apikey=%s",
		p.baseURL, p.apiKey)

	slog.Debug("fetching BSC current block",
		"provider", p.Name(),
	)

	body, err := p.doGet(ctx, url)
	if err != nil {
		return 0, fmt.Errorf("bscscan get block number: %w", err)
	}

	var result bscBlockNumberResult
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("bscscan parse block number: %w", err)
	}

	// Parse hex block number (remove "0x" prefix)
	blockStr := strings.TrimPrefix(result.Result, "0x")
	block, err := strconv.ParseUint(blockStr, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("bscscan parse hex block %q: %w", result.Result, err)
	}

	slog.Debug("BSC current block",
		"block", block,
	)

	return block, nil
}

// tokenContracts returns the USDC and USDT contract addresses for the current network.
func (p *BscScanProvider) tokenContracts() (usdc, usdt string) {
	if p.network == "testnet" {
		return hdconfig.BSCTestnetUSDCContract, hdconfig.BSCTestnetUSDTContract
	}
	return hdconfig.BSCUSDCContract, hdconfig.BSCUSDTContract
}

// identifyToken determines the token name and decimals from a contract address.
// Returns empty string if the contract is not a tracked token.
func (p *BscScanProvider) identifyToken(contract, usdcContract, usdtContract string) (string, int) {
	switch {
	case strings.EqualFold(contract, usdcContract) && usdcContract != "":
		return "USDC", p.tokenDecimals("USDC")
	case strings.EqualFold(contract, usdtContract) && usdtContract != "":
		return "USDT", p.tokenDecimals("USDT")
	default:
		return "", 0
	}
}

// tokenDecimals returns the decimal count for a BSC token.
func (p *BscScanProvider) tokenDecimals(token string) int {
	if p.network == "testnet" {
		// BSC testnet USDC is 6 decimals, USDT is 18
		switch token {
		case "USDC":
			return 6 // BSCTestnetUSDCContract is 6 decimals
		case "USDT":
			return hdconfig.BSCUSDTDecimals
		}
	}
	switch token {
	case "USDC":
		return hdconfig.BSCUSDCDecimals
	case "USDT":
		return hdconfig.BSCUSDTDecimals
	default:
		return 18
	}
}

// doGet performs a GET request and returns the response body.
func (p *BscScanProvider) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited (HTTP 429) from bscscan")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from bscscan: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// weiToHuman converts a raw amount string to human-readable using the given decimals.
func weiToHuman(rawAmount string, decimals int) string {
	amount := new(big.Float)
	if _, ok := amount.SetString(rawAmount); !ok {
		return "0"
	}

	divisor := new(big.Float).SetInt(new(big.Int).Exp(
		big.NewInt(10),
		big.NewInt(int64(decimals)),
		nil,
	))

	result := new(big.Float).Quo(amount, divisor)
	return result.Text('f', decimals)
}
