package proxmox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
)

// VMChecker checks the aggregate status of all VMs and containers.
type VMChecker struct {
	client *Client
	node   string
}

// NewVMChecker creates a checker for Proxmox VM status.
func NewVMChecker(client *Client, node string) *VMChecker {
	return &VMChecker{client: client, node: node}
}

// Name returns the check name.
func (c *VMChecker) Name() string {
	return "proxmox.vm.status"
}

// Check returns an aggregate status for all VMs and containers.
func (c *VMChecker) Check(ctx context.Context) (checker.CheckResult, error) {
	start := time.Now()

	qemuVMs, qemuErr := c.client.ListVMs(ctx, c.node)
	lxcContainers, lxcErr := c.client.ListContainers(ctx, c.node)

	latency := time.Since(start)

	result := checker.CheckResult{
		Timestamp: time.Now(),
		Target:    fmt.Sprintf("proxmox:%s", c.node),
		Check:     "proxmox.vm.status",
		Latency:   latency,
	}

	if qemuErr != nil || lxcErr != nil {
		result.Status = checker.StatusFail
		var errs []string
		if qemuErr != nil {
			errs = append(errs, fmt.Sprintf("qemu: %v", qemuErr))
		}
		if lxcErr != nil {
			errs = append(errs, fmt.Sprintf("lxc: %v", lxcErr))
		}
		result.Message = fmt.Sprintf("API error: %s", strings.Join(errs, "; "))
		return result, nil
	}

	allVMs := append(qemuVMs, lxcContainers...)

	stopped := 0
	warn := 0
	for _, vm := range allVMs {
		switch vm.Status {
		case "running":
			// good
		case "stopped":
			stopped++
		default:
			warn++
		}
	}

	total := len(allVMs)
	running := total - stopped - warn

	switch {
	case total == 0:
		result.Status = checker.StatusWarn
		result.Message = "no VMs or containers found"
	case stopped == 0 && warn == 0:
		result.Status = checker.StatusPass
		result.Message = fmt.Sprintf("all %d guests running", total)
	case stopped > 0:
		result.Status = checker.StatusWarn
		result.Message = fmt.Sprintf("%d/%d running, %d stopped, %d other", running, total, stopped, warn)
	default:
		result.Status = checker.StatusWarn
		result.Message = fmt.Sprintf("%d/%d running, %d other", running, total, warn)
	}

	return result, nil
}
