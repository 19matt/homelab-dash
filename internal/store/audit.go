package store

import (
	"context"
	"fmt"
	"time"
)

// Audit event type constants.
const (
	AuditCheckFail    = "check.fail"
	AuditCheckRecover = "check.recover"
	AuditScanComplete = "scan.complete"
	AuditAlertFired   = "alert.fired"
	AuditConfigChange = "config.change"
	AuditStartup      = "startup"
)

// AuditEvent represents an audit log entry.
type AuditEvent struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Target    string    `json:"target"`
	Message   string    `json:"message"`
}

// SaveAuditEvent inserts an audit event into the database.
func (s *Store) SaveAuditEvent(ctx context.Context, event AuditEvent) error {
	if event.Target == "" {
		event.Target = "-"
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_events (timestamp, type, target, message) VALUES (?, ?, ?, ?)`,
		event.Timestamp, event.Type, event.Target, event.Message,
	)
	if err != nil {
		return fmt.Errorf("store: save audit event: %w", err)
	}
	_ = result
	return nil
}

// GetAuditEvents returns audit events with optional filters and pagination.
func (s *Store) GetAuditEvents(ctx context.Context, since time.Time, eventType string, limit, offset int) ([]AuditEvent, error) {
	query := `SELECT id, timestamp, type, target, message FROM audit_events WHERE timestamp > ?`
	args := []interface{}{since}

	if eventType != "" && eventType != "all" {
		query += " AND type = ?"
		args = append(args, eventType)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: get audit events: %w", err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Type, &e.Target, &e.Message); err != nil {
			return nil, fmt.Errorf("store: scan audit event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetAuditEventCount returns the total count of audit events matching filters.
func (s *Store) GetAuditEventCount(ctx context.Context, since time.Time, eventType string) (int, error) {
	query := `SELECT COUNT(*) FROM audit_events WHERE timestamp > ?`
	args := []interface{}{since}

	if eventType != "" && eventType != "all" {
		query += " AND type = ?"
		args = append(args, eventType)
	}

	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: get audit event count: %w", err)
	}
	return count, nil
}
