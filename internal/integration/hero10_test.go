package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/doctor"
	"github.com/ricrsantos/ai_workflow_hero/internal/status"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/upgrade"
)

// TestIntegration_Hero10CycleAPI covers install → cycle new → status/metrics/events → approve.
func TestIntegration_Hero10CycleAPI(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "1.0.0")

	if _, err := os.Stat(filepath.Join(dir, store.RelativeDBPath)); err != nil {
		t.Fatalf("hero.db missing after install: %v", err)
	}

	// Ensure workflow-config exists in current (install copies templates; copy into current).
	tpl := filepath.Join(dir, cursoradapter.HeroTemplatesDir, "workflow-config.yml")
	cfg := filepath.Join(dir, cursoradapter.HeroCurrentCycleDir, "workflow-config.yml")
	data, err := os.ReadFile(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, data, 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	res, err := svc.NewCycle("Integration", "API path")
	if err != nil {
		t.Fatal(err)
	}
	if res.Cycle.Number != 1 {
		t.Fatalf("cycle number=%d", res.Cycle.Number)
	}

	ws, err := status.Run(status.Options{ProjectDir: dir})
	if err != nil || len(ws.Stages) == 0 {
		t.Fatalf("status: %+v %v", ws, err)
	}

	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "ok", `{"agent":"discover_agent","input_tokens":20,"output_tokens":5}`, false); err != nil {
		t.Fatal(err)
	}

	m, err := svc.Metrics()
	if err != nil || m.TotalIn != 20 {
		t.Fatalf("metrics: %+v %v", m, err)
	}
	ev, err := svc.Events("", 50)
	if err != nil || len(ev.Events) == 0 {
		t.Fatalf("events: %+v %v", ev, err)
	}

	// Drive a pending-approval stage.
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Approve("lgtm", `{"agent":"qa_agent","input_tokens":4}`); err != nil {
		t.Fatal(err)
	}

	report := doctor.Run(doctor.Options{ProjectDir: dir, BinaryVersion: "1.0.0"})
	if !report.OK {
		t.Fatalf("doctor not ok after cycle API use: %+v", report)
	}
}

// TestIntegration_UpgradeFrom09LikeTree imports legacy workflow.md into SQLite.
func TestIntegration_UpgradeFrom09LikeTree(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "0.9.0")

	// Simulate 0.9: remove DB, add workflow.md + metrics.md.
	_ = os.Remove(filepath.Join(dir, store.RelativeDBPath))
	cycleDir := filepath.Join(dir, cursoradapter.HeroCurrentCycleDir)
	workflow := `# Workflow — Cycle C1

**Title**: Legacy Up
**Objective**: Import on upgrade
**Status**: In Progress
**Started**: 2026-07-01
**Completed**:

## Stages

| Stage | Status | Iteration | Human Approval | Extra Iterations Granted |
|-------|--------|-----------|----------------|--------------------------|
| Research | Completed | 1/50 | Auto | +0 |
| Planning | In Progress | 1/3 | N/A | +0 |
| QA | Waiting | 0/2 | Required | +0 |
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow.md"), []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}
	metrics := `# Metrics — Cycle C1

## Stage Metrics

| Stage | Agent | Model | Input Tokens | Output Tokens | Cost (USD) | Duration |
|-------|-------|-------|-------------|---------------|------------|----------|
| Research | discover_agent | composer-2.5 | 100 | 40 | 0.01 | 1s |
`
	if err := os.WriteFile(filepath.Join(cycleDir, "metrics.md"), []byte(metrics), 0o644); err != nil {
		t.Fatal(err)
	}

	var sb strings.Builder
	if _, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "1.0.0",
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("upgrade: %v\n%s", err, sb.String())
	}

	if _, err := os.Stat(filepath.Join(dir, store.RelativeDBPath)); err != nil {
		t.Fatalf("hero.db missing after upgrade: %v", err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	ws, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if ws.CycleNumber != 1 || ws.Title != "Legacy Up" || len(ws.Stages) < 2 {
		t.Fatalf("status after upgrade: %+v", ws)
	}
	m, err := svc.Metrics()
	if err != nil || m.TotalIn != 100 {
		t.Fatalf("metrics after upgrade: %+v %v", m, err)
	}

	report := doctor.Run(doctor.Options{ProjectDir: dir, BinaryVersion: "1.0.0"})
	if !report.OK {
		t.Fatalf("doctor after 0.9 upgrade: %+v", report)
	}
}
