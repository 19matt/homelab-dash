package handlers

import (
	"context"

	"github.com/homelab/homelab-dash/internal/integration/frigate"
	"github.com/homelab/homelab-dash/internal/integration/jellyfin"
)

// JellyfinClient defines the interface for Jellyfin API operations.
type JellyfinClient interface {
	GetSystemInfo(ctx context.Context) (jellyfin.SystemInfo, error)
	GetSessions(ctx context.Context) ([]jellyfin.Session, error)
	GetItemCounts(ctx context.Context) (jellyfin.ItemCounts, error)
}

// FrigateClient defines the interface for Frigate API operations.
type FrigateClient interface {
	GetVersion(ctx context.Context) (string, error)
	GetStats(ctx context.Context) (frigate.Stats, error)
}
