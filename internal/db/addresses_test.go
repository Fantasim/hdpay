package db

import (
	"path/filepath"
	"testing"

	"github.com/Fantasim/hdpay/internal/models"
)

// setupTestDB creates a temporary database with migrations applied.
func setupTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	t.Cleanup(func() { d.Close() })
	return d
}

// seedAddresses inserts a set of test addresses.
func seedAddresses(t *testing.T, d *DB, chain models.Chain, count int) {
	t.Helper()
	addresses := make([]models.Address, count)
	for i := 0; i < count; i++ {
		addresses[i] = models.Address{
			Chain:        chain,
			AddressIndex: i,
			Address:      "addr_" + string(chain) + "_" + itoa(i),
		}
	}
	if err := d.InsertAddressBatch(chain, addresses); err != nil {
		t.Fatalf("InsertAddressBatch() error = %v", err)
	}
}

func itoa(n int) string {
	s := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func TestGetAddressesWithBalances_BasicPagination(t *testing.T) {
	d := setupTestDB(t)
	seedAddresses(t, d, models.ChainBTC, 25)

	tests := []struct {
		name       string
		page       int
		pageSize   int
		wantCount  int
		wantTotal  int64
		wantFirst  int
	}{
		{"page 1 of 3", 1, 10, 10, 25, 0},
		{"page 2 of 3", 2, 10, 10, 25, 10},
		{"page 3 of 3", 3, 10, 5, 25, 20},
		{"beyond last page", 4, 10, 0, 25, -1},
		{"single page", 1, 100, 25, 25, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total, err := d.GetAddressesWithBalances(AddressFilter{
				Chain:    models.ChainBTC,
				Page:     tt.page,
				PageSize: tt.pageSize,
			})
			if err != nil {
				t.Fatalf("GetAddressesWithBalances() error = %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
			if len(results) != tt.wantCount {
				t.Errorf("len(results) = %d, want %d", len(results), tt.wantCount)
			}
			if tt.wantFirst >= 0 && len(results) > 0 {
				if results[0].AddressIndex != tt.wantFirst {
					t.Errorf("first index = %d, want %d", results[0].AddressIndex, tt.wantFirst)
				}
			}
		})
	}
}

func TestGetAddressesWithBalances_HasBalanceFilter(t *testing.T) {
	d := setupTestDB(t)
	seedAddresses(t, d, models.ChainBSC, 10)

	// Insert balances for some addresses
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance) VALUES (?, ?, ?, ?, ?)",
		"BSC", "testnet", 2, "NATIVE", "1000000000000000000")
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance) VALUES (?, ?, ?, ?, ?)",
		"BSC", "testnet", 5, "NATIVE", "500000000000000000")
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance) VALUES (?, ?, ?, ?, ?)",
		"BSC", "testnet", 7, "USDC", "1000000")

	// Without filter — all addresses
	results, total, err := d.GetAddressesWithBalances(AddressFilter{
		Chain:    models.ChainBSC,
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("GetAddressesWithBalances() error = %v", err)
	}
	if total != 10 {
		t.Errorf("total without filter = %d, want 10", total)
	}
	if len(results) != 10 {
		t.Errorf("len(results) without filter = %d, want 10", len(results))
	}

	// With hasBalance filter — only funded
	results, total, err = d.GetAddressesWithBalances(AddressFilter{
		Chain:      models.ChainBSC,
		Page:       1,
		PageSize:   100,
		HasBalance: true,
	})
	if err != nil {
		t.Fatalf("GetAddressesWithBalances(hasBalance) error = %v", err)
	}
	if total != 3 {
		t.Errorf("total with hasBalance = %d, want 3", total)
	}
	if len(results) != 3 {
		t.Errorf("len(results) with hasBalance = %d, want 3", len(results))
	}
}

