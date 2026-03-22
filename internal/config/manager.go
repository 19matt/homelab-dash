package config

import "sync"

// Manager provides thread-safe access to the config.
type Manager struct {
	mu  sync.RWMutex
	cfg *Config
}

// NewManager creates a new config manager.
func NewManager(cfg *Config) *Manager {
	return &Manager{cfg: cfg}
}

// Get returns a copy of the current config (read lock).
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Update applies a function to the config under write lock and saves to disk.
func (m *Manager) Update(fn func(*Config)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fn(m.cfg)

	return m.cfg.Save()
}

// Targets returns the current targets list (read lock).
func (m *Manager) Targets() []TargetConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.Targets
}

// AddTarget adds a new target and saves config.
func (m *Manager) AddTarget(t TargetConfig) error {
	return m.Update(func(cfg *Config) {
		cfg.Targets = append(cfg.Targets, t)
	})
}

// RemoveTarget removes a target by name and saves config.
// Returns true if the target was found and removed.
func (m *Manager) RemoveTarget(name string) (bool, error) {
	found := false
	err := m.Update(func(cfg *Config) {
		var newTargets []TargetConfig
		for _, t := range cfg.Targets {
			if t.Name == name {
				found = true
				continue
			}
			newTargets = append(newTargets, t)
		}
		if found {
			cfg.Targets = newTargets
		}
	})
	return found, err
}
