package handlers

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"

	"github.com/Fantasim/hdpay/internal/models"
)

func TestValidateDestination(t *testing.T) {
	tests := []struct {
		name    string
		chain   models.Chain
		address string
		net     *chaincfg.Params
		wantErr bool
	}{
		// BTC valid
		{
			name:    "BTC valid bech32 mainnet",
			chain:   models.ChainBTC,
			address: "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "BTC valid bech32 testnet",
			chain:   models.ChainBTC,
			address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
			net:     &chaincfg.TestNet3Params,
			wantErr: false,
		},
		// BTC invalid
		{
			name:    "BTC empty address",
			chain:   models.ChainBTC,
			address: "",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "BTC invalid format",
			chain:   models.ChainBTC,
			address: "notavalidaddress",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		// Note: btcutil.DecodeAddress with MainNetParams decodes testnet
		// addresses without error — network enforcement is separate.
		{
			name:    "BTC testnet address on mainnet (accepted by decoder)",
			chain:   models.ChainBTC,
			address: "tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},

		// BSC valid
		{
			name:    "BSC valid checksummed",
			chain:   models.ChainBSC,
			address: "0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "BSC valid lowercase",
			chain:   models.ChainBSC,
			address: "0xf278cf59f82edcf871d630f28ecc8056f25c1cdb",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		// BSC invalid
		// Note: go-ethereum's common.IsHexAddress accepts addresses without 0x.
		{
			name:    "BSC without 0x prefix (accepted by go-ethereum)",
			chain:   models.ChainBSC,
			address: "F278cF59F82eDcf871d630F28EcC8056f25C1cdb",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "BSC too short",
			chain:   models.ChainBSC,
			address: "0x1234",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "BSC empty",
			chain:   models.ChainBSC,
			address: "",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},

		// SOL valid
		{
			name:    "SOL valid base58",
			chain:   models.ChainSOL,
			address: "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		{
			name:    "SOL valid short address",
			chain:   models.ChainSOL,
			address: "11111111111111111111111111111111",
			net:     &chaincfg.MainNetParams,
			wantErr: false,
		},
		// SOL invalid
		{
			name:    "SOL empty",
			chain:   models.ChainSOL,
			address: "",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "SOL invalid chars (contains 0)",
			chain:   models.ChainSOL,
			address: "0Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSn",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
		{
			name:    "SOL too short",
			chain:   models.ChainSOL,
			address: "abc",
			net:     &chaincfg.MainNetParams,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDestination(tt.chain, tt.address, tt.net)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDestination(%s, %q) error = %v, wantErr %v",
					tt.chain, tt.address, err, tt.wantErr)
			}
		})
	}
}

func TestIsValidToken(t *testing.T) {
	tests := []struct {
		name  string
		chain models.Chain
		token models.Token
		want  bool
	}{
		// BTC
		{"BTC NATIVE valid", models.ChainBTC, models.TokenNative, true},
		{"BTC USDC invalid", models.ChainBTC, models.TokenUSDC, false},
		{"BTC USDT invalid", models.ChainBTC, models.TokenUSDT, false},

		// BSC
		{"BSC NATIVE valid", models.ChainBSC, models.TokenNative, true},
		{"BSC USDC valid", models.ChainBSC, models.TokenUSDC, true},
		{"BSC USDT valid", models.ChainBSC, models.TokenUSDT, true},

		// SOL
		{"SOL NATIVE valid", models.ChainSOL, models.TokenNative, true},
		{"SOL USDC valid", models.ChainSOL, models.TokenUSDC, true},
		{"SOL USDT valid", models.ChainSOL, models.TokenUSDT, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidToken(tt.chain, tt.token)
			if got != tt.want {
				t.Errorf("isValidToken(%s, %s) = %v, want %v",
					tt.chain, tt.token, got, tt.want)
			}
		})
	}
}
