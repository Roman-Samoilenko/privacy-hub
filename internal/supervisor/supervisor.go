package supervisor

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/api"
	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/hubctl"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/Roman-Samoilenko/privacy-hub/internal/proxyserver"
)

var running bool
var mu sync.Mutex

func Start() {
	mu.Lock()
	if running {
		logger.Warn("Supervisor is already running")
		mu.Unlock()
		return
	}
	running = true
	mu.Unlock()

	logger.Info("Starting supervisor...")

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Start API
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := api.Start(ctx, cfg.API); err != nil {
			logger.Error("API error: %v", err)
		}
	}()

	// Start Proxy
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := proxyserver.Start(ctx, cfg.Proxy); err != nil {
			logger.Error("Proxy error: %v", err)
		}
	}()

	// Start DNS Control
	wg.Add(1)
	go func() {
		defer wg.Done()
		hubctl.DNSControl(ctx, cfg.DockerContainer)
	}()

	logger.Success("Supervisor started successfully")

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutdown signal received")

	cancel()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("All services stopped gracefully")
	case <-time.After(10 * time.Second):
		logger.Warn("Shutdown timeout, forcing exit")
	}

	Stop()
}

func Stop() {
	mu.Lock()
	defer mu.Unlock()

	if !running {
		return
	}

	logger.Info("Stopping supervisor...")
	running = false
	logger.Success("Supervisor stopped successfully")
}

func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return running
}
