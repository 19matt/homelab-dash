package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
)

// SaveCheckResult inserts a check result into the database.
func (s *Store) SaveCheckResult(ctx context.Context, r checker.CheckResult) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO check_results (timestamp, target, check_name, status, message, latency_ms)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.Timestamp, r.Target, r.Check, int(r.Status), r.Message, r.Latency.Milliseconds(),
	)
	if err != nil {
		return fmt.Errorf("store: save check result: %w", err)
	}
	return nil
}

// GetLatestCheckResults returns the most recent check result per target+check combination.
func (s *Store) GetLatestCheckResults(ctx context.Context) ([]checker.CheckResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cr.timestamp, cr.target, cr.check_name, cr.status, cr.message, cr.latency_ms
		 FROM check_results cr
		 INNER JOIN (
		   SELECT target, check_name, MAX(timestamp) AS max_ts
		   FROM check_results
		   GROUP BY target, check_name
		 ) latest ON cr.target = latest.target
		     AND cr.check_name = latest.check_name
		     AND cr.timestamp = latest.max_ts
		 ORDER BY cr.target, cr.check_name`)
	if err != nil {
		return nil, fmt.Errorf("store: get latest check results: %w", err)
	}
	defer rows.Close()

	var results []checker.CheckResult
	for rows.Next() {
		var r checker.CheckResult
		var latencyMs int64
		var statusInt int
		if err := rows.Scan(&r.Timestamp, &r.Target, &r.Check, &statusInt, &r.Message, &latencyMs); err != nil {
			return nil, fmt.Errorf("store: scan check result: %w", err)
		}
		r.Status = checker.Status(statusInt)
		r.Latency = time.Duration(latencyMs) * time.Millisecond
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetUptimePercent calculates the percentage of pass results over the given time window.
func (s *Store) GetUptimePercent(ctx context.Context, target, check string, window time.Duration) (float64, error) {
	since := time.Now().Add(-window)

	var total, passed int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END), 0)
		 FROM check_results
		 WHERE target = ? AND check_name = ? AND timestamp > ?`,
		target, check, since,
	).Scan(&total, &passed)
	if err != nil {
		if err == sql.ErrNoRows {
			return 100.0, nil
		}
		return 0, fmt.Errorf("store: get uptime percent: %w", err)
	}

	if total == 0 {
		return 100.0, nil
	}

	return float64(passed) / float64(total) * 100.0, nil
}

// GetRecentCheckResults returns the most recent check results for a target.
func (s *Store) GetRecentCheckResults(ctx context.Context, target string, limit int) ([]checker.CheckResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, target, check_name, status, message, latency_ms
		 FROM check_results
		 WHERE target = ?
		 ORDER BY timestamp DESC
		 LIMIT ?`,
		target, limit)
	if err != nil {
		return nil, fmt.Errorf("store: get recent check results: %w", err)
	}
	defer rows.Close()

	var results []checker.CheckResult
	for rows.Next() {
		var r checker.CheckResult
		var latencyMs int64
		var statusInt int
		if err := rows.Scan(&r.Timestamp, &r.Target, &r.Check, &statusInt, &r.Message, &latencyMs); err != nil {
			return nil, fmt.Errorf("store: scan check result: %w", err)
		}
		r.Status = checker.Status(statusInt)
		r.Latency = time.Duration(latencyMs) * time.Millisecond
		results = append(results, r)
	}
	return results, rows.Err()
}

// DailyUptime represents uptime percentage for a single day.
type DailyUptime struct {
	Date     string  `json:"date"`
	Percent  float64 `json:"percent"`
	DayIndex int     `json:"day_index,omitempty"`
}

// GetDailyUptime returns daily uptime percentages for a target over the given number of days.
func (s *Store) GetDailyUptime(ctx context.Context, target, check string, days int) ([]DailyUptime, error) {
	since := time.Now().AddDate(0, 0, -days)

	rows, err := s.db.QueryContext(ctx,
		`SELECT
			strftime('%Y-%m-%d', timestamp) AS day,
			COUNT(*) AS total,
			SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END) AS passed
		 FROM check_results
		 WHERE target = ? AND check_name = ? AND timestamp > ?
		 GROUP BY day
		 ORDER BY day`,
		target, check, since)
	if err != nil {
		return nil, fmt.Errorf("store: get daily uptime: %w", err)
	}
	defer rows.Close()

	var results []DailyUptime
	for rows.Next() {
		var d DailyUptime
		var total, passed int
		var dayPtr *string
		if err := rows.Scan(&dayPtr, &total, &passed); err != nil {
			return nil, fmt.Errorf("store: scan daily uptime: %w", err)
		}
		if dayPtr != nil {
			d.Date = *dayPtr
		} else {
			continue
		}
		if total > 0 {
			d.Percent = float64(passed) / float64(total) * 100
		} else {
			d.Percent = 100
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

// GetFindingsForTarget returns security findings for a specific target.
func (s *Store) GetFindingsForTarget(ctx context.Context, target string, limit int) ([]Finding, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, timestamp, target, scanner, title, description, severity, remediation
		 FROM findings
		 WHERE target = ?
		 ORDER BY severity DESC
		 LIMIT ?`,
		target, limit)
	if err != nil {
		return nil, fmt.Errorf("store: get findings for target: %w", err)
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
