package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

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

	if result, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		_ = result
		return nil, fmt.Errorf("store: set journal_mode: %w", err)
	}

	if result, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		_ = result
		return nil, fmt.Errorf("store: set busy_timeout: %w", err)
	}

	s := &Store{db: db}

	if err := s.runMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: run migrations: %w", err)
	}

	return s, nil
}

// runMigrations creates the schema_versions table and applies pending migrations.
func (s *Store) runMigrations() error {
	// Create schema_versions table
	if result, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_versions (
			version TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL
		)
	`); err != nil {
		_ = result
		return fmt.Errorf("create schema_versions: %w", err)
	}

	// Get applied versions
	applied := make(map[string]bool)
	rows, err := s.db.Query("SELECT version FROM schema_versions")
	if err != nil {
		return fmt.Errorf("query applied versions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("scan version: %w", err)
		}
		applied[v] = true
	}

	// Apply pending migrations
	for _, m := range Migrations {
		if applied[m.Version] {
			continue
		}

		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", m.Version, err)
		}

		if _, err := tx.Exec(m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", m.Version, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_versions (version, applied_at) VALUES (?, datetime('now'))", m.Version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.Version, err)
		}
	}

	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for direct access if needed.
func (s *Store) DB() *sql.DB {
	return s.db
}
