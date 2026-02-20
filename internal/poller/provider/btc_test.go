package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- BTC test fixtures ---

func btcTxFixture(txid, address string, sats int64, confirmed bool, blockTime int64) blockstreamTx {
	return blockstreamTx{
		TxID: txid,
		Status: blockstreamStatus{
			Confirmed: confirmed,
			BlockTime: blockTime,
		},
		Vout: []blockstreamVout{
			{ScriptPubKeyAddr: address, Value: sats},
		},
	}
}

func TestBlockstreamProvider_FetchTransactions_Basic(t *testing.T) {
	address := "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"

	txs := []blockstreamTx{
		btcTxFixture("tx1", address, 168841, true, 1708300000),
		btcTxFixture("tx2", address, 50000, true, 1708200000),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(txs)
	}))
	defer server.Close()

	provider := &BlockstreamProvider{
		client:  server.Client(),
		baseURL: server.URL,
	}

	ctx := context.Background()
	result, err := provider.FetchTransactions(ctx, address, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 txs, got %d", len(result))
	}

	// Check first tx
	if result[0].TxHash != "tx1" {
		t.Errorf("TxHash = %q, want %q", result[0].TxHash, "tx1")
	}
	if result[0].Token != "BTC" {
		t.Errorf("Token = %q, want %q", result[0].Token, "BTC")
	}
	if result[0].AmountRaw != "168841" {
		t.Errorf("AmountRaw = %q, want %q", result[0].AmountRaw, "168841")
	}
	if result[0].AmountHuman != "0.00168841" {
		t.Errorf("AmountHuman = %q, want %q", result[0].AmountHuman, "0.00168841")
	}
	if result[0].Decimals != 8 {
		t.Errorf("Decimals = %d, want 8", result[0].Decimals)
	}
	if !result[0].Confirmed {
		t.Error("expected Confirmed=true")
	}
}

func TestBlockstreamProvider_FetchTransactions_CutoffFilter(t *testing.T) {
	address := "tb1qtest"
	cutoff := int64(1708250000)

	txs := []blockstreamTx{
		btcTxFixture("tx_new", address, 100000, true, 1708300000), // after cutoff
		btcTxFixture("tx_old", address, 50000, true, 1708200000),  // before cutoff
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(txs)
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	result, err := provider.FetchTransactions(context.Background(), address, cutoff)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tx (cutoff filter), got %d", len(result))
	}
	if result[0].TxHash != "tx_new" {
		t.Errorf("TxHash = %q, want %q", result[0].TxHash, "tx_new")
	}
}

func TestBlockstreamProvider_FetchTransactions_SkipsOutgoing(t *testing.T) {
	myAddress := "tb1qmine"
	otherAddress := "tb1qother"

	txs := []blockstreamTx{
		btcTxFixture("tx_incoming", myAddress, 100000, true, 1708300000),
		btcTxFixture("tx_outgoing", otherAddress, 50000, true, 1708300000), // not to us
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(txs)
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	result, err := provider.FetchTransactions(context.Background(), myAddress, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tx (skip outgoing), got %d", len(result))
	}
	if result[0].TxHash != "tx_incoming" {
		t.Errorf("expected incoming tx only")
	}
}

func TestBlockstreamProvider_FetchTransactions_MultipleOutputs(t *testing.T) {
	myAddress := "tb1qmine"

	tx := blockstreamTx{
		TxID: "multi_out_tx",
		Status: blockstreamStatus{
			Confirmed: true,
			BlockTime: 1708300000,
		},
		Vout: []blockstreamVout{
			{ScriptPubKeyAddr: myAddress, Value: 50000},
			{ScriptPubKeyAddr: "other", Value: 30000},
			{ScriptPubKeyAddr: myAddress, Value: 25000}, // second output to us
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]blockstreamTx{tx})
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	result, err := provider.FetchTransactions(context.Background(), myAddress, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 aggregated tx, got %d", len(result))
	}
	// 50000 + 25000 = 75000 satoshis
	if result[0].AmountRaw != "75000" {
		t.Errorf("AmountRaw = %q, want %q (sum of both outputs)", result[0].AmountRaw, "75000")
	}
}

func TestBlockstreamProvider_FetchTransactions_Pagination(t *testing.T) {
	myAddress := "tb1qmine"
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First page: 25 txs (full page → triggers next page)
			txs := make([]blockstreamTx, btcPageSize)
			for i := 0; i < btcPageSize; i++ {
				txs[i] = btcTxFixture(
					fmt.Sprintf("tx_%d", i),
					myAddress,
					int64(1000+i),
					true,
					1708300000-int64(i*10),
				)
			}
			json.NewEncoder(w).Encode(txs)
		} else {
			// Second page: fewer than 25 (last page)
			txs := []blockstreamTx{
				btcTxFixture("tx_page2", myAddress, 500, true, 1708290000),
			}
			json.NewEncoder(w).Encode(txs)
		}
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	result, err := provider.FetchTransactions(context.Background(), myAddress, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls (pagination), got %d", callCount)
	}
	if len(result) != btcPageSize+1 {
		t.Errorf("expected %d txs, got %d", btcPageSize+1, len(result))
	}
}

