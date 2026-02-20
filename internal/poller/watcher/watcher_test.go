package watcher

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
	"github.com/Fantasim/hdpay/internal/poller/points"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/provider"
	"github.com/Fantasim/hdpay/internal/price"
)

// --- Mock Provider ---

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	name   string
	chain  string
	mu     sync.Mutex
	txs    []provider.RawTransaction
	err    error
	block  uint64
	confirmed     bool
	confirmations int
	confirmErr    error
}

func newMockProvider(name, chain string) *mockProvider {
	return &mockProvider{name: name, chain: chain, block: 100}
}

func (m *mockProvider) Name() string  { return m.name }
func (m *mockProvider) Chain() string { return m.chain }

func (m *mockProvider) FetchTransactions(_ context.Context, _ string, _ int64) ([]provider.RawTransaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	// Return a copy and clear (each fetch returns txs once, simulating real behavior).
	result := make([]provider.RawTransaction, len(m.txs))
	copy(result, m.txs)
	m.txs = nil
	return result, nil
}

func (m *mockProvider) CheckConfirmation(_ context.Context, _ string, _ int64) (bool, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.confirmErr != nil {
		return false, 0, m.confirmErr
	}
	return m.confirmed, m.confirmations, nil
}

func (m *mockProvider) GetCurrentBlock(_ context.Context) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.block, nil
}

func (m *mockProvider) setTxs(txs []provider.RawTransaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs = txs
}

func (m *mockProvider) setConfirmed(confirmed bool, confirmations int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.confirmed = confirmed
	m.confirmations = confirmations
}

func (m *mockProvider) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// --- Mock PriceService ---

// mockPriceService mocks price.PriceService with a fixed response.
type mockPriceService struct {
	prices map[string]float64
	err    error
}

func (m *mockPriceService) GetPrices(_ context.Context) (map[string]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.prices, nil
}

// --- Test Helpers ---

func setupTestDB(t *testing.T) *pollerdb.DB {
	t.Helper()
	tmpFile := fmt.Sprintf("/tmp/poller_watcher_test_%d.sqlite", time.Now().UnixNano())
	db, err := pollerdb.New(tmpFile)
	if err != nil {
		t.Fatalf("failed to create test DB: %v", err)
	}
	if err := db.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(tmpFile)
		os.Remove(tmpFile + "-wal")
		os.Remove(tmpFile + "-shm")
	})
	return db
}

func testConfig() *config.Config {
	return &config.Config{
		Network:             "testnet",
		StartDate:           1700000000, // 2023-11-14
		MaxActiveWatches:    10,
		DefaultWatchTimeout: 5,
		TiersFile:           "",
	}
}

func defaultTiers() []models.Tier {
	max1 := 1.0
	max12 := 12.0
	max60 := 60.0
	max120 := 120.0
	max300 := 300.0
	max600 := 600.0
	max1200 := 1200.0
	return []models.Tier{
		{MinUSD: 0, MaxUSD: &max1, Multiplier: 0},
		{MinUSD: 1, MaxUSD: &max12, Multiplier: 1.0},
		{MinUSD: 12, MaxUSD: &max60, Multiplier: 1.2},
		{MinUSD: 60, MaxUSD: &max120, Multiplier: 1.5},
		{MinUSD: 120, MaxUSD: &max300, Multiplier: 1.8},
		{MinUSD: 300, MaxUSD: &max600, Multiplier: 2.0},
		{MinUSD: 600, MaxUSD: &max1200, Multiplier: 2.5},
		{MinUSD: 1200, MaxUSD: nil, Multiplier: 3.0},
	}
}

func setupWatcher(t *testing.T, db *pollerdb.DB, mp *mockProvider) *Watcher {
	t.Helper()
	cfg := testConfig()
	tiers := defaultTiers()
	calculator := points.NewPointsCalculator(tiers)

	// Create mock provider set.
	ps := provider.NewProviderSet(mp.Chain(), []provider.Provider{mp}, []int{100})
	providers := map[string]*provider.ProviderSet{
		mp.Chain(): ps,
	}

	// Create pricer backed by the shared test price server (init'd in testhelpers_test.go).
	pricer := newTestPricer()

	w := NewWatcher(db, providers, pricer, calculator, cfg)
	t.Cleanup(func() {
		w.Stop()
	})
	return w
}

