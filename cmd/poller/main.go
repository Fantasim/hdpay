package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	hdconfig "github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/logging"
	pollerapi "github.com/Fantasim/hdpay/internal/poller/api"
	pollermw "github.com/Fantasim/hdpay/internal/poller/api/middleware"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/points"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/provider"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
	"github.com/Fantasim/hdpay/internal/shared/price"
	webpoller "github.com/Fantasim/hdpay/web/poller"
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

	// Instance lock: prevent two pollers from running against the same database.
	pidFile := filepath.Join(filepath.Dir(cfg.DBPath), "poller.pid")
	if err := acquirePIDLock(pidFile); err != nil {
		slog.Error("instance lock failed — another poller may be running", "pidFile", pidFile, "error", err)
		os.Exit(1)
	}
	defer func() {
		if pidLockFile != nil {
			pidLockFile.Close()
		}
		os.Remove(pidFile)
	}()

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

	// Wire up DB-backed provider usage tracking.
	for _, ps := range providerSets {
		ps.SetOnRecord(func(chain, prov string, success bool, is429 bool) {
			if err := db.IncrementUsage(chain, prov, success, is429); err != nil {
				slog.Warn("failed to record provider usage",
					"chain", chain,
					"provider", prov,
					"error", err,
				)
			}
		})
	}
	slog.Info("provider usage tracking wired to database")

	// Initialize watcher.
	w := watcher.NewWatcher(db, providerSets, pricer, calculator, cfg)

	// Run startup recovery (blocks until complete).
	recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), pollerconfig.RecoveryTimeout)
	if err := w.RunRecovery(recoveryCtx); err != nil {
		slog.Error("recovery failed", "error", err)
		// Recovery failure is non-fatal — log and continue.
	}
	recoveryCancel()

	// Start periodic orphan recovery (rechecks PENDING txs from expired watches).
	w.StartOrphanRecovery()

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

	// Extract the embedded SPA build directory (strip the "build/" prefix from the embed FS).
	staticFS, err := fs.Sub(webpoller.StaticFiles, "build")
	if err != nil {
		slog.Error("failed to access embedded static files", "error", err)
		os.Exit(1)
	}
	slog.Info("embedded SPA loaded")

	// Build API router with all dependencies.
	deps := &pollerapi.Dependencies{
		DB:           db,
		Watcher:      w,
		Calculator:   calculator,
		Allowlist:    allowlist,
		Sessions:     sessions,
		Config:       cfg,
		Pricer:       pricer,
		ProviderSets: providerSets,
		StaticFS:     staticFS,
	}
	router := pollerapi.NewRouter(deps)

	// Start HTTP server.
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
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

	// BTC providers: Blockstream + Mempool + Bitaps (round-robin for redundancy).
	btcProviders := []provider.Provider{
		provider.NewBlockstreamProvider(httpClient, cfg.Network),
		provider.NewMempoolProvider(httpClient, cfg.Network),
		provider.NewBitapsProvider(httpClient, cfg.Network),
	}
	btcRPS := []int{
		hdconfig.RateLimitBlockstream,
		hdconfig.RateLimitMempool,
		hdconfig.RateLimitBitaps,
	}
	sets["BTC"] = provider.NewProviderSet("BTC", btcProviders, btcRPS, []int64{
		hdconfig.KnownMonthlyLimitBlockstream,
		hdconfig.KnownMonthlyLimitMempool,
		hdconfig.KnownMonthlyLimitBitaps,
	})

	// BSC providers: BSC RPC with multi-URL fallback (BscScan was shut down Dec 18, 2025).
	// BSCRPCPollerProvider maintains per-address balance state for change detection;
	// it must be a single instance (not multiple) to avoid split state.
	bscRPCProvider, err := provider.NewBSCRPCPollerProvider(cfg.Network)
	if err != nil {
		slog.Error("failed to initialize BSC RPC provider", "error", err)
		// Non-fatal — BSC watching will fail gracefully via ErrNoProviders.
	}
	bscProviders := []provider.Provider{}
	bscRPS := []int{}
	bscMonthly := []int64{}
	if bscRPCProvider != nil {
		bscProviders = append(bscProviders, bscRPCProvider)
		bscRPS = append(bscRPS, hdconfig.RateLimitBSCRPC)
		bscMonthly = append(bscMonthly, hdconfig.KnownMonthlyLimitBSCRPC)
	}
	sets["BSC"] = provider.NewProviderSet("BSC", bscProviders, bscRPS, bscMonthly)

	// SOL providers: public RPC + Ankr + dRPC + OnFinality (no-key),
	// plus optional Helius and Alchemy if API keys are configured.
	solProviders := []provider.Provider{
		provider.NewSolanaRPCProvider(httpClient, cfg.Network),
		provider.NewAnkrSolanaProvider(httpClient, cfg.Network),
		provider.NewDRPCSolanaProvider(httpClient, cfg.Network),
		provider.NewOnFinalitySolanaProvider(httpClient, cfg.Network),
	}
	solRPS := []int{
		hdconfig.RateLimitSolanaRPC,
		hdconfig.RateLimitAnkrSOL,
		hdconfig.RateLimitDRPC,
		hdconfig.RateLimitOnFinality,
	}
	solMonthly := []int64{
		hdconfig.KnownMonthlyLimitSolanaRPC,
		hdconfig.KnownMonthlyLimitAnkrSOL,
		hdconfig.KnownMonthlyLimitDRPC,
		hdconfig.KnownMonthlyLimitOnFinality,
	}
	if cfg.HeliusAPIKey != "" {
		solProviders = append(solProviders,
			provider.NewHeliusProvider(httpClient, cfg.Network, cfg.HeliusAPIKey),
		)
		solRPS = append(solRPS, hdconfig.RateLimitHelius)
		solMonthly = append(solMonthly, hdconfig.KnownMonthlyLimitHelius)
		slog.Info("helius solana provider enabled")
	}
	if cfg.AlchemyAPIKey != "" {
		solProviders = append(solProviders,
			provider.NewAlchemySolanaProvider(httpClient, cfg.Network, cfg.AlchemyAPIKey),
		)
		solRPS = append(solRPS, hdconfig.RateLimitAlchemy)
		solMonthly = append(solMonthly, hdconfig.KnownMonthlyLimitAlchemy)
		slog.Info("alchemy solana provider enabled")
	}
	sets["SOL"] = provider.NewProviderSet("SOL", solProviders, solRPS, solMonthly)

	slog.Info("provider sets initialized",
		"btcProviders", len(btcProviders),
		"bscProviders", len(bscProviders),
		"solProviders", len(solProviders),
	)

	return sets
}

// pidLockFile holds the open PID lock file descriptor so the flock is held
// for the lifetime of the process.
var pidLockFile *os.File

// acquirePIDLock creates a PID file with an exclusive flock to prevent multiple
// poller instances. The flock is automatically released when the process exits.
func acquirePIDLock(pidFile string) error {
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	f, err := os.OpenFile(pidFile, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open PID file: %w", err)
	}

	// Try non-blocking exclusive lock — fails immediately if another process holds it.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return fmt.Errorf("another poller instance is running (flock on %s)", pidFile)
	}

	// Write our PID into the locked file.
	if err := f.Truncate(0); err != nil {
		f.Close()
		return fmt.Errorf("failed to truncate PID file: %w", err)
	}
	if _, err := fmt.Fprintf(f, "%d\n", os.Getpid()); err != nil {
		f.Close()
		return fmt.Errorf("failed to write PID: %w", err)
	}

	// Keep file open so the flock persists for the process lifetime.
	pidLockFile = f
	slog.Info("PID lock acquired", "pidFile", pidFile, "pid", os.Getpid())
	return nil
}
