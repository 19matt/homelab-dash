package store

import (
	"context"
	"fmt"
	"time"
)

// Finding is the store representation of a scanner finding.
type Finding struct {
	ID          int64
	Timestamp   time.Time
	Target      string
	Scanner     string
	Title       string
	Description string
	Severity    int
	Remediation string
	ScanID      string
}

// SaveFindings replaces current findings with new ones and tracks history.
// It compares new findings with previous ones, marks resolved issues in history,
// and saves only the latest scan results.
func (s *Store) SaveFindings(ctx context.Context, findings []Finding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Get previous findings before clearing
	prevRows, err := tx.QueryContext(ctx,
		`SELECT target, scanner, title, description, severity, remediation FROM findings`)
	if err != nil {
		return fmt.Errorf("store: get previous findings: %w", err)
	}

	type findingKey struct {
		target  string
		scanner string
		title   string
	}
	prevFindings := make(map[findingKey]Finding)
	for prevRows.Next() {
		var f Finding
		if err := prevRows.Scan(&f.Target, &f.Scanner, &f.Title, &f.Description, &f.Severity, &f.Remediation); err != nil {
			prevRows.Close()
			return fmt.Errorf("store: scan previous finding: %w", err)
		}
		prevFindings[findingKey{f.Target, f.Scanner, f.Title}] = f
	}
	prevRows.Close()

	// Build set of new finding keys
	newKeys := make(map[findingKey]bool)
	for _, f := range findings {
		newKeys[findingKey{f.Target, f.Scanner, f.Title}] = true
	}

	// Mark resolved findings in history (findings that were in previous scan but not in new)
	for key := range prevFindings {
		if !newKeys[key] {
			// Finding was resolved - update history record
			tx.ExecContext(ctx,
				`UPDATE findings_history
				 SET resolved_at = ?
				 WHERE target = ? AND scanner = ? AND title = ? AND resolved_at IS NULL`,
				now, key.target, key.scanner, key.title)
		}
	}

	// Clear current findings
	if _, err := tx.ExecContext(ctx, `DELETE FROM findings`); err != nil {
		return fmt.Errorf("store: clear findings: %w", err)
	}

	// Insert new findings and update history
	if len(findings) > 0 {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO findings (timestamp, target, scanner, title, description, severity, remediation, scan_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("store: prepare insert: %w", err)
		}
		defer stmt.Close()

		for _, f := range findings {
			if _, err := stmt.ExecContext(ctx,
				f.Timestamp, f.Target, f.Scanner, f.Title, f.Description, f.Severity, f.Remediation, f.ScanID); err != nil {
				return fmt.Errorf("store: insert finding: %w", err)
			}

			// Check if finding exists in history
			var historyID int64
			err := tx.QueryRowContext(ctx,
				`SELECT id FROM findings_history
				 WHERE target = ? AND scanner = ? AND title = ? AND resolved_at IS NULL`,
				f.Target, f.Scanner, f.Title).Scan(&historyID)

			if err != nil {
				// New finding - insert into history
				tx.ExecContext(ctx,
					`INSERT INTO findings_history (first_seen, last_seen, target, scanner, title, description, severity, remediation)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					now, now, f.Target, f.Scanner, f.Title, f.Description, f.Severity, f.Remediation)
			} else {
				// Existing finding - update last_seen
				tx.ExecContext(ctx,
					`UPDATE findings_history SET last_seen = ?, severity = ?, description = ?, remediation = ?
					 WHERE id = ?`,
					now, f.Severity, f.Description, f.Remediation, historyID)
			}
		}
	}

	return tx.Commit()
}

// GetFindings returns all current findings ordered by severity descending then timestamp.
func (s *Store) GetFindings(ctx context.Context) ([]Finding, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, timestamp, target, scanner, title, description, severity, remediation, scan_id
		 FROM findings
		 ORDER BY severity DESC, timestamp DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: get findings: %w", err)
	}
	defer rows.Close()

	var findings []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.ID, &f.Timestamp, &f.Target, &f.Scanner,
			&f.Title, &f.Description, &f.Severity, &f.Remediation, &f.ScanID); err != nil {
			return nil, fmt.Errorf("store: scan finding: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// GetFindingsFiltered returns current findings with optional filters.
func (s *Store) GetFindingsFiltered(ctx context.Context, target string, severity int, since time.Time) ([]Finding, error) {
	query := `SELECT id, timestamp, target, scanner, title, description, severity, remediation, scan_id
	          FROM findings WHERE 1=1`
	var args []interface{}

	if target != "" {
		query += " AND target = ?"
		args = append(args, target)
	}
	if severity >= 0 {
		query += " AND severity = ?"
		args = append(args, severity)
	}
	if !since.IsZero() {
		query += " AND timestamp > ?"
		args = append(args, since)
	}
	query += " ORDER BY severity DESC, timestamp DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: get findings filtered: %w", err)
	}
	defer rows.Close()

	var findings []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.ID, &f.Timestamp, &f.Target, &f.Scanner,
			&f.Title, &f.Description, &f.Severity, &f.Remediation, &f.ScanID); err != nil {
			return nil, fmt.Errorf("store: scan finding: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// GetFindingsSummary returns a count of current findings grouped by severity.
func (s *Store) GetFindingsSummary(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT severity, COUNT(*) FROM findings GROUP BY severity`)
	if err != nil {
		return nil, fmt.Errorf("store: get findings summary: %w", err)
	}
	defer rows.Close()

	severityNames := map[int]string{0: "info", 1: "low", 2: "medium", 3: "high", 4: "critical"}
	summary := make(map[string]int)
	for _, name := range severityNames {
		summary[name] = 0
	}

	for rows.Next() {
		var severity, count int
		if err := rows.Scan(&severity, &count); err != nil {
			return nil, fmt.Errorf("store: scan summary: %w", err)
		}
		if name, ok := severityNames[severity]; ok {
			summary[name] = count
		}
	}
	return summary, rows.Err()
}

// GetFindingByID returns a single current finding by ID.
func (s *Store) GetFindingByID(ctx context.Context, id int64) (*Finding, error) {
	var f Finding
	err := s.db.QueryRowContext(ctx,
		`SELECT id, timestamp, target, scanner, title, description, severity, remediation, scan_id
		 FROM findings WHERE id = ?`, id,
	).Scan(&f.ID, &f.Timestamp, &f.Target, &f.Scanner,
		&f.Title, &f.Description, &f.Severity, &f.Remediation, &f.ScanID)
	if err != nil {
		return nil, fmt.Errorf("store: get finding by id: %w", err)
	}
	return &f, nil
}

// GetResolvedFindings returns findings that were resolved (not in current scan).
func (s *Store) GetResolvedFindings(ctx context.Context, limit int) ([]Finding, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, last_seen, target, scanner, title, description, severity, remediation
		 FROM findings_history
		 WHERE resolved_at IS NOT NULL
		 ORDER BY resolved_at DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: get resolved findings: %w", err)
	}
	defer rows.Close()

	var findings []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.ID, &f.Timestamp, &f.Target, &f.Scanner,
			&f.Title, &f.Description, &f.Severity, &f.Remediation); err != nil {
			return nil, fmt.Errorf("store: scan resolved finding: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}
