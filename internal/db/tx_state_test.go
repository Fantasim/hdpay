package db

import (
	"testing"

	"github.com/Fantasim/hdpay/internal/config"
)

func TestCreateTxState(t *testing.T) {
	d := setupTestDB(t)

	tx := TxStateRow{
		ID:           "tx-001",
		SweepID:      "sweep-abc",
		Chain:        "BTC",
		Token:        "NATIVE",
		AddressIndex: 0,
		FromAddress:  "addr0",
		ToAddress:    "dest0",
		Amount:       "100000",
		Status:       config.TxStatePending,
	}

	if err := d.CreateTxState(tx); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	// Retrieve via sweep
	states, err := d.GetTxStatesBySweepID("sweep-abc")
	if err != nil {
		t.Fatalf("GetTxStatesBySweepID() error = %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
	if states[0].ID != "tx-001" {
		t.Errorf("expected ID tx-001, got %s", states[0].ID)
	}
	if states[0].Status != config.TxStatePending {
		t.Errorf("expected status pending, got %s", states[0].Status)
	}
	if states[0].Amount != "100000" {
		t.Errorf("expected amount 100000, got %s", states[0].Amount)
	}
}

func TestUpdateTxStatus(t *testing.T) {
	d := setupTestDB(t)

	tx := TxStateRow{
		ID:           "tx-002",
		SweepID:      "sweep-def",
		Chain:        "BSC",
		Token:        "NATIVE",
		AddressIndex: 1,
		FromAddress:  "addr1",
		ToAddress:    "dest1",
		Amount:       "500",
		Status:       config.TxStatePending,
	}
	if err := d.CreateTxState(tx); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	// Transition to broadcasting
	if err := d.UpdateTxStatus("tx-002", config.TxStateBroadcasting, "", ""); err != nil {
		t.Fatalf("UpdateTxStatus() error = %v", err)
	}

	states, _ := d.GetTxStatesBySweepID("sweep-def")
	if states[0].Status != config.TxStateBroadcasting {
		t.Errorf("expected broadcasting, got %s", states[0].Status)
	}

	// Transition to failed with error
	if err := d.UpdateTxStatus("tx-002", config.TxStateFailed, "", "insufficient gas"); err != nil {
		t.Fatalf("UpdateTxStatus() error = %v", err)
	}

	states, _ = d.GetTxStatesBySweepID("sweep-def")
	if states[0].Status != config.TxStateFailed {
		t.Errorf("expected failed, got %s", states[0].Status)
	}
	if states[0].Error != "insufficient gas" {
		t.Errorf("expected error 'insufficient gas', got %q", states[0].Error)
	}
}

func TestUpdateTxStatusWithHash(t *testing.T) {
	d := setupTestDB(t)

	tx := TxStateRow{
		ID:           "tx-003",
		SweepID:      "sweep-ghi",
		Chain:        "SOL",
		Token:        "NATIVE",
		AddressIndex: 2,
		FromAddress:  "addr2",
		ToAddress:    "dest2",
		Amount:       "9000000000",
		Status:       config.TxStatePending,
	}
	if err := d.CreateTxState(tx); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	// Set tx hash when transitioning to confirming
	if err := d.UpdateTxStatus("tx-003", config.TxStateConfirming, "0xabc123", ""); err != nil {
		t.Fatalf("UpdateTxStatus() error = %v", err)
	}

	states, _ := d.GetTxStatesBySweepID("sweep-ghi")
	if states[0].TxHash != "0xabc123" {
		t.Errorf("expected txHash 0xabc123, got %s", states[0].TxHash)
	}
	if states[0].Status != config.TxStateConfirming {
		t.Errorf("expected confirming, got %s", states[0].Status)
	}
}

func TestGetPendingTxStates(t *testing.T) {
	d := setupTestDB(t)

	// Insert mix of statuses
	statuses := []struct {
		id     string
		chain  string
		status string
	}{
		{"tx-a", "BTC", config.TxStatePending},
		{"tx-b", "BTC", config.TxStateBroadcasting},
		{"tx-c", "BTC", config.TxStateConfirmed},
		{"tx-d", "BTC", config.TxStateUncertain},
		{"tx-e", "BTC", config.TxStateFailed},
		{"tx-f", "BSC", config.TxStatePending},
	}

	for _, s := range statuses {
		tx := TxStateRow{
			ID:          s.id,
			SweepID:     "sweep-mix",
			Chain:       s.chain,
			Token:       "NATIVE",
			FromAddress: "from",
			ToAddress:   "to",
			Amount:      "100",
			Status:      s.status,
		}
		if err := d.CreateTxState(tx); err != nil {
			t.Fatalf("CreateTxState(%s) error = %v", s.id, err)
		}
	}

	// Should return pending, broadcasting, uncertain for BTC (not confirmed, not failed)
	pending, err := d.GetPendingTxStates("BTC")
	if err != nil {
		t.Fatalf("GetPendingTxStates() error = %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending states for BTC, got %d", len(pending))
	}

	// BSC should return 1
	pendingBSC, err := d.GetPendingTxStates("BSC")
	if err != nil {
		t.Fatalf("GetPendingTxStates(BSC) error = %v", err)
	}
	if len(pendingBSC) != 1 {
		t.Fatalf("expected 1 pending state for BSC, got %d", len(pendingBSC))
	}
}

func TestGetTxStatesBySweepID(t *testing.T) {
	d := setupTestDB(t)

	for i := 0; i < 3; i++ {
		tx := TxStateRow{
			ID:           "tx-s" + itoa(i),
			SweepID:      "sweep-123",
			Chain:        "BTC",
			Token:        "NATIVE",
			AddressIndex: i,
			FromAddress:  "from" + itoa(i),
			ToAddress:    "dest",
			Amount:       "500",
			Status:       config.TxStatePending,
		}
		if err := d.CreateTxState(tx); err != nil {
			t.Fatalf("CreateTxState() error = %v", err)
		}
	}

	states, err := d.GetTxStatesBySweepID("sweep-123")
	if err != nil {
		t.Fatalf("GetTxStatesBySweepID() error = %v", err)
	}
	if len(states) != 3 {
		t.Errorf("expected 3 states, got %d", len(states))
	}

	// Verify ordered by address_index
	for i, s := range states {
		if s.AddressIndex != i {
			t.Errorf("expected index %d, got %d", i, s.AddressIndex)
		}
	}
}

func TestGetTxStateByNonce(t *testing.T) {
	d := setupTestDB(t)

	tx := TxStateRow{
		ID:          "tx-nonce",
		SweepID:     "sweep-n",
		Chain:       "BSC",
		Token:       "NATIVE",
		FromAddress: "0xabc",
		ToAddress:   "0xdef",
		Amount:      "1000",
		Nonce:       42,
		Status:      config.TxStateBroadcasting,
	}
	if err := d.CreateTxState(tx); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	// Found
	found, err := d.GetTxStateByNonce("BSC", "0xabc", 42)
	if err != nil {
		t.Fatalf("GetTxStateByNonce() error = %v", err)
	}
	if found == nil {
		t.Fatal("expected to find tx state, got nil")
	}
	if found.ID != "tx-nonce" {
		t.Errorf("expected ID tx-nonce, got %s", found.ID)
	}

	// Not found
	notFound, err := d.GetTxStateByNonce("BSC", "0xabc", 99)
	if err != nil {
		t.Fatalf("GetTxStateByNonce() error = %v", err)
	}
	if notFound != nil {
		t.Errorf("expected nil, got %+v", notFound)
	}
}

func TestCountTxStatesByStatus(t *testing.T) {
	d := setupTestDB(t)

	statuses := []string{
		config.TxStatePending,
		config.TxStatePending,
		config.TxStateBroadcasting,
		config.TxStateConfirmed,
		config.TxStateFailed,
	}

	for i, status := range statuses {
		tx := TxStateRow{
			ID:          "tx-c" + itoa(i),
			SweepID:     "sweep-count",
			Chain:       "SOL",
			Token:       "NATIVE",
			FromAddress: "from",
			ToAddress:   "to",
			Amount:      "100",
			Status:      status,
		}
		if err := d.CreateTxState(tx); err != nil {
			t.Fatalf("CreateTxState() error = %v", err)
		}
	}

	counts, err := d.CountTxStatesByStatus("sweep-count")
	if err != nil {
		t.Fatalf("CountTxStatesByStatus() error = %v", err)
	}

	if counts[config.TxStatePending] != 2 {
		t.Errorf("expected 2 pending, got %d", counts[config.TxStatePending])
	}
	if counts[config.TxStateBroadcasting] != 1 {
		t.Errorf("expected 1 broadcasting, got %d", counts[config.TxStateBroadcasting])
	}
	if counts[config.TxStateConfirmed] != 1 {
		t.Errorf("expected 1 confirmed, got %d", counts[config.TxStateConfirmed])
	}
	if counts[config.TxStateFailed] != 1 {
		t.Errorf("expected 1 failed, got %d", counts[config.TxStateFailed])
	}
}

func TestTxStateNotFound(t *testing.T) {
	d := setupTestDB(t)

	// Empty sweep
	states, err := d.GetTxStatesBySweepID("nonexistent")
	if err != nil {
		t.Fatalf("GetTxStatesBySweepID() error = %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}

	// Empty nonce lookup
	found, err := d.GetTxStateByNonce("BTC", "unknown", 0)
	if err != nil {
		t.Fatalf("GetTxStateByNonce() error = %v", err)
	}
	if found != nil {
		t.Errorf("expected nil, got %+v", found)
	}
}
