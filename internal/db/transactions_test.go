package db

import (
	"testing"

	"github.com/Fantasim/hdpay/internal/models"
)

func TestInsertTransaction_AndRetrieve(t *testing.T) {
	d := setupTestDB(t)

	tx := models.Transaction{
		Chain:        models.ChainBTC,
		AddressIndex: 0,
		TxHash:       "aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122",
		Direction:    "send",
		Token:        models.TokenNative,
		Amount:       "50000",
		FromAddress:  "bc1qsender",
		ToAddress:    "bc1qreceiver",
		Status:       "pending",
	}

	id, err := d.InsertTransaction(tx)
	if err != nil {
		t.Fatalf("InsertTransaction() error = %v", err)
	}

	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	// Retrieve by ID.
	got, err := d.GetTransaction(id)
	if err != nil {
		t.Fatalf("GetTransaction() error = %v", err)
	}

	if got.Chain != models.ChainBTC {
		t.Errorf("Chain = %s, want BTC", got.Chain)
	}
	if got.TxHash != tx.TxHash {
		t.Errorf("TxHash = %s, want %s", got.TxHash, tx.TxHash)
	}
	if got.Direction != "send" {
		t.Errorf("Direction = %s, want send", got.Direction)
	}
	if got.Amount != "50000" {
		t.Errorf("Amount = %s, want 50000", got.Amount)
	}
	if got.Status != "pending" {
		t.Errorf("Status = %s, want pending", got.Status)
	}
	if got.FromAddress != "bc1qsender" {
		t.Errorf("FromAddress = %s, want bc1qsender", got.FromAddress)
	}
	if got.ToAddress != "bc1qreceiver" {
		t.Errorf("ToAddress = %s, want bc1qreceiver", got.ToAddress)
	}
}

func TestUpdateTransactionStatus(t *testing.T) {
	d := setupTestDB(t)

	tx := models.Transaction{
		Chain:        models.ChainBTC,
		AddressIndex: 0,
		TxHash:       "aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122",
		Direction:    "send",
		Token:        models.TokenNative,
		Amount:       "50000",
		FromAddress:  "bc1qsender",
		ToAddress:    "bc1qreceiver",
		Status:       "pending",
	}

	id, err := d.InsertTransaction(tx)
	if err != nil {
		t.Fatal(err)
	}

	// Update to confirmed.
	confirmedAt := "2026-02-18T12:00:00Z"
	if err := d.UpdateTransactionStatus(id, "confirmed", &confirmedAt); err != nil {
		t.Fatalf("UpdateTransactionStatus() error = %v", err)
	}

	got, err := d.GetTransaction(id)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != "confirmed" {
		t.Errorf("Status = %s, want confirmed", got.Status)
	}
	if got.ConfirmedAt != confirmedAt {
		t.Errorf("ConfirmedAt = %s, want %s", got.ConfirmedAt, confirmedAt)
	}
}

func TestUpdateTransactionStatus_NoConfirmedAt(t *testing.T) {
	d := setupTestDB(t)

	tx := models.Transaction{
		Chain:        models.ChainBTC,
		AddressIndex: 0,
		TxHash:       "aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122aabb1122",
		Direction:    "send",
		Token:        models.TokenNative,
		Amount:       "50000",
		FromAddress:  "bc1qsender",
		ToAddress:    "bc1qreceiver",
		Status:       "pending",
	}

	id, err := d.InsertTransaction(tx)
	if err != nil {
		t.Fatal(err)
	}

	// Update to failed without confirmedAt.
	if err := d.UpdateTransactionStatus(id, "failed", nil); err != nil {
		t.Fatalf("UpdateTransactionStatus() error = %v", err)
	}

	got, err := d.GetTransaction(id)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != "failed" {
		t.Errorf("Status = %s, want failed", got.Status)
	}
	if got.ConfirmedAt != "" {
		t.Errorf("ConfirmedAt = %s, want empty", got.ConfirmedAt)
	}
}

func TestGetTransactionByHash(t *testing.T) {
	d := setupTestDB(t)

	txHash := "ddee1122ddee1122ddee1122ddee1122ddee1122ddee1122ddee1122ddee1122"
	tx := models.Transaction{
		Chain:        models.ChainBTC,
		AddressIndex: 5,
		TxHash:       txHash,
		Direction:    "send",
		Token:        models.TokenNative,
		Amount:       "75000",
		FromAddress:  "bc1qfrom",
		ToAddress:    "bc1qto",
		Status:       "pending",
	}

	_, err := d.InsertTransaction(tx)
	if err != nil {
		t.Fatal(err)
	}

	got, err := d.GetTransactionByHash(models.ChainBTC, txHash)
	if err != nil {
		t.Fatalf("GetTransactionByHash() error = %v", err)
	}

	if got.TxHash != txHash {
		t.Errorf("TxHash = %s, want %s", got.TxHash, txHash)
	}
	if got.AddressIndex != 5 {
		t.Errorf("AddressIndex = %d, want 5", got.AddressIndex)
	}
}

