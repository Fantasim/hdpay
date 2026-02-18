package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// bscScanMultiBalanceResponse represents BscScan balancemulti response.
type bscScanMultiBalanceResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  []struct {
		Account string `json:"account"`
		Balance string `json:"balance"`
	} `json:"result"`
}

// bscScanTokenBalanceResponse represents BscScan tokenbalance response.
type bscScanTokenBalanceResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

// BscScanProvider fetches BSC balances from the BscScan REST API.
type BscScanProvider struct {
	client *http.Client
	rl     *RateLimiter
	apiURL string
	apiKey string
}

// NewBscScanProvider creates a provider for the BscScan API.
func NewBscScanProvider(client *http.Client, rl *RateLimiter, apiKey string, network string) *BscScanProvider {
	apiURL := config.BscScanAPIURL
	if network == string(models.NetworkTestnet) {
		apiURL = config.BscScanTestnetURL
	}

	slog.Info("bscscan provider created",
		"apiURL", apiURL,
		"hasAPIKey", apiKey != "",
		"network", network,
	)

	return &BscScanProvider{
		client: client,
		rl:     rl,
		apiURL: apiURL,
		apiKey: apiKey,
	}
}

func (p *BscScanProvider) Name() string              { return "BscScan" }
func (p *BscScanProvider) Chain() models.Chain        { return models.ChainBSC }
func (p *BscScanProvider) MaxBatchSize() int          { return config.ScanBatchSizeBscScan }

// FetchNativeBalances fetches BNB balances using BscScan balancemulti (up to 20 addresses).
func (p *BscScanProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	if err := p.rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}

	// Build comma-separated address list.
	addrList := make([]string, len(addresses))
	for i, a := range addresses {
		addrList[i] = a.Address
	}
	addrParam := strings.Join(addrList, ",")

	url := fmt.Sprintf("%s?module=account&action=balancemulti&address=%s&tag=latest", p.apiURL, addrParam)
	if p.apiKey != "" {
		url += "&apikey=" + p.apiKey
	}

	slog.Debug("bscscan balancemulti request",
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
		slog.Warn("bscscan rate limited", "retryAfter", retryAfter)
		return nil, config.NewTransientErrorWithRetry(config.ErrProviderRateLimit, retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("bscscan non-200 response", "status", resp.StatusCode)
		return nil, config.NewTransientError(fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode))
	}

	var data bscScanMultiBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if data.Status != "1" {
		slog.Warn("bscscan error response",
			"status", data.Status,
			"message", data.Message,
		)
		// BscScan returns status "0" for rate limits with specific messages.
		if strings.Contains(data.Message, "rate limit") || strings.Contains(strings.ToLower(data.Message), "max rate") {
			return nil, config.NewTransientError(config.ErrProviderRateLimit)
		}
		return nil, config.NewTransientError(fmt.Errorf("%w: %s", config.ErrProviderUnavailable, data.Message))
	}

	// Build address→index lookup and track which addresses were returned.
	indexMap := make(map[string]int, len(addresses))
	addrMap := make(map[string]models.Address, len(addresses))
	for _, a := range addresses {
		indexMap[strings.ToLower(a.Address)] = a.AddressIndex
		addrMap[strings.ToLower(a.Address)] = a
	}

	returned := make(map[string]bool, len(data.Result))
	results := make([]BalanceResult, 0, len(addresses))
	for _, item := range data.Result {
		lowerAddr := strings.ToLower(item.Account)
		addrIndex, ok := indexMap[lowerAddr]
		if !ok {
			slog.Warn("bscscan returned unknown address", "address", item.Account)
			continue
		}

		returned[lowerAddr] = true
		results = append(results, BalanceResult{
			Address:      item.Account,
			AddressIndex: addrIndex,
			Balance:      item.Balance,
			Source:       p.Name(),
		})

		slog.Debug("bscscan native balance fetched",
			"address", item.Account,
			"index", addrIndex,
			"balance", item.Balance,
		)
	}

	// Detect missing addresses — those requested but not returned by BscScan.
	for _, a := range addresses {
		if !returned[strings.ToLower(a.Address)] {
			slog.Warn("bscscan did not return balance for address",
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

// FetchTokenBalances fetches BEP-20 token balances one address at a time via BscScan tokenbalance.
// Continues on per-address errors instead of early-returning.
func (p *BscScanProvider) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractAddress string) ([]BalanceResult, error) {
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

			slog.Warn("bscscan token balance fetch failed",
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

		slog.Debug("bscscan token balance fetched",
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

// fetchTokenBalance queries a single address for a specific token balance.
func (p *BscScanProvider) fetchTokenBalance(ctx context.Context, address, contractAddress string) (string, error) {
	url := fmt.Sprintf("%s?module=account&action=tokenbalance&contractaddress=%s&address=%s&tag=latest",
		p.apiURL, contractAddress, address)
	if p.apiKey != "" {
		url += "&apikey=" + p.apiKey
	}

	slog.Debug("bscscan tokenbalance request",
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
		slog.Warn("bscscan token rate limited", "address", address, "retryAfter", retryAfter)
		return "0", config.NewTransientErrorWithRetry(config.ErrProviderRateLimit, retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		return "0", config.NewTransientError(fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode))
	}

	var data bscScanTokenBalanceResponse
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
