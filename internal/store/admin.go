package store

import (
	"context"
	"fmt"
	"os"
	"time"
)

// AdminStatus contains system status information for the admin endpoint.
type AdminStatus struct {
	Version          string         `json:"version"`
	BuildTime        string         `json:"build_time"`
	UptimeSeconds    int64          `json:"uptime_seconds"`
	DBSizeBytes      int64          `json:"db_size_bytes"`
	TableRowCounts   map[string]int `json:"table_row_counts"`
	SchedulerRunning bool           `json:"scheduler_running"`
	Integrations     []string       `json:"integrations_enabled"`
}

// GetAdminStatus returns system status information.
func (s *Store) GetAdminStatus(ctx context.Context, version, buildTime string, startTime time.Time, integrations []string) (AdminStatus, error) {
	status := AdminStatus{
		Version:          version,
		BuildTime:        buildTime,
		UptimeSeconds:    int64(time.Since(startTime).Seconds()),
		SchedulerRunning: true,
		Integrations:     integrations,
		TableRowCounts:   make(map[string]int),
	}

	// Get DB file size
	if dbPath := s.getDBPath(); dbPath != "" {
		if info, err := os.Stat(dbPath); err == nil {
			status.DBSizeBytes = info.Size()
		}
	}

	// Get table row counts
	tables := []string{"check_results", "data_points", "findings", "alert_events", "audit_events"}
	for _, table := range tables {
		var count int
		err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err == nil {
			status.TableRowCounts[table] = count
		}
	}

	return status, nil
}

// getDBPath returns the database file path if possible.
func (s *Store) getDBPath() string {
	var path string
	s.db.QueryRow("PRAGMA database_list").Scan(nil, nil, &path)
	return path
}
