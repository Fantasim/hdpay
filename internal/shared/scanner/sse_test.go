package scanner

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSSEHub_SubscribeAndBroadcast(t *testing.T) {
	hub := NewSSEHub()

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	event := Event{
		Type: "scan_progress",
		Data: ScanProgressData{Chain: "BTC", Scanned: 100, Total: 1000},
	}

	hub.Broadcast(event)

	select {
	case received := <-ch:
		if received.Type != "scan_progress" {
			t.Errorf("expected event type scan_progress, got %s", received.Type)
		}
		data, ok := received.Data.(ScanProgressData)
		if !ok {
			t.Fatal("expected ScanProgressData type")
		}
		if data.Chain != "BTC" {
			t.Errorf("expected chain BTC, got %s", data.Chain)
		}
		if data.Scanned != 100 {
			t.Errorf("expected scanned 100, got %d", data.Scanned)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestSSEHub_MultipleSubscribers(t *testing.T) {
	hub := NewSSEHub()

	ch1 := hub.Subscribe()
	ch2 := hub.Subscribe()
	ch3 := hub.Subscribe()
	defer hub.Unsubscribe(ch1)
	defer hub.Unsubscribe(ch2)
	defer hub.Unsubscribe(ch3)

	if hub.ClientCount() != 3 {
		t.Errorf("expected 3 clients, got %d", hub.ClientCount())
	}

	event := Event{Type: "scan_complete", Data: ScanCompleteData{Chain: "SOL"}}
	hub.Broadcast(event)

	for i, ch := range []chan Event{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.Type != "scan_complete" {
				t.Errorf("subscriber %d: expected scan_complete, got %s", i, received.Type)
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d: timed out", i)
		}
	}
}

func TestSSEHub_Unsubscribe(t *testing.T) {
	hub := NewSSEHub()

	ch := hub.Subscribe()
	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.Unsubscribe(ch)
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after unsubscribe, got %d", hub.ClientCount())
	}

	// Double unsubscribe should not panic.
	hub.Unsubscribe(ch)
}

func TestSSEHub_SlowClientNonBlocking(t *testing.T) {
	hub := NewSSEHub()

	// Subscribe but never read from channel.
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	// Fill the buffer completely.
	for range cap(ch) {
		hub.Broadcast(Event{Type: "filler"})
	}

	// This broadcast should not block — it should drop the event for the slow client.
	done := make(chan struct{})
	go func() {
		hub.Broadcast(Event{Type: "should_not_block"})
		close(done)
	}()

	select {
	case <-done:
		// Success: broadcast completed without blocking.
	case <-time.After(time.Second):
		t.Fatal("broadcast blocked on slow client")
	}
}

func TestSSEHub_RunShutdown(t *testing.T) {
	hub := NewSSEHub()

	ch := hub.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		hub.Run(ctx)
	}()

	// Cancel context to trigger shutdown.
	cancel()
	wg.Wait()

	// After shutdown, the client channel should be closed.
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after hub shutdown")
	}
}

func TestSSEHub_ConcurrentAccess(t *testing.T) {
	hub := NewSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	var wg sync.WaitGroup

	// Concurrently subscribe, broadcast, and unsubscribe.
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := hub.Subscribe()
			hub.Broadcast(Event{Type: "test"})
			hub.Unsubscribe(ch)
		}()
	}

	wg.Wait()
}
