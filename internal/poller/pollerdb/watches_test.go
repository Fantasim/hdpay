package pollerdb

import (
	"testing"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

func TestCreateWatch_And_GetWatch(t *testing.T) {
	db := newTestDB(t)

	w := &models.Watch{
		ID:        "watch-001",
		Chain:     "BTC",
		Address:   "bc1qtest",
		Status:    models.WatchStatusActive,
		StartedAt: "2026-01-01T00:00:00Z",
		ExpiresAt: "2026-01-01T00:30:00Z",
	}

	if err := db.CreateWatch(w); err != nil {
		t.Fatalf("CreateWatch() error = %v", err)
	}

	got, err := db.GetWatch("watch-001")
	if err != nil {
		t.Fatalf("GetWatch() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetWatch() returned nil")
	}
	if got.Chain != "BTC" {
		t.Errorf("Chain = %q, want BTC", got.Chain)
	}
	if got.Address != "bc1qtest" {
		t.Errorf("Address = %q, want bc1qtest", got.Address)
	}
	if got.Status != models.WatchStatusActive {
		t.Errorf("Status = %q, want ACTIVE", got.Status)
	}
	if got.PollCount != 0 {
		t.Errorf("PollCount = %d, want 0", got.PollCount)
	}
}

func TestGetWatch_NotFound(t *testing.T) {
	db := newTestDB(t)

	got, err := db.GetWatch("nonexistent")
	if err != nil {
		t.Fatalf("GetWatch() error = %v", err)
	}
	if got != nil {
		t.Error("GetWatch() should return nil for nonexistent ID")
	}
}

func TestListWatches_Filters(t *testing.T) {
	db := newTestDB(t)

	watches := []models.Watch{
		{ID: "w1", Chain: "BTC", Address: "addr1", Status: models.WatchStatusActive, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02"},
		{ID: "w2", Chain: "BSC", Address: "addr2", Status: models.WatchStatusCompleted, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02"},
		{ID: "w3", Chain: "BTC", Address: "addr3", Status: models.WatchStatusExpired, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02"},
	}
	for _, w := range watches {
		ww := w
		if err := db.CreateWatch(&ww); err != nil {
			t.Fatalf("CreateWatch() error = %v", err)
		}
	}

	// No filter — get all.
	all, err := db.ListWatches(models.WatchFilters{})
	if err != nil {
		t.Fatalf("ListWatches() error = %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListWatches(all) len = %d, want 3", len(all))
	}

	// Filter by status.
	active := models.WatchStatusActive
	filtered, err := db.ListWatches(models.WatchFilters{Status: &active})
	if err != nil {
		t.Fatalf("ListWatches(status=ACTIVE) error = %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("ListWatches(status=ACTIVE) len = %d, want 1", len(filtered))
	}

	// Filter by chain.
	chain := "BTC"
	byChain, err := db.ListWatches(models.WatchFilters{Chain: &chain})
	if err != nil {
		t.Fatalf("ListWatches(chain=BTC) error = %v", err)
	}
	if len(byChain) != 2 {
		t.Errorf("ListWatches(chain=BTC) len = %d, want 2", len(byChain))
	}
}

func TestUpdateWatchStatus(t *testing.T) {
	db := newTestDB(t)

	w := &models.Watch{
		ID: "w1", Chain: "SOL", Address: "sol1",
		Status: models.WatchStatusActive, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02",
	}
	db.CreateWatch(w)

	completedAt := "2026-01-01T00:15:00Z"
	if err := db.UpdateWatchStatus("w1", models.WatchStatusCompleted, &completedAt); err != nil {
		t.Fatalf("UpdateWatchStatus() error = %v", err)
	}

	got, _ := db.GetWatch("w1")
	if got.Status != models.WatchStatusCompleted {
		t.Errorf("Status = %q, want COMPLETED", got.Status)
	}
	if got.CompletedAt == nil || *got.CompletedAt != completedAt {
		t.Errorf("CompletedAt = %v, want %q", got.CompletedAt, completedAt)
	}
}

func TestUpdateWatchPollResult(t *testing.T) {
	db := newTestDB(t)

	w := &models.Watch{
		ID: "w1", Chain: "BTC", Address: "addr1",
		Status: models.WatchStatusActive, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02",
	}
	db.CreateWatch(w)

	if err := db.UpdateWatchPollResult("w1", 5, `{"new_txs":1}`); err != nil {
		t.Fatalf("UpdateWatchPollResult() error = %v", err)
	}

	got, _ := db.GetWatch("w1")
	if got.PollCount != 5 {
		t.Errorf("PollCount = %d, want 5", got.PollCount)
	}
	if got.LastPollResult == nil || *got.LastPollResult != `{"new_txs":1}` {
		t.Errorf("LastPollResult = %v, want {\"new_txs\":1}", got.LastPollResult)
	}
	if got.LastPollAt == nil {
		t.Error("LastPollAt should not be nil after poll")
	}
}

func TestExpireAllActiveWatches(t *testing.T) {
	db := newTestDB(t)

	for _, w := range []models.Watch{
		{ID: "w1", Chain: "BTC", Address: "a1", Status: models.WatchStatusActive, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02"},
		{ID: "w2", Chain: "BSC", Address: "a2", Status: models.WatchStatusActive, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02"},
		{ID: "w3", Chain: "SOL", Address: "a3", Status: models.WatchStatusCompleted, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02"},
	} {
		ww := w
		db.CreateWatch(&ww)
	}

	count, err := db.ExpireAllActiveWatches()
	if err != nil {
		t.Fatalf("ExpireAllActiveWatches() error = %v", err)
	}
	if count != 2 {
		t.Errorf("expired count = %d, want 2", count)
	}

	// Verify states.
	w1, _ := db.GetWatch("w1")
	if w1.Status != models.WatchStatusExpired {
		t.Errorf("w1 status = %q, want EXPIRED", w1.Status)
	}
	w3, _ := db.GetWatch("w3")
	if w3.Status != models.WatchStatusCompleted {
		t.Errorf("w3 status = %q, should remain COMPLETED", w3.Status)
	}
}

func TestGetActiveWatchByAddress(t *testing.T) {
	db := newTestDB(t)

	db.CreateWatch(&models.Watch{
		ID: "w1", Chain: "BTC", Address: "addr1",
		Status: models.WatchStatusActive, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02",
	})
	db.CreateWatch(&models.Watch{
		ID: "w2", Chain: "BTC", Address: "addr2",
		Status: models.WatchStatusExpired, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02",
	})

	// Active watch exists.
	got, err := db.GetActiveWatchByAddress("addr1")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if got == nil || got.ID != "w1" {
		t.Errorf("expected w1, got %v", got)
	}

	// No active watch.
	got, err = db.GetActiveWatchByAddress("addr2")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for expired watch, got %v", got)
	}

	// Unknown address.
	got, err = db.GetActiveWatchByAddress("unknown")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if got != nil {
		t.Error("expected nil for unknown address")
	}
}
