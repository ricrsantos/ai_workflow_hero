package tui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func newValidationProgressFixture(t *testing.T, timeoutMinutes int) (model, *time.Time) {
	t.Helper()
	m, svc, cycleID, now := newBudgetTUIFixture(t, stageBrowserUI, timeoutMinutes)
	coverage, err := svc.Store.ListStageCoverage(cycleID, stageBrowserUI)
	if err != nil || len(coverage) != 2 {
		t.Fatalf("stage-start plan snapshot = %+v, err = %v", coverage, err)
	}
	m = m.beginValidationProgress(stageBrowserUI)
	return m, now
}

func TestProgressWarningAfterSixtySecondsAndCoalescesUntilProgress(t *testing.T) {
	m, now := newValidationProgressFixture(t, 10)
	initial := *now
	updated, message, emitted := m.checkValidationProgressWarning(initial.Add(59 * time.Second))
	if emitted || message != "" {
		t.Fatalf("warning before threshold: emitted=%v message=%q", emitted, message)
	}
	updated, message, emitted = updated.checkValidationProgressWarning(initial.Add(60 * time.Second))
	if !emitted || !strings.Contains(message, "no coverage progress for 60 seconds") || !strings.Contains(message, "screen-dashboard-admin / admin") || !strings.Contains(message, "remaining") {
		t.Fatalf("warning=%q emitted=%v", message, emitted)
	}
	updated, message, emitted = updated.checkValidationProgressWarning(initial.Add(2 * time.Minute))
	if emitted || message != "" {
		t.Fatalf("warning was not coalesced: emitted=%v message=%q", emitted, message)
	}
	delta := harness.StreamDelta{Kind: harness.StreamKindActivity, HarnessType: validationProgressHarnessType, Metadata: map[string]string{
		"phase": "visual", "coverage_id": "screen-dashboard-admin", "profile_id": "admin", "coverage_complete": "true",
	}}
	updated, message, emitted = updated.recordValidationProgressDelta(delta, initial.Add(2*time.Minute))
	if emitted || message != "" || len(updated.validationProgress.completedIDs) != 1 {
		t.Fatalf("coverage progress was not accepted safely: state=%+v message=%q emitted=%v", updated.validationProgress, message, emitted)
	}
	updated, message, emitted = updated.checkValidationProgressWarning(initial.Add(2*time.Minute + validationProgressNoProgress))
	if !emitted || !strings.Contains(message, "no coverage progress") {
		t.Fatalf("progress did not reopen the coalesced warning gate: emitted=%v message=%q", emitted, message)
	}
}

func TestProgressWarningSuspendsDuringHumanOnlyWait(t *testing.T) {
	m, now := newValidationProgressFixture(t, 10)
	initial := *now
	m = m.setValidationProgressWaiting(true, initial.Add(10*time.Second))
	updated, message, emitted := m.checkValidationProgressWarning(initial.Add(5 * time.Minute))
	if emitted || message != "" {
		t.Fatalf("warning emitted during human wait: emitted=%v message=%q", emitted, message)
	}
	updated = updated.setValidationProgressWaiting(false, initial.Add(5*time.Minute))
	updated, message, emitted = updated.checkValidationProgressWarning(initial.Add(5*time.Minute + 49*time.Second))
	if emitted || message != "" {
		t.Fatalf("human wait counted toward no-progress threshold: emitted=%v message=%q", emitted, message)
	}
	_, message, emitted = updated.checkValidationProgressWarning(initial.Add(5*time.Minute + 50*time.Second))
	if !emitted || !strings.Contains(message, "60 seconds") {
		t.Fatalf("active no-progress time did not resume: emitted=%v message=%q", emitted, message)
	}
}

func TestProgressWarningAfterThirtyRepetitiveEvents(t *testing.T) {
	m, now := newValidationProgressFixture(t, 10)
	var warning string
	var emitted bool
	for i := 0; i < validationProgressRepeatLimit; i++ {
		m, warning, emitted = m.recordValidationProgressDelta(harness.StreamDelta{
			Kind: harness.StreamKindTool, Phase: harness.StreamPhaseStarted, Text: "SENTINEL_TOOL_PAYLOAD",
		}, now.Add(time.Duration(i)*time.Second))
	}
	if !emitted || !strings.Contains(warning, "30 repetitive events") || strings.Contains(warning, "SENTINEL_TOOL_PAYLOAD") {
		t.Fatalf("repetitive warning=%q emitted=%v", warning, emitted)
	}
	if m.validationProgress.toolLifecycle != "browser tool running" {
		t.Fatalf("tool lifecycle was not sanitized: %q", m.validationProgress.toolLifecycle)
	}
}

