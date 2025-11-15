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

func DNSControl(ctx context.Context, DockerContainerConf config.DockerContainerConfig) {
	targetPort := DockerContainerConf.Listen

	for {
		select {
		case ready := <-api.ReadyChan:
			switch ready {
			case true:
				logger.Info("Setting up DNS redirection to port %d", targetPort)
				dnsRedirect(targetPort)
			case false:
				logger.Info("Removing DNS redirection from port %d", targetPort)
				removeDNSRedirect(targetPort)
			}
		case <-ctx.Done():
			logger.Info("DNSControl shutting down...")
			removeDNSRedirect(targetPort)
			return
		}
	}
}

func runCommand(cmd string) error {
	parts := strings.Fields(cmd)
	return exec.Command(parts[0], parts[1:]...).Run()
}

func dnsRedirect(targetPort int) {
	commands := []string{
		// Redirect UDP DNS to our custom port
		fmt.Sprintf("iptables -t nat -A OUTPUT -p udp --dport %d -j REDIRECT --to-port %d",
			53, targetPort),
		// Redirect TCP DNS
		fmt.Sprintf("iptables -t nat -A OUTPUT -p tcp --dport %d -j REDIRECT --to-port %d",
			53, targetPort+1), // +1 для TCP чтобы не конфликтовать
		// Exclude our own process
		fmt.Sprintf("iptables -t nat -I OUTPUT -p udp --dport %d -m owner --uid-owner %d -j ACCEPT",
			53, os.Getuid()),
	}
	for _, cmdStr := range commands {
		if err := runCommand(cmdStr); err != nil {
			logger.Error("Failed to run command '%s': %v", cmdStr, err)
		} else {
			logger.Success("ran command: %s", cmdStr)
		}

	}
}

func removeDNSRedirect(targetPort int) {

	cleanupCommands := []string{
		fmt.Sprintf("iptables -t nat -D OUTPUT -p udp --dport %d -j REDIRECT --to-port %d",
			53, targetPort),
		fmt.Sprintf("iptables -t nat -D OUTPUT -p tcp --dport %d -j REDIRECT --to-port %d",
			53, targetPort+1),
		fmt.Sprintf("iptables -t nat -D OUTPUT -p udp --dport %d -m owner --uid-owner %d -j ACCEPT",
			53, os.Getuid()),
	}
	for _, cmdStr := range cleanupCommands {
		runCommand(cmdStr)
	}
}
