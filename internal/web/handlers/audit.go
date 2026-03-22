package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/homelab/homelab-dash/internal/store"
)

// AuditEventsHandler returns audit events as JSON.
func AuditEventsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		since := time.Now().Add(-7 * 24 * time.Hour)
		eventType := r.URL.Query().Get("type")
		pageStr := r.URL.Query().Get("page")

		page := 1
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
		limit := 50
		offset := (page - 1) * limit

		events, err := s.GetAuditEvents(r.Context(), since, eventType, limit, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if events == nil {
			events = []store.AuditEvent{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	}
}

// AuditData is the data for the audit page.
type AuditData struct {
	PageData
	Events     []store.AuditEvent
	EventType  string
	Page       int
	TotalCount int
}

// AuditHandler renders the audit page.
func AuditHandler(tmpl *template.Template, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		since := time.Now().Add(-7 * 24 * time.Hour)
		eventType := r.URL.Query().Get("type")
		pageStr := r.URL.Query().Get("page")

		page := 1
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
		limit := 50
		offset := (page - 1) * limit

		events, _ := s.GetAuditEvents(r.Context(), since, eventType, limit, offset)
		totalCount, _ := s.GetAuditEventCount(r.Context(), since, eventType)

		contentData := struct {
			Events     []store.AuditEvent
			EventType  string
			Page       int
			TotalCount int
		}{
			Events:     events,
			EventType:  eventType,
			Page:       page,
			TotalCount: totalCount,
		}

		data := AuditData{
			PageData: PageData{
				ActivePage: "audit",
				Content:    renderTemplate(tmpl, "audit-content", contentData),
			},
			Events:     events,
			EventType:  eventType,
			Page:       page,
			TotalCount: totalCount,
		}

		tmpl.ExecuteTemplate(w, "layout", data)
	}
}
