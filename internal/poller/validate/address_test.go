package validate

import (
	"testing"
)

func TestAddress_BTC_Valid(t *testing.T) {
	tests := []struct {
		name    string
		address string
		network string
	}{
		{"testnet bech32 index 0", "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr", "testnet"},
		{"testnet bech32 index 1", "tb1qgadxe2kacxtw44un284vskrn6w2xgsmm7h2hfg", "testnet"},
		{"testnet bech32 index 2", "tb1qkmq5vclvgp022zg00r6w8k36s9nnysge5a5m83", "testnet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Address("BTC", tt.address, tt.network); err != nil {
				t.Errorf("Address(BTC, %s, %s) error = %v", tt.address, tt.network, err)
			}
		})
	}
}

func TestAddress_BTC_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		address string
		network string
	}{
		{"empty", "", "mainnet"},
		{"garbage", "notanaddress", "mainnet"},
		{"testnet on mainnet", "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr", "mainnet"},
		{"wrong checksum", "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t5", "mainnet"}, // modified last char
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Address("BTC", tt.address, tt.network); err == nil {
				t.Errorf("Address(BTC, %s, %s) should fail", tt.address, tt.network)
			}
		})
	}
}

func TestAddress_BTC_UnsupportedNetwork(t *testing.T) {
	if err := Address("BTC", "bc1qexample", "regtest"); err == nil {
		t.Error("should fail for unsupported network")
	}
}

func TestAddress_BSC_Valid(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"testnet index 0", "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb"},
		{"testnet index 1", "0xf785bD075874b8423D3583728a981399f31e95aA"},
		{"testnet index 2", "0x60Af1c6A5D03F9f1B1b74931499bC99E72fF8DA9"},
		{"lowercase", "0xf278cf59f82edcf871d630f28ecc8056f25c1cdb"},
		{"uppercase", "0xF278CF59F82EDCF871D630F28ECC8056F25C1CDB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Address("BSC", tt.address, "mainnet"); err != nil {
				t.Errorf("Address(BSC, %s) error = %v", tt.address, err)
			}
		})
	}
}

func TestAddress_BSC_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"empty", ""},
		{"no prefix", "F278cF59F82eDcf871d630F28EcC8056f25C1cdb"},
		{"too short", "0xF278cF59F82eDcf871d630F28EcC8056f25C1cd"},
		{"too long", "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb0"},
		{"invalid hex char", "0xG278cF59F82eDcf871d630F28EcC8056f25C1cdb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Address("BSC", tt.address, "mainnet"); err == nil {
				t.Errorf("Address(BSC, %s) should fail", tt.address)
			}
		})
	}
}

func TestAddress_SOL_Valid(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"devnet index 0", "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx"},
		{"devnet index 1", "5frqxtii9LeGq2bz3dSNokvZcEooF483MzeU24JrhcTA"},
		{"devnet index 2", "3SuKj3MZU9dMZ9oR1R7afttihZFkWpfUmduuv9rmfMa1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Address("SOL", tt.address, "mainnet"); err != nil {
				t.Errorf("Address(SOL, %s) error = %v", tt.address, err)
			}
		})
	}
}

func TestAddress_SOL_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"empty", ""},
		{"too short base58", "abc"},
		{"invalid base58 char O", "OOOOOOOOOOOOOOO"},
		{"invalid base58 char 0", "0x0000000000000000000000000000000000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Address("SOL", tt.address, "mainnet"); err == nil {
				t.Errorf("Address(SOL, %s) should fail", tt.address)
			}
		})
	}
}

func TestAddress_UnsupportedChain(t *testing.T) {
	if err := Address("ETH", "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb", "mainnet"); err == nil {
		t.Error("should fail for unsupported chain")
	}
}

func TestAddress_BSC_NetworkIndependent(t *testing.T) {
	// BSC addresses are the same format for both networks.
	addr := "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb"
	for _, net := range []string{"mainnet", "testnet"} {
		if err := Address("BSC", addr, net); err != nil {
			t.Errorf("BSC address should be valid on %s, got error = %v", net, err)
		}
	}
}

func TestAddress_SOL_NetworkIndependent(t *testing.T) {
	// SOL addresses are the same format for all networks.
	addr := "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx"
	for _, net := range []string{"mainnet", "testnet"} {
		if err := Address("SOL", addr, net); err != nil {
			t.Errorf("SOL address should be valid on %s, got error = %v", net, err)
		}
	}
}
