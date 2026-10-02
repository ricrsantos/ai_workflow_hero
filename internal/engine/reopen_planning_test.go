package engine

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func seedReopenCycle(t *testing.T, s *store.Store, implStatus string) int64 {
	t.Helper()
	id, err := s.CreateCycle(store.Cycle{
		Number: 1, Title: "T", Objective: "O", Status: store.CycleStatusActive,
		StartedAt: "2026-10-01T12:00:00Z", ConfigSnapshotJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]store.Stage{
		{CycleID: id, Name: "research", Status: store.StageCompleted, MaxIterations: 3, Iteration: 1, SortOrder: 0},
		{CycleID: id, Name: "planning", Status: store.StageCompleted, MaxIterations: 3, Iteration: 1, Summary: "SDD ready", CompletedAt: "2026-10-01T12:30:00Z", SortOrder: 1},
		{CycleID: id, Name: "implementation", Status: implStatus, MaxIterations: 4, Iteration: 2, StartedAt: "2026-10-01T13:00:00Z", SortOrder: 2},
		{CycleID: id, Name: "qa", Status: store.StageWaiting, MaxIterations: 2, SortOrder: 3},
		{CycleID: id, Name: "judge", Status: store.StageWaiting, MaxIterations: 3, SortOrder: 4},
		{CycleID: id, Name: "browser_ui_validation", Status: store.StageSkipped, MaxIterations: 2, SortOrder: 5},
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestReopenPlanningFromEscalatedImplementation(t *testing.T) {
	e, s := openTestEngine(t)
	id := seedReopenCycle(t, s, store.StageEscalated)
	reason := "Which key persists the opt-in? Recommend test_access.enabled."

	if err := e.ReopenPlanning(id, "implementation", reason); err != nil {
		t.Fatal(err)
	}

	planning, _ := s.GetStage(id, "planning")
	if planning.Status != store.StageWaiting || planning.Summary != reason || planning.Iteration != 1 || planning.CompletedAt != "" {
		t.Fatalf("planning=%+v", planning)
	}
	impl, _ := s.GetStage(id, "implementation")
	if impl.Status != store.StageWaiting || impl.Iteration != 2 || impl.StartedAt != "" {
		t.Fatalf("implementation=%+v", impl)
	}
	if skipped, _ := s.GetStage(id, "browser_ui_validation"); skipped.Status != store.StageSkipped {
		t.Fatalf("skipped stage changed: %+v", skipped)
	}
	if research, _ := s.GetStage(id, "research"); research.Status != store.StageCompleted {
		t.Fatalf("research changed: %+v", research)
	}
	events, _ := s.ListEvents(id, store.EventPlanningReopened, 5)
	if len(events) != 1 || !strings.Contains(events[0].PayloadJSON, `"from":"implementation"`) {
		t.Fatalf("events=%+v", events)
	}
	if err := e.StartStage(id, "planning"); err != nil {
		t.Fatalf("reopened planning must start: %v", err)
	}
}

func TestReopenPlanningRejectsInvalidSources(t *testing.T) {
	cases := []struct {
		name, from, implStatus, reason string
	}{
		{"qa is not a source", "qa", store.StageEscalated, "x"},
		{"empty reason", "implementation", store.StageEscalated, " "},
		{"implementation still waiting", "implementation", store.StageWaiting, "x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, s := openTestEngine(t)
			id := seedReopenCycle(t, s, c.implStatus)
			if err := e.ReopenPlanning(id, c.from, c.reason); err == nil {
				t.Fatal("expected an error")
			}
			if planning, _ := s.GetStage(id, "planning"); planning.Status != store.StageCompleted {
				t.Fatalf("planning mutated on error: %+v", planning)
			}
		})
	}
}
