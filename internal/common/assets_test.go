package common_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

// TestAssets_CommandInventory verifies all ADR-011 command files exist in the embedded FS.
func TestAssets_CommandInventory(t *testing.T) {
	commands := []string{
		"hero-init", "hero-start", "hero-approve", "hero-reject",
		"hero-cancel", "hero-finish", "hero-archive", "hero-resume",
		"hero-sync", "hero-status", "hero-help", "hero-continue", "hero-back",
	}

	for _, cmd := range commands {
		path := "cursor/commands/" + cmd + ".md"
		if _, err := assets.FS.Open(path); err != nil {
			t.Errorf("missing command asset: %s", path)
		}
	}
}

// TestAssets_AgentInventory verifies all ADR-011 agent files exist in the embedded FS.
func TestAssets_AgentInventory(t *testing.T) {
	agents := []string{
		"orchestration_agent", "discover_agent", "planning_agent", "context_agent",
		"backend_agent", "frontend_agent", "generic_agent",
		"qa_agent", "judge_agent", "end2end_qa_agent",
	}

	for _, agent := range agents {
		path := "cursor/agents/" + agent + ".md"
		if _, err := assets.FS.Open(path); err != nil {
			t.Errorf("missing agent asset: %s", path)
		}
	}
}

// TestAssets_SkillFiles verifies skill stubs exist.
func TestAssets_SkillFiles(t *testing.T) {
	skills := []string{
		"cursor/skills/workflow-hero/SKILL.md",
		"cursor/skills/grilling/SKILL.md",
	}
	for _, path := range skills {
		if _, err := assets.FS.Open(path); err != nil {
			t.Errorf("missing skill asset: %s", path)
		}
	}
}

// TestAssets_ModelFiles verifies all model YAML files exist.
func TestAssets_ModelFiles(t *testing.T) {
	models := []string{
		"openai.yml", "anthropic.yml", "google.yml",
		"cursor.yml", "moonshot.yml", "zhipu.yml", "xai.yml",
	}
	for _, m := range models {
		path := "models/" + m
		if _, err := assets.FS.Open(path); err != nil {
			t.Errorf("missing model asset: %s", path)
		}
	}
}

// TestAssets_TemplateFiles verifies template files exist.
func TestAssets_TemplateFiles(t *testing.T) {
	templates := []string{
		"workflow-config.yml",
		"workflow.md",
		"metrics.md",
		"AGENTS.md",
		"current-state.md",
		"context-log.md",
	}
	for _, tmpl := range templates {
		path := "templates/" + tmpl
		if _, err := assets.FS.Open(path); err != nil {
			t.Errorf("missing template asset: %s", path)
		}
	}
}

// TestAssets_ConfigFiles verifies config template files exist.
func TestAssets_ConfigFiles(t *testing.T) {
	cfgFiles := []string{"documents.json"}
	for _, f := range cfgFiles {
		path := "config/" + f
		if _, err := assets.FS.Open(path); err != nil {
			t.Errorf("missing config asset: %s", path)
		}
	}
}

// TestAssets_OneFilePerCommand verifies command count matches ADR-011 (13 commands).
func TestAssets_OneFilePerCommand(t *testing.T) {
	count := 0
	_ = fs.WalkDir(assets.FS, "cursor/commands", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		return nil
	})
	if count != 13 {
		t.Errorf("expected 13 command files, got %d", count)
	}
}

// TestAssets_OneFilePerAgent verifies agent count matches ADR-011 (10 agents).
func TestAssets_OneFilePerAgent(t *testing.T) {
	count := 0
	_ = fs.WalkDir(assets.FS, "cursor/agents", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		return nil
	})
	if count != 10 {
		t.Errorf("expected 10 agent files, got %d", count)
	}
}

// TestAssets_WorkflowConfigFallbackModel verifies the workflow-config template defines fallback_model with full model options.
func TestAssets_WorkflowConfigFallbackModel(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "templates/workflow-config.yml")
	if err != nil {
		t.Fatalf("read workflow-config template: %v", err)
	}
	content := string(data)
	for _, kw := range []string{"fallback_model:", "reasoning_effort:", "enable_fast_model:", "thinking:"} {
		if !strings.Contains(content, kw) {
			t.Errorf("workflow-config.yml template missing %q", kw)
		}
	}
	if strings.Contains(content, "generic_model") {
		t.Error("workflow-config.yml template still references generic_model")
	}
}

// TestAssets_PathConstants verifies cursor adapter path constants match expected patterns.
func TestAssets_PathConstants(t *testing.T) {
	expectedPairs := map[string]string{
		"CommandsDir":          cursoradapter.CommandsDir,
		"AgentsDir":            cursoradapter.AgentsDir,
		"WorkflowHeroSkillDir": cursoradapter.WorkflowHeroSkillDir,
		"GrillingSkillDir":     cursoradapter.GrillingSkillDir,
	}
	for name, val := range expectedPairs {
		if val == "" {
			t.Errorf("cursor adapter constant %s is empty", name)
		}
	}
}
