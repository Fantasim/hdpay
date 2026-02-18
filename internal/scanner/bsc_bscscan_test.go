package scanner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

// TestBscScanProvider_FetchNativeBalances_MissingAddress tests that missing addresses
// from BscScan response are detected and annotated (B3 for BSC).
func TestBscScanProvider_FetchNativeBalances_MissingAddress(t *testing.T) {
	// BscScan returns only 2 of 3 requested addresses.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := bscScanMultiBalanceResponse{
			Status:  "1",
			Message: "OK",
			Result: []struct {
				Account string `json:"account"`
				Balance string `json:"balance"`
			}{
				{Account: "0xAddr1", Balance: "1000"},
				{Account: "0xAddr3", Balance: "3000"},
				// 0xAddr2 is missing from response
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
	}

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0xAddr1"},
		{Chain: models.ChainBSC, AddressIndex: 1, Address: "0xAddr2"},
		{Chain: models.ChainBSC, AddressIndex: 2, Address: "0xAddr3"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("expected no error for partial results, got %v", err)
	}

	// Should have 3 results — 2 successful + 1 missing with error annotation.
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Find the missing address result.
	var missingResult *BalanceResult
	for i := range results {
		if results[i].AddressIndex == 1 {
			missingResult = &results[i]
			break
		}
	}

	if missingResult == nil {
		t.Fatal("missing address result not found")
	}
	if missingResult.Error == "" {
		t.Error("expected error annotation for missing address")
	}
	if missingResult.Balance != "0" {
		t.Errorf("expected balance 0 for missing address, got %s", missingResult.Balance)
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

// TestBscScanProvider_FetchTokenBalances_ErrorCollection tests that token balance
// errors don't abort remaining addresses (B1 fix).
func TestBscScanProvider_FetchTokenBalances_ErrorCollection(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 2 {
			// Second call returns error.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := bscScanTokenBalanceResponse{
			Status:  "1",
			Message: "OK",
			Result:  "1000",
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
		{Chain: models.ChainBSC, AddressIndex: 1, Address: "0xAddr2"},
		{Chain: models.ChainBSC, AddressIndex: 2, Address: "0xAddr3"},
	}

	results, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, "0xContract")
	// Partial failure → no error (only all-fail returns error).
	if err != nil {
		t.Fatalf("expected no error for partial failure, got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Address 1 (index 1) should have error annotation.
	if results[1].Error == "" {
		t.Error("expected error annotation for failed address")
	}
	// Addresses 0 and 2 should succeed.
	if results[0].Error != "" {
		t.Errorf("expected no error for addr0, got %s", results[0].Error)
	}
	if results[2].Error != "" {
		t.Errorf("expected no error for addr2, got %s", results[2].Error)
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