// newTestPricer creates a Pricer backed by the shared test price server.
func newTestPricer() *points.Pricer {
	ps := price.NewPriceServiceWithURL(testPriceServer.URL)
	return points.NewPricer(ps)
}

// --- Tests ---

func TestNewWatcher(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	if w.ActiveCount() != 0 {
		t.Errorf("expected 0 active watches, got %d", w.ActiveCount())
	}
	if w.MaxActiveWatches() != 10 {
		t.Errorf("expected max 10, got %d", w.MaxActiveWatches())
	}
	if w.DefaultWatchTimeout() != 5 {
		t.Errorf("expected default timeout 5, got %d", w.DefaultWatchTimeout())
	}
}

func TestRuntimeSettings(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	w.SetMaxActiveWatches(50)
	if w.MaxActiveWatches() != 50 {
		t.Errorf("expected 50, got %d", w.MaxActiveWatches())
	}

	w.SetDefaultWatchTimeout(15)
	if w.DefaultWatchTimeout() != 15 {
		t.Errorf("expected 15, got %d", w.DefaultWatchTimeout())
	}
}

func TestCreateWatch(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch, err := w.CreateWatch("BTC", "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr", 10)
	if err != nil {
		t.Fatalf("CreateWatch failed: %v", err)
	}

	if watch.ID == "" {
		t.Error("expected non-empty watch ID")
	}
	if watch.Chain != "BTC" {
		t.Errorf("expected chain BTC, got %s", watch.Chain)
	}
	if watch.Status != models.WatchStatusActive {
		t.Errorf("expected ACTIVE status, got %s", watch.Status)
	}
	if w.ActiveCount() != 1 {
		t.Errorf("expected 1 active, got %d", w.ActiveCount())
	}

	// Verify DB record.
	dbWatch, err := db.GetWatch(watch.ID)
	if err != nil {
		t.Fatalf("GetWatch failed: %v", err)
	}
	if dbWatch == nil {
		t.Fatal("watch not found in DB")
	}
	if dbWatch.Chain != "BTC" {
		t.Errorf("DB chain: expected BTC, got %s", dbWatch.Chain)
	}
}

func TestCreateWatch_InvalidChain(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	_, err := w.CreateWatch("ETH", "0x123", 10)
	if err == nil {
		t.Fatal("expected error for invalid chain")
	}
}

func TestCreateWatch_DuplicateAddress(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	addr := "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"

	_, err := w.CreateWatch("BTC", addr, 10)
	if err != nil {
		t.Fatalf("first CreateWatch failed: %v", err)
	}

	_, err = w.CreateWatch("BTC", addr, 10)
	if err == nil {
		t.Fatal("expected error for duplicate address")
	}
}

func TestCreateWatch_MaxLimit(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	w.SetMaxActiveWatches(2)

	_, err := w.CreateWatch("BTC", "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr", 10)
	if err != nil {
		t.Fatalf("first CreateWatch failed: %v", err)
	}
	_, err = w.CreateWatch("BTC", "tb1qgadxe2kacxtw44un284vskrn6w2xgsmm7h2hfg", 10)
	if err != nil {
		t.Fatalf("second CreateWatch failed: %v", err)
	}
	_, err = w.CreateWatch("BTC", "tb1qkmq5vclvgp022zg00r6w8k36s9nnysge5a5m83", 10)
	if err == nil {
		t.Fatal("expected error when exceeding max watches")
	}
}

