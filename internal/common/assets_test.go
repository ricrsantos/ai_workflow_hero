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
		"hero-new", "hero-start", "hero-approve", "hero-reject",
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
		"qa_agent", "judge_agent", "browser_ui_agent", "end2end_qa_agent",
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
		"env.example",
		"gitignore-secrets",
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

// TestAssets_UserGuide verifies the end-user guide asset exists and covers core topics.
func TestAssets_UserGuide(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "docs/workflow-help.md")
	if err != nil {
		t.Fatalf("missing docs/workflow-help.md: %v", err)
	}
	content := string(data)
	for _, kw := range []string{
		"Philosophy",
		"hero install",
		"hero uninstall",
		"hero upgrade",
		"workflow-config.yml",
		"/hero:new",
		"/hero:start",
		"Architecture",
		"Logging",
		"error",
		"info",
		"debug",
		".workflow-hero/docs/workflow-help.md",
	} {
		if !strings.Contains(content, kw) {
			t.Errorf("workflow-help.md missing %q", kw)
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

// TestAssets_OneFilePerAgent verifies agent count matches ADR-011 (11 agents).
func TestAssets_OneFilePerAgent(t *testing.T) {
	count := 0
	_ = fs.WalkDir(assets.FS, "cursor/agents", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		return nil
	})
	if count != 11 {
		t.Errorf("expected 11 agent files, got %d", count)
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
	// fallback_model must appear after agents and before workflow_rules
	agentsIdx := strings.Index(content, "\nagents:")
	fallbackIdx := strings.Index(content, "\nfallback_model:")
	rulesIdx := strings.Index(content, "\nworkflow_rules:")
	if agentsIdx < 0 || fallbackIdx < 0 || rulesIdx < 0 {
		t.Fatal("workflow-config.yml missing agents, fallback_model, or workflow_rules section")
	}
	if !(agentsIdx < fallbackIdx && fallbackIdx < rulesIdx) {
		t.Errorf("fallback_model must be after agents and before workflow_rules (agents=%d fallback=%d rules=%d)", agentsIdx, fallbackIdx, rulesIdx)
	}
}

// TestAssets_WorkflowConfigNestedSubagent verifies every agent block has a nested subagent
// config (same_of_agent + model fields) for nested Task fan-out.
func TestAssets_WorkflowConfigNestedSubagent(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "templates/workflow-config.yml")
	if err != nil {
		t.Fatalf("read workflow-config template: %v", err)
	}
	content := string(data)
	agents := []string{
		"planning_agent", "context_agent", "backend_agent", "frontend_agent",
		"generic_agent", "qa_agent", "judge_agent", "browser_ui_agent", "end2end_qa_agent",
	}
	agentsIdx := strings.Index(content, "\nagents:")
	fallbackIdx := strings.Index(content, "\nfallback_model:")
	if agentsIdx < 0 || fallbackIdx < 0 || fallbackIdx <= agentsIdx {
		t.Fatal("workflow-config.yml missing agents or fallback_model section")
	}
	agentsSection := content[agentsIdx:fallbackIdx]
	subagentCount := strings.Count(agentsSection, "\n    subagent:")
	sameOfCount := strings.Count(agentsSection, "\n      same_of_agent:")
	if subagentCount != len(agents) {
		t.Errorf("expected %d nested subagent: blocks under agents, got %d", len(agents), subagentCount)
	}
	if sameOfCount != len(agents) {
		t.Errorf("expected %d same_of_agent: entries under agents, got %d", len(agents), sameOfCount)
	}
	for _, agent := range agents {
		marker := "\n  " + agent + ":"
		idx := strings.Index(agentsSection, marker)
		if idx < 0 {
			t.Errorf("agents section missing %s", agent)
			continue
		}
		// Slice until the next top-level agent key (exactly two spaces after newline).
		rest := agentsSection[idx+len(marker):]
		nextAgent := -1
		for i := 0; i+3 < len(rest); i++ {
			if rest[i] == '\n' && rest[i+1] == ' ' && rest[i+2] == ' ' && rest[i+3] != ' ' {
				nextAgent = i
				break
			}
		}
		block := rest
		if nextAgent >= 0 {
			block = rest[:nextAgent]
		}
		for _, kw := range []string{"subagent:", "same_of_agent:", "model:", "reasoning_effort:", "enable_fast_model:", "thinking:"} {
			if !strings.Contains(block, kw) {
				t.Errorf("%s block missing %q", agent, kw)
			}
		}
	}
}

// TestAssets_WorkflowConfigUserPreferredLanguage verifies workflow_config.user_preferred_language defaults to EN and precedes scope.
func TestAssets_WorkflowConfigUserPreferredLanguage(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "templates/workflow-config.yml")
	if err != nil {
		t.Fatalf("read workflow-config template: %v", err)
	}
	content := string(data)
	for _, kw := range []string{
		"workflow_config:",
		"user_preferred_language: EN",
		"All agents must communicate with the user in chat using workflow_config.user_preferred_language",
	} {
		if !strings.Contains(content, kw) {
			t.Errorf("workflow-config.yml template missing %q", kw)
		}
	}
	cfgIdx := strings.Index(content, "\nworkflow_config:")
	scopeIdx := strings.Index(content, "\nscope:")
	if cfgIdx < 0 || scopeIdx < 0 {
		t.Fatal("workflow-config.yml missing workflow_config or scope section")
	}
	if !(cfgIdx < scopeIdx) {
		t.Errorf("workflow_config must appear before scope (workflow_config=%d scope=%d)", cfgIdx, scopeIdx)
	}
}

