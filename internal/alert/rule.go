package alert

import "time"

// RuleCondition represents the type of condition an alert rule evaluates.
type RuleCondition string

const (
	ConditionStatusDown     RuleCondition = "status == down"
	ConditionStatusDegraded RuleCondition = "status == degraded"
	ConditionMetricAbove    RuleCondition = "metric > threshold"
	ConditionSecurityHigh   RuleCondition = "security.severity >= high"
	ConditionVMStopped      RuleCondition = "vm.status == stopped"
)

// Rule defines an alert rule.
type Rule struct {
	Name      string
	Condition RuleCondition
	Target    string        // "" = all targets
	Metric    string        // for ConditionMetricAbove
	Threshold float64       // for ConditionMetricAbove
	Duration  time.Duration // how long condition must hold before firing
	Webhook   string
	Severity  string // "info" | "warn" | "critical"
}

// FiredAlert represents an alert that has been triggered.
type FiredAlert struct {
	Rule    Rule
	Target  string
	Message string
	FiredAt time.Time
}
