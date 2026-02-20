package pollerdb

import (
	"testing"
)

func TestGetOrCreatePoints(t *testing.T) {
	db := newTestDB(t)

	p, err := db.GetOrCreatePoints("addr1", "BTC")
	if err != nil {
		t.Fatalf("GetOrCreatePoints() error = %v", err)
	}
	if p.Address != "addr1" || p.Chain != "BTC" {
		t.Errorf("got %s/%s, want addr1/BTC", p.Address, p.Chain)
	}
	if p.Unclaimed != 0 || p.Pending != 0 || p.Total != 0 {
		t.Errorf("new account should have all zeros, got %d/%d/%d", p.Unclaimed, p.Pending, p.Total)
	}

	// Calling again should return same row, not error.
	p2, err := db.GetOrCreatePoints("addr1", "BTC")
	if err != nil {
		t.Fatalf("GetOrCreatePoints() second call error = %v", err)
	}
	if p2.Address != "addr1" {
		t.Error("second call should return same account")
	}
}

func TestAddUnclaimed(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "BTC")

	if err := db.AddUnclaimed("addr1", "BTC", 1000); err != nil {
		t.Fatalf("AddUnclaimed() error = %v", err)
	}

	p, _ := db.GetOrCreatePoints("addr1", "BTC")
	if p.Unclaimed != 1000 {
		t.Errorf("Unclaimed = %d, want 1000", p.Unclaimed)
	}
	if p.Total != 1000 {
		t.Errorf("Total = %d, want 1000", p.Total)
	}

	// Add more.
	db.AddUnclaimed("addr1", "BTC", 500)
	p, _ = db.GetOrCreatePoints("addr1", "BTC")
	if p.Unclaimed != 1500 {
		t.Errorf("Unclaimed = %d, want 1500", p.Unclaimed)
	}
	if p.Total != 1500 {
		t.Errorf("Total = %d, want 1500", p.Total)
	}
}

func TestAddPending(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "SOL")

	if err := db.AddPending("addr1", "SOL", 200); err != nil {
		t.Fatalf("AddPending() error = %v", err)
	}

	p, _ := db.GetOrCreatePoints("addr1", "SOL")
	if p.Pending != 200 {
		t.Errorf("Pending = %d, want 200", p.Pending)
	}
	if p.Total != 0 {
		t.Errorf("Total = %d, want 0 (pending doesn't affect total)", p.Total)
	}
}

func TestMovePendingToUnclaimed(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "BSC")

	db.AddPending("addr1", "BSC", 300)

	if err := db.MovePendingToUnclaimed("addr1", "BSC", 300, 350); err != nil {
		t.Fatalf("MovePendingToUnclaimed() error = %v", err)
	}

	p, _ := db.GetOrCreatePoints("addr1", "BSC")
	if p.Pending != 0 {
		t.Errorf("Pending = %d, want 0", p.Pending)
	}
	if p.Unclaimed != 350 {
		t.Errorf("Unclaimed = %d, want 350", p.Unclaimed)
	}
	if p.Total != 350 {
		t.Errorf("Total = %d, want 350", p.Total)
	}
}

func TestClaimPoints(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "BTC")
	db.AddUnclaimed("addr1", "BTC", 5000)

	claimed, err := db.ClaimPoints("addr1", "BTC")
	if err != nil {
		t.Fatalf("ClaimPoints() error = %v", err)
	}
	if claimed != 5000 {
		t.Errorf("claimed = %d, want 5000", claimed)
	}

	p, _ := db.GetOrCreatePoints("addr1", "BTC")
	if p.Unclaimed != 0 {
		t.Errorf("Unclaimed after claim = %d, want 0", p.Unclaimed)
	}
	if p.Total != 5000 {
		t.Errorf("Total after claim = %d, want 5000 (total never decreases)", p.Total)
	}
}

func TestClaimPoints_WhilePendingExists(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "BTC")
	db.AddUnclaimed("addr1", "BTC", 3000)
	db.AddPending("addr1", "BTC", 1000)

	claimed, _ := db.ClaimPoints("addr1", "BTC")
	if claimed != 3000 {
		t.Errorf("claimed = %d, want 3000 (only unclaimed)", claimed)
	}

	p, _ := db.GetOrCreatePoints("addr1", "BTC")
	if p.Unclaimed != 0 {
		t.Errorf("Unclaimed = %d, want 0", p.Unclaimed)
	}
	if p.Pending != 1000 {
		t.Errorf("Pending = %d, want 1000 (untouched)", p.Pending)
	}
}

func TestClaimPoints_NonexistentAddress(t *testing.T) {
	db := newTestDB(t)

	claimed, err := db.ClaimPoints("unknown", "BTC")
	if err != nil {
		t.Fatalf("ClaimPoints() error = %v (should skip silently)", err)
	}
	if claimed != 0 {
		t.Errorf("claimed = %d, want 0", claimed)
	}
}

func TestClaimPoints_ZeroUnclaimed(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "BTC")

	claimed, err := db.ClaimPoints("addr1", "BTC")
	if err != nil {
		t.Fatalf("ClaimPoints() error = %v", err)
	}
	if claimed != 0 {
		t.Errorf("claimed = %d, want 0", claimed)
	}
}

func TestClaimPoints_NewFundsAfterClaim(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "BTC")
	db.AddUnclaimed("addr1", "BTC", 1000)

	db.ClaimPoints("addr1", "BTC")

	// New funds arrive.
	db.AddUnclaimed("addr1", "BTC", 2000)

	p, _ := db.GetOrCreatePoints("addr1", "BTC")
	if p.Unclaimed != 2000 {
		t.Errorf("Unclaimed = %d, want 2000", p.Unclaimed)
	}
	if p.Total != 3000 {
		t.Errorf("Total = %d, want 3000 (1000 original + 2000 new)", p.Total)
	}
}

func TestListWithUnclaimed(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "BTC")
	db.GetOrCreatePoints("addr2", "BSC")
	db.GetOrCreatePoints("addr3", "SOL")

	db.AddUnclaimed("addr1", "BTC", 500)
	db.AddUnclaimed("addr3", "SOL", 300)
	// addr2 has 0 unclaimed.

	accounts, err := db.ListWithUnclaimed()
	if err != nil {
		t.Fatalf("ListWithUnclaimed() error = %v", err)
	}
	if len(accounts) != 2 {
		t.Errorf("len = %d, want 2", len(accounts))
	}
}

func TestListWithPending(t *testing.T) {
	db := newTestDB(t)
	db.GetOrCreatePoints("addr1", "BTC")
	db.GetOrCreatePoints("addr2", "BSC")

	db.AddPending("addr2", "BSC", 100)

	accounts, err := db.ListWithPending()
	if err != nil {
		t.Fatalf("ListWithPending() error = %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("len = %d, want 1", len(accounts))
	}
	if accounts[0].Address != "addr2" {
		t.Errorf("Address = %q, want addr2", accounts[0].Address)
	}
}
