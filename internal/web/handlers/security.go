package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/homelab/homelab-dash/internal/store"
)

// FindingsHandler returns findings as JSON with optional filters.
func FindingsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		severityStr := r.URL.Query().Get("severity")
		sinceStr := r.URL.Query().Get("since")

		severity := -1
		if severityStr != "" {
			severity = severityToInt(severityStr)
		}

		var since time.Time
		if sinceStr != "" {
			t, err := time.Parse(time.RFC3339, sinceStr)
			if err == nil {
				since = t
			}
		}

		findings, err := s.GetFindingsFiltered(r.Context(), target, severity, since)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(findings)
	}
}

// SummaryHandler returns a severity count summary.
func SummaryHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, err := s.GetFindingsSummary(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	}
}

// ReportHandler returns all findings from the last 30 days as a downloadable JSON file.
func ReportHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		since := time.Now().Add(-30 * 24 * time.Hour)
		findings, err := s.GetFindingsFiltered(r.Context(), "", -1, since)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		filename := fmt.Sprintf("security-%s.json", time.Now().Format("2006-01-02"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		json.NewEncoder(w).Encode(findings)
	}
}

// FindingDetailHandler returns a single finding as an HTML fragment.
func FindingDetailHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		finding, err := s.GetFindingByID(r.Context(), id)
		if err != nil {
			http.Error(w, "finding not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<tr class="finding-detail">
			<td colspan="5" style="padding: 0.75rem; background: var(--surface-hover);">
				<strong>Description:</strong> %s<br>
				<strong>Remediation:</strong> <em>%s</em>
			</td>
		</tr>`, finding.Description, finding.Remediation)
	}
}

func severityToInt(s string) int {
	switch s {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
