package scanner

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// setupTestDB creates a temporary database for testing.
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()

	tmpFile := t.TempDir() + "/test.sqlite"
	database, err := db.New(tmpFile)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
		os.Remove(tmpFile)
	})

	return database
}

// seedAddresses inserts test addresses into the database.
func seedAddresses(t *testing.T, database *db.DB, chain models.Chain, count int) {
	t.Helper()
	addrs := make([]models.Address, count)
	for i := 0; i < count; i++ {
		addrs[i] = models.Address{
			Chain:        chain,
			AddressIndex: i,
			Address:      fmt.Sprintf("addr_%s_%d", chain, i),
		}
	}
	if err := database.InsertAddressBatch(chain, addrs); err != nil {
		t.Fatalf("failed to insert addresses: %v", err)
	}
}

func TestScanner_ScanCompletesSuccessfully(t *testing.T) {
	database := setupTestDB(t)
	hub := NewSSEHub()

	seedAddresses(t, database, models.ChainBTC, 5)

	provider := &mockProvider{
		name:      "TestProvider",
		chain:     models.ChainBTC,
		batchSize: 2,
	}

	pool := NewPool(models.ChainBTC, provider)
	scanner := SetupScannerForTest(database, hub, map[models.Chain]*Pool{
		models.ChainBTC: pool,
	})

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	err := scanner.StartScan(context.Background(), models.ChainBTC, 5)
	if err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}

	// Wait for scan to complete.
	deadline := time.After(5 * time.Second)
	completed := false
	for !completed {
		select {
		case event := <-ch:
			if event.Type == "scan_complete" {
				completed = true
			}
		case <-deadline:
			t.Fatal("scan did not complete within timeout")
		}
	}

	// Verify scan state.
	state := scanner.Status(models.ChainBTC)
	if state == nil {
		t.Fatal("expected scan state, got nil")
	}
	if state.Status != db.ScanStatusCompleted {
		t.Errorf("expected status completed, got %s", state.Status)
	}
	if state.LastScannedIndex != 5 {
		t.Errorf("expected last scanned 5, got %d", state.LastScannedIndex)
	}
}

func TestScanner_ScanCancellation(t *testing.T) {
	database := setupTestDB(t)
	hub := NewSSEHub()

	// Seed many addresses so scan takes a while.
	seedAddresses(t, database, models.ChainBTC, 100)

	callCount := 0
	provider := &mockProvider{
		name:      "SlowProvider",
		chain:     models.ChainBTC,
		batchSize: 1,
		nativeFunc: func(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
			callCount++
			// Simulate some work.
			time.Sleep(10 * time.Millisecond)
			return []BalanceResult{{
				Address:      addresses[0].Address,
				AddressIndex: addresses[0].AddressIndex,
				Balance:      "0",
			}}, nil
		},
	}

	pool := NewPool(models.ChainBTC, provider)
	scanner := SetupScannerForTest(database, hub, map[models.Chain]*Pool{
		models.ChainBTC: pool,
	})

	err := scanner.StartScan(context.Background(), models.ChainBTC, 100)
	if err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}

	// Let a few batches run, then cancel.
	time.Sleep(50 * time.Millisecond)
	scanner.StopScan(models.ChainBTC)

	// Wait for goroutine to finish.
	time.Sleep(100 * time.Millisecond)

	if scanner.IsRunning(models.ChainBTC) {
		t.Error("expected scan to not be running after stop")
	}

	// Should have scanned fewer than 100 addresses.
	if callCount >= 100 {
		t.Errorf("expected scan to stop early, but provider called %d times", callCount)
	}
}

