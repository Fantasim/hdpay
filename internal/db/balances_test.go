package db

import (
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/models"
)

// setupTestDB and seedAddresses are defined in addresses_test.go.

func TestUpsertBalance(t *testing.T) {
	database := setupTestDB(t)

	seedAddresses(t, database, models.ChainBTC, 1)

	err := database.UpsertBalance(models.ChainBTC, 0, models.TokenNative, "100000")
	if err != nil {
		t.Fatalf("UpsertBalance() error = %v", err)
	}

	// Verify via GetFundedAddresses.
	funded, err := database.GetFundedAddresses(models.ChainBTC, models.TokenNative)
	if err != nil {
		t.Fatalf("GetFundedAddresses() error = %v", err)
	}
	if len(funded) != 1 {
		t.Fatalf("expected 1 funded, got %d", len(funded))
	}
	if funded[0].Balance != "100000" {
		t.Errorf("expected balance 100000, got %s", funded[0].Balance)
	}
}

func TestUpsertBalance_Update(t *testing.T) {
	database := setupTestDB(t)

	seedAddresses(t, database, models.ChainBTC, 1)

	// Insert initial balance.
	database.UpsertBalance(models.ChainBTC, 0, models.TokenNative, "100000")

	// Update balance.
	err := database.UpsertBalance(models.ChainBTC, 0, models.TokenNative, "200000")
	if err != nil {
		t.Fatalf("UpsertBalance() update error = %v", err)
	}

	funded, err := database.GetFundedAddresses(models.ChainBTC, models.TokenNative)
	if err != nil {
		t.Fatalf("GetFundedAddresses() error = %v", err)
	}
	if len(funded) != 1 {
		t.Fatalf("expected 1 funded, got %d", len(funded))
	}
	if funded[0].Balance != "200000" {
		t.Errorf("expected balance 200000, got %s", funded[0].Balance)
	}
}

func TestUpsertBalanceBatch(t *testing.T) {
	database := setupTestDB(t)

	seedAddresses(t, database, models.ChainBSC, 3)

	balances := []models.Balance{
		{Chain: models.ChainBSC, AddressIndex: 0, Token: models.TokenNative, Balance: "1000"},
		{Chain: models.ChainBSC, AddressIndex: 1, Token: models.TokenNative, Balance: "0"},
		{Chain: models.ChainBSC, AddressIndex: 2, Token: models.TokenNative, Balance: "5000"},
	}

	err := database.UpsertBalanceBatch(balances)
	if err != nil {
		t.Fatalf("UpsertBalanceBatch() error = %v", err)
	}

	funded, err := database.GetFundedAddresses(models.ChainBSC, models.TokenNative)
	if err != nil {
		t.Fatalf("GetFundedAddresses() error = %v", err)
	}

	// Only 2 addresses have non-zero balance.
	if len(funded) != 2 {
		t.Fatalf("expected 2 funded, got %d", len(funded))
	}
}

func TestUpsertBalanceBatch_Empty(t *testing.T) {
	database := setupTestDB(t)

	err := database.UpsertBalanceBatch(nil)
	if err != nil {
		t.Errorf("UpsertBalanceBatch(nil) error = %v", err)
	}

	err = database.UpsertBalanceBatch([]models.Balance{})
	if err != nil {
		t.Errorf("UpsertBalanceBatch([]) error = %v", err)
	}
}

func TestGetFundedAddresses_NoResults(t *testing.T) {
	database := setupTestDB(t)

	funded, err := database.GetFundedAddresses(models.ChainSOL, models.TokenNative)
	if err != nil {
		t.Fatalf("GetFundedAddresses() error = %v", err)
	}
	if len(funded) != 0 {
		t.Errorf("expected 0 funded, got %d", len(funded))
	}
}

func TestGetFundedAddresses_TokenFilter(t *testing.T) {
	database := setupTestDB(t)

	seedAddresses(t, database, models.ChainBSC, 2)

	// Insert native and USDC balances.
	database.UpsertBalance(models.ChainBSC, 0, models.TokenNative, "1000")
	database.UpsertBalance(models.ChainBSC, 0, models.TokenUSDC, "5000000")
	database.UpsertBalance(models.ChainBSC, 1, models.TokenNative, "2000")

	// Filter by USDC — only 1 result.
	funded, err := database.GetFundedAddresses(models.ChainBSC, models.TokenUSDC)
	if err != nil {
		t.Fatalf("GetFundedAddresses(USDC) error = %v", err)
	}
	if len(funded) != 1 {
		t.Fatalf("expected 1 funded USDC, got %d", len(funded))
	}
	if funded[0].Token != models.TokenUSDC {
		t.Errorf("expected USDC token, got %s", funded[0].Token)
	}

	// Filter by native — 2 results.
	nativeFunded, err := database.GetFundedAddresses(models.ChainBSC, models.TokenNative)
	if err != nil {
		t.Fatalf("GetFundedAddresses(NATIVE) error = %v", err)
	}
	if len(nativeFunded) != 2 {
		t.Errorf("expected 2 funded native, got %d", len(nativeFunded))
	}
}

