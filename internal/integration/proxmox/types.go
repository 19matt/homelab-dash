package proxmox

import "encoding/json"

// NodeStatus represents the current status of a Proxmox node.
type NodeStatus struct {
	CPU     float64    `json:"cpu"`
	Memory  MemoryInfo `json:"memory"`
	Uptime  int64      `json:"uptime"`
	LoadAvg [3]float64 `json:"loadavg"`
}

// MemoryInfo holds memory statistics.
type MemoryInfo struct {
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

// RRDEntry represents a single RRD data point from Proxmox.
type RRDEntry struct {
	Time     int64   `json:"time"`
	CPU      float64 `json:"cpu"`
	Mem      float64 `json:"mem"`
	NetIn    float64 `json:"netin"`
	NetOut   float64 `json:"netout"`
	MemTotal float64 `json:"memtotal"`
	MemUsed  float64 `json:"memused"`
}

// VMSummary represents a VM or LXC container.
type VMSummary struct {
	VMID    int     `json:"vmid"`
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	CPU     float64 `json:"cpu"`
	Mem     uint64  `json:"mem"`
	MaxMem  uint64  `json:"maxmem"`
	Disk    uint64  `json:"disk"`
	MaxDisk uint64  `json:"maxdisk"`
	Uptime  int64   `json:"uptime"`
	NetIn   uint64  `json:"netin"`
	NetOut  uint64  `json:"netout"`
	Type    string  `json:"-"` // "qemu" or "lxc", set during parsing
}

// proxmoxResponse wraps the Proxmox API JSON response structure.
type proxmoxResponse struct {
	Data json.RawMessage `json:"data"`
}
