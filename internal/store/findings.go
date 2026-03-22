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
}

// SaveFindings inserts findings into the database in a transaction.
func (s *Store) SaveFindings(ctx context.Context, findings []Finding) error {
	if len(findings) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO findings (timestamp, target, scanner, title, description, severity, remediation)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("store: prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, f := range findings {
		if _, err := stmt.ExecContext(ctx,
			f.Timestamp, f.Target, f.Scanner, f.Title, f.Description, f.Severity, f.Remediation); err != nil {
			return fmt.Errorf("store: insert finding: %w", err)
		}
	}

	return tx.Commit()
}

// GetFindings returns all findings ordered by severity descending then timestamp.
func (s *Store) GetFindings(ctx context.Context) ([]Finding, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, timestamp, target, scanner, title, description, severity, remediation
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
			&f.Title, &f.Description, &f.Severity, &f.Remediation); err != nil {
			return nil, fmt.Errorf("store: scan finding: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// GetFindingsFiltered returns findings with optional filters.
func (s *Store) GetFindingsFiltered(ctx context.Context, target string, severity int, since time.Time) ([]Finding, error) {
	query := `SELECT id, timestamp, target, scanner, title, description, severity, remediation
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
			&f.Title, &f.Description, &f.Severity, &f.Remediation); err != nil {
			return nil, fmt.Errorf("store: scan finding: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// GetFindingsSummary returns a count of findings grouped by severity.
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

// GetFindingByID returns a single finding by ID.
func (s *Store) GetFindingByID(ctx context.Context, id int64) (*Finding, error) {
	var f Finding
	err := s.db.QueryRowContext(ctx,
		`SELECT id, timestamp, target, scanner, title, description, severity, remediation
		 FROM findings WHERE id = ?`, id,
	).Scan(&f.ID, &f.Timestamp, &f.Target, &f.Scanner,
		&f.Title, &f.Description, &f.Severity, &f.Remediation)
	if err != nil {
		return nil, fmt.Errorf("store: get finding by id: %w", err)
	}
	return &f, nil
}
