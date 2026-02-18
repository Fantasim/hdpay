package scanner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/models"
)

func TestBscScanProvider_FetchNativeBalances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := bscScanMultiBalanceResponse{
			Status:  "1",
			Message: "OK",
			Result: []struct {
				Account string `json:"account"`
				Balance string `json:"balance"`
			}{
				{Account: "0xAddr1", Balance: "1000000000000000000"},
				{Account: "0xAddr2", Balance: "0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &BscScanProvider{
		client: server.Client(),
		rl:     rl,
		apiURL: server.URL,
		apiKey: "testkey",
	}

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0xAddr1"},
		{Chain: models.ChainBSC, AddressIndex: 1, Address: "0xAddr2"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Balance != "1000000000000000000" {
		t.Errorf("expected balance 1000000000000000000, got %s", results[0].Balance)
	}

	if results[1].Balance != "0" {
		t.Errorf("expected balance 0, got %s", results[1].Balance)
	}
}

func TestBscScanProvider_FetchNativeBalances_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := bscScanMultiBalanceResponse{
			Status:  "0",
			Message: "NOTOK - max rate limit reached",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &BscScanProvider{
		client: server.Client(),
		rl:     rl,
		apiURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0xAddr1"},
	}

	_, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for rate limit response")
	}
}

func TestBscScanProvider_FetchTokenBalances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := bscScanTokenBalanceResponse{
			Status:  "1",
			Message: "OK",
			Result:  "5000000000000000000",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &BscScanProvider{
		client: server.Client(),
		rl:     rl,
		apiURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0xAddr1"},
	}

	results, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, "0xContract")
	if err != nil {
		t.Fatalf("FetchTokenBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != "5000000000000000000" {
		t.Errorf("expected balance 5000000000000000000, got %s", results[0].Balance)
	}
}

func TestBscScanProvider_Metadata(t *testing.T) {
	provider := &BscScanProvider{}
	if provider.Name() != "BscScan" {
		t.Errorf("expected name BscScan, got %s", provider.Name())
	}
	if provider.Chain() != models.ChainBSC {
		t.Errorf("expected chain BSC, got %s", provider.Chain())
	}
	if provider.MaxBatchSize() != 20 {
		t.Errorf("expected batch size 20, got %d", provider.MaxBatchSize())
	}
}
