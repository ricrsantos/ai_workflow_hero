package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestRenderStatusFindingsBoard(t *testing.T) {
	m := NewTestModel(nil)
	m = SetScreen(m, ScreenStatus)
	m.status = cycle.StatusView{
		CycleNumber: 15,
		Title:       "Loop-back",
		Status:      store.CycleStatusActive,
		Stages: []cycle.StatusStage{
			{Name: "QA", Status: store.StageFailed, Iteration: "2/2", HumanApproval: "N/A"},
		},
		LoopBacks: []cycle.StatusLoopBackRow{
			{From: "qa", To: "implementation", Round: 2, FindingIDs: []string{"find-qa-1"}, OccurredAt: "2026-09-11T17:32:00Z"},
		},
		Findings: &cycle.StatusFindingsBlock{
			Counts: cycle.StatusFindingCounts{Open: 1, Reopened: 0, Done: 0, DeferredTodo: 1},
			Items: []cycle.StatusFindingRow{
				{ID: "find-qa-1", SourceStage: "qa", Owner: store.FindingOwnerGeneric, Status: store.FindingStatusOpen, Round: 1, Issue: "migration loses event rows"},
				{ID: "find-qa-3", SourceStage: "qa", Owner: store.FindingOwnerGeneric, Status: store.FindingStatusDeferredTodo, Round: 1, Issue: "manual fixture cleanup"},
			},
		},
		Todos:            &cycle.StatusTodosBlock{Pending: 1, Adopted: 0, DeferredFromCycle: 1},
		AvailableActions: []string{"hero-continue", "hero-add-todo"},
	}
	view := m.renderStatus()
	for _, want := range []string{
		"Loop-backs",
		"QA → Implementation",
		"find-qa-1",
		"Findings  open 1",
		"ToDo 1",
		"find-qa-3",
		"GEN",
		"ToDo",
		"ToDos  pending 1",
		"Escalated",
		"/hero-add-todo",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in status view:\n%s", want, view)
		}
	}
}

func TestRenderStatusFindingsNone(t *testing.T) {
	m := SetScreen(NewTestModel(nil), ScreenStatus)
	m.status = cycle.StatusView{
		CycleNumber: 1,
		Title:       "Empty",
		Status:      store.CycleStatusActive,
		Stages:      []cycle.StatusStage{{Name: "Research", Status: store.StageCompleted}},
		Findings:    &cycle.StatusFindingsBlock{Items: nil},
	}
	view := m.renderStatus()
	if !strings.Contains(view, "Findings  none") {
		t.Fatalf("expected empty findings label: %q", view)
	}
}

func TestFormatLifecycleEventSummary(t *testing.T) {
	cases := []struct {
		typ     string
		payload string
		want    string
	}{
		{
			store.EventFindingCreated,
			`{"finding_id":"find-qa-1","source_stage":"qa","owner":"generic_agent"}`,
			"find-qa-1 · QA → GEN",
		},
		{
			store.EventFindingReopened,
			`{"finding_id":"find-qa-1","source_stage":"qa","round":2}`,
			"find-qa-1 · QA · round 2",
		},
		{
			store.EventFindingDeferred,
			`{"finding_id":"find-judge-1","source_stage":"judge","cycle_number":15}`,
			"find-judge-1 · from C15/Judge",
		},
		{
			store.EventTodoAdopted,
			`{"todo_id":"todo-4","cycle_number":16}`,
			"todo-4 · C16",
		},
	}
	for _, tc := range cases {
		got := formatLifecycleEventSummary(store.Event{Type: tc.typ, PayloadJSON: tc.payload})
		if got != tc.want {
			t.Fatalf("type %s got %q want %q", tc.typ, got, tc.want)
		}
	}
}

func TestStatusFindingFocusKeys(t *testing.T) {
	m := SetScreen(NewTestModel(nil), ScreenStatus)
	m.status.Findings = &cycle.StatusFindingsBlock{
		Items: []cycle.StatusFindingRow{
			{ID: "find-qa-1"},
			{ID: "find-qa-2"},
		},
	}
	next, _ := m.handleStatusFindingKey("enter")
	if next.statusFindingFocus != 0 {
		t.Fatalf("focus=%d want 0", next.statusFindingFocus)
	}
	next, _ = next.handleStatusFindingKey("down")
	if next.statusFindingFocus != 1 {
		t.Fatalf("focus=%d want 1", next.statusFindingFocus)
	}
	next, _ = next.handleStatusFindingKey("esc")
	if next.statusFindingFocus != -1 {
		t.Fatalf("focus=%d want -1", next.statusFindingFocus)
	}
}
