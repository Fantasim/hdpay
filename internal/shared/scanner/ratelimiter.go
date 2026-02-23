package scanner

import (
	"context"
	"log/slog"

	"golang.org/x/time/rate"
)

// RateLimiter wraps a token bucket rate limiter for a specific provider
// and embeds ProviderMetrics to track call counts and period-based usage.
type RateLimiter struct {
	limiter *rate.Limiter
	name    string
	metrics *ProviderMetrics
}

// NewRateLimiter creates a rate limiter allowing rps requests per second.
// knownMonthlyLimit is the provider's documented monthly call cap (0 = no cap).
func NewRateLimiter(name string, rps int, knownMonthlyLimit int64) *RateLimiter {
	slog.Debug("rate limiter created",
		"provider", name,
		"rps", rps,
		"knownMonthlyLimit", knownMonthlyLimit,
	)
	return &RateLimiter{
		// Burst(1) ensures requests are spread evenly across the second,
		// preventing bursty traffic that can trigger provider rate limiting
		// even when the average rate is within limits.
		limiter: rate.NewLimiter(rate.Limit(rps), 1),
		name:    name,
		metrics: NewProviderMetrics(name, knownMonthlyLimit),
	}
}

// Wait blocks until the rate limiter allows another request or ctx is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	if err := rl.limiter.Wait(ctx); err != nil {
		slog.Warn("rate limiter wait cancelled",
			"provider", rl.name,
			"error", err,
		)
		return err
	}
	return nil
}

// Name returns the provider name this limiter is associated with.
func (rl *RateLimiter) Name() string {
	return rl.name
}

// RecordSuccess records a successful API call in the usage metrics.
func (rl *RateLimiter) RecordSuccess() {
	rl.metrics.RecordSuccess()
}

// RecordFailure records a failed API call. Pass is429=true for HTTP 429 responses.
func (rl *RateLimiter) RecordFailure(is429 bool) {
	rl.metrics.RecordFailure(is429)
}

// Stats returns a point-in-time snapshot of this provider's usage metrics.
func (rl *RateLimiter) Stats() MetricsSnapshot {
	return rl.metrics.Snapshot()
}
