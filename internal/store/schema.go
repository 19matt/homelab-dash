package store

// Table name constants for database queries.
const (
	TableCheckResults    = "check_results"
	TableDataPoints      = "data_points"
	TableFindings        = "findings"
	TableFindingsHistory = "findings_history"
	TableAlertEvents     = "alert_events"
	TableAuditEvents     = "audit_events"
	TableSchemaVersions  = "schema_versions"
)

// Column name constants for check_results table.
const (
	ColTimestamp = "timestamp"
	ColTarget    = "target"
	ColCheckName = "check_name"
	ColStatus    = "status"
	ColMessage   = "message"
	ColLatencyMs = "latency_ms"
	ColValue     = "value"
	ColMetric    = "metric"
	ColLabels    = "labels"
)

// Column name constants for findings/findings_history tables.
const (
	ColID          = "id"
	ColScanner     = "scanner"
	ColTitle       = "title"
	ColDescription = "description"
	ColSeverity    = "severity"
	ColRemediation = "remediation"
	ColScanID      = "scan_id"
	ColFirstSeen   = "first_seen"
	ColLastSeen    = "last_seen"
	ColResolvedAt  = "resolved_at"
)

// Column name constants for alert_events table.
const (
	ColRuleName = "rule_name"
)

// Column name constants for audit_events table.
const (
	ColType = "type"
)

// Column name constants for schema_versions table.
const (
	ColVersion   = "version"
	ColAppliedAt = "applied_at"
)
