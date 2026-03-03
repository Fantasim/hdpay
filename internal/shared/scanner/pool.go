package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/models"
	"github.com/Fantasim/hdpay/internal/wallet/db"
)

// Pool manages a set of providers for a single chain with parallel fan-out,
// circuit breaker integration, and automatic failover retry.
type Pool struct {
	providers []Provider
	breakers  []*CircuitBreaker // one per provider, same index
	current   atomic.Int32
	chain     models.Chain
	database  *db.DB // optional — for persisting provider health
}

// NewPool creates a provider pool for parallel fan-out with per-provider circuit breakers.
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

// SetDB sets the database reference for persisting provider health.
func (p *Pool) SetDB(database *db.DB) {
	p.database = database

	for _, provider := range p.providers {
		row := db.ProviderHealthRow{
			ProviderName: provider.Name(),
			Chain:        string(p.chain),
			ProviderType: config.ProviderTypeScan,
			Status:       config.ProviderStatusHealthy,
			CircuitState: config.CircuitClosed,
		}
		if err := database.UpsertProviderHealth(row); err != nil {
			slog.Error("failed to upsert initial provider health",
				"provider", provider.Name(),
				"chain", p.chain,
				"error", err,
			)
		}
	}

	slog.Info("provider pool DB wired",
		"chain", p.chain,
		"providerCount", len(p.providers),
	)
}

// recordHealthSuccess records a provider success in the DB (non-blocking).
func (p *Pool) recordHealthSuccess(providerName string) {
	if p.database == nil {
		return
	}
	if err := p.database.RecordProviderSuccess(providerName); err != nil {
		slog.Error("failed to record provider success in DB",
			"provider", providerName,
			"error", err,
		)
	}
}

// recordHealthFailure records a provider failure in the DB (non-blocking).
func (p *Pool) recordHealthFailure(providerName string, cbState string, errMsg string) {
	if p.database == nil {
		return
	}
	if err := p.database.RecordProviderFailure(providerName, errMsg); err != nil {
		slog.Error("failed to record provider failure in DB",
			"provider", providerName,
			"error", err,
		)
	}
	if err := p.database.UpdateProviderCircuitState(providerName, cbState); err != nil {
		slog.Error("failed to update provider circuit state in DB",
			"provider", providerName,
			"state", cbState,
			"error", err,
		)
	}
}

// nextIndex returns the next provider index in round-robin order.
func (p *Pool) nextIndex() int {
	idx := p.current.Add(1)
	return int(idx-1) % len(p.providers)
}

// MaxBatchSize returns the maximum batch size across all providers.
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

// ── Parallel Fan-Out ─────────────────────────────────────────────────────────

// providerAssignment pairs a healthy provider with addresses assigned to it.
type providerAssignment struct {
	provider Provider
	breaker  *CircuitBreaker
	poolIdx  int // index in pool.providers
	addrs    []models.Address
}

// healthyAssignments returns all providers whose circuit breakers allow requests.
func (p *Pool) healthyAssignments() []providerAssignment {
	var out []providerAssignment
	for i, prov := range p.providers {
		if p.breakers[i].Allow() {
			out = append(out, providerAssignment{
				provider: prov,
				breaker:  p.breakers[i],
				poolIdx:  i,
			})
		}
	}
	return out
}

// distributeAddresses splits addresses across healthy providers proportional
// to their MaxBatchSize. A provider with MaxBatchSize=200 gets 10x more
// addresses than one with MaxBatchSize=20.
func distributeAddresses(addrs []models.Address, healthy []providerAssignment) {
	n := len(addrs)
	if n == 0 || len(healthy) == 0 {
		return
	}

	totalWeight := 0
	for _, h := range healthy {
		w := h.provider.MaxBatchSize()
		if w < 1 {
			w = 1
		}
		totalWeight += w
	}

	offset := 0
	for i := range healthy {
		weight := healthy[i].provider.MaxBatchSize()
		if weight < 1 {
			weight = 1
		}

		share := (n * weight) / totalWeight
		if share < 1 && offset < n {
			share = 1
		}
		if offset+share > n {
			share = n - offset
		}

		healthy[i].addrs = addrs[offset : offset+share]
		offset += share
	}

	// Assign rounding remainder to last provider.
	if offset < n && len(healthy) > 0 {
		last := len(healthy) - 1
		healthy[last].addrs = append(healthy[last].addrs, addrs[offset:n]...)
	}
}