func TestListTransactions_All(t *testing.T) {
	d := setupTestDB(t)

	// Insert 5 BTC transactions and 3 BSC transactions.
	for i := 0; i < 5; i++ {
		d.InsertTransaction(models.Transaction{
			Chain:        models.ChainBTC,
			AddressIndex: i,
			TxHash:       "btctx" + itoa(i) + "0000000000000000000000000000000000000000000000000000000000",
			Direction:    "send",
			Token:        models.TokenNative,
			Amount:       itoa((i + 1) * 10000),
			FromAddress:  "bc1qfrom" + itoa(i),
			ToAddress:    "bc1qdest",
			Status:       "pending",
		})
	}
	for i := 0; i < 3; i++ {
		d.InsertTransaction(models.Transaction{
			Chain:        models.ChainBSC,
			AddressIndex: i,
			TxHash:       "bsctx" + itoa(i) + "0000000000000000000000000000000000000000000000000000000000",
			Direction:    "send",
			Token:        models.TokenNative,
			Amount:       itoa((i + 1) * 20000),
			FromAddress:  "0xfrom" + itoa(i),
			ToAddress:    "0xdest",
			Status:       "pending",
		})
	}

	// List all transactions.
	txs, total, err := d.ListTransactions(nil, 1, 100)
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}

	if total != 8 {
		t.Errorf("total = %d, want 8", total)
	}
	if len(txs) != 8 {
		t.Errorf("returned = %d, want 8", len(txs))
	}
}

func TestListTransactions_FilterByChain(t *testing.T) {
	d := setupTestDB(t)

	for i := 0; i < 5; i++ {
		d.InsertTransaction(models.Transaction{
			Chain:        models.ChainBTC,
			AddressIndex: i,
			TxHash:       "btctx" + itoa(i) + "0000000000000000000000000000000000000000000000000000000000",
			Direction:    "send",
			Token:        models.TokenNative,
			Amount:       "10000",
			FromAddress:  "bc1qfrom",
			ToAddress:    "bc1qdest",
			Status:       "pending",
		})
	}
	for i := 0; i < 3; i++ {
		d.InsertTransaction(models.Transaction{
			Chain:        models.ChainBSC,
			AddressIndex: i,
			TxHash:       "bsctx" + itoa(i) + "0000000000000000000000000000000000000000000000000000000000",
			Direction:    "send",
			Token:        models.TokenNative,
			Amount:       "20000",
			FromAddress:  "0xfrom",
			ToAddress:    "0xdest",
			Status:       "pending",
		})
	}

	// Filter by BTC.
	chain := models.ChainBTC
	txs, total, err := d.ListTransactions(&chain, 1, 100)
	if err != nil {
		t.Fatalf("ListTransactions(BTC) error = %v", err)
	}

	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(txs) != 5 {
		t.Errorf("returned = %d, want 5", len(txs))
	}

	// All should be BTC.
	for _, tx := range txs {
		if tx.Chain != models.ChainBTC {
			t.Errorf("expected chain BTC, got %s", tx.Chain)
		}
	}
}

func TestListTransactions_Pagination(t *testing.T) {
	d := setupTestDB(t)

	for i := 0; i < 10; i++ {
		d.InsertTransaction(models.Transaction{
			Chain:        models.ChainBTC,
			AddressIndex: i,
			TxHash:       "tx" + itoa(i) + "000000000000000000000000000000000000000000000000000000000000",
			Direction:    "send",
			Token:        models.TokenNative,
			Amount:       "10000",
			FromAddress:  "bc1qfrom",
			ToAddress:    "bc1qdest",
			Status:       "pending",
		})
	}

	// Page 1, size 3.
	txs, total, err := d.ListTransactions(nil, 1, 3)
	if err != nil {
		t.Fatal(err)
	}

	if total != 10 {
		t.Errorf("total = %d, want 10", total)
	}
	if len(txs) != 3 {
		t.Errorf("page 1 returned = %d, want 3", len(txs))
	}

	// Page 4 (last page with 1 item).
	txs2, total2, err := d.ListTransactions(nil, 4, 3)
	if err != nil {
		t.Fatal(err)
	}

	if total2 != 10 {
		t.Errorf("total = %d, want 10", total2)
	}
	if len(txs2) != 1 {
		t.Errorf("page 4 returned = %d, want 1", len(txs2))
	}
}

