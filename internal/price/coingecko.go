package price

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
)

// coinGeckoIDToSymbol maps CoinGecko coin IDs to our internal token symbols.
var coinGeckoIDToSymbol = map[string]string{
	"bitcoin":     "BTC",
	"binancecoin": "BNB",
	"solana":      "SOL",
	"usd-coin":    "USDC",
	"tether":      "USDT",
}

// PriceService fetches and caches cryptocurrency prices from CoinGecko.
type PriceService struct {
	client   *http.Client
	baseURL  string
	cache    map[string]float64
	cachedAt time.Time
	mu       sync.RWMutex
}

// NewPriceService creates a new PriceService with default configuration.
func NewPriceService() *PriceService {
	slog.Info("price service initialized",
		"baseURL", config.CoinGeckoBaseURL,
		"cacheDuration", config.PriceCacheDuration,
	)

	return &PriceService{
		client: &http.Client{
			Timeout: config.APITimeout,
		},
		baseURL: config.CoinGeckoBaseURL,
		cache:   make(map[string]float64),
	}
}

// NewPriceServiceWithURL creates a PriceService with a custom base URL (for testing).
func NewPriceServiceWithURL(baseURL string) *PriceService {
	return &PriceService{
		client: &http.Client{
			Timeout: config.APITimeout,
		},
		baseURL: baseURL,
		cache:   make(map[string]float64),
	}
}

// GetPrices returns current USD prices for all supported coins.
// Returns cached prices if the cache is still valid, otherwise fetches fresh.
// Prices are keyed by symbol (BTC, BNB, SOL, USDC, USDT).
func (ps *PriceService) GetPrices(ctx context.Context) (map[string]float64, error) {
	ps.mu.RLock()
	if len(ps.cache) > 0 && time.Since(ps.cachedAt) < config.PriceCacheDuration {
		prices := make(map[string]float64, len(ps.cache))
		for k, v := range ps.cache {
			prices[k] = v
		}
		ps.mu.RUnlock()

		slog.Debug("price cache hit",
			"age", time.Since(ps.cachedAt).Round(time.Second),
			"coins", len(prices),
		)

		return prices, nil
	}
	ps.mu.RUnlock()

	prices, err := ps.fetchPrices(ctx)
	if err != nil {
		return nil, err
	}

	ps.mu.Lock()
	ps.cache = prices
	ps.cachedAt = time.Now()
	ps.mu.Unlock()

	return prices, nil
}

// coinGeckoResponse represents the CoinGecko /simple/price response.
// Each key is a coin ID mapping to currency values.
type coinGeckoResponse map[string]map[string]float64

// fetchPrices fetches fresh prices from the CoinGecko API.
func (ps *PriceService) fetchPrices(ctx context.Context) (map[string]float64, error) {
	url := fmt.Sprintf("%s/simple/price?ids=%s&vs_currencies=usd", ps.baseURL, config.CoinGeckoIDs)

	slog.Info("fetching prices from CoinGecko",
		"url", url,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create price request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	start := time.Now()
	resp, err := ps.client.Do(req)
	if err != nil {
		slog.Error("CoinGecko request failed",
			"error", err,
			"elapsed", time.Since(start).Round(time.Millisecond),
		)
		return nil, fmt.Errorf("%w: %v", config.ErrPriceFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("CoinGecko non-200 response",
			"status", resp.StatusCode,
			"elapsed", time.Since(start).Round(time.Millisecond),
		)
		return nil, fmt.Errorf("%w: HTTP %d", config.ErrPriceFetchFailed, resp.StatusCode)
	}

	var cgResp coinGeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&cgResp); err != nil {
		slog.Error("CoinGecko response decode failed",
			"error", err,
			"elapsed", time.Since(start).Round(time.Millisecond),
		)
		return nil, fmt.Errorf("%w: decode error: %v", config.ErrPriceFetchFailed, err)
	}

	// Map CoinGecko IDs to our symbols.
	prices := make(map[string]float64, len(coinGeckoIDToSymbol))
	for cgID, symbol := range coinGeckoIDToSymbol {
		if coinData, ok := cgResp[cgID]; ok {
			if usd, ok := coinData["usd"]; ok {
				prices[symbol] = usd
			}
		}
	}

	slog.Info("prices fetched",
		"coins", len(prices),
		"elapsed", time.Since(start).Round(time.Millisecond),
		"BTC", prices["BTC"],
		"BNB", prices["BNB"],
		"SOL", prices["SOL"],
	)

	return prices, nil
}
