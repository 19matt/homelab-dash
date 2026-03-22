package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/homelab/homelab-dash/internal/config"
)

// Client is an HTTP client for the Jellyfin API.
type Client struct {
	host   string
	apiKey string
	http   *http.Client
}

// NewClient creates a Jellyfin API client.
func NewClient(cfg config.JellyfinConfig) *Client {
	return &Client{
		host:   cfg.Host,
		apiKey: cfg.APIKey,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// get performs an authenticated GET request.
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	url := c.host + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("jellyfin: create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Emby-Token", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jellyfin: request %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jellyfin: request %s: status %d", path, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("jellyfin: decode %s: %w", path, err)
	}

	return nil
}

// GetSystemInfo returns server system information.
func (c *Client) GetSystemInfo(ctx context.Context) (SystemInfo, error) {
	var info SystemInfo
	err := c.get(ctx, "/System/Info", &info)
	return info, err
}

// GetSessions returns active sessions.
func (c *Client) GetSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	err := c.get(ctx, "/Sessions?ActiveWithinSeconds=960", &sessions)
	return sessions, err
}

// GetItemCounts returns library item counts.
func (c *Client) GetItemCounts(ctx context.Context) (ItemCounts, error) {
	var counts ItemCounts
	err := c.get(ctx, "/Items/Counts", &counts)
	return counts, err
}

// SystemInfo is the response from /System/Info.
type SystemInfo struct {
	ServerName string `json:"ServerName"`
	Version    string `json:"Version"`
}

// Session represents an active Jellyfin session.
type Session struct {
	UserName        string           `json:"UserName"`
	Client          string           `json:"Client"`
	DeviceName      string           `json:"DeviceName"`
	NowPlayingItem  *NowPlayingItem  `json:"NowPlayingItem"`
	PlayState       PlayState        `json:"PlayState"`
	TranscodingInfo *TranscodingInfo `json:"TranscodingInfo"`
}

// NowPlayingItem is the currently playing media.
type NowPlayingItem struct {
	Name string `json:"Name"`
	Type string `json:"Type"`
}

// PlayState is the playback state.
type PlayState struct {
	IsPaused   bool  `json:"IsPaused"`
	PositionMs int64 `json:"PositionTicks"`
}

// TranscodingInfo is present when transcoding is active.
type TranscodingInfo struct {
	VideoCodec string `json:"VideoCodec"`
	AudioCodec string `json:"AudioCodec"`
	Container  string `json:"Container"`
}

// ItemCounts is the response from /Items/Counts.
type ItemCounts struct {
	MovieCount   int `json:"MovieCount"`
	SeriesCount  int `json:"SeriesCount"`
	EpisodeCount int `json:"EpisodeCount"`
	SongCount    int `json:"SongCount"`
}
