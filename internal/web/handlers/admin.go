package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/homelab/homelab-dash/internal/store"
)

// AdminStatusHandler returns system status as JSON.
func AdminStatusHandler(s *store.Store, version, buildTime string, startTime time.Time, integrations []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := s.GetAdminStatus(r.Context(), version, buildTime, startTime, integrations)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}
