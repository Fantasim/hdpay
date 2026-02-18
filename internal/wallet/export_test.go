package wallet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Fantasim/hdpay/internal/models"
)

// mockStreamer implements AddressStreamer for testing.
type mockStreamer struct {
	addresses map[models.Chain][]models.Address
}

func (m *mockStreamer) CountAddresses(chain models.Chain) (int, error) {
	return len(m.addresses[chain]), nil
}

func (m *mockStreamer) StreamAddresses(chain models.Chain, fn func(addr models.Address) error) error {
	for _, addr := range m.addresses[chain] {
		if err := fn(addr); err != nil {
			return err
		}
	}
	return nil
}

func TestExportAddresses(t *testing.T) {
	mock := &mockStreamer{
		addresses: map[models.Chain][]models.Address{
			models.ChainBTC: {
				{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qtest0", CreatedAt: "2026-01-01"},
				{Chain: models.ChainBTC, AddressIndex: 1, Address: "bc1qtest1", CreatedAt: "2026-01-01"},
				{Chain: models.ChainBTC, AddressIndex: 2, Address: "bc1qtest2", CreatedAt: "2026-01-01"},
			},
		},
	}

	outputDir := t.TempDir()

	err := ExportAddresses(mock, models.ChainBTC, "mainnet", outputDir)
	if err != nil {
		t.Fatalf("ExportAddresses() error = %v", err)
	}

	// Read and verify exported file.
	data, err := os.ReadFile(filepath.Join(outputDir, "BTC_addresses.json"))
	if err != nil {
		t.Fatal(err)
	}

	var export models.AddressExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}

	if export.Chain != models.ChainBTC {
		t.Errorf("export.Chain = %v, want BTC", export.Chain)
	}
	if export.Network != "mainnet" {
		t.Errorf("export.Network = %v, want mainnet", export.Network)
	}
	if export.Count != 3 {
		t.Errorf("export.Count = %d, want 3", export.Count)
	}
	if len(export.Addresses) != 3 {
		t.Errorf("export.Addresses length = %d, want 3", len(export.Addresses))
	}
	if export.DerivationPathTemplate != "m/84'/0'/0'/0/{index}" {
		t.Errorf("export.DerivationPathTemplate = %v", export.DerivationPathTemplate)
	}
	if export.Addresses[0].Address != "bc1qtest0" {
		t.Errorf("export.Addresses[0].Address = %v, want bc1qtest0", export.Addresses[0].Address)
	}
}

func TestExportAddressesEmptyChain(t *testing.T) {
	mock := &mockStreamer{
		addresses: map[models.Chain][]models.Address{},
	}

	err := ExportAddresses(mock, models.ChainBTC, "mainnet", t.TempDir())
	if err == nil {
		t.Error("ExportAddresses() expected error for empty chain")
	}
}

func TestDerivationPathTemplate(t *testing.T) {
	tests := []struct {
		chain models.Chain
		want  string
	}{
		{models.ChainBTC, "m/84'/0'/0'/0/{index}"},
		{models.ChainBSC, "m/44'/60'/0'/0/{index}"},
		{models.ChainSOL, "m/44'/501'/{index}'/0'"},
	}

	for _, tt := range tests {
		if got := derivationPathTemplate(tt.chain); got != tt.want {
			t.Errorf("derivationPathTemplate(%s) = %v, want %v", tt.chain, got, tt.want)
		}
	}
}
