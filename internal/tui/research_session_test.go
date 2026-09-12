package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestResearchPendingTodoPromptSection_empty(t *testing.T) {
	got := researchPendingTodoPromptSection(researchTodoSnapshot{CycleNumber: 16})
	for _, kw := range []string{
		"→ Checking pending project ToDos",
		"✓ No pending project ToDos",
		"before grilling",
	} {
		if !strings.Contains(got, kw) {
			t.Fatalf("missing %q in %q", kw, got)
		}
	}
}

func TestResearchPendingTodoPromptSection_withCandidates(t *testing.T) {
	snap := researchTodoSnapshot{
		CycleNumber: 16,
		Objective:   "deterministic loop-back assignment",
		Pending: []store.Todo{
			{ID: "find-qa-1", Summary: "deterministic loop-back assignment"},
			{ID: "todo-4", Summary: "Windows CLI support"},
		},
	}
	got := researchPendingTodoPromptSection(snap)
	for _, kw := range []string{
		"hero adopt-todo",
		"Already implied by this cycle objective",
		"find-qa-1",
		"Other candidates",
		"todo-4",
		"Adopted by C16",
		"do NOT finalize requirements",
	} {
		if !strings.Contains(got, kw) {
			t.Fatalf("missing %q in %q", kw, got)
		}
	}
}

func TestStartDiscoverResearchSession_injectsPendingTodos(t *testing.T) {
	dir := t.TempDir()
	setupDiscoverRuntimeFiles(t, dir)
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeDiscoverAgentYAML(t, dir)
	id, err := svc.Store.CreateLegacyTodo(store.CreateLegacyTodoParams{Summary: "Research adoption probe"})
	if err != nil {
		t.Fatal(err)
	}
	h := &streamingHarness{
		deltas:     []string{"ok"},
		sessionIDs: []string{"orch-sess", "disc-sess"},
	}
	svc.Harness = h

	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next = drainConversationStream(t, next, cmd)

	if !strings.Contains(h.lastPrompt, "Research pending ToDo adoption") {
		t.Fatalf("prompt missing adoption section: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, id) {
		t.Fatalf("prompt missing pending todo %q: %q", id, h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, "hero adopt-todo") {
		t.Fatalf("prompt missing adopt CLI: %q", h.lastPrompt)
	}
	_ = next
}