// fetchNativeSubBatched calls provider.FetchNativeBalances in MaxBatchSize chunks.
// Returns successful results so far + slice of addresses that weren't fetched.
func fetchNativeSubBatched(ctx context.Context, pa providerAssignment, addrs []models.Address) (results []BalanceResult, failed []models.Address, err error) {
	batchSize := pa.provider.MaxBatchSize()
	if batchSize < 1 {
		batchSize = 1
	}

	for start := 0; start < len(addrs); start += batchSize {
		// Check for cancellation between sub-batches.
		if ctx.Err() != nil {
			return results, addrs[start:], ctx.Err()
		}

		end := start + batchSize
		if end > len(addrs) {
			end = len(addrs)
		}

		batchResults, batchErr := pa.provider.FetchNativeBalances(ctx, addrs[start:end])
		if batchErr != nil {
			return results, addrs[start:], batchErr
		}
		results = append(results, batchResults...)
	}
	return results, nil, nil
}

// fetchTokenSubBatched calls provider.FetchTokenBalances in MaxBatchSize chunks.
func fetchTokenSubBatched(ctx context.Context, pa providerAssignment, addrs []models.Address, token models.Token, contract string) (results []BalanceResult, failed []models.Address, err error) {
	batchSize := pa.provider.MaxBatchSize()
	if batchSize < 1 {
		batchSize = 1
	}

	for start := 0; start < len(addrs); start += batchSize {
		// Check for cancellation between sub-batches.
		if ctx.Err() != nil {
			return results, addrs[start:], ctx.Err()
		}

		end := start + batchSize
		if end > len(addrs) {
			end = len(addrs)
		}

		batchResults, batchErr := pa.provider.FetchTokenBalances(ctx, addrs[start:end], token, contract)
		if batchErr != nil {
			return results, addrs[start:], batchErr
		}
		results = append(results, batchResults...)
	}
	return results, nil, nil
}

// recordSuccess records success in circuit breaker, health DB, and usage metrics.
func (p *Pool) recordSuccess(pa providerAssignment) {
	pa.breaker.RecordSuccess()
	p.recordHealthSuccess(pa.provider.Name())
	if mr, ok := pa.provider.(MetricsReporter); ok {
		mr.RecordSuccess()
	}
}

// recordFailure records failure in circuit breaker, health DB, and usage metrics.
func (p *Pool) recordFailure(pa providerAssignment, err error) {
	pa.breaker.RecordFailure()
	p.recordHealthFailure(pa.provider.Name(), pa.breaker.State(), err.Error())
	if mr, ok := pa.provider.(MetricsReporter); ok {
		mr.RecordFailure(errors.Is(err, config.ErrProviderRateLimit))
	}
}

