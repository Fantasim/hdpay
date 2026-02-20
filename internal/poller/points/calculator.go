package points

import (
	"fmt"
	"log/slog"
	"math"
	"sync"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

// PointsCalculator computes points from a USD value using the tier system.
// Thread-safe: tiers are protected by a RWMutex for hot-reload from dashboard.
type PointsCalculator struct {
	tiers []models.Tier
	mu    sync.RWMutex
}

// CalculationResult holds the output of a points calculation.
type CalculationResult struct {
	Points     int
	TierIndex  int
	Multiplier float64
}

// NewPointsCalculator creates a calculator pre-loaded with the given tiers.
func NewPointsCalculator(tiers []models.Tier) *PointsCalculator {
	slog.Info("points calculator initialized", "tierCount", len(tiers))
	return &PointsCalculator{tiers: tiers}
}

// Calculate returns the points, matching tier index, and multiplier for a USD value.
// Uses flat tier matching: the entire amount uses the single matching tier's multiplier.
// Formula: cents = floor(usdValue * 100), points = round(cents * multiplier).
func (pc *PointsCalculator) Calculate(usdValue float64) CalculationResult {
	pc.mu.RLock()
	tiers := pc.tiers
	pc.mu.RUnlock()

	tierIdx, multiplier := matchTier(tiers, usdValue)

	cents := math.Floor(usdValue * 100)
	points := int(math.Round(cents * multiplier))

	slog.Debug("points calculated",
		"usdValue", usdValue,
		"tierIndex", tierIdx,
		"multiplier", multiplier,
		"cents", cents,
		"points", points,
	)

	return CalculationResult{
		Points:     points,
		TierIndex:  tierIdx,
		Multiplier: multiplier,
	}
}

// Reload replaces the current tier configuration. Used when tiers.json is
// saved from the dashboard. Returns error if the new tiers are invalid.
func (pc *PointsCalculator) Reload(tiers []models.Tier) error {
	if err := ValidateTiers(tiers); err != nil {
		return fmt.Errorf("reload tiers: %w", err)
	}

	pc.mu.Lock()
	pc.tiers = tiers
	pc.mu.Unlock()

	slog.Info("points calculator tiers reloaded", "tierCount", len(tiers))
	return nil
}

// Tiers returns a copy of the current tier configuration.
func (pc *PointsCalculator) Tiers() []models.Tier {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	out := make([]models.Tier, len(pc.tiers))
	copy(out, pc.tiers)
	return out
}

// matchTier finds the tier index and multiplier for a given USD value.
// Returns (tierIndex, multiplier). If no tier matches (should not happen with
// a valid config ending in an unbounded tier), returns (0, 0).
func matchTier(tiers []models.Tier, usdValue float64) (int, float64) {
	for i, t := range tiers {
		if t.MaxUSD == nil {
			// Unbounded last tier: min_usd <= value.
			if usdValue >= t.MinUSD {
				return i, t.Multiplier
			}
		} else {
			// Bounded tier: min_usd <= value < max_usd.
			if usdValue >= t.MinUSD && usdValue < *t.MaxUSD {
				return i, t.Multiplier
			}
		}
	}
	return 0, 0
}
