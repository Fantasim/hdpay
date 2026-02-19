package tx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/wallet"
)

func TestEstimateBTCVsize(t *testing.T) {
	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		wantVsize  int
	}{
		{
			name:       "1 input, 1 output",
			numInputs:  1,
			numOutputs: 1,
			// weight = 42 + 1*(164+108) + 1*124 = 438
			// vsize = ceil(438/4) = 110
			wantVsize: 110,
		},
		{
			name:       "1 input, 2 outputs",
			numInputs:  1,
			numOutputs: 2,
			// weight = 42 + 272 + 248 = 562
			// vsize = ceil(562/4) = 141
			wantVsize: 141,
		},
		{
			name:       "10 inputs, 1 output",
			numInputs:  10,
			numOutputs: 1,
			// weight = 42 + 10*272 + 124 = 2886
			// vsize = ceil(2886/4) = 722
			wantVsize: 722,
		},
		{
			name:       "100 inputs, 1 output",
			numInputs:  100,
			numOutputs: 1,
			// weight = 42 + 100*272 + 124 = 27366
			// vsize = ceil(27366/4) = 6842
			wantVsize: 6842,
		},
		{
			name:       "3 inputs, 1 output",
			numInputs:  3,
			numOutputs: 1,
			// weight = 42 + 3*272 + 124 = 982
			// vsize = ceil(982/4) = 246
			wantVsize: 246,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateBTCVsize(tt.numInputs, tt.numOutputs)
			if got != tt.wantVsize {
				t.Errorf("EstimateBTCVsize(%d, %d) = %d, want %d",
					tt.numInputs, tt.numOutputs, got, tt.wantVsize)
			}
		})
	}
}

func TestBuildBTCConsolidationTx_Basic(t *testing.T) {
	utxos := []models.UTXO{
		{TxID: "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111", Vout: 0, Value: 50000, Address: "bc1qtest0", AddressIndex: 0},
		{TxID: "bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222", Vout: 1, Value: 30000, Address: "bc1qtest1", AddressIndex: 1},
		{TxID: "cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333", Vout: 0, Value: 20000, Address: "bc1qtest2", AddressIndex: 2},
	}

	// Use a valid bech32 address for the destination.
	destAddr := "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu"
	feeRate := int64(10)

	built, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       utxos,
		DestAddress: destAddr,
		FeeRate:     feeRate,
		NetParams:   &chaincfg.MainNetParams,
	})
	if err != nil {
		t.Fatalf("BuildBTCConsolidationTx() error = %v", err)
	}

	// Verify input count.
	if len(built.Tx.TxIn) != 3 {
		t.Errorf("expected 3 inputs, got %d", len(built.Tx.TxIn))
	}

	// Verify output count (consolidation = 1 output).
	if len(built.Tx.TxOut) != 1 {
		t.Errorf("expected 1 output, got %d", len(built.Tx.TxOut))
	}

	// Verify total input.
	if built.TotalInputSats != 100000 {
		t.Errorf("TotalInputSats = %d, want 100000", built.TotalInputSats)
	}

	// Verify fee (includes BTCFeeSafetyMarginPct safety margin).
	expectedVsize := EstimateBTCVsize(3, 1)
	baseFee := feeRate * int64(expectedVsize)
	safetyMargin := baseFee * int64(config.BTCFeeSafetyMarginPct) / 100
	if safetyMargin < 1 {
		safetyMargin = 1
	}
	expectedFee := baseFee + safetyMargin
	if built.FeeSats != expectedFee {
		t.Errorf("FeeSats = %d, want %d", built.FeeSats, expectedFee)
	}

	// Verify output value.
	expectedOutput := built.TotalInputSats - built.FeeSats
	if built.OutputSats != expectedOutput {
		t.Errorf("OutputSats = %d, want %d", built.OutputSats, expectedOutput)
	}
	if built.Tx.TxOut[0].Value != expectedOutput {
		t.Errorf("tx.TxOut[0].Value = %d, want %d", built.Tx.TxOut[0].Value, expectedOutput)
	}
}

func TestBuildBTCConsolidationTx_InsufficientFunds(t *testing.T) {
	// A single tiny UTXO that can't cover fees.
	utxos := []models.UTXO{
		{TxID: "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111", Vout: 0, Value: 100, Address: "bc1qtest0", AddressIndex: 0},
	}

	destAddr := "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu"

	_, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       utxos,
		DestAddress: destAddr,
		FeeRate:     10,
		NetParams:   &chaincfg.MainNetParams,
	})
	if err == nil {
		t.Fatal("expected error for insufficient funds")
	}
}

