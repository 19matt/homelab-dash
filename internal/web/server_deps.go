package web

import (
	"time"

	"github.com/homelab/homelab-dash/internal/config"
	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/store"
	"github.com/homelab/homelab-dash/internal/web/handlers"
)

// ServerDependencies holds the core dependencies for the web server.
type ServerDependencies struct {
	Store          *store.Store
	Hub            *Hub
	VMCollectors   []*proxmox.VMCollector
	JellyfinClient handlers.JellyfinClient
	FrigateClient  handlers.FrigateClient
	ConfigManager  *config.Manager
	Integrations   []string
}

// ServerVersion holds version information for the server.
type ServerVersion struct {
	Version   string
	BuildTime string
	StartTime time.Time
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Config config.ServerConfig
}
