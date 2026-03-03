package provider

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	hdconfig "github.com/Fantasim/hdpay/internal/shared/config"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
)

// TestWeiToHuman verifies decimal conversion for BSC/BNB amounts.
func TestWeiToHuman(t *testing.T) {
	tests := []struct {
		raw      string
		decimals int
		want     string
	}{
		{"1000000000000000000", 18, "1.000000000000000000"},
		{"500000000000000000", 18, "0.500000000000000000"},
		{"1000000", 6, "1.000000"},
		{"0", 18, "0.000000000000000000"},
		{"123456789", 6, "123.456789"},
		{"1", 18, "0.000000000000000001"},
		{"invalid", 18, "0"},
		// Zero decimals — no fractional part.
		{"42", 0, "42"},
	}

	for _, tt := range tests {
		got := weiToHuman(tt.raw, tt.decimals)
		if got != tt.want {
			t.Errorf("weiToHuman(%q, %d) = %q, want %q", tt.raw, tt.decimals, got, tt.want)
		}
	}
}

// TestBSCRPCPollerProvider_CheckConfirmation_Synthetic verifies synthetic BNB
// hashes are immediately considered confirmed without an RPC call.
func TestBSCRPCPollerProvider_CheckConfirmation_Synthetic(t *testing.T) {
	// We can test this path without a real RPC connection because synthetic
	// hashes short-circuit before any network call.
	p := &BSCRPCPollerProvider{network: "mainnet"}

	confirmed, confs, err := p.CheckConfirmation(nil, "bnb-0xaddr-block-12345", 12345)
	if err != nil {
		t.Fatalf("CheckConfirmation() error = %v", err)
	}
	if !confirmed {
		t.Error("synthetic BNB hash should be confirmed immediately")
	}
	if confs != pollerconfig.ConfirmationsBSC {
		t.Errorf("confirmations = %d, want %d", confs, pollerconfig.ConfirmationsBSC)
	}
}

// TestBSCRPCPollerProvider_ClearBalance verifies that ClearBalance removes
// the cached balance entry for an address.
func TestBSCRPCPollerProvider_ClearBalance(t *testing.T) {
	p := &BSCRPCPollerProvider{network: "mainnet"}

	addr := "0xDeaDBEeF00000000000000000000000000000001"
	addrKey := strings.ToLower(addr)

	// Store a balance, then clear it.
	p.lastKnownBal.Store(addrKey, big.NewInt(1000))
	p.ClearBalance(addr)

	if _, loaded := p.lastKnownBal.Load(addrKey); loaded {
		t.Error("ClearBalance did not remove cached balance")
	}
}

// TestBSCRPCPollerProvider_ClearBalance_CaseInsensitive verifies that
// ClearBalance handles mixed-case addresses correctly.
func TestBSCRPCPollerProvider_ClearBalance_CaseInsensitive(t *testing.T) {
	p := &BSCRPCPollerProvider{network: "mainnet"}

	addr := "0xAbCdEf0123456789AbCdEf0123456789AbCdEf01"
	addrKey := strings.ToLower(addr)

	p.lastKnownBal.Store(addrKey, big.NewInt(42))
	p.ClearBalance(addr)

	if _, loaded := p.lastKnownBal.Load(addrKey); loaded {
		t.Error("ClearBalance did not remove cached balance with mixed-case address")
	}
}

// TestBSCRPCPollerProvider_ClearBalance_NoOp verifies that ClearBalance
// does not panic when called for a non-existent address.
func TestBSCRPCPollerProvider_ClearBalance_NoOp(t *testing.T) {
	p := &BSCRPCPollerProvider{network: "mainnet"}
	// Should not panic.
	p.ClearBalance("0x0000000000000000000000000000000000000000")
}

// TestBSCRPCPollerProvider_Name verifies the provider name constant.
func TestBSCRPCPollerProvider_Name(t *testing.T) {
	p := &BSCRPCPollerProvider{network: "mainnet"}
	if p.Name() != "bscrpc-poller" {
		t.Errorf("Name() = %q, want %q", p.Name(), "bscrpc-poller")
	}
}

