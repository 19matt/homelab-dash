package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
)

// SaveCheckResult inserts a check result into the database.
func (s *Store) SaveCheckResult(ctx context.Context, r checker.CheckResult) error {
	query := fmt.Sprintf(`INSERT INTO %s (%s, %s, %s, %s, %s, %s)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		TableCheckResults, ColTimestamp, ColTarget, ColCheckName, ColStatus, ColMessage, ColLatencyMs)
	result, err := s.db.ExecContext(ctx, query,
		r.Timestamp, r.Target, r.Check, int(r.Status), r.Message, r.Latency.Milliseconds(),
	)
	if err != nil {
		return fmt.Errorf("store: save check result: %w", err)
	}
	_ = result
	return nil
}

// GetLatestCheckResults returns the most recent check result per target+check combination.
func (s *Store) GetLatestCheckResults(ctx context.Context) ([]checker.CheckResult, error) {
	query := fmt.Sprintf(`SELECT cr.%s, cr.%s, cr.%s, cr.%s, cr.%s, cr.%s
		 FROM %s cr
		 INNER JOIN (
		   SELECT %s, %s, MAX(%s) AS max_ts
		   FROM %s
		   GROUP BY %s, %s
		 ) latest ON cr.%s = latest.%s
		     AND cr.%s = latest.%s
		     AND cr.%s = latest.max_ts
		 ORDER BY cr.%s, cr.%s`,
		ColTimestamp, ColTarget, ColCheckName, ColStatus, ColMessage, ColLatencyMs,
		TableCheckResults,
		ColTarget, ColCheckName, ColTimestamp, TableCheckResults,
		ColTarget, ColCheckName,
		ColTarget, ColTarget,
		ColCheckName, ColCheckName,
		ColTimestamp,
		ColTarget, ColCheckName)
	rows, err := s.db.QueryContext(ctx, query)
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

	// Get all check results for the target and compute daily uptime in Go
	// (SQLite strftime doesn't work with modernc's time.Time storage)
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, status
		 FROM check_results
		 WHERE target = ? AND check_name = ? AND timestamp > ?
		 ORDER BY timestamp`,
		target, check, since)
	if err != nil {
		return nil, fmt.Errorf("store: get daily uptime: %w", err)
	}
	defer rows.Close()

	// Group by day in Go
	type dayStats struct {
		total  int
		passed int
	}
	dayMap := make(map[string]*dayStats)

	for rows.Next() {
		var ts time.Time
		var status int
		if err := rows.Scan(&ts, &status); err != nil {
			return nil, fmt.Errorf("store: scan daily uptime: %w", err)
		}
		day := ts.Format("2006-01-02")
		if dayMap[day] == nil {
			dayMap[day] = &dayStats{}
		}
		dayMap[day].total++
		if status == 0 {
			dayMap[day].passed++
		}
	}

	// Convert to sorted slice
	var dates []string
	for d := range dayMap {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	var results []DailyUptime
	for _, d := range dates {
		stats := dayMap[d]
		pct := 100.0
		if stats.total > 0 {
			pct = float64(stats.passed) / float64(stats.total) * 100
		}
		results = append(results, DailyUptime{Date: d, Percent: pct})
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

// BatchUptime represents uptime for a target+check combination.
type BatchUptime struct {
	Target  string
	Check   string
	Percent float64
}

// GetBatchUptime calculates uptime for multiple targets in a single query.
func (s *Store) GetBatchUptime(ctx context.Context, targets []string, checks map[string]string, window time.Duration) (map[string]float64, error) {
	if len(targets) == 0 {
		return make(map[string]float64), nil
	}

	since := time.Now().Add(-window)

	// Build query with IN clause
	query := `SELECT target, check_name,
	                 COUNT(*) as total,
	                 COALESCE(SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END), 0) as passed
	          FROM check_results
	          WHERE timestamp > ? AND target IN (?` + strings.Repeat(",?", len(targets)-1) + `)
	          GROUP BY target, check_name`

	args := []interface{}{since}
	for _, t := range targets {
		args = append(args, t)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: get batch uptime: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var target, check string
		var total, passed int
		if err := rows.Scan(&target, &check, &total, &passed); err != nil {
			return nil, fmt.Errorf("store: scan batch uptime: %w", err)
		}
		pct := 100.0
		if total > 0 {
			pct = float64(passed) / float64(total) * 100.0
		}
		result[target+":"+check] = pct
	}
	return result, rows.Err()
}
