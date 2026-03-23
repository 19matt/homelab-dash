package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/integration/frigate"
	"github.com/homelab/homelab-dash/internal/integration/jellyfin"
	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/store"
)

// OverviewHandler renders the overview page.
func OverviewHandler(tmpl *template.Template, s *store.Store, vmCollectors []*proxmox.VMCollector, jellyfinClient JellyfinClient, frigateClient FrigateClient) http.HandlerFunc {
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
		summary, err := s.GetFindingsSummary(r.Context())
		if err != nil {
			log.Printf("overview: failed to get findings summary: %v", err)
			summary = make(map[string]int)
		}
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

			uptime24h, err := s.GetUptimePercent(r.Context(), target, cr.Check, 24*time.Hour)
			if err != nil {
				log.Printf("services: failed to get 24h uptime for %s: %v", target, err)
			}
			uptime7d, err := s.GetUptimePercent(r.Context(), target, cr.Check, 7*24*time.Hour)
			if err != nil {
				log.Printf("services: failed to get 7d uptime for %s: %v", target, err)
			}
			dailyUptime, err := s.GetDailyUptime(r.Context(), target, cr.Check, 90)
			if err != nil {
				log.Printf("services: failed to get daily uptime for %s: %v", target, err)
			}

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

		// Prepare targets for batch query
		targets := make([]string, len(allVMs))
		for i, vm := range allVMs {
			targets[i] = fmt.Sprintf("vm:%d:%s", vm.VMID, vm.Name)
		}

		// Fetch daily uptime for all VMs in a single batch query
		vmUptimeBars, err := s.GetDailyUptimeBatch(r.Context(), targets, "proxmox.vm.status", 90)
		if err != nil {
			log.Printf("proxmox: failed to get daily uptime batch: %v", err)
			vmUptimeBars = make(map[string][]store.DailyUptime)
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
		vmTargets, err := s.GetAllTargetsWithMetric(r.Context(), "vm.cpu.percent")
		if err != nil {
			log.Printf("metrics: failed to get VM targets: %v", err)
			vmTargets = nil
		}
		nodeTargets, err := s.GetAllTargetsWithMetric(r.Context(), "node.cpu.percent")
		if err != nil {
			log.Printf("metrics: failed to get node targets: %v", err)
			nodeTargets = nil
		}

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
			checks, err := s.GetRecentCheckResults(r.Context(), target, 1)
			if err != nil {
				log.Printf("host: failed to get recent checks for %s: %v", target, err)
			}
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
			dp, err := s.GetLatestDataPoint(r.Context(), target, m)
			if err != nil {
				log.Printf("host: failed to get latest data point for %s/%s: %v", target, m, err)
			}
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

		recentChecks, err := s.GetRecentCheckResults(r.Context(), target, 20)
		if err != nil {
			log.Printf("host: failed to get recent checks for %s: %v", target, err)
		}

		// Get findings for this target
		storeFindings, err := s.GetFindingsForTarget(r.Context(), target, 5)
		if err != nil {
			log.Printf("host: failed to get findings for %s: %v", target, err)
		}
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

// SecurityHandler renders the security page.
func SecurityHandler(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse pagination parameters
		page := 1
		pageSize := 50

		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
			if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 200 {
				pageSize = ps
			}
		}

		offset := (page - 1) * pageSize

		// Get paginated findings and total count
		findings, err := s.GetFindingsPaginated(r.Context(), pageSize, offset)
		if err != nil {
			log.Printf("security: failed to get findings: %v", err)
			findings = nil
		}
		totalCount, err := s.GetFindingsCount(r.Context())
		if err != nil {
			log.Printf("security: failed to get findings count: %v", err)
			totalCount = 0
		}
		summary, err := s.GetFindingsSummary(r.Context())
		if err != nil {
			log.Printf("security: failed to get findings summary: %v", err)
			summary = make(map[string]int)
		}

		// Calculate pagination info
		totalPages := (totalCount + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		hasPrevPage := page > 1
		hasNextPage := page < totalPages

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
			Findings:    rows,
			Summary:     summary,
			Targets:     targets,
			Scanners:    scanners,
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalCount:  totalCount,
			HasPrevPage: hasPrevPage,
			HasNextPage: hasNextPage,
			PrevPage:    page - 1,
			NextPage:    page + 1,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// AlertsHandler renders the alerts page.
func AlertsHandler(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse pagination parameters
		page := 1
		pageSize := 50

		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
			if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 200 {
				pageSize = ps
			}
		}

		offset := (page - 1) * pageSize
		since := time.Now().Add(-7 * 24 * time.Hour)

		// Get paginated events and total count
		events, err := s.GetAlertEventsPaginated(r.Context(), since, pageSize, offset)
		if err != nil {
			log.Printf("alerts: failed to get events: %v", err)
			events = nil
		}
		totalCount, err := s.GetAlertEventsCount(r.Context(), since)
		if err != nil {
			log.Printf("alerts: failed to get events count: %v", err)
			totalCount = 0
		}

		// Calculate pagination info
		totalPages := (totalCount + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		hasPrevPage := page > 1
		hasNextPage := page < totalPages

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
			Events:      rows,
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalCount:  totalCount,
			HasPrevPage: hasPrevPage,
			HasNextPage: hasNextPage,
			PrevPage:    page - 1,
			NextPage:    page + 1,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// JellyfinHandler renders the Jellyfin page.
func JellyfinHandler(tmpl *template.Template, client JellyfinClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		info, err := client.GetSystemInfo(ctx)
		if err != nil {
			respondError(w, tmpl, err, "Failed to connect to Jellyfin")
			return
		}

		sessions, err := client.GetSessions(ctx)
		if err != nil {
			log.Printf("jellyfin: failed to get sessions: %v", err)
			sessions = nil
		}
		counts, err := client.GetItemCounts(ctx)
		if err != nil {
			log.Printf("jellyfin: failed to get item counts: %v", err)
		}

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
func FrigateHandler(tmpl *template.Template, client FrigateClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		version, err := client.GetVersion(ctx)
		if err != nil {
			respondError(w, tmpl, err, "Failed to connect to Frigate")
			return
		}

		stats, err := client.GetStats(ctx)
		if err != nil {
			log.Printf("frigate: failed to get stats: %v", err)
		}

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
