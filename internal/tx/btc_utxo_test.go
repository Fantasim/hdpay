package tx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/scanner"
)

func TestBTCUTXOFetcher_FetchUTXOs(t *testing.T) {
	utxoResponse := []esploraUTXO{
		{
			TxID: "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111",
			Vout: 0,
			Status: struct {
				Confirmed   bool  `json:"confirmed"`
				BlockHeight int64 `json:"block_height"`
			}{Confirmed: true, BlockHeight: 700000},
			Value: 50000,
		},
		{
			TxID: "bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222",
			Vout: 1,
			Status: struct {
				Confirmed   bool  `json:"confirmed"`
				BlockHeight int64 `json:"block_height"`
			}{Confirmed: true, BlockHeight: 700001},
			Value: 30000,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(utxoResponse)
	}))
	defer server.Close()

	rl := scanner.NewRateLimiter("test", 100)
	fetcher := NewBTCUTXOFetcher(server.Client(), []string{server.URL}, []*scanner.RateLimiter{rl})

	utxos, err := fetcher.FetchUTXOs(context.Background(), "bc1qtest", 0)
	if err != nil {
		t.Fatalf("FetchUTXOs() error = %v", err)
	}

	if len(utxos) != 2 {
		t.Fatalf("expected 2 UTXOs, got %d", len(utxos))
	}

	if utxos[0].Value != 50000 {
		t.Errorf("UTXO[0].Value = %d, want 50000", utxos[0].Value)
	}
	if utxos[1].Value != 30000 {
		t.Errorf("UTXO[1].Value = %d, want 30000", utxos[1].Value)
	}
	if utxos[0].Address != "bc1qtest" {
		t.Errorf("UTXO[0].Address = %s, want bc1qtest", utxos[0].Address)
	}
	if utxos[0].AddressIndex != 0 {
		t.Errorf("UTXO[0].AddressIndex = %d, want 0", utxos[0].AddressIndex)
	}
}

func TestBTCUTXOFetcher_FiltersUnconfirmed(t *testing.T) {
	utxoResponse := []esploraUTXO{
		{
			TxID: "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111",
			Vout: 0,
			Status: struct {
				Confirmed   bool  `json:"confirmed"`
				BlockHeight int64 `json:"block_height"`
			}{Confirmed: true, BlockHeight: 700000},
			Value: 50000,
		},
		{
			TxID: "cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333",
			Vout: 0,
			Status: struct {
				Confirmed   bool  `json:"confirmed"`
				BlockHeight int64 `json:"block_height"`
			}{Confirmed: false},
			Value: 10000,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(utxoResponse)
	}))
	defer server.Close()

	rl := scanner.NewRateLimiter("test", 100)
	fetcher := NewBTCUTXOFetcher(server.Client(), []string{server.URL}, []*scanner.RateLimiter{rl})

	utxos, err := fetcher.FetchUTXOs(context.Background(), "bc1qtest", 0)
	if err != nil {
		t.Fatalf("FetchUTXOs() error = %v", err)
	}

	if len(utxos) != 1 {
		t.Fatalf("expected 1 confirmed UTXO, got %d", len(utxos))
	}

	if utxos[0].Value != 50000 {
		t.Errorf("expected confirmed UTXO value 50000, got %d", utxos[0].Value)
	}
}

func TestBTCUTXOFetcher_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]esploraUTXO{})
	}))
	defer server.Close()

	rl := scanner.NewRateLimiter("test", 100)
	fetcher := NewBTCUTXOFetcher(server.Client(), []string{server.URL}, []*scanner.RateLimiter{rl})

	utxos, err := fetcher.FetchUTXOs(context.Background(), "bc1qtest", 0)
	if err != nil {
		t.Fatalf("FetchUTXOs() error = %v", err)
	}

	if len(utxos) != 0 {
		t.Fatalf("expected 0 UTXOs, got %d", len(utxos))
	}
}

