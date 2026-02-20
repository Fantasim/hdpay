package tx

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTxSSEHub_SubscribeReturnsChannel(t *testing.T) {
	hub := NewTxSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	ch := hub.Subscribe()
	if ch == nil {
		t.Fatal("expected non-nil channel from Subscribe()")
	}

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.Unsubscribe(ch)
}

func TestTxSSEHub_UnsubscribeRemovesClient(t *testing.T) {
	hub := NewTxSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	ch := hub.Subscribe()
	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client after subscribe, got %d", hub.ClientCount())
	}

	hub.Unsubscribe(ch)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after unsubscribe, got %d", hub.ClientCount())
	}

	// Verify channel is closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after unsubscribe")
		}
	default:
		t.Error("expected closed channel to be readable (should return zero value)")
	}
}

func TestTxSSEHub_UnsubscribeIdempotent(t *testing.T) {
	hub := NewTxSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	ch := hub.Subscribe()
	hub.Unsubscribe(ch)

	// Second unsubscribe should not panic.
	hub.Unsubscribe(ch)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestTxSSEHub_BroadcastSingleClient(t *testing.T) {
	hub := NewTxSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	event := TxEvent{
		Type: "tx_status",
		Data: TxStatusData{Chain: "BTC", TxHash: "abc123", Status: "confirmed"},
	}

	hub.Broadcast(event)

	select {
	case received := <-ch:
		if received.Type != "tx_status" {
			t.Errorf("expected event type tx_status, got %s", received.Type)
		}
		data, ok := received.Data.(TxStatusData)
		if !ok {
			t.Fatal("expected TxStatusData")
		}
		if data.TxHash != "abc123" {
			t.Errorf("expected txHash abc123, got %s", data.TxHash)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast event")
	}
}

func TestTxSSEHub_BroadcastMultipleClients(t *testing.T) {
	hub := NewTxSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	ch1 := hub.Subscribe()
	ch2 := hub.Subscribe()
	ch3 := hub.Subscribe()
	defer hub.Unsubscribe(ch1)
	defer hub.Unsubscribe(ch2)
	defer hub.Unsubscribe(ch3)

	if hub.ClientCount() != 3 {
		t.Fatalf("expected 3 clients, got %d", hub.ClientCount())
	}

	event := TxEvent{Type: "tx_complete", Data: TxCompleteData{Chain: "SOL", SuccessCount: 5}}
	hub.Broadcast(event)

	for i, ch := range []chan TxEvent{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.Type != "tx_complete" {
				t.Errorf("client %d: expected event type tx_complete, got %s", i, received.Type)
			}
		case <-time.After(time.Second):
			t.Errorf("client %d: timeout waiting for broadcast", i)
		}
	}
}

func TestTxSSEHub_SlowClientDrop(t *testing.T) {
	hub := NewTxSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	slowCh := hub.Subscribe()
	fastCh := hub.Subscribe()
	defer hub.Unsubscribe(slowCh)
	defer hub.Unsubscribe(fastCh)

	// Fill slow client's buffer completely.
	bufferSize := cap(slowCh)
	for i := 0; i < bufferSize; i++ {
		hub.Broadcast(TxEvent{Type: "tx_status", Data: TxStatusData{Chain: "BTC", Current: i}})
	}

	// Drain fast client.
	for i := 0; i < bufferSize; i++ {
		<-fastCh
	}

	// Broadcast one more event — should be dropped for slow client.
	hub.Broadcast(TxEvent{Type: "tx_complete", Data: TxCompleteData{Chain: "BTC"}})

	// Fast client should receive.
	select {
	case received := <-fastCh:
		if received.Type != "tx_complete" {
			t.Errorf("fast client: expected tx_complete, got %s", received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("fast client: timeout waiting for event")
	}

	// Slow client's channel should still have buffered events (the overflow one was dropped).
	// Drain the buffer and check we get exactly bufferSize events (not bufferSize+1).
	count := 0
	for {
		select {
		case <-slowCh:
			count++
		default:
			goto done
		}
	}
done:
	if count != bufferSize {
		t.Errorf("slow client: expected %d buffered events, got %d", bufferSize, count)
	}
}

func TestTxSSEHub_ConcurrentSubscribeBroadcast(t *testing.T) {
	hub := NewTxSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	var wg sync.WaitGroup

	// Concurrent subscribers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := hub.Subscribe()
			// Small delay to allow broadcasts to arrive.
			time.Sleep(10 * time.Millisecond)
			hub.Unsubscribe(ch)
		}()
	}

	// Concurrent broadcasters.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			hub.Broadcast(TxEvent{Type: "tx_status", Data: TxStatusData{Current: idx}})
		}(i)
	}

	// Should complete without panic or deadlock.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success — no panic, no deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: concurrent subscribe/broadcast deadlocked")
	}
}

func TestTxSSEHub_RunCancellationClosesClients(t *testing.T) {
	hub := NewTxSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	ch1 := hub.Subscribe()
	ch2 := hub.Subscribe()

	// Cancel the context to stop the hub.
	cancel()

	// Give the hub a moment to process cancellation.
	time.Sleep(50 * time.Millisecond)

	// Both channels should be closed.
	for i, ch := range []chan TxEvent{ch1, ch2} {
		select {
		case _, ok := <-ch:
			if ok {
				t.Errorf("client %d: expected channel to be closed", i)
			}
		case <-time.After(time.Second):
			t.Errorf("client %d: timeout waiting for channel close", i)
		}
	}
}
