package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/integration/frigate"
	"github.com/homelab/homelab-dash/internal/integration/jellyfin"
	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/store"
)

// TemplateFuncMap provides helper functions for templates.
var TemplateFuncMap = template.FuncMap{
	"mulf": func(a, b float64) float64 { return a * b },
	"mul":  func(a, b int) int { return a * b },
	"divf": func(a, b float64, c float64) float64 {
		if b == 0 {
			return 0
		}
		return (a / b) * c
	},
	"tof": func(v uint64) float64 { return float64(v) },
	"severityName": func(s int) string {
		names := map[int]string{0: "info", 1: "low", 2: "medium", 3: "high", 4: "critical"}
		if n, ok := names[s]; ok {
			return n
		}
		return "unknown"
	},
	"severityBadge": func(s int) string {
		badges := map[int]string{0: "badge-unknown", 1: "badge-low", 2: "badge-warn", 3: "badge-fail", 4: "badge-critical"}
		if b, ok := badges[s]; ok {
			return b
		}
		return "badge-unknown"
	},
	"uptimeColor": func(pct float64) string {
		if pct >= 100 {
			return "#22c55e"
		} else if pct >= 95 {
			return "#f59e0b"
		} else if pct > 0 {
			return "#ef4444"
		}
		return "#2a2d3a"
	},
	"barColor": func(pct float64) string {
		if pct > 80 {
			return "#ef4444"
		} else if pct > 50 {
			return "#f59e0b"
		}
		return "#22c55e"
	},
	"formatUptime": func(seconds int) string {
		return formatUptime(int64(seconds))
	},
	"toInt": func(v int) int { return v },
	"add":   func(a, b int) int { return a + b },
	"sub":   func(a, b int) int { return a - b },
}

// renderTemplate executes a named template and returns the HTML.
func renderTemplate(tmpl *template.Template, name string, data interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", name, err)
	}
	return template.HTML(buf.String()), nil
}

// respondError renders an error page and logs the error.
func respondError(w http.ResponseWriter, tmpl *template.Template, err error, msg string) {
	log.Printf("handler error: %s: %v", msg, err)
	w.WriteHeader(http.StatusInternalServerError)
	content := fmt.Sprintf(`<div class="card" style="text-align:center;padding:3rem;">
		<h2>Error</h2>
		<p class="muted">%s</p>
		<p class="muted" style="font-size:0.8rem;margin-top:1rem;">%v</p>
	</div>`, msg, err)
	data := PageData{
		ActivePage: "",
		Content:    template.HTML(content),
	}
	tmpl.ExecuteTemplate(w, "layout", data)
}

// renderOrError renders a template or returns false if rendering fails.
func renderOrError(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) (template.HTML, bool) {
	content, err := renderTemplate(tmpl, name, data)
	if err != nil {
		respondError(w, tmpl, err, "Failed to render page")
		return "", false
	}
	return content, true
}

