package api

import (
	"log/slog"

	"github.com/Fantasim/hdpay/internal/api/handlers"
	"github.com/Fantasim/hdpay/internal/api/middleware"
	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/scanner"
	"github.com/go-chi/chi/v5"
)

// Version is set at build time via ldflags.
var Version = "dev"

// NewRouter creates and configures the Chi router with all middleware and routes.
func NewRouter(database *db.DB, cfg *config.Config, sc *scanner.Scanner, hub *scanner.SSEHub) chi.Router {
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
		r.Get("/scan/sse", handlers.ScanSSE(hub))
	})

	return r
}
