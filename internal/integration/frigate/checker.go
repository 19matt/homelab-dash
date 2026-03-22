package frigate

import (
	"context"
	"fmt"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
)

// Checker checks Frigate server health.
type Checker struct {
	client *Client
}

// NewChecker creates a Frigate health checker.
func NewChecker(client *Client) *Checker {
	return &Checker{client: client}
}

// Name returns the check name.
func (c *Checker) Name() string {
	return "frigate.health"
}

// Check verifies Frigate is reachable and returns version info.
func (c *Checker) Check(ctx context.Context) (checker.CheckResult, error) {
	start := time.Now()

	version, err := c.client.GetVersion(ctx)
	latency := time.Since(start)

	result := checker.CheckResult{
		Timestamp: time.Now(),
		Target:    "frigate",
		Check:     "frigate.health",
		Latency:   latency,
	}

	if err != nil {
		result.Status = checker.StatusFail
		result.Message = fmt.Sprintf("unreachable: %v", err)
		return result, nil
	}

	result.Status = checker.StatusPass
	result.Message = fmt.Sprintf("Frigate v%s OK", version)
	return result, nil
}
