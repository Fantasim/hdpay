package tx

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sort"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/mr-tron/base58"
)

// SolPublicKey is a 32-byte Solana public key.
type SolPublicKey [32]byte

// SolSignature is a 64-byte ed25519 signature.
type SolSignature [64]byte

// SolPublicKeyFromBase58 decodes a base58-encoded Solana address into a SolPublicKey.
func SolPublicKeyFromBase58(addr string) (SolPublicKey, error) {
	b, err := base58.Decode(addr)
	if err != nil {
		return SolPublicKey{}, fmt.Errorf("decode base58 address %q: %w", addr, err)
	}
	if len(b) != 32 {
		return SolPublicKey{}, fmt.Errorf("invalid public key length %d, expected 32", len(b))
	}
	var pk SolPublicKey
	copy(pk[:], b)
	return pk, nil
}

// ToBase58 returns the base58 string representation of the public key.
func (pk SolPublicKey) ToBase58() string {
	return base58.Encode(pk[:])
}

// IsZero returns true if the public key is all zeros.
func (pk SolPublicKey) IsZero() bool {
	return pk == SolPublicKey{}
}

// SolAccountMeta describes the role of an account in an instruction.
type SolAccountMeta struct {
	PubKey     SolPublicKey
	IsSigner   bool
	IsWritable bool
}

// SolInstruction is a high-level Solana instruction before compilation.
type SolInstruction struct {
	ProgramID SolPublicKey
	Accounts  []SolAccountMeta
	Data      []byte
}

// SolCompiledInstruction is the compiled form using indexes into the account keys array.
type SolCompiledInstruction struct {
	ProgramIDIndex uint8
	AccountIndexes []uint8
	Data           []byte
}

// SolMessageHeader is the 3-byte header of a Solana message.
type SolMessageHeader struct {
	NumRequiredSignatures       uint8
	NumReadonlySignedAccounts   uint8
	NumReadonlyUnsignedAccounts uint8
}

// SolMessage is a compiled Solana transaction message (legacy format).
type SolMessage struct {
	Header          SolMessageHeader
	AccountKeys     []SolPublicKey
	RecentBlockhash [32]byte
	Instructions    []SolCompiledInstruction
}

// SolTransaction is a fully signed Solana transaction ready for serialization.
type SolTransaction struct {
	Signatures []SolSignature
	Message    SolMessage
}

// Well-known Solana program IDs (parsed once at init).
var (
	solSystemProgramID          SolPublicKey
	solTokenProgramID           SolPublicKey
	solAssociatedTokenProgramID SolPublicKey
	solRentSysvarID             SolPublicKey
)

func init() {
	var err error
	solSystemProgramID, err = SolPublicKeyFromBase58(config.SOLSystemProgramID)
	if err != nil {
		panic("invalid system program ID: " + err.Error())
	}
	solTokenProgramID, err = SolPublicKeyFromBase58(config.SOLTokenProgramID)
	if err != nil {
		panic("invalid token program ID: " + err.Error())
	}
	solAssociatedTokenProgramID, err = SolPublicKeyFromBase58(config.SOLAssociatedTokenProgramID)
	if err != nil {
		panic("invalid associated token program ID: " + err.Error())
	}
	solRentSysvarID, err = SolPublicKeyFromBase58(config.SOLRentSysvarID)
	if err != nil {
		panic("invalid rent sysvar ID: " + err.Error())
	}
}

// EncodeCompactU16 encodes an integer as Solana's compact-u16 variable-length format.
func EncodeCompactU16(buf *bytes.Buffer, val int) error {
	if val < 0 || val > 65535 {
		return fmt.Errorf("compact-u16 value out of range: %d", val)
	}
	rem := val
	for {
		elem := uint8(rem & 0x7f)
		rem >>= 7
		if rem == 0 {
			buf.WriteByte(elem)
			break
		}
		elem |= 0x80
		buf.WriteByte(elem)
	}
	return nil
}

