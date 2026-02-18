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
		slog.Warn("bscscan rate limited")
		return nil, config.ErrProviderRateLimit
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("bscscan non-200 response", "status", resp.StatusCode)
		return nil, fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode)
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
			return nil, config.ErrProviderRateLimit
		}
		return nil, fmt.Errorf("%w: %s", config.ErrProviderUnavailable, data.Message)
	}

	// Build address→index lookup.
	indexMap := make(map[string]int, len(addresses))
	for _, a := range addresses {
		indexMap[strings.ToLower(a.Address)] = a.AddressIndex
	}

	results := make([]BalanceResult, 0, len(data.Result))
	for _, item := range data.Result {
		addrIndex, ok := indexMap[strings.ToLower(item.Account)]
		if !ok {
			slog.Warn("bscscan returned unknown address", "address", item.Account)
			continue
		}

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

	return results, nil
}

// FetchTokenBalances fetches BEP-20 token balances one address at a time via BscScan tokenbalance.
func (p *BscScanProvider) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractAddress string) ([]BalanceResult, error) {
	if contractAddress == "" {
		return nil, fmt.Errorf("contract address required for BSC token balance")
	}

	results := make([]BalanceResult, 0, len(addresses))

	for _, addr := range addresses {
		if err := p.rl.Wait(ctx); err != nil {
			return results, fmt.Errorf("rate limiter wait: %w", err)
		}

		balance, err := p.fetchTokenBalance(ctx, addr.Address, contractAddress)
		if err != nil {
			return results, fmt.Errorf("fetch %s balance for %s (index %d): %w", token, addr.Address, addr.AddressIndex, err)
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
		slog.Warn("bscscan token rate limited", "address", address)
		return "0", config.ErrProviderRateLimit
	}

	if resp.StatusCode != http.StatusOK {
		return "0", fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode)
	}

	var data bscScanTokenBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "0", fmt.Errorf("decode response: %w", err)
	}

	if data.Status != "1" {
		if strings.Contains(data.Message, "rate limit") || strings.Contains(strings.ToLower(data.Message), "max rate") {
			return "0", config.ErrProviderRateLimit
		}
		return "0", fmt.Errorf("%w: %s", config.ErrProviderUnavailable, data.Message)
	}

	return data.Result, nil
}
