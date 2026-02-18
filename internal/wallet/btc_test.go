package wallet

import (
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestDeriveBTCAddressKnownVector(t *testing.T) {
	// The well-known test vector uses the 12-word mnemonic.
	seed, err := MnemonicToSeed(testMnemonic12)
	if err != nil {
		t.Fatal(err)
	}

	masterKey, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	got, err := DeriveBTCAddress(masterKey, 0, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("DeriveBTCAddress() error = %v", err)
	}

	want := "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu"
	if got != want {
		t.Errorf("DeriveBTCAddress(12-word, index 0) = %v, want %v", got, want)
	}
}

func TestDeriveBTCAddress(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	masterKey, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	addresses := make(map[string]bool)

	for i := uint32(0); i < 5; i++ {
		t.Run(fmt.Sprintf("index_%d", i), func(t *testing.T) {
			got, err := DeriveBTCAddress(masterKey, i, &chaincfg.MainNetParams)
			if err != nil {
				t.Fatalf("DeriveBTCAddress() error = %v", err)
			}

			if !strings.HasPrefix(got, "bc1q") {
				t.Errorf("DeriveBTCAddress() = %v, want prefix bc1q", got)
			}

			if addresses[got] {
				t.Errorf("DeriveBTCAddress() duplicate address: %v", got)
			}
			addresses[got] = true
		})
	}
}

func TestDeriveBTCAddressTestnet(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	testnetKey, err := DeriveMasterKey(seed, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}

	addr, err := DeriveBTCAddress(testnetKey, 0, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatalf("DeriveBTCAddress(testnet) error = %v", err)
	}

	if !strings.HasPrefix(addr, "tb1q") {
		t.Errorf("DeriveBTCAddress(testnet) = %v, want prefix tb1q", addr)
	}

	// Testnet address should differ from mainnet.
	mainnetKey, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	mainnetAddr, err := DeriveBTCAddress(mainnetKey, 0, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	if addr == mainnetAddr {
		t.Error("testnet and mainnet addresses should differ")
	}
}

func TestDeriveBTCAddressDeterministic(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	masterKey, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	addr1, err := DeriveBTCAddress(masterKey, 42, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	// Re-derive master key and derive same index again.
	masterKey2, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	addr2, err := DeriveBTCAddress(masterKey2, 42, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	if addr1 != addr2 {
		t.Errorf("DeriveBTCAddress() not deterministic: %v != %v", addr1, addr2)
	}
}
