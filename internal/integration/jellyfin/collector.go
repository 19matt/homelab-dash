package jellyfin

import (
	"context"
	"fmt"
	"time"

	"github.com/homelab/homelab-dash/internal/collector"
)

// Collector collects metrics from Jellyfin.
type Collector struct {
	client *Client
}

// NewCollector creates a Jellyfin collector.
func NewCollector(client *Client) *Collector {
	return &Collector{client: client}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "jellyfin"
}

// Collect fetches Jellyfin metrics and returns DataPoints.
func (c *Collector) Collect(ctx context.Context) ([]collector.DataPoint, error) {
	now := time.Now()
	target := "jellyfin"

	var points []collector.DataPoint

	// Get sessions
	sessions, err := c.client.GetSessions(ctx)
	if err == nil {
		active := 0
		transcoding := 0
		for _, s := range sessions {
			if s.NowPlayingItem != nil {
				active++
				if s.TranscodingInfo != nil {
					transcoding++
				}
			}
		}
		points = append(points,
			collector.DataPoint{Timestamp: now, Target: target, Metric: "jellyfin.sessions.active", Value: float64(active)},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "jellyfin.sessions.transcoding", Value: float64(transcoding)},
		)
	}

	// Get item counts
	counts, err := c.client.GetItemCounts(ctx)
	if err == nil {
		points = append(points,
			collector.DataPoint{Timestamp: now, Target: target, Metric: "jellyfin.library.movies", Value: float64(counts.MovieCount)},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "jellyfin.library.series", Value: float64(counts.SeriesCount)},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "jellyfin.library.episodes", Value: float64(counts.EpisodeCount)},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "jellyfin.library.songs", Value: float64(counts.SongCount)},
		)
	}

	if len(points) == 0 {
		return nil, fmt.Errorf("jellyfin: no data collected")
	}

	return points, nil
}
