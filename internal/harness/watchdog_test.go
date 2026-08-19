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