// BuildSystemTransferInstruction creates a SystemProgram.Transfer instruction.
// Data: [u32 LE: 2 (Transfer variant)] [u64 LE: lamports] = 12 bytes.
func BuildSystemTransferInstruction(from, to SolPublicKey, lamports uint64) SolInstruction {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], 2) // Transfer = variant index 2
	binary.LittleEndian.PutUint64(data[4:12], lamports)

	return SolInstruction{
		ProgramID: solSystemProgramID,
		Accounts: []SolAccountMeta{
			{PubKey: from, IsSigner: true, IsWritable: true},
			{PubKey: to, IsSigner: false, IsWritable: true},
		},
		Data: data,
	}
}

// BuildSPLTransferInstruction creates an SPL Token.Transfer instruction.
// Data: [u8: 3 (Transfer variant)] [u64 LE: amount] = 9 bytes.
func BuildSPLTransferInstruction(sourceATA, destATA, owner SolPublicKey, amount uint64) SolInstruction {
	data := make([]byte, 9)
	data[0] = 3 // Transfer = variant index 3
	binary.LittleEndian.PutUint64(data[1:9], amount)

	return SolInstruction{
		ProgramID: solTokenProgramID,
		Accounts: []SolAccountMeta{
			{PubKey: sourceATA, IsSigner: false, IsWritable: true},
			{PubKey: destATA, IsSigner: false, IsWritable: true},
			{PubKey: owner, IsSigner: true, IsWritable: false},
		},
		Data: data,
	}
}

// BuildCreateATAInstruction creates a CreateAssociatedTokenAccount instruction.
// Data: empty (0 bytes). Accounts: payer, ata, wallet, mint, system, token, rent (7 accounts).
func BuildCreateATAInstruction(payer, ata, wallet, mint SolPublicKey) SolInstruction {
	return SolInstruction{
		ProgramID: solAssociatedTokenProgramID,
		Accounts: []SolAccountMeta{
			{PubKey: payer, IsSigner: true, IsWritable: true},
			{PubKey: ata, IsSigner: false, IsWritable: true},
			{PubKey: wallet, IsSigner: false, IsWritable: false},
			{PubKey: mint, IsSigner: false, IsWritable: false},
			{PubKey: solSystemProgramID, IsSigner: false, IsWritable: false},
			{PubKey: solTokenProgramID, IsSigner: false, IsWritable: false},
			{PubKey: solRentSysvarID, IsSigner: false, IsWritable: false},
		},
		Data: nil,
	}
}

// accountEntry tracks an account's role during message compilation.
type accountEntry struct {
	pubKey     SolPublicKey
	isSigner   bool
	isWritable bool
}

