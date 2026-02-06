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
var wg sync.WaitGroup

const (
	ShutdownTimeout = 10 * time.Second
)

func Start() {
	mu.Lock()
	if running {
		logger.Warnf("Supervisor is already running")
		mu.Unlock()
		return
	}
	running = true
	mu.Unlock()

	logger.Infof("Starting supervisor...")

	cfg, err := config.Load()
	if err != nil {
		logger.Errorf("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := api.Start(ctx, cfg.API); err != nil {
			logger.Errorf("API error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := proxyserver.Start(ctx, cfg.Proxy); err != nil {
			logger.Errorf("Proxy error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		hubctl.DNSControl(ctx, cfg.DockerContainer)
	}()

	logger.Successf("Supervisor started successfully")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Infof("Shutdown signal received")

	cancel()

	gracefulShutdown()
}

func gracefulShutdown() {
	logger.Infof("Initiating graceful shutdown...")
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Infof("All services stopped gracefully")
	case <-time.After(ShutdownTimeout):
		logger.Warnf("Shutdown timeout, forcing exit")
	}

	Stop()
}

func Stop() {
	mu.Lock()
	defer mu.Unlock()

	if !running {
		return
	}

	logger.Infof("Stopping supervisor...")
	running = false
	logger.Successf("Supervisor stopped successfully")
}

func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return running
}
