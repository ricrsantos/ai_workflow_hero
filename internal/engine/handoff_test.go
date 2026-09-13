package engine

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func seedHandoffCycle(t *testing.T, s *store.Store, scopeNativeOnly bool) int64 {
	t.Helper()
	snapshot := `{"scope":{"backend":true,"frontend":true,"native":true}}`
	if scopeNativeOnly {
		snapshot = `{"scope":{"native":true}}`
	}
	id, err := s.CreateCycle(store.Cycle{
		Number: 1, Title: "T", Objective: "O", Status: store.CycleStatusActive,
		StartedAt: "2026-08-07T12:00:00Z", ConfigSnapshotJSON: snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	stages := []store.Stage{
		{CycleID: id, Name: "research", Status: store.StageCompleted, MaxIterations: 2, Iteration: 1, SortOrder: 0},
		{CycleID: id, Name: "planning", Status: store.StageCompleted, MaxIterations: 2, Iteration: 1, SortOrder: 1},
		{CycleID: id, Name: "implementation", Status: store.StageCompleted, MaxIterations: 4, Iteration: 1, SortOrder: 2},
		{CycleID: id, Name: "qa", Status: store.StageWaiting, MaxIterations: 2, SortOrder: 3},
		{CycleID: id, Name: "judge", Status: store.StageWaiting, MaxIterations: 2, SortOrder: 4},
	}
	if err := s.CreateStages(stages); err != nil {
		t.Fatal(err)
	}
	return id
}

func qaFailedReportJSON(t *testing.T) []byte {
	t.Helper()
	raw := `{
		"status":"failed",
		"summary":"handoff failure",
		"failures":[{
			"owner":"generic_agent",
			"file":"internal/tui/stage_handoff.go",
			"issue":"missing atomic close",
			"acceptance_criteria":"CloseStageFailedWithFindings persists findings in one transaction",
			"repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}
		}]
	}`
	return []byte(raw)
}

func TestCloseStageFailedWithFindings_AtomicSuccess(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	if err := e.StartStage(id, "qa"); err != nil {
		t.Fatal(err)
	}

	out, err := e.CloseStageFailedWithFindings(id, "qa", qaFailedReportJSON(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.FindingIDs) != 1 || out.FindingIDs[0] != "find-qa-1" {
		t.Fatalf("finding_ids=%v", out.FindingIDs)
	}

	// Loop-back resets implementation and all later stages (including the source)
	// to Waiting, matching Engine.LoopBackToImplementation.
	qa, _ := s.GetStage(id, "qa")
	if qa.Status != store.StageWaiting {
		t.Fatalf("qa status=%s want Waiting after loop-back reset", qa.Status)
	}
	impl, _ := s.GetStage(id, "implementation")
	if impl.Status != store.StageWaiting || impl.Summary != "handoff failure" {
		t.Fatalf("implementation=%+v", impl)
	}

	findings, err := s.ListFindingsByCycle(id)
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings=%v err=%v", findings, err)
	}

	events, err := s.ListEventsByTypes(id, []string{store.EventLoopBack, store.EventStageCompleted})
	if err != nil {
		t.Fatal(err)
	}
	var loopBack *store.Event
	for i := range events {
		if events[i].Type == store.EventLoopBack {
			loopBack = &events[i]
			break
		}
	}
	if loopBack == nil {
		t.Fatal("missing loop-back event")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(loopBack.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	ids, _ := payload["finding_ids"].([]any)
	if len(ids) != 1 || ids[0] != "find-qa-1" {
		t.Fatalf("payload=%s", loopBack.PayloadJSON)
	}
}

func TestCloseStageFailedWithFindings_AcceptsGoRecursiveEvidence(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	if err := e.StartStage(id, "qa"); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
		"status":"failed",
		"summary":"toolchain mismatch",
		"failures":[{
			"owner":"generic_agent",
			"file":"go.mod",
			"issue":"staticcheck built with older Go",
			"acceptance_criteria":"staticcheck analyzes the module",
			"evidence":["go test ./internal/tui","go test ./..."],
			"repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}
		}]
	}`)
	out, err := e.CloseStageFailedWithFindings(id, "qa", raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.FindingIDs) != 1 || out.FindingIDs[0] != "find-qa-1" {
		t.Fatalf("finding_ids=%v", out.FindingIDs)
	}
}

func TestCloseStageFailedWithFindings_RejectsEvidenceParentSegment(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	_ = e.StartStage(id, "qa")
	raw := []byte(`{
		"status":"failed",
		"summary":"bad evidence",
		"failures":[{
			"owner":"generic_agent",
			"file":"a.go",
			"issue":"x",
			"acceptance_criteria":"y",
			"evidence":["go test ./internal/tui","../secret.txt"],
			"repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}
		}]
	}`)
	_, err := e.CloseStageFailedWithFindings(id, "qa", raw, nil)
	var rv *ReportValidationError
	if !errors.As(err, &rv) || rv.Diagnostic.Code != reports.CodeInvalidEnum {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(rv.Diagnostic.Field, "evidence[1]") {
		t.Fatalf("field=%s", rv.Diagnostic.Field)
	}
}

func TestCloseStageFailedWithFindings_NoActionableFinding(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	_ = e.StartStage(id, "qa")

	raw := []byte(`{"status":"failed","failures":[],"summary":"empty"}`)
	_, err := e.CloseStageFailedWithFindings(id, "qa", raw, nil)
	var rv *ReportValidationError
	if !errors.As(err, &rv) || rv.Diagnostic.Code != reports.CodeNoActionableFinding {
		t.Fatalf("expected no_actionable_finding, got %v", err)
	}
	qa, _ := s.GetStage(id, "qa")
	if qa.Status != store.StageRunning {
		t.Fatalf("qa status=%s want Running", qa.Status)
	}
	if n, _ := s.ListFindingsByCycle(id); len(n) != 0 {
		t.Fatalf("findings=%v", n)
	}
}

func TestCloseStageFailedWithFindings_InvalidJSON(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	_ = e.StartStage(id, "qa")

	_, err := e.CloseStageFailedWithFindings(id, "qa", []byte(`{`), nil)
	var rv *ReportValidationError
	if !errors.As(err, &rv) || rv.Diagnostic.Code != reports.CodeInvalidJSON {
		t.Fatalf("expected invalid_json, got %v", err)
	}
	assertNoHandoffSideEffects(t, s, id, store.StageRunning)
}

func TestCloseStageFailedWithFindings_InvalidOwner(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	_ = e.StartStage(id, "qa")

	raw := []byte(`{
		"status":"failed","summary":"x",
		"failures":[{"owner":"backend_agent","file":"a.go","issue":"i","acceptance_criteria":"a"}]
	}`)
	_, err := e.CloseStageFailedWithFindings(id, "qa", raw, nil)
	var rv *ReportValidationError
	if !errors.As(err, &rv) || rv.Diagnostic.Code != reports.CodeInvalidOwner {
		t.Fatalf("expected invalid_owner, got %v", err)
	}
	assertNoHandoffSideEffects(t, s, id, store.StageRunning)
}

func TestCloseStageFailedWithFindings_MissingAcceptance(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	_ = e.StartStage(id, "qa")

	raw := []byte(`{
		"status":"failed","summary":"x",
		"failures":[{"owner":"generic_agent","file":"a.go","issue":"i","acceptance_criteria":""}]
	}`)
	_, err := e.CloseStageFailedWithFindings(id, "qa", raw, nil)
	var rv *ReportValidationError
	if !errors.As(err, &rv) || rv.Diagnostic.Code != reports.CodeMissingField {
		t.Fatalf("expected missing_field, got %v", err)
	}
	assertNoHandoffSideEffects(t, s, id, store.StageRunning)
}

func TestCloseStageFailedWithFindings_MidTxRollback(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	_ = e.StartStage(id, "qa")

	handoffTestHook = func(*sql.Tx) error { return fmt.Errorf("injected mid-tx failure") }
	t.Cleanup(func() { handoffTestHook = nil })

	_, err := e.CloseStageFailedWithFindings(id, "qa", qaFailedReportJSON(t), nil)
	if err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("expected injected error, got %v", err)
	}
	assertNoHandoffSideEffects(t, s, id, store.StageRunning)
}

func TestLoopBackStandalone_CreatesNoFindings(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedHandoffCycle(t, s, true)
	qa, _ := s.GetStage(id, "qa")
	qa.Status = store.StageFailed
	qa.CompletedAt = "2026-08-07T13:00:00Z"
	_ = s.UpdateStage(qa)
	impl, _ := s.GetStage(id, "implementation")
	impl.Status = store.StageCompleted
	_ = s.UpdateStage(impl)

	if err := e.LoopBackToImplementation(id, "qa", "admin loop-back"); err != nil {
		t.Fatal(err)
	}
	findings, err := s.ListFindingsByCycle(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("standalone loop-back created findings: %+v", findings)
	}
	events, _ := s.ListEvents(id, store.EventLoopBack, 5)
	if len(events) != 1 || strings.Contains(events[0].PayloadJSON, "finding_ids") {
		t.Fatalf("payload=%s", events[0].PayloadJSON)
	}
}

func assertNoHandoffSideEffects(t *testing.T, s *store.Store, cycleID int64, wantQAStatus string) {
	t.Helper()
	qa, _ := s.GetStage(cycleID, "qa")
	if qa.Status != wantQAStatus {
		t.Fatalf("qa status=%s want %s", qa.Status, wantQAStatus)
	}
	if f, _ := s.ListFindingsByCycle(cycleID); len(f) != 0 {
		t.Fatalf("unexpected findings: %+v", f)
	}
	events, _ := s.ListEventsByTypes(cycleID, []string{store.EventLoopBack, store.EventStageCompleted})
	if len(events) != 0 {
		t.Fatalf("unexpected events: %+v", events)
	}
}
