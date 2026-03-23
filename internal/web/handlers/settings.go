package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/config"
	"github.com/homelab/homelab-dash/internal/store"
)

// SettingsData is the data for the settings page.
type SettingsData struct {
	PageData
	Config  *config.Config
	Targets []config.TargetConfig
}

// SettingsHandler renders the settings page.
func SettingsHandler(tmpl *template.Template, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentData := struct {
			Config  *config.Config
			Targets []config.TargetConfig
		}{
			Config:  cfg,
			Targets: cfg.Targets,
		}

		content, ok := renderOrError(w, tmpl, "settings-content", contentData)
		if !ok {
			return
		}

		data := SettingsData{
			PageData: PageData{
				ActivePage: "settings",
				Content:    content,
			},
			Config:  cfg,
			Targets: cfg.Targets,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}

// AddTargetRequest is the JSON body for adding a target.
type AddTargetRequest struct {
	Name   string   `json:"name"`
	Host   string   `json:"host"`
	Checks []string `json:"checks"`
	Ports  []int    `json:"ports"`
}

// AddTargetHandler adds a new target to the config.
func AddTargetHandler(mgr *config.Manager, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AddTargetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if req.Name == "" || req.Host == "" {
			http.Error(w, "name and host are required", http.StatusBadRequest)
			return
		}

		// Convert ports to interface slice
		var ports []interface{}
		for _, p := range req.Ports {
			ports = append(ports, p)
		}

		target := config.TargetConfig{
			Name:   req.Name,
			Host:   req.Host,
			Checks: req.Checks,
			Ports:  ports,
		}

		// Add target with thread-safe mutation
		if err := mgr.AddTarget(target); err != nil {
			http.Error(w, fmt.Sprintf("failed to save config: %v", err), http.StatusInternalServerError)
			return
		}

		// Audit
		s.SaveAuditEvent(r.Context(), store.AuditEvent{
			Timestamp: time.Now(),
			Type:      store.AuditConfigChange,
			Target:    req.Name,
			Message:   fmt.Sprintf("Target added: %s (%s)", req.Name, req.Host),
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// RemoveTargetHandler removes a target from the config.
func RemoveTargetHandler(mgr *config.Manager, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			http.Error(w, "missing target name", http.StatusBadRequest)
			return
		}

		found, err := mgr.RemoveTarget(name)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to save config: %v", err), http.StatusInternalServerError)
			return
		}

		if !found {
			http.Error(w, "target not found", http.StatusNotFound)
			return
		}

		// Audit
		s.SaveAuditEvent(r.Context(), store.AuditEvent{
			Timestamp: time.Now(),
			Type:      store.AuditConfigChange,
			Target:    name,
			Message:   fmt.Sprintf("Target removed: %s", name),
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// TestTargetRequest is the JSON body for testing a target.
type TestTargetRequest struct {
	Host   string   `json:"host"`
	Checks []string `json:"checks"`
	Port   int      `json:"port"`
}

// TestTargetHandler runs a test check against a target.
func TestTargetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req TestTargetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if req.Port == 0 {
			req.Port = 80
		}

		ctx := r.Context()
		results := make(map[string]string)

		for _, check := range req.Checks {
			switch check {
			case "ping":
				c := checker.NewPingChecker("test", req.Host, req.Port)
				result, err := c.Check(ctx)
				if err != nil {
					log.Printf("test target ping check failed: %v", err)
					results["ping"] = "error"
				} else {
					results["ping"] = result.Status.String()
				}
			case "http":
				c := checker.NewHTTPChecker("test", req.Host, req.Port, "http", false)
				result, err := c.Check(ctx)
				if err != nil {
					log.Printf("test target http check failed: %v", err)
					results["http"] = "error"
				} else {
					results["http"] = result.Status.String()
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