// FetchNativeBalances distributes addresses across all healthy providers in parallel,
// sub-batching by MaxBatchSize, with automatic retry for failed partitions.
func (p *Pool) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	healthy := p.healthyAssignments()
	if len(healthy) == 0 {
		slog.Error("all providers circuit-breaker open",
			"chain", p.chain,
			"providerCount", len(p.providers),
		)
		return nil, fmt.Errorf("all %s providers circuit-breaker open: %w", p.chain, config.ErrAllProvidersFailed)
	}

	slog.Debug("pool native fetch starting",
		"chain", p.chain,
		"addresses", len(addresses),
		"healthyProviders", len(healthy),
	)

	// Fast path: single healthy provider — no goroutine overhead.
	if len(healthy) == 1 {
		results, failed, err := fetchNativeSubBatched(ctx, healthy[0], addresses)
		if err != nil {
			p.recordFailure(healthy[0], err)
			if len(results) == 0 {
				return nil, fmt.Errorf("%s: %w", healthy[0].provider.Name(), err)
			}
			// Annotate failed addresses.
			for _, addr := range failed {
				results = append(results, BalanceResult{
					Address:      addr.Address,
					AddressIndex: addr.AddressIndex,
					Balance:      "0",
					Error:        err.Error(),
					Source:       healthy[0].provider.Name(),
				})
			}
			return results, nil
		}
		p.recordSuccess(healthy[0])
		return results, nil
	}

	// Distribute addresses proportionally across healthy providers.
	distributeAddresses(addresses, healthy)

	// Fan out: one goroutine per provider.
	type partResult struct {
		idx     int
		results []BalanceResult
		failed  []models.Address
		err     error
	}

	var wg sync.WaitGroup
	ch := make(chan partResult, len(healthy))

	for i, hw := range healthy {
		if len(hw.addrs) == 0 {
			continue
		}
		wg.Add(1)
		go func(i int, hw providerAssignment) {
			defer wg.Done()
			results, failed, err := fetchNativeSubBatched(ctx, hw, hw.addrs)
			ch <- partResult{idx: i, results: results, failed: failed, err: err}
		}(i, hw)
	}

	// Close channel when all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect results.
	var allResults []BalanceResult
	var failedAddrs []models.Address
	var failedProviders int

	for pr := range ch {
		hw := healthy[pr.idx]
		allResults = append(allResults, pr.results...)

		if pr.err != nil {
			if ctx.Err() != nil {
				return allResults, fmt.Errorf("context cancelled: %w", ctx.Err())
			}
			p.recordFailure(hw, pr.err)
			failedAddrs = append(failedAddrs, pr.failed...)
			failedProviders++

			slog.Warn("provider partition failed",
				"chain", p.chain,
				"provider", hw.provider.Name(),
				"assigned", len(hw.addrs),
				"succeeded", len(pr.results),
				"failed", len(pr.failed),
				"error", pr.err,
			)
		} else {
			p.recordSuccess(hw)
		}
	}

	// Retry failed addresses on remaining healthy providers (sequential failover).
	if len(failedAddrs) > 0 {
		slog.Info("retrying failed addresses",
			"chain", p.chain,
			"failedCount", len(failedAddrs),
		)

		retryHealthy := p.healthyAssignments()
		retried := false
		for _, hw := range retryHealthy {
			retryResults, retryFailed, err := fetchNativeSubBatched(ctx, hw, failedAddrs)
			allResults = append(allResults, retryResults...)
			if err == nil {
				p.recordSuccess(hw)
				retried = true
				break
			}
			p.recordFailure(hw, err)
			failedAddrs = retryFailed

			slog.Warn("retry provider also failed",
				"chain", p.chain,
				"provider", hw.provider.Name(),
				"error", err,
			)
		}

		// Annotate any remaining unresolved addresses as zero-balance.
		if !retried && len(failedAddrs) > 0 {
			for _, addr := range failedAddrs {
				allResults = append(allResults, BalanceResult{
					Address:      addr.Address,
					AddressIndex: addr.AddressIndex,
					Balance:      "0",
					Error:        "all providers failed",
					Source:       string(p.chain),
				})
			}
		}
	}

	// Total failure: no results at all, or every result has an error.
	allFailed := len(allResults) == 0
	if !allFailed {
		allFailed = true
		for _, r := range allResults {
			if r.Error == "" {
				allFailed = false
				break
			}
		}
	}
	if allFailed && len(addresses) > 0 {
		return allResults, fmt.Errorf("all %s native providers failed: %w", p.chain, config.ErrAllProvidersFailed)
	}

	slog.Debug("pool native fetch complete",
		"chain", p.chain,
		"totalResults", len(allResults),
		"failedProviders", failedProviders,
	)

	return allResults, nil
}

