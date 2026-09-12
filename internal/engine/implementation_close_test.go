package engine

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func seedPipelineForEmptyClose(t *testing.T, s *store.Store) int64 {
	t.Helper()
	id, err := s.CreateCycle(store.Cycle{
		Number: 1, Title: "T", Objective: "O", Status: store.CycleStatusActive,
		StartedAt: "2026-08-07T12:00:00Z", ConfigSnapshotJSON: `{"scope":{"native":true}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	stages := []store.Stage{
		{CycleID: id, Name: "research", Status: store.StageCompleted, MaxIterations: 2, SortOrder: 0},
		{CycleID: id, Name: "planning", Status: store.StageCompleted, MaxIterations: 2, SortOrder: 1},
		{CycleID: id, Name: "implementation", Status: store.StageWaiting, MaxIterations: 4, SortOrder: 2},
		{CycleID: id, Name: "qa", Status: store.StageWaiting, MaxIterations: 2, SortOrder: 3},
		{CycleID: id, Name: "judge", Status: store.StageWaiting, MaxIterations: 2, SortOrder: 4},
	}
	if err := s.CreateStages(stages); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCloseImplementationWhenAssignmentEmpty(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedPipelineForEmptyClose(t, s)
	if err := e.StartStage(id, "implementation"); err != nil {
		t.Fatal(err)
	}
	if err := e.CloseImplementationWhenAssignmentEmpty(id, "verified empty"); err != nil {
		t.Fatal(err)
	}
	impl, _ := s.GetStage(id, "implementation")
	if impl.Status != store.StageCompleted {
		t.Fatalf("implementation=%s", impl.Status)
	}
}

func TestCompleteCycleWithDeferredTodosDisposition(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedPipelineForEmptyClose(t, s)
	qa, _ := s.GetStage(id, "qa")
	qa.Status = store.StageEscalated
	if err := s.UpdateStage(qa); err != nil {
		t.Fatal(err)
	}
	err := e.CompleteCycleWithDeferredTodosDisposition(id, DeferredCompletionDetails{
		TodoIDs: []string{"find-qa-1"},
		Summary: "all deferred",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := s.GetCycle(id)
	if c.Status != store.CycleStatusCompleted {
		t.Fatalf("cycle status=%s", c.Status)
	}
	disp, json, err := s.GetCycleCompletionDisposition(id)
	if err != nil || disp != store.CompletionDispositionDeferredTodos || !strings.Contains(json, "find-qa-1") {
		t.Fatalf("disposition=%q json=%q err=%v", disp, json, err)
	}
	judge, _ := s.GetStage(id, "judge")
	if judge.Status != store.StageSkipped {
		t.Fatalf("judge=%s", judge.Status)
	}
}
