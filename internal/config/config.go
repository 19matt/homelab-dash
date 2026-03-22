package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure.
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Defaults     DefaultsConfig     `yaml:"defaults"`
	Targets      []TargetConfig     `yaml:"targets"`
	Integrations IntegrationsConfig `yaml:"integrations"`
	Alerts       []AlertConfig      `yaml:"alerts"`

	// Parsed from Defaults.Interval, defaults to 60s.
	Interval time.Duration `yaml:"-"`
	// Parsed from Defaults.ScanInterval, defaults to 6h.
	ScanInterval time.Duration `yaml:"-"`
	// Parsed from Defaults.AlertInterval, defaults to Interval.
	AlertInterval time.Duration `yaml:"-"`
}

// DefaultsConfig holds global default settings.
type DefaultsConfig struct {
	Interval      string `yaml:"interval"`
	ScanInterval  string `yaml:"scan_interval"`
	AlertInterval string `yaml:"alert_interval"`
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
	Name string `yaml:"name"`
	Host string `yaml:"host"`

	Checks []string `yaml:"checks"`

	// Ports supports both int and "protocol:port" string formats.
	// Parsed into Endpoints during validation.
	Ports    []interface{} `yaml:"ports"`
	Endpoint []Endpoint    `yaml:"-"`
}

// Endpoint represents a protocol+port pair for a target.
type Endpoint struct {
	Port     int
	Protocol string // "http" or "https"
}

// IntegrationsConfig holds optional integration modules.
type IntegrationsConfig struct {
	Proxmox  ProxmoxConfig  `yaml:"proxmox"`
	Jellyfin JellyfinConfig `yaml:"jellyfin"`
	Frigate  FrigateConfig  `yaml:"frigate"`
	NAS      NASConfig      `yaml:"nas"`
}

// ProxmoxConfig holds Proxmox API connection settings.
type ProxmoxConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Host        string   `yaml:"host,omitempty"`  // single host (legacy)
	Hosts       []string `yaml:"hosts,omitempty"` // multiple hosts for redundancy
	TokenID     string   `yaml:"token_id"`
	TokenSecret string   `yaml:"token_secret"`
	InsecureTLS bool     `yaml:"insecure_tls"`
	Nodes       []string `yaml:"nodes"`
}

// AllHosts returns the list of hosts to try, merging Host and Hosts.
func (c ProxmoxConfig) AllHosts() []string {
	if len(c.Hosts) > 0 {
		return c.Hosts
	}
	if c.Host != "" {
		return []string{c.Host}
	}
	return nil
}

// JellyfinConfig holds Jellyfin API connection settings.
type JellyfinConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	APIKey  string `yaml:"api_key"`
}

// FrigateConfig holds Frigate NVR connection settings.
type FrigateConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
}

// NASConfig holds NAS monitoring settings.
type NASConfig struct {
	Enabled      bool     `yaml:"enabled"`
	SmartDevices []string `yaml:"smart_devices"`
}

// AlertConfig defines an alert rule.
type AlertConfig struct {
	Name      string  `yaml:"name"`
	Condition string  `yaml:"condition"`
	Target    string  `yaml:"target,omitempty"`
	Metric    string  `yaml:"metric,omitempty"`
	Threshold float64 `yaml:"threshold,omitempty"`
	Duration  string  `yaml:"duration"`
	Interval  string  `yaml:"interval,omitempty"`
	Webhook   string  `yaml:"webhook,omitempty"`
	Severity  string  `yaml:"severity,omitempty"`
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

	// Parse interval
	if c.Defaults.Interval == "" {
		c.Interval = 60 * time.Second
	} else {
		d, err := time.ParseDuration(c.Defaults.Interval)
		if err != nil {
			return fmt.Errorf("defaults.interval: invalid duration %q: %w", c.Defaults.Interval, err)
		}
		if d <= 0 {
			return fmt.Errorf("defaults.interval must be positive")
		}
		c.Interval = d
	}

	// Parse scan interval
	if c.Defaults.ScanInterval == "" {
		c.ScanInterval = 6 * time.Hour
	} else {
		d, err := time.ParseDuration(c.Defaults.ScanInterval)
		if err != nil {
			return fmt.Errorf("defaults.scan_interval: invalid duration %q: %w", c.Defaults.ScanInterval, err)
		}
		if d <= 0 {
			return fmt.Errorf("defaults.scan_interval must be positive")
		}
		c.ScanInterval = d
	}

	// Parse alert interval (defaults to checker interval)
	if c.Defaults.AlertInterval == "" {
		c.AlertInterval = c.Interval
	} else {
		d, err := time.ParseDuration(c.Defaults.AlertInterval)
		if err != nil {
			return fmt.Errorf("defaults.alert_interval: invalid duration %q: %w", c.Defaults.AlertInterval, err)
		}
		if d <= 0 {
			return fmt.Errorf("defaults.alert_interval must be positive")
		}
		c.AlertInterval = d
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

		// Parse ports into endpoints
		endpoints, err := parsePorts(t.Ports)
		if err != nil {
			return fmt.Errorf("targets[%d].ports: %w", i, err)
		}
		c.Targets[i].Endpoint = endpoints
	}

	if c.Integrations.Proxmox.Enabled {
		if len(c.Integrations.Proxmox.AllHosts()) == 0 {
			return fmt.Errorf("integrations.proxmox.host or hosts is required when enabled")
		}
		if len(c.Integrations.Proxmox.Nodes) == 0 {
			return fmt.Errorf("integrations.proxmox.nodes is required when enabled")
		}
		if c.Integrations.Proxmox.TokenID == "" {
			return fmt.Errorf("integrations.proxmox.token_id is required when enabled")
		}
		if c.Integrations.Proxmox.TokenSecret == "" {
			return fmt.Errorf("integrations.proxmox.token_secret is required when enabled")
		}
	}
	if c.Integrations.Jellyfin.Enabled && c.Integrations.Jellyfin.Host == "" {
		return fmt.Errorf("integrations.jellyfin.host is required when enabled")
	}
	if c.Integrations.Frigate.Enabled && c.Integrations.Frigate.Host == "" {
		return fmt.Errorf("integrations.frigate.host is required when enabled")
	}

	return nil
}

// parsePorts converts mixed int/string port list into Endpoint slice.
func parsePorts(ports []interface{}) ([]Endpoint, error) {
	var endpoints []Endpoint
	for i, p := range ports {
		switch v := p.(type) {
		case int:
			endpoints = append(endpoints, Endpoint{Port: v, Protocol: inferProtocol(v)})
		case string:
			ep, err := parsePortString(v)
			if err != nil {
				return nil, fmt.Errorf("port[%d]: %w", i, err)
			}
			endpoints = append(endpoints, ep)
		default:
			return nil, fmt.Errorf("port[%d]: unsupported type %T (expected int or string)", i, p)
		}
	}
	return endpoints, nil
}

// parsePortString parses "protocol:port" format.
func parsePortString(s string) (Endpoint, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return Endpoint{}, fmt.Errorf("invalid format %q, expected \"protocol:port\"", s)
	}
	protocol := parts[0]
	if protocol != "http" && protocol != "https" {
		return Endpoint{}, fmt.Errorf("unsupported protocol %q (expected http or https)", protocol)
	}
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	if port <= 0 {
		return Endpoint{}, fmt.Errorf("invalid port %q", parts[1])
	}
	return Endpoint{Port: port, Protocol: protocol}, nil
}

// inferProtocol returns "https" for port 443, "http" otherwise.
func inferProtocol(port int) string {
	if port == 443 {
		return "https"
	}
	return "http"
}
