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

func TestCompactFindingsStatusIncludesBlockedCorrection(t *testing.T) {
	view := cycle.StatusView{
		BlockedStages: []cycle.StatusBlockedStage{{
			Name: "Browser UI Validation",
			Blockers: []cycle.StatusStageBlocker{{
				ID: "browser-capability-missing", Reason: "tool_unavailable",
				AffectedCoverageIDs: []string{"screen-dashboard"},
				AffectedProfileIDs:  []string{"operator"},
				NextAction:          "configure the planned browser method, then continue",
			}},
		}},
		AvailableActions: []string{"hero-continue"},
	}
	got := CompactFindingsStatus(view)
	for _, want := range []string{
		"Blocked: Browser UI Validation — tool_unavailable",
		"coverage screen-dashboard · profiles operator",
		"Next: configure the planned browser method, then continue",
		"Actions: /hero-continue",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestCompactFindingsStatusIncludesActiveBudgetWithoutFindings(t *testing.T) {
	view := cycle.StatusView{StageBudgets: []cycle.StatusStageBudget{{Name: "Implementation", State: "interrupted", ConsumedMS: 60000, RemainingMS: 120000}}}
	got := CompactFindingsStatus(view)
	if !strings.Contains(got, "Budget: Implementation · interrupted · elapsed 1m0s · remaining 2m0s") {
		t.Fatalf("missing durable budget: %s", got)
	}
}
