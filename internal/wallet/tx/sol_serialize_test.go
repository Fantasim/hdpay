package tx

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
)

func TestEncodeCompactU16(t *testing.T) {
	tests := []struct {
		name  string
		val   int
		want  []byte
	}{
		{"zero", 0, []byte{0x00}},
		{"one", 1, []byte{0x01}},
		{"max_single_byte", 127, []byte{0x7f}},
		{"two_bytes_min", 128, []byte{0x80, 0x01}},
		{"255", 255, []byte{0xff, 0x01}},
		{"max_two_bytes", 16383, []byte{0xff, 0x7f}},
		{"three_bytes_min", 16384, []byte{0x80, 0x80, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			if err := EncodeCompactU16(buf, tt.val); err != nil {
				t.Fatalf("EncodeCompactU16(%d) error = %v", tt.val, err)
			}
			got := buf.Bytes()
			if !bytes.Equal(got, tt.want) {
				t.Errorf("EncodeCompactU16(%d) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestEncodeCompactU16_OutOfRange(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := EncodeCompactU16(buf, -1); err == nil {
		t.Error("expected error for negative value")
	}
	if err := EncodeCompactU16(buf, 65536); err == nil {
		t.Error("expected error for value > 65535")
	}
}

func TestSolPublicKeyFromBase58_RoundTrip(t *testing.T) {
	// Use a known Solana address.
	addr := "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx"
	pk, err := SolPublicKeyFromBase58(addr)
	if err != nil {
		t.Fatalf("SolPublicKeyFromBase58() error = %v", err)
	}

	got := pk.ToBase58()
	if got != addr {
		t.Errorf("round-trip: got %s, want %s", got, addr)
	}
}

func TestSolPublicKeyFromBase58_Invalid(t *testing.T) {
	// Invalid base58 characters.
	_, err := SolPublicKeyFromBase58("invalid!@#$")
	if err == nil {
		t.Error("expected error for invalid base58")
	}

	// Wrong length (too short).
	_, err = SolPublicKeyFromBase58("111111")
	if err == nil {
		t.Error("expected error for wrong length")
	}
}

func TestBuildSystemTransferInstruction(t *testing.T) {
	from := SolPublicKey{1} // dummy keys
	to := SolPublicKey{2}
	lamports := uint64(1_000_000_000) // 1 SOL

	ix := BuildSystemTransferInstruction(from, to, lamports)

	// Data should be 12 bytes: u32 LE (2) + u64 LE (lamports).
	if len(ix.Data) != 12 {
		t.Fatalf("SystemTransfer data length = %d, want 12", len(ix.Data))
	}

	variant := binary.LittleEndian.Uint32(ix.Data[0:4])
	if variant != 2 {
		t.Errorf("variant = %d, want 2", variant)
	}

	amount := binary.LittleEndian.Uint64(ix.Data[4:12])
	if amount != lamports {
		t.Errorf("lamports = %d, want %d", amount, lamports)
	}

	// Should have 2 accounts: from (signer+writable), to (writable).
	if len(ix.Accounts) != 2 {
		t.Fatalf("account count = %d, want 2", len(ix.Accounts))
	}
	if !ix.Accounts[0].IsSigner || !ix.Accounts[0].IsWritable {
		t.Error("from account should be signer+writable")
	}
	if ix.Accounts[1].IsSigner || !ix.Accounts[1].IsWritable {
		t.Error("to account should be writable, not signer")
	}

	// Program ID should be system program.
	if ix.ProgramID != solSystemProgramID {
		t.Errorf("program ID = %s, want system program", ix.ProgramID.ToBase58())
	}
}

func TestBuildSPLTransferInstruction(t *testing.T) {
	sourceATA := SolPublicKey{3}
	destATA := SolPublicKey{4}
	owner := SolPublicKey{5}
	amount := uint64(20_000_000) // 20 USDC (6 decimals)

	ix := BuildSPLTransferInstruction(sourceATA, destATA, owner, amount)

	// Data should be 9 bytes: u8 (3) + u64 LE (amount).
	if len(ix.Data) != 9 {
		t.Fatalf("SPLTransfer data length = %d, want 9", len(ix.Data))
	}

	if ix.Data[0] != 3 {
		t.Errorf("variant = %d, want 3", ix.Data[0])
	}

	parsedAmount := binary.LittleEndian.Uint64(ix.Data[1:9])
	if parsedAmount != amount {
		t.Errorf("amount = %d, want %d", parsedAmount, amount)
	}

	// Should have 3 accounts.
	if len(ix.Accounts) != 3 {
		t.Fatalf("account count = %d, want 3", len(ix.Accounts))
	}

	// sourceATA: writable, not signer.
	if ix.Accounts[0].IsSigner || !ix.Accounts[0].IsWritable {
		t.Error("sourceATA should be writable, not signer")
	}
	// destATA: writable, not signer.
	if ix.Accounts[1].IsSigner || !ix.Accounts[1].IsWritable {
		t.Error("destATA should be writable, not signer")
	}
	// owner: signer, not writable.
	if !ix.Accounts[2].IsSigner || ix.Accounts[2].IsWritable {
		t.Error("owner should be signer, not writable")
	}

	if ix.ProgramID != solTokenProgramID {
		t.Errorf("program ID = %s, want token program", ix.ProgramID.ToBase58())
	}
}

func TestBuildCreateATAInstruction(t *testing.T) {
	payer := SolPublicKey{1}
	ata := SolPublicKey{2}
	wallet := SolPublicKey{3}
	mint := SolPublicKey{4}

	ix := BuildCreateATAInstruction(payer, ata, wallet, mint)

	// Data should be empty.
	if len(ix.Data) != 0 {
		t.Fatalf("CreateATA data length = %d, want 0", len(ix.Data))
	}

	// Should have 7 accounts.
	if len(ix.Accounts) != 7 {
		t.Fatalf("account count = %d, want 7", len(ix.Accounts))
	}

	// Payer: writable + signer.
	if !ix.Accounts[0].IsSigner || !ix.Accounts[0].IsWritable {
		t.Error("payer should be signer+writable")
	}
	// ATA: writable, not signer.
	if ix.Accounts[1].IsSigner || !ix.Accounts[1].IsWritable {
		t.Error("ata should be writable, not signer")
	}
	// Wallet: readonly, not signer.
	if ix.Accounts[2].IsSigner || ix.Accounts[2].IsWritable {
		t.Error("wallet should be readonly, not signer")
	}

	if ix.ProgramID != solAssociatedTokenProgramID {
		t.Errorf("program ID = %s, want associated token program", ix.ProgramID.ToBase58())
	}
}

func TestCompileMessage_AccountOrdering(t *testing.T) {
	feePayer := SolPublicKey{1}
	dest := SolPublicKey{2}

	ix := BuildSystemTransferInstruction(feePayer, dest, 100)
	blockhash := [32]byte{0xab}

	msg, err := CompileMessage(feePayer, []SolInstruction{ix}, blockhash)
	if err != nil {
		t.Fatalf("CompileMessage error = %v", err)
	}

	// Fee payer should be at index 0.
	if msg.AccountKeys[0] != feePayer {
		t.Errorf("account[0] = %v, want fee payer", msg.AccountKeys[0])
	}

	// Header: 1 signer (fee payer), 0 readonly signed, system program + dest accounted.
	if msg.Header.NumRequiredSignatures != 1 {
		t.Errorf("numRequiredSignatures = %d, want 1", msg.Header.NumRequiredSignatures)
	}
}

func TestCompileMessage_Deduplication(t *testing.T) {
	feePayer := SolPublicKey{1}
	dest := SolPublicKey{2}

	// Two transfer instructions using the same fee payer and destination.
	ix1 := BuildSystemTransferInstruction(feePayer, dest, 100)
	ix2 := BuildSystemTransferInstruction(feePayer, dest, 200)

	msg, err := CompileMessage(feePayer, []SolInstruction{ix1, ix2}, [32]byte{})
	if err != nil {
		t.Fatalf("CompileMessage error = %v", err)
	}

	// Should have 3 accounts: feePayer, dest, system program.
	if len(msg.AccountKeys) != 3 {
		t.Errorf("account count = %d, want 3 (feePayer, dest, system program)", len(msg.AccountKeys))
	}

	// Should have 2 compiled instructions.
	if len(msg.Instructions) != 2 {
		t.Errorf("instruction count = %d, want 2", len(msg.Instructions))
	}
}

func TestSerializeMessage_Size(t *testing.T) {
	feePayer := SolPublicKey{1}
	dest := SolPublicKey{2}

	ix := BuildSystemTransferInstruction(feePayer, dest, 100)
	msg, err := CompileMessage(feePayer, []SolInstruction{ix}, [32]byte{})
	if err != nil {
		t.Fatalf("CompileMessage error = %v", err)
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		t.Fatalf("SerializeMessage error = %v", err)
	}

	// Expected size for single SystemTransfer:
	// Header: 3 bytes
	// AccountKeys: 1 (compact) + 3 * 32 = 97
	// Blockhash: 32
	// Instructions: 1 (compact) + 1 (progIdx) + 1 (compact) + 2 (accountIdxs) + 1 (compact) + 12 (data) = 18
	// Total: 3 + 97 + 32 + 18 = 150
	expectedSize := 150
	if len(msgBytes) != expectedSize {
		t.Errorf("serialized message size = %d, want %d", len(msgBytes), expectedSize)
	}
}

func TestSignTransaction_SingleSigner(t *testing.T) {
	// Generate a test keypair.
	pubBytes, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var feePayer SolPublicKey
	copy(feePayer[:], pubBytes)
	dest := SolPublicKey{2}

	ix := BuildSystemTransferInstruction(feePayer, dest, 100)
	msg, err := CompileMessage(feePayer, []SolInstruction{ix}, [32]byte{0xab})
	if err != nil {
		t.Fatalf("CompileMessage error = %v", err)
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		t.Fatalf("SerializeMessage error = %v", err)
	}

	signers := map[SolPublicKey]ed25519.PrivateKey{
		feePayer: privKey,
	}

	tx, err := SignTransaction(msg, msgBytes, signers)
	if err != nil {
		t.Fatalf("SignTransaction error = %v", err)
	}

	if len(tx.Signatures) != 1 {
		t.Fatalf("signature count = %d, want 1", len(tx.Signatures))
	}

	// Verify the signature.
	valid := ed25519.Verify(pubBytes, msgBytes, tx.Signatures[0][:])
	if !valid {
		t.Error("signature verification failed")
	}
}

func TestSignTransaction_MultiSigner(t *testing.T) {
	// Generate 3 keypairs.
	type kp struct {
		pub  SolPublicKey
		priv ed25519.PrivateKey
	}
	var keypairs []kp
	for i := 0; i < 3; i++ {
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		var pk SolPublicKey
		copy(pk[:], pub)
		keypairs = append(keypairs, kp{pk, priv})
	}

	feePayer := keypairs[0].pub
	dest := SolPublicKey{99}

	// Build 3 transfer instructions — one from each signer.
	var instructions []SolInstruction
	for _, pair := range keypairs {
		instructions = append(instructions, BuildSystemTransferInstruction(pair.pub, dest, 100))
	}

	msg, err := CompileMessage(feePayer, instructions, [32]byte{0xcc})
	if err != nil {
		t.Fatalf("CompileMessage error = %v", err)
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		t.Fatal(err)
	}

	// NumRequiredSignatures should be 3.
	if msg.Header.NumRequiredSignatures != 3 {
		t.Fatalf("numRequiredSignatures = %d, want 3", msg.Header.NumRequiredSignatures)
	}

	signers := make(map[SolPublicKey]ed25519.PrivateKey)
	for _, pair := range keypairs {
		signers[pair.pub] = pair.priv
	}

	tx, err := SignTransaction(msg, msgBytes, signers)
	if err != nil {
		t.Fatalf("SignTransaction error = %v", err)
	}

	if len(tx.Signatures) != 3 {
		t.Fatalf("signature count = %d, want 3", len(tx.Signatures))
	}

	// Verify each signature matches the correct signer.
	for i := 0; i < 3; i++ {
		pubKey := msg.AccountKeys[i]
		valid := ed25519.Verify(pubKey[:], msgBytes, tx.Signatures[i][:])
		if !valid {
			t.Errorf("signature %d verification failed for %s", i, pubKey.ToBase58())
		}
	}
}

func TestSignTransaction_MissingSigner(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	var feePayer SolPublicKey
	copy(feePayer[:], pub)

	// Create another signer needed by the instruction.
	otherPub := SolPublicKey{42}
	ix := SolInstruction{
		ProgramID: solSystemProgramID,
		Accounts: []SolAccountMeta{
			{PubKey: feePayer, IsSigner: true, IsWritable: true},
			{PubKey: otherPub, IsSigner: true, IsWritable: true},
		},
		Data: make([]byte, 12),
	}

	msg, err := CompileMessage(feePayer, []SolInstruction{ix}, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}

	msgBytes, _ := SerializeMessage(msg)

	// Only provide feePayer's key — otherPub is missing.
	signers := map[SolPublicKey]ed25519.PrivateKey{
		feePayer: priv,
	}

	_, err = SignTransaction(msg, msgBytes, signers)
	if err == nil {
		t.Error("expected error for missing signer")
	}
}

func TestBuildAndSerializeTransaction_FullFlow(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	var feePayer SolPublicKey
	copy(feePayer[:], pub)
	dest := SolPublicKey{2}

	ix := BuildSystemTransferInstruction(feePayer, dest, 1_000_000)
	signers := map[SolPublicKey]ed25519.PrivateKey{feePayer: priv}

	txBytes, txSig, err := BuildAndSerializeTransaction(feePayer, []SolInstruction{ix}, [32]byte{0xab}, signers)
	if err != nil {
		t.Fatalf("BuildAndSerializeTransaction error = %v", err)
	}

	if len(txBytes) == 0 {
		t.Error("transaction bytes should not be empty")
	}

	if txSig == "" {
		t.Error("transaction signature should not be empty")
	}

	// TX should be well under 1232 bytes for a single transfer.
	if len(txBytes) > 300 {
		t.Errorf("unexpectedly large tx: %d bytes", len(txBytes))
	}
}
