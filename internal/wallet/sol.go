package wallet

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"log/slog"

	"github.com/mr-tron/base58"
)

const (
	slip10Curve    = "ed25519 seed"
	hardenedOffset = uint32(0x80000000)
)

// slip10Key holds a SLIP-10 ed25519 key pair (private key seed + chain code).
type slip10Key struct {
	key       []byte // 32 bytes — raw ed25519 seed
	chainCode []byte // 32 bytes
}

// DeriveSOLAddress derives a Solana address using SLIP-10 ed25519 at the given account index.
// Path: m/44'/501'/N'/0' (all hardened, Phantom/Solflare standard).
func DeriveSOLAddress(seed []byte, index uint32) (string, error) {
	// SLIP-10 master key: HMAC-SHA512(Key="ed25519 seed", Data=BIP39 seed)
	mac := hmac.New(sha512.New, []byte(slip10Curve))
	mac.Write(seed)
	I := mac.Sum(nil)

	master := slip10Key{
		key:       I[:32],
		chainCode: I[32:],
	}

	// Derive path m/44'/501'/index'/0'
	segments := []uint32{
		44 + hardenedOffset,
		501 + hardenedOffset,
		index + hardenedOffset,
		0 + hardenedOffset,
	}

	current := master
	for _, seg := range segments {
		current = slip10DeriveChild(current, seg)
	}

	// ed25519 public key from 32-byte seed
	privKey := ed25519.NewKeyFromSeed(current.key)
	pubKey := privKey.Public().(ed25519.PublicKey)

	addr := base58.Encode(pubKey)

	slog.Debug("derived SOL address",
		"index", index,
		"address", addr,
	)

	return addr, nil
}

// DeriveSOLPrivateKey derives the ed25519 private key for a Solana address at the given index.
// Used only during transaction signing — caller must discard the key immediately after use.
func DeriveSOLPrivateKey(seed []byte, index uint32) (ed25519.PrivateKey, error) {
	mac := hmac.New(sha512.New, []byte(slip10Curve))
	mac.Write(seed)
	I := mac.Sum(nil)

	master := slip10Key{
		key:       I[:32],
		chainCode: I[32:],
	}

	segments := []uint32{
		44 + hardenedOffset,
		501 + hardenedOffset,
		index + hardenedOffset,
		0 + hardenedOffset,
	}

	current := master
	for _, seg := range segments {
		current = slip10DeriveChild(current, seg)
	}

	privKey := ed25519.NewKeyFromSeed(current.key)

	slog.Debug("derived SOL private key for signing", "index", index)
	return privKey, nil
}

// slip10DeriveChild performs SLIP-10 hardened child key derivation for ed25519.
// data = 0x00 || parent_key (32 bytes) || index (4 bytes big-endian)
func slip10DeriveChild(parent slip10Key, index uint32) slip10Key {
	data := make([]byte, 0, 37) // 1 + 32 + 4
	data = append(data, 0x00)
	data = append(data, parent.key...)

	var indexBytes [4]byte
	binary.BigEndian.PutUint32(indexBytes[:], index)
	data = append(data, indexBytes[:]...)

	mac := hmac.New(sha512.New, parent.chainCode)
	mac.Write(data)
	I := mac.Sum(nil)

	return slip10Key{
		key:       I[:32],
		chainCode: I[32:],
	}
}

// slip10MasterKeyFromSeed generates a SLIP-10 master key from a BIP-39 seed.
// Exported for testing against SLIP-10 spec test vectors.
func slip10MasterKeyFromSeed(seed []byte) (privateKey []byte, chainCode []byte) {
	mac := hmac.New(sha512.New, []byte(slip10Curve))
	mac.Write(seed)
	I := mac.Sum(nil)
	return I[:32], I[32:]
}

// slip10DeriveChildFromRaw derives a child key from raw key + chain code at the given index.
// Exported for testing against SLIP-10 spec test vectors.
func slip10DeriveChildFromRaw(key, chainCode []byte, index uint32) (childKey []byte, childChainCode []byte) {
	parent := slip10Key{key: key, chainCode: chainCode}
	child := slip10DeriveChild(parent, index)
	return child.key, child.chainCode
}

// formatDerivationPathSOL returns the derivation path string for logging.
func formatDerivationPathSOL(index uint32) string {
	return fmt.Sprintf("m/44'/501'/%d'/0'", index)
}
