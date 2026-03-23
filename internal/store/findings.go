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
		// Batch insert findings
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
		}

		// Get all existing history records at once
		historyRows, err := tx.QueryContext(ctx,
			`SELECT id, target, scanner, title FROM findings_history WHERE resolved_at IS NULL`)
		if err != nil {
			return fmt.Errorf("store: get history: %w", err)
		}
		defer historyRows.Close()

		historyMap := make(map[findingKey]int64)
		for historyRows.Next() {
			var id int64
			var target, scanner, title string
			if err := historyRows.Scan(&id, &target, &scanner, &title); err != nil {
				return fmt.Errorf("store: scan history: %w", err)
			}
			historyMap[findingKey{target, scanner, title}] = id
		}
		historyRows.Close()

		// Prepare statements for history operations
		insertHistoryStmt, err := tx.PrepareContext(ctx,
			`INSERT INTO findings_history (first_seen, last_seen, target, scanner, title, description, severity, remediation)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("store: prepare history insert: %w", err)
		}
		defer insertHistoryStmt.Close()

		updateHistoryStmt, err := tx.PrepareContext(ctx,
			`UPDATE findings_history SET last_seen = ?, severity = ?, description = ?, remediation = ?
			 WHERE id = ?`)
		if err != nil {
			return fmt.Errorf("store: prepare history update: %w", err)
		}
		defer updateHistoryStmt.Close()

		// Process findings for history
		for _, f := range findings {
			key := findingKey{f.Target, f.Scanner, f.Title}
			if historyID, exists := historyMap[key]; exists {
				// Existing finding - update last_seen
				if _, err := updateHistoryStmt.ExecContext(ctx,
					now, f.Severity, f.Description, f.Remediation, historyID); err != nil {
					return fmt.Errorf("store: update history: %w", err)
				}
			} else {
				// New finding - insert into history
				if _, err := insertHistoryStmt.ExecContext(ctx,
					now, now, f.Target, f.Scanner, f.Title, f.Description, f.Severity, f.Remediation); err != nil {
					return fmt.Errorf("store: insert history: %w", err)
				}
			}
		}
	}

	return tx.Commit()
}

// GetFindings returns all current findings ordered by severity descending then timestamp.
// Deprecated: Use GetFindingsPaginated for better performance with large datasets.
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

// GetFindingsPaginated returns current findings with pagination support.
func (s *Store) GetFindingsPaginated(ctx context.Context, limit, offset int) ([]Finding, error) {
	if limit <= 0 {
		limit = 50 // Default page size
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, timestamp, target, scanner, title, description, severity, remediation, scan_id
		 FROM findings
		 ORDER BY severity DESC, timestamp DESC
		 LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: get findings paginated: %w", err)
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

// GetFindingsCount returns the total count of current findings.
func (s *Store) GetFindingsCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM findings`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: get findings count: %w", err)
	}
	return count, nil
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
func (s *Store) GetResolvedFindings(ctx context.Context, limit int) ([]ResolvedFinding, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, first_seen, last_seen, resolved_at, target, scanner, title, description, severity, remediation
		 FROM findings_history
		 WHERE resolved_at IS NOT NULL
		 ORDER BY resolved_at DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: get resolved findings: %w", err)
	}
	defer rows.Close()

	var findings []ResolvedFinding
	for rows.Next() {
		var f ResolvedFinding
		if err := rows.Scan(&f.ID, &f.FirstSeen, &f.LastSeen, &f.ResolvedAt,
			&f.Target, &f.Scanner, &f.Title, &f.Description, &f.Severity, &f.Remediation); err != nil {
			return nil, fmt.Errorf("store: scan resolved finding: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// ResolvedFinding represents a finding from the history table with resolution info.
type ResolvedFinding struct {
	ID          int64      `json:"id"`
	FirstSeen   time.Time  `json:"first_seen"`
	LastSeen    time.Time  `json:"last_seen"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	Target      string     `json:"target"`
	Scanner     string     `json:"scanner"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Severity    int        `json:"severity"`
	Remediation string     `json:"remediation"`
}
