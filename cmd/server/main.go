package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fantasim/hdpay/internal/api"
	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/logging"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/wallet"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		if err := runServe(); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	case "init":
		if err := runInit(); err != nil {
			slog.Error("init error", "error", err)
			os.Exit(1)
		}
	case "export":
		if err := runExport(); err != nil {
			slog.Error("export error", "error", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("hdpay %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: hdpay <command>

Commands:
  serve     Start the HTTP server
  init      Generate HD wallet addresses and store in DB
  export    Export addresses to JSON files
  version   Print version information
`)
}

func runServe() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := logging.Setup(cfg.LogLevel, cfg.LogDir); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	slog.Info("starting hdpay",
		"version", version,
		"network", cfg.Network,
		"port", cfg.Port,
		"dbPath", cfg.DBPath,
		"logLevel", cfg.LogLevel,
	)

	database, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	slog.Info("database opened", "path", cfg.DBPath)

	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("database migrations applied")

	router := api.NewRouter(database, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  config.ServerReadTimeout,
		WriteTimeout: config.ServerWriteTimeout,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server listen error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutdown signal received, draining connections...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	slog.Info("server stopped gracefully")
	return nil
}

func runInit() error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	mnemonicFile := fs.String("mnemonic-file", "", "Path to file containing 24-word BIP-39 mnemonic (required)")
	dbPath := fs.String("db", "", "Database path (default: from HDPAY_DB_PATH or ./data/hdpay.sqlite)")
	network := fs.String("network", "", "Network: mainnet or testnet (default: from HDPAY_NETWORK or mainnet)")
	count := fs.Int("count", config.MaxAddressesPerChain, "Number of addresses per chain")
	fs.Parse(os.Args[2:])

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := logging.Setup(cfg.LogLevel, cfg.LogDir); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	// Override config with flags if provided.
	if *mnemonicFile != "" {
		cfg.MnemonicFile = *mnemonicFile
	}
	if *dbPath != "" {
		cfg.DBPath = *dbPath
	}
	if *network != "" {
		cfg.Network = *network
	}

	if cfg.MnemonicFile == "" {
		return fmt.Errorf("--mnemonic-file is required (or set HDPAY_MNEMONIC_FILE)")
	}

	slog.Info("starting address initialization",
		"mnemonicFile", cfg.MnemonicFile,
		"dbPath", cfg.DBPath,
		"network", cfg.Network,
		"countPerChain", *count,
	)

	// Read and validate mnemonic.
	mnemonic, err := wallet.ReadMnemonicFromFile(cfg.MnemonicFile)
	if err != nil {
		return fmt.Errorf("read mnemonic: %w", err)
	}

	// Derive seed.
	seed, err := wallet.MnemonicToSeed(mnemonic)
	if err != nil {
		return fmt.Errorf("derive seed: %w", err)
	}

	// Derive master key for BTC/BSC (BIP-32).
	net := wallet.NetworkParams(cfg.Network)
	masterKey, err := wallet.DeriveMasterKey(seed, net)
	if err != nil {
		return fmt.Errorf("derive master key: %w", err)
	}

	// Open database.
	database, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	progress := func(chain models.Chain, generated int, total int) {
		slog.Info("address generation progress",
			"chain", chain,
			"generated", generated,
			"total", total,
			"progress", fmt.Sprintf("%.1f%%", float64(generated)/float64(total)*100),
		)
	}

	totalStart := time.Now()

	// Generate BTC addresses.
	if err := generateAndStore(database, models.ChainBTC, *count, func() ([]models.Address, error) {
		return wallet.GenerateBTCAddresses(masterKey, *count, net, progress)
	}); err != nil {
		return err
	}

	// Generate BSC addresses.
	if err := generateAndStore(database, models.ChainBSC, *count, func() ([]models.Address, error) {
		return wallet.GenerateBSCAddresses(masterKey, *count, progress)
	}); err != nil {
		return err
	}

	// Generate SOL addresses.
	if err := generateAndStore(database, models.ChainSOL, *count, func() ([]models.Address, error) {
		return wallet.GenerateSOLAddresses(seed, *count, progress)
	}); err != nil {
		return err
	}

	slog.Info("address initialization complete",
		"totalDuration", time.Since(totalStart).Round(time.Millisecond),
	)

	// Auto-export after generation.
	slog.Info("exporting addresses to JSON")
	for _, chain := range models.AllChains {
		if err := wallet.ExportAddresses(database, chain, cfg.Network, ""); err != nil {
			slog.Error("export failed", "chain", chain, "error", err)
		}
	}

	return nil
}

// generateAndStore generates addresses for a chain and stores them in DB.
// Skips if the chain already has the expected count of addresses.
func generateAndStore(database *db.DB, chain models.Chain, expectedCount int, generate func() ([]models.Address, error)) error {
	existing, err := database.CountAddresses(chain)
	if err != nil {
		return fmt.Errorf("count existing %s addresses: %w", chain, err)
	}

	if existing == expectedCount {
		slog.Info("addresses already exist, skipping",
			"chain", chain,
			"count", existing,
		)
		return nil
	}

	if existing > 0 && existing != expectedCount {
		slog.Warn("partial address set detected, regenerating",
			"chain", chain,
			"existing", existing,
			"expected", expectedCount,
		)
		if err := database.DeleteAddresses(chain); err != nil {
			return fmt.Errorf("delete partial %s addresses: %w", chain, err)
		}
	}

	addresses, err := generate()
	if err != nil {
		return fmt.Errorf("generate %s addresses: %w", chain, err)
	}

	if err := database.InsertAddressBatch(chain, addresses); err != nil {
		return fmt.Errorf("insert %s addresses: %w", chain, err)
	}

	return nil
}

func runExport() error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	dbPath := fs.String("db", "", "Database path (default: from HDPAY_DB_PATH or ./data/hdpay.sqlite)")
	network := fs.String("network", "", "Network: mainnet or testnet (default: from HDPAY_NETWORK or mainnet)")
	outputDir := fs.String("output", "", "Output directory (default: ./data/export)")
	fs.Parse(os.Args[2:])

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := logging.Setup(cfg.LogLevel, cfg.LogDir); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	if *dbPath != "" {
		cfg.DBPath = *dbPath
	}
	if *network != "" {
		cfg.Network = *network
	}

	database, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	slog.Info("exporting addresses",
		"dbPath", cfg.DBPath,
		"network", cfg.Network,
		"outputDir", *outputDir,
	)

	for _, chain := range models.AllChains {
		if err := wallet.ExportAddresses(database, chain, cfg.Network, *outputDir); err != nil {
			slog.Error("export failed", "chain", chain, "error", err)
			continue
		}
	}

	slog.Info("export complete")
	return nil
}
