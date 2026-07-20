package status_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/status"
)

const sampleWorkflowMD = `# Workflow — Cycle 1

**Title**: New Feature

## Stages

| Stage | Status | Iteration | Human Approval | Extra Iterations Granted |
|-------|--------|-----------|----------------|--------------------------|
| Configuration | Completed | 1/1 | N/A | +0 |
| Research | Completed | 1/1 | Approved | +0 |
| Planning | In Progress | 1/3 | Pending | +0 |
| Implementation | Waiting | 0/4 | N/A | +0 |
| QA | Waiting | 0/2 | N/A | +0 |
| Judge | Waiting | 0/3 | N/A | +0 |
| QA End-to-End | Waiting | 0/1 | N/A | +0 |
`

func TestStatus_ParseWorkflowMD(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, cursoradapter.HeroCurrentCycleDir)
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow.md"), []byte(sampleWorkflowMD), 0o644); err != nil {
		t.Fatalf("write workflow.md: %v", err)
	}

	ws, err := status.Run(status.Options{ProjectDir: dir})
	if err != nil {
		t.Fatalf("status.Run: %v", err)
	}

	if len(ws.Stages) != 7 {
		t.Errorf("expected 7 stages, got %d", len(ws.Stages))
	}

	if ws.Stages[0].Name != "Configuration" {
		t.Errorf("stages[0].Name = %q, want Configuration", ws.Stages[0].Name)
	}
	if ws.Stages[0].Status != "Completed" {
		t.Errorf("stages[0].Status = %q, want Completed", ws.Stages[0].Status)
	}
	if ws.Stages[2].Status != "In Progress" {
		t.Errorf("stages[2].Status = %q, want In Progress", ws.Stages[2].Status)
	}
}

func TestStatus_NoWorkflowMD_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, cursoradapter.HeroCurrentCycleDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ws, err := status.Run(status.Options{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ws.Stages) != 0 {
		t.Errorf("expected empty stages for missing workflow.md, got %d", len(ws.Stages))
	}
}

func TestStatus_TableOutput(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, cursoradapter.HeroCurrentCycleDir)
	_ = os.MkdirAll(cycleDir, 0o755)
	_ = os.WriteFile(filepath.Join(cycleDir, "workflow.md"), []byte(sampleWorkflowMD), 0o644)

	ws, _ := status.Run(status.Options{ProjectDir: dir})

	var sb strings.Builder
	status.PrintTable(&sb, ws)
	out := sb.String()

	if !strings.Contains(out, "Configuration") {
		t.Errorf("table output missing 'Configuration': %s", out)
	}
	if !strings.Contains(out, "Planning") {
		t.Errorf("table output missing 'Planning': %s", out)
	}
}

func TestStatus_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, cursoradapter.HeroCurrentCycleDir)
	_ = os.MkdirAll(cycleDir, 0o755)
	_ = os.WriteFile(filepath.Join(cycleDir, "workflow.md"), []byte(sampleWorkflowMD), 0o644)

	ws, _ := status.Run(status.Options{ProjectDir: dir})

	data, err := json.Marshal(ws)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded status.WorkflowStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(decoded.Stages) != 7 {
		t.Errorf("expected 7 stages in JSON output, got %d", len(decoded.Stages))
	}
}

func TestStatus_InstalledProject(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	var sb strings.Builder
	if err := install.Run(install.Options{
		ProjectDir: dir,
		Name:       "Test",
		Summary:    "test",
		Tools:      []string{"cursor"},
		Version:    "1.0.0",
		GitInit:    false,
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("install: %v", err)
	}

	// After install, no workflow.md in current/ — should return empty gracefully.
	ws, err := status.Run(status.Options{ProjectDir: dir})
	if err != nil {
		t.Fatalf("status.Run: %v", err)
	}
	if len(ws.Stages) != 0 {
		t.Errorf("expected empty stages, got %d", len(ws.Stages))
	}
}