func TestProgressUsesOnlyStoreBackedCoverageAndSanitizesValidationTools(t *testing.T) {
	m, now := newValidationProgressFixture(t, 10)
	initial := *now
	unknown := harness.StreamDelta{Kind: harness.StreamKindActivity, HarnessType: validationProgressHarnessType,
		Text: "SENTINEL_PROGRESS_PAYLOAD", Metadata: map[string]string{"phase": "visual", "coverage_id": "SENTINEL_PASSWORD", "profile_id": "admin"}}
	updated, _, _ := m.recordValidationProgressDelta(unknown, initial.Add(time.Second))
	if updated.validationProgress.currentID != "screen-dashboard-admin" {
		t.Fatalf("unknown coverage ID was trusted: %q", updated.validationProgress.currentID)
	}
	valid := harness.StreamDelta{Kind: harness.StreamKindActivity, HarnessType: validationProgressHarnessType,
		Text: "SENTINEL_PROGRESS_PAYLOAD", Metadata: map[string]string{"phase": "visual", "coverage_id": "screen-dashboard-admin", "profile_id": "admin"}}
	updated, _, _ = updated.recordValidationProgressDelta(valid, initial.Add(2*time.Second))
	lines := strings.Join(updated.validationProgressStatusLines(initial.Add(2*time.Second)), "\n")
	if !strings.Contains(lines, "screen-dashboard-admin") || !strings.Contains(lines, "admin") || strings.Contains(lines, "SENTINEL") {
		t.Fatalf("progress display=%q", lines)
	}

	updated.executes = map[string]convExecute{"validation": {ID: "validation", StageName: stageBrowserUI, AgentMsgIndex: -1}}
	updated.applyStreamDelta(streamDeltaMsg{executeID: "validation", delta: harness.StreamDelta{
		Kind: harness.StreamKindTool, Phase: harness.StreamPhaseStarted, Text: "SENTINEL_TOOL_PAYLOAD", AgentName: "secret-agent", Model: "private-model",
	}})
	for _, entry := range updated.transcript {
		if strings.Contains(entry.content, "SENTINEL_TOOL_PAYLOAD") || strings.Contains(entry.content, "secret-agent") || strings.Contains(entry.content, "private-model") {
			t.Fatalf("raw validation tool data reached transcript: %+v", entry)
		}
	}
	if updated.validationProgress.toolLifecycle != "browser tool running" {
		t.Fatalf("sanitized lifecycle=%q", updated.validationProgress.toolLifecycle)
	}
}

func TestBlockedBrowserHandoffPersistsAndStopsAutomaticResume(t *testing.T) {
	m, _ := newValidationProgressFixture(t, 10)
	cycleRow, err := m.svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{
		"status": "blocked", "summary": "SENTINEL_REPORT_SUMMARY",
		"preparation": map[string]any{"status": "blocked", "method": "cli", "verified_profile_ids": []string{}},
		"blockers": []any{map[string]any{
			"id": "blocked-admin", "reason": "account_unusable",
			"affected_coverage_ids": []string{"screen-dashboard-admin"}, "affected_profile_ids": []string{"admin"},
			"uncertainty": "SENTINEL_BLOCKER_UNCERTAINTY", "next_action": "SENTINEL_BLOCKER_ACTION",
		}},
		"coverage": map[string]any{"planned_ids": []string{"screen-dashboard-admin", "screen-settings-operator"}, "items": []any{
			map[string]any{"id": "screen-dashboard-admin", "result": "blocked", "evidence": []string{}, "checks": map[string]any{}},
			map[string]any{"id": "screen-settings-operator", "result": "skipped", "evidence": []string{}, "checks": map[string]any{}},
		}},
		"failures": []any{},
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	streaming := &streamingHarness{}
	m.svc.Harness = streaming
	m.stageHandoffLive = true
	m.stageHandoffStage = stageBrowserUI
	m.stageHandoffOutputs = []string{"browser_ui_agent:\n" + string(encoded)}
	m.stageHandoffExpectedAgents = []string{agentBrowserUI}
	m.orchestrationLive = true
	m.runtimeAgentName = agentBrowserUI

	updated, refresh := m.resumeOrchestratorAfterStageHandoff()
	if refresh == nil {
		t.Fatal("blocked result must refresh durable status")
	}
	stage, err := m.svc.Store.GetStage(cycleRow.ID, stageBrowserUI)
	if err != nil || stage.Status != store.StageBlocked {
		t.Fatalf("stage=%+v err=%v, want durable Blocked; transcript=%q", stage, err, strings.Join(transcriptContents(updated), "\n"))
	}
	if streaming.ExecuteCount() != 0 {
		t.Fatalf("blocked handoff automatically resumed orchestrator/repair: executes=%d", streaming.ExecuteCount())
	}
	if !strings.Contains(strings.Join(transcriptContents(updated), "\n"), "/hero-continue") || !strings.Contains(strings.Join(transcriptContents(updated), "\n"), "screen-dashboard-admin") {
		t.Fatalf("blocked intervention details missing: %+v", updated.transcript)
	}
	transcript := strings.Join(transcriptContents(updated), "\n")
	for _, secret := range []string{"SENTINEL_REPORT_SUMMARY", "SENTINEL_BLOCKER_UNCERTAINTY", "SENTINEL_BLOCKER_ACTION"} {
		if strings.Contains(transcript, secret) {
			t.Fatalf("untrusted report free text reached chat: %s", secret)
		}
	}
	if updated.validationProgress.active {
		t.Fatal("blocked stage retained active progress state")
	}
}

func transcriptContents(m model) []string {
	contents := make([]string, 0, len(m.transcript))
	for _, entry := range m.transcript {
		contents = append(contents, entry.content)
	}
	return contents
}
