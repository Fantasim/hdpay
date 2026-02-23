package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/models"
)

// nodeRealMultiBalanceResponse is the BSCTrace (Etherscan-compatible) balancemulti response.
// Same wire format as the old BscScan API. On error, result is a string, not an array,
// so we use json.RawMessage and decode conditionally.
type nodeRealMultiBalanceResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

// nodeRealBalanceItem is a single item in the BSCTrace balancemulti result array.
type nodeRealBalanceItem struct {
	Account string `json:"account"`
	Balance string `json:"balance"`
}

// nodeRealTokenBalanceResponse is the BSCTrace tokenbalance response.
type nodeRealTokenBalanceResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

// NodeRealBSCTraceProvider fetches BSC balances from NodeReal's BSCTrace API.
// BSCTrace is the official replacement for the deprecated BscScan API (shut down Dec 18, 2025).
// It uses the same Etherscan-compatible wire format, enabling 20-address batch native balance
// queries that are not possible via plain JSON-RPC.
//
// Requires a free API key from nodereal.io (10M CU/month, 150 CUPS).
// URL format: https://open-platform.nodereal.io/{api_key}/bsctrace/api
type NodeRealBSCTraceProvider struct {
	client *http.Client
	rl     *RateLimiter
	apiURL string
}

// NewNodeRealBSCTraceProvider creates a NodeReal BSCTrace provider.
// apiKey must be non-empty; callers should only create this provider when the key is set.
func NewNodeRealBSCTraceProvider(client *http.Client, rl *RateLimiter, apiKey string) *NodeRealBSCTraceProvider {
	apiURL := fmt.Sprintf("%s/%s/bsctrace/api", config.NodeRealBSCTraceBaseURL, apiKey)

	slog.Info("nodereal bsctrace provider created",
		"monthlyLimit", config.KnownMonthlyLimitNodeReal,
	)

	return &NodeRealBSCTraceProvider{
		client: client,
		rl:     rl,
		apiURL: apiURL,
	}
}

func (p *NodeRealBSCTraceProvider) Name() string              { return "NodeRealBSCTrace" }
func (p *NodeRealBSCTraceProvider) Chain() models.Chain        { return models.ChainBSC }
func (p *NodeRealBSCTraceProvider) MaxBatchSize() int          { return config.ScanBatchSizeBscScan }
func (p *NodeRealBSCTraceProvider) RecordSuccess()             { p.rl.RecordSuccess() }
func (p *NodeRealBSCTraceProvider) RecordFailure(is429 bool)   { p.rl.RecordFailure(is429) }
func (p *NodeRealBSCTraceProvider) Stats() MetricsSnapshot     { return p.rl.Stats() }

// FetchNativeBalances fetches BNB balances using BSCTrace balancemulti (up to 20 addresses).
func (p *NodeRealBSCTraceProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	if err := p.rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}

	addrList := make([]string, len(addresses))
	for i, a := range addresses {
		addrList[i] = a.Address
	}
	addrParam := strings.Join(addrList, ",")

	url := fmt.Sprintf("%s?module=account&action=balancemulti&address=%s&tag=latest", p.apiURL, addrParam)

	slog.Debug("nodereal bsctrace balancemulti request",
		"addressCount", len(addresses),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header)
		slog.Warn("nodereal bsctrace rate limited", "retryAfter", retryAfter)
		return nil, config.NewTransientErrorWithRetry(config.ErrProviderRateLimit, retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("nodereal bsctrace non-200 response", "status", resp.StatusCode)
		return nil, config.NewTransientError(fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode))
	}

	var data nodeRealMultiBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if data.Status != "1" {
		var resultMsg string
		_ = json.Unmarshal(data.Result, &resultMsg)
		slog.Warn("nodereal bsctrace error response",
			"status", data.Status,
			"message", data.Message,
			"result", resultMsg,
		)
		if strings.Contains(data.Message, "rate limit") || strings.Contains(strings.ToLower(data.Message), "max rate") {
			return nil, config.NewTransientError(config.ErrProviderRateLimit)
		}
		return nil, config.NewTransientError(fmt.Errorf("%w: %s", config.ErrProviderUnavailable, data.Message))
	}

	var balanceItems []nodeRealBalanceItem
	if err := json.Unmarshal(data.Result, &balanceItems); err != nil {
		slog.Error("nodereal bsctrace failed to decode result array",
			"error", err,
			"rawResult", string(data.Result),
		)
		return nil, fmt.Errorf("decode result array: %w", err)
	}

	indexMap := make(map[string]int, len(addresses))
	for _, a := range addresses {
		indexMap[strings.ToLower(a.Address)] = a.AddressIndex
	}

	returned := make(map[string]bool, len(balanceItems))
	results := make([]BalanceResult, 0, len(addresses))
	for _, item := range balanceItems {
		lowerAddr := strings.ToLower(item.Account)
		addrIndex, ok := indexMap[lowerAddr]
		if !ok {
			slog.Warn("nodereal bsctrace returned unknown address", "address", item.Account)
			continue
		}

		returned[lowerAddr] = true
		results = append(results, BalanceResult{
			Address:      item.Account,
			AddressIndex: addrIndex,
			Balance:      item.Balance,
			Source:       p.Name(),
		})

		slog.Debug("nodereal bsctrace native balance fetched",
			"address", item.Account,
			"index", addrIndex,
			"balance", item.Balance,
		)
	}

	for _, a := range addresses {
		if !returned[strings.ToLower(a.Address)] {
			slog.Warn("nodereal bsctrace did not return balance for address",
				"address", a.Address,
				"index", a.AddressIndex,
			)
			results = append(results, BalanceResult{
				Address:      a.Address,
				AddressIndex: a.AddressIndex,
				Balance:      "0",
				Error:        "address not returned by provider",
				Source:       p.Name(),
			})
		}
	}

	return results, nil
}

