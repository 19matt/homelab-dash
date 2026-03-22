package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/store"
)

// statusResult is the JSON representation of a check result for the API.
type statusResult struct {
	Timestamp time.Time `json:"timestamp"`
	Target    string    `json:"target"`
	Check     string    `json:"check"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	LatencyMs int64     `json:"latency_ms"`
}

// uptimeResult is the JSON response for the uptime endpoint.
type uptimeResult struct {
	Target string  `json:"target"`
	Check  string  `json:"check"`
	Window string  `json:"window"`
	Uptime float64 `json:"uptime"`
}

// StatusHandler returns the latest check result for each target+check combination.
func StatusHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results, err := s.GetLatestCheckResults(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out := make([]statusResult, len(results))
		for i, r := range results {
			out[i] = statusResult{
				Timestamp: r.Timestamp,
				Target:    r.Target,
				Check:     r.Check,
				Status:    r.Status.String(),
				Message:   r.Message,
				LatencyMs: r.Latency.Milliseconds(),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

// UptimeHandler returns the uptime percentage for a target+check over a time window.
func UptimeHandler(s *store.Store) http.HandlerFunc {
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

		windowStr := r.URL.Query().Get("window")
		if windowStr == "" {
			windowStr = "24h"
		}
		window, err := time.ParseDuration(windowStr)
		if err != nil {
			http.Error(w, "invalid window duration", http.StatusBadRequest)
			return
		}

		pct, err := s.GetUptimePercent(r.Context(), target, check, window)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		result := uptimeResult{
			Target: target,
			Check:  check,
			Window: window.String(),
			Uptime: pct,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// MarshalStatus converts a CheckResult to its JSON representation.
func MarshalStatus(r checker.CheckResult) statusResult {
	return statusResult{
		Timestamp: r.Timestamp,
		Target:    r.Target,
		Check:     r.Check,
		Status:    r.Status.String(),
		Message:   r.Message,
		LatencyMs: r.Latency.Milliseconds(),
	}
}