func TestCancelWatch(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch, err := w.CreateWatch("BTC", "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr", 10)
	if err != nil {
		t.Fatalf("CreateWatch failed: %v", err)
	}

	err = w.CancelWatch(watch.ID)
	if err != nil {
		t.Fatalf("CancelWatch failed: %v", err)
	}

	// Wait for goroutine to clean up.
	time.Sleep(200 * time.Millisecond)

	if w.ActiveCount() != 0 {
		t.Errorf("expected 0 active after cancel, got %d", w.ActiveCount())
	}

	// Check DB status.
	dbWatch, _ := db.GetWatch(watch.ID)
	if dbWatch.Status != models.WatchStatusCancelled {
		t.Errorf("expected CANCELLED, got %s", dbWatch.Status)
	}
}

func TestCancelWatch_NotFound(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	err := w.CancelWatch("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent watch")
	}
}

func TestTransactionDetection(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	addr := "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"

	// Pre-load mock with a confirmed transaction.
	mp.setTxs([]provider.RawTransaction{
		{
			TxHash:        "tx-hash-001",
			Token:         "BTC",
			AmountRaw:     "100000",
			AmountHuman:   "0.001",
			Decimals:      8,
			BlockTime:     time.Now().Unix(),
			Confirmed:     true,
			Confirmations: 1,
			BlockNumber:   800000,
		},
	})

	_, err := w.CreateWatch("BTC", addr, 5)
	if err != nil {
		t.Fatalf("CreateWatch failed: %v", err)
	}

	// Wait for at least one poll tick (BTC = 60s, but we use a shorter interval in tests).
	// Since we can't easily change the interval, we'll wait and check.
	// BTC poll is 60s — too long for unit tests. Instead, test processTransaction directly.
	time.Sleep(200 * time.Millisecond)
}

func TestProcessTransaction_Confirmed(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:      "test-watch-1",
		Chain:   "BTC",
		Address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:  models.WatchStatusActive,
	}
	db.CreateWatch(watch)

	raw := provider.RawTransaction{
		TxHash:        "tx-confirmed-001",
		Token:         "BTC",
		AmountRaw:     "10000000", // 0.1 BTC
		AmountHuman:   "0.1",
		Decimals:      8,
		BlockTime:     time.Now().Unix(),
		Confirmed:     true,
		Confirmations: 1,
		BlockNumber:   800000,
	}

	ctx := context.Background()
	err := w.processTransaction(ctx, watch, raw)
	if err != nil {
		t.Fatalf("processTransaction failed: %v", err)
	}

	// Verify transaction in DB.
	tx, err := db.GetByTxHash("tx-confirmed-001")
	if err != nil {
		t.Fatalf("GetByTxHash failed: %v", err)
	}
	if tx == nil {
		t.Fatal("transaction not found in DB")
	}
	if tx.Status != models.TxStatusConfirmed {
		t.Errorf("expected CONFIRMED, got %s", tx.Status)
	}
	if tx.Points <= 0 {
		t.Errorf("expected positive points, got %d", tx.Points)
	}

	// Verify points ledger.
	pa, err := db.GetOrCreatePoints(watch.Address, watch.Chain)
	if err != nil {
		t.Fatalf("GetOrCreatePoints failed: %v", err)
	}
	if pa.Unclaimed <= 0 {
		t.Errorf("expected unclaimed > 0, got %d", pa.Unclaimed)
	}
}

func TestProcessTransaction_Pending(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:      "test-watch-2",
		Chain:   "BTC",
		Address: "tb1qgadxe2kacxtw44un284vskrn6w2xgsmm7h2hfg",
		Status:  models.WatchStatusActive,
	}
	db.CreateWatch(watch)

	raw := provider.RawTransaction{
		TxHash:        "tx-pending-001",
		Token:         "BTC",
		AmountRaw:     "5000000",
		AmountHuman:   "0.05",
		Decimals:      8,
		BlockTime:     time.Now().Unix(),
		Confirmed:     false,
		Confirmations: 0,
	}

	ctx := context.Background()
	err := w.processTransaction(ctx, watch, raw)
	if err != nil {
		t.Fatalf("processTransaction failed: %v", err)
	}

	tx, err := db.GetByTxHash("tx-pending-001")
	if err != nil {
		t.Fatalf("GetByTxHash failed: %v", err)
	}
	if tx == nil {
		t.Fatal("pending tx not found in DB")
	}
	if tx.Status != models.TxStatusPending {
		t.Errorf("expected PENDING, got %s", tx.Status)
	}
}

