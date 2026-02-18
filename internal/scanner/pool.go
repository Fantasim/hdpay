package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// Pool manages a set of providers for a single chain with round-robin rotation,
// circuit breaker integration, and failover.
type Pool struct {
	providers []Provider
	breakers  []*CircuitBreaker // one per provider, same index
	current   atomic.Int32
	chain     models.Chain
}

// NewPool creates a provider pool for round-robin load balancing with per-provider circuit breakers.
func NewPool(chain models.Chain, providers ...Provider) *Pool {
	names := make([]string, len(providers))
	breakers := make([]*CircuitBreaker, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
		breakers[i] = NewCircuitBreaker(
			config.CircuitBreakerThreshold,
			config.CircuitBreakerCooldown,
		)
	}

	slog.Info("provider pool created",
		"chain", chain,
		"providers", names,
		"count", len(providers),
	)

	return &Pool{
		providers: providers,
		breakers:  breakers,
		chain:     chain,
	}
}

// next returns the next provider index in round-robin order.
func (p *Pool) nextIndex() int {
	idx := p.current.Add(1)
	return int(idx-1) % len(p.providers)
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

// SuggestBackoff returns a backoff duration based on consecutive all-provider failures.
// Uses exponential backoff: base * 2^(failures-1), capped at max.
func (p *Pool) SuggestBackoff(consecutiveFailures int) time.Duration {
	if consecutiveFailures <= 0 {
		return 0
	}
	delay := config.ExponentialBackoffBase * time.Duration(1<<uint(consecutiveFailures-1))
	if delay > config.ExponentialBackoffMax {
		delay = config.ExponentialBackoffMax
	}
	return delay
}

// FetchNativeBalances tries providers in round-robin order with circuit breaker and failover.
func (p *Pool) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	var allErrors []error

	for range len(p.providers) {
		idx := p.nextIndex()
		provider := p.providers[idx]
		cb := p.breakers[idx]

		// Check circuit breaker.
		if !cb.Allow() {
			slog.Debug("circuit breaker open, skipping provider",
				"chain", p.chain,
				"provider", provider.Name(),
				"state", cb.State(),
			)
			allErrors = append(allErrors, fmt.Errorf("%s: %w", provider.Name(), config.ErrCircuitOpen))
			continue
		}

		results, err := provider.FetchNativeBalances(ctx, addresses)
		if err == nil {
			cb.RecordSuccess()
			return results, nil
		}

		// Record failure in circuit breaker.
		cb.RecordFailure()
		allErrors = append(allErrors, fmt.Errorf("%s: %w", provider.Name(), err))

		if errors.Is(err, config.ErrProviderRateLimit) ||
			errors.Is(err, config.ErrProviderUnavailable) ||
			config.IsTransient(err) {
			slog.Warn("provider failed, trying next",
				"chain", p.chain,
				"provider", provider.Name(),
				"circuitState", cb.State(),
				"consecutiveFailures", cb.ConsecutiveFailures(),
				"error", err,
			)
			continue
		}

		// Non-retriable error (context cancelled, etc.) — return immediately.
		return results, err
	}

	return nil, fmt.Errorf("all %s providers failed: %w", p.chain, errors.Join(allErrors...))
}

// FetchTokenBalances tries providers in round-robin order with circuit breaker and failover.
func (p *Pool) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractOrMint string) ([]BalanceResult, error) {
	var allErrors []error

	for range len(p.providers) {
		idx := p.nextIndex()
		provider := p.providers[idx]
		cb := p.breakers[idx]

		// Check circuit breaker.
		if !cb.Allow() {
			slog.Debug("circuit breaker open for token, skipping provider",
				"chain", p.chain,
				"provider", provider.Name(),
				"token", token,
				"state", cb.State(),
			)
			allErrors = append(allErrors, fmt.Errorf("%s: %w", provider.Name(), config.ErrCircuitOpen))
			continue
		}

		results, err := provider.FetchTokenBalances(ctx, addresses, token, contractOrMint)
		if err == nil {
			cb.RecordSuccess()
			return results, nil
		}

		// Skip providers that don't support tokens (not a failure).
		if errors.Is(err, config.ErrTokensNotSupported) {
			continue
		}

		// Record failure in circuit breaker.
		cb.RecordFailure()
		allErrors = append(allErrors, fmt.Errorf("%s: %w", provider.Name(), err))

		if errors.Is(err, config.ErrProviderRateLimit) ||
			errors.Is(err, config.ErrProviderUnavailable) ||
			config.IsTransient(err) {
			slog.Warn("token provider failed, trying next",
				"chain", p.chain,
				"provider", provider.Name(),
				"token", token,
				"circuitState", cb.State(),
				"consecutiveFailures", cb.ConsecutiveFailures(),
				"error", err,
			)
			continue
		}

		// Non-retriable error — return immediately.
		return results, err
	}

	if len(allErrors) == 0 {
		return nil, config.ErrTokensNotSupported
	}

	return nil, fmt.Errorf("all %s token providers failed for %s: %w", p.chain, token, errors.Join(allErrors...))
}
