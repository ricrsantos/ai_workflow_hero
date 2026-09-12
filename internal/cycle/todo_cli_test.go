package cycle_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestAddFindingTodosRequiresEscalated(t *testing.T) {
	svc, _ := handoffProject(t)
	defer svc.Close()
	_, err := svc.AddFindingTodos([]string{"find-qa-1"}, "")
	if err == nil {
		t.Fatal("expected error without escalated stage")
	}
}

func TestAddFindingTodosPartialDeferral(t *testing.T) {
	svc, cycleID := handoffProject(t)
	defer svc.Close()
	seedImplementationPipeline(t, svc.Store, cycleID)

	qa, err := svc.Store.GetStage(cycleID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	qa.Status = store.StageEscalated
	if err := svc.Store.UpdateStage(qa); err != nil {
		t.Fatal(err)
	}

	for i, issue := range []string{"issue one", "issue two"} {
		_, err := svc.Store.PersistFinding(store.FindingInput{
			CycleID: cycleID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
			File: "internal/cycle/todo_cli.go", Requirement: "PRD-C15",
			Issue: issue, AcceptanceCriteria: "fix " + issue,
		})
		if err != nil {
			t.Fatalf("persist %d: %v", i, err)
		}
	}
	findings, err := svc.Store.ListActionableFindings(cycleID, "")
	if err != nil || len(findings) != 2 {
		t.Fatalf("actionable=%d err=%v", len(findings), err)
	}

	res, err := svc.AddFindingTodos([]string{findings[0].ID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.CycleCompleted {
		t.Fatal("expected partial deferral")
	}
	if !res.PartialDeferral || len(res.TodoIDs) != 1 {
		t.Fatalf("result=%+v", res)
	}
	actionable, err := svc.Store.ListActionableFindings(cycleID, "")
	if err != nil || len(actionable) != 1 {
		t.Fatalf("actionable=%d err=%v", len(actionable), err)
	}
}

func TestCompleteManualTodosRequiresNote(t *testing.T) {
	svc, _ := handoffProject(t)
	defer svc.Close()
	if err := svc.CompleteManualTodos([]string{"find-qa-1"}, "  ", ""); err == nil {
		t.Fatal("expected error for empty note")
	}
}

func TestCompleteManualTodosIdempotent(t *testing.T) {
	svc, cycleID := handoffProject(t)
	defer svc.Close()
	_, err := svc.Store.PersistFinding(store.FindingInput{
		CycleID: cycleID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
		File: "internal/x.go", Requirement: "PRD", Issue: "manual", AcceptanceCriteria: "done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.CreateFindingTodo(store.CreateFindingTodoParams{
		ID: "find-qa-1", OriginCycleID: cycleID, OriginSourceStage: "qa",
		Summary: "manual", AcceptanceCriteria: "done",
	}); err != nil {
		t.Fatal(err)
	}
	note := "fixed outside hero"
	for i := 0; i < 2; i++ {
		if err := svc.CompleteManualTodos([]string{"find-qa-1"}, note, "complete-test-"); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
}

func TestCompleteManualTodosWithoutActiveCycle(t *testing.T) {
	svc, _ := handoffProject(t)
	defer svc.Close()
	id, err := svc.Store.CreateLegacyTodo(store.CreateLegacyTodoParams{Summary: "nocycle complete"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Cancel("clear active cycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.GetActiveCycle(); !errors.Is(err, store.ErrNoActiveCycle) {
		t.Fatalf("expected no active cycle, err=%v", err)
	}
	if err := svc.CompleteManualTodos([]string{id}, "resolved between cycles", "nocycle-"); err != nil {
		t.Fatalf("complete without cycle: %v", err)
	}
	todo, err := svc.Store.GetTodo(id)
	if err != nil || todo.Status != store.TodoStatusResolved {
		t.Fatalf("todo=%+v err=%v", todo, err)
	}
}

func TestCompleteManualTodosPromotesLegacyAndProjects(t *testing.T) {
	svc, _ := handoffProject(t)
	defer svc.Close()
	dir := svc.ProjectDir
	ctxDir := filepath.Join(dir, "context")
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "## Pending Features\n\n- Windows CLI support\n- keep unmatched prose\n"
	if err := os.WriteFile(filepath.Join(ctxDir, "current-state.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteManualTodos([]string{"Windows CLI support"}, "shipped in helper", "legacy-"); err != nil {
		t.Fatalf("legacy complete: %v", err)
	}
	pending, err := svc.Store.ListPendingTodos()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pending {
		if strings.Contains(p.Summary, "Windows CLI support") {
			t.Fatalf("legacy line still pending: %+v", p)
		}
	}
	raw, err := os.ReadFile(filepath.Join(ctxDir, "current-state.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "- Windows CLI support\n") {
		t.Fatalf("projection still has raw legacy line:\n%s", text)
	}
	if !strings.Contains(text, "keep unmatched prose") {
		t.Fatalf("unmatched prose was lost:\n%s", text)
	}
}