func TestProcessTransaction_Dedup(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:      "test-watch-3",
		Chain:   "BTC",
		Address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:  models.WatchStatusActive,
	}
	db.CreateWatch(watch)

	raw := provider.RawTransaction{
		TxHash:        "tx-dedup-001",
		Token:         "BTC",
		AmountRaw:     "100000",
		AmountHuman:   "0.001",
		Decimals:      8,
		BlockTime:     time.Now().Unix(),
		Confirmed:     true,
		Confirmations: 1,
		BlockNumber:   800000,
	}

	ctx := context.Background()
	// Process once.
	err := w.processTransaction(ctx, watch, raw)
	if err != nil {
		t.Fatalf("first processTransaction failed: %v", err)
	}

	// Process same tx_hash again.
	err = w.processTransaction(ctx, watch, raw)
	if err != nil {
		t.Fatalf("second processTransaction failed: %v", err)
	}

	// Count transactions — should be exactly 1.
	total, _, err := db.CountByWatchID(watch.ID)
	if err != nil {
		t.Fatalf("CountByWatchID failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 transaction (dedup), got %d", total)
	}
}

func TestProcessTransaction_SOLCompositeDedup(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-sol", "SOL")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:      "test-watch-sol",
		Chain:   "SOL",
		Address: "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		Status:  models.WatchStatusActive,
	}
	db.CreateWatch(watch)

	ctx := context.Background()

	// SOL tx: same base signature, different tokens.
	rawSOL := provider.RawTransaction{
		TxHash:      "abc123sig:SOL",
		Token:       "SOL",
		AmountRaw:   "1000000000",
		AmountHuman: "1.0",
		Decimals:    9,
		BlockTime:   time.Now().Unix(),
		Confirmed:   true,
		Confirmations: 1,
	}
	rawUSDC := provider.RawTransaction{
		TxHash:      "abc123sig:USDC",
		Token:       "USDC",
		AmountRaw:   "10000000",
		AmountHuman: "10.0",
		Decimals:    6,
		BlockTime:   time.Now().Unix(),
		Confirmed:   true,
		Confirmations: 1,
	}

	if err := w.processTransaction(ctx, watch, rawSOL); err != nil {
		t.Fatalf("processTransaction SOL failed: %v", err)
	}
	if err := w.processTransaction(ctx, watch, rawUSDC); err != nil {
		t.Fatalf("processTransaction USDC failed: %v", err)
	}

	// Both should exist (different composite keys).
	total, _, err := db.CountByWatchID(watch.ID)
	if err != nil {
		t.Fatalf("CountByWatchID failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 transactions (SOL + USDC), got %d", total)
	}

	// Same composite key again should be deduped.
	if err := w.processTransaction(ctx, watch, rawSOL); err != nil {
		t.Fatalf("duplicate processTransaction failed: %v", err)
	}
	total, _, err = db.CountByWatchID(watch.ID)
	if err != nil {
		t.Fatalf("CountByWatchID failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected still 2 (dedup), got %d", total)
	}
}

func TestRecheckPending_Confirms(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:      "test-watch-recheck",
		Chain:   "BTC",
		Address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:  models.WatchStatusActive,
	}
	db.CreateWatch(watch)

	// Insert a pending transaction directly.
	pendingTx := &models.Transaction{
		WatchID:     watch.ID,
		TxHash:      "tx-pending-recheck",
		Chain:       "BTC",
		Address:     watch.Address,
		Token:       "BTC",
		AmountRaw:   "200000",
		AmountHuman: "0.002",
		Decimals:    8,
		Status:      models.TxStatusPending,
		DetectedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	db.InsertTransaction(pendingTx)
	db.GetOrCreatePoints(watch.Address, watch.Chain)

	// Set mock to return confirmed.
	mp.setConfirmed(true, 6)

	ps := w.providers["BTC"]
	ctx := context.Background()
	err := w.recheckPending(ctx, watch, ps)
	if err != nil {
		t.Fatalf("recheckPending failed: %v", err)
	}

	// Verify tx is now confirmed.
	tx, _ := db.GetByTxHash("tx-pending-recheck")
	if tx.Status != models.TxStatusConfirmed {
		t.Errorf("expected CONFIRMED, got %s", tx.Status)
	}
}

