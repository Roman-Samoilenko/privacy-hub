package config

import (
	"os"
	"path/filepath"

	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"gopkg.in/yaml.v3"
)

type DNSConfig struct {
	Listen    string   `yaml:"listen"`
	Upstreams []string `yaml:"upstreams"`
}
type APIConfig struct {
	Listen string `yaml:"listen"`
}
type ProxyConfig struct {
	Listen            string   `yaml:"listen"`
	FilterHeads       bool     `yaml:"filter_headers"`
	UserAgent         string   `yaml:"user_agent"`
	FilterListHeaders []string `yaml:"filter_list_headers"`
}

type DockerContainerConfig struct {
	Name   string `yaml:"name"`
	Listen int    `yaml:"listen"`
}

type Config struct {
	DNS             DNSConfig             `yaml:"dns"`
	Proxy           ProxyConfig           `yaml:"proxy"`
	API             APIConfig             `yaml:"api"`
	DockerContainer DockerContainerConfig `yaml:"docker_container"`
}

func Load() (*Config, error) {
	path := filepath.Join("configs", "config.yaml")

	conf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(conf, &config)
	if err != nil {
		return nil, err
	}

	logger.Debugf("\nConfiguration:\n"+
		"  DNS:\n"+
		"    Listen:    %s\n"+
		"    Upstreams: %v\n"+
		"  Proxy:\n"+
		"    Listen:      %s\n"+
		"    FilterHeads: %t\n"+
		"    UserAgent:   %s\n"+
		"  API:\n"+
		"    Listen:      %s",
		config.DNS.Listen,
		config.DNS.Upstreams,
		config.Proxy.Listen,
		config.Proxy.FilterHeads,
		config.Proxy.UserAgent,
		config.API.Listen)

	return &config, nil
}
