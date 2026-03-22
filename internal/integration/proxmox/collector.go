package proxmox

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/homelab/homelab-dash/internal/collector"
)

// NodeCollector collects node-level metrics from Proxmox.
type NodeCollector struct {
	client *Client
	node   string
}

// NewNodeCollector creates a collector for Proxmox node metrics.
func NewNodeCollector(client *Client, node string) *NodeCollector {
	return &NodeCollector{client: client, node: node}
}

// Name returns the collector name.
func (c *NodeCollector) Name() string {
	return "proxmox.node"
}

// Collect fetches node status and returns DataPoints.
func (c *NodeCollector) Collect(ctx context.Context) ([]collector.DataPoint, error) {
	status, err := c.client.GetNodeStatus(ctx, c.node)
	if err != nil {
		return nil, fmt.Errorf("proxmox.node: %w", err)
	}

	now := time.Now()
	target := fmt.Sprintf("node:%s", c.node)

	memPercent := 0.0
	if status.Memory.Total > 0 {
		memPercent = float64(status.Memory.Used) / float64(status.Memory.Total) * 100
	}

	points := []collector.DataPoint{
		{Timestamp: now, Target: target, Metric: "node.cpu.percent", Value: status.CPU * 100},
		{Timestamp: now, Target: target, Metric: "node.mem.used", Value: float64(status.Memory.Used)},
		{Timestamp: now, Target: target, Metric: "node.mem.total", Value: float64(status.Memory.Total)},
		{Timestamp: now, Target: target, Metric: "node.mem.percent", Value: memPercent},
		{Timestamp: now, Target: target, Metric: "node.uptime", Value: float64(status.Uptime)},
		{Timestamp: now, Target: target, Metric: "node.load1", Value: status.LoadAvg[0]},
		{Timestamp: now, Target: target, Metric: "node.load5", Value: status.LoadAvg[1]},
		{Timestamp: now, Target: target, Metric: "node.load15", Value: status.LoadAvg[2]},
	}

	log.Printf("collector %s: collected %d points for %s", c.Name(), len(points), target)
	return points, nil
}

// VMCollector collects metrics for all VMs and containers from Proxmox.
type VMCollector struct {
	client  *Client
	node    string
	mu      sync.RWMutex
	lastVMs []VMSummary
}

// NewVMCollector creates a collector for Proxmox VM metrics.
func NewVMCollector(client *Client, node string) *VMCollector {
	return &VMCollector{client: client, node: node}
}

// Name returns the collector name.
func (c *VMCollector) Name() string {
	return "proxmox.vms"
}

// LatestVMs returns the most recently collected VM list (thread-safe).
func (c *VMCollector) LatestVMs() []VMSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()
	vms := make([]VMSummary, len(c.lastVMs))
	copy(vms, c.lastVMs)
	return vms
}

// Collect fetches all VMs and containers and returns DataPoints.
func (c *VMCollector) Collect(ctx context.Context) ([]collector.DataPoint, error) {
	qemuVMs, err := c.client.ListVMs(ctx, c.node)
	if err != nil {
		return nil, fmt.Errorf("proxmox.vms: list qemu: %w", err)
	}

	lxcContainers, err := c.client.ListContainers(ctx, c.node)
	if err != nil {
		return nil, fmt.Errorf("proxmox.vms: list lxc: %w", err)
	}

	allVMs := append(qemuVMs, lxcContainers...)

	c.mu.Lock()
	c.lastVMs = make([]VMSummary, len(allVMs))
	copy(c.lastVMs, allVMs)
	c.mu.Unlock()

	now := time.Now()
	var points []collector.DataPoint

	for _, vm := range allVMs {
		target := fmt.Sprintf("vm:%d:%s", vm.VMID, vm.Name)
		labels := map[string]string{
			"vmid":   strconv.Itoa(vm.VMID),
			"name":   vm.Name,
			"type":   vm.Type,
			"status": vm.Status,
		}

		memPercent := 0.0
		if vm.MaxMem > 0 {
			memPercent = float64(vm.Mem) / float64(vm.MaxMem) * 100
		}

		points = append(points,
			collector.DataPoint{Timestamp: now, Target: target, Metric: "vm.cpu.percent", Value: vm.CPU * 100, Labels: labels},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "vm.mem.used", Value: float64(vm.Mem), Labels: labels},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "vm.mem.total", Value: float64(vm.MaxMem), Labels: labels},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "vm.mem.percent", Value: memPercent, Labels: labels},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "vm.disk.used", Value: float64(vm.Disk), Labels: labels},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "vm.disk.total", Value: float64(vm.MaxDisk), Labels: labels},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "vm.netin", Value: float64(vm.NetIn), Labels: labels},
			collector.DataPoint{Timestamp: now, Target: target, Metric: "vm.netout", Value: float64(vm.NetOut), Labels: labels},
		)
	}

	log.Printf("collector %s: collected %d points for %d guests", c.Name(), len(points), len(allVMs))
	return points, nil
}