// fragmentError logs an error and returns an empty HTML response for htmx fragments.
func fragmentError(w http.ResponseWriter, err error, msg string) {
	log.Printf("fragment error: %s: %v", msg, err)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<p class="muted">%s</p>`, msg)
}

// apiError logs an error and returns a JSON error response.
func apiError(w http.ResponseWriter, err error, msg string, status int) {
	log.Printf("api error: %s: %v", msg, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// OverviewHandler renders the overview page.
func OverviewHandler(tmpl *template.Template, s *store.Store, vmCollectors []*proxmox.VMCollector, jellyfinClient *jellyfin.Client, frigateClient *frigate.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results, err := s.GetLatestCheckResults(r.Context())
		if err != nil {
			respondError(w, tmpl, err, "Failed to load check results")
			return
		}

		var servicesUp int
		for _, r := range results {
			if r.Status == checker.StatusPass {
				servicesUp++
			}
		}

		var allVMs []proxmox.VMSummary
		for _, c := range vmCollectors {
			allVMs = append(allVMs, c.LatestVMs()...)
		}

		var running, stopped int
		for _, vm := range allVMs {
			if vm.Status == "running" {
				running++
			} else {
				stopped++
			}
		}

		nodes := getNodeStats(s)

		// Get findings count
		summary, _ := s.GetFindingsSummary(r.Context())
		findingsTotal := 0
		for _, count := range summary {
			findingsTotal += count
		}

		contentData := struct {
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
		}{
			NodesRunning:    len(nodes),
			VMsRunning:      running,
			VMsStopped:      stopped,
			ServicesUp:      servicesUp,
			ServicesTotal:   len(results),
			FindingsTotal:   findingsTotal,
			StatusResults:   results,
			VMs:             allVMs,
			Nodes:           nodes,
			JellyfinEnabled: jellyfinClient != nil,
			FrigateEnabled:  frigateClient != nil,
		}

		content, ok := renderOrError(w, tmpl, "overview-content", contentData)
		if !ok {
			return
		}

		data := OverviewData{
			PageData: PageData{
				ActivePage: "overview",
				Content:    content,
			},
			NodesRunning:    contentData.NodesRunning,
			VMsRunning:      contentData.VMsRunning,
			VMsStopped:      contentData.VMsStopped,
			ServicesUp:      contentData.ServicesUp,
			ServicesTotal:   contentData.ServicesTotal,
			FindingsTotal:   contentData.FindingsTotal,
			StatusResults:   contentData.StatusResults,
			VMs:             contentData.VMs,
			Nodes:           contentData.Nodes,
			JellyfinEnabled: contentData.JellyfinEnabled,
			FrigateEnabled:  contentData.FrigateEnabled,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// ServicesHandler renders the services page.
func ServicesHandler(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results, err := s.GetLatestCheckResults(r.Context())
		if err != nil {
			respondError(w, tmpl, err, "Failed to load services")
			return
		}

		var rows []ServiceRow
		seen := make(map[string]bool)
		for _, cr := range results {
			target := cr.Target
			if seen[target] {
				continue
			}
			seen[target] = true

			uptime24h, _ := s.GetUptimePercent(r.Context(), target, cr.Check, 24*time.Hour)
			uptime7d, _ := s.GetUptimePercent(r.Context(), target, cr.Check, 7*24*time.Hour)
			dailyUptime, _ := s.GetDailyUptime(r.Context(), target, cr.Check, 90)

			rows = append(rows, ServiceRow{
				Target:      target,
				Check:       cr.Check,
				Status:      cr.Status.String(),
				LatencyMs:   cr.Latency.Milliseconds(),
				Uptime24h:   uptime24h,
				Uptime7d:    uptime7d,
				Timestamp:   cr.Timestamp,
				DailyUptime: dailyUptime,
			})
		}

		contentData := struct{ Rows []ServiceRow }{Rows: rows}

		content, ok := renderOrError(w, tmpl, "services-content", contentData)
		if !ok {
			return
		}

		data := ServicesData{
			PageData: PageData{
				ActivePage: "services",
				Content:    content,
			},
			Rows: rows,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// ProxmoxHandler renders the proxmox page.
func ProxmoxHandler(tmpl *template.Template, s *store.Store, vmCollectors []*proxmox.VMCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var allVMs []proxmox.VMSummary
		for _, c := range vmCollectors {
			allVMs = append(allVMs, c.LatestVMs()...)
		}

		nodes := getNodeStats(s)

		// Fetch daily uptime for each VM
		vmUptimeBars := make(map[string][]store.DailyUptime)
		for _, vm := range allVMs {
			target := fmt.Sprintf("vm:%d:%s", vm.VMID, vm.Name)
			uptime, _ := s.GetDailyUptime(r.Context(), target, "proxmox.vm.status", 90)
			if uptime != nil {
				vmUptimeBars[target] = uptime
			}
		}

		contentData := struct {
			Nodes        []NodeStat
			VMs          []proxmox.VMSummary
			VMUptimeBars map[string][]store.DailyUptime
		}{Nodes: nodes, VMs: allVMs, VMUptimeBars: vmUptimeBars}

		content, ok := renderOrError(w, tmpl, "proxmox-content", contentData)
		if !ok {
			return
		}

		data := ProxmoxData{
			PageData: PageData{
				ActivePage: "proxmox",
				Content:    content,
			},
			Nodes:        nodes,
			VMs:          allVMs,
			VMUptimeBars: vmUptimeBars,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// MetricsHandler renders the metrics page.
func MetricsHandler(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vmTargets, _ := s.GetAllTargetsWithMetric(r.Context(), "vm.cpu.percent")
		nodeTargets, _ := s.GetAllTargetsWithMetric(r.Context(), "node.cpu.percent")

		seen := make(map[string]bool)
		var targets []string
		for _, t := range nodeTargets {
			if !seen[t] {
				targets = append(targets, t)
				seen[t] = true
			}
		}
		for _, t := range vmTargets {
			if !seen[t] {
				targets = append(targets, t)
				seen[t] = true
			}
		}
		sort.Strings(targets)

		contentData := struct{ Targets []string }{Targets: targets}

		content, ok := renderOrError(w, tmpl, "metrics-content", contentData)
		if !ok {
			return
		}

		data := MetricsData{
			PageData: PageData{
				ActivePage: "metrics",
				Content:    content,
			},
			Targets: targets,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// HostHandler renders the host detail page.
func HostHandler(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := r.PathValue("target")
		if target == "" {
			http.Error(w, "missing target", http.StatusBadRequest)
			return
		}

		isVM := strings.HasPrefix(target, "vm:")
		var vmID, vmType, vmStatus, vmUptime string
		if isVM {
			parts := strings.SplitN(target, ":", 3)
			if len(parts) == 3 {
				vmID = parts[1]
			}
			vmType = "qemu"

			// Get VM status from recent checks
			checks, _ := s.GetRecentCheckResults(r.Context(), target, 1)
			if len(checks) > 0 {
				if checks[0].Check == "proxmox.vm.status" {
					if checks[0].Status == checker.StatusPass {
						vmStatus = "running"
					} else if checks[0].Status == checker.StatusFail {
						vmStatus = "stopped"
					} else {
						vmStatus = "unknown"
					}
				}
			}
		}

		metrics := make(map[string]float64)
		metricNames := []string{
			"node.cpu.percent", "node.mem.percent", "node.uptime",
			"vm.cpu.percent", "vm.mem.percent", "vm.uptime",
		}
		for _, m := range metricNames {
			dp, _ := s.GetLatestDataPoint(r.Context(), target, m)
			if dp != nil {
				metrics[m] = dp.Value
			}
		}

		// Format uptime
		if uptime, ok := metrics["node.uptime"]; ok && uptime > 0 {
			vmUptime = formatUptime(int64(uptime))
		} else if uptime, ok := metrics["vm.uptime"]; ok && uptime > 0 {
			vmUptime = formatUptime(int64(uptime))
		}

		recentChecks, _ := s.GetRecentCheckResults(r.Context(), target, 20)

		// Get findings for this target
		storeFindings, _ := s.GetFindingsForTarget(r.Context(), target, 5)
		var findings []FindingRow
		for _, f := range storeFindings {
			findings = append(findings, FindingRow{
				ID:          f.ID,
				Timestamp:   f.Timestamp,
				Target:      f.Target,
				Scanner:     f.Scanner,
				Title:       f.Title,
				Description: f.Description,
				Severity:    f.Severity,
				Remediation: f.Remediation,
			})
		}

		contentData := struct {
			Target        string
			IsVM          bool
			VMID          string
			VMType        string
			VMStatus      string
			VMUptime      string
			LatestMetrics map[string]float64
			RecentChecks  []checker.CheckResult
			Findings      []FindingRow
		}{
			Target:        target,
			IsVM:          isVM,
			VMID:          vmID,
			VMType:        vmType,
			VMStatus:      vmStatus,
			VMUptime:      vmUptime,
			LatestMetrics: metrics,
			RecentChecks:  recentChecks,
			Findings:      findings,
		}

		content, ok := renderOrError(w, tmpl, "host-content", contentData)
		if !ok {
			return
		}

		data := HostData{
			PageData: PageData{
				ActivePage: "",
				Content:    content,
			},
			Target:        target,
			IsVM:          isVM,
			VMID:          vmID,
			VMType:        vmType,
			VMStatus:      vmStatus,
			VMUptime:      vmUptime,
			LatestMetrics: metrics,
			RecentChecks:  recentChecks,
			Findings:      findings,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// getNodeStats extracts node stats from latest data points.
func getNodeStats(s *store.Store) []NodeStat {
	nodeTargets, _ := s.GetAllTargetsWithMetric(context.Background(), "node.cpu.percent")

	var nodes []NodeStat
	for _, target := range nodeTargets {
		name := strings.TrimPrefix(target, "node:")

		cpuDP, _ := s.GetLatestDataPoint(context.Background(), target, "node.cpu.percent")
		memDP, _ := s.GetLatestDataPoint(context.Background(), target, "node.mem.percent")
		uptimeDP, _ := s.GetLatestDataPoint(context.Background(), target, "node.uptime")

		cpu := 0.0
		mem := 0.0
		uptime := "unknown"

		if cpuDP != nil {
			cpu = cpuDP.Value
		}
		if memDP != nil {
			mem = memDP.Value
		}
		if uptimeDP != nil {
			uptime = formatUptime(int64(uptimeDP.Value))
		}

		nodes = append(nodes, NodeStat{
			Name:       name,
			CPUPercent: cpu,
			MemPercent: mem,
			Uptime:     uptime,
			Status:     "online",
		})
	}

	return nodes
}

// formatUptime formats seconds to human readable.
func formatUptime(seconds int64) string {
	if seconds <= 0 {
		return "-"
	}
	d := seconds / 86400
	h := (seconds % 86400) / 3600
	m := (seconds % 3600) / 60

	var parts []string
	if d > 0 {
		parts = append(parts, fmt.Sprintf("%dd", d))
	}
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if len(parts) == 0 {
		return "< 1m"
	}
	return strings.Join(parts, " ")
}

// SecurityHandler renders the security page.
func SecurityHandler(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		findings, _ := s.GetFindings(r.Context())
		summary, _ := s.GetFindingsSummary(r.Context())

		var rows []FindingRow
		targetSet := make(map[string]bool)
		scannerSet := make(map[string]bool)

		for _, f := range findings {
			rows = append(rows, FindingRow{
				ID:          f.ID,
				Timestamp:   f.Timestamp,
				Target:      f.Target,
				Scanner:     f.Scanner,
				Title:       f.Title,
				Description: f.Description,
				Severity:    f.Severity,
				Remediation: f.Remediation,
			})
			targetSet[f.Target] = true
			scannerSet[f.Scanner] = true
		}

		var targets []string
		for t := range targetSet {
			targets = append(targets, t)
		}
		sort.Strings(targets)

		var scanners []string
		for s := range scannerSet {
			scanners = append(scanners, s)
		}
		sort.Strings(scanners)

		contentData := struct {
			Findings []FindingRow
			Summary  map[string]int
			Targets  []string
			Scanners []string
		}{
			Findings: rows,
			Summary:  summary,
			Targets:  targets,
			Scanners: scanners,
		}

		content, ok := renderOrError(w, tmpl, "security-content", contentData)
		if !ok {
			return
		}

		data := SecurityData{
			PageData: PageData{
				ActivePage: "security",
				Content:    content,
			},
			Findings: rows,
			Summary:  summary,
			Targets:  targets,
			Scanners: scanners,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// SecuritySummaryFragment returns the security summary badges for htmx refresh.
func SecuritySummaryFragment(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, _ := s.GetFindingsSummary(r.Context())

		data := struct {
			Summary map[string]int
		}{Summary: summary}

		w.Header().Set("Content-Type", "text/html")
		tmpl.ExecuteTemplate(w, "fragment-security-summary", data)
	}
}

// AlertsHandler renders the alerts page.
func AlertsHandler(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		since := time.Now().Add(-7 * 24 * time.Hour)
		events, _ := s.GetAlertEvents(r.Context(), since, 100)

		var rows []AlertEventRow
		for _, e := range events {
			rows = append(rows, AlertEventRow{
				ID:        e.ID,
				Timestamp: e.Timestamp,
				RuleName:  e.RuleName,
				Target:    e.Target,
				Message:   e.Message,
				Severity:  e.Severity,
			})
		}

		contentData := struct {
			Events []AlertEventRow
		}{Events: rows}

		content, ok := renderOrError(w, tmpl, "alerts-content", contentData)
		if !ok {
			return
		}

		data := AlertsData{
			PageData: PageData{
				ActivePage: "alerts",
				Content:    content,
			},
			Events: rows,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// JellyfinHandler renders the Jellyfin page.
func JellyfinHandler(tmpl *template.Template, client *jellyfin.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		info, _ := client.GetSystemInfo(ctx)
		sessions, _ := client.GetSessions(ctx)
		counts, _ := client.GetItemCounts(ctx)

		// Filter to active sessions (with now playing)
		var activeSessions []jellyfin.Session
		for _, s := range sessions {
			if s.NowPlayingItem != nil {
				activeSessions = append(activeSessions, s)
			}
		}

		contentData := struct {
			Info     jellyfin.SystemInfo
			Sessions []jellyfin.Session
			Counts   jellyfin.ItemCounts
		}{
			Info:     info,
			Sessions: activeSessions,
			Counts:   counts,
		}

		content, ok := renderOrError(w, tmpl, "jellyfin-content", contentData)
		if !ok {
			return
		}

		data := struct {
			PageData
			Info     jellyfin.SystemInfo
			Sessions []jellyfin.Session
			Counts   jellyfin.ItemCounts
		}{
			PageData: PageData{
				ActivePage: "jellyfin",
				Content:    content,
			},
			Info:     info,
			Sessions: activeSessions,
			Counts:   counts,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// FrigateHandler renders the Frigate page.
func FrigateHandler(tmpl *template.Template, client *frigate.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		version, _ := client.GetVersion(ctx)
		stats, _ := client.GetStats(ctx)

		contentData := struct {
			Version string
			Stats   frigate.Stats
		}{
			Version: version,
			Stats:   stats,
		}

		content, ok := renderOrError(w, tmpl, "frigate-content", contentData)
		if !ok {
			return
		}

		data := struct {
			PageData
			Version string
			Stats   frigate.Stats
		}{
			PageData: PageData{
				ActivePage: "frigate",
				Content:    content,
			},
			Version: version,
			Stats:   stats,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}
