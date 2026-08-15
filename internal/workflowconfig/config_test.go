package workflowconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestResolveModelSlug_BareID(t *testing.T) {
	got := workflowconfig.ResolveModelSlug(workflowconfig.AgentModelConfig{
		Model:           "composer-2.5",
		ReasoningEffort: "na",
		EnableFastModel: false,
		Thinking:        "na",
	})
	if got != "composer-2.5" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveModelSlug_Fast(t *testing.T) {
	got := workflowconfig.ResolveModelSlug(workflowconfig.AgentModelConfig{
		Model:           "composer-2.5",
		EnableFastModel: true,
	})
	if got != "composer-2.5-fast" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveModelSlug_Effort(t *testing.T) {
	got := workflowconfig.ResolveModelSlug(workflowconfig.AgentModelConfig{
		Model:           "gpt-5.3-codex",
		ReasoningEffort: "medium",
		EnableFastModel: false,
		Thinking:        "na",
	})
	if got != "gpt-5.3-codex-medium" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveModelSlug_EffortAndThinking(t *testing.T) {
	got := workflowconfig.ResolveModelSlug(workflowconfig.AgentModelConfig{
		Model:           "cursor-grok-4.5",
		ReasoningEffort: "high",
		EnableFastModel: false,
		Thinking:        "true",
	})
	if got != "cursor-grok-4.5-high-thinking" {
		t.Fatalf("got %q", got)
	}
}

func TestOrchestratorModelSlug_FromCurrent(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`agents:
  orchestration_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  model: cursor-grok-4.5
  reasoning_effort: high
  enable_fast_model: false
  thinking: na
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := workflowconfig.OrchestratorModelSlug(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "gpt-5.3-codex-medium" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentModelSlug_NamedAgent(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`agents:
  discover_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  model: cursor-grok-4.5
  reasoning_effort: high
  enable_fast_model: false
  thinking: na
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	got, usedFallback, err := workflowconfig.AgentModelSlug(dir, "discover_agent")
	if err != nil {
		t.Fatal(err)
	}
	if usedFallback {
		t.Fatal("expected agent block, not fallback")
	}
	if got != "gpt-5.3-codex-medium" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentModelSlug_FallbackWhenAgentMissing(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`fallback_model:
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	got, usedFallback, err := workflowconfig.AgentModelSlug(dir, "discover_agent")
	if err != nil {
		t.Fatal(err)
	}
	if !usedFallback {
		t.Fatal("expected fallback")
	}
	if got != "composer-2.5" {
		t.Fatalf("got %q", got)
	}
}

func TestOrchestratorModelSlug_FallbackWhenOrchestratorMissing(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`fallback_model:
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := workflowconfig.OrchestratorModelSlug(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "composer-2.5" {
		t.Fatalf("got %q", got)
	}
}
