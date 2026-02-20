package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/go-chi/chi/v5"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/tx"
)

func TestValidateDestination(t *testing.T) {
	tests := []struct {
		name    string
		chain   models.Chain
		address string
		net     *chaincfg.Params
		wantErr bool
	}{
		// BTC valid
		{
			name:    "BTC valid bech32 mainnet",
			chain:   models.ChainBTC,
			address: "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "BTC valid bech32 testnet",
			chain:   models.ChainBTC,
			address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
			net:     &chaincfg.TestNet3Params,
			wantErr: false,
		},
		// BTC invalid
		{
			name:    "BTC empty address",
			chain:   models.ChainBTC,
			address: "",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "BTC invalid format",
			chain:   models.ChainBTC,
			address: "notavalidaddress",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		// Note: btcutil.DecodeAddress with MainNetParams decodes testnet
		// addresses without error — network enforcement is separate.
		{
			name:    "BTC testnet address on mainnet (accepted by decoder)",
			chain:   models.ChainBTC,
			address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},

		// BSC valid
		{
			name:    "BSC valid checksummed",
			chain:   models.ChainBSC,
			address: "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "BSC valid lowercase",
			chain:   models.ChainBSC,
			address: "0xf278cf59f82edcf871d630f28ecc8056f25c1cdb",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		// BSC invalid
		// Note: go-ethereum's common.IsHexAddress accepts addresses without 0x.
		{
			name:    "BSC without 0x prefix (accepted by go-ethereum)",
			chain:   models.ChainBSC,
			address: "F278cF59F82eDcf871d630F28EcC8056f25C1cdb",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "BSC too short",
			chain:   models.ChainBSC,
			address: "0x1234",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "BSC empty",
			chain:   models.ChainBSC,
			address: "",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},

		// BSC EIP-55 checksum
		{
			name:    "BSC valid EIP-55 checksummed",
			chain:   models.ChainBSC,
			address: "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "BSC wrong EIP-55 checksum (mixed case typo)",
			chain:   models.ChainBSC,
			address: "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdB", // last char B instead of b
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "BSC all uppercase (no checksum, accepted)",
			chain:   models.ChainBSC,
			address: "0xF278CF59F82EDCF871D630F28ECC8056F25C1CDB",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},

		// SOL valid
		{
			name:    "SOL valid base58",
			chain:   models.ChainSOL,
			address: "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "SOL valid short address",
			chain:   models.ChainSOL,
			address: "11111111111111111111111111111111",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		// SOL invalid
		{
			name:    "SOL empty",
			chain:   models.ChainSOL,
			address: "",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "SOL invalid chars (contains 0)",
			chain:   models.ChainSOL,
			address: "0Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSn",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "SOL too short",
			chain:   models.ChainSOL,
			address: "abc",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDestination(tt.chain, tt.address, tt.net)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDestination(%s, %q) error = %v, wantErr %v",
					tt.chain, tt.address, err, tt.wantErr)
			}
		})
	}
}

func TestIsValidToken(t *testing.T) {
	tests := []struct {
		name  string
		chain models.Chain
		token models.Token
		want  bool
	}{
		// BTC
		{"BTC NATIVE valid", models.ChainBTC, models.TokenNative, true},
		{"BTC USDC invalid", models.ChainBTC, models.TokenUSDC, false},
		{"BTC USDT invalid", models.ChainBTC, models.TokenUSDT, false},

		// BSC
		{"BSC NATIVE valid", models.ChainBSC, models.TokenNative, true},
		{"BSC USDC valid", models.ChainBSC, models.TokenUSDC, true},
		{"BSC USDT valid", models.ChainBSC, models.TokenUSDT, true},

		// SOL
		{"SOL NATIVE valid", models.ChainSOL, models.TokenNative, true},
		{"SOL USDC valid", models.ChainSOL, models.TokenUSDC, true},
		{"SOL USDT valid", models.ChainSOL, models.TokenUSDT, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidToken(tt.chain, tt.token)
			if got != tt.want {
				t.Errorf("isValidToken(%s, %s) = %v, want %v",
					tt.chain, tt.token, got, tt.want)
			}
		})
	}
}

// --- Send handler test helpers ---

func setupSendTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_send.sqlite")

	database, err := db.New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	t.Cleanup(func() { database.Close() })
	return database
}

