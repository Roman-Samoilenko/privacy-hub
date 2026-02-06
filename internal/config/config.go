package config

import (
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/api/types/container"
	"gopkg.in/yaml.v3"
)

type DNSConfig struct {
	Listen          string        `yaml:"listen"`
	Upstreams       []string      `yaml:"upstreams"`
	DoHUpstreams    []string      `yaml:"doh_upstreams"`
	Timeout         time.Duration `yaml:"timeout"`
	CacheSize       int           `yaml:"cache_size"`
	CacheTTL        int           `yaml:"cache_ttl"`
	EnableFiltering bool          `yaml:"enable_filtering"`
	Blocklist       []string      `yaml:"blocklist"`
	Allowlist       []string      `yaml:"allowlist"`
}

type ProxyConfig struct {
	Listen            string   `yaml:"listen"`
	FilterHeads       bool     `yaml:"filter_headers"`
	UserAgent         string   `yaml:"user_agent"`
	FilterListHeaders []string `yaml:"filter_list_headers"`
	MITMEnabled       bool     `yaml:"mitm_enabled"`
	MITMCACert        string   `yaml:"mitm_ca_cert"`
	MITMCAKey         string   `yaml:"mitm_ca_key"`
}

type APIConfig struct {
	Listen      string `yaml:"listen"`
	CORSEnabled bool   `yaml:"cors_enabled"`
	RateLimit   int    `yaml:"rate_limit"`
	APIKey      string `yaml:"api_key"`
}

type DockerContainerConfig struct {
	Name          string                      `yaml:"name"`
	Listen        int                         `yaml:"listen"`
	Image         string                      `yaml:"image"`
	Network       string                      `yaml:"network"`
	RestartPolicy container.RestartPolicyMode `yaml:"restart_policy"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
	Format string `yaml:"format"`
}

type Config struct {
	DNS             DNSConfig             `yaml:"dns"`
	Proxy           ProxyConfig           `yaml:"proxy"`
	API             APIConfig             `yaml:"api"`
	DockerContainer DockerContainerConfig `yaml:"docker_container"`
	Logging         LoggingConfig         `yaml:"logging"`
}

func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.DNS.Listen == "" {
		return fmt.Errorf("dns.listen is required")
	}
	if len(c.DNS.Upstreams) == 0 && len(c.DNS.DoHUpstreams) == 0 {
		return fmt.Errorf("at least one upstream is required")
	}
	if c.Proxy.Listen == "" {
		return fmt.Errorf("proxy.listen is required")
	}
	if c.API.Listen == "" {
		return fmt.Errorf("api.listen is required")
	}
	if c.DockerContainer.Name == "" {
		return fmt.Errorf("docker_container.name is required")
	}
	return nil
}
