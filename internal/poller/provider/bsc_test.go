package provider

import (
	"testing"

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
		{"invalid", 18, "0"},
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