func makeSendDeps(t *testing.T, database *db.DB) *SendDeps {
	t.Helper()
	return &SendDeps{
		DB:     database,
		Config: &config.Config{Network: "testnet"},
		TxHub:  tx.NewTxSSEHub(),
		NetParams: &chaincfg.TestNet3Params,
		ChainLocks: map[models.Chain]*sync.Mutex{
			models.ChainBTC: {},
			models.ChainBSC: {},
			models.ChainSOL: {},
		},
	}
}

func setupSendRouter(t *testing.T, deps *SendDeps) http.Handler {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/api/send/preview", PreviewSend(deps))
	r.Post("/api/send/execute", ExecuteSend(deps))
	r.Post("/api/send/gas-preseed", GasPreSeedHandler(deps))
	r.Get("/api/send/sse", SendSSE(deps.TxHub))
	r.Get("/api/send/pending", GetPendingTxStates(deps))
	r.Get("/api/send/sweep/{sweepID}", GetSweepStatus(deps))
	r.Get("/api/send/resume/{sweepID}", GetResumeSummary(deps))
	r.Post("/api/send/resume", ExecuteResume(deps))
	r.Post("/api/send/dismiss/{id}", DismissTxState(deps))
	return r
}

func assertErrorCode(t *testing.T, body []byte, expectedCode string) {
	t.Helper()
	var errResp models.APIError
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v\nbody: %s", err, string(body))
	}
	if errResp.Error.Code != expectedCode {
		t.Errorf("error code = %q, want %q\nfull body: %s", errResp.Error.Code, expectedCode, string(body))
	}
}

// --- PreviewSend tests ---

func TestPreviewSend_InvalidBody(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	req := httptest.NewRequest("POST", "/api/send/preview", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPreviewSend_InvalidChain(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"chain":"DOGE","token":"NATIVE","destination":"some_address"}`
	req := httptest.NewRequest("POST", "/api/send/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorInvalidChain)
}

func TestPreviewSend_InvalidToken(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"chain":"BTC","token":"USDC","destination":"tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"}`
	req := httptest.NewRequest("POST", "/api/send/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorInvalidToken)
}

func TestPreviewSend_InvalidDestination(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"chain":"BTC","token":"NATIVE","destination":"invalid_addr"}`
	req := httptest.NewRequest("POST", "/api/send/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorInvalidDestination)
}

func TestPreviewSend_NoFundedAddresses(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	// Seed addresses but no balances.
	addrs := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"},
	}
	if err := database.InsertAddressBatch(models.ChainBTC, addrs); err != nil {
		t.Fatalf("InsertAddressBatch() error = %v", err)
	}

	body := `{"chain":"BTC","token":"NATIVE","destination":"tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"}`
	req := httptest.NewRequest("POST", "/api/send/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorNoFundedAddresses)
}

func TestPreviewSend_CaseInsensitiveChain(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	// Lowercase chain should be accepted (normalized to uppercase).
	body := `{"chain":"btc","token":"native","destination":"tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"}`
	req := httptest.NewRequest("POST", "/api/send/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail with "no funded addresses" (not "invalid chain"), proving normalization works.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorNoFundedAddresses)
}

// --- ExecuteSend tests ---

func TestExecuteSend_InvalidBody(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	req := httptest.NewRequest("POST", "/api/send/execute", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestExecuteSend_InvalidChain(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"chain":"XRP","token":"NATIVE","destination":"some_addr"}`
	req := httptest.NewRequest("POST", "/api/send/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorInvalidChain)
}

func TestExecuteSend_NoFundedAddresses(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"chain":"BSC","token":"NATIVE","destination":"0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb"}`
	req := httptest.NewRequest("POST", "/api/send/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorNoFundedAddresses)
}

func TestExecuteSend_ChainLockConflict(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	// Seed BSC addresses with balances.
	addrs := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb"},
	}
	if err := database.InsertAddressBatch(models.ChainBSC, addrs); err != nil {
		t.Fatalf("InsertAddressBatch() error = %v", err)
	}
	balances := []models.Balance{
		{Chain: models.ChainBSC, AddressIndex: 0, Token: models.TokenNative, Balance: "1000000000000000000"},
	}
	if err := database.UpsertBalanceBatch(balances); err != nil {
		t.Fatalf("UpsertBalanceBatch() error = %v", err)
	}

	// Pre-lock the BSC chain mutex.
	deps.ChainLocks[models.ChainBSC].Lock()
	defer deps.ChainLocks[models.ChainBSC].Unlock()

	body := `{"chain":"BSC","token":"NATIVE","destination":"0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb"}`
	req := httptest.NewRequest("POST", "/api/send/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorSendBusy)
}

// --- GasPreSeedHandler tests ---

func TestGasPreSeed_InvalidBody(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	req := httptest.NewRequest("POST", "/api/send/gas-preseed", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGasPreSeed_EmptyTargets(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"sourceIndex":0,"targetAddresses":[]}`
	req := httptest.NewRequest("POST", "/api/send/gas-preseed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorGasPreSeedFailed)
}

func TestGasPreSeed_InvalidTargetAddress(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"sourceIndex":0,"targetAddresses":["not_a_valid_address"]}`
	req := httptest.NewRequest("POST", "/api/send/gas-preseed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorInvalidAddress)
}

// --- DismissTxState tests ---

func TestDismissTxState_EmptyID(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)

	r := chi.NewRouter()
	r.Post("/api/send/dismiss/{id}", DismissTxState(deps))

	// Chi won't match a route with an empty URL param — the route just won't match.
	// Test with a valid param that doesn't exist in DB.
	req := httptest.NewRequest("POST", "/api/send/dismiss/nonexistent-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// UpdateTxStatus for nonexistent ID should succeed (no rows affected = no error in SQLite).
	// The handler returns 200 regardless.
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

func TestDismissTxState_Success(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)

	// Create a tx_state to dismiss.
	txState := db.TxStateRow{
		ID:           "test-dismiss-id",
		SweepID:      "sweep-1",
		Chain:        "BSC",
		Token:        "USDC",
		AddressIndex: 0,
		FromAddress:  "0xabc",
		ToAddress:    "0xdef",
		Amount:       "1000000",
		Status:       config.TxStateUncertain,
	}
	if err := database.CreateTxState(txState); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	r := chi.NewRouter()
	r.Post("/api/send/dismiss/{id}", DismissTxState(deps))

	req := httptest.NewRequest("POST", "/api/send/dismiss/test-dismiss-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

// --- GetPendingTxStates tests ---

func TestGetPendingTxStates_NoFilter(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	// Create some tx_states.
	for i, status := range []string{config.TxStatePending, config.TxStateConfirmed, config.TxStateBroadcasting} {
		if err := database.CreateTxState(db.TxStateRow{
			ID:           fmt.Sprintf("tx-%d", i),
			SweepID:      "sweep-pending",
			Chain:        "BTC",
			Token:        "NATIVE",
			AddressIndex: i,
			FromAddress:  fmt.Sprintf("addr-%d", i),
			ToAddress:    "dest",
			Amount:       "1000",
			Status:       status,
		}); err != nil {
			t.Fatalf("CreateTxState() error = %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/api/send/pending", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []db.TxStateRow `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should return pending + broadcasting (non-terminal), not confirmed.
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 pending states, got %d", len(resp.Data))
	}
}

