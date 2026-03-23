package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/homelab/homelab-dash/internal/collector"
)

// SaveDataPoints inserts multiple data points in a single transaction.
func (s *Store) SaveDataPoints(ctx context.Context, points []collector.DataPoint) error {
	if len(points) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO data_points (timestamp, target, metric, value, labels)
		 VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("store: prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, p := range points {
		var labelsJSON string
		if len(p.Labels) > 0 {
			b, err := json.Marshal(p.Labels)
			if err != nil {
				return fmt.Errorf("store: marshal labels: %w", err)
			}
			labelsJSON = string(b)
		}

		if result, err := stmt.ExecContext(ctx, p.Timestamp, p.Target, p.Metric, p.Value, labelsJSON); err != nil {
			_ = result
			return fmt.Errorf("store: insert data point: %w", err)
		}
	}

	return tx.Commit()
}

// GetDataPoints returns data points for a target+metric within a time range.
func (s *Store) GetDataPoints(ctx context.Context, target, metric string, from, to time.Time) ([]collector.DataPoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT timestamp, target, metric, value, labels
		 FROM data_points
		 WHERE target = ? AND metric = ? AND timestamp BETWEEN ? AND ?
		 ORDER BY timestamp`,
		target, metric, from, to)
	if err != nil {
		return nil, fmt.Errorf("store: get data points: %w", err)
	}
	defer rows.Close()

	var points []collector.DataPoint
	for rows.Next() {
		var p collector.DataPoint
		var labelsJSON sql.NullString
		if err := rows.Scan(&p.Timestamp, &p.Target, &p.Metric, &p.Value, &labelsJSON); err != nil {
			return nil, fmt.Errorf("store: scan data point: %w", err)
		}
		if labelsJSON.Valid && labelsJSON.String != "" {
			json.Unmarshal([]byte(labelsJSON.String), &p.Labels)
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// GetLatestDataPoint returns the most recent data point for a target+metric.
func (s *Store) GetLatestDataPoint(ctx context.Context, target, metric string) (*collector.DataPoint, error) {
	var p collector.DataPoint
	var labelsJSON sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT timestamp, target, metric, value, labels
		 FROM data_points
		 WHERE target = ? AND metric = ?
		 ORDER BY timestamp DESC
		 LIMIT 1`,
		target, metric,
	).Scan(&p.Timestamp, &p.Target, &p.Metric, &p.Value, &labelsJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get latest data point: %w", err)
	}
	if labelsJSON.Valid && labelsJSON.String != "" {
		json.Unmarshal([]byte(labelsJSON.String), &p.Labels)
	}
	return &p, nil
}

// GetAllTargetsWithMetric returns distinct targets that have data for the given metric.
func (s *Store) GetAllTargetsWithMetric(ctx context.Context, metric string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT target FROM data_points WHERE metric = ? ORDER BY target`,
		metric)
	if err != nil {
		return nil, fmt.Errorf("store: get targets with metric: %w", err)
	}
	defer rows.Close()

	var targets []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("store: scan target: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// GetAllLatestDataPoints returns the latest value for every metric on a target.
func (s *Store) GetAllLatestDataPoints(ctx context.Context, target string) (map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT metric, value
		 FROM data_points
		 WHERE target = ?
		 GROUP BY metric
		 HAVING timestamp = MAX(timestamp)`,
		target)
	if err != nil {
		return nil, fmt.Errorf("store: get all latest data points: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var metric string
		var value float64
		if err := rows.Scan(&metric, &value); err != nil {
			return nil, fmt.Errorf("store: scan data point: %w", err)
		}
		result[metric] = value
	}
	if result == nil {
		result = make(map[string]float64)
	}

	return result, rows.Err()
}

// GetAllNodeStats returns latest metrics for all node:* targets.
func (s *Store) GetAllNodeStats(ctx context.Context) (map[string]map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT target, metric, value
		 FROM data_points
		 WHERE target LIKE 'node:%'
		   AND metric IN ('node.cpu.percent', 'node.mem.percent', 'node.uptime')
		 GROUP BY target, metric
		 HAVING timestamp = MAX(timestamp)
		 ORDER BY target, metric`)
	if err != nil {
		return nil, fmt.Errorf("store: get all node stats: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]float64)
	for rows.Next() {
		var target, metric string
		var value float64
		if err := rows.Scan(&target, &metric, &value); err != nil {
			return nil, fmt.Errorf("store: scan node stat: %w", err)
		}
		if result[target] == nil {
			result[target] = make(map[string]float64)
		}
		result[target][metric] = value
	}
	return result, rows.Err()
}
