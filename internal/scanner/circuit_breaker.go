package scanner

import (
	"log/slog"
	"sync"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
)

// CircuitBreaker implements the circuit breaker pattern to prevent
// cascading failures when a provider becomes unhealthy.
//
// State machine:
//   - Closed (normal): All requests pass. On failure, increment counter.
//     If counter >= threshold → Open.
//   - Open (tripped): All requests blocked (return ErrCircuitOpen).
//     After cooldown elapsed → Half-Open.
//   - Half-Open (testing): Allow 1 request through.
//     If success → Closed (reset counter). If failure → Open (restart cooldown).
type CircuitBreaker struct {
	mu              sync.Mutex
	state           string
	consecutiveFails int
	threshold       int
	cooldown        time.Duration
	lastFailure     time.Time
	halfOpenAllowed int
	halfOpenCount   int
}

// NewCircuitBreaker creates a new circuit breaker with the given threshold and cooldown.
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:           config.CircuitClosed,
		threshold:       threshold,
		cooldown:        cooldown,
		halfOpenAllowed: config.CircuitBreakerHalfOpenMax,
	}
}

// Allow returns true if a request should be allowed through the circuit breaker.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case config.CircuitClosed:
		return true

	case config.CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.cooldown {
			slog.Debug("circuit breaker transitioning to half-open",
				"consecutiveFails", cb.consecutiveFails,
				"cooldown", cb.cooldown,
			)
			cb.state = config.CircuitHalfOpen
			cb.halfOpenCount = 0
			return true
		}
		return false

	case config.CircuitHalfOpen:
		if cb.halfOpenCount < cb.halfOpenAllowed {
			cb.halfOpenCount++
			return true
		}
		return false

	default:
		return false
	}
}

// RecordSuccess records a successful call, resetting the circuit breaker to closed state.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	previousState := cb.state

	cb.consecutiveFails = 0
	cb.state = config.CircuitClosed
	cb.halfOpenCount = 0

	if previousState != config.CircuitClosed {
		slog.Info("circuit breaker closed after success",
			"previousState", previousState,
		)
	}
}

// RecordFailure records a failed call and may trip the circuit breaker to open state.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails++
	cb.lastFailure = time.Now()

	if cb.state == config.CircuitHalfOpen {
		slog.Warn("circuit breaker reopened from half-open after failure",
			"consecutiveFails", cb.consecutiveFails,
		)
		cb.state = config.CircuitOpen
		cb.halfOpenCount = 0
		return
	}

	if cb.consecutiveFails >= cb.threshold {
		slog.Warn("circuit breaker tripped to open",
			"consecutiveFails", cb.consecutiveFails,
			"threshold", cb.threshold,
		)
		cb.state = config.CircuitOpen
		cb.halfOpenCount = 0
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// ConsecutiveFailures returns the current failure count.
func (cb *CircuitBreaker) ConsecutiveFailures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.consecutiveFails
}
