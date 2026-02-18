package wallet

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Fantasim/hdpay/internal/models"
)

// ExportDir is the default directory for JSON address exports.
const ExportDir = "./data/export"

// AddressStreamer is the interface for streaming addresses from DB.
type AddressStreamer interface {
	StreamAddresses(chain models.Chain, fn func(addr models.Address) error) error
	CountAddresses(chain models.Chain) (int, error)
}

// derivationPathTemplate returns the derivation path template string for a chain.
func derivationPathTemplate(chain models.Chain) string {
	switch chain {
	case models.ChainBTC:
		return "m/84'/0'/0'/0/{index}"
	case models.ChainBSC:
		return "m/44'/60'/0'/0/{index}"
	case models.ChainSOL:
		return "m/44'/501'/{index}'/0'"
	default:
		return ""
	}
}

// ExportAddresses exports all addresses for a chain to a JSON file using streaming.
// The file is written incrementally to avoid loading all 500K addresses into memory.
func ExportAddresses(db AddressStreamer, chain models.Chain, network string, outputDir string) error {
	if outputDir == "" {
		outputDir = ExportDir
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create export directory %q: %w", outputDir, err)
	}

	count, err := db.CountAddresses(chain)
	if err != nil {
		return fmt.Errorf("count addresses for export: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("no addresses found for chain %s", chain)
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("%s_addresses.json", chain))
	slog.Info("exporting addresses",
		"chain", chain,
		"count", count,
		"file", filename,
	)

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create export file %q: %w", filename, err)
	}
	defer f.Close()

	// Write header
	header := fmt.Sprintf(
		`{"chain":"%s","network":"%s","derivation_path_template":"%s","generated_at":"%s","count":%d,"addresses":[`,
		chain, network, derivationPathTemplate(chain),
		time.Now().UTC().Format(time.RFC3339), count,
	)
	if _, err := io.WriteString(f, header); err != nil {
		return fmt.Errorf("write export header: %w", err)
	}

	// Stream addresses
	first := true
	exported := 0
	err = db.StreamAddresses(chain, func(addr models.Address) error {
		if !first {
			if _, err := io.WriteString(f, ","); err != nil {
				return err
			}
		}
		first = false

		entry, err := json.Marshal(models.AddressExportItem{
			Index:   addr.AddressIndex,
			Address: addr.Address,
		})
		if err != nil {
			return fmt.Errorf("marshal address entry: %w", err)
		}

		if _, err := f.Write(entry); err != nil {
			return err
		}

		exported++
		if exported%100_000 == 0 {
			slog.Info("export progress",
				"chain", chain,
				"exported", exported,
				"total", count,
			)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("stream addresses for export: %w", err)
	}

	// Write footer
	if _, err := io.WriteString(f, "]}"); err != nil {
		return fmt.Errorf("write export footer: %w", err)
	}

	slog.Info("export complete",
		"chain", chain,
		"exported", exported,
		"file", filename,
	)
	return nil
}
