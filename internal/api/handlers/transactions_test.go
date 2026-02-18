package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/go-chi/chi/v5"
)

func setupTransactionsRouter(t *testing.T) (http.Handler, *db.DB) {
	t.Helper()
	database := setupTestDB(t)

	r := chi.NewRouter()
	r.Get("/api/transactions", ListTransactions(database))
	r.Get("/api/transactions/{chain}", ListTransactions(database))

	return r, database
}

func seedTransactions(t *testing.T, database *db.DB) {
	t.Helper()

	txs := []models.Transaction{
		{Chain: models.ChainBTC, AddressIndex: 0, TxHash: "btc00000000000000000000000000000000000000000000000000000000000000", Direction: "out", Token: models.TokenNative, Amount: "10000", FromAddress: "bc1qa", ToAddress: "bc1qb", Status: "confirmed"},
		{Chain: models.ChainBTC, AddressIndex: 1, TxHash: "btc10000000000000000000000000000000000000000000000000000000000000", Direction: "in", Token: models.TokenNative, Amount: "20000", FromAddress: "bc1qc", ToAddress: "bc1qd", Status: "pending"},
		{Chain: models.ChainBSC, AddressIndex: 0, TxHash: "bsc00000000000000000000000000000000000000000000000000000000000000", Direction: "out", Token: models.TokenUSDC, Amount: "5000000", FromAddress: "0xa", ToAddress: "0xb", Status: "confirmed"},
		{Chain: models.ChainBSC, AddressIndex: 1, TxHash: "bsc10000000000000000000000000000000000000000000000000000000000000", Direction: "out", Token: models.TokenNative, Amount: "100000", FromAddress: "0xc", ToAddress: "0xd", Status: "failed"},
		{Chain: models.ChainSOL, AddressIndex: 0, TxHash: "sol00000000000000000000000000000000000000000000000000000000000000", Direction: "out", Token: models.TokenNative, Amount: "1000000000", FromAddress: "So1a", ToAddress: "So1b", Status: "confirmed"},
	}

	for _, tx := range txs {
		if _, err := database.InsertTransaction(tx); err != nil {
			t.Fatalf("InsertTransaction() error = %v", err)
		}
	}
}

func TestListTransactionsHandler_All(t *testing.T) {
	router, database := setupTransactionsRouter(t)
	seedTransactions(t, database)

	req := httptest.NewRequest("GET", "/api/transactions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta == nil {
		t.Fatal("meta is nil")
	}
	if resp.Meta.Total != 5 {
		t.Errorf("total = %d, want 5", resp.Meta.Total)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("data is not a slice")
	}
	if len(data) != 5 {
		t.Errorf("len(data) = %d, want 5", len(data))
	}
}

func TestListTransactionsHandler_FilterByChainPath(t *testing.T) {
	router, database := setupTransactionsRouter(t)
	seedTransactions(t, database)

	req := httptest.NewRequest("GET", "/api/transactions/BTC", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Meta.Total)
	}
}

func TestListTransactionsHandler_FilterByChainQuery(t *testing.T) {
	router, database := setupTransactionsRouter(t)
	seedTransactions(t, database)

	req := httptest.NewRequest("GET", "/api/transactions?chain=BSC", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Meta.Total)
	}
}

func TestListTransactionsHandler_FilterByDirection(t *testing.T) {
	router, database := setupTransactionsRouter(t)
	seedTransactions(t, database)

	req := httptest.NewRequest("GET", "/api/transactions?direction=in", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Meta.Total)
	}
}

func TestListTransactionsHandler_FilterByToken(t *testing.T) {
	router, database := setupTransactionsRouter(t)
	seedTransactions(t, database)

	req := httptest.NewRequest("GET", "/api/transactions?token=USDC", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Meta.Total)
	}
}

func TestListTransactionsHandler_FilterByStatus(t *testing.T) {
	router, database := setupTransactionsRouter(t)
	seedTransactions(t, database)

	req := httptest.NewRequest("GET", "/api/transactions?status=confirmed", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Meta.Total)
	}
}

func TestListTransactionsHandler_InvalidChain(t *testing.T) {
	router, _ := setupTransactionsRouter(t)

	req := httptest.NewRequest("GET", "/api/transactions/INVALID", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestListTransactionsHandler_InvalidDirection(t *testing.T) {
	router, _ := setupTransactionsRouter(t)

	req := httptest.NewRequest("GET", "/api/transactions?direction=sideways", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestListTransactionsHandler_InvalidToken(t *testing.T) {
	router, _ := setupTransactionsRouter(t)

	req := httptest.NewRequest("GET", "/api/transactions?token=DOGE", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestListTransactionsHandler_InvalidStatus(t *testing.T) {
	router, _ := setupTransactionsRouter(t)

	req := httptest.NewRequest("GET", "/api/transactions?status=unknown", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestListTransactionsHandler_Pagination(t *testing.T) {
	router, database := setupTransactionsRouter(t)
	seedTransactions(t, database)

	req := httptest.NewRequest("GET", "/api/transactions?page=1&pageSize=2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta.Total != 5 {
		t.Errorf("total = %d, want 5", resp.Meta.Total)
	}
	if resp.Meta.PageSize != 2 {
		t.Errorf("pageSize = %d, want 2", resp.Meta.PageSize)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("data is not a slice")
	}
	if len(data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(data))
	}
}
