package scanner

import (
	"context"
	"log/slog"

	"golang.org/x/time/rate"
)

// RateLimiter wraps a token bucket rate limiter for a specific provider.
type RateLimiter struct {
	limiter *rate.Limiter
	name    string
}

// NewRateLimiter creates a rate limiter allowing rps requests per second.
func NewRateLimiter(name string, rps int) *RateLimiter {
	slog.Debug("rate limiter created",
		"provider", name,
		"rps", rps,
	)
	return &RateLimiter{
		// Burst(1) ensures requests are spread evenly across the second,
		// preventing bursty traffic that can trigger provider rate limiting
		// even when the average rate is within limits.
		limiter: rate.NewLimiter(rate.Limit(rps), 1),
		name:    name,
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
