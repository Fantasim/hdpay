package wallet

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"

	"github.com/Fantasim/hdpay/internal/models"
)

func TestGenerateBTCAddresses(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	masterKey, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	var progressCalls int
	progress := func(chain models.Chain, generated int, total int) {
		progressCalls++
	}

	// Generate with count < 10000 so no progress callback fires.
	addresses, err := GenerateBTCAddresses(masterKey, 5, &chaincfg.MainNetParams, progress)
	if err != nil {
		t.Fatalf("GenerateBTCAddresses() error = %v", err)
	}

	if len(addresses) != 5 {
		t.Errorf("GenerateBTCAddresses() count = %d, want 5", len(addresses))
	}

	for i, addr := range addresses {
		if addr.Chain != models.ChainBTC {
			t.Errorf("address[%d].Chain = %v, want BTC", i, addr.Chain)
		}
		if addr.AddressIndex != i {
			t.Errorf("address[%d].AddressIndex = %d, want %d", i, addr.AddressIndex, i)
		}
		if !strings.HasPrefix(addr.Address, "bc1q") {
			t.Errorf("address[%d].Address = %v, want bc1q prefix", i, addr.Address)
		}
	}

	if progressCalls != 0 {
		t.Errorf("progress called %d times, want 0 (count < 10000)", progressCalls)
	}
}

func TestGenerateBSCAddresses(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	masterKey, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	addresses, err := GenerateBSCAddresses(masterKey, 5, nil)
	if err != nil {
		t.Fatalf("GenerateBSCAddresses() error = %v", err)
	}

	if len(addresses) != 5 {
		t.Errorf("GenerateBSCAddresses() count = %d, want 5", len(addresses))
	}

	for i, addr := range addresses {
		if addr.Chain != models.ChainBSC {
			t.Errorf("address[%d].Chain = %v, want BSC", i, addr.Chain)
		}
		if !strings.HasPrefix(addr.Address, "0x") {
			t.Errorf("address[%d].Address = %v, want 0x prefix", i, addr.Address)
		}
	}
}

func TestGenerateSOLAddresses(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	addresses, err := GenerateSOLAddresses(seed, 5, nil)
	if err != nil {
		t.Fatalf("GenerateSOLAddresses() error = %v", err)
	}

	if len(addresses) != 5 {
		t.Errorf("GenerateSOLAddresses() count = %d, want 5", len(addresses))
	}

	for i, addr := range addresses {
		if addr.Chain != models.ChainSOL {
			t.Errorf("address[%d].Chain = %v, want SOL", i, addr.Chain)
		}
		if len(addr.Address) < 32 || len(addr.Address) > 44 {
			t.Errorf("address[%d].Address length = %d, want 32-44", i, len(addr.Address))
		}
	}
}
