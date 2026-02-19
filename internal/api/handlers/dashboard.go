package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/price"
)

// tokenPortfolioItem represents a single token within a chain's portfolio.
type tokenPortfolioItem struct {
	Symbol      models.Token `json:"symbol"`
	Balance     string       `json:"balance"`
	USD         float64      `json:"usd"`
	FundedCount int          `json:"fundedCount"`
}

// chainPortfolio represents a single chain's portfolio data.
type chainPortfolio struct {
	Chain        models.Chain         `json:"chain"`
	AddressCount int                  `json:"addressCount"`
	FundedCount  int                  `json:"fundedCount"`
	Tokens       []tokenPortfolioItem `json:"tokens"`
}

// portfolioData is the data payload for GET /api/dashboard/portfolio.
type portfolioData struct {
	TotalUSD float64          `json:"totalUsd"`
	LastScan *string          `json:"lastScan"`
	Chains   []chainPortfolio `json:"chains"`
}

// GetPrices handles GET /api/dashboard/prices.
func GetPrices(ps *price.PriceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		slog.Info("prices requested",
			"remoteAddr", r.RemoteAddr,
		)

		prices, err := ps.GetPrices(r.Context())
		if err != nil {
			slog.Error("failed to fetch prices",
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, config.ErrorPriceFetchFailed, "failed to fetch prices: "+err.Error())
			return
		}

		stale := ps.IsStale()

		slog.Info("prices response sent",
			"coins", len(prices),
			"stale", stale,
			"elapsed", time.Since(start).Round(time.Millisecond),
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: map[string]interface{}{
				"prices": prices,
				"stale":  stale,
			},
			Meta: &models.APIMeta{
				ExecutionTime: time.Since(start).Milliseconds(),
			},
		})
	}
}

// tokenToSymbol maps our internal token to the price symbol key.
func tokenToSymbol(chain models.Chain, token models.Token) string {
	if token == models.TokenNative {
		switch chain {
		case models.ChainBTC:
			return "BTC"
		case models.ChainBSC:
			return "BNB"
		case models.ChainSOL:
			return "SOL"
		}
	}
	return string(token)
}

// GetPortfolio handles GET /api/dashboard/portfolio.
func GetPortfolio(database *db.DB, ps *price.PriceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		slog.Info("portfolio requested",
			"remoteAddr", r.RemoteAddr,
		)

		// Fetch balance aggregates.
		aggregates, err := database.GetBalanceAggregates()
		if err != nil {
			slog.Error("failed to fetch balance aggregates",
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch balance aggregates")
			return
		}

		// Fetch prices.
		prices, err := ps.GetPrices(r.Context())
		if err != nil {
			slog.Error("failed to fetch prices for portfolio",
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, config.ErrorPriceFetchFailed, "failed to fetch prices")
			return
		}

		// Fetch latest scan time.
		lastScan, err := database.GetLatestScanTime()
		if err != nil {
			slog.Warn("failed to fetch latest scan time",
				"error", err,
			)
		}

		// Build per-chain data.
		chainMap := make(map[models.Chain]*chainPortfolio)
		for _, chain := range models.AllChains {
			chainMap[chain] = &chainPortfolio{
				Chain:  chain,
				Tokens: []tokenPortfolioItem{},
			}
		}

		var totalUSD float64

		for _, agg := range aggregates {
			cp, ok := chainMap[agg.Chain]
			if !ok {
				slog.Warn("unknown chain in aggregates", "chain", agg.Chain)
				continue
			}

			symbol := tokenToSymbol(agg.Chain, agg.Token)
			priceUSD := prices[symbol]

			balance, err := strconv.ParseFloat(agg.TotalBalance, 64)
			if err != nil {
				slog.Warn("failed to parse aggregate balance",
					"chain", agg.Chain,
					"token", agg.Token,
					"balance", agg.TotalBalance,
					"error", err,
				)
				continue
			}

			usdValue := balance * priceUSD
			totalUSD += usdValue

			cp.Tokens = append(cp.Tokens, tokenPortfolioItem{
				Symbol:      agg.Token,
				Balance:     agg.TotalBalance,
				USD:         usdValue,
				FundedCount: agg.FundedCount,
			})

			cp.FundedCount += agg.FundedCount
		}

		// Fetch address counts per chain.
		for _, chain := range models.AllChains {
			count, err := database.CountAddresses(chain)
			if err != nil {
				slog.Warn("failed to count addresses for portfolio",
					"chain", chain,
					"error", err,
				)
				continue
			}
			chainMap[chain].AddressCount = count
		}

		// Build response.
		chains := make([]chainPortfolio, 0, len(models.AllChains))
		for _, chain := range models.AllChains {
			chains = append(chains, *chainMap[chain])
		}

		data := portfolioData{
			TotalUSD: totalUSD,
			Chains:   chains,
		}

		if lastScan != "" {
			data.LastScan = &lastScan
		}

		slog.Info("portfolio response sent",
			"totalUsd", totalUSD,
			"chains", len(chains),
			"elapsed", time.Since(start).Round(time.Millisecond),
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: data,
			Meta: &models.APIMeta{
				ExecutionTime: time.Since(start).Milliseconds(),
			},
		})
	}
}
