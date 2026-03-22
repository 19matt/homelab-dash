package collector

import (
	"context"
	"time"
)

// DataPoint represents a single time-series data point collected from a target.
type DataPoint struct {
	Timestamp time.Time
	Target    string
	Metric    string
	Value     float64
	Labels    map[string]string
}

// Collector produces time-series DataPoints from a source.
type Collector interface {
	Name() string
	Collect(ctx context.Context) ([]DataPoint, error)
}
