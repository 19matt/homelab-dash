package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	content := `server:
  host: "0.0.0.0"
  port: 8080
  auth:
    enabled: false
    username: ""
    password: ""
defaults:
  interval: "60s"
targets:
  - name: "test"
    host: "127.0.0.1"
    checks: [ping]
    ports: [80]
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host '0.0.0.0', got '%s'", cfg.Server.Host)
	}

	if cfg.Interval != 60_000_000_000 { // 60s in nanoseconds
		t.Errorf("expected interval 60s, got %v", cfg.Interval)
	}

	if len(cfg.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(cfg.Targets))
	}
}

func TestValidate_MissingHost(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Host: "", Port: 8080},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing host")
	}
}

func TestValidate_MissingPassword(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
			Auth: AuthConfig{Enabled: true, Username: "admin", Password: ""},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing password")
	}
}

func TestValidate_DuplicateTargets(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 8080},
		Targets: []TargetConfig{
			{Name: "test", Host: "127.0.0.1", Ports: []interface{}{80}},
			{Name: "test", Host: "127.0.0.2", Ports: []interface{}{80}},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for duplicate target names")
	}
}

func TestParsePorts_Int(t *testing.T) {
	ports := []interface{}{80, 443}
	endpoints, err := parsePorts(ports)
	if err != nil {
		t.Fatalf("parsePorts failed: %v", err)
	}

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	if endpoints[0].Port != 80 || endpoints[0].Protocol != "http" {
		t.Errorf("expected http:80, got %s:%d", endpoints[0].Protocol, endpoints[0].Port)
	}

	if endpoints[1].Port != 443 || endpoints[1].Protocol != "https" {
		t.Errorf("expected https:443, got %s:%d", endpoints[1].Protocol, endpoints[1].Port)
	}
}

func TestParsePorts_String(t *testing.T) {
	ports := []interface{}{"https:8006"}
	endpoints, err := parsePorts(ports)
	if err != nil {
		t.Fatalf("parsePorts failed: %v", err)
	}

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	if endpoints[0].Port != 8006 || endpoints[0].Protocol != "https" {
		t.Errorf("expected https:8006, got %s:%d", endpoints[0].Protocol, endpoints[0].Port)
	}
}

func TestParsePorts_Mixed(t *testing.T) {
	ports := []interface{}{80, "https:443", "http:8080"}
	endpoints, err := parsePorts(ports)
	if err != nil {
		t.Fatalf("parsePorts failed: %v", err)
	}

	if len(endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(endpoints))
	}
}

func TestInferProtocol(t *testing.T) {
	tests := []struct {
		port     int
		expected string
	}{
		{80, "http"},
		{443, "https"},
		{8080, "http"},
		{8443, "http"},
	}

	for _, tt := range tests {
		result := inferProtocol(tt.port)
		if result != tt.expected {
			t.Errorf("inferProtocol(%d) = %s, want %s", tt.port, result, tt.expected)
		}
	}
}

func TestManager_AddRemoveTarget(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 8080},
	}

	dir := t.TempDir()
	cfg.configPath = filepath.Join(dir, "test.yaml")
	cfg.Save()

	mgr := NewManager(cfg)

	target := TargetConfig{
		Name:   "test",
		Host:   "127.0.0.1",
		Checks: []string{"ping"},
		Ports:  []interface{}{80},
	}

	if err := mgr.AddTarget(target); err != nil {
		t.Fatalf("AddTarget failed: %v", err)
	}

	if len(mgr.Targets()) != 1 {
		t.Errorf("expected 1 target, got %d", len(mgr.Targets()))
	}

	found, err := mgr.RemoveTarget("test")
	if err != nil {
		t.Fatalf("RemoveTarget failed: %v", err)
	}

	if !found {
		t.Error("expected target to be found")
	}

	if len(mgr.Targets()) != 0 {
		t.Errorf("expected 0 targets, got %d", len(mgr.Targets()))
	}
}
