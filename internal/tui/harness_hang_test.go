package tui

import (
	"testing"
	"time"

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

func TestHealthFailedWarnsWithoutCancel(t *testing.T) {
	m := NewTestModel(nil)
	m.streaming = true
	before := len(m.transcript)

	next, cmd := m.handleHarnessHealthResult(harnessHealthResultMsg{
		status: harness.HealthFailed,
		health: harness.HarnessHealth{ProcessAlive: false, SessionAlive: false, Details: "session idle"},
	})
	if cmd != nil {
		t.Fatal("health path must not return cancelStreamCmd")
	}
	if !next.streaming {
		t.Fatal("HealthFailed must leave the stream running")
	}
	if next.harnessHealthStatus != harness.HealthFailed {
		t.Fatalf("status=%q want failed", next.harnessHealthStatus)
	}
	if len(next.transcript) <= before {
		t.Fatal("expected a HealthFailed warning in the transcript")
	}
}

func TestHealthFailedWhileReconnectingDoesNotCancel(t *testing.T) {
	m := NewTestModel(nil)
	m.streaming = true
	m.harnessReconnecting = true

	next, cmd := m.handleHarnessHealthResult(harnessHealthResultMsg{
		status: harness.HealthFailed,
		health: harness.HarnessHealth{ProcessAlive: false, Details: "connection closed"},
	})
	if cmd != nil {
		t.Fatal("reconnecting HealthFailed must not cancel Execute")
	}
	if !next.streaming {
		t.Fatal("stream must remain active while reconnecting")
	}
	if next.harnessHealthStatus != harness.HealthDegraded {
		t.Fatalf("status=%q want degraded while reconnecting", next.harnessHealthStatus)
	}
}

func TestStaleHealthProbeDoesNotCancelNextExecute(t *testing.T) {
	m := NewTestModel(nil)
	m.streaming = true
	m.harnessHealthGeneration = 2
	beforeTranscript := len(m.transcript)

	next, cmd := m.handleHarnessHealthResult(harnessHealthResultMsg{
		generation: 1,
		status:     harness.HealthFailed,
		health:     harness.HarnessHealth{Details: "session idle", ProcessAlive: false, SessionAlive: false},
	})
	if cmd != nil {
		t.Fatal("stale HealthFailed must not cancel the next Execute")
	}
	if !next.streaming {
		t.Fatal("stale probe must leave the current stream running")
	}
	if len(next.transcript) != beforeTranscript {
		t.Fatalf("stale probe must not insert a warning, got %d extra rows", len(next.transcript)-beforeTranscript)
	}
}

func TestHarnessPermissionPausesWatchdogUntilResponse(t *testing.T) {
	m := NewTestModel(nil)
	m.harnessWatchdog.Reset(time.Now())
	respCh := make(chan harness.PermissionResponse, 1)

	updated, _ := m.handleConversationMsg(harnessPermissionRequestMsg{respCh: respCh})
	next, ok := updated.(model)
	if !ok || !next.harnessPermissionPending || !next.harnessWatchdog.IsPaused() {
		t.Fatalf("permission must pause watchdog: %+v", next)
	}

	next = next.replyHarnessPermission(true)
	if next.harnessPermissionPending || next.harnessWatchdog.IsPaused() {
		t.Fatalf("permission response must resume watchdog: %+v", next)
	}
}
