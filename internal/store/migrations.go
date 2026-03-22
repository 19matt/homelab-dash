package store

// Migration represents a database schema migration.
type Migration struct {
	Version string
	SQL     string
}

// Migrations is the ordered list of all schema migrations.
var Migrations = []Migration{
	{
		Version: "001",
		SQL: `
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
	remediation TEXT,
	scan_id TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_findings_target ON findings(target);
CREATE INDEX IF NOT EXISTS idx_findings_severity ON findings(severity);
CREATE INDEX IF NOT EXISTS idx_findings_scan_id ON findings(scan_id);

CREATE TABLE IF NOT EXISTS findings_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	first_seen DATETIME NOT NULL,
	last_seen DATETIME NOT NULL,
	resolved_at DATETIME,
	target TEXT NOT NULL,
	scanner TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT,
	severity INTEGER NOT NULL,
	remediation TEXT
);

CREATE INDEX IF NOT EXISTS idx_history_target ON findings_history(target);
CREATE INDEX IF NOT EXISTS idx_history_resolved ON findings_history(resolved_at);

CREATE TABLE IF NOT EXISTS alert_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME NOT NULL,
	rule_name TEXT NOT NULL,
	target TEXT NOT NULL,
	message TEXT,
	severity TEXT
);

CREATE INDEX IF NOT EXISTS idx_alerts_timestamp ON alert_events(timestamp);
`,
	},
	{
		Version: "002",
		SQL: `
CREATE TABLE IF NOT EXISTS audit_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME NOT NULL,
	type TEXT NOT NULL,
	target TEXT,
	message TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_type ON audit_events(type);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp);
`,
	},
	{
		Version: "003",
		SQL: `
CREATE INDEX IF NOT EXISTS idx_data_points_target_metric_ts ON data_points(target, metric, timestamp);
CREATE INDEX IF NOT EXISTS idx_check_results_target_check_ts ON check_results(target, check_name, timestamp);
`,
	},
}
