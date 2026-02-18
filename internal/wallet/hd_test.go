package wallet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

// Standard BIP-39 test mnemonic (12-word — used for basic validation testing).
const testMnemonic12 = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

// Standard BIP-39 test mnemonic (24-word — primary test vector).
const testMnemonic24 = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"

func TestValidateMnemonic(t *testing.T) {
	tests := []struct {
		name     string
		mnemonic string
		wantErr  bool
	}{
		{
			name:     "valid 24-word mnemonic",
			mnemonic: testMnemonic24,
			wantErr:  false,
		},
		{
			name:     "invalid — 12 words rejected",
			mnemonic: testMnemonic12,
			wantErr:  true,
		},
		{
			name:     "invalid — empty",
			mnemonic: "",
			wantErr:  true,
		},
		{
			name:     "invalid — wrong words",
			mnemonic: "hello world foo bar baz qux quux corge grault garply waldo fred plugh xyzzy thud foo bar baz qux quux corge grault garply waldo",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMnemonic(tt.mnemonic)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMnemonic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMnemonicToSeed(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatalf("MnemonicToSeed() error = %v", err)
	}

	if len(seed) != 64 {
		t.Errorf("MnemonicToSeed() seed length = %d, want 64", len(seed))
	}

	// Same mnemonic should produce the same seed.
	seed2, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatalf("MnemonicToSeed() second call error = %v", err)
	}

	for i := range seed {
		if seed[i] != seed2[i] {
			t.Fatalf("MnemonicToSeed() seed not deterministic at byte %d", i)
		}
	}
}

func TestReadMnemonicFromFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid file", func(t *testing.T) {
		path := filepath.Join(dir, "valid.txt")
		if err := os.WriteFile(path, []byte(testMnemonic24+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		mnemonic, err := ReadMnemonicFromFile(path)
		if err != nil {
			t.Fatalf("ReadMnemonicFromFile() error = %v", err)
		}

		if mnemonic != testMnemonic24 {
			t.Errorf("ReadMnemonicFromFile() = %q, want %q", mnemonic, testMnemonic24)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(dir, "empty.txt")
		if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := ReadMnemonicFromFile(path)
		if err == nil {
			t.Error("ReadMnemonicFromFile() expected error for empty file")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := ReadMnemonicFromFile(filepath.Join(dir, "nonexistent.txt"))
		if err == nil {
			t.Error("ReadMnemonicFromFile() expected error for missing file")
		}
	})

	t.Run("file with extra whitespace", func(t *testing.T) {
		path := filepath.Join(dir, "whitespace.txt")
		if err := os.WriteFile(path, []byte("  "+testMnemonic24+"  \n\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		mnemonic, err := ReadMnemonicFromFile(path)
		if err != nil {
			t.Fatalf("ReadMnemonicFromFile() error = %v", err)
		}

		if mnemonic != testMnemonic24 {
			t.Errorf("ReadMnemonicFromFile() = %q, want trimmed mnemonic", mnemonic)
		}
	})
}

func TestDeriveMasterKey(t *testing.T) {
	seed, err := MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}

	key, err := DeriveMasterKey(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("DeriveMasterKey() error = %v", err)
	}

	if key == nil {
		t.Fatal("DeriveMasterKey() returned nil key")
	}

	// Master key should be private.
	if !key.IsPrivate() {
		t.Error("DeriveMasterKey() returned non-private key")
	}
}

func TestNetworkParams(t *testing.T) {
	if p := NetworkParams("mainnet"); p != &chaincfg.MainNetParams {
		t.Error("NetworkParams(mainnet) did not return MainNetParams")
	}

	if p := NetworkParams("testnet"); p != &chaincfg.TestNet3Params {
		t.Error("NetworkParams(testnet) did not return TestNet3Params")
	}

	if p := NetworkParams("anything"); p != &chaincfg.MainNetParams {
		t.Error("NetworkParams(unknown) did not default to MainNetParams")
	}
}
