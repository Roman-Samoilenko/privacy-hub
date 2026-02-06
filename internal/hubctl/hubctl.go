package hubctl

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"context"

	"github.com/Roman-Samoilenko/privacy-hub/internal/api"
	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
)

var isDNSRedirect bool = false

const (
	DNSDPort = 53
)

func DNSControl(ctx context.Context, dockerContainerConf config.DockerContainerConfig) {
	targetPort := dockerContainerConf.Listen

	for {
		select {
		case isReady := <-api.ReadyChan:
			switch isReady {
			case true && !isDNSRedirect:
				logger.Infof("Setting up DNS redirection to port %d", targetPort)
				dnsRedirect(targetPort)
			case false && isDNSRedirect:
				logger.Infof("Removing DNS redirection from port %d", targetPort)
				removeDNSRedirect(targetPort)
			}
		case <-ctx.Done():
			logger.Infof("DNSControl shutting down...")
			removeDNSRedirect(targetPort)
			return
		}
	}
}

func runCommand(cmd string) error {
	parts := strings.Fields(cmd)
	return exec.CommandContext(context.Background(), parts[0], parts[1:]...).Run()
}

func dnsRedirect(targetPort int) {
	commands := []string{

		fmt.Sprintf("iptables -t nat -A OUTPUT -p udp --dport %d -j REDIRECT --to-port %d",
			DNSDPort, targetPort),

		fmt.Sprintf("iptables -t nat -A OUTPUT -p tcp --dport %d -j REDIRECT --to-port %d",
			DNSDPort, targetPort+1),

		fmt.Sprintf("iptables -t nat -I OUTPUT -p udp --dport %d -m owner --uid-owner %d -j ACCEPT",
			DNSDPort, os.Getuid()),
	}
	for _, cmdStr := range commands {
		if err := runCommand(cmdStr); err != nil {
			logger.Errorf("Failed to run command '%s': %v", cmdStr, err)
			removeDNSRedirect(targetPort)
			return
		} else {
			logger.Successf("ran command: %s", cmdStr)
		}
	}
	isDNSRedirect = true
}

func removeDNSRedirect(targetPort int) {
	cleanupCommands := []string{
		fmt.Sprintf("iptables -t nat -D OUTPUT -p udp --dport %d -j REDIRECT --to-port %d",
			DNSDPort, targetPort),
		fmt.Sprintf("iptables -t nat -D OUTPUT -p tcp --dport %d -j REDIRECT --to-port %d",
			DNSDPort, targetPort+1),
		fmt.Sprintf("iptables -t nat -D OUTPUT -p udp --dport %d -m owner --uid-owner %d -j ACCEPT",
			DNSDPort, os.Getuid()),
	}
	for _, cmdStr := range cleanupCommands {
		if err := runCommand(cmdStr); err != nil {
			logger.Errorf("Failed to run command '%s': %v", cmdStr, err)
		} else {
			logger.Successf("ran command: %s", cmdStr)
		}
	}
	isDNSRedirect = false
}
