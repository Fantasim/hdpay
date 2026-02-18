package wallet

import (
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestDeriveBSCAddressKnownVector(t *testing.T) {
	// The well-known test vector uses the 12-word mnemonic.
	seed, err := MnemonicToSeed(testMnemonic12)
	if err != nil {
		t.Fatal(err)
	}

	masterKey, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	got, err := DeriveBSCAddress(masterKey, 0)
	if err != nil {
		t.Fatalf("DeriveBSCAddress() error = %v", err)
	}

	want := "0x9858EfFD232B4033E47d90003D41EC34EcaEda94"
	if got != want {
		t.Errorf("DeriveBSCAddress(12-word, index 0) = %v, want %v", got, want)
	}
}

func TestDeriveBSCAddress(t *testing.T) {
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
			got, err := DeriveBSCAddress(masterKey, i)
			if err != nil {
				t.Fatalf("DeriveBSCAddress() error = %v", err)
			}

			if !strings.HasPrefix(got, "0x") {
				t.Errorf("DeriveBSCAddress() = %v, want 0x prefix", got)
			}

			if len(got) != 42 {
				t.Errorf("DeriveBSCAddress() length = %d, want 42", len(got))
			}

			if addresses[got] {
				t.Errorf("DeriveBSCAddress() duplicate address: %v", got)
			}
			addresses[got] = true
		})
	}
}

func TestDeriveBSCAddressEIP55Checksum(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	masterKey, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	addr, err := DeriveBSCAddress(masterKey, 0)
	if err != nil {
		t.Fatal(err)
	}

	// EIP-55 checksum: address should NOT be all lowercase or all uppercase after 0x.
	hexPart := addr[2:]
	allLower := strings.ToLower(hexPart) == hexPart
	allUpper := strings.ToUpper(hexPart) == hexPart

	if allLower || allUpper {
		t.Errorf("DeriveBSCAddress() address %v is not EIP-55 checksummed (all lower: %v, all upper: %v)",
			addr, allLower, allUpper)
	}
}

func TestDeriveBSCAddressDeterministic(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	masterKey1, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	addr1, err := DeriveBSCAddress(masterKey1, 99)
	if err != nil {
		t.Fatal(err)
	}

	masterKey2, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	addr2, err := DeriveBSCAddress(masterKey2, 99)
	if err != nil {
		t.Fatal(err)
	}

	if addr1 != addr2 {
		t.Errorf("DeriveBSCAddress() not deterministic: %v != %v", addr1, addr2)
	}
}
