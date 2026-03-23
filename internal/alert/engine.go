package alert

import (
	"fmt"
	"sync"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
	"github.com/homelab/homelab-dash/internal/collector"
	"github.com/homelab/homelab-dash/internal/scanner"
)

// Engine evaluates alert rules against check results, metrics, and findings.
type Engine struct {
	rules  []Rule
	firing map[string]time.Time // key: ruleName+":" + target → first seen time
	fired  map[string]bool      // key: ruleName+":" + target → already fired this episode
	mu     sync.Mutex
}

// NewEngine creates a new alert engine.
func NewEngine(rules []Rule) *Engine {
	return &Engine{
		rules:  rules,
		firing: make(map[string]time.Time),
		fired:  make(map[string]bool),
	}
}

// Rules returns the configured rules.
func (e *Engine) Rules() []Rule {
	return e.rules
}

// Evaluate checks rules against the provided data and returns any alerts that should fire.
func (e *Engine) Evaluate(
	results []checker.CheckResult,
	points []collector.DataPoint,
	findings []scanner.Finding,
) []FiredAlert {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	var alerts []FiredAlert

	// Track which targets matched each rule this tick
	matched := make(map[string]bool)

	for _, rule := range e.rules {
		matches := e.checkRule(rule, results, points, findings)

		for _, m := range matches {
			key := rule.Name + ":" + m.target
			matched[key] = true

			if firstSeen, ok := e.firing[key]; ok {
				// Condition still holds - check if duration elapsed
				if !e.fired[key] && now.Sub(firstSeen) >= rule.Duration {
					alerts = append(alerts, FiredAlert{
						Rule:    rule,
						Target:  m.target,
						Message: m.message,
						FiredAt: now,
					})
					e.fired[key] = true
				}
			} else {
				// First time seeing this condition
				e.firing[key] = now
				e.fired[key] = false
				// If duration is 0, fire immediately
				if rule.Duration == 0 {
					alerts = append(alerts, FiredAlert{
						Rule:    rule,
						Target:  m.target,
						Message: m.message,
						FiredAt: now,
					})
					e.fired[key] = true
				}
			}
		}
	}

	// Clear firing state for rules that no longer match
	for key := range e.firing {
		if !matched[key] {
			delete(e.firing, key)
			delete(e.fired, key)
		}
	}

	return alerts
}

type ruleMatch struct {
	target  string
	message string
}

func (e *Engine) checkRule(
	rule Rule,
	results []checker.CheckResult,
	points []collector.DataPoint,
	findings []scanner.Finding,
) []ruleMatch {
	switch rule.Condition {
	case ConditionStatusDown:
		return e.checkStatusDown(rule, results, checker.StatusFail)
	case ConditionStatusDegraded:
		return e.checkStatusDown(rule, results, checker.StatusWarn)
	case ConditionMetricAbove:
		return e.checkMetricAbove(rule, points)
	case ConditionSecurityHigh:
		return e.checkSecurityHigh(rule, findings)
	case ConditionVMStopped:
		return e.checkVMStopped(rule, results)
	}
	return nil
}

func (e *Engine) checkStatusDown(rule Rule, results []checker.CheckResult, status checker.Status) []ruleMatch {
	var matches []ruleMatch
	for _, r := range results {
		if r.Status != status {
			continue
		}
		if rule.Target != "" && r.Target != rule.Target {
			continue
		}
		matches = append(matches, ruleMatch{
			target:  r.Target,
			message: fmt.Sprintf("%s is %s: %s", r.Target, status.String(), r.Message),
		})
	}
	return matches
}

func (e *Engine) checkMetricAbove(rule Rule, points []collector.DataPoint) []ruleMatch {
	var matches []ruleMatch

	// Find latest point for the target+metric
	var latest *collector.DataPoint
	for i := range points {
		p := &points[i]
		if p.Metric != rule.Metric {
			continue
		}
		if rule.Target != "" && p.Target != rule.Target {
			continue
		}
		if latest == nil || p.Timestamp.After(latest.Timestamp) {
			latest = p
		}
	}

	if latest != nil && latest.Value > rule.Threshold {
		matches = append(matches, ruleMatch{
			target:  latest.Target,
			message: fmt.Sprintf("%s is %.1f (threshold: %.1f)", rule.Metric, latest.Value, rule.Threshold),
		})
	}
	return matches
}

func (e *Engine) checkSecurityHigh(rule Rule, findings []scanner.Finding) []ruleMatch {
	var matches []ruleMatch
	seen := make(map[string]bool)

	for _, f := range findings {
		if f.Severity < scanner.SeverityHigh {
			continue
		}
		if rule.Target != "" && f.Target != rule.Target {
			continue
		}
		if !seen[f.Target] {
			matches = append(matches, ruleMatch{
				target:  f.Target,
				message: fmt.Sprintf("Security finding: %s (severity: %s)", f.Title, f.Severity.String()),
			})
			seen[f.Target] = true
		}
	}
	return matches
}

func (e *Engine) checkVMStopped(rule Rule, results []checker.CheckResult) []ruleMatch {
	var matches []ruleMatch
	for _, r := range results {
		if r.Check != "proxmox.vm.status" {
			continue
		}
		if r.Status != checker.StatusFail {
			continue
		}
		if rule.Target != "" && r.Target != rule.Target {
			continue
		}
		matches = append(matches, ruleMatch{
			target:  r.Target,
			message: fmt.Sprintf("VM status check failed: %s", r.Message),
		})
	}
	return matches
}