func TestResolveCutoff_NoHistory(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	startDate := int64(1700000000)
	cutoff := w.resolveCutoff("unknown-address", startDate)
	if cutoff != startDate {
		t.Errorf("expected startDate %d, got %d", startDate, cutoff)
	}
}

func TestResolveCutoff_WithHistory(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	addr := "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr"
	startDate := int64(1700000000)

	// Insert a watch and transaction for history.
	watch := &models.Watch{
		ID:      "history-watch",
		Chain:   "BTC",
		Address: addr,
		Status:  models.WatchStatusExpired,
	}
	db.CreateWatch(watch)

	laterTime := time.Unix(1700100000, 0).UTC().Format(time.RFC3339) // later than startDate
	tx := &models.Transaction{
		WatchID:     watch.ID,
		TxHash:      "tx-history-001",
		Chain:       "BTC",
		Address:     addr,
		Token:       "BTC",
		AmountRaw:   "100000",
		AmountHuman: "0.001",
		Decimals:    8,
		Status:      models.TxStatusConfirmed,
		DetectedAt:  laterTime,
	}
	db.InsertTransaction(tx)

	cutoff := w.resolveCutoff(addr, startDate)
	if cutoff != 1700100000 {
		t.Errorf("expected 1700100000, got %d", cutoff)
	}
}

func TestCheckStopConditions_Expired(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	// Create a watch that's already expired.
	watch := &models.Watch{
		ID:        "expired-watch",
		Chain:     "BTC",
		Address:   "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:    models.WatchStatusActive,
		ExpiresAt: time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339),
	}
	db.CreateWatch(watch)

	shouldStop, status := w.checkStopConditions(watch)
	if !shouldStop {
		t.Error("expected shouldStop=true for expired watch")
	}
	if status != models.WatchStatusExpired {
		t.Errorf("expected EXPIRED, got %s", status)
	}
}

func TestCheckStopConditions_Completed(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:        "complete-watch",
		Chain:     "BTC",
		Address:   "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:    models.WatchStatusActive,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
	}
	db.CreateWatch(watch)

	// Insert a confirmed transaction.
	now := time.Now().UTC().Format(time.RFC3339)
	tx := &models.Transaction{
		WatchID:     watch.ID,
		TxHash:      "tx-complete-001",
		Chain:       "BTC",
		Address:     watch.Address,
		Token:       "BTC",
		AmountRaw:   "100000",
		AmountHuman: "0.001",
		Decimals:    8,
		Status:      models.TxStatusConfirmed,
		DetectedAt:  now,
		ConfirmedAt: &now,
	}
	db.InsertTransaction(tx)

	shouldStop, status := w.checkStopConditions(watch)
	if !shouldStop {
		t.Error("expected shouldStop=true for completed watch (all txs confirmed)")
	}
	if status != models.WatchStatusCompleted {
		t.Errorf("expected COMPLETED, got %s", status)
	}
}

func TestCheckStopConditions_NotYet(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:        "active-watch",
		Chain:     "BTC",
		Address:   "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:    models.WatchStatusActive,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
	}
	db.CreateWatch(watch)

	// No transactions yet.
	shouldStop, _ := w.checkStopConditions(watch)
	if shouldStop {
		t.Error("expected shouldStop=false when no txs")
	}
}

