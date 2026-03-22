package frigate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/homelab/homelab-dash/internal/config"
)

// Client is an HTTP client for the Frigate API.
type Client struct {
	host string
	http *http.Client
}

// NewClient creates a Frigate API client.
func NewClient(cfg config.FrigateConfig) *Client {
	return &Client{
		host: cfg.Host,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// get performs a GET request.
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	url := c.host + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("frigate: create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("frigate: request %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("frigate: request %s: status %d", path, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("frigate: decode %s: %w", path, err)
	}

	return nil
}

// GetVersion returns the Frigate version string.
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	var raw json.RawMessage
	err := c.get(ctx, "/api/version", &raw)
	if err != nil {
		return "", err
	}

	// Try string first, then number
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str, nil
	}

	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		return fmt.Sprintf("%.2f", num), nil
	}

	return string(raw), nil
}

// GetStats returns system and camera statistics.
func (c *Client) GetStats(ctx context.Context) (Stats, error) {
	var stats Stats
	err := c.get(ctx, "/api/stats", &stats)
	return stats, err
}

// Stats is the response from /api/stats.
type Stats struct {
	Service   ServiceStats             `json:"service"`
	Cameras   map[string]CameraStats   `json:"cameras"`
	Detectors map[string]DetectorStats `json:"detectors"`
}

// ServiceStats holds service-level statistics.
type ServiceStats struct {
	Uptime int `json:"uptime"`
}

// CameraStats holds per-camera statistics.
type CameraStats struct {
	CameraFPS        float64 `json:"camera_fps"`
	DetectionFPS     float64 `json:"detection_fps"`
	ProcessFPS       float64 `json:"process_fps"`
	SkippedFPS       float64 `json:"skipped_fps"`
	DetectionEnabled bool    `json:"detection_enabled"`
}

// DetectorStats holds per-detector statistics.
type DetectorStats struct {
	InferenceSpeed float64 `json:"inference_speed"`
	DetectorStart  int     `json:"detector_start"`
}
