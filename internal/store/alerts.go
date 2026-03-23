package store

import (
	"context"
	"fmt"
	"time"
)

// AlertEvent represents an alert event in the database.
type AlertEvent struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	RuleName  string    `json:"rule_name"`
	Target    string    `json:"target"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
}

// SaveAlertEvent inserts an alert event into the database with retry logic.
func (s *Store) SaveAlertEvent(ctx context.Context, event AlertEvent) error {
	for i := 0; i < 3; i++ {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO alert_events (timestamp, rule_name, target, message, severity)
			 VALUES (?, ?, ?, ?, ?)`,
			event.Timestamp, event.RuleName, event.Target, event.Message, event.Severity,
		)
		if err == nil {
			return nil
		}
		// Retry on database locked
		if i < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return fmt.Errorf("store: save alert event after retries")
}

// GetAlertEvents returns alert events since the given time, up to limit.
// Deprecated: Use GetAlertEventsPaginated for better pagination support.
func (s *Store) GetAlertEvents(ctx context.Context, since time.Time, limit int) ([]AlertEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, timestamp, rule_name, target, message, severity
		 FROM alert_events
		 WHERE timestamp > ?
		 ORDER BY timestamp DESC
		 LIMIT ?`,
		since, limit)
	if err != nil {
		return nil, fmt.Errorf("store: get alert events: %w", err)
	}
	defer rows.Close()

	var events []AlertEvent
	for rows.Next() {
		var e AlertEvent
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.RuleName, &e.Target, &e.Message, &e.Severity); err != nil {
			return nil, fmt.Errorf("store: scan alert event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetAlertEventsPaginated returns alert events with pagination support.
func (s *Store) GetAlertEventsPaginated(ctx context.Context, since time.Time, limit, offset int) ([]AlertEvent, error) {
	if limit <= 0 {
		limit = 50 // Default page size
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, timestamp, rule_name, target, message, severity
		 FROM alert_events
		 WHERE timestamp > ?
		 ORDER BY timestamp DESC
		 LIMIT ? OFFSET ?`,
		since, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: get alert events paginated: %w", err)
	}
	defer rows.Close()

	var events []AlertEvent
	for rows.Next() {
		var e AlertEvent
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.RuleName, &e.Target, &e.Message, &e.Severity); err != nil {
			return nil, fmt.Errorf("store: scan alert event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetAlertEventsCount returns the total count of alert events since the given time.
func (s *Store) GetAlertEventsCount(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM alert_events WHERE timestamp > ?`,
		since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: get alert events count: %w", err)
	}
	return count, nil
}