func TestBlockstreamProvider_FetchTransactions_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]blockstreamTx{})
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	result, err := provider.FetchTransactions(context.Background(), "addr", 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 txs, got %d", len(result))
	}
}

func TestBlockstreamProvider_FetchTransactions_HTTP429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	_, err := provider.FetchTransactions(context.Background(), "addr", 0)
	if err == nil {
		t.Fatal("expected error on HTTP 429")
	}
}

func TestBlockstreamProvider_FetchTransactions_HTTP500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	_, err := provider.FetchTransactions(context.Background(), "addr", 0)
	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}

func TestBlockstreamProvider_FetchTransactions_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	_, err := provider.FetchTransactions(context.Background(), "addr", 0)
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestBlockstreamProvider_CheckConfirmation(t *testing.T) {
	tests := []struct {
		name         string
		confirmed    bool
		wantConfirm  bool
		wantConfirms int
	}{
		{"confirmed", true, true, 1},
		{"unconfirmed", false, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := blockstreamTx{
				TxID:   "test_tx",
				Status: blockstreamStatus{Confirmed: tt.confirmed},
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(tx)
			}))
			defer server.Close()

			provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

			confirmed, confirmations, err := provider.CheckConfirmation(context.Background(), "test_tx", 0)
			if err != nil {
				t.Fatalf("CheckConfirmation() error = %v", err)
			}
			if confirmed != tt.wantConfirm {
				t.Errorf("confirmed = %v, want %v", confirmed, tt.wantConfirm)
			}
			if confirmations != tt.wantConfirms {
				t.Errorf("confirmations = %d, want %d", confirmations, tt.wantConfirms)
			}
		})
	}
}

func TestMempoolProvider_Name(t *testing.T) {
	provider := NewMempoolProvider(http.DefaultClient, "testnet")
	if provider.Name() != "mempool" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "mempool")
	}
	if provider.Chain() != "BTC" {
		t.Errorf("Chain() = %q, want %q", provider.Chain(), "BTC")
	}
}

func TestSatoshisToHuman(t *testing.T) {
	tests := []struct {
		sats int64
		want string
	}{
		{100_000_000, "1.00000000"},
		{168841, "0.00168841"},
		{1, "0.00000001"},
		{0, "0.00000000"},
		{50_000_000, "0.50000000"},
	}

	for _, tt := range tests {
		got := satoshisToHuman(tt.sats)
		if got != tt.want {
			t.Errorf("satoshisToHuman(%d) = %q, want %q", tt.sats, got, tt.want)
		}
	}
}

func TestBlockstreamProvider_UnconfirmedTx(t *testing.T) {
	address := "tb1qtest"

	// Unconfirmed tx has block_time=0
	txs := []blockstreamTx{
		{
			TxID:   "unconfirmed_tx",
			Status: blockstreamStatus{Confirmed: false, BlockTime: 0},
			Vout: []blockstreamVout{
				{ScriptPubKeyAddr: address, Value: 10000},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(txs)
	}))
	defer server.Close()

	provider := &BlockstreamProvider{client: server.Client(), baseURL: server.URL}

	result, err := provider.FetchTransactions(context.Background(), address, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	// Unconfirmed tx with block_time=0 should still be included (cutoff=0 allows it)
	if len(result) != 1 {
		t.Fatalf("expected 1 tx, got %d", len(result))
	}
	if result[0].Confirmed {
		t.Error("expected unconfirmed tx")
	}
	if result[0].Confirmations != 0 {
		t.Errorf("confirmations = %d, want 0", result[0].Confirmations)
	}
}