func TestScanner_DuplicateScanRejected(t *testing.T) {
	database := setupTestDB(t)
	hub := NewSSEHub()

	seedAddresses(t, database, models.ChainBTC, 10)

	provider := &mockProvider{
		name:      "SlowProvider",
		chain:     models.ChainBTC,
		batchSize: 1,
		nativeFunc: func(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
			time.Sleep(100 * time.Millisecond)
			return []BalanceResult{{Balance: "0", Address: addresses[0].Address, AddressIndex: addresses[0].AddressIndex}}, nil
		},
	}

	pool := NewPool(models.ChainBTC, provider)
	scanner := SetupScannerForTest(database, hub, map[models.Chain]*Pool{
		models.ChainBTC: pool,
	})

	err := scanner.StartScan(context.Background(), models.ChainBTC, 10)
	if err != nil {
		t.Fatalf("first StartScan() error = %v", err)
	}

	// Second scan should fail.
	err = scanner.StartScan(context.Background(), models.ChainBTC, 10)
	if err != config.ErrScanAlreadyRunning {
		t.Errorf("expected ErrScanAlreadyRunning, got %v", err)
	}

	scanner.StopScan(models.ChainBTC)
	time.Sleep(200 * time.Millisecond) // Wait for cleanup.
}

func TestScanner_NoPoolRegistered(t *testing.T) {
	database := setupTestDB(t)
	hub := NewSSEHub()

	scanner := SetupScannerForTest(database, hub, map[models.Chain]*Pool{})

	err := scanner.StartScan(context.Background(), models.ChainBTC, 10)
	if err == nil {
		t.Fatal("expected error for missing pool")
	}
}

func TestScanner_IsRunning(t *testing.T) {
	database := setupTestDB(t)
	hub := NewSSEHub()

	seedAddresses(t, database, models.ChainBTC, 5)

	provider := &mockProvider{
		name:      "TestProvider",
		chain:     models.ChainBTC,
		batchSize: 1,
		nativeFunc: func(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
			time.Sleep(50 * time.Millisecond)
			return []BalanceResult{{Balance: "0", Address: addresses[0].Address, AddressIndex: addresses[0].AddressIndex}}, nil
		},
	}

	pool := NewPool(models.ChainBTC, provider)
	scanner := SetupScannerForTest(database, hub, map[models.Chain]*Pool{
		models.ChainBTC: pool,
	})

	if scanner.IsRunning(models.ChainBTC) {
		t.Error("expected not running before start")
	}

	scanner.StartScan(context.Background(), models.ChainBTC, 5)

	if !scanner.IsRunning(models.ChainBTC) {
		t.Error("expected running after start")
	}

	scanner.StopScan(models.ChainBTC)
	time.Sleep(200 * time.Millisecond)

	if scanner.IsRunning(models.ChainBTC) {
		t.Error("expected not running after stop")
	}
}

func TestScanner_BalancesStoredInDB(t *testing.T) {
	database := setupTestDB(t)
	hub := NewSSEHub()

	seedAddresses(t, database, models.ChainBTC, 3)

	provider := &mockProvider{
		name:      "TestProvider",
		chain:     models.ChainBTC,
		batchSize: 10,
		nativeFunc: func(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
			results := make([]BalanceResult, len(addresses))
			for i, a := range addresses {
				bal := "0"
				if a.AddressIndex == 1 {
					bal = "50000"
				}
				results[i] = BalanceResult{
					Address:      a.Address,
					AddressIndex: a.AddressIndex,
					Balance:      bal,
				}
			}
			return results, nil
		},
	}

	pool := NewPool(models.ChainBTC, provider)
	scanner := SetupScannerForTest(database, hub, map[models.Chain]*Pool{
		models.ChainBTC: pool,
	})

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	scanner.StartScan(context.Background(), models.ChainBTC, 3)

	// Wait for completion.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case event := <-ch:
			if event.Type == "scan_complete" {
				goto done
			}
		case <-deadline:
			t.Fatal("scan did not complete within timeout")
		}
	}
done:

	// Verify funded addresses in DB.
	funded, err := database.GetFundedAddresses(models.ChainBTC, models.TokenNative)
	if err != nil {
		t.Fatalf("GetFundedAddresses() error = %v", err)
	}

	if len(funded) != 1 {
		t.Fatalf("expected 1 funded address, got %d", len(funded))
	}

	if funded[0].AddressIndex != 1 {
		t.Errorf("expected funded address at index 1, got %d", funded[0].AddressIndex)
	}

	if funded[0].Balance != "50000" {
		t.Errorf("expected balance 50000, got %s", funded[0].Balance)
	}
}
