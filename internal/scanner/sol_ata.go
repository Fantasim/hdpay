package scanner

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/mr-tron/base58"
)

// Edwards25519 constants for on-curve check.
var (
	edwardsP = func() *big.Int {
		p := new(big.Int).Lsh(big.NewInt(1), 255)
		p.Sub(p, big.NewInt(19))
		return p
	}()
	edwardsD = func() *big.Int {
		// d = -121665 * inverse(121666) mod p
		num := big.NewInt(-121665)
		den := big.NewInt(121666)
		denInv := new(big.Int).ModInverse(den, edwardsP)
		d := new(big.Int).Mul(num, denInv)
		d.Mod(d, edwardsP)
		return d
	}()
	bigOne = big.NewInt(1)
)

// DeriveATA computes the Associated Token Account address for a wallet and token mint.
// Uses standard PDA derivation: seeds = [wallet, TOKEN_PROGRAM_ID, mint],
// program = ASSOCIATED_TOKEN_PROGRAM_ID.
func DeriveATA(walletAddress, mintAddress string) (string, error) {
	wallet, err := base58.Decode(walletAddress)
	if err != nil {
		return "", fmt.Errorf("decode wallet address: %w", err)
	}
	if len(wallet) != 32 {
		return "", fmt.Errorf("invalid wallet address length: %d", len(wallet))
	}

	mint, err := base58.Decode(mintAddress)
	if err != nil {
		return "", fmt.Errorf("decode mint address: %w", err)
	}
	if len(mint) != 32 {
		return "", fmt.Errorf("invalid mint address length: %d", len(mint))
	}

	tokenProgram, err := base58.Decode(config.SOLTokenProgramID)
	if err != nil {
		return "", fmt.Errorf("decode token program ID: %w", err)
	}

	associatedProgram, err := base58.Decode(config.SOLAssociatedTokenProgramID)
	if err != nil {
		return "", fmt.Errorf("decode associated token program ID: %w", err)
	}

	seeds := [][]byte{wallet, tokenProgram, mint}

	pda, err := findProgramAddress(seeds, associatedProgram)
	if err != nil {
		return "", fmt.Errorf("find program address: %w", err)
	}

	return base58.Encode(pda), nil
}

// findProgramAddress derives a Program Derived Address (PDA) from seeds and a program ID.
// Tries bump seeds 255 down to 0, returning the first valid PDA (not on the ed25519 curve).
func findProgramAddress(seeds [][]byte, programID []byte) ([]byte, error) {
	for bump := byte(255); ; bump-- {
		candidate := deriveAddress(seeds, bump, programID)

		// A valid PDA must NOT be on the ed25519 curve.
		if !isOnCurve(candidate) {
			return candidate, nil
		}

		if bump == 0 {
			break
		}
	}

	return nil, fmt.Errorf("could not find valid PDA")
}

// deriveAddress computes SHA-256(seed1 + seed2 + ... + bump + programID + "ProgramDerivedAddress").
func deriveAddress(seeds [][]byte, bump byte, programID []byte) []byte {
	h := sha256.New()
	for _, seed := range seeds {
		h.Write(seed)
	}
	h.Write([]byte{bump})
	h.Write(programID)
	h.Write([]byte("ProgramDerivedAddress"))
	return h.Sum(nil)
}

// isOnCurve checks if 32 bytes represent a valid Edwards25519 curve point.
// PDA addresses must NOT be on the curve.
func isOnCurve(key []byte) bool {
	if len(key) != 32 {
		return false
	}

	// Extract y-coordinate (clear the sign bit in the high byte).
	yBytes := make([]byte, 32)
	copy(yBytes, key)
	xSign := yBytes[31] >> 7
	yBytes[31] &= 0x7f

	// Convert y from little-endian bytes to big.Int.
	y := littleEndianToBigInt(yBytes)

	// y must be < p
	if y.Cmp(edwardsP) >= 0 {
		return false
	}

	// Edwards curve: -x^2 + y^2 = 1 + d*x^2*y^2
	// Solving for x^2: x^2 = (y^2 - 1) / (d*y^2 + 1) mod p

	// y^2 mod p
	y2 := new(big.Int).Mul(y, y)
	y2.Mod(y2, edwardsP)

	// u = y^2 - 1 mod p
	u := new(big.Int).Sub(y2, bigOne)
	u.Mod(u, edwardsP)
	if u.Sign() < 0 {
		u.Add(u, edwardsP)
	}

	// v = d*y^2 + 1 mod p
	v := new(big.Int).Mul(edwardsD, y2)
	v.Mod(v, edwardsP)
	v.Add(v, bigOne)
	v.Mod(v, edwardsP)

	// v^(-1) mod p
	vInv := new(big.Int).ModInverse(v, edwardsP)
	if vInv == nil {
		return false
	}

	// x^2 = u * v^(-1) mod p
	x2 := new(big.Int).Mul(u, vInv)
	x2.Mod(x2, edwardsP)

	// If x^2 == 0, x == 0 (valid only if sign bit is 0)
	if x2.Sign() == 0 {
		return xSign == 0
	}

	// Check if x^2 is a quadratic residue mod p using Euler's criterion.
	// x^2 is a QR if (x^2)^((p-1)/2) ≡ 1 mod p
	exp := new(big.Int).Sub(edwardsP, bigOne)
	exp.Rsh(exp, 1) // (p-1)/2

	check := new(big.Int).Exp(x2, exp, edwardsP)
	return check.Cmp(bigOne) == 0
}

// littleEndianToBigInt converts a little-endian byte slice to a big.Int.
func littleEndianToBigInt(b []byte) *big.Int {
	// Reverse to big-endian for big.Int.
	reversed := make([]byte, len(b))
	for i, v := range b {
		reversed[len(b)-1-i] = v
	}
	return new(big.Int).SetBytes(reversed)
}
