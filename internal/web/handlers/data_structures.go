package handlers

import (
	"html/template"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/store"
)

// PageData is the base data passed to all page templates.
type PageData struct {
	ActivePage string
	Content    template.HTML
}

// OverviewData is the data for the overview page.
type OverviewData struct {
	PageData
	NodesRunning    int
	VMsRunning      int
	VMsStopped      int
	ServicesUp      int
	ServicesTotal   int
	FindingsTotal   int
	StatusResults   []checker.CheckResult
	VMs             []proxmox.VMSummary
	Nodes           []NodeStat
	JellyfinEnabled bool
	FrigateEnabled  bool
}

// NodeStat holds stats for a Proxmox node.
type NodeStat struct {
	Name       string
	CPUPercent float64
	MemPercent float64
	Uptime     string
	Status     string
}

// ServiceRow is a row in the services table.
type ServiceRow struct {
	Target      string
	Check       string
	Status      string
	LatencyMs   int64
	Uptime24h   float64
	Uptime7d    float64
	Timestamp   time.Time
	DailyUptime []store.DailyUptime
}

// SparklineDay is a single day in a 90-day sparkline.
type SparklineDay struct {
	Date     string
	Percent  float64
	DayIndex int
}

// ServicesData is the data for the services page.
type ServicesData struct {
	PageData
	Rows []ServiceRow
}

// ProxmoxData is the data for the proxmox page.
type ProxmoxData struct {
	PageData
	Nodes        []NodeStat
	VMs          []proxmox.VMSummary
	VMUptimeBars map[string][]store.DailyUptime
}

// MetricsData is the data for the metrics page.
type MetricsData struct {
	PageData
	Targets []string
}

// HostData is the data for the host detail page.
type HostData struct {
	PageData
	Target        string
	IsVM          bool
	VMID          string
	VMType        string
	VMStatus      string
	VMUptime      string
	LatestMetrics map[string]float64
	RecentChecks  []checker.CheckResult
	Findings      []FindingRow
}

// SecurityData is the data for the security page.
type SecurityData struct {
	PageData
	Findings []FindingRow
	Summary  map[string]int
	Targets  []string
	Scanners []string
}

// AlertsData is the data for the alerts page.
type AlertsData struct {
	PageData
	Events []AlertEventRow
}

// AlertEventRow is a row in the alerts table.
type AlertEventRow struct {
	ID        int64
	Timestamp time.Time
	RuleName  string
	Target    string
	Message   string
	Severity  string
}

// FindingRow is a row in the findings table.
type FindingRow struct {
	ID          int64
	Timestamp   time.Time
	Target      string
	Scanner     string
	Title       string
	Description string
	Severity    int
	Remediation string
}
