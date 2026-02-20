package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	hdconfig "github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/logging"
	pollerapi "github.com/Fantasim/hdpay/internal/poller/api"
	pollermw "github.com/Fantasim/hdpay/internal/poller/api/middleware"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/points"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/provider"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
	"github.com/Fantasim/hdpay/internal/price"
)

func main() {
	// Load configuration.
	cfg, err := pollerconfig.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging with Poller-specific file prefix.
	logCloser, err := logging.SetupWithPrefix(
		cfg.LogLevel,
		cfg.LogDir,
		pollerconfig.PollerLogFilePattern,
		pollerconfig.PollerLogPrefix,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer logCloser.Close()

	slog.Info("poller starting",
		"port", cfg.Port,
		"network", cfg.Network,
		"dbPath", cfg.DBPath,
		"startDate", time.Unix(cfg.StartDate, 0).UTC().Format(time.RFC3339),
		"maxActiveWatches", cfg.MaxActiveWatches,
		"defaultWatchTimeout", cfg.DefaultWatchTimeout,
	)

	// Open database and run migrations.
	db, err := pollerdb.New(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	slog.Info("database ready", "path", cfg.DBPath)

	// Load tier configuration.
	tiers, err := points.LoadOrCreateTiers(cfg.TiersFile)
	if err != nil {
		slog.Error("failed to load tiers", "error", err)
		os.Exit(1)
	}
	slog.Info("tiers loaded", "count", len(tiers), "file", cfg.TiersFile)

	// Initialize price service (HDPay's CoinGecko service).
	priceSvc := price.NewPriceService()

	// Initialize Poller services.
	pricer := points.NewPricer(priceSvc)
	calculator := points.NewPointsCalculator(tiers)

	// Initialize blockchain providers (one ProviderSet per chain).
	httpClient := provider.NewHTTPClient()
	providerSets := initProviderSets(httpClient, cfg)

	// Initialize watcher.
	w := watcher.NewWatcher(db, providerSets, pricer, calculator, cfg)

	// Run startup recovery (blocks until complete).
	recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), pollerconfig.RecoveryTimeout)
	if err := w.RunRecovery(recoveryCtx); err != nil {
		slog.Error("recovery failed", "error", err)
		// Recovery failure is non-fatal — log and continue.
	}
	recoveryCancel()

	// Initialize IP allowlist from DB.
	ipCache, err := db.LoadAllIPsIntoMap()
	if err != nil {
		slog.Error("failed to load IP allowlist", "error", err)
		os.Exit(1)
	}
	allowlist := pollermw.NewIPAllowlist(ipCache)

	// Initialize session store with admin credentials.
	sessions, err := pollermw.NewSessionStore(cfg.AdminUsername, cfg.AdminPassword)
	if err != nil {
		slog.Error("failed to initialize session store", "error", err)
		os.Exit(1)
	}

	// Build API router with all dependencies.
	deps := &pollerapi.Dependencies{
		DB:         db,
		Watcher:    w,
		Calculator: calculator,
		Allowlist:  allowlist,
		Sessions:   sessions,
		Config:     cfg,
		Pricer:     pricer,
	}
	router := pollerapi.NewRouter(deps)

	// Start HTTP server.
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  pollerconfig.ServerReadTimeout,
		WriteTimeout: pollerconfig.ServerWriteTimeout,
	}

	// Graceful shutdown.
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("poller HTTP server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal.
	sig := <-done
	slog.Info("shutdown signal received", "signal", sig)

	// Stop watcher first (cancel all watches, wait for goroutines).
	w.Stop()

	// Then shut down HTTP server.
	ctx, cancel := context.WithTimeout(context.Background(), pollerconfig.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	slog.Info("poller stopped")
}

// initProviderSets creates ProviderSets for each supported chain based on config.
func initProviderSets(httpClient *http.Client, cfg *pollerconfig.Config) map[string]*provider.ProviderSet {
	sets := make(map[string]*provider.ProviderSet)

	// BTC providers: Blockstream + Mempool.
	btcProviders := []provider.Provider{
		provider.NewBlockstreamProvider(httpClient, cfg.Network),
		provider.NewMempoolProvider(httpClient, cfg.Network),
	}
	btcRPS := []int{
		hdconfig.RateLimitBlockstream,
		hdconfig.RateLimitMempool,
	}
	sets["BTC"] = provider.NewProviderSet("BTC", btcProviders, btcRPS)

	// BSC providers: BscScan.
	bscProviders := []provider.Provider{
		provider.NewBscScanProvider(httpClient, cfg.Network, cfg.BscScanAPIKey),
	}
	bscRPS := []int{
		hdconfig.RateLimitBscScan,
	}
	sets["BSC"] = provider.NewProviderSet("BSC", bscProviders, bscRPS)

	// SOL providers: Solana RPC + Helius (if API key provided).
	solProviders := []provider.Provider{
		provider.NewSolanaRPCProvider(httpClient, cfg.Network),
	}
	solRPS := []int{
		hdconfig.RateLimitSolanaRPC,
	}
	if cfg.HeliusAPIKey != "" {
		solProviders = append(solProviders,
			provider.NewHeliusProvider(httpClient, cfg.Network, cfg.HeliusAPIKey),
		)
		solRPS = append(solRPS, hdconfig.RateLimitHelius)
	}
	sets["SOL"] = provider.NewProviderSet("SOL", solProviders, solRPS)

	slog.Info("provider sets initialized",
		"btcProviders", len(btcProviders),
		"bscProviders", len(bscProviders),
		"solProviders", len(solProviders),
	)

	return sets
}
