package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestAddTodoGateRequiresEscalated(t *testing.T) {
	svc := newEscalatedFindingsService(t, "", false)
	m := EnterConversationForTest(NewTestModel(svc))
	next, _ := m.beginHeroAddTodo(nil)
	if next.convError != addTodoGateMessage {
		t.Fatalf("convError=%q want gate", next.convError)
	}
}

func TestAddTodoChecklistAndPartialDefer(t *testing.T) {
	svc := newEscalatedFindingsService(t, "", true)
	m := EnterConversationForTest(NewTestModel(svc))
	next, _ := m.beginHeroAddTodo(nil)
	if next.todoPhase != todoPhaseAddSelect {
		t.Fatalf("phase=%v", next.todoPhase)
	}
	next, _ = HandleTestKey(next, "down")
	next, _ = HandleTestKey(next, " ")
	next, _ = HandleTestKey(next, "enter")
	if next.todoPhase != todoPhaseAddReviewPartial {
		t.Fatalf("phase=%v want partial review", next.todoPhase)
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "Resolve in this cycle") {
		t.Fatalf("missing partial review copy: %q", view)
	}
}

func TestAddTodoDeferAllRequiresTypedDEFER(t *testing.T) {
	svc := newEscalatedFindingsService(t, "", true)
	m := EnterConversationForTest(NewTestModel(svc))
	next, _ := m.beginHeroAddTodo(nil)
	next, _ = HandleTestKey(next, "a")
	next, _ = HandleTestKey(next, "enter")
	if next.todoPhase != todoPhaseAddReviewDeferAll {
		t.Fatalf("phase=%v", next.todoPhase)
	}
	next, _ = HandleTestKey(next, "enter")
	if next.todoPhase != todoPhaseAddTypeDefer {
		t.Fatalf("phase=%v want type defer", next.todoPhase)
	}
	next = SetConversationInput(next, "DEFER")
	next, cmd := HandleTestKey(next, "enter")
	if cmd == nil {
		t.Fatal("expected add-todo async command")
	}
}

func TestCompleteTodoRequiresNote(t *testing.T) {
	svc := newPendingTodoService(t, "")
	m := EnterConversationForTest(NewTestModel(svc))
	next, _ := m.beginHeroCompleteTodo([]string{"find-qa-1"})
	if next.todoPhase != todoPhaseCompleteNote {
		t.Fatalf("phase=%v", next.todoPhase)
	}
	next, _ = HandleTestKey(next, "enter")
	if !strings.Contains(next.convError, "required") {
		t.Fatalf("convError=%q", next.convError)
	}
}

func TestCompleteTodoRejectsSecretNote(t *testing.T) {
	svc := newPendingTodoService(t, "")
	m := EnterConversationForTest(NewTestModel(svc))
	next, _ := m.beginHeroCompleteTodo([]string{"find-qa-1"})
	next = SetConversationInput(next, "fixed token 123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1")
	next, _ = HandleTestKey(next, "enter")
	if !strings.Contains(next.convError, "secret") && !strings.Contains(next.convError, "forbidden") {
		t.Fatalf("convError=%q", next.convError)
	}
}

func TestFinishWarningBlocksUntilFINISH(t *testing.T) {
	svc := newEscalatedFindingsService(t, "", true)
	m := EnterConversationForTest(NewTestModel(svc))
	writeHeroFinishCommand(t, svc.ProjectDir)
	next, _ := m.beginHeroFinish()
	if next.todoPhase != todoPhaseFinishTypeFinish {
		t.Fatalf("phase=%v want finish warning", next.todoPhase)
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "FINISH") || !strings.Contains(view, "/hero-add-todo") {
		t.Fatalf("missing finish warning copy: %q", view)
	}
}

func setupTodoControlProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: C15 Todo Control
objective: Test
stages:
  research:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: false
  planning:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: false
  implementation:
    enabled: true
    max_iterations: 4
    timeout_minutes: 30
    require_human_approval: false
  qa:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755)
	ctxDir := filepath.Join(dir, "context")
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "current-state.md"), []byte("## Pending Features\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func seedTodoControlStages(t *testing.T, s *store.Store, cycleID int64, escalated bool) {
	t.Helper()
	names := []string{"research", "planning", "implementation", "qa"}
	for i, name := range names {
		st, err := s.GetStage(cycleID, name)
		if err != nil {
			st = store.Stage{CycleID: cycleID, Name: name, Status: store.StageWaiting, MaxIterations: 2, SortOrder: i}
			if err := s.CreateStages([]store.Stage{st}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		switch name {
		case "research", "planning":
			st.Status = store.StageCompleted
		case "implementation":
			if escalated {
				st.Status = store.StageEscalated
			} else {
				st.Status = store.StageWaiting
			}
		default:
			st.Status = store.StageWaiting
		}
		if err := s.UpdateStage(st); err != nil {
			t.Fatal(err)
		}
	}
}

func newEscalatedFindingsService(t *testing.T, dir string, escalated bool) *cycle.Service {
	t.Helper()
	if dir == "" {
		dir = setupTodoControlProject(t)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	seedTodoControlStages(t, svc.Store, c.ID, escalated)
	for i, issue := range []string{"issue one", "issue two"} {
		if _, err := svc.Store.PersistFinding(store.FindingInput{
			CycleID: c.ID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
			File: "internal/tui/todo_control_test.go", Requirement: "PRD-C15",
			Issue: issue, AcceptanceCriteria: "fix " + issue,
		}); err != nil {
			t.Fatalf("persist %d: %v", i, err)
		}
	}
	return svc
}

func newPendingTodoService(t *testing.T, dir string) *cycle.Service {
	t.Helper()
	if dir == "" {
		dir = setupTodoControlProject(t)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.PersistFinding(store.FindingInput{
		CycleID: c.ID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
		File: "internal/x.go", Requirement: "PRD", Issue: "manual", AcceptanceCriteria: "done",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.CreateFindingTodo(store.CreateFindingTodoParams{
		ID: "find-qa-1", OriginCycleID: c.ID, OriginSourceStage: "qa",
		Summary: "manual", AcceptanceCriteria: "done",
	}); err != nil {
		t.Fatal(err)
	}
	return svc
}

func writeHeroFinishCommand(t *testing.T, dir string) {
	t.Helper()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-finish.md"), []byte("# /hero-finish\n\nfinish"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCompleteTodoPreselectAllowsLegacyProse(t *testing.T) {
	dir := setupTodoControlProject(t)
	svc := newPendingTodoService(t, dir)
	msg := validateCompleteTodoPreselect(svc, []string{"Windows CLI support"})
	if msg != "" {
		t.Fatalf("legacy prose should be allowed through preselect, got %q", msg)
	}
}
