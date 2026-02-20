package points

import (
	"testing"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

func TestCalculate_AllTierBoundaries(t *testing.T) {
	calc := NewPointsCalculator(defaultTiers)

	tests := []struct {
		name       string
		usdValue   float64
		wantPoints int
		wantTier   int
		wantMult   float64
	}{
		{"$0.50 — below $1 ignore tier", 0.50, 0, 0, 0.0},
		{"$1.00 — tier 1 start", 1.00, 100, 1, 1.0},
		{"$5.00 — tier 1 middle", 5.00, 500, 1, 1.0},
		{"$11.99 — tier 1 top", 11.99, 1199, 1, 1.0},
		{"$12.00 — tier 2 start", 12.00, 1320, 2, 1.1},
		{"$20.00 — tier 2 middle", 20.00, 2200, 2, 1.1},
		{"$50.00 — tier 3", 50.00, 6000, 3, 1.2},
		{"$100.00 — tier 4", 100.00, 13000, 4, 1.3},
		{"$200.00 — tier 5", 200.00, 28000, 5, 1.4},
		{"$500.00 — tier 6", 500.00, 75000, 6, 1.5},
		{"$1000.00 — tier 7", 1000.00, 200000, 7, 2.0},
		{"$2000.00 — tier 8 (unbounded)", 2000.00, 600000, 8, 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.Calculate(tt.usdValue)

			if result.Points != tt.wantPoints {
				t.Errorf("Calculate(%.2f) points = %d, want %d", tt.usdValue, result.Points, tt.wantPoints)
			}
			if result.TierIndex != tt.wantTier {
				t.Errorf("Calculate(%.2f) tier = %d, want %d", tt.usdValue, result.TierIndex, tt.wantTier)
			}
			if result.Multiplier != tt.wantMult {
				t.Errorf("Calculate(%.2f) multiplier = %.2f, want %.2f", tt.usdValue, result.Multiplier, tt.wantMult)
			}
		})
	}
}

func TestCalculate_Zero(t *testing.T) {
	calc := NewPointsCalculator(defaultTiers)
	result := calc.Calculate(0)

	if result.Points != 0 {
		t.Errorf("Calculate(0) points = %d, want 0", result.Points)
	}
	if result.TierIndex != 0 {
		t.Errorf("Calculate(0) tier = %d, want 0", result.TierIndex)
	}
}

func TestCalculate_VeryLargeAmount(t *testing.T) {
	calc := NewPointsCalculator(defaultTiers)
	result := calc.Calculate(10000.00)

	// 10000 * 100 cents * 3.0 = 3,000,000
	if result.Points != 3000000 {
		t.Errorf("Calculate(10000) points = %d, want 3000000", result.Points)
	}
	if result.TierIndex != 8 {
		t.Errorf("Calculate(10000) tier = %d, want 8", result.TierIndex)
	}
}

func TestCalculate_FractionalCents(t *testing.T) {
	calc := NewPointsCalculator(defaultTiers)

	// $1.005 → floor(100.5) = 100 cents → 100 * 1.0 = 100 points
	result := calc.Calculate(1.005)
	if result.Points != 100 {
		t.Errorf("Calculate(1.005) points = %d, want 100", result.Points)
	}

	// $1.999 → floor(199.9) = 199 cents → 199 * 1.0 = 199 points
	result = calc.Calculate(1.999)
	if result.Points != 199 {
		t.Errorf("Calculate(1.999) points = %d, want 199", result.Points)
	}
}

func TestReload_Valid(t *testing.T) {
	calc := NewPointsCalculator(defaultTiers)

	newTiers := []models.Tier{
		{MinUSD: 0, MaxUSD: ptrFloat(10), Multiplier: 0.5},
		{MinUSD: 10, MaxUSD: nil, Multiplier: 2.0},
	}

	if err := calc.Reload(newTiers); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	// $5 with new tiers: tier 0 (0.5), cents=500, 500*0.5=250
	result := calc.Calculate(5.0)
	if result.Points != 250 {
		t.Errorf("after reload Calculate(5.0) points = %d, want 250", result.Points)
	}
}

func TestReload_Invalid(t *testing.T) {
	calc := NewPointsCalculator(defaultTiers)

	// Invalid: gap between tiers.
	invalid := []models.Tier{
		{MinUSD: 0, MaxUSD: ptrFloat(5), Multiplier: 1.0},
		{MinUSD: 10, MaxUSD: nil, Multiplier: 2.0},
	}

	if err := calc.Reload(invalid); err == nil {
		t.Error("Reload() should reject invalid tiers")
	}

	// Original tiers should be unchanged.
	result := calc.Calculate(1.0)
	if result.Points != 100 {
		t.Errorf("after failed reload Calculate(1.0) points = %d, want 100", result.Points)
	}
}

func TestTiers_ReturnsCopy(t *testing.T) {
	calc := NewPointsCalculator(defaultTiers)
	tiers := calc.Tiers()

	if len(tiers) != len(defaultTiers) {
		t.Errorf("Tiers() returned %d tiers, want %d", len(tiers), len(defaultTiers))
	}

	// Modifying the copy should not affect the calculator.
	tiers[0].Multiplier = 999.0
	result := calc.Calculate(0.5)
	if result.Multiplier != 0.0 {
		t.Errorf("Tiers() should return a copy, but modification affected calculator")
	}
}

func TestMatchTier_ExactBoundary(t *testing.T) {
	// At exact boundary $12.00, should be tier 2 (min=12, max=30), not tier 1 (min=1, max=12).
	calc := NewPointsCalculator(defaultTiers)
	result := calc.Calculate(12.00)

	if result.TierIndex != 2 {
		t.Errorf("at boundary $12.00 tier = %d, want 2", result.TierIndex)
	}
}