func TestBuildBTCConsolidationTx_DustOutput(t *testing.T) {
	// UTXO value that results in output below dust threshold after fees.
	// 1 input, 1 output: vsize = 110, fee at 10 sat/vB = 1100
	// So 1600 sats → output = 500, which is below dust (546).
	utxos := []models.UTXO{
		{TxID: "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111", Vout: 0, Value: 1600, Address: "bc1qtest0", AddressIndex: 0},
	}

	destAddr := "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu"

	_, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       utxos,
		DestAddress: destAddr,
		FeeRate:     10,
		NetParams:   &chaincfg.MainNetParams,
	})
	if err == nil {
		t.Fatal("expected error for dust output")
	}
}

func TestBuildBTCConsolidationTx_NoUTXOs(t *testing.T) {
	_, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       nil,
		DestAddress: "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
		FeeRate:     10,
		NetParams:   &chaincfg.MainNetParams,
	})
	if err == nil {
		t.Fatal("expected error for no UTXOs")
	}
}

func TestSignBTCTx_Integration(t *testing.T) {
	// Full integration test: derive keys, build TX, sign, and verify witnesses.
	net := &chaincfg.MainNetParams

	// Derive master key from test mnemonic.
	seed, err := wallet.MnemonicToSeed(testMnemonic24)
	if err != nil {
		t.Fatal(err)
	}
	masterKey, err := wallet.DeriveMasterKey(seed, net)
	if err != nil {
		t.Fatal(err)
	}

	// Get addresses and keys for indices 0, 1, 2.
	type addrKey struct {
		address  string
		pkScript []byte
	}
	addrKeys := make([]addrKey, 3)
	for i := 0; i < 3; i++ {
		addr, err := wallet.DeriveBTCAddress(masterKey, uint32(i), net)
		if err != nil {
			t.Fatal(err)
		}

		decoded, err := btcutil.DecodeAddress(addr, net)
		if err != nil {
			t.Fatal(err)
		}
		pkScript, err := txscript.PayToAddrScript(decoded)
		if err != nil {
			t.Fatal(err)
		}

		addrKeys[i] = addrKey{address: addr, pkScript: pkScript}
	}

	// Build UTXOs (synthetic — we don't care about real txids for signing test).
	utxos := []models.UTXO{
		{TxID: "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111", Vout: 0, Value: 50000, Address: addrKeys[0].address, AddressIndex: 0},
		{TxID: "bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222", Vout: 1, Value: 30000, Address: addrKeys[1].address, AddressIndex: 1},
		{TxID: "cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333cccc3333", Vout: 0, Value: 20000, Address: addrKeys[2].address, AddressIndex: 2},
	}

	// Build the TX.
	built, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       utxos,
		DestAddress: addrKeys[0].address, // Send to first address.
		FeeRate:     10,
		NetParams:   net,
	})
	if err != nil {
		t.Fatalf("BuildBTCConsolidationTx() error = %v", err)
	}

	// Derive private keys for signing.
	mnemonicPath := filepath.Join(t.TempDir(), "mnemonic.txt")
	if err := os.WriteFile(mnemonicPath, []byte(testMnemonic24), 0o600); err != nil {
		t.Fatal(err)
	}
	ks := NewKeyService(mnemonicPath, "mainnet")

	signingUTXOs := make([]SigningUTXO, 3)
	for i := 0; i < 3; i++ {
		privKey, err := ks.DeriveBTCPrivateKey(context.Background(), uint32(i))
		if err != nil {
			t.Fatalf("derive key %d: %v", i, err)
		}
		signingUTXOs[i] = SigningUTXO{
			UTXO:     utxos[i],
			PKScript: addrKeys[i].pkScript,
			PrivKey:  privKey,
		}
	}

	// Sign.
	if err := SignBTCTx(built.Tx, signingUTXOs); err != nil {
		t.Fatalf("SignBTCTx() error = %v", err)
	}

	// Verify all inputs have witnesses.
	for i, txIn := range built.Tx.TxIn {
		if len(txIn.Witness) == 0 {
			t.Errorf("input %d has no witness", i)
		}
		if len(txIn.Witness) != 2 {
			t.Errorf("input %d witness has %d items, want 2 (sig + pubkey)", i, len(txIn.Witness))
		}
		// SignatureScript should be nil for native SegWit.
		if len(txIn.SignatureScript) != 0 {
			t.Errorf("input %d has non-nil SignatureScript (should be nil for P2WPKH)", i)
		}
	}

	// Verify serialization works.
	rawHex, err := SerializeBTCTx(built.Tx)
	if err != nil {
		t.Fatalf("SerializeBTCTx() error = %v", err)
	}

	if len(rawHex) == 0 {
		t.Error("serialized tx hex is empty")
	}
}

