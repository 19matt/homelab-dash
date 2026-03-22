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
`

// Open opens a SQLite database at the given path and runs migrations.
func Open(path string) (*sql.DB, error) {
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

	return db, nil
}