func TestGetPendingTxStates_ChainFilter(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	// Create tx_states for different chains.
	for i, chain := range []string{"BTC", "BSC"} {
		if err := database.CreateTxState(db.TxStateRow{
			ID:           fmt.Sprintf("tx-chain-%d", i),
			SweepID:      "sweep-filter",
			Chain:        chain,
			Token:        "NATIVE",
			AddressIndex: i,
			FromAddress:  fmt.Sprintf("addr-%d", i),
			ToAddress:    "dest",
			Amount:       "1000",
			Status:       config.TxStatePending,
		}); err != nil {
			t.Fatalf("CreateTxState() error = %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/api/send/pending?chain=btc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []db.TxStateRow `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Only BTC pending state should be returned.
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 BTC pending state, got %d", len(resp.Data))
	}
}

// --- GetSweepStatus tests ---

func TestGetSweepStatus_EmptySweepID(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)

	r := chi.NewRouter()
	r.Get("/api/send/sweep/{sweepID}", GetSweepStatus(deps))

	// Empty sweep ID won't match route. Test with a non-existent sweep.
	req := httptest.NewRequest("GET", "/api/send/sweep/nonexistent-sweep", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []models.TxResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Empty sweep returns empty array.
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 results for nonexistent sweep, got %d", len(resp.Data))
	}
}

func TestGetSweepStatus_WithResults(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)

	// Create tx_states for a sweep.
	for i := 0; i < 3; i++ {
		if err := database.CreateTxState(db.TxStateRow{
			ID:           fmt.Sprintf("sweep-status-%d", i),
			SweepID:      "sweep-status-test",
			Chain:        "BSC",
			Token:        "NATIVE",
			AddressIndex: i,
			FromAddress:  fmt.Sprintf("0xaddr%d", i),
			ToAddress:    "0xdest",
			Amount:       "1000",
			Status:       config.TxStateConfirmed,
		}); err != nil {
			t.Fatalf("CreateTxState() error = %v", err)
		}
	}

	r := chi.NewRouter()
	r.Get("/api/send/sweep/{sweepID}", GetSweepStatus(deps))

	req := httptest.NewRequest("GET", "/api/send/sweep/sweep-status-test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []models.TxResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Data) != 3 {
		t.Errorf("expected 3 results, got %d", len(resp.Data))
	}
}

