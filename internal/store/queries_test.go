package store

import (
	"context"
	"testing"
	"time"

	"github.com/homelab/homelab-dash/internal/checker"
)

func TestSaveCheckResult(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	result := checker.CheckResult{
		Timestamp: time.Now(),
		Target:    "test-target",
		Check:     "ping",
		Status:    checker.StatusPass,
		Message:   "ok",
		Latency:   5 * time.Millisecond,
	}

	if err := s.SaveCheckResult(ctx, result); err != nil {
		t.Fatalf("SaveCheckResult failed: %v", err)
	}
}

func TestGetLatestCheckResults(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert multiple results
	now := time.Now()
	results := []checker.CheckResult{
		{Timestamp: now.Add(-2 * time.Minute), Target: "svc1", Check: "ping", Status: checker.StatusFail, Latency: 10 * time.Millisecond},
		{Timestamp: now.Add(-1 * time.Minute), Target: "svc1", Check: "ping", Status: checker.StatusPass, Latency: 5 * time.Millisecond},
		{Timestamp: now, Target: "svc2", Check: "http", Status: checker.StatusPass, Latency: 20 * time.Millisecond},
	}

	for _, r := range results {
		if err := s.SaveCheckResult(ctx, r); err != nil {
			t.Fatalf("SaveCheckResult failed: %v", err)
		}
	}

	latest, err := s.GetLatestCheckResults(ctx)
	if err != nil {
		t.Fatalf("GetLatestCheckResults failed: %v", err)
	}

	// Should have 2 results (svc1:ping latest, svc2:http)
	if len(latest) != 2 {
		t.Fatalf("expected 2 results, got %d", len(latest))
	}

	// svc1 should be pass (latest)
	for _, r := range latest {
		if r.Target == "svc1" && r.Check == "ping" {
			if r.Status != checker.StatusPass {
				t.Errorf("expected svc1:ping to be pass, got %s", r.Status)
			}
		}
	}
}

func TestGetUptimePercent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := time.Now()

	// Insert 3 pass, 1 fail
	for i := 0; i < 3; i++ {
		s.SaveCheckResult(ctx, checker.CheckResult{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			Target:    "svc1",
			Check:     "ping",
			Status:    checker.StatusPass,
		})
	}
	s.SaveCheckResult(ctx, checker.CheckResult{
		Timestamp: now.Add(-4 * time.Minute),
		Target:    "svc1",
		Check:     "ping",
		Status:    checker.StatusFail,
	})

	pct, err := s.GetUptimePercent(ctx, "svc1", "ping", time.Hour)
	if err != nil {
		t.Fatalf("GetUptimePercent failed: %v", err)
	}

	// 3 pass / 4 total = 75%
	if pct < 74 || pct > 76 {
		t.Errorf("expected ~75%%, got %.1f%%", pct)
	}
}

func TestGetUptimePercent_NoData(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	pct, err := s.GetUptimePercent(ctx, "nonexistent", "ping", time.Hour)
	if err != nil {
		t.Fatalf("GetUptimePercent failed: %v", err)
	}

	if pct != 100.0 {
		t.Errorf("expected 100%% for no data, got %.1f%%", pct)
	}
}

func TestGetDailyUptime(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert results with explicit dates to ensure they're in different days
	yesterday := time.Now().Add(-25 * time.Hour)
	today := time.Now()

	s.SaveCheckResult(ctx, checker.CheckResult{
		Timestamp: yesterday,
		Target:    "svc1",
		Check:     "ping",
		Status:    checker.StatusFail,
	})

	s.SaveCheckResult(ctx, checker.CheckResult{
		Timestamp: today,
		Target:    "svc1",
		Check:     "ping",
		Status:    checker.StatusPass,
	})

	// Verify data was inserted
	results, err := s.GetRecentCheckResults(ctx, "svc1", 10)
	if err != nil {
		t.Fatalf("GetRecentCheckResults failed: %v", err)
	}
	t.Logf("Inserted %d check results", len(results))

	uptime, err := s.GetDailyUptime(ctx, "svc1", "ping", 90)
	if err != nil {
		t.Fatalf("GetDailyUptime failed: %v", err)
	}

	t.Logf("Got %d days of uptime data", len(uptime))
	for _, u := range uptime {
		t.Logf("  %s: %.1f%%", u.Date, u.Percent)
	}

	if len(uptime) < 1 {
		t.Fatalf("expected at least 1 day of data, got %d", len(uptime))
	}
}

func TestSaveAuditEvent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	event := AuditEvent{
		Timestamp: time.Now(),
		Type:      AuditStartup,
		Target:    "-",
		Message:   "test startup",
	}

	if err := s.SaveAuditEvent(ctx, event); err != nil {
		t.Fatalf("SaveAuditEvent failed: %v", err)
	}

	events, err := s.GetAuditEvents(ctx, time.Now().Add(-time.Hour), "", 10, 0)
	if err != nil {
		t.Fatalf("GetAuditEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != AuditStartup {
		t.Errorf("expected type %s, got %s", AuditStartup, events[0].Type)
	}
}

func TestGetFindingsSummary(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.SaveFindings(ctx, []Finding{
		{Timestamp: time.Now(), Target: "svc1", Scanner: "ports", Title: "test", Severity: 4},    // critical
		{Timestamp: time.Now(), Target: "svc1", Scanner: "tls", Title: "test2", Severity: 3},     // high
		{Timestamp: time.Now(), Target: "svc2", Scanner: "headers", Title: "test3", Severity: 0}, // info
	})

	summary, err := s.GetFindingsSummary(ctx)
	if err != nil {
		t.Fatalf("GetFindingsSummary failed: %v", err)
	}

	if summary["critical"] != 1 {
		t.Errorf("expected 1 critical, got %d", summary["critical"])
	}
	if summary["high"] != 1 {
		t.Errorf("expected 1 high, got %d", summary["high"])
	}
	if summary["info"] != 1 {
		t.Errorf("expected 1 info, got %d", summary["info"])
	}
}

func TestRunRetentionCleanup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := time.Now()

	// Insert old and new data
	s.SaveCheckResult(ctx, checker.CheckResult{
		Timestamp: now.Add(-100 * 24 * time.Hour), // 100 days old
		Target:    "old",
		Check:     "ping",
		Status:    checker.StatusPass,
	})
	s.SaveCheckResult(ctx, checker.CheckResult{
		Timestamp: now,
		Target:    "new",
		Check:     "ping",
		Status:    checker.StatusPass,
	})

	deleted, err := s.RunRetentionCleanup(ctx, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("RunRetentionCleanup failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deletion, got %d", deleted)
	}

	// Verify new data still exists
	results, err := s.GetLatestCheckResults(ctx)
	if err != nil {
		t.Fatalf("GetLatestCheckResults failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result remaining, got %d", len(results))
	}

	if results[0].Target != "new" {
		t.Errorf("expected target 'new', got '%s'", results[0].Target)
	}
}
