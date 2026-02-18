package db

import (
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/models"
)

func TestGetScanState_NotFound(t *testing.T) {
	database := setupTestDB(t)

	state, err := database.GetScanState(models.ChainBTC)
	if err != nil {
		t.Fatalf("GetScanState() error = %v", err)
	}
	if state != nil {
		t.Errorf("expected nil state, got %+v", state)
	}
}

func TestUpsertScanState_InsertAndGet(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)
	err := database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBTC,
		LastScannedIndex: 500,
		MaxScanID:        5000,
		Status:           ScanStatusScanning,
		StartedAt:        now,
	})
	if err != nil {
		t.Fatalf("UpsertScanState() error = %v", err)
	}

	state, err := database.GetScanState(models.ChainBTC)
	if err != nil {
		t.Fatalf("GetScanState() error = %v", err)
	}
	if state == nil {
		t.Fatal("expected state, got nil")
	}
	if state.LastScannedIndex != 500 {
		t.Errorf("expected lastScannedIndex 500, got %d", state.LastScannedIndex)
	}
	if state.MaxScanID != 5000 {
		t.Errorf("expected maxScanID 5000, got %d", state.MaxScanID)
	}
	if state.Status != ScanStatusScanning {
		t.Errorf("expected status scanning, got %s", state.Status)
	}
}

func TestUpsertScanState_Update(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)
	database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBSC,
		LastScannedIndex: 100,
		MaxScanID:        1000,
		Status:           ScanStatusScanning,
		StartedAt:        now,
	})

	// Update progress.
	database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBSC,
		LastScannedIndex: 500,
		MaxScanID:        1000,
		Status:           ScanStatusScanning,
	})

	state, err := database.GetScanState(models.ChainBSC)
	if err != nil {
		t.Fatalf("GetScanState() error = %v", err)
	}
	if state.LastScannedIndex != 500 {
		t.Errorf("expected lastScannedIndex 500, got %d", state.LastScannedIndex)
	}
	// StartedAt should be preserved from first insert.
	if state.StartedAt != now {
		t.Errorf("expected startedAt preserved as %s, got %s", now, state.StartedAt)
	}
}

func TestShouldResume_NoState(t *testing.T) {
	database := setupTestDB(t)

	shouldResume, idx, err := database.ShouldResume(models.ChainBTC)
	if err != nil {
		t.Fatalf("ShouldResume() error = %v", err)
	}
	if shouldResume {
		t.Error("expected no resume with no state")
	}
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}
}

func TestShouldResume_RecentScanning(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)
	database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBTC,
		LastScannedIndex: 250,
		MaxScanID:        1000,
		Status:           ScanStatusScanning,
		StartedAt:        now,
	})

	shouldResume, idx, err := database.ShouldResume(models.ChainBTC)
	if err != nil {
		t.Fatalf("ShouldResume() error = %v", err)
	}
	if !shouldResume {
		t.Error("expected resume for recent scanning state")
	}
	if idx != 250 {
		t.Errorf("expected resume index 250, got %d", idx)
	}
}

func TestShouldResume_FailedState(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)
	database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBTC,
		LastScannedIndex: 100,
		MaxScanID:        1000,
		Status:           ScanStatusFailed,
		StartedAt:        now,
	})

	shouldResume, _, err := database.ShouldResume(models.ChainBTC)
	if err != nil {
		t.Fatalf("ShouldResume() error = %v", err)
	}
	if shouldResume {
		t.Error("expected no resume for failed state")
	}
}

func TestShouldResume_CompletedRecently(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)
	database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBTC,
		LastScannedIndex: 1000,
		MaxScanID:        1000,
		Status:           ScanStatusCompleted,
		StartedAt:        now,
	})

	shouldResume, _, err := database.ShouldResume(models.ChainBTC)
	if err != nil {
		t.Fatalf("ShouldResume() error = %v", err)
	}
	if shouldResume {
		t.Error("expected no resume for completed scan — should start fresh")
	}
}

func TestShouldResume_MultipleChains(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)

	// BTC scanning.
	database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBTC,
		LastScannedIndex: 250,
		MaxScanID:        1000,
		Status:           ScanStatusScanning,
		StartedAt:        now,
	})

	// BSC completed.
	database.UpsertScanState(models.ScanState{
		Chain:            models.ChainBSC,
		LastScannedIndex: 500,
		MaxScanID:        500,
		Status:           ScanStatusCompleted,
		StartedAt:        now,
	})

	btcResume, btcIdx, _ := database.ShouldResume(models.ChainBTC)
	bscResume, _, _ := database.ShouldResume(models.ChainBSC)

	if !btcResume {
		t.Error("expected BTC to be resumable")
	}
	if btcIdx != 250 {
		t.Errorf("expected BTC resume from 250, got %d", btcIdx)
	}
	if bscResume {
		t.Error("expected BSC not resumable (completed)")
	}
}
