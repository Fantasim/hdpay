package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// Pool manages a set of providers for a single chain with round-robin rotation and failover.
type Pool struct {
	providers []Provider
	current   atomic.Int32
	chain     models.Chain
}

// NewPool creates a provider pool for round-robin load balancing.
func NewPool(chain models.Chain, providers ...Provider) *Pool {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}

	slog.Info("provider pool created",
		"chain", chain,
		"providers", names,
		"count", len(providers),
	)

	return &Pool{
		providers: providers,
		chain:     chain,
	}
}

// next returns the next provider in round-robin order.
func (p *Pool) next() Provider {
	idx := p.current.Add(1)
	return p.providers[int(idx-1)%len(p.providers)]
}

// MaxBatchSize returns the maximum batch size across all providers.
// The scanner should use this to determine batch sizes.
func (p *Pool) MaxBatchSize() int {
	maxSize := 1
	for _, provider := range p.providers {
		if s := provider.MaxBatchSize(); s > maxSize {
			maxSize = s
		}
	}
	return maxSize
}

// FetchNativeBalances tries providers in round-robin order with failover on rate limit/unavailable errors.
func (p *Pool) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	var lastErr error

	for range len(p.providers) {
		provider := p.next()

		results, err := provider.FetchNativeBalances(ctx, addresses)
		if err == nil {
			return results, nil
		}

		lastErr = err

		if errors.Is(err, config.ErrProviderRateLimit) || errors.Is(err, config.ErrProviderUnavailable) {
			slog.Warn("provider failed, trying next",
				"chain", p.chain,
				"provider", provider.Name(),
				"error", err,
			)
			continue
		}

		// Non-retriable error (e.g., context cancelled).
		return results, err
	}

	return nil, fmt.Errorf("all %s providers failed: %w", p.chain, lastErr)
}

// FetchTokenBalances tries providers in round-robin order with failover.
func (p *Pool) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractOrMint string) ([]BalanceResult, error) {
	var lastErr error

	for range len(p.providers) {
		provider := p.next()

		results, err := provider.FetchTokenBalances(ctx, addresses, token, contractOrMint)
		if err == nil {
			return results, nil
		}

		// Skip providers that don't support tokens.
		if errors.Is(err, config.ErrTokensNotSupported) {
			continue
		}

		lastErr = err

		if errors.Is(err, config.ErrProviderRateLimit) || errors.Is(err, config.ErrProviderUnavailable) {
			slog.Warn("token provider failed, trying next",
				"chain", p.chain,
				"provider", provider.Name(),
				"token", token,
				"error", err,
			)
			continue
		}

		return results, err
	}

	if lastErr == nil {
		return nil, config.ErrTokensNotSupported
	}

	return nil, fmt.Errorf("all %s token providers failed for %s: %w", p.chain, token, lastErr)
}
