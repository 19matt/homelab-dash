package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

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

// getNodeStats extracts node stats from latest data points.
func getNodeStats(s *store.Store) []NodeStat {
	// Get all node stats in a single batch query
	nodeStats, err := s.GetAllNodeStats(context.Background())
	if err != nil {
		log.Printf("getNodeStats: failed to get node stats: %v", err)
		return nil
	}

	var nodes []NodeStat
	for target, metrics := range nodeStats {
		name := strings.TrimPrefix(target, "node:")

		cpu := metrics["node.cpu.percent"]
		mem := metrics["node.mem.percent"]

		uptime := "unknown"
		if uptimeVal, ok := metrics["node.uptime"]; ok && uptimeVal > 0 {
			uptime = formatUptime(int64(uptimeVal))
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
