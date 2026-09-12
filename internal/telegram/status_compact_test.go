package telegram

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

func TestCompactFindingsStatusEmptyWhenNoData(t *testing.T) {
	if got := CompactFindingsStatus(cycle.StatusView{}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestCompactFindingsStatusCountsAndActionableIDs(t *testing.T) {
	view := cycle.StatusView{
		Findings: &cycle.StatusFindingsBlock{
			Counts: cycle.StatusFindingCounts{Open: 1, Reopened: 1, Done: 2, DeferredTodo: 1},
			Items: []cycle.StatusFindingRow{
				{ID: "find-qa-1", Status: "open"},
				{ID: "find-qa-2", Status: "reopened"},
				{ID: "find-done-1", Status: "done"},
				{ID: "find-extra", Status: "open"},
			},
		},
		AvailableActions: []string{"hero-continue", "hero-add-todo"},
	}
	got := CompactFindingsStatus(view)
	for _, want := range []string{
		"Findings: open 1 · reopened 1 · done 2 · ToDo 1",
		"Actionable: find-qa-1, find-qa-2, find-extra",
		"Actions: /hero-continue, /hero-add-todo",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestCompactFindingsStatusNone(t *testing.T) {
	view := cycle.StatusView{
		Findings: &cycle.StatusFindingsBlock{
			Counts: cycle.StatusFindingCounts{},
			Items:  nil,
		},
	}
	got := CompactFindingsStatus(view)
	if got != "Findings none" {
		t.Fatalf("got %q", got)
	}
}
