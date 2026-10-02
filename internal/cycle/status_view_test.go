package cycle_test

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestStatusBudgetUsesActiveElapsedAndPreservesHumanPause(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Error("close status verification service")
		}
	})
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	svc.Engine.Now = func() time.Time { return now }
	if err := svc.Engine.StartStage(c.ID, "research"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Second)
	view, err := svc.Status()
	if err != nil || len(view.StageBudgets) != 1 {
		t.Fatalf("budget projection missing: %v", err)
	}
	row := view.StageBudgets[0]
	if row.State != "active" || row.ConsumedMS != 3000 || row.RemainingMS <= 0 {
		t.Fatalf("active elapsed incorrectly projected: %+v", row)
	}
	budget, err := svc.Engine.StageBudget(c.ID, "research")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Engine.PauseStageBudget(c.ID, "research", budget.Generation, "human_question"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(24 * time.Hour)
	view, err = svc.Status()
	if err != nil || view.StageBudgets[0].ConsumedMS != row.ConsumedMS || view.StageBudgets[0].RemainingMS != row.RemainingMS || view.StageBudgets[0].PauseReason != "human_question" {
		t.Fatalf("paused budget counted offline time: %+v, %v", view.StageBudgets, err)
	}
}

func TestStatusJSONNoCycleBackwardCompatible(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Error("close status verification service")
		}
	})

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
	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Error("close status verification service")
		}
	})
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
	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Error("close status verification service")
		}
	})
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
			"acceptance_criteria":"loop-back row includes findingIds","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}
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
	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Error("close status verification service")
		}
	})
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

func TestBlockedStateStatusProjectionAndContinueAction(t *testing.T) {
	svc, cycleID := handoffProject(t)
	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Error("close status verification service")
		}
	})
	if err := svc.Store.CreateStages([]store.Stage{{
		CycleID: cycleID, Name: "browser_ui_validation", Status: store.StageBlocked, MaxIterations: 2, SortOrder: 6,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.InTx(func(tx *sql.Tx) error {
		return svc.Store.ReplaceActiveStageBlockersTx(tx, cycleID, "browser_ui_validation", []store.StageBlocker{{
			ID: "browser-capability-missing", Reason: "tool_unavailable",
			AffectedCoverageIDs: []string{"screen-dashboard"}, AffectedProfileIDs: []string{"operator"},
			DiagnosticUncertainty: "SENTINEL_STATUS_SECRET",
			NextAction:            "SENTINEL_STATUS_SECRET",
		}}, "2026-08-07T12:00:00Z")
	}); err != nil {
		t.Fatal(err)
	}
	view, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.BlockedStages) != 1 || view.BlockedStages[0].Name != "Browser Ui Validation" || len(view.BlockedStages[0].Blockers) != 1 {
		t.Fatalf("blocked status=%+v", view.BlockedStages)
	}
	canContinue := false
	for _, action := range view.AvailableActions {
		canContinue = canContinue || action == "hero-continue"
	}
	if !canContinue {
		t.Fatalf("available actions=%v, want hero-continue", view.AvailableActions)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "tool_unavailable") || !strings.Contains(string(raw), "screen-dashboard") || !strings.Contains(string(raw), "Configure the selected browser method") {
		t.Fatalf("status omitted corrective blocker details: %s", raw)
	}
	if strings.Contains(string(raw), "SENTINEL_STATUS_SECRET") {
		t.Fatalf("status projection exposed report free text: %s", raw)
	}
}

func TestNoAutoAdvanceWhileBlocked(t *testing.T) {
	svc, cycleID := handoffProject(t)
	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Error("close blocked-cycle service")
		}
	})
	stages, err := svc.Store.ListStages(cycleID)
	if err != nil {
		t.Fatal(err)
	}
	known := make(map[string]bool, len(stages))
	for _, stage := range stages {
		known[stage.Name] = true
	}
	var missing []store.Stage
	for _, stage := range []store.Stage{
		{CycleID: cycleID, Name: "browser_ui_validation", Status: store.StageBlocked, MaxIterations: 2, SortOrder: 6},
		{CycleID: cycleID, Name: "qa_end_to_end", Status: store.StageWaiting, MaxIterations: 2, SortOrder: 7},
	} {
		if !known[stage.Name] {
			missing = append(missing, stage)
		}
	}
	if len(missing) > 0 {
		if err := svc.Store.CreateStages(missing); err != nil {
			t.Fatal(err)
		}
		stages, err = svc.Store.ListStages(cycleID)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := range stages {
		switch stages[i].Name {
		case "browser_ui_validation":
			stages[i].Status = store.StageBlocked
		case "qa_end_to_end":
			stages[i].Status = store.StageWaiting
		default:
			stages[i].Status = store.StageCompleted
		}
		if err := svc.Store.UpdateStage(stages[i]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.ActiveStage(); err == nil || !strings.Contains(err.Error(), "is blocked") {
		t.Fatalf("ActiveStage() error=%v, want blocked scheduler stop", err)
	}
	if _, err := svc.ActiveRunStage(); err == nil || !strings.Contains(err.Error(), "is blocked") {
		t.Fatalf("ActiveRunStage() error=%v, want blocked scheduler stop", err)
	}
}