func TestGetAddressesWithBalances_TokenFilter(t *testing.T) {
	d := setupTestDB(t)
	seedAddresses(t, d, models.ChainSOL, 10)

	// Insert balances
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance) VALUES (?, ?, ?, ?, ?)",
		"SOL", "testnet", 0, "NATIVE", "5000000000")
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance) VALUES (?, ?, ?, ?, ?)",
		"SOL", "testnet", 0, "USDC", "1000000")
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance) VALUES (?, ?, ?, ?, ?)",
		"SOL", "testnet", 3, "USDT", "5000000")
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance) VALUES (?, ?, ?, ?, ?)",
		"SOL", "testnet", 5, "USDC", "2500000")

	// Filter by USDC
	results, total, err := d.GetAddressesWithBalances(AddressFilter{
		Chain:    models.ChainSOL,
		Page:     1,
		PageSize: 100,
		Token:    "USDC",
	})
	if err != nil {
		t.Fatalf("GetAddressesWithBalances(USDC) error = %v", err)
	}
	if total != 2 {
		t.Errorf("total USDC = %d, want 2", total)
	}
	if len(results) != 2 {
		t.Errorf("len USDC = %d, want 2", len(results))
	}

	// Filter by NATIVE
	results, total, err = d.GetAddressesWithBalances(AddressFilter{
		Chain:    models.ChainSOL,
		Page:     1,
		PageSize: 100,
		Token:    "NATIVE",
	})
	if err != nil {
		t.Fatalf("GetAddressesWithBalances(NATIVE) error = %v", err)
	}
	if total != 1 {
		t.Errorf("total NATIVE = %d, want 1", total)
	}
}

func TestGetAddressesWithBalances_HydratesBalances(t *testing.T) {
	d := setupTestDB(t)
	seedAddresses(t, d, models.ChainBSC, 5)

	// Insert balances for index 2
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance, last_scanned) VALUES (?, ?, ?, ?, ?, ?)",
		"BSC", "testnet", 2, "NATIVE", "1000000000000000000", "2026-02-18 10:00:00")
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance, last_scanned) VALUES (?, ?, ?, ?, ?, ?)",
		"BSC", "testnet", 2, "USDC", "5000000", "2026-02-18 10:00:00")
	d.Conn().Exec("INSERT INTO balances (chain, network, address_index, token, balance, last_scanned) VALUES (?, ?, ?, ?, ?, ?)",
		"BSC", "testnet", 2, "USDT", "3000000", "2026-02-18 10:00:00")

	results, _, err := d.GetAddressesWithBalances(AddressFilter{
		Chain:    models.ChainBSC,
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	// Index 2 should have balances
	addr2 := results[2]
	if addr2.NativeBalance != "1000000000000000000" {
		t.Errorf("native balance = %q, want 1000000000000000000", addr2.NativeBalance)
	}
	if len(addr2.TokenBalances) != 2 {
		t.Errorf("token balances count = %d, want 2", len(addr2.TokenBalances))
	}
	if addr2.LastScanned == nil {
		t.Error("expected lastScanned to be set")
	}

	// Index 0 should have zero native and empty tokens
	addr0 := results[0]
	if addr0.NativeBalance != "0" {
		t.Errorf("index 0 native balance = %q, want 0", addr0.NativeBalance)
	}
	if len(addr0.TokenBalances) != 0 {
		t.Errorf("index 0 token balances = %d, want 0", len(addr0.TokenBalances))
	}
	if addr0.LastScanned != nil {
		t.Error("index 0 lastScanned should be nil")
	}
}

func TestInsertAddressBatch_LargeBatch(t *testing.T) {
	d := setupTestDB(t)

	// 15,000 addresses crosses the 10K batch boundary, which previously caused
	// "too many SQL variables" with the multi-value INSERT approach.
	const count = 15_000
	addresses := make([]models.Address, count)
	for i := 0; i < count; i++ {
		addresses[i] = models.Address{
			Chain:        models.ChainBTC,
			AddressIndex: i,
			Address:      "bc1q_test_" + itoa(i),
		}
	}

	if err := d.InsertAddressBatch(models.ChainBTC, addresses); err != nil {
		t.Fatalf("InsertAddressBatch(15000) error = %v", err)
	}

	// Verify count.
	got, err := d.CountAddresses(models.ChainBTC)
	if err != nil {
		t.Fatalf("CountAddresses() error = %v", err)
	}
	if got != count {
		t.Errorf("CountAddresses() = %d, want %d", got, count)
	}

	// Verify boundary addresses survive correctly.
	for _, idx := range []int{0, 9_999, 10_000, 14_999} {
		addr, err := d.GetAddressByIndex(models.ChainBTC, idx)
		if err != nil {
			t.Fatalf("GetAddressByIndex(%d) error = %v", idx, err)
		}
		want := "bc1q_test_" + itoa(idx)
		if addr.Address != want {
			t.Errorf("address at index %d = %q, want %q", idx, addr.Address, want)
		}
	}
}

func TestGetAddressesWithBalances_EmptyChain(t *testing.T) {
	d := setupTestDB(t)

	results, total, err := d.GetAddressesWithBalances(AddressFilter{
		Chain:    models.ChainBTC,
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if results != nil {
		t.Errorf("results = %v, want nil", results)
	}
}