// --- GetResumeSummary tests ---

func TestGetResumeSummary_NotFound(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)

	r := chi.NewRouter()
	r.Get("/api/send/resume/{sweepID}", GetResumeSummary(deps))

	req := httptest.NewRequest("GET", "/api/send/resume/nonexistent-sweep", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorSweepNotFound)
}

func TestGetResumeSummary_WithMixedStatuses(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)

	// Create tx_states with various statuses.
	statuses := []string{
		config.TxStateConfirmed,
		config.TxStateConfirmed,
		config.TxStateFailed,
		config.TxStateUncertain,
		config.TxStatePending,
	}
	for i, status := range statuses {
		if err := database.CreateTxState(db.TxStateRow{
			ID:           fmt.Sprintf("resume-%d", i),
			SweepID:      "resume-test-sweep",
			Chain:        "SOL",
			Token:        "USDC",
			AddressIndex: i,
			FromAddress:  fmt.Sprintf("addr%d", i),
			ToAddress:    "dest",
			Amount:       "1000000",
			Status:       status,
		}); err != nil {
			t.Fatalf("CreateTxState() error = %v", err)
		}
	}

	r := chi.NewRouter()
	r.Get("/api/send/resume/{sweepID}", GetResumeSummary(deps))

	req := httptest.NewRequest("GET", "/api/send/resume/resume-test-sweep", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data models.ResumeSummary `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	summary := resp.Data
	if summary.SweepID != "resume-test-sweep" {
		t.Errorf("sweepID = %q, want resume-test-sweep", summary.SweepID)
	}
	if summary.Chain != "SOL" {
		t.Errorf("chain = %q, want SOL", summary.Chain)
	}
	if summary.TotalTxs != 5 {
		t.Errorf("totalTxs = %d, want 5", summary.TotalTxs)
	}
	if summary.Confirmed != 2 {
		t.Errorf("confirmed = %d, want 2", summary.Confirmed)
	}
	if summary.Failed != 1 {
		t.Errorf("failed = %d, want 1", summary.Failed)
	}
	if summary.Uncertain != 1 {
		t.Errorf("uncertain = %d, want 1", summary.Uncertain)
	}
	if summary.ToRetry != 2 {
		t.Errorf("toRetry = %d, want 2 (1 failed + 1 uncertain)", summary.ToRetry)
	}
}

// --- ExecuteResume tests ---

func TestExecuteResume_InvalidBody(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	req := httptest.NewRequest("POST", "/api/send/resume", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestExecuteResume_EmptySweepID(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"sweepID":"","destination":"tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"}`
	req := httptest.NewRequest("POST", "/api/send/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorSweepNotFound)
}

func TestExecuteResume_SweepNotFound(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	body := `{"sweepID":"nonexistent-sweep"}`
	req := httptest.NewRequest("POST", "/api/send/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorSweepNotFound)
}

func TestExecuteResume_NoRetryableTxs(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	// Create a sweep with all confirmed (no retryable).
	if err := database.CreateTxState(db.TxStateRow{
		ID:           "resume-no-retry",
		SweepID:      "all-confirmed-sweep",
		Chain:        "BTC",
		Token:        "NATIVE",
		AddressIndex: 0,
		FromAddress:  "addr0",
		ToAddress:    "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Amount:       "1000",
		Status:       config.TxStateConfirmed,
	}); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	body := `{"sweepID":"all-confirmed-sweep"}`
	req := httptest.NewRequest("POST", "/api/send/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data models.UnifiedSendResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Data.TotalSwept != "0" {
		t.Errorf("totalSwept = %q, want \"0\"", resp.Data.TotalSwept)
	}
}

func TestExecuteResume_ChainLockConflict(t *testing.T) {
	database := setupSendTestDB(t)
	deps := makeSendDeps(t, database)
	router := setupSendRouter(t, deps)

	// Need BSC addresses with balances for the resume to find funded addresses.
	addrs := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb"},
	}
	if err := database.InsertAddressBatch(models.ChainBSC, addrs); err != nil {
		t.Fatalf("InsertAddressBatch() error = %v", err)
	}
	balances := []models.Balance{
		{Chain: models.ChainBSC, AddressIndex: 0, Token: models.TokenNative, Balance: "1000000000000000000"},
	}
	if err := database.UpsertBalanceBatch(balances); err != nil {
		t.Fatalf("UpsertBalanceBatch() error = %v", err)
	}

	// Create a sweep with a failed TX.
	if err := database.CreateTxState(db.TxStateRow{
		ID:           "resume-lock-conflict",
		SweepID:      "lock-conflict-sweep",
		Chain:        "BSC",
		Token:        "NATIVE",
		AddressIndex: 0,
		FromAddress:  "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb",
		ToAddress:    "0xf785bD075874b8423D3583728a981399f31e95aA",
		Amount:       "1000000000000000000",
		Status:       config.TxStateFailed,
	}); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	// Pre-lock BSC chain.
	deps.ChainLocks[models.ChainBSC].Lock()
	defer deps.ChainLocks[models.ChainBSC].Unlock()

	body := `{"sweepID":"lock-conflict-sweep","destination":"0xf785bD075874b8423D3583728a981399f31e95aA"}`
	req := httptest.NewRequest("POST", "/api/send/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409. body: %s", w.Code, w.Body.String())
	}
	assertErrorCode(t, w.Body.Bytes(), config.ErrorSendBusy)
}