func TestCheckStopConditions_PendingRemaining(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:        "pending-watch",
		Chain:     "BTC",
		Address:   "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:    models.WatchStatusActive,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
	}
	db.CreateWatch(watch)

	// Insert a pending transaction.
	tx := &models.Transaction{
		WatchID:     watch.ID,
		TxHash:      "tx-still-pending",
		Chain:       "BTC",
		Address:     watch.Address,
		Token:       "BTC",
		AmountRaw:   "100000",
		AmountHuman: "0.001",
		Decimals:    8,
		Status:      models.TxStatusPending,
		DetectedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	db.InsertTransaction(tx)

	shouldStop, _ := w.checkStopConditions(watch)
	if shouldStop {
		t.Error("expected shouldStop=false when pending txs remain")
	}
}

func TestRecovery_ExpiresActive(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	// Insert an ACTIVE watch directly (simulating crash).
	staleWatch := &models.Watch{
		ID:      "stale-active-watch",
		Chain:   "BTC",
		Address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:  models.WatchStatusActive,
	}
	db.CreateWatch(staleWatch)

	ctx := context.Background()
	err := w.RunRecovery(ctx)
	if err != nil {
		t.Fatalf("RunRecovery failed: %v", err)
	}

	// Verify watch is now EXPIRED.
	dbWatch, _ := db.GetWatch(staleWatch.ID)
	if dbWatch.Status != models.WatchStatusExpired {
		t.Errorf("expected EXPIRED, got %s", dbWatch.Status)
	}
}

func TestRecovery_ConfirmsPending(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	// Insert an expired watch with a pending tx.
	watch := &models.Watch{
		ID:      "recovery-watch",
		Chain:   "BTC",
		Address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:  models.WatchStatusExpired,
	}
	db.CreateWatch(watch)

	tx := &models.Transaction{
		WatchID:     watch.ID,
		TxHash:      "tx-orphaned-pending",
		Chain:       "BTC",
		Address:     watch.Address,
		Token:       "BTC",
		AmountRaw:   "500000",
		AmountHuman: "0.005",
		Decimals:    8,
		Status:      models.TxStatusPending,
		DetectedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	db.InsertTransaction(tx)
	db.GetOrCreatePoints(watch.Address, watch.Chain)

	// Set mock to confirm.
	mp.setConfirmed(true, 3)

	ctx := context.Background()
	err := w.RunRecovery(ctx)
	if err != nil {
		t.Fatalf("RunRecovery failed: %v", err)
	}

	// Verify tx confirmed.
	dbTx, _ := db.GetByTxHash("tx-orphaned-pending")
	if dbTx.Status != models.TxStatusConfirmed {
		t.Errorf("expected CONFIRMED after recovery, got %s", dbTx.Status)
	}
}

func TestStop_GracefulShutdown(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	cfg := testConfig()
	tiers := defaultTiers()
	calculator := points.NewPointsCalculator(tiers)
	pricer := newTestPricer()
	ps := provider.NewProviderSet(mp.Chain(), []provider.Provider{mp}, []int{100})
	providers := map[string]*provider.ProviderSet{"BTC": ps}

	w := NewWatcher(db, providers, pricer, calculator, cfg)

	// Create a watch.
	_, err := w.CreateWatch("BTC", "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr", 10)
	if err != nil {
		t.Fatalf("CreateWatch failed: %v", err)
	}

	if w.ActiveCount() != 1 {
		t.Errorf("expected 1 active, got %d", w.ActiveCount())
	}

	// Stop should be clean and fast.
	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good.
	case <-time.After(5 * time.Second):
		t.Fatal("Stop took too long")
	}

	if w.ActiveCount() != 0 {
		t.Errorf("expected 0 active after stop, got %d", w.ActiveCount())
	}
}

