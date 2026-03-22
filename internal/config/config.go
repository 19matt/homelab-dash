package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure.
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Targets      []TargetConfig     `yaml:"targets"`
	Integrations IntegrationsConfig `yaml:"integrations"`
	Alerts       []AlertConfig      `yaml:"alerts"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string     `yaml:"host"`
	Port int        `yaml:"port"`
	Auth AuthConfig `yaml:"auth"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// TargetConfig defines a monitored target.
type TargetConfig struct {
	Name   string   `yaml:"name"`
	Host   string   `yaml:"host"`
	Checks []string `yaml:"checks"`
	Ports  []int    `yaml:"ports"`
}

// IntegrationsConfig holds optional integration modules.
type IntegrationsConfig struct {
	Proxmox  ProxmoxConfig  `yaml:"proxmox"`
	Jellyfin JellyfinConfig `yaml:"jellyfin"`
	NAS      NASConfig      `yaml:"nas"`
}

// ProxmoxConfig holds Proxmox API connection settings.
type ProxmoxConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Host        string `yaml:"host"`
	TokenID     string `yaml:"token_id"`
	TokenSecret string `yaml:"token_secret"`
}

// JellyfinConfig holds Jellyfin API connection settings.
type JellyfinConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	APIKey  string `yaml:"api_key"`
}

// NASConfig holds NAS monitoring settings.
type NASConfig struct {
	Enabled      bool     `yaml:"enabled"`
	SmartDevices []string `yaml:"smart_devices"`
}

// AlertConfig defines an alert rule.
type AlertConfig struct {
	Name      string `yaml:"name"`
	Condition string `yaml:"condition"`
	Duration  string `yaml:"duration"`
	Webhook   string `yaml:"webhook"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validation: %w", err)
	}

	return &cfg, nil
}

// Validate checks that all required fields are present and consistent.
func (c *Config) Validate() error {
	if c.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if c.Server.Port <= 0 {
		return fmt.Errorf("server.port must be positive")
	}
	if c.Server.Auth.Enabled {
		if c.Server.Auth.Username == "" {
			return fmt.Errorf("server.auth.username is required when auth is enabled")
		}
		if c.Server.Auth.Password == "" {
			return fmt.Errorf("server.auth.password is required when auth is enabled")
		}
	}

	seen := make(map[string]bool)
	for i, t := range c.Targets {
		if t.Name == "" {
			return fmt.Errorf("targets[%d].name is required", i)
		}
		if t.Host == "" {
			return fmt.Errorf("targets[%d].host is required", i)
		}
		if seen[t.Name] {
			return fmt.Errorf("targets: duplicate name %q", t.Name)
		}
		seen[t.Name] = true
	}

	if c.Integrations.Proxmox.Enabled && c.Integrations.Proxmox.Host == "" {
		return fmt.Errorf("integrations.proxmox.host is required when enabled")
	}
	if c.Integrations.Jellyfin.Enabled && c.Integrations.Jellyfin.Host == "" {
		return fmt.Errorf("integrations.jellyfin.host is required when enabled")
	}

	return nil
}
