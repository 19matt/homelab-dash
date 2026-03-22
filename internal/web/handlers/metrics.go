package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/homelab/homelab-dash/internal/integration/proxmox"
	"github.com/homelab/homelab-dash/internal/store"
)

// MetricsLatestHandler returns the latest value for each metric on a target.
func MetricsLatestHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		if target == "" {
			http.Error(w, "missing target parameter", http.StatusBadRequest)
			return
		}

		// We don't know which metrics exist, so we get from common ones.
		// For a more complete solution, we'd need a "get all metrics for target" query.
		// For now, return the latest data points for known metric prefixes.
		metrics := []string{
			"node.cpu.percent", "node.mem.used", "node.mem.total", "node.mem.percent",
			"node.uptime", "node.load1", "node.load5", "node.load15",
			"vm.cpu.percent", "vm.mem.used", "vm.mem.total", "vm.mem.percent",
			"vm.disk.used", "vm.disk.total", "vm.netin", "vm.netout",
		}

		result := make(map[string]float64)
		for _, metric := range metrics {
			dp, err := s.GetLatestDataPoint(r.Context(), target, metric)
			if err != nil {
				continue
			}
			if dp != nil {
				result[metric] = dp.Value
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// historyPoint is a single point in a time series for Chart.js.
type historyPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricsHistoryHandler returns time-series data for a target+metric.
func MetricsHistoryHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		if target == "" {
			http.Error(w, "missing target parameter", http.StatusBadRequest)
			return
		}

		metric := r.URL.Query().Get("metric")
		if metric == "" {
			http.Error(w, "missing metric parameter", http.StatusBadRequest)
			return
		}

		var from, to time.Time

		// Support custom from/to range
		fromStr := r.URL.Query().Get("from")
		toStr := r.URL.Query().Get("to")
		if fromStr != "" && toStr != "" {
			from, _ = time.Parse(time.RFC3339, fromStr)
			to, _ = time.Parse(time.RFC3339, toStr)
		} else {
			// Fall back to window shorthand
			windowStr := r.URL.Query().Get("window")
			if windowStr == "" {
				windowStr = "1h"
			}
			window, err := parseWindow(windowStr)
			if err != nil {
				http.Error(w, "invalid window: use 1h, 6h, 24h, or 7d", http.StatusBadRequest)
				return
			}
			to = time.Now()
			from = to.Add(-window)
		}

		points, err := s.GetDataPoints(r.Context(), target, metric, from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out := make([]historyPoint, len(points))
		for i, p := range points {
			out[i] = historyPoint{Timestamp: p.Timestamp, Value: p.Value}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

// MetricsTargetsHandler returns all targets that have data points.
func MetricsTargetsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get targets from a common metric
		targets, err := s.GetAllTargetsWithMetric(r.Context(), "vm.cpu.percent")
		if err != nil {
			// Try node metric if no VMs
			targets, err = s.GetAllTargetsWithMetric(r.Context(), "node.cpu.percent")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Also add node targets
		nodeTargets, _ := s.GetAllTargetsWithMetric(r.Context(), "node.cpu.percent")
		seen := make(map[string]bool)
		for _, t := range targets {
			seen[t] = true
		}
		for _, t := range nodeTargets {
			if !seen[t] {
				targets = append(targets, t)
				seen[t] = true
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(targets)
	}
}

// ProxmoxVMsHandler returns the latest VM list from the collector cache.
func ProxmoxVMsHandler(vmCollector *proxmox.VMCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vms := vmCollector.LatestVMs()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(vms)
	}
}

// ProxmoxVMsHandlerMulti returns VMs from all node collectors combined.
func ProxmoxVMsHandlerMulti(vmCollectors []*proxmox.VMCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var allVMs []proxmox.VMSummary
		for _, c := range vmCollectors {
			allVMs = append(allVMs, c.LatestVMs()...)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(allVMs)
	}
}

// parseWindow converts window strings like "1h", "6h", "24h", "7d" to durations.
func parseWindow(s string) (time.Duration, error) {
	switch s {
	case "1h":
		return time.Hour, nil
	case "6h":
		return 6 * time.Hour, nil
	case "24h":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	default:
		return time.ParseDuration(s)
	}
}

// DailyUptimeHandler returns daily uptime percentages as JSON.
func DailyUptimeHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		if target == "" {
			http.Error(w, "missing target parameter", http.StatusBadRequest)
			return
		}

		check := r.URL.Query().Get("check")
		if check == "" {
			check = "ping"
		}

		days := 90
		if d := r.URL.Query().Get("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
				days = parsed
			}
		}

		uptime, err := s.GetDailyUptime(r.Context(), target, check, days)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if uptime == nil {
			uptime = []store.DailyUptime{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(uptime)
	}
}
