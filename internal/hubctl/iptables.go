package hubctl

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"sync"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
)

const DNSPort = 53

type IPTablesManager struct {
	targetPort int
	active     bool
	mu         sync.Mutex
}

func NewIPTablesManager(cfg config.DockerContainerConfig) *IPTablesManager {
	return &IPTablesManager{
		targetPort: cfg.Listen,
		active:     false,
	}
}

func (ipt *IPTablesManager) Setup() error {
	ipt.mu.Lock()
	defer ipt.mu.Unlock()

	if ipt.active {
		logger.Warnf("iptables rules already active")
		return nil
	}

	logger.Infof("Setting up DNS redirection to port %d", ipt.targetPort)

	rules := [][]string{
		// Redirect UDP DNS queries
		{"-t", "nat", "-A", "OUTPUT", "-p", "udp", "--dport", "53",
			"-j", "REDIRECT", "--to-port", strconv.Itoa(ipt.targetPort)},

		// Redirect TCP DNS queries
		{"-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--dport", "53",
			"-j", "REDIRECT", "--to-port", strconv.Itoa(ipt.targetPort + 1)},
	}

	for _, rule := range rules {
		if err := ipt.runIPTablesCommand(rule...); err != nil {
			logger.Errorf("Failed to apply rule: %v", err)
			ipt.Cleanup() // Rollback
			return err
		}
		logger.Successf("Applied: iptables %v", rule)
	}

	ipt.active = true
	return nil
}

func (ipt *IPTablesManager) Cleanup() error {
	ipt.mu.Lock()
	defer ipt.mu.Unlock()

	if !ipt.active {
		return nil
	}

	logger.Infof("Removing DNS redirection rules")

	rules := [][]string{
		{"-t", "nat", "-D", "OUTPUT", "-p", "udp", "--dport", "53",
			"-j", "REDIRECT", "--to-port", strconv.Itoa(ipt.targetPort)},

		{"-t", "nat", "-D", "OUTPUT", "-p", "tcp", "--dport", "53",
			"-j", "REDIRECT", "--to-port", strconv.Itoa(ipt.targetPort + 1)},
	}

	for _, rule := range rules {
		if err := ipt.runIPTablesCommand(rule...); err != nil {
			logger.Warnf("Failed to remove rule: %v", err)
			// Continue anyway
		} else {
			logger.Successf("Removed: iptables %v", rule)
		}
	}

	ipt.active = false
	return nil
}

func (ipt *IPTablesManager) runIPTablesCommand(args ...string) error {
	cmd := exec.CommandContext(context.Background(), "iptables", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, output)
	}
	return nil
}

func (ipt *IPTablesManager) IsActive() bool {
	ipt.mu.Lock()
	defer ipt.mu.Unlock()
	return ipt.active
}