func TestGetBalanceSummary(t *testing.T) {
	database := setupTestDB(t)

	seedAddresses(t, database, models.ChainBSC, 3)

	database.UpsertBalance(models.ChainBSC, 0, models.TokenNative, "1000")
	database.UpsertBalance(models.ChainBSC, 1, models.TokenNative, "2000")
	database.UpsertBalance(models.ChainBSC, 2, models.TokenNative, "0")
	database.UpsertBalance(models.ChainBSC, 0, models.TokenUSDC, "5000000")

	summary, err := database.GetBalanceSummary(models.ChainBSC)
	if err != nil {
		t.Fatalf("GetBalanceSummary() error = %v", err)
	}

	if summary.Chain != models.ChainBSC {
		t.Errorf("expected chain BSC, got %s", summary.Chain)
	}

	nativeTokenSummary, ok := summary.Tokens[models.TokenNative]
	if !ok {
		t.Fatal("expected NATIVE token in summary")
	}
	if nativeTokenSummary.FundedCount != 2 {
		t.Errorf("expected 2 funded native, got %d", nativeTokenSummary.FundedCount)
	}

	usdcSummary, ok := summary.Tokens[models.TokenUSDC]
	if !ok {
		t.Fatal("expected USDC token in summary")
	}
	if usdcSummary.FundedCount != 1 {
		t.Errorf("expected 1 funded USDC, got %d", usdcSummary.FundedCount)
	}

	// Total funded = 2 (native) + 1 (USDC) = 3.
	if summary.FundedCount != 3 {
		t.Errorf("expected total funded 3, got %d", summary.FundedCount)
	}
}

func TestGetBalanceSummary_Empty(t *testing.T) {
	database := setupTestDB(t)

	summary, err := database.GetBalanceSummary(models.ChainBTC)
	if err != nil {
		t.Fatalf("GetBalanceSummary() error = %v", err)
	}
	if summary.FundedCount != 0 {
		t.Errorf("expected 0 funded, got %d", summary.FundedCount)
	}
	if len(summary.Tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(summary.Tokens))
	}
}

func TestGetAddressesBatch(t *testing.T) {
	database := setupTestDB(t)

	seedAddresses(t, database, models.ChainBTC, 5)

	// Fetch indices 1, 2, 3 (start=1, count=3).
	addresses, err := database.GetAddressesBatch(models.ChainBTC, 1, 3)
	if err != nil {
		t.Fatalf("GetAddressesBatch() error = %v", err)
	}

	if len(addresses) != 3 {
		t.Fatalf("expected 3 addresses, got %d", len(addresses))
	}

	for i, addr := range addresses {
		expectedIndex := i + 1
		if addr.AddressIndex != expectedIndex {
			t.Errorf("address %d: expected index %d, got %d", i, expectedIndex, addr.AddressIndex)
		}
	}
}

func TestGetAddressesBatch_OutOfRange(t *testing.T) {
	database := setupTestDB(t)

	seedAddresses(t, database, models.ChainBTC, 3)

	// Requesting indices beyond what exists.
	addresses, err := database.GetAddressesBatch(models.ChainBTC, 10, 5)
	if err != nil {
		t.Fatalf("GetAddressesBatch() error = %v", err)
	}

	if len(addresses) != 0 {
		t.Errorf("expected 0 addresses, got %d", len(addresses))
	}
}

func TestGetScanTimesByChain_Empty(t *testing.T) {
	database := setupTestDB(t)

	result, err := database.GetScanTimesByChain()
	if err != nil {
		t.Fatalf("GetScanTimesByChain() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries for empty DB, got %d", len(result))
	}
}

func TestGetScanTimesByChain_WithData(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)

	// Insert scan states for BTC and BSC.
	if err := database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBTC,
		LastScannedIndex: 100,
		MaxScanID:        5000,
		Status:           "completed",
		StartedAt:        now,
	}); err != nil {
		t.Fatalf("UpsertScanState(BTC) error = %v", err)
	}

	if err := database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBSC,
		LastScannedIndex: 50,
		MaxScanID:        5000,
		Status:           "completed",
		StartedAt:        now,
	}); err != nil {
		t.Fatalf("UpsertScanState(BSC) error = %v", err)
	}

	result, err := database.GetScanTimesByChain()
	if err != nil {
		t.Fatalf("GetScanTimesByChain() error = %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}

	if _, ok := result["BTC"]; !ok {
		t.Error("expected BTC entry in scan times")
	}
	if _, ok := result["BSC"]; !ok {
		t.Error("expected BSC entry in scan times")
	}
	// SOL should not be present (no scan state).
	if _, ok := result["SOL"]; ok {
		t.Error("SOL should not be present (no scan state)")
	}
}
