package alert

import (
	"testing"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
)

func TestEvaluate_StatusDown_FiresAfterDuration(t *testing.T) {
	rules := []Rule{
		{Name: "down", Condition: ConditionStatusDown, Duration: 100 * time.Millisecond, Severity: "critical"},
	}
	engine := NewEngine(rules)

	result := checker.CheckResult{
		Target:  "svc1",
		Check:   "ping",
		Status:  checker.StatusFail,
		Message: "timeout",
	}

	// First tick: should record first seen, not fire
	alerts := engine.Evaluate([]checker.CheckResult{result}, nil, nil)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts on first tick, got %d", len(alerts))
	}

	// Wait for duration to elapse
	time.Sleep(150 * time.Millisecond)

	// Second tick: should fire
	alerts = engine.Evaluate([]checker.CheckResult{result}, nil, nil)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert after duration, got %d", len(alerts))
	}

	// Third tick: should not re-fire
	alerts = engine.Evaluate([]checker.CheckResult{result}, nil, nil)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts on re-fire, got %d", len(alerts))
	}
}

func TestEvaluate_StatusDown_ClearsOnRecover(t *testing.T) {
	rules := []Rule{
		{Name: "down", Condition: ConditionStatusDown, Duration: 0, Severity: "critical"},
	}
	engine := NewEngine(rules)

	fail := checker.CheckResult{Target: "svc1", Check: "ping", Status: checker.StatusFail}
	pass := checker.CheckResult{Target: "svc1", Check: "ping", Status: checker.StatusPass}

	// Fail: fire immediately (duration=0)
	alerts := engine.Evaluate([]checker.CheckResult{fail}, nil, nil)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}

	// Recover: clear state
	engine.Evaluate([]checker.CheckResult{pass}, nil, nil)

	// Fail again: should fire again (new episode)
	time.Sleep(10 * time.Millisecond)
	alerts = engine.Evaluate([]checker.CheckResult{fail}, nil, nil)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert after recover+fail, got %d", len(alerts))
	}
}

func TestEvaluate_NoFiringWhenDurationNotElapsed(t *testing.T) {
	rules := []Rule{
		{Name: "down", Condition: ConditionStatusDown, Duration: time.Hour, Severity: "critical"},
	}
	engine := NewEngine(rules)

	result := checker.CheckResult{Target: "svc1", Check: "ping", Status: checker.StatusFail}

	// Multiple ticks within duration: should never fire
	for i := 0; i < 5; i++ {
		alerts := engine.Evaluate([]checker.CheckResult{result}, nil, nil)
		if len(alerts) != 0 {
			t.Errorf("tick %d: expected 0 alerts, got %d", i, len(alerts))
		}
	}
}

func TestEvaluate_TargetFilter(t *testing.T) {
	rules := []Rule{
		{Name: "svc1-down", Condition: ConditionStatusDown, Target: "svc1", Duration: 0, Severity: "critical"},
	}
	engine := NewEngine(rules)

	// svc2 should not trigger svc1 rule
	result := checker.CheckResult{Target: "svc2", Check: "ping", Status: checker.StatusFail}
	alerts := engine.Evaluate([]checker.CheckResult{result}, nil, nil)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for svc2, got %d", len(alerts))
	}

	// svc1 should trigger
	result.Target = "svc1"
	alerts = engine.Evaluate([]checker.CheckResult{result}, nil, nil)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert for svc1, got %d", len(alerts))
	}
}
