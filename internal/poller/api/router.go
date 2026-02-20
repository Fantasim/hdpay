package api

import (
	"log/slog"

	hdmiddleware "github.com/Fantasim/hdpay/internal/api/middleware"
	"github.com/Fantasim/hdpay/internal/poller/api/handlers"
	pollermw "github.com/Fantasim/hdpay/internal/poller/api/middleware"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/points"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Dependencies holds all service references needed by the API layer.
type Dependencies struct {
	DB         *pollerdb.DB
	Watcher    *watcher.Watcher
	Calculator *points.PointsCalculator
	Allowlist  *pollermw.IPAllowlist
	Sessions   *pollermw.SessionStore
	Config     *pollerconfig.Config
	Pricer     *points.Pricer
}

// NewRouter creates and configures the Chi router with all middleware and routes.
func NewRouter(deps *Dependencies) chi.Router {
	r := chi.NewRouter()

	// Global middleware (applied to ALL routes).
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(hdmiddleware.RequestLogging)
	r.Use(pollermw.CORS)

	slog.Info("poller router initialized",
		"middleware", []string{"realIP", "recoverer", "requestLogging", "cors"},
	)

	// Exempt routes: no IP check, no session auth.
	r.Get("/api/health", handlers.HealthHandler(deps.Config, deps.Watcher))
	r.Post("/api/admin/login", handlers.LoginHandler(deps.Sessions))

	// IP-restricted routes.
	r.Route("/api", func(r chi.Router) {
		r.Use(deps.Allowlist.Middleware)

		// Watch management — IP-restricted, no session needed.
		r.Post("/watch", handlers.CreateWatchHandler(deps.Watcher, deps.Config))
		r.Delete("/watch/{id}", handlers.CancelWatchHandler(deps.Watcher))
		r.Get("/watches", handlers.ListWatchesHandler(deps.DB))

		// Points — IP-restricted, no session needed.
		r.Get("/points", handlers.GetPointsHandler(deps.DB))
		r.Get("/points/pending", handlers.GetPendingPointsHandler(deps.DB))
		r.Post("/points/claim", handlers.ClaimPointsHandler(deps.DB))

		// Admin — IP-restricted + session required.
		r.Route("/admin", func(r chi.Router) {
			r.Use(deps.Sessions.Middleware)

			r.Post("/logout", handlers.LogoutHandler(deps.Sessions))
			r.Get("/allowlist", handlers.GetAllowlistHandler(deps.DB))
			r.Post("/allowlist", handlers.AddAllowlistHandler(deps.DB, deps.Allowlist))
			r.Delete("/allowlist/{id}", handlers.RemoveAllowlistHandler(deps.DB, deps.Allowlist))
			r.Get("/settings", handlers.GetSettingsHandler(deps.Config, deps.Watcher, deps.Calculator))
			r.Put("/tiers", handlers.UpdateTiersHandler(deps.Config, deps.Calculator))
			r.Put("/watch-defaults", handlers.UpdateWatchDefaultsHandler(deps.Watcher))
		})

		// Dashboard — IP-restricted + session required.
		r.Route("/dashboard", func(r chi.Router) {
			r.Use(deps.Sessions.Middleware)

			r.Get("/stats", handlers.DashboardStatsHandler(deps.DB, deps.Watcher))
			r.Get("/transactions", handlers.DashboardTransactionsHandler(deps.DB))
			r.Get("/charts", handlers.DashboardChartsHandler(deps.DB))
			r.Get("/errors", handlers.DashboardErrorsHandler(deps.DB))
		})
	})

	return r
}
