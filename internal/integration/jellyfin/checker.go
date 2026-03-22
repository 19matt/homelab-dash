package jellyfin

import (
	"context"
	"fmt"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
)

// Checker checks Jellyfin server health.
type Checker struct {
	client *Client
}

// NewChecker creates a Jellyfin health checker.
func NewChecker(client *Client) *Checker {
	return &Checker{client: client}
}

// Name returns the check name.
func (c *Checker) Name() string {
	return "jellyfin.health"
}

// Check verifies Jellyfin is reachable and returns system info.
func (c *Checker) Check(ctx context.Context) (checker.CheckResult, error) {
	start := time.Now()

	info, err := c.client.GetSystemInfo(ctx)
	latency := time.Since(start)

	result := checker.CheckResult{
		Timestamp: time.Now(),
		Target:    "jellyfin",
		Check:     "jellyfin.health",
		Latency:   latency,
	}

	if err != nil {
		result.Status = checker.StatusFail
		result.Message = fmt.Sprintf("unreachable: %v", err)
		return result, nil
	}

	result.Status = checker.StatusPass
	result.Message = fmt.Sprintf("%s v%s OK", info.ServerName, info.Version)
	return result, nil
}