func TestBTCUTXOFetcher_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	rl := scanner.NewRateLimiter("test", 100)
	fetcher := NewBTCUTXOFetcher(server.Client(), []string{server.URL}, []*scanner.RateLimiter{rl})

	_, err := fetcher.FetchUTXOs(context.Background(), "bc1qtest", 0)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestBTCUTXOFetcher_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	rl := scanner.NewRateLimiter("test", 100)
	fetcher := NewBTCUTXOFetcher(server.Client(), []string{server.URL}, []*scanner.RateLimiter{rl})

	_, err := fetcher.FetchUTXOs(context.Background(), "bc1qtest", 0)
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
}

func TestBTCUTXOFetcher_FetchAllUTXOs(t *testing.T) {
	callCount := 0
	utxoSets := [][]esploraUTXO{
		{
			{
				TxID: "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111",
				Vout: 0,
				Status: struct {
					Confirmed   bool  `json:"confirmed"`
					BlockHeight int64 `json:"block_height"`
				}{Confirmed: true, BlockHeight: 700000},
				Value: 50000,
			},
		},
		{}, // Second address has no UTXOs.
		{
			{
				TxID: "bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222",
				Vout: 0,
				Status: struct {
					Confirmed   bool  `json:"confirmed"`
					BlockHeight int64 `json:"block_height"`
				}{Confirmed: true, BlockHeight: 700002},
				Value: 20000,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := callCount
		if idx >= len(utxoSets) {
			idx = len(utxoSets) - 1
		}
		callCount++
		json.NewEncoder(w).Encode(utxoSets[idx])
	}))
	defer server.Close()

	rl := scanner.NewRateLimiter("test", 100)
	fetcher := NewBTCUTXOFetcher(server.Client(), []string{server.URL}, []*scanner.RateLimiter{rl})

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qaddr0"},
		{Chain: models.ChainBTC, AddressIndex: 1, Address: "bc1qaddr1"},
		{Chain: models.ChainBTC, AddressIndex: 2, Address: "bc1qaddr2"},
	}

	utxos, err := fetcher.FetchAllUTXOs(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchAllUTXOs() error = %v", err)
	}

	if len(utxos) != 2 {
		t.Fatalf("expected 2 UTXOs total, got %d", len(utxos))
	}

	if utxos[0].Value != 50000 || utxos[1].Value != 20000 {
		t.Errorf("unexpected UTXO values: %d, %d", utxos[0].Value, utxos[1].Value)
	}
}

func TestBTCUTXOFetcher_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{totally broken json!!!`))
	}))
	defer server.Close()

	rl := scanner.NewRateLimiter("test", 100)
	fetcher := NewBTCUTXOFetcher(server.Client(), []string{server.URL}, []*scanner.RateLimiter{rl})

	_, err := fetcher.FetchUTXOs(context.Background(), "bc1qtest", 0)
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

func TestBTCUTXOFetcher_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	rl := scanner.NewRateLimiter("test", 100)
	fetcher := NewBTCUTXOFetcher(http.DefaultClient, []string{"http://localhost:1"}, []*scanner.RateLimiter{rl})

	_, err := fetcher.FetchUTXOs(ctx, "bc1qtest", 0)
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
}

func TestBTCUTXOFetcher_RoundRobin(t *testing.T) {
	provider1Calls := 0
	provider2Calls := 0

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provider1Calls++
		json.NewEncoder(w).Encode([]esploraUTXO{})
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provider2Calls++
		json.NewEncoder(w).Encode([]esploraUTXO{})
	}))
	defer server2.Close()

	rl1 := scanner.NewRateLimiter("provider1", 100)
	rl2 := scanner.NewRateLimiter("provider2", 100)
	fetcher := NewBTCUTXOFetcher(
		http.DefaultClient,
		[]string{server1.URL, server2.URL},
		[]*scanner.RateLimiter{rl1, rl2},
	)

	// Fetch 4 times — should alternate between providers.
	for i := 0; i < 4; i++ {
		_, err := fetcher.FetchUTXOs(context.Background(), "bc1qtest", 0)
		if err != nil {
			t.Fatalf("FetchUTXOs() call %d error = %v", i, err)
		}
	}

	if provider1Calls != 2 || provider2Calls != 2 {
		t.Errorf("expected 2 calls each, got provider1=%d provider2=%d", provider1Calls, provider2Calls)
	}
}
