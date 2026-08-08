package cycle_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestServiceCyclesSQLiteAndArchive(t *testing.T) {
	dir := t.TempDir()
	heroDir := filepath.Join(dir, ".workflow-hero")
	cycleDir := filepath.Join(heroDir, "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: Active Cycle
objective: Test cycles listing
stages:
  research:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: false
  qa:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	res, err := svc.NewCycle("", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "done", `{"agent":"discover_agent","input_tokens":12000,"output_tokens":3000,"cost_usd":0.05,"duration_ms":480000}`, false); err != nil {
		t.Fatal(err)
	}

	archiveDir := filepath.Join(heroDir, "cycles", "archive", "C2-2026-08-08-slash-parity-tui-harness")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyMetrics := `# Metrics — Cycle C2

**Title**: slash-parity-tui-harness

## Stage Metrics

| Stage | Agent | Model | Input Tokens | Output Tokens | Cost (USD) | Duration |
|-------|-------|-------|-------------|---------------|------------|----------|
| Research | discover_agent | composer-2.5 | 8,000 | 2,000 | 0.04 | 8m |
| Planning | planning_agent | composer-2.5 | 5,000 | 1,500 | 0.03 | 5m |
`
	if err := os.WriteFile(filepath.Join(archiveDir, "metrics.md"), []byte(legacyMetrics), 0o644); err != nil {
		t.Fatal(err)
	}

	view, err := svc.Cycles()
	if err != nil {
		t.Fatal(err)
	}
	if view.Total != 2 {
		t.Fatalf("total=%d want 2: %+v", view.Total, view)
	}

	var active, archived *cycle.CycleEntry
	for i := range view.Cycles {
		switch view.Cycles[i].Number {
		case res.Cycle.Number:
			active = &view.Cycles[i]
		case 2:
			archived = &view.Cycles[i]
		}
	}
	if active == nil || active.Source != "sqlite" || active.Status != store.CycleStatusActive {
		t.Fatalf("active cycle: %+v", active)
	}
	if active.TotalIn != 12000 || active.TotalOut != 3000 {
		t.Fatalf("active totals: in=%d out=%d", active.TotalIn, active.TotalOut)
	}
	if len(active.Stages) == 0 {
		t.Fatalf("active stages: %+v", active.Stages)
	}
	var research *cycle.CycleStageRow
	for i := range active.Stages {
		if active.Stages[i].Name == "Research" {
			research = &active.Stages[i]
			break
		}
	}
	if research == nil || research.Status != "completed" {
		t.Fatalf("research stage: %+v stages=%+v", research, active.Stages)
	}

	if archived == nil || archived.Source != "archive" {
		t.Fatalf("archive cycle: %+v", archived)
	}
	if archived.Title != "slash-parity-tui-harness" || archived.ArchivedDate != "2026-08-08" {
		t.Fatalf("archive meta: %+v", archived)
	}
	if archived.TotalIn != 13000 || archived.TotalOut != 3500 {
		t.Fatalf("archive totals: in=%d out=%d", archived.TotalIn, archived.TotalOut)
	}
}

func TestFormatCyclesMatchesUIC03(t *testing.T) {
	view := cycle.CyclesView{
		Total: 2,
		Cycles: []cycle.CycleEntry{
			{
				Number: 3,
				Title:  "Correção de distorções na implementação da V1",
				Status: store.CycleStatusActive,
				Stages: []cycle.CycleStageRow{
					{Name: "Research", Status: "running", InputTokens: 12000, OutputTokens: 3000, CostUSD: 0.05, DurationMS: 480000},
				},
				TotalIn: 12000, TotalOut: 3000, TotalCost: 0.05, TotalTokens: 15000,
			},
			{
				Number:       2,
				Title:        "slash-parity-tui-harness",
				Status:       store.CycleStatusArchived,
				ArchivedDate: "2026-08-08",
				Source:       "archive",
				Stages: []cycle.CycleStageRow{
					{Name: "Research", InputTokens: 8000, OutputTokens: 2000, CostUSD: 0.04, DurationMS: 480000},
				},
				TotalIn: 8000, TotalOut: 2000, TotalCost: 0.04, TotalTokens: 10000,
			},
		},
	}

	var buf bytes.Buffer
	cycle.FormatCycles(&buf, view)
	out := buf.String()

	for _, want := range []string{
		"Cycles (2 total)",
		"C3 — Correção de distorções na implementação da V1 [active]",
		"Research",
		"running",
		"in: 12k",
		"out: 3k",
		"~$0.05",
		"8m",
		"Total: 15k tokens",
		"C2 — slash-parity-tui-harness [archived 2026-08-08]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestParseLegacyMetricsMeta(t *testing.T) {
	content := `# Metrics — Cycle C7

**Title**: Legacy Title Here
`
	meta := store.ParseLegacyMetricsMeta(content)
	if meta.Number != 7 || meta.Title != "Legacy Title Here" {
		t.Fatalf("meta=%+v", meta)
	}
}