// --- SendSSE tests ---

func TestSendSSE_Headers(t *testing.T) {
	hub := tx.NewTxSSEHub()
	r := chi.NewRouter()
	r.Get("/api/send/sse", SendSSE(hub))

	// Use a context with timeout to avoid hanging.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/send/sse", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Run in goroutine since SSE blocks.
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(w, req)
		close(done)
	}()

	<-done

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if conn := w.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", conn)
	}
}

func TestSendSSE_ReceivesEvent(t *testing.T) {
	hub := tx.NewTxSSEHub()
	r := chi.NewRouter()
	r.Get("/api/send/sse", SendSSE(hub))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/send/sse", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		r.ServeHTTP(w, req)
		close(done)
	}()

	// Give SSE handler time to subscribe.
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event.
	hub.Broadcast(tx.TxEvent{
		Type: "tx_status",
		Data: map[string]string{"chain": "BTC", "status": "confirmed"},
	})

	// Give time for the event to be written.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := w.Body.String()
	if !strings.Contains(body, "event: tx_status") {
		t.Errorf("expected SSE body to contain 'event: tx_status', got: %s", body)
	}
}

// --- getTokenContractAddress tests ---

func TestGetTokenContractAddress(t *testing.T) {
	tests := []struct {
		name    string
		chain   models.Chain
		token   models.Token
		network string
		want    string
	}{
		{"BSC USDC mainnet", models.ChainBSC, models.TokenUSDC, "mainnet", config.BSCUSDCContract},
		{"BSC USDT mainnet", models.ChainBSC, models.TokenUSDT, "mainnet", config.BSCUSDTContract},
		{"BSC USDC testnet", models.ChainBSC, models.TokenUSDC, "testnet", config.BSCTestnetUSDCContract},
		{"BSC USDT testnet", models.ChainBSC, models.TokenUSDT, "testnet", config.BSCTestnetUSDTContract},
		{"SOL USDC mainnet", models.ChainSOL, models.TokenUSDC, "mainnet", config.SOLUSDCMint},
		{"SOL USDT mainnet", models.ChainSOL, models.TokenUSDT, "mainnet", config.SOLUSDTMint},
		{"SOL USDC testnet", models.ChainSOL, models.TokenUSDC, "testnet", config.SOLDevnetUSDCMint},
		{"SOL USDT testnet", models.ChainSOL, models.TokenUSDT, "testnet", config.SOLDevnetUSDTMint},
		{"BTC NATIVE (no contract)", models.ChainBTC, models.TokenNative, "mainnet", ""},
		{"BSC NATIVE (no contract)", models.ChainBSC, models.TokenNative, "mainnet", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTokenContractAddress(tt.chain, tt.token, tt.network)
			if got != tt.want {
				t.Errorf("getTokenContractAddress(%s, %s, %s) = %q, want %q",
					tt.chain, tt.token, tt.network, got, tt.want)
			}
		})
	}
}