func TestListTransactionsFiltered_ByDirection(t *testing.T) {
	d := setupTestDB(t)

	// Insert "send" and "receive" transactions.
	for i := 0; i < 4; i++ {
		d.InsertTransaction(models.Transaction{
			Chain:        models.ChainBTC,
			AddressIndex: i,
			TxHash:       "send" + itoa(i) + "0000000000000000000000000000000000000000000000000000000000",
			Direction:    "out",
			Token:        models.TokenNative,
			Amount:       "10000",
			FromAddress:  "bc1qfrom",
			ToAddress:    "bc1qdest",
			Status:       "confirmed",
		})
	}
	for i := 0; i < 2; i++ {
		d.InsertTransaction(models.Transaction{
			Chain:        models.ChainBTC,
			AddressIndex: i,
			TxHash:       "recv" + itoa(i) + "0000000000000000000000000000000000000000000000000000000000",
			Direction:    "in",
			Token:        models.TokenNative,
			Amount:       "5000",
			FromAddress:  "bc1qext",
			ToAddress:    "bc1qlocal",
			Status:       "confirmed",
		})
	}

	dir := "out"
	txs, total, err := d.ListTransactionsFiltered(TransactionFilter{
		Direction: &dir,
		Page:      1,
		PageSize:  100,
	})
	if err != nil {
		t.Fatalf("ListTransactionsFiltered(out) error = %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if len(txs) != 4 {
		t.Errorf("returned = %d, want 4", len(txs))
	}
}

func TestListTransactionsFiltered_ByToken(t *testing.T) {
	d := setupTestDB(t)

	d.InsertTransaction(models.Transaction{
		Chain: models.ChainBSC, AddressIndex: 0,
		TxHash: "nat10000000000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenNative, Amount: "100",
		FromAddress: "0xfrom", ToAddress: "0xto", Status: "confirmed",
	})
	d.InsertTransaction(models.Transaction{
		Chain: models.ChainBSC, AddressIndex: 1,
		TxHash: "usdc0000000000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenUSDC, Amount: "200",
		FromAddress: "0xfrom2", ToAddress: "0xto2", Status: "confirmed",
	})

	token := models.TokenUSDC
	txs, total, err := d.ListTransactionsFiltered(TransactionFilter{
		Token:    &token,
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(txs) != 1 {
		t.Errorf("returned = %d, want 1", len(txs))
	}
	if txs[0].Token != models.TokenUSDC {
		t.Errorf("token = %s, want USDC", txs[0].Token)
	}
}

func TestListTransactionsFiltered_ByStatus(t *testing.T) {
	d := setupTestDB(t)

	d.InsertTransaction(models.Transaction{
		Chain: models.ChainBTC, AddressIndex: 0,
		TxHash: "pend0000000000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenNative, Amount: "100",
		FromAddress: "bc1qa", ToAddress: "bc1qb", Status: "pending",
	})
	d.InsertTransaction(models.Transaction{
		Chain: models.ChainBTC, AddressIndex: 1,
		TxHash: "conf0000000000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenNative, Amount: "200",
		FromAddress: "bc1qc", ToAddress: "bc1qd", Status: "confirmed",
	})

	status := "pending"
	txs, total, err := d.ListTransactionsFiltered(TransactionFilter{
		Status:   &status,
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(txs) != 1 {
		t.Errorf("returned = %d, want 1", len(txs))
	}
	if txs[0].Status != "pending" {
		t.Errorf("status = %s, want pending", txs[0].Status)
	}
}

func TestListTransactionsFiltered_MultipleFilters(t *testing.T) {
	d := setupTestDB(t)

	// BTC send pending
	d.InsertTransaction(models.Transaction{
		Chain: models.ChainBTC, AddressIndex: 0,
		TxHash: "btcsendp000000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenNative, Amount: "100",
		FromAddress: "bc1qa", ToAddress: "bc1qb", Status: "pending",
	})
	// BTC send confirmed
	d.InsertTransaction(models.Transaction{
		Chain: models.ChainBTC, AddressIndex: 1,
		TxHash: "btcsendc000000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenNative, Amount: "200",
		FromAddress: "bc1qc", ToAddress: "bc1qd", Status: "confirmed",
	})
	// BSC send pending
	d.InsertTransaction(models.Transaction{
		Chain: models.ChainBSC, AddressIndex: 0,
		TxHash: "bscsendp000000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenNative, Amount: "300",
		FromAddress: "0xa", ToAddress: "0xb", Status: "pending",
	})

	chain := models.ChainBTC
	dir := "out"
	status := "confirmed"
	txs, total, err := d.ListTransactionsFiltered(TransactionFilter{
		Chain:     &chain,
		Direction: &dir,
		Status:    &status,
		Page:      1,
		PageSize:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(txs) != 1 {
		t.Errorf("returned = %d, want 1", len(txs))
	}
}
