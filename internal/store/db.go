package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS check_results (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME NOT NULL,
	target TEXT NOT NULL,
	check_name TEXT NOT NULL,
	status INTEGER NOT NULL,
	message TEXT,
	latency_ms INTEGER
);

CREATE TABLE IF NOT EXISTS data_points (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME NOT NULL,
	target TEXT NOT NULL,
	metric TEXT NOT NULL,
	value REAL NOT NULL,
	labels TEXT
);

CREATE TABLE IF NOT EXISTS findings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME NOT NULL,
	target TEXT NOT NULL,
	scanner TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT,
	severity INTEGER NOT NULL,
	remediation TEXT
);

CREATE INDEX IF NOT EXISTS idx_findings_target ON findings(target);
CREATE INDEX IF NOT EXISTS idx_findings_severity ON findings(severity);
`

// Store wraps a SQLite database connection.
type Store struct {
	db *sql.DB
}

// Open opens a SQLite database at the given path and runs migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: set journal_mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: set busy_timeout: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for direct access if needed.
func (s *Store) DB() *sql.DB {
	return s.db
}