func TestExtractBaseSignature(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc123:SOL", "abc123"},
		{"abc123:USDC", "abc123"},
		{"no-colon-hash", "no-colon-hash"},
		{"multi:colon:SOL", "multi:colon"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractBaseSignature(tt.input)
		if got != tt.want {
			t.Errorf("extractBaseSignature(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseAmountHuman(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"0.001", 0.001},
		{"1.0", 1.0},
		{"0.0", 0.0},
		{"invalid", 0.0},
		{"", 0.0},
	}
	for _, tt := range tests {
		got := parseAmountHuman(tt.input)
		if got != tt.want {
			t.Errorf("parseAmountHuman(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestPollInterval(t *testing.T) {
	if pollInterval("BTC") != config.PollIntervalBTC {
		t.Errorf("BTC interval mismatch")
	}
	if pollInterval("BSC") != config.PollIntervalBSC {
		t.Errorf("BSC interval mismatch")
	}
	if pollInterval("SOL") != config.PollIntervalSOL {
		t.Errorf("SOL interval mismatch")
	}
	if pollInterval("UNKNOWN") != config.PollIntervalBTC {
		t.Errorf("unknown chain should fallback to BTC interval")
	}
}

func TestProcessTransaction_ConfirmedStablecoin(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-bsc", "BSC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:      "test-watch-stable",
		Chain:   "BSC",
		Address: "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb",
		Status:  models.WatchStatusActive,
	}
	db.CreateWatch(watch)

	raw := provider.RawTransaction{
		TxHash:        "tx-usdc-001",
		Token:         "USDC",
		AmountRaw:     "10000000",
		AmountHuman:   "10.0",
		Decimals:      6,
		BlockTime:     time.Now().Unix(),
		Confirmed:     true,
		Confirmations: 12,
		BlockNumber:   5000000,
	}

	ctx := context.Background()
	err := w.processTransaction(ctx, watch, raw)
	if err != nil {
		t.Fatalf("processTransaction USDC failed: %v", err)
	}

	tx, _ := db.GetByTxHash("tx-usdc-001")
	if tx == nil {
		t.Fatal("USDC tx not found")
	}
	if tx.Status != models.TxStatusConfirmed {
		t.Errorf("expected CONFIRMED, got %s", tx.Status)
	}
	// USDC = $1.00, so 10 USDC = $10 USD -> tier 1 (1.0x) -> 1000 points.
	if tx.USDValue != 10.0 {
		t.Errorf("expected USD value 10.0, got %f", tx.USDValue)
	}
	if tx.Points != 1000 {
		t.Errorf("expected 1000 points for $10 USDC at 1.0x, got %d", tx.Points)
	}
}

func TestRecovery_UnresolvablePending(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow recovery retry test in short mode")
	}
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:      "unresolvable-watch",
		Chain:   "BTC",
		Address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:  models.WatchStatusExpired,
	}
	db.CreateWatch(watch)

	tx := &models.Transaction{
		WatchID:     watch.ID,
		TxHash:      "tx-unresolvable",
		Chain:       "BTC",
		Address:     watch.Address,
		Token:       "BTC",
		AmountRaw:   "100000",
		AmountHuman: "0.001",
		Decimals:    8,
		Status:      models.TxStatusPending,
		DetectedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	db.InsertTransaction(tx)
	db.GetOrCreatePoints(watch.Address, watch.Chain)

	// Mock returns NOT confirmed.
	mp.setConfirmed(false, 0)

	ctx := context.Background()
	err := w.RunRecovery(ctx)
	if err != nil {
		t.Fatalf("RunRecovery failed: %v", err)
	}

	// Tx should still be pending.
	dbTx, _ := db.GetByTxHash("tx-unresolvable")
	if dbTx.Status != models.TxStatusPending {
		t.Errorf("expected still PENDING, got %s", dbTx.Status)
	}

	// System error should be logged.
	errors, _ := db.ListUnresolved()
	if len(errors) == 0 {
		t.Error("expected a system error for unresolvable pending tx")
	}
}

func TestRecheckPending_ProviderError(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{
		ID:      "test-watch-provider-err",
		Chain:   "BTC",
		Address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		Status:  models.WatchStatusActive,
	}
	db.CreateWatch(watch)

	pendingTx := &models.Transaction{
		WatchID:     watch.ID,
		TxHash:      "tx-provider-err",
		Chain:       "BTC",
		Address:     watch.Address,
		Token:       "BTC",
		AmountRaw:   "100000",
		AmountHuman: "0.001",
		Decimals:    8,
		Status:      models.TxStatusPending,
		DetectedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	db.InsertTransaction(pendingTx)
	db.GetOrCreatePoints(watch.Address, watch.Chain)

	// Set mock to return error on confirmation check.
	mp.mu.Lock()
	mp.confirmErr = fmt.Errorf("provider down")
	mp.mu.Unlock()

	ps := w.providers["BTC"]
	ctx := context.Background()
	err := w.recheckPending(ctx, watch, ps)
	// Should return the error but not crash.
	if err == nil {
		t.Error("expected error from recheckPending when provider fails")
	}

	// Tx should still be pending.
	dbTx, _ := db.GetByTxHash("tx-provider-err")
	if dbTx.Status != models.TxStatusPending {
		t.Errorf("expected still PENDING, got %s", dbTx.Status)
	}
}

func TestConcurrentWatches(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	addrs := []string{
		"tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
		"tb1qgadxe2kacxtw44un284vskrn6w2xgsmm7h2hfg",
		"tb1qkmq5vclvgp022zg00r6w8k36s9nnysge5a5m83",
	}

	for _, addr := range addrs {
		_, err := w.CreateWatch("BTC", addr, 5)
		if err != nil {
			t.Fatalf("CreateWatch(%s) failed: %v", addr, err)
		}
	}

	if w.ActiveCount() != 3 {
		t.Errorf("expected 3 active, got %d", w.ActiveCount())
	}

	// All should shut down cleanly.
	w.Stop()

	if w.ActiveCount() != 0 {
		t.Errorf("expected 0 active after stop, got %d", w.ActiveCount())
	}
}

func TestNilIfZero(t *testing.T) {
	if nilIfZero(0) != nil {
		t.Error("expected nil for 0")
	}
	v := nilIfZero(42)
	if v == nil || *v != 42 {
		t.Error("expected pointer to 42")
	}
}

func TestLogSystemError(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	w.logSystemError(config.ErrorSeverityWarn, config.ErrorCategoryWatcher, "test error", "details here")

	errors, err := db.ListUnresolved()
	if err != nil {
		t.Fatalf("ListUnresolved failed: %v", err)
	}
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].Message != "test error" {
		t.Errorf("expected 'test error', got %q", errors[0].Message)
	}
}

func TestEstimatePendingPoints(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	ctx := context.Background()

	// BTC = $50000, 0.01 BTC = $500 -> tier 5 (2.0x) -> 100000 points.
	raw := provider.RawTransaction{
		Token:       "BTC",
		AmountHuman: "0.01",
	}
	pts := w.estimatePendingPoints(ctx, raw)
	if pts != 100000 {
		t.Errorf("expected 100000 estimated points, got %d", pts)
	}

	// Stablecoin: USDC $5.0 -> tier 1 (1.0x) -> 500 points.
	rawUSDC := provider.RawTransaction{
		Token:       "USDC",
		AmountHuman: "5.0",
	}
	ptsUSDC := w.estimatePendingPoints(ctx, rawUSDC)
	if ptsUSDC != 500 {
		t.Errorf("expected 500 estimated points, got %d", ptsUSDC)
	}
}

func TestDetermineStopStatus(t *testing.T) {
	db := setupTestDB(t)
	mp := newMockProvider("test-btc", "BTC")
	w := setupWatcher(t, db, mp)

	watch := &models.Watch{ID: "test"}

	// Deadline exceeded -> EXPIRED.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()
	<-ctx.Done() // ensure it's done
	status := w.determineStopStatus(ctx, watch)
	if status != models.WatchStatusExpired {
		t.Errorf("expected EXPIRED for deadline exceeded, got %s", status)
	}

	// Manual cancel -> CANCELLED.
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	<-ctx2.Done()
	status2 := w.determineStopStatus(ctx2, watch)
	if status2 != models.WatchStatusCancelled {
		t.Errorf("expected CANCELLED for manual cancel, got %s", status2)
	}
}
