package pollerdb

import (
	"testing"
)

func TestInsertError_And_ListUnresolved(t *testing.T) {
	db := newTestDB(t)

	id, err := db.InsertError("ERROR", "PROVIDER", "BTC provider down", "timeout after 30s")
	if err != nil {
		t.Fatalf("InsertError() error = %v", err)
	}
	if id == 0 {
		t.Error("InsertError() should return non-zero ID")
	}

	db.InsertError("WARN", "PRICE", "CoinGecko rate limited", "")

	unresolved, err := db.ListUnresolved()
	if err != nil {
		t.Fatalf("ListUnresolved() error = %v", err)
	}
	if len(unresolved) != 2 {
		t.Errorf("len = %d, want 2", len(unresolved))
	}

	// Verify both errors are present.
	categories := map[string]bool{}
	for _, e := range unresolved {
		categories[e.Category] = true
	}
	if !categories["PROVIDER"] {
		t.Error("expected PROVIDER in unresolved errors")
	}
	if !categories["PRICE"] {
		t.Error("expected PRICE in unresolved errors")
	}
}

func TestMarkResolved(t *testing.T) {
	db := newTestDB(t)

	id, _ := db.InsertError("ERROR", "PROVIDER", "BSC timeout", "details")

	if err := db.MarkResolved(int(id)); err != nil {
		t.Fatalf("MarkResolved() error = %v", err)
	}

	unresolved, _ := db.ListUnresolved()
	if len(unresolved) != 0 {
		t.Errorf("len = %d, want 0 after resolving", len(unresolved))
	}
}

func TestMarkResolved_NotFound(t *testing.T) {
	db := newTestDB(t)

	err := db.MarkResolved(999)
	if err == nil {
		t.Error("MarkResolved() should error for nonexistent ID")
	}
}

func TestListByCategory(t *testing.T) {
	db := newTestDB(t)

	db.InsertError("ERROR", "PROVIDER", "error 1", "")
	db.InsertError("WARN", "PROVIDER", "error 2", "")
	db.InsertError("ERROR", "PRICE", "error 3", "")

	providers, err := db.ListByCategory("PROVIDER")
	if err != nil {
		t.Fatalf("ListByCategory() error = %v", err)
	}
	if len(providers) != 2 {
		t.Errorf("len = %d, want 2", len(providers))
	}

	prices, _ := db.ListByCategory("PRICE")
	if len(prices) != 1 {
		t.Errorf("len = %d, want 1", len(prices))
	}

	empty, _ := db.ListByCategory("NONEXISTENT")
	if len(empty) != 0 {
		t.Errorf("len = %d, want 0", len(empty))
	}
}

func TestListByCategory_IncludesResolved(t *testing.T) {
	db := newTestDB(t)

	id, _ := db.InsertError("ERROR", "PROVIDER", "resolved error", "")
	db.MarkResolved(int(id))
	db.InsertError("ERROR", "PROVIDER", "unresolved error", "")

	all, err := db.ListByCategory("PROVIDER")
	if err != nil {
		t.Fatalf("ListByCategory() error = %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len = %d, want 2 (includes resolved)", len(all))
	}
}
