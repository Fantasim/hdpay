package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	hdconfig "github.com/Fantasim/hdpay/internal/config"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
)

// --- BSC test fixtures ---

func bscNormalTxJSON(hash, to, value, timestamp string, isError string) bscNormalTx {
	return bscNormalTx{
		Hash:      hash,
		From:      "0xsender",
		To:        to,
		Value:     value,
		IsError:   isError,
		TimeStamp: timestamp,
	}
}

func bscTokenTxJSON(hash, to, contract, value, timestamp string) bscTokenTx {
	return bscTokenTx{
		Hash:            hash,
		From:            "0xsender",
		To:              to,
		Value:           value,
		ContractAddress: contract,
		TokenDecimal:    "18",
		TimeStamp:       timestamp,
	}
}

func bscScanSuccessResponse(t *testing.T, result interface{}) []byte {
	resultBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	resp := bscScanResponse{
		Status:  "1",
		Message: "OK",
		Result:  json.RawMessage(resultBytes),
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestBscScanProvider_FetchTransactions_NormalBNB(t *testing.T) {
	address := "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb"

	normalTxs := []bscNormalTx{
		bscNormalTxJSON("0xhash1", address, "1000000000000000000", "1708300000", "0"), // 1 BNB
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		action := r.URL.Query().Get("action")
		switch action {
		case "txlist":
			w.Write(bscScanSuccessResponse(t, normalTxs))
		case "tokentx":
			w.Write(bscScanSuccessResponse(t, []bscTokenTx{}))
		default:
			t.Errorf("unexpected action: %s", action)
		}
	}))
	defer server.Close()

	provider := &BscScanProvider{
		client:  server.Client(),
		baseURL: server.URL,
		apiKey:  "test",
		network: "mainnet",
	}

	result, err := provider.FetchTransactions(context.Background(), address, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tx, got %d", len(result))
	}
	if result[0].Token != "BNB" {
		t.Errorf("Token = %q, want %q", result[0].Token, "BNB")
	}
	if result[0].AmountRaw != "1000000000000000000" {
		t.Errorf("AmountRaw = %q, want 1 BNB in wei", result[0].AmountRaw)
	}
	if result[0].AmountHuman != "1.000000000000000000" {
		t.Errorf("AmountHuman = %q, want %q", result[0].AmountHuman, "1.000000000000000000")
	}
	if result[0].Decimals != hdconfig.BNBDecimals {
		t.Errorf("Decimals = %d, want %d", result[0].Decimals, hdconfig.BNBDecimals)
	}
}

func TestBscScanProvider_FetchTransactions_TokenUSDC(t *testing.T) {
	address := "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb"

	tokenTxs := []bscTokenTx{
		bscTokenTxJSON("0xhash2", address, hdconfig.BSCUSDCContract, "1000000000000000000", "1708300000"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		switch action {
		case "txlist":
			w.Write(bscScanSuccessResponse(t, []bscNormalTx{}))
		case "tokentx":
			w.Write(bscScanSuccessResponse(t, tokenTxs))
		}
	}))
	defer server.Close()

	provider := &BscScanProvider{
		client:  server.Client(),
		baseURL: server.URL,
		apiKey:  "test",
		network: "mainnet",
	}

	result, err := provider.FetchTransactions(context.Background(), address, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 token tx, got %d", len(result))
	}
	if result[0].Token != "USDC" {
		t.Errorf("Token = %q, want %q", result[0].Token, "USDC")
	}
	if result[0].TxHash != "0xhash2" {
		t.Errorf("TxHash = %q, want %q", result[0].TxHash, "0xhash2")
	}
}

func TestBscScanProvider_FetchTransactions_SkipsOutgoing(t *testing.T) {
	address := "0xmine"

	normalTxs := []bscNormalTx{
		bscNormalTxJSON("0xout", "0xother", "1000000000000000000", "1708300000", "0"), // outgoing
		bscNormalTxJSON("0xin", address, "500000000000000000", "1708300000", "0"),     // incoming
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		switch action {
		case "txlist":
			w.Write(bscScanSuccessResponse(t, normalTxs))
		case "tokentx":
			w.Write(bscScanSuccessResponse(t, []bscTokenTx{}))
		}
	}))
	defer server.Close()

	provider := &BscScanProvider{
		client:  server.Client(),
		baseURL: server.URL,
		apiKey:  "test",
		network: "mainnet",
	}

	result, err := provider.FetchTransactions(context.Background(), address, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tx (skip outgoing), got %d", len(result))
	}
	if result[0].TxHash != "0xin" {
		t.Error("expected incoming tx only")
	}
}

func TestBscScanProvider_FetchTransactions_SkipsFailedTx(t *testing.T) {
	address := "0xmine"

	normalTxs := []bscNormalTx{
		bscNormalTxJSON("0xfailed", address, "1000000000000000000", "1708300000", "1"), // failed
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		switch action {
		case "txlist":
			w.Write(bscScanSuccessResponse(t, normalTxs))
		case "tokentx":
			w.Write(bscScanSuccessResponse(t, []bscTokenTx{}))
		}
	}))
	defer server.Close()

	provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

	result, err := provider.FetchTransactions(context.Background(), address, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 txs (failed skipped), got %d", len(result))
	}
}