// CompileMessage compiles high-level instructions into a Solana message.
// The fee payer is always placed at index 0 as writable + signer.
// Accounts are ordered: writable+signer, readonly+signer, writable+nonsigner, readonly+nonsigner.
func CompileMessage(feePayer SolPublicKey, instructions []SolInstruction, recentBlockhash [32]byte) (SolMessage, error) {
	if len(instructions) == 0 {
		return SolMessage{}, fmt.Errorf("no instructions provided")
	}

	// Collect all unique accounts and merge their permissions.
	accountMap := make(map[SolPublicKey]*accountEntry)

	// Fee payer is always writable + signer.
	accountMap[feePayer] = &accountEntry{
		pubKey:     feePayer,
		isSigner:   true,
		isWritable: true,
	}

	for _, ix := range instructions {
		// Program ID is a readonly, non-signer account.
		if _, exists := accountMap[ix.ProgramID]; !exists {
			accountMap[ix.ProgramID] = &accountEntry{
				pubKey:     ix.ProgramID,
				isSigner:   false,
				isWritable: false,
			}
		}

		for _, acc := range ix.Accounts {
			if entry, exists := accountMap[acc.PubKey]; exists {
				// Merge: upgrade to signer/writable if any instruction requires it.
				if acc.IsSigner {
					entry.isSigner = true
				}
				if acc.IsWritable {
					entry.isWritable = true
				}
			} else {
				accountMap[acc.PubKey] = &accountEntry{
					pubKey:     acc.PubKey,
					isSigner:   acc.IsSigner,
					isWritable: acc.IsWritable,
				}
			}
		}
	}

	// Sort into four privilege groups.
	var writableSigners, readonlySigners, writableNonSigners, readonlyNonSigners []accountEntry
	for _, entry := range accountMap {
		if entry.pubKey == feePayer {
			continue // fee payer handled separately
		}
		switch {
		case entry.isSigner && entry.isWritable:
			writableSigners = append(writableSigners, *entry)
		case entry.isSigner && !entry.isWritable:
			readonlySigners = append(readonlySigners, *entry)
		case !entry.isSigner && entry.isWritable:
			writableNonSigners = append(writableNonSigners, *entry)
		default:
			readonlyNonSigners = append(readonlyNonSigners, *entry)
		}
	}

	// Sort each group by base58 for deterministic ordering.
	sortByBase58 := func(a []accountEntry) {
		sort.Slice(a, func(i, j int) bool {
			return a[i].pubKey.ToBase58() < a[j].pubKey.ToBase58()
		})
	}
	sortByBase58(writableSigners)
	sortByBase58(readonlySigners)
	sortByBase58(writableNonSigners)
	sortByBase58(readonlyNonSigners)

	// Build ordered account keys: fee payer first, then groups.
	accountKeys := make([]SolPublicKey, 0, len(accountMap))
	accountKeys = append(accountKeys, feePayer)
	for _, e := range writableSigners {
		accountKeys = append(accountKeys, e.pubKey)
	}
	for _, e := range readonlySigners {
		accountKeys = append(accountKeys, e.pubKey)
	}
	for _, e := range writableNonSigners {
		accountKeys = append(accountKeys, e.pubKey)
	}
	for _, e := range readonlyNonSigners {
		accountKeys = append(accountKeys, e.pubKey)
	}

	// Build index lookup.
	keyIndex := make(map[SolPublicKey]uint8, len(accountKeys))
	for i, k := range accountKeys {
		keyIndex[k] = uint8(i)
	}

	// Compute header counts.
	numSigners := uint8(1 + len(writableSigners) + len(readonlySigners)) // fee payer + other signers
	numReadonlySigned := uint8(len(readonlySigners))
	numReadonlyUnsigned := uint8(len(readonlyNonSigners))

	// Compile instructions.
	compiledInstructions := make([]SolCompiledInstruction, len(instructions))
	for i, ix := range instructions {
		progIdx, ok := keyIndex[ix.ProgramID]
		if !ok {
			return SolMessage{}, fmt.Errorf("program ID %s not found in account keys", ix.ProgramID.ToBase58())
		}

		accountIdxs := make([]uint8, len(ix.Accounts))
		for j, acc := range ix.Accounts {
			idx, ok := keyIndex[acc.PubKey]
			if !ok {
				return SolMessage{}, fmt.Errorf("account %s not found in account keys", acc.PubKey.ToBase58())
			}
			accountIdxs[j] = idx
		}

		compiledInstructions[i] = SolCompiledInstruction{
			ProgramIDIndex: progIdx,
			AccountIndexes: accountIdxs,
			Data:           ix.Data,
		}
	}

	msg := SolMessage{
		Header: SolMessageHeader{
			NumRequiredSignatures:       numSigners,
			NumReadonlySignedAccounts:   numReadonlySigned,
			NumReadonlyUnsignedAccounts: numReadonlyUnsigned,
		},
		AccountKeys:     accountKeys,
		RecentBlockhash: recentBlockhash,
		Instructions:    compiledInstructions,
	}

	slog.Debug("compiled SOL message",
		"accountCount", len(accountKeys),
		"signerCount", numSigners,
		"instructionCount", len(compiledInstructions),
	)

	return msg, nil
}

// SerializeMessage serializes a SolMessage into bytes (the part that gets signed).
func SerializeMessage(msg SolMessage) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Header (3 bytes, no length prefix).
	buf.WriteByte(msg.Header.NumRequiredSignatures)
	buf.WriteByte(msg.Header.NumReadonlySignedAccounts)
	buf.WriteByte(msg.Header.NumReadonlyUnsignedAccounts)

	// Account keys (compact-u16 count + 32 bytes each).
	if err := EncodeCompactU16(buf, len(msg.AccountKeys)); err != nil {
		return nil, fmt.Errorf("encode account key count: %w", err)
	}
	for _, k := range msg.AccountKeys {
		buf.Write(k[:])
	}

	// Recent blockhash (32 bytes, no prefix).
	buf.Write(msg.RecentBlockhash[:])

	// Instructions (compact-u16 count + each compiled instruction).
	if err := EncodeCompactU16(buf, len(msg.Instructions)); err != nil {
		return nil, fmt.Errorf("encode instruction count: %w", err)
	}
	for _, ix := range msg.Instructions {
		buf.WriteByte(ix.ProgramIDIndex)

		if err := EncodeCompactU16(buf, len(ix.AccountIndexes)); err != nil {
			return nil, fmt.Errorf("encode account index count: %w", err)
		}
		for _, idx := range ix.AccountIndexes {
			buf.WriteByte(idx)
		}

		dataLen := 0
		if ix.Data != nil {
			dataLen = len(ix.Data)
		}
		if err := EncodeCompactU16(buf, dataLen); err != nil {
			return nil, fmt.Errorf("encode instruction data length: %w", err)
		}
		if dataLen > 0 {
			buf.Write(ix.Data)
		}
	}

	return buf.Bytes(), nil
}

