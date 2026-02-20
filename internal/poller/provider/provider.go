package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	hdconfig "github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/scanner"
)

// Provider defines the interface for blockchain transaction detection.
// This is DIFFERENT from HDPay's scanner.Provider — Poller detects
// individual transactions instead of scanning balances.
type Provider interface {
	// Name returns the provider identifier (e.g. "blockstream", "bscscan").
	Name() string
	// Chain returns the blockchain this provider serves (e.g. "BTC", "BSC", "SOL").
	Chain() string
	// FetchTransactions returns incoming transactions for an address since cutoffUnix.
	FetchTransactions(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error)
	// CheckConfirmation checks whether a transaction is confirmed.
	CheckConfirmation(ctx context.Context, txHash string, blockNumber int64) (confirmed bool, confirmations int, err error)
	// GetCurrentBlock returns the latest block number (used for confirmation counting).
	GetCurrentBlock(ctx context.Context) (uint64, error)
}

// RawTransaction represents a detected incoming transaction from a blockchain provider.
type RawTransaction struct {
	TxHash        string
	Token         string // "BTC", "BNB", "SOL", "USDC", "USDT"
	AmountRaw     string // Raw amount in smallest unit (satoshis, wei, lamports)
	AmountHuman   string // Human-readable amount (e.g. "0.001")
	Decimals      int
	BlockTime     int64
	Confirmed     bool
	Confirmations int
	BlockNumber   int64
}

// wrappedProvider pairs a Provider with its rate limiter and circuit breaker.
type wrappedProvider struct {
	provider       Provider
	rateLimiter    *scanner.RateLimiter
	circuitBreaker *scanner.CircuitBreaker
}

// ProviderSet manages a set of providers for one chain with round-robin rotation.
// Thread-safe. On failure, rotates to the next provider and retries.
type ProviderSet struct {
	mu        sync.Mutex
	providers []wrappedProvider
	current   int
	chain     string
}

// NewProviderSet creates a ProviderSet for a specific chain.
// Each provider gets its own rate limiter and circuit breaker from HDPay's scanner package.
func NewProviderSet(chain string, providers []Provider, rpsPerProvider []int) *ProviderSet {
	wrapped := make([]wrappedProvider, len(providers))
	for i, p := range providers {
		rps := rpsPerProvider[i]
		wrapped[i] = wrappedProvider{
			provider:       p,
			rateLimiter:    scanner.NewRateLimiter(p.Name(), rps),
			circuitBreaker: scanner.NewCircuitBreaker(hdconfig.CircuitBreakerThreshold, hdconfig.CircuitBreakerCooldown),
		}
	}

	slog.Info("provider set created",
		"chain", chain,
		"providers", len(providers),
	)

	return &ProviderSet{
		providers: wrapped,
		chain:     chain,
	}
}

// Errors returned by ProviderSet.
var (
	ErrAllProvidersFailed = errors.New("all providers failed")
	ErrNoProviders        = errors.New("no providers configured")
)

// ExecuteFetch tries to fetch transactions using round-robin rotation.
// On failure, rotates to the next provider and retries until all have been tried.
func (ps *ProviderSet) ExecuteFetch(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error) {
	fn := func(p Provider) ([]RawTransaction, error) {
		return p.FetchTransactions(ctx, address, cutoffUnix)
	}
	return ps.executeFetch(ctx, fn)
}

// ExecuteConfirmation checks transaction confirmation using round-robin rotation.
func (ps *ProviderSet) ExecuteConfirmation(ctx context.Context, txHash string, blockNumber int64) (confirmed bool, confirmations int, err error) {
	var resConfirmed bool
	var resConfirmations int

	fn := func(p Provider) ([]RawTransaction, error) {
		c, n, e := p.CheckConfirmation(ctx, txHash, blockNumber)
		if e != nil {
			return nil, e
		}
		resConfirmed = c
		resConfirmations = n
		return nil, nil // success signaled by nil error
	}

	_, execErr := ps.executeFetch(ctx, fn)
	if execErr != nil {
		return false, 0, execErr
	}
	return resConfirmed, resConfirmations, nil
}

// ExecuteGetBlock gets the current block number using round-robin rotation.
func (ps *ProviderSet) ExecuteGetBlock(ctx context.Context) (uint64, error) {
	var block uint64

	fn := func(p Provider) ([]RawTransaction, error) {
		b, e := p.GetCurrentBlock(ctx)
		if e != nil {
			return nil, e
		}
		block = b
		return nil, nil
	}

	_, err := ps.executeFetch(ctx, fn)
	if err != nil {
		return 0, err
	}
	return block, nil
}

// executeFetch is the round-robin execution engine.
// Tries each provider once, rotating on failure.
func (ps *ProviderSet) executeFetch(ctx context.Context, fn func(Provider) ([]RawTransaction, error)) ([]RawTransaction, error) {
	ps.mu.Lock()
	n := len(ps.providers)
	if n == 0 {
		ps.mu.Unlock()
		return nil, ErrNoProviders
	}
	startIdx := ps.current
	ps.mu.Unlock()

	var lastErr error
	for attempt := 0; attempt < n; attempt++ {
		ps.mu.Lock()
		idx := (startIdx + attempt) % n
		wp := ps.providers[idx]
		ps.mu.Unlock()

		// Check circuit breaker
		if !wp.circuitBreaker.Allow() {
			slog.Debug("provider circuit open, skipping",
				"chain", ps.chain,
				"provider", wp.provider.Name(),
			)
			lastErr = fmt.Errorf("provider %s circuit open", wp.provider.Name())
			continue
		}

		// Wait for rate limiter
		if err := wp.rateLimiter.Wait(ctx); err != nil {
			slog.Warn("rate limiter wait cancelled",
				"chain", ps.chain,
				"provider", wp.provider.Name(),
				"error", err,
			)
			return nil, fmt.Errorf("rate limiter cancelled for %s: %w", wp.provider.Name(), err)
		}

		// Execute the function
		result, err := fn(wp.provider)
		if err != nil {
			wp.circuitBreaker.RecordFailure()
			slog.Warn("provider call failed, rotating",
				"chain", ps.chain,
				"provider", wp.provider.Name(),
				"error", err,
				"attempt", attempt+1,
				"totalProviders", n,
			)
			lastErr = err

			// Advance current pointer to next provider
			ps.mu.Lock()
			ps.current = (idx + 1) % n
			ps.mu.Unlock()
			continue
		}

		wp.circuitBreaker.RecordSuccess()

		slog.Debug("provider call succeeded",
			"chain", ps.chain,
			"provider", wp.provider.Name(),
		)
		return result, nil
	}

	slog.Error("all providers failed",
		"chain", ps.chain,
		"lastError", lastErr,
	)
	return nil, fmt.Errorf("%w: chain=%s: %v", ErrAllProvidersFailed, ps.chain, lastErr)
}

// Chain returns the chain this provider set serves.
func (ps *ProviderSet) Chain() string {
	return ps.chain
}

// ProviderCount returns the number of providers in this set.
func (ps *ProviderSet) ProviderCount() int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return len(ps.providers)
}

// NewHTTPClient creates a configured HTTP client for provider use.
// Uses HDPay's connection pool constants.
func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxConnsPerHost:     hdconfig.HTTPMaxConnsPerHost,
		MaxIdleConnsPerHost: hdconfig.HTTPMaxIdleConnsPerHost,
		MaxIdleConns:        hdconfig.HTTPMaxIdleConns,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   hdconfig.ProviderRequestTimeout,
	}
}
