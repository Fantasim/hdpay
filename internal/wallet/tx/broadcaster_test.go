package tx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBTCBroadcaster_Broadcast(t *testing.T) {
	expectedTxID := "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("expected Content-Type text/plain, got %s", r.Header.Get("Content-Type"))
		}
		if r.URL.Path != "/tx" {
			t.Errorf("expected path /tx, got %s", r.URL.Path)
		}

		// Verify body is the raw hex.
		body, _ := io.ReadAll(r.Body)
		if string(body) != "deadbeef" {
			t.Errorf("expected body deadbeef, got %s", string(body))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedTxID))
	}))
	defer server.Close()

	broadcaster := NewBTCBroadcaster(server.Client(), []string{server.URL})

	txHash, err := broadcaster.Broadcast(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("Broadcast() error = %v", err)
	}

	if txHash != expectedTxID {
		t.Errorf("txHash = %s, want %s", txHash, expectedTxID)
	}
}

func TestBTCBroadcaster_FallbackOnServerError(t *testing.T) {
	expectedTxID := "fallback_txid_hash"

	callCount := 0
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedTxID))
	}))
	defer server2.Close()

	broadcaster := NewBTCBroadcaster(http.DefaultClient, []string{server1.URL, server2.URL})

	txHash, err := broadcaster.Broadcast(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("Broadcast() error = %v", err)
	}

	if txHash != expectedTxID {
		t.Errorf("txHash = %s, want %s", txHash, expectedTxID)
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls (first fail, second succeed), got %d", callCount)
	}
}

func TestBTCBroadcaster_BadTxNoRetry(t *testing.T) {
	callCount := 0

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("sendrawtransaction RPC error"))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("should_not_reach"))
	}))
	defer server2.Close()

	broadcaster := NewBTCBroadcaster(http.DefaultClient, []string{server1.URL, server2.URL})

	_, err := broadcaster.Broadcast(context.Background(), "invalid_hex")
	if err == nil {
		t.Fatal("expected error for bad transaction")
	}

	// Should NOT have tried the second provider.
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry on 400), got %d", callCount)
	}
}

func TestBTCBroadcaster_AllProvidersFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("down"))
	}))
	defer server.Close()

	broadcaster := NewBTCBroadcaster(server.Client(), []string{server.URL})

	_, err := broadcaster.Broadcast(context.Background(), "deadbeef")
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}
