package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func openTodoTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "hero.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return s
}

func createTodoTestCycle(t *testing.T, s *Store, number int) int64 {
	t.Helper()
	id, err := s.CreateCycle(Cycle{
		Number: number, Title: "t", Objective: "o", Status: CycleStatusActive,
		StartedAt: nowRFC3339(), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	return id
}

func TestTodoLifecyclePendingAdoptedResolved(t *testing.T) {
	s := openTodoTestStore(t)
	defer s.Close()

	originCycle := createTodoTestCycle(t, s, 1)
	adoptCycle := createTodoTestCycle(t, s, 2)

	const todoID = "find-qa-1"
	if err := s.CreateFindingTodo(CreateFindingTodoParams{
		ID: todoID, OriginCycleID: originCycle, OriginSourceStage: "qa",
		Summary: "migration loses event rows", AcceptanceCriteria: "events survive upgrade",
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.AdoptTodo(todoID, adoptCycle, "research pick"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTodo(todoID)
	if err != nil || got.Status != TodoStatusAdopted || got.AdoptedCycleID != adoptCycle {
		t.Fatalf("after adopt: %+v err=%v", got, err)
	}

	if err := s.ResolveAdoptedTodosForCycle(adoptCycle); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetTodo(todoID)
	if err != nil || got.Status != TodoStatusResolved || got.ResolvedAt == "" {
		t.Fatalf("after resolve: %+v err=%v", got, err)
	}
	adoptions, err := s.ListTodoAdoptions(todoID)
	if err != nil || len(adoptions) != 2 {
		t.Fatalf("adoptions: %+v err=%v", adoptions, err)
	}
	if adoptions[0].Status != AdoptionStatusAdopted || adoptions[1].Status != AdoptionStatusResolved {
		t.Fatalf("adoption statuses: %+v", adoptions)
	}
}

func TestTodoReleaseRetainsAdoptionHistory(t *testing.T) {
	s := openTodoTestStore(t)
	defer s.Close()

	originCycle := createTodoTestCycle(t, s, 1)
	adoptCycle := createTodoTestCycle(t, s, 2)
	const todoID = "find-judge-1"

	if err := s.CreateFindingTodo(CreateFindingTodoParams{
		ID: todoID, OriginCycleID: originCycle, OriginSourceStage: "judge",
		Summary: "gap in SDD", AcceptanceCriteria: "fix assignment",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.AdoptTodo(todoID, adoptCycle, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseTodo(todoID, adoptCycle, "cycle cancelled"); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetTodo(todoID)
	if err != nil || got.Status != TodoStatusPending || got.AdoptedCycleID != 0 {
		t.Fatalf("after release: %+v err=%v", got, err)
	}
	adoptions, err := s.ListTodoAdoptions(todoID)
	if err != nil || len(adoptions) != 2 {
		t.Fatalf("adoptions: %+v err=%v", adoptions, err)
	}
	if adoptions[0].Status != AdoptionStatusAdopted || adoptions[1].Status != AdoptionStatusReleased {
		t.Fatalf("adoption statuses: %+v", adoptions)
	}
}

func TestProjectionOpIdempotencyNoDuplicateTodosOrConflictingNotes(t *testing.T) {
	s := openTodoTestStore(t)
	defer s.Close()

	cycleID := createTodoTestCycle(t, s, 1)
	const todoID = "find-qa-1"
	const deferKey = "defer-find-qa-1-c15"
	_, err := s.PersistFinding(FindingInput{
		CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric,
		File: "internal/store/todos.go", Requirement: "PRD",
		Issue: "pending projection", AcceptanceCriteria: "sqlite authoritative",
	})
	if err != nil {
		t.Fatal(err)
	}
	p := CreateFindingTodoParams{
		ID: todoID, OriginCycleID: cycleID, OriginSourceStage: "qa",
		Summary: "pending projection", AcceptanceCriteria: "sqlite authoritative",
	}

	for i := 0; i < 2; i++ {
		if err := s.DeferFindingTodoIdempotent(deferKey, cycleID, p); err != nil {
			t.Fatalf("defer attempt %d: %v", i, err)
		}
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE id = ?`, todoID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("todo row count = %d err=%v", n, err)
	}
	op, err := s.GetTodoProjectionOpByKey(deferKey)
	if err != nil || op.OpKind != ProjectionOpDefer {
		t.Fatalf("defer op: %+v err=%v", op, err)
	}

	const completeKey = "complete-find-qa-1-c15"
	note := "fixed outside cycle"
	if err := s.CompleteTodoIdempotent(CompleteTodoIdempotentParams{
		TodoID: todoID, ResolutionNote: note, IdempotencyKey: completeKey, CycleID: cycleID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteTodoIdempotent(CompleteTodoIdempotentParams{
		TodoID: todoID, ResolutionNote: note, IdempotencyKey: completeKey, CycleID: cycleID,
	}); err != nil {
		t.Fatalf("idempotent complete retry: %v", err)
	}
	if err := s.CompleteTodoIdempotent(CompleteTodoIdempotentParams{
		TodoID: todoID, ResolutionNote: "different note", IdempotencyKey: completeKey, CycleID: cycleID,
	}); !errors.Is(err, ErrTodoResolutionNoteConflict) {
		t.Fatalf("conflicting note err = %v, want ErrTodoResolutionNoteConflict", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE id = ?`, todoID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("todo row count after complete = %d err=%v", n, err)
	}
	got, err := s.GetTodo(todoID)
	if err != nil || got.ResolutionNote != note {
		t.Fatalf("resolution note = %q err=%v", got.ResolutionNote, err)
	}
}

func TestAllocateLegacyTodoID(t *testing.T) {
	s := openTodoTestStore(t)
	defer s.Close()

	id1, err := s.CreateLegacyTodo(CreateLegacyTodoParams{Summary: "windows cli"})
	if err != nil || id1 != "todo-1" {
		t.Fatalf("first legacy: %q err=%v", id1, err)
	}
	id2, err := s.CreateLegacyTodo(CreateLegacyTodoParams{Summary: "other"})
	if err != nil || id2 != "todo-2" {
		t.Fatalf("second legacy: %q err=%v", id2, err)
	}
}

func TestSetCycleCompletionDisposition(t *testing.T) {
	s := openTodoTestStore(t)
	defer s.Close()

	cycleID := createTodoTestCycle(t, s, 1)
	json := `{"todo_ids":["find-qa-1"],"count":1}`
	if err := s.SetCycleCompletionDisposition(cycleID, CompletionDispositionDeferredTodos, json); err != nil {
		t.Fatal(err)
	}
	d, j, err := s.GetCycleCompletionDisposition(cycleID)
	if err != nil || d != CompletionDispositionDeferredTodos || j != json {
		t.Fatalf("disposition = (%q, %q) err=%v", d, j, err)
	}
}