// TestAssets_WorkflowConfigUsePlaywright verifies qa_end_to_end exposes use_playwright and related workflow rules.
func TestAssets_WorkflowConfigUsePlaywright(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "templates/workflow-config.yml")
	if err != nil {
		t.Fatalf("read workflow-config template: %v", err)
	}
	content := string(data)
	for _, kw := range []string{
		"qa_end_to_end:",
		"use_playwright:",
		"stages.qa_end_to_end.use_playwright may be true only when scope.frontend is true",
		"When use_playwright is true, end2end_qa_agent uses Playwright",
	} {
		if !strings.Contains(content, kw) {
			t.Errorf("workflow-config.yml template missing %q", kw)
		}
	}
}

// TestAssets_WorkflowConfigBrowserUIValidation verifies browser_ui_validation stage + agent + scope gate.
func TestAssets_WorkflowConfigBrowserUIValidation(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "templates/workflow-config.yml")
	if err != nil {
		t.Fatalf("read workflow-config template: %v", err)
	}
	content := string(data)
	for _, kw := range []string{
		"browser_ui_validation:",
		"visual_validation:",
		"reference_dir: docs/ui/visual_reference",
		"browser_ui_agent:",
		"stages.browser_ui_validation.enabled may be true only when scope.frontend is true",
		"Browser Health always runs",
	} {
		if !strings.Contains(content, kw) {
			t.Errorf("workflow-config.yml template missing %q", kw)
		}
	}
	// Defaults: stage and visual off
	idx := strings.Index(content, "browser_ui_validation:")
	if idx < 0 {
		t.Fatal("browser_ui_validation block missing")
	}
	block := content[idx:]
	if end := strings.Index(block, "\n  qa_end_to_end:"); end > 0 {
		block = block[:end]
	}
	if !strings.Contains(block, "enabled: false") {
		t.Error("browser_ui_validation should default enabled: false")
	}
	if !strings.Contains(block, "visual_validation:") || !strings.Contains(block, "enabled: false") {
		t.Error("visual_validation should default enabled: false")
	}
}

// TestAssets_AgentTemplateSections verifies the AGENTS.md template includes standard agent guidance sections.
func TestAssets_AgentTemplateSections(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "templates/AGENTS.md")
	if err != nil {
		t.Fatalf("read AGENTS.md template: %v", err)
	}
	content := string(data)
	for _, section := range []string{
		"Context compression files",
		"context/current-state.md",
		"context/context-log.md",
		"## Reference Lookup Order",
		"## Ambiguity and Missing Information",
		"## Testing",
		"## Secrets and Environment Variables",
		".env.example",
		"Never leave the project in a failing state",
	} {
		if !strings.Contains(content, section) {
			t.Errorf("AGENTS.md template missing %q", section)
		}
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
