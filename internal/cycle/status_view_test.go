package cycle_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestStatusJSONNoCycleBackwardCompatible(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Stages != nil {
		t.Fatalf("expected nil stages, got %+v", st.Stages)
	}
	if st.Findings != nil || st.Todos != nil || st.CompletionDisposition != nil {
		t.Fatalf("expected no additive blocks without a cycle: %+v", st)
	}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, key := range []string{"findings", "loopBacks", "todos", "availableActions"} {
		if strings.Contains(body, key) {
			t.Fatalf("no-cycle JSON should omit %q, got %s", key, body)
		}
	}
}

func TestStatusJSONEmptyCycleAdditiveFields(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}

	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Findings == nil || st.Todos == nil {
		t.Fatalf("expected findings/todos blocks: %+v", st)
	}
	if st.Findings.Counts.Open != 0 || len(st.Findings.Items) != 0 {
		t.Fatalf("expected zero findings: %+v", st.Findings)
	}
	if st.Todos.Pending != 0 || st.Todos.Adopted != 0 || st.Todos.DeferredFromCycle != 0 {
		t.Fatalf("expected zero todos: %+v", st.Todos)
	}
	if st.CompletionDisposition != nil {
		t.Fatalf("expected nil disposition, got %v", *st.CompletionDisposition)
	}

	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["completionDisposition"] != nil {
		t.Fatalf("expected completionDisposition null, got %#v", doc["completionDisposition"])
	}
	findings, ok := doc["findings"].(map[string]any)
	if !ok {
		t.Fatalf("missing findings in %s", raw)
	}
	counts, ok := findings["counts"].(map[string]any)
	if !ok || counts["open"] != float64(0) {
		t.Fatalf("counts: %#v", findings["counts"])
	}
	items, ok := findings["items"].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("items: %#v", findings["items"])
	}
}

func TestStatusJSONFindingsLoopBacksAndActions(t *testing.T) {
	svc, cycleID := handoffProject(t)
	defer svc.Close()
	seedImplementationPipeline(t, svc.Store, cycleID)

	if err := svc.Engine.StartStage(cycleID, "qa"); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
		"status":"failed",
		"summary":"loop-back",
		"failures":[{
			"owner":"generic_agent",
			"file":"internal/cycle/status_view.go",
			"issue":"second finding",
			"acceptance_criteria":"loop-back row includes findingIds"
		}]
	}`)
	if _, err := svc.CloseStageFailedWithFindings("qa", raw, ""); err != nil {
		t.Fatal(err)
	}
	qa, err := svc.Store.GetStage(cycleID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	qa.Status = store.StageEscalated
	if err := svc.Store.UpdateStage(qa); err != nil {
		t.Fatal(err)
	}

	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Findings.Counts.Open < 1 {
		t.Fatalf("counts=%+v", st.Findings.Counts)
	}
	if len(st.LoopBacks) != 1 {
		t.Fatalf("loopBacks=%+v", st.LoopBacks)
	}
	if st.LoopBacks[0].Round != 1 || st.LoopBacks[0].From != "qa" || st.LoopBacks[0].To != "implementation" {
		t.Fatalf("loopBack=%+v", st.LoopBacks[0])
	}
	if len(st.LoopBacks[0].FindingIDs) == 0 {
		t.Fatalf("expected findingIds in loop-back")
	}
	wantActions := []string{"hero-continue", "hero-add-todo", "hero-cancel", "hero-finish"}
	if len(st.AvailableActions) != len(wantActions) {
		t.Fatalf("actions=%v", st.AvailableActions)
	}
	for i, a := range wantActions {
		if st.AvailableActions[i] != a {
			t.Fatalf("actions=%v want %v", st.AvailableActions, wantActions)
		}
	}
}

func TestStatusJSONCompletionDisposition(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	res, err := svc.NewCycle("", "")
	if err != nil {
		t.Fatal(err)
	}
	cycleID := res.Cycle.ID
	seedImplementationPipeline(t, svc.Store, cycleID)
	qa, _ := svc.Store.GetStage(cycleID, "qa")
	qa.Status = store.StageEscalated
	if err := svc.Store.UpdateStage(qa); err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteCycleWithDeferredTodos(cycle.DeferredCompletionDetails{
		TodoIDs: []string{"find-qa-1"},
		Summary: "deferred",
	}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.CompletionDisposition == nil || *st.CompletionDisposition != store.CompletionDispositionDeferredTodos {
		t.Fatalf("disposition=%v", st.CompletionDisposition)
	}
}
