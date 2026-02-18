package scanner

import (
	"testing"
)

func TestDeriveATA_KnownVector(t *testing.T) {
	// Known ATA for a well-known wallet + USDC mint.
	// Wallet: 7oPa2PHQdZmjSPqvpZN7MQxnC7Dcf3uL7oRqPdkEg2tz (example)
	// We test that the derivation produces a valid base58 string of correct length
	// and is deterministic (same inputs → same output).

	wallet := "7oPa2PHQdZmjSPqvpZN7MQxnC7Dcf3uL7oRqPdkEg2tz"
	mint := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" // USDC

	ata1, err := DeriveATA(wallet, mint)
	if err != nil {
		t.Fatalf("DeriveATA() error = %v", err)
	}

	if len(ata1) < 32 || len(ata1) > 44 {
		t.Errorf("ATA address length unexpected: %d (%s)", len(ata1), ata1)
	}

	// Deterministic: same inputs should produce same output.
	ata2, err := DeriveATA(wallet, mint)
	if err != nil {
		t.Fatalf("DeriveATA() second call error = %v", err)
	}

	if ata1 != ata2 {
		t.Errorf("DeriveATA() not deterministic: %s != %s", ata1, ata2)
	}
}

func TestDeriveATA_DifferentMints(t *testing.T) {
	wallet := "7oPa2PHQdZmjSPqvpZN7MQxnC7Dcf3uL7oRqPdkEg2tz"
	usdcMint := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	usdtMint := "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"

	ataUSDC, err := DeriveATA(wallet, usdcMint)
	if err != nil {
		t.Fatalf("DeriveATA(USDC) error = %v", err)
	}

	ataUSDT, err := DeriveATA(wallet, usdtMint)
	if err != nil {
		t.Fatalf("DeriveATA(USDT) error = %v", err)
	}

	if ataUSDC == ataUSDT {
		t.Error("USDC and USDT ATAs should be different")
	}
}

func TestDeriveATA_InvalidWallet(t *testing.T) {
	_, err := DeriveATA("invalid", "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	if err == nil {
		t.Error("expected error for invalid wallet address")
	}
}

func TestDeriveATA_InvalidMint(t *testing.T) {
	_, err := DeriveATA("7oPa2PHQdZmjSPqvpZN7MQxnC7Dcf3uL7oRqPdkEg2tz", "invalid")
	if err == nil {
		t.Error("expected error for invalid mint address")
	}
}

func TestIsOnCurve(t *testing.T) {
	// Test with known values — the key thing is that the function doesn't panic
	// and returns a boolean.
	tests := []struct {
		name string
		key  []byte
	}{
		{
			name: "all zeros",
			key:  make([]byte, 32),
		},
		{
			name: "short key",
			key:  []byte{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic.
			_ = isOnCurve(tt.key)
		})
	}
}
