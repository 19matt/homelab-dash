package checker

import (
	"context"
	"time"
)

// Status represents the result of a health check.
type Status int

const (
	StatusPass Status = iota
	StatusWarn
	StatusFail
	StatusUnknown
)

// String returns the string representation of a Status.
func (s Status) String() string {
	switch s {
	case StatusPass:
		return "pass"
	case StatusWarn:
		return "warn"
	case StatusFail:
		return "fail"
	default:
		return "unknown"
	}
}

// CheckResult represents the outcome of a single health check.
type CheckResult struct {
	Timestamp time.Time
	Target    string
	Check     string
	Status    Status
	Message   string
	Latency   time.Duration
}

// Checker performs a health check and returns a CheckResult.
type Checker interface {
	Name() string
	Check(ctx context.Context) (CheckResult, error)
}