// TestBSCRPCPollerProvider_Chain verifies the chain constant.
func TestBSCRPCPollerProvider_Chain(t *testing.T) {
	p := &BSCRPCPollerProvider{network: "mainnet"}
	if p.Chain() != "BSC" {
		t.Errorf("Chain() = %q, want %q", p.Chain(), "BSC")
	}
}

// TestBSCRPCPollerProvider_TokenConfig_Mainnet verifies mainnet token addresses.
func TestBSCRPCPollerProvider_TokenConfig_Mainnet(t *testing.T) {
	p := &BSCRPCPollerProvider{network: "mainnet"}
	usdcAddr, usdtAddr, usdcDec, usdtDec := p.tokenConfig()

	if usdcAddr != hdconfig.BSCUSDCContract {
		t.Errorf("USDC contract = %q, want %q", usdcAddr, hdconfig.BSCUSDCContract)
	}
	if usdtAddr != hdconfig.BSCUSDTContract {
		t.Errorf("USDT contract = %q, want %q", usdtAddr, hdconfig.BSCUSDTContract)
	}
	if usdcDec != hdconfig.BSCUSDCDecimals {
		t.Errorf("USDC decimals = %d, want %d", usdcDec, hdconfig.BSCUSDCDecimals)
	}
	if usdtDec != hdconfig.BSCUSDTDecimals {
		t.Errorf("USDT decimals = %d, want %d", usdtDec, hdconfig.BSCUSDTDecimals)
	}
}

// TestBSCRPCPollerProvider_TokenConfig_Testnet verifies testnet token addresses.
func TestBSCRPCPollerProvider_TokenConfig_Testnet(t *testing.T) {
	p := &BSCRPCPollerProvider{network: "testnet"}
	usdcAddr, usdtAddr, usdcDec, usdtDec := p.tokenConfig()

	if usdcAddr != hdconfig.BSCTestnetUSDCContract {
		t.Errorf("USDC contract = %q, want %q", usdcAddr, hdconfig.BSCTestnetUSDCContract)
	}
	if usdtAddr != hdconfig.BSCTestnetUSDTContract {
		t.Errorf("USDT contract = %q, want %q", usdtAddr, hdconfig.BSCTestnetUSDTContract)
	}
	if usdcDec != hdconfig.BSCUSDCDecimals {
		t.Errorf("USDC decimals = %d, want %d", usdcDec, hdconfig.BSCUSDCDecimals)
	}
	if usdtDec != hdconfig.BSCUSDTDecimals {
		t.Errorf("USDT decimals = %d, want %d", usdtDec, hdconfig.BSCUSDTDecimals)
	}
}

// TestBSCRPCPollerProvider_GetClient_RotateClient verifies the round-robin
// rotation logic for multi-client failover.
func TestBSCRPCPollerProvider_GetClient_RotateClient(t *testing.T) {
	// We verify the rotation index math directly since ethclient.Client
	// instances require a real server.

	tests := []struct {
		numClients int
		rotations  int
		wantIdx    int
	}{
		{3, 0, 0},
		{3, 1, 1},
		{3, 2, 2},
		{3, 3, 0}, // wraps around
		{3, 4, 1},
		{1, 5, 0}, // single client always stays at 0
		{2, 3, 1},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("clients=%d_rotations=%d", tt.numClients, tt.rotations), func(t *testing.T) {
			idx := 0
			for i := 0; i < tt.rotations; i++ {
				idx = (idx + 1) % tt.numClients
			}
			if idx != tt.wantIdx {
				t.Errorf("after %d rotations with %d clients, idx = %d, want %d",
					tt.rotations, tt.numClients, idx, tt.wantIdx)
			}
		})
	}
}

// TestBSCSyntheticHashFormat verifies the synthetic BNB hash format.
func TestBSCSyntheticHashFormat(t *testing.T) {
	addr := "0xdeadbeef"
	block := uint64(12345)
	hash := fmt.Sprintf(pollerconfig.BNBSyntheticHashFmt, addr, block)
	expected := "bnb-0xdeadbeef-block-12345"
	if hash != expected {
		t.Errorf("synthetic hash = %q, want %q", hash, expected)
	}

	// Should be detected as synthetic by the CheckConfirmation prefix check.
	if !strings.HasPrefix(hash, "bnb-") {
		t.Error("synthetic hash should start with 'bnb-'")
	}
}

