package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/Roman-Samoilenko/privacy-hub/internal/supervisor"
)

var (
	configPath = flag.String("config", "configs/config.yaml", "Path to configuration file")
	version    = flag.Bool("version", false, "Print version and exit")
)

const Version = "1.0.0"

func main() {
	flag.Parse()

	if *version {
		logger.Infof("Privacy Hub v%s", Version)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		logger.Errorf("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	// Initialize logger with config
	logger.SetLevel(cfg.Logging.Level)
	logger.Infof("Starting Privacy Hub v%s...", Version)

	// Start supervisor
	sup := supervisor.New(cfg)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start services
	if err := sup.Start(); err != nil {
		logger.Errorf("Failed to start supervisor: %v", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Infof("Received signal: %v", sig)

	// Graceful shutdown
	if err := sup.Stop(); err != nil {
		logger.Errorf("Shutdown error: %v", err)
		os.Exit(1)
	}

	logger.Infof("Privacy Hub stopped successfully")
}