// FetchTokenBalances distributes addresses across all healthy providers in parallel,
// sub-batching by MaxBatchSize, with automatic retry for failed partitions.
func (p *Pool) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractOrMint string) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	healthy := p.healthyAssignments()
	if len(healthy) == 0 {
		return nil, fmt.Errorf("all %s providers circuit-breaker open: %w", p.chain, config.ErrAllProvidersFailed)
	}

	slog.Debug("pool token fetch starting",
		"chain", p.chain,
		"token", token,
		"addresses", len(addresses),
		"healthyProviders", len(healthy),
	)

	// Fast path: single healthy provider.
	if len(healthy) == 1 {
		results, failed, err := fetchTokenSubBatched(ctx, healthy[0], addresses, token, contractOrMint)
		if err != nil {
			// Token not supported — propagate directly.
			if errors.Is(err, config.ErrTokensNotSupported) {
				return nil, config.ErrTokensNotSupported
			}
			p.recordFailure(healthy[0], err)
			if len(results) == 0 {
				return nil, fmt.Errorf("%s: %w", healthy[0].provider.Name(), err)
			}
			for _, addr := range failed {
				results = append(results, BalanceResult{
					Address:      addr.Address,
					AddressIndex: addr.AddressIndex,
					Balance:      "0",
					Error:        err.Error(),
					Source:       healthy[0].provider.Name(),
				})
			}
			return results, nil
		}
		p.recordSuccess(healthy[0])
		return results, nil
	}

	// Distribute addresses proportionally.
	distributeAddresses(addresses, healthy)

	// Fan out.
	type partResult struct {
		idx     int
		results []BalanceResult
		failed  []models.Address
		err     error
	}

	var wg sync.WaitGroup
	ch := make(chan partResult, len(healthy))

	for i, hw := range healthy {
		if len(hw.addrs) == 0 {
			continue
		}
		wg.Add(1)
		go func(i int, hw providerAssignment) {
			defer wg.Done()
			results, failed, err := fetchTokenSubBatched(ctx, hw, hw.addrs, token, contractOrMint)
			ch <- partResult{idx: i, results: results, failed: failed, err: err}
		}(i, hw)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect results.
	var allResults []BalanceResult
	var failedAddrs []models.Address
	var failedProviders int
	allTokensUnsupported := true

	for pr := range ch {
		hw := healthy[pr.idx]
		allResults = append(allResults, pr.results...)

		if pr.err != nil {
			if errors.Is(pr.err, config.ErrTokensNotSupported) {
				// Not a failure — provider doesn't support tokens.
				failedAddrs = append(failedAddrs, hw.addrs...)
				continue
			}

			if ctx.Err() != nil {
				return allResults, fmt.Errorf("context cancelled: %w", ctx.Err())
			}

			p.recordFailure(hw, pr.err)
			failedAddrs = append(failedAddrs, pr.failed...)
			failedProviders++
			allTokensUnsupported = false

			slog.Warn("token provider partition failed",
				"chain", p.chain,
				"provider", hw.provider.Name(),
				"token", token,
				"error", pr.err,
			)
		} else {
			allTokensUnsupported = false
			p.recordSuccess(hw)
		}
	}

	// All providers returned ErrTokensNotSupported.
	if allTokensUnsupported && len(allResults) == 0 {
		return nil, config.ErrTokensNotSupported
	}

	// Retry failed addresses.
	if len(failedAddrs) > 0 {
		slog.Info("retrying failed token addresses",
			"chain", p.chain,
			"token", token,
			"failedCount", len(failedAddrs),
		)

		retryHealthy := p.healthyAssignments()
		retried := false
		for _, hw := range retryHealthy {
			retryResults, retryFailed, err := fetchTokenSubBatched(ctx, hw, failedAddrs, token, contractOrMint)
			allResults = append(allResults, retryResults...)
			if err == nil {
				p.recordSuccess(hw)
				retried = true
				break
			}
			if errors.Is(err, config.ErrTokensNotSupported) {
				continue
			}
			p.recordFailure(hw, err)
			failedAddrs = retryFailed
		}

		if !retried && len(failedAddrs) > 0 {
			for _, addr := range failedAddrs {
				allResults = append(allResults, BalanceResult{
					Address:      addr.Address,
					AddressIndex: addr.AddressIndex,
					Balance:      "0",
					Error:        "all token providers failed",
					Source:       string(p.chain),
				})
			}
		}
	}

	// Total failure: every result has an error.
	tokenAllFailed := len(allResults) == 0
	if !tokenAllFailed {
		tokenAllFailed = true
		for _, r := range allResults {
			if r.Error == "" {
				tokenAllFailed = false
				break
			}
		}
	}
	if tokenAllFailed && len(addresses) > 0 {
		return allResults, fmt.Errorf("all %s token providers failed for %s: %w", p.chain, token, config.ErrAllProvidersFailed)
	}

	slog.Debug("pool token fetch complete",
		"chain", p.chain,
		"token", token,
		"totalResults", len(allResults),
		"failedProviders", failedProviders,
	)

	return allResults, nil
}
