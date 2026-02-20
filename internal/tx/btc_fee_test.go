package tx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

func TestBTCFeeEstimator_EstimateFee(t *testing.T) {
	feeResp := models.FeeEstimate{
		FastestFee:  15,
		HalfHourFee: 12,
		HourFee:     9,
		EconomyFee:  6,
		MinimumFee:  1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != config.MempoolFeeEstimatePath {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(feeResp)
	}))
	defer server.Close()

	estimator := NewBTCFeeEstimator(server.Client(), server.URL)

	estimate, err := estimator.EstimateFee(context.Background())
	if err != nil {
		t.Fatalf("EstimateFee() error = %v", err)
	}

	if estimate.FastestFee != 15 {
		t.Errorf("FastestFee = %d, want 15", estimate.FastestFee)
	}
	if estimate.HalfHourFee != 12 {
		t.Errorf("HalfHourFee = %d, want 12", estimate.HalfHourFee)
	}
	if estimate.HourFee != 9 {
		t.Errorf("HourFee = %d, want 9", estimate.HourFee)
	}
	if estimate.EconomyFee != 6 {
		t.Errorf("EconomyFee = %d, want 6", estimate.EconomyFee)
	}
	if estimate.MinimumFee != 1 {
		t.Errorf("MinimumFee = %d, want 1", estimate.MinimumFee)
	}
}

func TestBTCFeeEstimator_FallbackOnAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	estimator := NewBTCFeeEstimator(server.Client(), server.URL)

	estimate, err := estimator.EstimateFee(context.Background())
	if err != nil {
		t.Fatalf("EstimateFee() should not error on fallback, got: %v", err)
	}

	// Should fall back to default values.
	if estimate.HalfHourFee != int64(config.BTCDefaultFeeRate) {
		t.Errorf("HalfHourFee = %d, want default %d", estimate.HalfHourFee, config.BTCDefaultFeeRate)
	}
}

func TestBTCFeeEstimator_EnforcesMinimum(t *testing.T) {
	// Return fee rates below the minimum.
	feeResp := models.FeeEstimate{
		FastestFee:  0,
		HalfHourFee: 0,
		HourFee:     0,
		EconomyFee:  0,
		MinimumFee:  0,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(feeResp)
	}))
	defer server.Close()

	estimator := NewBTCFeeEstimator(server.Client(), server.URL)

	estimate, err := estimator.EstimateFee(context.Background())
	if err != nil {
		t.Fatalf("EstimateFee() error = %v", err)
	}

	min := int64(config.BTCMinFeeRate)
	if estimate.FastestFee < min {
		t.Errorf("FastestFee = %d, want >= %d", estimate.FastestFee, min)
	}
	if estimate.HalfHourFee < min {
		t.Errorf("HalfHourFee = %d, want >= %d", estimate.HalfHourFee, min)
	}
	if estimate.MinimumFee < min {
		t.Errorf("MinimumFee = %d, want >= %d", estimate.MinimumFee, min)
	}
}

func TestBTCFeeEstimator_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{this is not valid json at all!!!`))
	}))
	defer server.Close()

	estimator := NewBTCFeeEstimator(server.Client(), server.URL)

	// Malformed JSON causes fetchFromAPI to fail, so EstimateFee should
	// fall back to the default estimate (no error returned).
	estimate, err := estimator.EstimateFee(context.Background())
	if err != nil {
		t.Fatalf("EstimateFee() should not error on fallback, got: %v", err)
	}

	// Should fall back to default values.
	if estimate.HalfHourFee != int64(config.BTCDefaultFeeRate) {
		t.Errorf("HalfHourFee = %d, want default %d", estimate.HalfHourFee, config.BTCDefaultFeeRate)
	}
}

func TestDefaultFeeRate(t *testing.T) {
	estimate := &models.FeeEstimate{
		FastestFee:  20,
		HalfHourFee: 15,
		HourFee:     10,
		EconomyFee:  5,
		MinimumFee:  1,
	}

	rate := DefaultFeeRate(estimate)
	if rate != 15 {
		t.Errorf("DefaultFeeRate() = %d, want 15 (halfHourFee)", rate)
	}
}
