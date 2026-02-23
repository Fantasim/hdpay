package points

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/shared/price"
)

// Pricer wraps HDPay's PriceService with Poller-specific behavior:
//   - Stablecoins (USDC, USDT) short-circuit to $1.00
//   - Local cache (PriceCacheDuration TTL) to avoid redundant CoinGecko calls
//   - Retries on failure (configurable count + delay)
type Pricer struct {
	ps        *price.PriceService
	mu        sync.RWMutex
	cache     map[string]float64
	cacheTime time.Time
}

// NewPricer creates a Pricer backed by HDPay's CoinGecko PriceService.
func NewPricer(ps *price.PriceService) *Pricer {
	slog.Info("poller pricer initialized")
	return &Pricer{
		ps:    ps,
		cache: make(map[string]float64),
	}
}

// GetTokenPrice returns the USD price for a token symbol.
// Stablecoins return StablecoinPrice immediately without an API call.
// Native tokens are fetched from CoinGecko with retry logic.
func (p *Pricer) GetTokenPrice(ctx context.Context, token string) (float64, error) {
	// Short-circuit stablecoins.
	if token == "USDC" || token == "USDT" {
		slog.Debug("stablecoin price short-circuit",
			"token", token,
			"price", config.StablecoinPrice,
		)
		return config.StablecoinPrice, nil
	}

	// Map token to the key used in PriceService response.
	priceKey, err := tokenToPriceKey(token)
	if err != nil {
		return 0, err
	}

	// Check local cache first.
	p.mu.RLock()
	if time.Since(p.cacheTime) < config.PriceCacheDuration {
		if usd, ok := p.cache[priceKey]; ok {
			p.mu.RUnlock()
			slog.Debug("price cache hit",
				"token", token,
				"price", usd,
			)
			return usd, nil
		}
	}
	p.mu.RUnlock()

	// Retry loop.
	var lastErr error
	for attempt := 1; attempt <= config.PriceRetryCount; attempt++ {
		slog.Debug("fetching token price",
			"token", token,
			"priceKey", priceKey,
			"attempt", attempt,
		)

		prices, fetchErr := p.ps.GetPrices(ctx)
		if fetchErr == nil {
			// Update local cache with all fetched prices.
			p.mu.Lock()
			p.cache = prices
			p.cacheTime = time.Now()
			p.mu.Unlock()

			if usd, ok := prices[priceKey]; ok {
				slog.Debug("token price fetched",
					"token", token,
					"price", usd,
					"attempt", attempt,
				)
				return usd, nil
			}
			lastErr = fmt.Errorf("price key %q not found in response", priceKey)
		} else {
			lastErr = fetchErr
		}

		slog.Warn("price fetch attempt failed",
			"token", token,
			"attempt", attempt,
			"maxAttempts", config.PriceRetryCount,
			"error", lastErr,
		)

		if attempt < config.PriceRetryCount {
			select {
			case <-ctx.Done():
				return 0, fmt.Errorf("price fetch cancelled: %w", ctx.Err())
			case <-time.After(config.PriceRetryDelay):
			}
		}
	}

	slog.Error("all price fetch attempts failed",
		"token", token,
		"attempts", config.PriceRetryCount,
		"error", lastErr,
	)

	return 0, fmt.Errorf("failed to fetch price for %s after %d attempts: %w", token, config.PriceRetryCount, lastErr)
}

// tokenToPriceKey maps a Poller token string to the key used in HDPay's
// PriceService response map.
func tokenToPriceKey(token string) (string, error) {
	switch token {
	case "BTC":
		return "BTC", nil
	case "BNB":
		return "BNB", nil
	case "SOL":
		return "SOL", nil
	default:
		return "", fmt.Errorf("unknown native token %q for price lookup", token)
	}
}
