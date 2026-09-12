package cycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/engine"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func handoffProject(t *testing.T) (*cycle.Service, int64) {
	svc, cycleID, _ := handoffProjectWithDir(t)
	return svc, cycleID
}

func handoffProjectWithDir(t *testing.T) (*cycle.Service, int64, string) {
	t.Helper()
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.NewCycle("", "")
	if err != nil {
		t.Fatal(err)
	}
	return svc, res.Cycle.ID, dir
}

func seedImplementationPipeline(t *testing.T, s *store.Store, cycleID int64) {
	t.Helper()
	names := []string{"research", "planning", "implementation", "qa", "judge"}
	for i, name := range names {
		st, err := s.GetStage(cycleID, name)
		if err != nil {
			st = store.Stage{
				CycleID: cycleID, Name: name, Status: store.StageWaiting,
				MaxIterations: 2, SortOrder: i,
			}
			if err := s.CreateStages([]store.Stage{st}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if name == "research" || name == "planning" {
			st.Status = store.StageCompleted
		} else {
			st.Status = store.StageWaiting
		}
		if err := s.UpdateStage(st); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCloseStageFailedWithFindingsDelegatesToEngine(t *testing.T) {
	svc, cycleID := handoffProject(t)
	defer svc.Close()
	seedImplementationPipeline(t, svc.Store, cycleID)
	if err := svc.Engine.StartStage(cycleID, "qa"); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
		"status":"failed",
		"summary":"svc handoff",
		"failures":[{
			"owner":"generic_agent",
			"file":"internal/cycle/handoff.go",
			"issue":"facade test",
			"acceptance_criteria":"service delegates atomic close"
		}]
	}`)
	out, err := svc.CloseStageFailedWithFindings("qa", raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.FindingIDs) != 1 {
		t.Fatalf("finding_ids=%v", out.FindingIDs)
	}
}

func TestCloseImplementationWhenAssignmentEmpty(t *testing.T) {
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

	changeDir := filepath.Join(dir, "openspec", "changes", "c15-handoff")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := "- [x] [task-done] [agent:generic_agent] Done\n"
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("c15-handoff"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Engine.StartStage(cycleID, "implementation"); err != nil {
		t.Fatal(err)
	}
	empty, err := svc.ImplementationWorkloadEmpty()
	if err != nil || !empty {
		t.Fatalf("workload empty=%v err=%v", empty, err)
	}
	if err := svc.CloseImplementationWhenAssignmentEmpty("all assignment items complete"); err != nil {
		t.Fatal(err)
	}
	impl, err := svc.Store.GetStage(cycleID, "implementation")
	if err != nil || impl.Status != store.StageCompleted {
		t.Fatalf("implementation=%+v err=%v", impl, err)
	}
	qa, err := svc.Store.GetStage(cycleID, "qa")
	if err != nil || qa.Status != store.StageWaiting {
		t.Fatalf("qa should advance to waiting: %+v", qa)
	}
}

func TestCompleteCycleWithDeferredTodos(t *testing.T) {
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

	changeDir := filepath.Join(dir, "openspec", "changes", "c15-defer")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("- [x] [task-a] [agent:generic_agent] Done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("c15-defer"); err != nil {
		t.Fatal(err)
	}
	qa, _ := svc.Store.GetStage(cycleID, "qa")
	qa.Status = store.StageEscalated
	if err := svc.Store.UpdateStage(qa); err != nil {
		t.Fatal(err)
	}

	details := engine.DeferredCompletionDetails{
		TodoIDs: []string{"find-qa-1"},
		Summary: "deferred all blockers",
	}
	if err := svc.CompleteCycleWithDeferredTodos(details); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetCycle(cycleID)
	if err != nil || c.Status != store.CycleStatusCompleted {
		t.Fatalf("cycle=%+v err=%v", c, err)
	}
	disp, json, err := svc.Store.GetCycleCompletionDisposition(cycleID)
	if err != nil || disp != store.CompletionDispositionDeferredTodos || !strings.Contains(json, "find-qa-1") {
		t.Fatalf("disposition=%q json=%q err=%v", disp, json, err)
	}
	judge, err := svc.Store.GetStage(cycleID, "judge")
	if err != nil || judge.Status != store.StageSkipped {
		t.Fatalf("judge skipped=%+v", judge)
	}
}

func TestAdoptAndReleaseTodoHooks(t *testing.T) {
	svc, _ := handoffProject(t)
	defer svc.Close()
	id, err := svc.Store.CreateLegacyTodo(store.CreateLegacyTodoParams{Summary: "hook test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AdoptPendingTodoForResearch(id, "research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ReleaseAdoptedTodosForCycle("cancelled"); err != nil {
		t.Fatal(err)
	}
	todo, err := svc.Store.GetTodo(id)
	if err != nil || todo.Status != store.TodoStatusPending {
		t.Fatalf("todo=%+v", todo)
	}
}

func TestResearchAdoptionReconcilesProjection(t *testing.T) {
	svc, _ := handoffProject(t)
	defer svc.Close()
	id, err := svc.Store.CreateLegacyTodo(store.CreateLegacyTodoParams{Summary: "research adopt projection"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AdoptPendingTodoForResearch(id, "research"); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("adopt-c%d-%s", c.Number, id)
	op, err := svc.Store.GetTodoProjectionOpByKey(key)
	if err != nil {
		t.Fatalf("projection op missing: %v", err)
	}
	if op.Status != store.ProjectionStatusVerified {
		t.Fatalf("projection status=%q want verified", op.Status)
	}
}

func TestCancelReleasesAdoptedTodos(t *testing.T) {
	svc, _ := handoffProject(t)
	defer svc.Close()
	id, err := svc.Store.CreateLegacyTodo(store.CreateLegacyTodoParams{Summary: "cancel release"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AdoptPendingTodoForResearch(id, "research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Cancel("abort with adopted todo"); err != nil {
		t.Fatal(err)
	}
	todo, err := svc.Store.GetTodo(id)
	if err != nil || todo.Status != store.TodoStatusPending {
		t.Fatalf("todo after cancel=%+v err=%v", todo, err)
	}
}

func TestRejectReleasesAdoptedTodos(t *testing.T) {
	svc, _, dir := handoffProjectWithDir(t)
	defer svc.Close()
	id, err := svc.Store.CreateLegacyTodo(store.CreateLegacyTodoParams{Summary: "reject release"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AdoptPendingTodoForResearch(id, "research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Reject("rollback with adopted todo"); err != nil {
		t.Fatal(err)
	}
	todo, err := svc.Store.GetTodo(id)
	if err != nil || todo.Status != store.TodoStatusPending {
		t.Fatalf("todo after reject=%+v err=%v", todo, err)
	}
	adoptions, err := svc.Store.ListTodoAdoptions(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(adoptions) < 2 {
		t.Fatalf("adoption history=%+v want adopted+released", adoptions)
	}
	if adoptions[0].Status != store.AdoptionStatusAdopted || adoptions[1].Status != store.AdoptionStatusReleased {
		t.Fatalf("adoption history statuses=%+v", adoptions)
	}
	key := fmt.Sprintf("release-c%d-%s", c.Number, id)
	op, err := svc.Store.GetTodoProjectionOpByKey(key)
	if err != nil {
		t.Fatalf("release projection op missing: %v", err)
	}
	if op.Status != store.ProjectionStatusVerified {
		t.Fatalf("release projection status=%q want verified", op.Status)
	}
	body, err := os.ReadFile(filepath.Join(dir, "context", "current-state.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantLine := fmt.Sprintf("`%s` · %s ·", id, store.TodoStatusPending)
	if !strings.Contains(string(body), wantLine) {
		t.Fatalf("current-state.md missing pending projection for %s:\n%s", id, body)
	}
}

func TestFinishResolvesAdoptedTodos(t *testing.T) {
	svc, _ := handoffProject(t)
	defer svc.Close()
	id, err := svc.Store.CreateLegacyTodo(store.CreateLegacyTodoParams{Summary: "finish resolve"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AdoptPendingTodoForResearch(id, "research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Finish(""); err != nil {
		t.Fatal(err)
	}
	todo, err := svc.Store.GetTodo(id)
	if err != nil || todo.Status != store.TodoStatusResolved {
		t.Fatalf("todo after finish=%+v err=%v", todo, err)
	}
}
