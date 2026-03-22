package frigate

import (
	"context"
	"fmt"
	"time"

	"github.com/homelab/homelab-dash/internal/collector"
)

// Collector collects metrics from Frigate.
type Collector struct {
	client *Client
}

// NewCollector creates a Frigate collector.
func NewCollector(client *Client) *Collector {
	return &Collector{client: client}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "frigate"
}

// Collect fetches Frigate stats and returns DataPoints.
func (c *Collector) Collect(ctx context.Context) ([]collector.DataPoint, error) {
	stats, err := c.client.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("frigate: %w", err)
	}

	now := time.Now()
	target := "frigate"

	var points []collector.DataPoint

	// Service-level metrics
	points = append(points,
		collector.DataPoint{Timestamp: now, Target: target, Metric: "frigate.uptime", Value: float64(stats.Service.Uptime)},
		collector.DataPoint{Timestamp: now, Target: target, Metric: "frigate.cameras.count", Value: float64(len(stats.Cameras))},
	)

	activeCameras := 0
	for name, cam := range stats.Cameras {
		if cam.DetectionEnabled {
			activeCameras++
		}
		cameraTarget := fmt.Sprintf("frigate:%s", name)
		points = append(points,
			collector.DataPoint{Timestamp: now, Target: cameraTarget, Metric: "frigate.camera.fps", Value: cam.CameraFPS},
			collector.DataPoint{Timestamp: now, Target: cameraTarget, Metric: "frigate.camera.detection_fps", Value: cam.DetectionFPS},
			collector.DataPoint{Timestamp: now, Target: cameraTarget, Metric: "frigate.camera.process_fps", Value: cam.ProcessFPS},
			collector.DataPoint{Timestamp: now, Target: cameraTarget, Metric: "frigate.camera.skipped_fps", Value: cam.SkippedFPS},
		)
	}

	points = append(points,
		collector.DataPoint{Timestamp: now, Target: target, Metric: "frigate.cameras.active", Value: float64(activeCameras)},
	)

	// Detector metrics
	for name, det := range stats.Detectors {
		detTarget := fmt.Sprintf("frigate:detector:%s", name)
		points = append(points,
			collector.DataPoint{Timestamp: now, Target: detTarget, Metric: "frigate.detector.inference_speed", Value: det.InferenceSpeed},
		)
	}

	return points, nil
}
