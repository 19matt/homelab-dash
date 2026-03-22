package store

import (
	"context"
	"fmt"
	"time"
)

// RunRetentionCleanup deletes old data from the database.
func (s *Store) RunRetentionCleanup(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	var totalDeleted int64

	// Delete old data_points
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM data_points WHERE timestamp < ?`, cutoff)
	if err != nil {
		return totalDeleted, fmt.Errorf("store: cleanup data_points: %w", err)
	}
	n, _ := result.RowsAffected()
	totalDeleted += n

	// Delete old check_results
	result, err = s.db.ExecContext(ctx,
		`DELETE FROM check_results WHERE timestamp < ?`, cutoff)
	if err != nil {
		return totalDeleted, fmt.Errorf("store: cleanup check_results: %w", err)
	}
	n, _ = result.RowsAffected()
	totalDeleted += n

	// Delete old findings (but keep findings_history)
	result, err = s.db.ExecContext(ctx,
		`DELETE FROM findings WHERE timestamp < ?`, cutoff)
	if err != nil {
		return totalDeleted, fmt.Errorf("store: cleanup findings: %w", err)
	}
	n, _ = result.RowsAffected()
	totalDeleted += n

	// Note: audit_events and alert_events are kept indefinitely

	return totalDeleted, nil
}
