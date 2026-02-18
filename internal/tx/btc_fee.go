package tx

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// BTCFeeEstimator fetches dynamic fee rates from mempool.space.
type BTCFeeEstimator struct {
	client     *http.Client
	mempoolURL string // Base URL, e.g. "https://mempool.space/api"
}

// NewBTCFeeEstimator creates a fee estimator using mempool.space API.
func NewBTCFeeEstimator(client *http.Client, mempoolURL string) *BTCFeeEstimator {
	slog.Info("BTC fee estimator created", "mempoolURL", mempoolURL)
	return &BTCFeeEstimator{
		client:     client,
		mempoolURL: mempoolURL,
	}
}

// EstimateFee fetches all fee tiers from mempool.space.
// Falls back to default constant values if the API is unreachable.
func (fe *BTCFeeEstimator) EstimateFee(ctx context.Context) (*models.FeeEstimate, error) {
	estimate, err := fe.fetchFromAPI(ctx)
	if err != nil {
		slog.Warn("fee estimation API failed, using default",
			"error", err,
			"defaultFeeRate", config.BTCDefaultFeeRate,
		)
		return fe.defaultEstimate(), nil
	}

	// Enforce minimum fee rate on all tiers.
	fe.enforceMinimum(estimate)

	slog.Info("fee estimate fetched",
		"fastestFee", estimate.FastestFee,
		"halfHourFee", estimate.HalfHourFee,
		"hourFee", estimate.HourFee,
		"economyFee", estimate.EconomyFee,
		"minimumFee", estimate.MinimumFee,
	)

	return estimate, nil
}

// DefaultFeeRate returns the medium-priority fee rate (halfHourFee) from an estimate.
func DefaultFeeRate(estimate *models.FeeEstimate) int64 {
	return estimate.HalfHourFee
}

// fetchFromAPI queries the mempool.space fee recommendation endpoint.
func (fe *BTCFeeEstimator) fetchFromAPI(ctx context.Context) (*models.FeeEstimate, error) {
	ctx, cancel := context.WithTimeout(ctx, config.FeeEstimateTimeout)
	defer cancel()

	url := fe.mempoolURL + config.MempoolFeeEstimatePath

	slog.Debug("fetching fee estimate", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create fee request: %w", err)
	}

	resp, err := fe.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", config.ErrFeeEstimateFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", config.ErrFeeEstimateFailed, resp.StatusCode)
	}

	var estimate models.FeeEstimate
	if err := json.NewDecoder(resp.Body).Decode(&estimate); err != nil {
		return nil, fmt.Errorf("decode fee response: %w", err)
	}

	return &estimate, nil
}

// defaultEstimate returns a conservative fallback fee estimate.
func (fe *BTCFeeEstimator) defaultEstimate() *models.FeeEstimate {
	return &models.FeeEstimate{
		FastestFee:  int64(config.BTCDefaultFeeRate) * 2,
		HalfHourFee: int64(config.BTCDefaultFeeRate),
		HourFee:     int64(config.BTCDefaultFeeRate),
		EconomyFee:  int64(config.BTCMinFeeRate),
		MinimumFee:  int64(config.BTCMinFeeRate),
	}
}

// enforceMinimum ensures no fee rate is below the minimum.
func (fe *BTCFeeEstimator) enforceMinimum(est *models.FeeEstimate) {
	min := int64(config.BTCMinFeeRate)
	if est.FastestFee < min {
		est.FastestFee = min
	}
	if est.HalfHourFee < min {
		est.HalfHourFee = min
	}
	if est.HourFee < min {
		est.HourFee = min
	}
	if est.EconomyFee < min {
		est.EconomyFee = min
	}
	if est.MinimumFee < min {
		est.MinimumFee = min
	}
}
