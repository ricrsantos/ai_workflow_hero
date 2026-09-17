package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
)

// The cycle is the priority: a rejected report is retried automatically, and
// only after the budget is spent does the user have to intervene.
func TestReportRetryIsBoundedPerStage(t *testing.T) {
	var m model
	for attempt := 1; attempt <= maxReportRetries; attempt++ {
		got, ok := m.nextReportRetry(stageQA)
		if !ok || got != attempt {
			t.Fatalf("attempt %d: got=%d ok=%v want %d true", attempt, got, ok, attempt)
		}
	}
	if _, ok := m.nextReportRetry(stageQA); ok {
		t.Fatal("retries must stop at the bound so the stage can escalate")
	}
}

// Moving to another stage starts a fresh budget; one bad QA report must not
// consume Judge's retries.
func TestReportRetryResetsOnStageChange(t *testing.T) {
	var m model
	for i := 0; i < maxReportRetries; i++ {
		if _, ok := m.nextReportRetry(stageQA); !ok {
			t.Fatalf("qa retry %d unexpectedly refused", i+1)
		}
	}
	got, ok := m.nextReportRetry(stageJudge)
	if !ok || got != 1 {
		t.Fatalf("judge got=%d ok=%v want 1 true", got, ok)
	}
}

func TestFormatContractWarnings(t *testing.T) {
	if got := formatContractWarnings(stageQA, nil); got != "" {
		t.Fatalf("no warnings must render nothing, got %q", got)
	}
	copy := formatContractWarnings(stageQA, []reports.ReportWarning{
		{Code: reports.CodeUnknownField, Field: "metrics"},
		{Code: reports.CodeFieldRenamed, Field: "reopenId", Value: "reopen_id"},
	})
	for _, want := range []string{"QA report accepted", "metrics ignored", "reopenId → reopen_id"} {
		if !strings.Contains(copy, want) {
			t.Errorf("chat copy %q missing %q", copy, want)
		}
	}
	if strings.Contains(copy, "rejected") {
		t.Errorf("a tolerated report must not read as rejected: %q", copy)
	}
}

// The field failure: a QA report carrying the metrics object its own prompt
// illustrated must still close the stage.
func TestQAReportWithStrayMetricsStillDecodes(t *testing.T) {
	raw := `{"status":"passed","failures":[],"summary":"all green",` +
		`"metrics":{"model":"muse-spark-1.3","input_chars":1,"output_chars":1}}`
	report, err := reports.DecodeQA([]byte(raw), reports.DecodeContext{})
	if err != nil {
		t.Fatalf("stray metrics must not stall the cycle: %v", err)
	}
	if report.Status != reports.ValidationStatusPassed {
		t.Fatalf("status=%q want passed", report.Status)
	}
	if len(report.ContractWarnings) != 1 || report.ContractWarnings[0].Field != "metrics" {
		t.Fatalf("warnings=%+v want one unknown_field on metrics", report.ContractWarnings)
	}
}