// SerializeTransaction serializes a full SolTransaction into the wire format.
func SerializeTransaction(tx SolTransaction) ([]byte, error) {
	msgBytes, err := SerializeMessage(tx.Message)
	if err != nil {
		return nil, fmt.Errorf("serialize message: %w", err)
	}

	buf := new(bytes.Buffer)

	// Signatures (compact-u16 count + 64 bytes each).
	if err := EncodeCompactU16(buf, len(tx.Signatures)); err != nil {
		return nil, fmt.Errorf("encode signature count: %w", err)
	}
	for _, sig := range tx.Signatures {
		buf.Write(sig[:])
	}

	// Append serialized message.
	buf.Write(msgBytes)

	return buf.Bytes(), nil
}

// SignTransaction signs the message with the provided private keys.
// The signers map keys on public key. Each of the first NumRequiredSignatures account keys
// must have a corresponding private key.
func SignTransaction(msg SolMessage, msgBytes []byte, signers map[SolPublicKey]ed25519.PrivateKey) (SolTransaction, error) {
	numSigs := int(msg.Header.NumRequiredSignatures)
	signatures := make([]SolSignature, numSigs)

	for i := 0; i < numSigs; i++ {
		pubKey := msg.AccountKeys[i]
		privKey, ok := signers[pubKey]
		if !ok {
			return SolTransaction{}, fmt.Errorf("missing signer for account %s (index %d)", pubKey.ToBase58(), i)
		}

		sig := ed25519.Sign(privKey, msgBytes)
		if len(sig) != 64 {
			return SolTransaction{}, fmt.Errorf("unexpected signature length %d for account %s", len(sig), pubKey.ToBase58())
		}
		copy(signatures[i][:], sig)
	}

	slog.Debug("signed SOL transaction",
		"signerCount", numSigs,
	)

	return SolTransaction{
		Signatures: signatures,
		Message:    msg,
	}, nil
}

// BuildAndSerializeTransaction is a convenience function that compiles, serializes, signs,
// and returns the final transaction bytes + the first signer's signature (transaction ID).
func BuildAndSerializeTransaction(
	feePayer SolPublicKey,
	instructions []SolInstruction,
	recentBlockhash [32]byte,
	signers map[SolPublicKey]ed25519.PrivateKey,
) (txBytes []byte, txSignature string, err error) {
	msg, err := CompileMessage(feePayer, instructions, recentBlockhash)
	if err != nil {
		return nil, "", fmt.Errorf("compile message: %w", err)
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		return nil, "", fmt.Errorf("serialize message: %w", err)
	}

	tx, err := SignTransaction(msg, msgBytes, signers)
	if err != nil {
		return nil, "", fmt.Errorf("sign transaction: %w", err)
	}

	txBytes, err = SerializeTransaction(tx)
	if err != nil {
		return nil, "", fmt.Errorf("serialize transaction: %w", err)
	}

	if len(txBytes) > config.SOLMaxTxSize {
		return nil, "", fmt.Errorf("%w: %d bytes (max %d)", config.ErrSOLTxTooLarge, len(txBytes), config.SOLMaxTxSize)
	}

	// The transaction ID is the base58 encoding of the first signature.
	txSignature = base58.Encode(tx.Signatures[0][:])

	slog.Debug("built SOL transaction",
		"size", len(txBytes),
		"signature", txSignature,
	)

	return txBytes, txSignature, nil
}
