package workflowconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestEnsureCurrentImportsHighestArchivedConfigAndKeepsTemplateOnlyData(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, ".workflow-hero", "templates", "workflow-config.yml")
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	template := `title: Template title
objective: Template objective
workflow_config:
  user_preferred_language: EN
  template_only: keep
scope:
  backend: true
  frontend: false
stages:
  research:
    enabled: true
    max_iterations: 50
    timeout_minutes: 15
  implementation:
    enabled: true
    max_iterations: 4
    timeout_minutes: 30
agents:
  orchestration_agent:
    harness: cursor
    model: template-orchestrator
  backend_agent:
    harness: cursor
    model: template-backend
fallback_model:
  harness: cursor
  model: template-fallback
workflow_rules:
  - keep template rules
`
	if err := os.WriteFile(templatePath, []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	archiveRoot := filepath.Join(dir, ".workflow-hero", "cycles", "archive")
	for _, cycle := range []string{"C8-old", "C9-latest"} {
		if err := os.MkdirAll(filepath.Join(archiveRoot, cycle), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(archiveRoot, "C8-old", "workflow-config.yml"), []byte("title: older\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := `title: Previous title
objective: Previous objective
workflow_config:
  user_preferred_language: PT-BR
  previous_only: import
scope:
  backend: false
stages:
  research:
    enabled: false
    max_iterations: 7
    timeout_minutes: 9
  previous_stage:
    enabled: true
agents:
  orchestration_agent:
    model: archived-orchestrator
    subagent:
      same_of_agent: false
  previous_agent:
    harness: codex
    model: gpt-5.4
fallback_model:
  harness: codex
  model: gpt-5.4
workflow_rules:
  - do not import this
`
	previousPath := filepath.Join(archiveRoot, "C9-latest", "workflow-config.yml")
	if err := os.WriteFile(previousPath, []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := workflowconfig.EnsureCurrent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.SourcePath != previousPath {
		t.Fatalf("result=%+v, want created from %q", result, previousPath)
	}

	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"title: Template title",
		"objective: Template objective",
		"user_preferred_language: PT-BR",
		"template_only: keep",
		"max_iterations: 7",
		"model: archived-orchestrator",
		"model: template-backend",
		"model: gpt-5.4",
		"keep template rules",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("prepared config missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Previous title") || strings.Contains(text, "do not import this") {
		t.Fatalf("cycle-specific or non-imported previous data leaked into config:\n%s", text)
	}

	snapshot := text
	second, err := workflowconfig.EnsureCurrent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.SourcePath != "" {
		t.Fatalf("existing config should not be recreated: %+v", second)
	}
	data, err = os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != snapshot {
		t.Fatal("existing current config changed during EnsureCurrent")
	}
}

func TestEnsureCurrentPreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	if err := os.MkdirAll(filepath.Dir(currentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	current := []byte("title: Existing\nobjective: Keep me\n")
	if err := os.WriteFile(currentPath, current, 0o640); err != nil {
		t.Fatal(err)
	}

	result, err := workflowconfig.EnsureCurrent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created || result.Path != currentPath {
		t.Fatalf("result=%+v", result)
	}
	got, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(current) {
		t.Fatalf("existing config changed: %q", got)
	}
}

func TestEnsureCurrentUsesTemplateForFirstCycle(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, ".workflow-hero", "templates", "workflow-config.yml")
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templatePath, []byte("title: First\nobjective: First objective\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := workflowconfig.EnsureCurrent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.SourcePath != "" {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("current config was not created: %v", err)
	}
}
