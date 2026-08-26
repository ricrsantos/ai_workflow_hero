package harness_test

import (
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestWatchdogEvaluateFailedWhenProcessDead(t *testing.T) {
	var w harness.Watchdog
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	w.Reset(now)
	got := w.Evaluate(now.Add(time.Minute), harness.HarnessHealth{ProcessAlive: false}, harness.CursorStallTimeout)
	if got != harness.HealthFailed {
		t.Fatalf("status=%q want failed", got)
	}
}

func TestWatchdogEvaluateDegradedWhenServerDown(t *testing.T) {
	var w harness.Watchdog
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	w.Reset(now)
	got := w.Evaluate(now.Add(time.Minute), harness.HarnessHealth{
		ProcessAlive: true,
		ServerAlive:  false,
	}, harness.OpenCodeStallTimeout)
	if got != harness.HealthDegraded {
		t.Fatalf("status=%q want degraded", got)
	}
}

func TestWatchdogEvaluateSuspectedHangAfterStall(t *testing.T) {
	var w harness.Watchdog
	start := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	w.Reset(start)
	probe := harness.HarnessHealth{ProcessAlive: true, ServerAlive: true, SessionAlive: true}
	got := w.Evaluate(start.Add(6*time.Minute), probe, 5*time.Minute)
	if got != harness.HealthSuspected {
		t.Fatalf("status=%q want suspected_hang", got)
	}
}

func TestWatchdogActivityResetsStallClock(t *testing.T) {
	var w harness.Watchdog
	start := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	w.Reset(start)
	w.RecordDelta(harness.StreamDelta{Kind: harness.StreamKindText, Text: "hi"}, start.Add(4*time.Minute))
	probe := harness.HarnessHealth{ProcessAlive: true, ServerAlive: true, SessionAlive: true}
	got := w.Evaluate(start.Add(6*time.Minute), probe, 5*time.Minute)
	if got != harness.HealthHealthy {
		t.Fatalf("status=%q want healthy after recent activity", got)
	}
}

func TestWatchdogHealthyWithinTimeout(t *testing.T) {
	var w harness.Watchdog
	start := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	w.Reset(start)
	probe := harness.HarnessHealth{ProcessAlive: true, ServerAlive: true, SessionAlive: true}
	got := w.Evaluate(start.Add(2*time.Minute), probe, 5*time.Minute)
	if got != harness.HealthHealthy {
		t.Fatalf("status=%q want healthy", got)
	}
}

func TestWatchdogEvaluateFailedWhenSessionDead(t *testing.T) {
	var w harness.Watchdog
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	w.Reset(now)
	got := w.Evaluate(now.Add(time.Minute), harness.HarnessHealth{
		ProcessAlive: true,
		ServerAlive:  true,
		SessionAlive: false,
	}, harness.OpenCodeStallTimeout)
	if got != harness.HealthFailed {
		t.Fatalf("status=%q want failed", got)
	}
}

func TestWatchdogFileWatcherActivityDoesNotResetStallClock(t *testing.T) {
	var w harness.Watchdog
	start := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	w.Reset(start)
	w.RecordDelta(harness.ActivityDelta("file.watcher.updated", "src/foo.go", "sess"), start.Add(4*time.Minute))
	w.RecordDelta(harness.StreamDelta{Kind: harness.StreamKindSession, HarnessType: "session.status", Text: "running"}, start.Add(4*time.Minute+time.Second))
	probe := harness.HarnessHealth{ProcessAlive: true, ServerAlive: true, SessionAlive: true}
	got := w.Evaluate(start.Add(6*time.Minute), probe, 5*time.Minute)
	if got != harness.HealthSuspected {
		t.Fatalf("status=%q want suspected_hang (file.watcher / session.status must not reset stall)", got)
	}
}

func TestWatchdogHasRecentActivity(t *testing.T) {
	var w harness.Watchdog
	start := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	w.Reset(start)
	if w.HasRecentActivity(start.Add(time.Second), harness.HealthProbeInterval) {
		t.Fatal("no activity yet")
	}
	w.RecordDelta(harness.StreamDelta{Kind: harness.StreamKindText, Text: "hi"}, start.Add(10*time.Second))
	if !w.HasRecentActivity(start.Add(20*time.Second), harness.HealthProbeInterval) {
		t.Fatal("expected recent activity within probe interval")
	}
	if w.HasRecentActivity(start.Add(10*time.Second+harness.HealthProbeInterval), harness.HealthProbeInterval) {
		t.Fatal("activity outside window must not count as recent")
	}
}
