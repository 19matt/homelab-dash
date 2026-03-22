package scanner

import (
	"context"
	"time"

	"github.com/homelab/homelab-dash/internal/config"
)

// Severity represents the severity level of a security finding.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// String returns the string representation of a Severity.
func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "critical"
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow:
		return "low"
	default:
		return "info"
	}
}

// BadgeClass returns the CSS badge class for a severity.
func (s Severity) BadgeClass() string {
	switch s {
	case SeverityCritical:
		return "badge-critical"
	case SeverityHigh:
		return "badge-fail"
	case SeverityMedium:
		return "badge-warn"
	case SeverityLow:
		return "badge-low"
	default:
		return "badge-unknown"
	}
}

// Finding represents a security finding from a scanner.
type Finding struct {
	ID          int64     `json:"id,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Target      string    `json:"target"`
	Scanner     string    `json:"scanner"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Severity    Severity  `json:"severity"`
	Remediation string    `json:"remediation"`
}

// Scanner defines the interface for security scanners.
type Scanner interface {
	Name() string
	Scan(ctx context.Context, target config.TargetConfig) ([]Finding, error)
}