// FetchTokenBalances fetches BEP-20 token balances one address at a time via BSCTrace tokenbalance.
func (p *NodeRealBSCTraceProvider) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractAddress string) ([]BalanceResult, error) {
	if contractAddress == "" {
		return nil, fmt.Errorf("contract address required for BSC token balance")
	}

	results := make([]BalanceResult, 0, len(addresses))
	var failCount int

	for _, addr := range addresses {
		if err := p.rl.Wait(ctx); err != nil {
			return results, fmt.Errorf("rate limiter wait: %w", err)
		}

		balance, err := p.fetchTokenBalance(ctx, addr.Address, contractAddress)
		if err != nil {
			if ctx.Err() != nil {
				return results, fmt.Errorf("context cancelled during token fetch: %w", err)
			}

			slog.Warn("nodereal bsctrace token balance fetch failed",
				"provider", p.Name(),
				"address", addr.Address,
				"index", addr.AddressIndex,
				"token", token,
				"error", err,
			)
			failCount++
			results = append(results, BalanceResult{
				Address:      addr.Address,
				AddressIndex: addr.AddressIndex,
				Balance:      "0",
				Error:        err.Error(),
				Source:       p.Name(),
			})
			continue
		}

		results = append(results, BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Balance:      balance,
			Source:       p.Name(),
		})

		slog.Debug("nodereal bsctrace token balance fetched",
			"address", addr.Address,
			"index", addr.AddressIndex,
			"token", token,
			"balance", balance,
		)
	}

	if failCount > 0 && failCount == len(addresses) {
		return results, fmt.Errorf("all %d token balance fetches failed: %w", failCount, config.ErrProviderUnavailable)
	}

	return results, nil
}

// fetchTokenBalance queries a single address for a specific BEP-20 token balance.
func (p *NodeRealBSCTraceProvider) fetchTokenBalance(ctx context.Context, address, contractAddress string) (string, error) {
	url := fmt.Sprintf("%s?module=account&action=tokenbalance&contractaddress=%s&address=%s&tag=latest",
		p.apiURL, contractAddress, address)

	slog.Debug("nodereal bsctrace tokenbalance request",
		"address", address,
		"contract", contractAddress,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "0", fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "0", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header)
		slog.Warn("nodereal bsctrace token rate limited", "address", address, "retryAfter", retryAfter)
		return "0", config.NewTransientErrorWithRetry(config.ErrProviderRateLimit, retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		return "0", config.NewTransientError(fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode))
	}

	var data nodeRealTokenBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "0", fmt.Errorf("decode response: %w", err)
	}

	if data.Status != "1" {
		if strings.Contains(data.Message, "rate limit") || strings.Contains(strings.ToLower(data.Message), "max rate") {
			return "0", config.NewTransientError(config.ErrProviderRateLimit)
		}
		return "0", config.NewTransientError(fmt.Errorf("%w: %s", config.ErrProviderUnavailable, data.Message))
	}

	return data.Result, nil
}
