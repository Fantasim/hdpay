package api

import (
	"io/fs"
	"log/slog"

	"github.com/Fantasim/hdpay/internal/api/handlers"
	"github.com/Fantasim/hdpay/internal/api/middleware"
	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/price"
	"github.com/Fantasim/hdpay/internal/scanner"
	"github.com/go-chi/chi/v5"
)

// Version is set at build time via ldflags.
var Version = "dev"

// NewRouter creates and configures the Chi router with all middleware and routes.
// If staticFS is non-nil, a catch-all SPA handler is registered to serve the embedded frontend.
func NewRouter(database *db.DB, cfg *config.Config, sc *scanner.Scanner, hub *scanner.SSEHub, ps *price.PriceService, sendDeps *handlers.SendDeps, staticFS fs.FS) chi.Router {
	r := chi.NewRouter()

	// Middleware stack (order matters)
	r.Use(middleware.RequestLogging)
	r.Use(middleware.HostCheck)
	r.Use(middleware.CORS)
	r.Use(middleware.CSRF)

	slog.Info("router initialized",
		"middleware", []string{"requestLogging", "hostCheck", "cors", "csrf"},
	)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handlers.HealthHandler(cfg, Version))

		// Address management
		r.Get("/addresses/{chain}", handlers.ListAddresses(database))
		r.Get("/addresses/{chain}/export", handlers.ExportAddresses(database))

		// Scanning
		r.Post("/scan/start", handlers.StartScan(sc))
		r.Post("/scan/stop", handlers.StopScan(sc))
		r.Get("/scan/status", handlers.GetScanStatus(sc, database))
		r.Get("/scan/sse", handlers.ScanSSE(hub, sc, database))

		// Dashboard
		r.Route("/dashboard", func(r chi.Router) {
			r.Get("/prices", handlers.GetPrices(ps))
			r.Get("/portfolio", handlers.GetPortfolio(database, ps))
		})

		// Transaction History
		r.Get("/transactions", handlers.ListTransactions(database))
		r.Get("/transactions/{chain}", handlers.ListTransactions(database))

		// Settings
		r.Get("/settings", handlers.GetSettings(database))
		r.Put("/settings", handlers.UpdateSettings(database))
		r.Post("/settings/reset-balances", handlers.ResetBalancesHandler(database))
		r.Post("/settings/reset-all", handlers.ResetAllHandler(database))

		// Send / Transaction
		r.Route("/send", func(r chi.Router) {
			r.Post("/preview", handlers.PreviewSend(sendDeps))
			r.Post("/execute", handlers.ExecuteSend(sendDeps))
			r.Post("/gas-preseed", handlers.GasPreSeedHandler(sendDeps))
			r.Get("/sse", handlers.SendSSE(sendDeps.TxHub))
			r.Get("/pending", handlers.GetPendingTxStates(sendDeps))
			r.Post("/dismiss/{id}", handlers.DismissTxState(sendDeps))
		})
	})

	// Embedded SPA: serve static files with client-side routing fallback.
	if staticFS != nil {
		slog.Info("embedded SPA enabled, serving static files with fallback")
		r.NotFound(handlers.SPAHandler(staticFS))
	}

	return r
}
