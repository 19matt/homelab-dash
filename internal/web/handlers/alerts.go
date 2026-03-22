package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/homelab/homelab-dash/internal/store"
)

// AlertEventsHandler returns alert events as JSON.
func AlertEventsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sinceStr := r.URL.Query().Get("since")
		limitStr := r.URL.Query().Get("limit")

		since := time.Now().Add(-7 * 24 * time.Hour) // default: last 7 days
		if sinceStr != "" {
			if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
				since = t
			}
		}

		limit := 100
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		events, err := s.GetAlertEvents(r.Context(), since, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if events == nil {
			events = []store.AlertEvent{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	}
}