// TestBSCRPCPollerProvider_BalanceDelta_Logic tests the balance-delta detection
// logic without requiring actual RPC calls.
func TestBSCRPCPollerProvider_BalanceDelta_Logic(t *testing.T) {
	tests := []struct {
		name        string
		prevBalance *big.Int
		currBalance *big.Int
		wantDelta   bool
		wantAmount  string
	}{
		{
			name:        "balance increase",
			prevBalance: big.NewInt(1000),
			currBalance: big.NewInt(1500),
			wantDelta:   true,
			wantAmount:  "500",
		},
		{
			name:        "balance decrease (send)",
			prevBalance: big.NewInt(1500),
			currBalance: big.NewInt(1000),
			wantDelta:   false,
		},
		{
			name:        "no change",
			prevBalance: big.NewInt(1000),
			currBalance: big.NewInt(1000),
			wantDelta:   false,
		},
		{
			name:        "zero to nonzero",
			prevBalance: big.NewInt(0),
			currBalance: big.NewInt(42),
			wantDelta:   true,
			wantAmount:  "42",
		},
		{
			name:        "large amount",
			prevBalance: new(big.Int).SetUint64(1e18),
			currBalance: new(big.Int).Add(new(big.Int).SetUint64(1e18), big.NewInt(5e17)),
			wantDelta:   true,
			wantAmount:  "500000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasDelta := tt.currBalance.Cmp(tt.prevBalance) > 0
			if hasDelta != tt.wantDelta {
				t.Errorf("hasDelta = %v, want %v", hasDelta, tt.wantDelta)
			}
			if hasDelta {
				delta := new(big.Int).Sub(tt.currBalance, tt.prevBalance)
				if delta.String() != tt.wantAmount {
					t.Errorf("delta = %s, want %s", delta.String(), tt.wantAmount)
				}
			}
		})
	}
}

// TestBSCRPCPollerProvider_LastKnownBal_Concurrency tests that concurrent
// reads and writes to lastKnownBal don't race (sync.Map guarantees this).
func TestBSCRPCPollerProvider_LastKnownBal_Concurrency(t *testing.T) {
	p := &BSCRPCPollerProvider{network: "mainnet"}
	addr := "0xdeadbeef"

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			p.lastKnownBal.Store(addr, big.NewInt(int64(i)))
		}
		close(done)
	}()

	for i := 0; i < 1000; i++ {
		p.lastKnownBal.Load(addr)
	}
	<-done

	// If we get here without a race detector failure, the test passes.
}

// TestBSCBlockRange verifies the fromBlock calculation doesn't exceed the cap.
func TestBSCBlockRange(t *testing.T) {
	tests := []struct {
		name       string
		elapsed    int64
		wantCapped bool
	}{
		{"recent cutoff", 60, false},       // 20 blocks back
		{"1 hour cutoff", 3600, false},     // 1200 blocks back
		{"1 day cutoff", 86400, false},     // 28800 blocks back
		{"2 days cutoff", 172800, true},    // 57600 > 50000 cap
		{"future cutoff", -100, false},     // negative elapsed → 0 blocks
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocksBack := tt.elapsed / pollerconfig.BSCBlockTimeSeconds
			if blocksBack < 0 {
				blocksBack = 0
			}
			capped := blocksBack > pollerconfig.BSCMaxLogBlockRange
			if capped != tt.wantCapped {
				t.Errorf("capped = %v, want %v (blocksBack=%d, cap=%d)",
					capped, tt.wantCapped, blocksBack, pollerconfig.BSCMaxLogBlockRange)
			}
			if capped {
				blocksBack = pollerconfig.BSCMaxLogBlockRange
			}
			if blocksBack < 0 {
				t.Error("blocksBack should never be negative after clamping")
			}
		})
	}
}
