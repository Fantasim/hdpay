package points

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
)

func TestLoadTiers_DefaultFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")

	// Write default tiers.
	data, err := json.MarshalIndent(defaultTiers, "", "  ")
	if err != nil {
		t.Fatalf("marshal default tiers: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write tiers file: %v", err)
	}

	tiers, err := LoadTiers(path)
	if err != nil {
		t.Fatalf("LoadTiers() error = %v", err)
	}

	if len(tiers) != 9 {
		t.Errorf("expected 9 tiers, got %d", len(tiers))
	}

	// Verify first tier (ignore tier).
	if tiers[0].MinUSD != 0 || tiers[0].Multiplier != 0.0 {
		t.Errorf("tier 0: want min=0 mult=0.0, got min=%.2f mult=%.2f", tiers[0].MinUSD, tiers[0].Multiplier)
	}

	// Verify last tier (unbounded).
	last := tiers[len(tiers)-1]
	if last.MaxUSD != nil {
		t.Error("last tier should have nil MaxUSD")
	}
	if last.Multiplier != 3.0 {
		t.Errorf("last tier multiplier: want 3.0, got %.2f", last.Multiplier)
	}
}

func TestLoadTiers_FileNotFound(t *testing.T) {
	_, err := LoadTiers("/nonexistent/tiers.json")
	if err == nil {
		t.Error("LoadTiers() should error on missing file")
	}
}

func TestLoadTiers_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")
	os.WriteFile(path, []byte(`not json`), 0644)

	_, err := LoadTiers(path)
	if err == nil {
		t.Error("LoadTiers() should error on invalid JSON")
	}
}

func TestValidateTiers_Valid(t *testing.T) {
	if err := ValidateTiers(defaultTiers); err != nil {
		t.Errorf("ValidateTiers(defaultTiers) error = %v", err)
	}
}

func TestValidateTiers_TooFewTiers(t *testing.T) {
	tiers := []models.Tier{
		{MinUSD: 0, MaxUSD: nil, Multiplier: 1.0},
	}
	err := ValidateTiers(tiers)
	if err == nil {
		t.Errorf("expected error for %d tiers (< %d)", len(tiers), config.MinTierCount)
	}
}

func TestValidateTiers_Unsorted(t *testing.T) {
	tiers := []models.Tier{
		{MinUSD: 10, MaxUSD: ptrFloat(20), Multiplier: 1.0},
		{MinUSD: 0, MaxUSD: ptrFloat(10), Multiplier: 0.5},
		{MinUSD: 20, MaxUSD: nil, Multiplier: 2.0},
	}
	err := ValidateTiers(tiers)
	if err == nil {
		t.Error("expected error for unsorted tiers")
	}
}

func TestValidateTiers_Gap(t *testing.T) {
	tiers := []models.Tier{
		{MinUSD: 0, MaxUSD: ptrFloat(10), Multiplier: 1.0},
		{MinUSD: 15, MaxUSD: nil, Multiplier: 2.0}, // gap: 10 -> 15
	}
	err := ValidateTiers(tiers)
	if err == nil {
		t.Error("expected error for gap between tiers")
	}
}

func TestValidateTiers_NegativeMinUSD(t *testing.T) {
	tiers := []models.Tier{
		{MinUSD: -5, MaxUSD: ptrFloat(10), Multiplier: 1.0},
		{MinUSD: 10, MaxUSD: nil, Multiplier: 2.0},
	}
	err := ValidateTiers(tiers)
	if err == nil {
		t.Error("expected error for negative min_usd")
	}
}

func TestValidateTiers_NegativeMultiplier(t *testing.T) {
	tiers := []models.Tier{
		{MinUSD: 0, MaxUSD: ptrFloat(10), Multiplier: -1.0},
		{MinUSD: 10, MaxUSD: nil, Multiplier: 2.0},
	}
	err := ValidateTiers(tiers)
	if err == nil {
		t.Error("expected error for negative multiplier")
	}
}

func TestValidateTiers_LastTierBounded(t *testing.T) {
	tiers := []models.Tier{
		{MinUSD: 0, MaxUSD: ptrFloat(10), Multiplier: 1.0},
		{MinUSD: 10, MaxUSD: ptrFloat(20), Multiplier: 2.0}, // last tier must be unbounded
	}
	err := ValidateTiers(tiers)
	if err == nil {
		t.Error("expected error for bounded last tier")
	}
}

func TestValidateTiers_NonLastUnbounded(t *testing.T) {
	tiers := []models.Tier{
		{MinUSD: 0, MaxUSD: nil, Multiplier: 1.0}, // non-last unbounded
		{MinUSD: 10, MaxUSD: nil, Multiplier: 2.0},
	}
	err := ValidateTiers(tiers)
	if err == nil {
		t.Error("expected error for non-last unbounded tier")
	}
}

func TestValidateTiers_MaxLessThanMin(t *testing.T) {
	tiers := []models.Tier{
		{MinUSD: 10, MaxUSD: ptrFloat(5), Multiplier: 1.0},
		{MinUSD: 5, MaxUSD: nil, Multiplier: 2.0},
	}
	err := ValidateTiers(tiers)
	if err == nil {
		t.Error("expected error for max_usd <= min_usd")
	}
}

func TestCreateDefaultTiers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")

	if err := CreateDefaultTiers(path); err != nil {
		t.Fatalf("CreateDefaultTiers() error = %v", err)
	}

	// File should exist and be valid.
	tiers, err := LoadTiers(path)
	if err != nil {
		t.Fatalf("LoadTiers() after create: %v", err)
	}

	if len(tiers) != 9 {
		t.Errorf("expected 9 default tiers, got %d", len(tiers))
	}
}

func TestLoadOrCreateTiers_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")

	// Write custom 2-tier config.
	custom := []models.Tier{
		{MinUSD: 0, MaxUSD: ptrFloat(5), Multiplier: 0.5},
		{MinUSD: 5, MaxUSD: nil, Multiplier: 1.0},
	}
	data, _ := json.MarshalIndent(custom, "", "  ")
	os.WriteFile(path, data, 0644)

	tiers, err := LoadOrCreateTiers(path)
	if err != nil {
		t.Fatalf("LoadOrCreateTiers() error = %v", err)
	}

	if len(tiers) != 2 {
		t.Errorf("expected 2 custom tiers, got %d", len(tiers))
	}
}

func TestLoadOrCreateTiers_FileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")

	tiers, err := LoadOrCreateTiers(path)
	if err != nil {
		t.Fatalf("LoadOrCreateTiers() error = %v", err)
	}

	if len(tiers) != 9 {
		t.Errorf("expected 9 default tiers, got %d", len(tiers))
	}

	// File should now exist.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("tiers file should have been created")
	}
}

func TestLoadOrCreateTiers_InvalidExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")

	// Write invalid tiers (gap).
	invalid := []models.Tier{
		{MinUSD: 0, MaxUSD: ptrFloat(5), Multiplier: 1.0},
		{MinUSD: 10, MaxUSD: nil, Multiplier: 2.0}, // gap
	}
	data, _ := json.MarshalIndent(invalid, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := LoadOrCreateTiers(path)
	if err == nil {
		t.Error("LoadOrCreateTiers() should fail on invalid existing file")
	}
}