func TestBscScanProvider_FetchTransactions_SkipsUnknownToken(t *testing.T) {
	address := "0xmine"

	tokenTxs := []bscTokenTx{
		bscTokenTxJSON("0xunknown", address, "0xUnknownContract", "1000000", "1708300000"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		switch action {
		case "txlist":
			w.Write(bscScanSuccessResponse(t, []bscNormalTx{}))
		case "tokentx":
			w.Write(bscScanSuccessResponse(t, tokenTxs))
		}
	}))
	defer server.Close()

	provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

	result, err := provider.FetchTransactions(context.Background(), address, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 txs (unknown token skipped), got %d", len(result))
	}
}

func TestBscScanProvider_FetchTransactions_CutoffFilter(t *testing.T) {
	address := "0xmine"
	cutoff := int64(1708250000)

	normalTxs := []bscNormalTx{
		bscNormalTxJSON("0xnew", address, "1000000000000000000", "1708300000", "0"), // after
		bscNormalTxJSON("0xold", address, "500000000000000000", "1708200000", "0"),  // before
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		switch action {
		case "txlist":
			w.Write(bscScanSuccessResponse(t, normalTxs))
		case "tokentx":
			w.Write(bscScanSuccessResponse(t, []bscTokenTx{}))
		}
	}))
	defer server.Close()

	provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

	result, err := provider.FetchTransactions(context.Background(), address, cutoff)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 tx after cutoff, got %d", len(result))
	}
}

func TestBscScanProvider_FetchTransactions_NoTransactionsFound(t *testing.T) {
	resp := bscScanResponse{
		Status:  "0",
		Message: "No transactions found",
		Result:  json.RawMessage(`[]`),
	}
	respBytes, _ := json.Marshal(resp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(respBytes)
	}))
	defer server.Close()

	provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

	result, err := provider.FetchTransactions(context.Background(), "0xaddr", 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 txs, got %d", len(result))
	}
}

func TestBscScanProvider_FetchTransactions_APIError(t *testing.T) {
	resp := bscScanResponse{
		Status:  "0",
		Message: "NOTOK",
		Result:  json.RawMessage(`"Max rate limit reached"`),
	}
	respBytes, _ := json.Marshal(resp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(respBytes)
	}))
	defer server.Close()

	provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

	_, err := provider.FetchTransactions(context.Background(), "0xaddr", 0)
	if err == nil {
		t.Fatal("expected error on API error response")
	}
}

func TestBscScanProvider_CheckConfirmation(t *testing.T) {
	tests := []struct {
		name          string
		currentBlock  uint64
		txBlock       int64
		wantConfirmed bool
		wantConfs     int
	}{
		{"enough confirmations", 1000, 980, true, 20},
		{"not enough", 1000, 995, false, 5},
		{"exact threshold", 1000, int64(1000 - pollerconfig.ConfirmationsBSC), true, pollerconfig.ConfirmationsBSC},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockResp := bscBlockNumberResult{
				Result: "0x" + uintToHex(tt.currentBlock),
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(blockResp)
			}))
			defer server.Close()

			provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

			confirmed, confs, err := provider.CheckConfirmation(context.Background(), "0xhash", tt.txBlock)
			if err != nil {
				t.Fatalf("CheckConfirmation() error = %v", err)
			}
			if confirmed != tt.wantConfirmed {
				t.Errorf("confirmed = %v, want %v", confirmed, tt.wantConfirmed)
			}
			if confs != tt.wantConfs {
				t.Errorf("confirmations = %d, want %d", confs, tt.wantConfs)
			}
		})
	}
}

func TestBscScanProvider_GetCurrentBlock(t *testing.T) {
	blockResp := bscBlockNumberResult{Result: "0x3e8"} // 1000

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(blockResp)
	}))
	defer server.Close()

	provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

	block, err := provider.GetCurrentBlock(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentBlock() error = %v", err)
	}
	if block != 1000 {
		t.Errorf("block = %d, want 1000", block)
	}
}

func TestBscScanProvider_HTTP429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

	_, err := provider.FetchTransactions(context.Background(), "0xaddr", 0)
	if err == nil {
		t.Fatal("expected error on HTTP 429")
	}
}

func TestBscScanProvider_SkipsZeroValue(t *testing.T) {
	address := "0xmine"

	normalTxs := []bscNormalTx{
		bscNormalTxJSON("0xzero", address, "0", "1708300000", "0"), // zero value
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		switch action {
		case "txlist":
			w.Write(bscScanSuccessResponse(t, normalTxs))
		case "tokentx":
			w.Write(bscScanSuccessResponse(t, []bscTokenTx{}))
		}
	}))
	defer server.Close()

	provider := &BscScanProvider{client: server.Client(), baseURL: server.URL, apiKey: "test", network: "mainnet"}

	result, err := provider.FetchTransactions(context.Background(), address, 0)
	if err != nil {
		t.Fatalf("FetchTransactions() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 txs (zero-value skipped), got %d", len(result))
	}
}

func TestWeiToHuman(t *testing.T) {
	tests := []struct {
		raw      string
		decimals int
		want     string
	}{
		{"1000000000000000000", 18, "1.000000000000000000"},
		{"500000000000000000", 18, "0.500000000000000000"},
		{"1000000", 6, "1.000000"},
		{"0", 18, "0.000000000000000000"},
		{"invalid", 18, "0"},
	}

	for _, tt := range tests {
		got := weiToHuman(tt.raw, tt.decimals)
		if got != tt.want {
			t.Errorf("weiToHuman(%q, %d) = %q, want %q", tt.raw, tt.decimals, got, tt.want)
		}
	}
}

// uintToHex converts a uint64 to hex string without 0x prefix.
func uintToHex(n uint64) string {
	return fmt.Sprintf("%x", n)
}