func TestSignBTCTx_InputCountMismatch(t *testing.T) {
	utxos := []models.UTXO{
		{TxID: "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111", Vout: 0, Value: 50000, Address: "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu", AddressIndex: 0},
	}

	built, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       utxos,
		DestAddress: "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
		FeeRate:     1,
		NetParams:   &chaincfg.MainNetParams,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pass wrong number of signing UTXOs.
	err = SignBTCTx(built.Tx, []SigningUTXO{})
	if err == nil {
		t.Fatal("expected error for input count mismatch")
	}
}

func TestPKScriptFromAddress(t *testing.T) {
	// Known bech32 address should produce a valid P2WPKH script.
	addr := "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu"
	pkScript, err := PKScriptFromAddress(addr, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("PKScriptFromAddress() error = %v", err)
	}

	// P2WPKH script is 22 bytes: OP_0 (0x00) + push 20 bytes (0x14) + 20-byte hash.
	if len(pkScript) != 22 {
		t.Errorf("pkScript length = %d, want 22", len(pkScript))
	}

	// First byte should be OP_0.
	if pkScript[0] != 0x00 {
		t.Errorf("pkScript[0] = 0x%02x, want 0x00 (OP_0)", pkScript[0])
	}

	// Second byte should be 0x14 (push 20 bytes).
	if pkScript[1] != 0x14 {
		t.Errorf("pkScript[1] = 0x%02x, want 0x14 (push 20)", pkScript[1])
	}
}

func TestPKScriptFromAddress_InvalidAddress(t *testing.T) {
	_, err := PKScriptFromAddress("invalid_address", &chaincfg.MainNetParams)
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestValidateUTXOsAgainstPreview_CountDiverged(t *testing.T) {
	// Expected 10 UTXOs, got 7 → 30% drop → should fail (threshold 5%).
	utxos := make([]models.UTXO, 7)
	for i := range utxos {
		utxos[i] = models.UTXO{Value: 10000}
	}

	err := ValidateUTXOsAgainstPreview(utxos, 10, 100000)
	if err == nil {
		t.Fatal("expected error for UTXO count divergence")
	}
	if !errors.Is(err, config.ErrUTXODiverged) {
		t.Errorf("expected ErrUTXODiverged, got %v", err)
	}
}

func TestValidateUTXOsAgainstPreview_ValueDiverged(t *testing.T) {
	// Expected 100000 sats total, got 85000 → 15% drop → should fail (threshold 3%).
	utxos := []models.UTXO{
		{Value: 50000},
		{Value: 35000},
	}

	err := ValidateUTXOsAgainstPreview(utxos, 2, 100000)
	if err == nil {
		t.Fatal("expected error for UTXO value divergence")
	}
	if !errors.Is(err, config.ErrUTXODiverged) {
		t.Errorf("expected ErrUTXODiverged, got %v", err)
	}
}

func TestValidateUTXOsAgainstPreview_WithinTolerance(t *testing.T) {
	// Expected 50 UTXOs / 500000 sats, got 48 UTXOs / 490000 sats.
	// Count drop = 4% (within 5% threshold), value drop = 2% (within 3% threshold).
	utxos := make([]models.UTXO, 48)
	for i := range utxos {
		utxos[i] = models.UTXO{Value: 10000}
	}
	// 48 * 10000 = 480000, need 490000 → add 10000 to last
	utxos[47].Value = 20000

	err := ValidateUTXOsAgainstPreview(utxos, 50, 500000)
	if err != nil {
		t.Fatalf("expected no error for within-tolerance UTXOs, got %v", err)
	}
}

func TestValidateUTXOsAgainstPreview_NoExpectedSkips(t *testing.T) {
	// expectedInputCount = 0 means no validation.
	utxos := []models.UTXO{{Value: 1000}}

	err := ValidateUTXOsAgainstPreview(utxos, 0, 0)
	if err != nil {
		t.Fatalf("expected no error when no expectations set, got %v", err)
	}
}

func TestBuildBTCConsolidationTx_MaxInputsExceeded(t *testing.T) {
	// Create more UTXOs than the max.
	utxos := make([]models.UTXO, config.BTCMaxInputsPerTx+1)
	for i := range utxos {
		utxos[i] = models.UTXO{
			TxID:         "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111",
			Vout:         uint32(i),
			Value:        10000,
			Address:      "bc1qtest",
			AddressIndex: i,
		}
	}

	_, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       utxos,
		DestAddress: "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
		FeeRate:     1,
		NetParams:   &chaincfg.MainNetParams,
	})
	if err == nil {
		t.Fatal("expected error for too many inputs")
	}
}
