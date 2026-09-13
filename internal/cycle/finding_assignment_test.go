package cycle

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestFindingAssignmentMarkdownResidualAndReproUntruncated(t *testing.T) {
	long := strings.Repeat("still broken after join ", 20)
	residual := strings.TrimSpace(long)
	f := store.Finding{
		ID:                 "find-qa-19",
		SourceStage:        store.FindingSourceQA,
		File:               "internal/tui/conversation.go",
		Requirement:        "PRD-C16-001 FR-08",
		Issue:              "Cancellation does not join the worker",
		AcceptanceCriteria: "cancel and join in-flight execution",
		ReproPackage:       "./internal/tui",
		ReproTest:          "TestFindQA19CancelJoinsWorker",
	}
	src := "package tui\n\nimport \"testing\"\n\nfunc TestFindQA19CancelJoinsWorker(t *testing.T) {\n\tt.Fatal(\"" + long + "\")\n}\n"
	occs := []store.FindingOccurrence{
		{Round: 1, Kind: store.OccurrenceCreated, Issue: "Quit races persist"},
		{Round: 2, Kind: store.OccurrenceReopened, Issue: residual, ReproPackage: f.ReproPackage, ReproTest: f.ReproTest, ReproSource: src},
	}
	block := FindingAssignmentMarkdown(f, occs)
	if !strings.Contains(block, "Residual: "+residual) {
		t.Fatalf("residual must be untruncated:\n%s", block)
	}
	if strings.Contains(block, "Residual: "+residual[:20]+"...") && !strings.Contains(block, residual) {
		t.Fatal("residual truncated")
	}
	if !strings.Contains(block, "Package: ./internal/tui") {
		t.Fatalf("missing package:\n%s", block)
	}
	if !strings.Contains(block, src) && !strings.Contains(block, "func TestFindQA19CancelJoinsWorker") {
		t.Fatalf("missing full source:\n%s", block)
	}
	if !strings.Contains(block, "Issue: Cancellation does not join the worker") {
		t.Fatalf("frozen issue missing:\n%s", block)
	}
	if !strings.Contains(block, "Fix Residual") {
		t.Fatalf("missing residual instruction:\n%s", block)
	}
}

func TestLatestFindingResidualSkipsDone(t *testing.T) {
	f := store.Finding{Issue: "frozen original"}
	occs := []store.FindingOccurrence{
		{Kind: store.OccurrenceCreated, Issue: "first"},
		{Kind: store.OccurrenceReopened, Issue: "current residual"},
		{Kind: store.OccurrenceDone, Issue: "claimed fixed"},
	}
	if got := store.LatestFindingResidual(f, occs); got != "current residual" {
		t.Fatalf("got %q", got)
	}
}
