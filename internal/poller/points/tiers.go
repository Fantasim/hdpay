package points

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
)

// defaultTiers is the 9-tier configuration written when tiers.json is missing.
var defaultTiers = []models.Tier{
	{MinUSD: 0, MaxUSD: ptrFloat(1), Multiplier: 0.0},
	{MinUSD: 1, MaxUSD: ptrFloat(12), Multiplier: 1.0},
	{MinUSD: 12, MaxUSD: ptrFloat(30), Multiplier: 1.1},
	{MinUSD: 30, MaxUSD: ptrFloat(60), Multiplier: 1.2},
	{MinUSD: 60, MaxUSD: ptrFloat(120), Multiplier: 1.3},
	{MinUSD: 120, MaxUSD: ptrFloat(240), Multiplier: 1.4},
	{MinUSD: 240, MaxUSD: ptrFloat(600), Multiplier: 1.5},
	{MinUSD: 600, MaxUSD: ptrFloat(1200), Multiplier: 2.0},
	{MinUSD: 1200, MaxUSD: nil, Multiplier: 3.0},
}

// ptrFloat returns a pointer to f.
func ptrFloat(f float64) *float64 {
	return &f
}

// LoadTiers reads tiers from a JSON file, validates them, and returns the slice.
func LoadTiers(path string) ([]models.Tier, error) {
	slog.Debug("loading tiers configuration", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tiers file %q: %w", path, err)
	}

	var tiers []models.Tier
	if err := json.Unmarshal(data, &tiers); err != nil {
		return nil, fmt.Errorf("parse tiers JSON: %w", err)
	}

	if err := ValidateTiers(tiers); err != nil {
		return nil, err
	}

	slog.Info("tiers configuration loaded",
		"path", path,
		"tierCount", len(tiers),
	)

	return tiers, nil
}

// ValidateTiers checks that a tier slice satisfies all invariants:
//   - at least MinTierCount tiers
//   - sorted by min_usd ascending
//   - no gaps between consecutive tiers (each min_usd == prev max_usd)
//   - all min_usd >= 0, all multiplier >= 0
//   - last tier must have max_usd == nil (unbounded)
//   - non-last tiers must have max_usd != nil and max_usd > min_usd
func ValidateTiers(tiers []models.Tier) error {
	if len(tiers) < config.MinTierCount {
		return fmt.Errorf("tiers validation: need at least %d tiers, got %d", config.MinTierCount, len(tiers))
	}

	// Check sorted by min_usd ascending.
	if !sort.SliceIsSorted(tiers, func(i, j int) bool {
		return tiers[i].MinUSD < tiers[j].MinUSD
	}) {
		return fmt.Errorf("tiers validation: tiers must be sorted by min_usd ascending")
	}

	for i, t := range tiers {
		if t.MinUSD < 0 {
			return fmt.Errorf("tiers validation: tier %d has negative min_usd %.2f", i, t.MinUSD)
		}
		if t.Multiplier < 0 {
			return fmt.Errorf("tiers validation: tier %d has negative multiplier %.2f", i, t.Multiplier)
		}

		isLast := i == len(tiers)-1

		if isLast {
			if t.MaxUSD != nil {
				return fmt.Errorf("tiers validation: last tier (index %d) must have max_usd null (unbounded)", i)
			}
		} else {
			if t.MaxUSD == nil {
				return fmt.Errorf("tiers validation: non-last tier (index %d) must have max_usd set", i)
			}
			if *t.MaxUSD <= t.MinUSD {
				return fmt.Errorf("tiers validation: tier %d has max_usd (%.2f) <= min_usd (%.2f)", i, *t.MaxUSD, t.MinUSD)
			}
		}

		// Check continuity: each tier's min_usd must equal previous tier's max_usd.
		if i > 0 {
			prev := tiers[i-1]
			if prev.MaxUSD == nil {
				return fmt.Errorf("tiers validation: tier %d follows an unbounded tier", i)
			}
			if t.MinUSD != *prev.MaxUSD {
				return fmt.Errorf("tiers validation: gap between tier %d (max_usd %.2f) and tier %d (min_usd %.2f)", i-1, *prev.MaxUSD, i, t.MinUSD)
			}
		}
	}

	return nil
}

// CreateDefaultTiers writes the default 9-tier configuration to the given path.
func CreateDefaultTiers(path string) error {
	slog.Info("creating default tiers configuration", "path", path)

	data, err := json.MarshalIndent(defaultTiers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal default tiers: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write default tiers to %q: %w", path, err)
	}

	slog.Info("default tiers configuration created",
		"path", path,
		"tierCount", len(defaultTiers),
	)

	return nil
}

// LoadOrCreateTiers tries to load tiers from path. If the file does not exist,
// it creates the default configuration first, then loads it.
func LoadOrCreateTiers(path string) ([]models.Tier, error) {
	tiers, err := LoadTiers(path)
	if err == nil {
		return tiers, nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		// File exists but is invalid.
		return nil, err
	}

	// File not found — create defaults.
	if err := CreateDefaultTiers(path); err != nil {
		return nil, err
	}

	return LoadTiers(path)
}
