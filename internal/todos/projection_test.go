package todos_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/todos"
)

func writeCurrentState(t *testing.T, dir string, body string) {
	t.Helper()
	ctx := filepath.Join(dir, "context")
	if err := os.MkdirAll(ctx, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctx, "current-state.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openStore(t *testing.T) (*store.Store, func()) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "hero.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return s, func() { _ = s.Close() }
}

func createCycle(t *testing.T, s *store.Store, number int) int64 {
	t.Helper()
	id, err := s.CreateCycle(store.Cycle{
		Number: number, Title: "t", Objective: "o", Status: store.CycleStatusActive,
		StartedAt: time.Now().UTC().Format(time.RFC3339), ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	return id
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestBuildCurrentStateCandidate_preservesUnmatchedProse(t *testing.T) {
	dir := t.TempDir()
	body := "## Pending Features\n\n_(note)_\n\n- legacy prose item stays\n- `find-old` · pending · C1/QA · stale structured line\n\n## Next Steps\n\n- ignore\n"
	writeCurrentState(t, dir, body)
	s, cleanup := openStore(t)
	defer cleanup()
	cycleID := createCycle(t, s, 15)
	if err := s.CreateFindingTodo(store.CreateFindingTodoParams{
		ID: "find-qa-2", OriginCycleID: cycleID, OriginSourceStage: "qa",
		Summary: "migration loses event rows",
	}); err != nil {
		t.Fatal(err)
	}

	out, err := todos.BuildCurrentStateCandidate(dir, s)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "legacy prose item stays") {
		t.Fatalf("missing unmatched prose:\n%s", text)
	}
	if strings.Contains(text, "find-old") {
		t.Fatalf("stale structured line must be replaced:\n%s", text)
	}
	wantLine := todos.FormatProjectedLine("find-qa-2", store.TodoStatusPending, "C15/QA", "migration loses event rows")
	if !strings.Contains(text, wantLine) {
		t.Fatalf("missing projected line %q in:\n%s", wantLine, text)
	}
}

func TestReconcileProjection_idempotentNoDuplicateLines(t *testing.T) {
	dir := t.TempDir()
	writeCurrentState(t, dir, "## Pending Features\n\n- keep me\n")
	s, cleanup := openStore(t)
	defer cleanup()
	cycleID := createCycle(t, s, 15)
	res, err := s.PersistFinding(store.FindingInput{
		CycleID: cycleID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
		File: "internal/todos/projection.go", Requirement: "PRD",
		Issue: "pending projection", AcceptanceCriteria: "sqlite authoritative",
	})
	if err != nil {
		t.Fatal(err)
	}
	todoID := res.Finding.ID
	if err := s.DeferFindingTodoIdempotent("defer-find-qa-2-c15", cycleID, store.CreateFindingTodoParams{
		ID: todoID, OriginCycleID: cycleID, OriginSourceStage: "qa", Summary: "pending projection",
	}); err != nil {
		t.Fatal(err)
	}
	const key = "defer-find-qa-2-c15"

	for i := 0; i < 3; i++ {
		if err := todos.ReconcileProjection(dir, s, key); err != nil {
			t.Fatalf("reconcile attempt %d: %v", i, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, todos.CurrentStateRelPath))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, todoID) != 1 {
		t.Fatalf("expected exactly one projected id line, got %d in:\n%s", strings.Count(text, todoID), text)
	}
	op, err := s.GetTodoProjectionOpByKey(key)
	if err != nil || op.Status != store.ProjectionStatusVerified {
		t.Fatalf("op status = %+v err=%v", op, err)
	}
}

func TestReconcileProjection_resumesFromCandidateReady(t *testing.T) {
	dir := t.TempDir()
	writeCurrentState(t, dir, "## Pending Features\n\n")
	s, cleanup := openStore(t)
	defer cleanup()
	cycleID := createCycle(t, s, 16)
	const todoID = "find-judge-1"
	if err := s.CreateFindingTodo(store.CreateFindingTodoParams{
		ID: todoID, OriginCycleID: cycleID, OriginSourceStage: "judge", Summary: "gap",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteTodoIdempotent(store.CompleteTodoIdempotentParams{
		TodoID: todoID, ResolutionNote: "fixed in test", IdempotencyKey: "complete-find-judge-1-seed", CycleID: cycleID,
	}); err != nil {
		t.Fatal(err)
	}
	const key = "complete-find-judge-1"
	if _, _, err := s.UpsertTodoProjectionOp(store.UpsertProjectionOpParams{
		CycleID: cycleID, OpKind: store.ProjectionOpComplete, IdempotencyKey: key,
		Status: store.ProjectionStatusIntentPersisted, TodoIDsJSON: `["find-judge-1"]`,
	}); err != nil {
		t.Fatal(err)
	}
	candidate, err := todos.BuildCurrentStateCandidate(dir, s)
	if err != nil {
		t.Fatal(err)
	}
	candidatePath := todos.CandidatePath(dir, key)
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidatePath, candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateTodoProjectionOpStatus(key, store.ProjectionStatusCandidateReady, sha256Hex(candidate)); err != nil {
		t.Fatal(err)
	}
	if err := todos.ReconcileProjection(dir, s, key); err != nil {
		t.Fatal(err)
	}
	op, err := s.GetTodoProjectionOpByKey(key)
	if err != nil || op.Status != store.ProjectionStatusVerified {
		t.Fatalf("op = %+v err=%v", op, err)
	}
}

func TestPromoteSelectedLegacyLine_onlySelected(t *testing.T) {
	dir := t.TempDir()
	writeCurrentState(t, dir, "## Pending Features\n\n- Windows CLI support\n- unrelated prose\n")
	s, cleanup := openStore(t)
	defer cleanup()

	id, err := todos.PromoteSelectedLegacyLine(s, dir, "Windows CLI support")
	if err != nil || id != "todo-1" {
		t.Fatalf("promote selected: id=%q err=%v", id, err)
	}
	projected, err := s.ListTodosForProjection()
	if err != nil || len(projected) != 1 {
		t.Fatalf("projected todos = %d err=%v", len(projected), err)
	}
	if _, err := todos.PromoteSelectedLegacyLine(s, dir, "unrelated prose"); err != nil {
		t.Fatalf("second promote: %v", err)
	}
	projected, err = s.ListTodosForProjection()
	if err != nil || len(projected) != 2 {
		t.Fatalf("after second promote count = %d err=%v", len(projected), err)
	}
	items, err := todos.ReadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("pending items still prose-only: %#v", items)
	}
}

func TestParseStructuredProjectedLine(t *testing.T) {
	line := todos.FormatProjectedLine("find-qa-2", "pending", "C15/QA", "summary text")
	got, ok := todos.ParseStructuredProjectedLine(line)
	if !ok || got.ID != "find-qa-2" || got.Status != "pending" || got.Origin != "C15/QA" || got.Summary != "summary text" {
		t.Fatalf("parse = %+v ok=%v", got, ok)
	}
}
