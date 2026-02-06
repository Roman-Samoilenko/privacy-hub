package supervisor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/api"
	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/hubctl"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/Roman-Samoilenko/privacy-hub/internal/proxyserver"
)

type Supervisor struct {
	cfg         *config.Config
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	dockerMgr   *hubctl.DockerManager
	iptablesMgr *hubctl.IPTablesManager
	running     bool
	mu          sync.Mutex
}

func New(cfg *config.Config) *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())

	dockerMgr, err := hubctl.NewDockerManager(cfg.DockerContainer)
	if err != nil {
		logger.Errorf("Failed to create docker manager: %v", err)
		cancel()
		return nil
	}

	return &Supervisor{
		cfg:         cfg,
		ctx:         ctx,
		cancel:      cancel,
		dockerMgr:   dockerMgr,
		iptablesMgr: hubctl.NewIPTablesManager(cfg.DockerContainer),
	}
}

func (s *Supervisor) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("supervisor already running")
	}
	s.running = true
	s.mu.Unlock()

	logger.Infof("Starting supervisor...")

	// Start DNS container
	if err := s.dockerMgr.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start DNS container: %v", err)
	}

	// Wait for container to be ready
	if err := s.dockerMgr.WaitReady(s.ctx, 30*time.Second); err != nil {
		return fmt.Errorf("DNS container not ready: %v", err)
	}

	// Setup iptables
	if err := s.iptablesMgr.Setup(); err != nil {
		return fmt.Errorf("failed to setup iptables: %v", err)
	}

	// Start API server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := api.Start(s.ctx, s.cfg.API); err != nil {
			logger.Errorf("API server error: %v", err)
		}
	}()

	// Start Proxy server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := proxyserver.Start(s.ctx, s.cfg.Proxy); err != nil {
			logger.Errorf("Proxy server error: %v", err)
		}
	}()

	logger.Successf("Supervisor started successfully")
	return nil
}

func (s *Supervisor) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	logger.Infof("Stopping supervisor...")

	// Cancel context
	s.cancel()

	// Wait for services with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Infof("All services stopped")
	case <-time.After(10 * time.Second):
		logger.Warnf("Shutdown timeout, forcing exit")
	}

	// Cleanup iptables
	if err := s.iptablesMgr.Cleanup(); err != nil {
		logger.Errorf("iptables cleanup error: %v", err)
	}

	// Stop container
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.dockerMgr.Stop(stopCtx); err != nil {
		logger.Errorf("Failed to stop DNS container: %v", err)
	}

	// Close docker client
	if err := s.dockerMgr.Close(); err != nil {
		logger.Errorf("Failed to close docker client: %v", err)
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	logger.Successf("Supervisor stopped successfully")
	return nil
}

func (s *Supervisor) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
