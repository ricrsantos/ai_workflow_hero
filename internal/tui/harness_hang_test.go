package tui

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestHarnessHealthSkipsExpectedHarnessResponse(t *testing.T) {
	cases := []struct {
		name       string
		permission bool
		question   bool
	}{
		{name: "permission", permission: true},
		{name: "question", question: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTestModel(nil)
			m.streaming = true
			m.harnessHealthStatus = harness.HealthSuspected
			m.harnessHealthInFlight = true
			m.harnessPermissionPending = tc.permission
			m.harnessQuestionPending = tc.question
			beforeTranscript := len(m.transcript)

			next, probeCmd := m.handleHarnessHealthProbe()
			if probeCmd == nil {
				t.Fatal("expected watchdog to schedule its next tick")
			}
			if next.harnessHealthInFlight {
				t.Fatal("health check must not remain in flight while waiting for input")
			}
			if next.harnessHealthStatus != harness.HealthHealthy {
				t.Fatalf("health status=%q want healthy", next.harnessHealthStatus)
			}

			next, cmd := next.handleHarnessHealthResult(harnessHealthResultMsg{
				status: harness.HealthFailed,
				health: harness.HarnessHealth{Details: "process unavailable"},
			})
			if cmd != nil {
				t.Fatal("expected no corrective command for a stale probe result")
			}
			if !next.streaming {
				t.Fatal("stale failed probe must not cancel the waiting execution")
			}
			if len(next.transcript) != beforeTranscript {
				t.Fatalf("unexpected watchdog alert while waiting: %d transcript rows", len(next.transcript))
			}
		})
	}
}
