package scanner

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_Name(t *testing.T) {
	rl := NewRateLimiter("blockstream", 10)
	if rl.Name() != "blockstream" {
		t.Errorf("Name() = %q, want %q", rl.Name(), "blockstream")
	}
}

func TestRateLimiter_WaitAllowsWithinLimit(t *testing.T) {
	rl := NewRateLimiter("test-provider", 100) // high RPS so it doesn't block

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Wait() error on iteration %d: %v", i, err)
		}
	}
}

func TestRateLimiter_WaitCancelledContext(t *testing.T) {
	// 1 request per second — after the first request, the second must wait.
	rl := NewRateLimiter("slow-provider", 1)

	ctx := context.Background()
	// Consume the initial token.
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first Wait() error: %v", err)
	}

	// Now cancel the context before the next token becomes available.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := rl.Wait(cancelCtx)
	if err == nil {
		t.Fatal("Wait() with cancelled context should return error")
	}
}

func TestRateLimiter_WaitContextTimeout(t *testing.T) {
	rl := NewRateLimiter("slow-provider", 1)

	ctx := context.Background()
	// Consume the initial token.
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first Wait() error: %v", err)
	}

	// Short timeout — won't be enough for the next token at 1 RPS.
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := rl.Wait(timeoutCtx)
	if err == nil {
		t.Fatal("Wait() with expired timeout should return error")
	}
}

func TestRateLimiter_ConcurrentWaiters(t *testing.T) {
	// 10 RPS — check that concurrent goroutines don't panic or deadlock.
	rl := NewRateLimiter("concurrent-provider", 10)

	const goroutines = 20
	var wg sync.WaitGroup
	errors := make(chan error, goroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.Wait(ctx); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent Wait() error: %v", err)
	}
}

func TestRateLimiter_RateCompliance(t *testing.T) {
	// 10 RPS with burst 1 — verify that 10 requests take at least ~900ms
	// (first request is instant, next 9 each wait ~100ms).
	rl := NewRateLimiter("rate-test", 10)

	ctx := context.Background()
	const requests = 10

	start := time.Now()
	for i := 0; i < requests; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Wait() error on iteration %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// 10 requests at 10 RPS (burst 1): first instant, then 9 waits of ~100ms = ~900ms.
	// Allow some margin for scheduling jitter.
	minExpected := 800 * time.Millisecond
	if elapsed < minExpected {
		t.Errorf("10 requests at 10 RPS completed in %v, expected at least %v", elapsed, minExpected)
	}
}
