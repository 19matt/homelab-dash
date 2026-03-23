package config

import "sync"

// Manager provides thread-safe access to the config.
type Manager struct {
	mu        sync.RWMutex
	cfg       *Config
	listeners []chan ConfigChangeEvent
}

// ConfigChangeEvent represents a change in the configuration.
type ConfigChangeEvent struct {
	Type      ChangeType
	Target    TargetConfig
	OldTarget *TargetConfig // For UPDATE type, the old value
}

// ChangeType indicates the type of config change.
type ChangeType string

const (
	ChangeTypeAdded   ChangeType = "added"
	ChangeTypeRemoved ChangeType = "removed"
	ChangeTypeUpdated ChangeType = "updated"
)

// NewManager creates a new config manager.
func NewManager(cfg *Config) *Manager {
	return &Manager{
		cfg:       cfg,
		listeners: make([]chan ConfigChangeEvent, 0),
	}
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

	// Store old targets for comparison if needed
	oldTargets := make([]TargetConfig, len(m.cfg.Targets))
	copy(oldTargets, m.cfg.Targets)

	fn(m.cfg)

	if err := m.cfg.Save(); err != nil {
		return err
	}

	// Notify listeners of changes
	m.notifyChanges(oldTargets, m.cfg.Targets)

	return nil
}

// NotifyRegister registers a channel to receive config change notifications.
func (m *Manager) NotifyRegister() chan ConfigChangeEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan ConfigChangeEvent, 10) // Buffered channel
	m.listeners = append(m.listeners, ch)
	return ch
}

// notifyChanges compares old and new target lists and sends notifications.
// Called with m.mu locked.
func (m *Manager) notifyChanges(oldTargets, newTargets []TargetConfig) {
	// Create maps for easy lookup
	oldMap := make(map[string]TargetConfig)
	for _, t := range oldTargets {
		oldMap[t.Name] = t
	}

	newMap := make(map[string]TargetConfig)
	for _, t := range newTargets {
		newMap[t.Name] = t
	}

	// Find added targets
	for _, t := range newTargets {
		if _, exists := oldMap[t.Name]; !exists {
			event := ConfigChangeEvent{
				Type:   ChangeTypeAdded,
				Target: t,
			}
			// Send to all listeners (non-blocking)
			for _, ch := range m.listeners {
				select {
				case ch <- event:
				default:
					// Skip if channel is full to avoid blocking
				}
			}
		}
	}

	// Find removed targets
	for _, t := range oldTargets {
		if _, exists := newMap[t.Name]; !exists {
			event := ConfigChangeEvent{
				Type:      ChangeTypeRemoved,
				Target:    t,
				OldTarget: &t,
			}
			// Send to all listeners (non-blocking)
			for _, ch := range m.listeners {
				select {
				case ch <- event:
				default:
					// Skip if channel is full to avoid blocking
				}
			}
		}
	}

	// Find updated targets
	for _, t := range newTargets {
		if oldT, exists := oldMap[t.Name]; exists {
			// Check if target actually changed (simplified comparison)
			if oldT.Host != t.Host ||
				len(oldT.Checks) != len(t.Checks) ||
				len(oldT.Ports) != len(t.Ports) {
				event := ConfigChangeEvent{
					Type:      ChangeTypeUpdated,
					Target:    t,
					OldTarget: &oldT,
				}
				// Send to all listeners (non-blocking)
				for _, ch := range m.listeners {
					select {
					case ch <- event:
					default:
						// Skip if channel is full to avoid blocking
					}
				}
			}
		}
	}
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
