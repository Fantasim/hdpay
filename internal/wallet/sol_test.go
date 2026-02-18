package wallet

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/mr-tron/base58"
)

func base58Encode(data []byte) string {
	return base58.Encode(data)
}

func TestSLIP10MasterKey(t *testing.T) {
	// Official SLIP-10 test vector 1: seed = 000102030405060708090a0b0c0d0e0f
	seed, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")

	privateKey, chainCode := slip10MasterKeyFromSeed(seed)

	wantKey := "2b4be7f19ee27bbf30c667b642d5f4aa69fd169872f8fc3059c08ebae2eb19e7"
	wantCC := "90046a93de5380a72b5e45010748567d5ea02bbf6522f979e05c0d8d8ca9fffb"

	gotKey := hex.EncodeToString(privateKey)
	gotCC := hex.EncodeToString(chainCode)

	if gotKey != wantKey {
		t.Errorf("master key = %s, want %s", gotKey, wantKey)
	}
	if gotCC != wantCC {
		t.Errorf("master chain code = %s, want %s", gotCC, wantCC)
	}
}

func TestSLIP10ChildDerivation(t *testing.T) {
	// Official SLIP-10 test vector 1: chain m/0'
	seed, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	masterKey, masterCC := slip10MasterKeyFromSeed(seed)

	childKey, childCC := slip10DeriveChildFromRaw(masterKey, masterCC, 0x80000000) // 0' = hardened 0

	wantKey := "68e0fe46dfb67e368c75379acec591dad19df3cde26e63b93a8e704f1dade7a3"
	wantCC := "8b59aa11380b624e81507a27fedda59fea6d0b779a778918a2fd3590e16e9c69"

	gotKey := hex.EncodeToString(childKey)
	gotCC := hex.EncodeToString(childCC)

	if gotKey != wantKey {
		t.Errorf("child m/0' key = %s, want %s", gotKey, wantKey)
	}
	if gotCC != wantCC {
		t.Errorf("child m/0' chain code = %s, want %s", gotCC, wantCC)
	}

	// Continue to m/0'/1'
	childKey2, childCC2 := slip10DeriveChildFromRaw(childKey, childCC, 0x80000001) // 1'

	wantKey2 := "b1d0bad404bf35da785a64ca1ac54b2617211d2777696fbffaf208f746ae84f2"
	wantCC2 := "a320425f77d1b5c2505a6b1b27382b37368ee640e3557c315416801243552f14"

	gotKey2 := hex.EncodeToString(childKey2)
	gotCC2 := hex.EncodeToString(childCC2)

	if gotKey2 != wantKey2 {
		t.Errorf("child m/0'/1' key = %s, want %s", gotKey2, wantKey2)
	}
	if gotCC2 != wantCC2 {
		t.Errorf("child m/0'/1' chain code = %s, want %s", gotCC2, wantCC2)
	}
}

func TestDeriveSOLAddressKnownVector(t *testing.T) {
	// 12-word "abandon...about" mnemonic at m/44'/501'/0'/0'
	seed, err := MnemonicToSeed(testMnemonic12)
	if err != nil {
		t.Fatal(err)
	}

	got, err := DeriveSOLAddress(seed, 0)
	if err != nil {
		t.Fatalf("DeriveSOLAddress() error = %v", err)
	}

	// Verified against Node.js SLIP-10 implementation (crypto + ed25519 seed).
	want := "HAgk14JpMQLgt6rVgv7cBQFJWFto5Dqxi472uT3DKpqk"
	if got != want {
		t.Errorf("DeriveSOLAddress(12-word, index 0) = %v, want %v", got, want)
	}
}

func TestDeriveSOLAddress(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	addresses := make(map[string]bool)

	for i := uint32(0); i < 5; i++ {
		t.Run(fmt.Sprintf("index_%d", i), func(t *testing.T) {
			got, err := DeriveSOLAddress(seed, i)
			if err != nil {
				t.Fatalf("DeriveSOLAddress() error = %v", err)
			}

			// Solana addresses are Base58-encoded 32-byte public keys (32-44 chars).
			if len(got) < 32 || len(got) > 44 {
				t.Errorf("DeriveSOLAddress() address length = %d, want 32-44", len(got))
			}

			if addresses[got] {
				t.Errorf("DeriveSOLAddress() duplicate address: %v", got)
			}
			addresses[got] = true
		})
	}
}

func TestDeriveSOLAddressDeterministic(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	addr1, err := DeriveSOLAddress(seed, 42)
	if err != nil {
		t.Fatal(err)
	}

	addr2, err := DeriveSOLAddress(seed, 42)
	if err != nil {
		t.Fatal(err)
	}

	if addr1 != addr2 {
		t.Errorf("DeriveSOLAddress() not deterministic: %v != %v", addr1, addr2)
	}
}

func TestDeriveSOLPrivateKey(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	privKey, err := DeriveSOLPrivateKey(seed, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Ed25519 private key should be 64 bytes.
	if len(privKey) != 64 {
		t.Errorf("DeriveSOLPrivateKey() key length = %d, want 64", len(privKey))
	}

	// Public key from private key should match DeriveSOLAddress output.
	addr, err := DeriveSOLAddress(seed, 0)
	if err != nil {
		t.Fatal(err)
	}

	// ed25519.PublicKey is just a []byte (32 bytes)
	pubKey := privKey[32:]
	addrFromPriv := base58Encode(pubKey)

	if addrFromPriv != addr {
		t.Errorf("DeriveSOLPrivateKey() public key address = %v, want %v", addrFromPriv, addr)
	}
}

func TestFormatDerivationPathSOL(t *testing.T) {
	if got := formatDerivationPathSOL(0); got != "m/44'/501'/0'/0'" {
		t.Errorf("formatDerivationPathSOL(0) = %v", got)
	}
	if got := formatDerivationPathSOL(42); got != "m/44'/501'/42'/0'" {
		t.Errorf("formatDerivationPathSOL(42) = %v", got)
	}
}
