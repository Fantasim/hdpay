package scanner

import (
	"sync"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
)

func TestCircuitBreaker_ClosedAllowsRequests(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	for i := 0; i < 10; i++ {
		if !cb.Allow() {
			t.Fatalf("expected Allow() = true in closed state, iteration %d", i)
		}
	}

	if cb.State() != config.CircuitClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Record failures below threshold
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != config.CircuitClosed {
		t.Errorf("expected closed after 2 failures, got %s", cb.State())
	}

	// Third failure trips the circuit
	cb.RecordFailure()
	if cb.State() != config.CircuitOpen {
		t.Errorf("expected open after 3 failures, got %s", cb.State())
	}

	if cb.ConsecutiveFailures() != 3 {
		t.Errorf("expected 3 consecutive failures, got %d", cb.ConsecutiveFailures())
	}
}

func TestCircuitBreaker_OpenBlocksRequests(t *testing.T) {
	cb := NewCircuitBreaker(1, 1*time.Hour) // long cooldown to ensure it stays open

	cb.RecordFailure() // trips immediately

	if cb.State() != config.CircuitOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	if cb.Allow() {
		t.Error("expected Allow() = false when circuit is open")
	}
}

func TestCircuitBreaker_HalfOpenAfterCooldown(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	if cb.State() != config.CircuitOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	// Wait for cooldown
	time.Sleep(60 * time.Millisecond)

	// Next Allow() should transition to half-open
	if !cb.Allow() {
		t.Error("expected Allow() = true after cooldown (half-open)")
	}

	if cb.State() != config.CircuitHalfOpen {
		t.Errorf("expected half_open, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.Allow() // transitions to half-open
	cb.RecordSuccess()

	if cb.State() != config.CircuitClosed {
		t.Errorf("expected closed after half-open success, got %s", cb.State())
	}

	if cb.ConsecutiveFailures() != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.ConsecutiveFailures())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.Allow() // transitions to half-open
	cb.RecordFailure()

	if cb.State() != config.CircuitOpen {
		t.Errorf("expected open after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreaker_SuccessResetsClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Accumulate some failures
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.ConsecutiveFailures() != 2 {
		t.Fatalf("expected 2 failures, got %d", cb.ConsecutiveFailures())
	}

	// Success resets
	cb.RecordSuccess()

	if cb.ConsecutiveFailures() != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.ConsecutiveFailures())
	}
	if cb.State() != config.CircuitClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(100, 50*time.Millisecond)

	var wg sync.WaitGroup
	iterations := 1000

	// Concurrent Allow/RecordSuccess/RecordFailure
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			cb.Allow()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			cb.RecordSuccess()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			cb.RecordFailure()
		}
	}()

	wg.Wait()

	// Should not panic; state should be valid
	state := cb.State()
	validStates := map[string]bool{
		config.CircuitClosed:   true,
		config.CircuitOpen:     true,
		config.CircuitHalfOpen: true,
	}
	if !validStates[state] {
		t.Errorf("invalid state after concurrent access: %s", state)
	}
}
