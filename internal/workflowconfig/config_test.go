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
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
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
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
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
  harness: cursor
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
  harness: cursor
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

func TestValidateAgents_HarnessCodex(t *testing.T) {
	cfg := workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"planning_agent": {
				Harness: "codex",
				Model:   "gpt-5.4",
			},
		},
		FallbackModel: workflowconfig.AgentModelConfig{
			Harness: "codex",
			Model:   "gpt-5.4",
		},
	}
	if err := workflowconfig.ValidateAgents(cfg); err != nil {
		t.Fatalf("harness: codex must be valid: %v", err)
	}
}

func TestResolvePair_CodexNativeModel(t *testing.T) {
	pair := workflowconfig.ResolvePair(workflowconfig.AgentModelConfig{
		Harness:         "codex",
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		EnableFastModel: false,
		Thinking:        "na",
	})
	if pair.Harness != "codex" {
		t.Fatalf("harness=%q", pair.Harness)
	}
	if pair.Model != "gpt-5.4" {
		t.Fatalf("model=%q", pair.Model)
	}
	if pair.Slug != "gpt-5.4" {
		t.Fatalf("codex slug must stay native id, got %q", pair.Slug)
	}
}

func TestResolvePair_OpenCodeNativeModelUnchanged(t *testing.T) {
	pair := workflowconfig.ResolvePair(workflowconfig.AgentModelConfig{
		Harness: "opencode",
		Model:   "anthropic/claude-sonnet-4",
	})
	if pair.Slug != "anthropic/claude-sonnet-4" {
		t.Fatalf("opencode slug=%q", pair.Slug)
	}
}

func TestInjectHarnessForNew_SingleCodex(t *testing.T) {
	cfg := &workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"planning_agent": {Model: "gpt-5.4"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Model: "gpt-5.4"},
	}
	workflowconfig.InjectHarnessForNew(cfg, []string{"codex"})
	if cfg.Agents["planning_agent"].Harness != "codex" {
		t.Fatalf("agent harness=%q", cfg.Agents["planning_agent"].Harness)
	}
	if cfg.FallbackModel.Harness != "codex" {
		t.Fatalf("fallback harness=%q", cfg.FallbackModel.Harness)
	}
}

func TestInjectHarnessForNew_SingleOpenCodeUnchanged(t *testing.T) {
	cfg := &workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"planning_agent": {Model: "anthropic/claude-sonnet-4"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Model: "anthropic/claude-sonnet-4"},
	}
	workflowconfig.InjectHarnessForNew(cfg, []string{"opencode"})
	if cfg.Agents["planning_agent"].Harness != "opencode" {
		t.Fatalf("agent harness=%q", cfg.Agents["planning_agent"].Harness)
	}
}

func TestInjectHarnessForNew_MultipleIncludingCursor(t *testing.T) {
	cfg := &workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"planning_agent": {Model: "composer-2.5"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Model: "composer-2.5"},
	}
	workflowconfig.InjectHarnessForNew(cfg, []string{"opencode", "codex", "cursor"})
	if cfg.Agents["planning_agent"].Harness != "cursor" {
		t.Fatalf("want cursor when Cursor enabled among many, got %q", cfg.Agents["planning_agent"].Harness)
	}
}

func TestInjectHarnessForNew_CursorAndOpenCodeUnchanged(t *testing.T) {
	cfg := &workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"planning_agent": {Model: "composer-2.5"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Model: "composer-2.5"},
	}
	workflowconfig.InjectHarnessForNew(cfg, []string{"cursor", "opencode"})
	if cfg.Agents["planning_agent"].Harness != "cursor" {
		t.Fatalf("want cursor, got %q", cfg.Agents["planning_agent"].Harness)
	}
}

func TestInjectHarnessForNew_MultipleWithoutCursor(t *testing.T) {
	cfg := &workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"planning_agent": {Model: "gpt-5.4"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Model: "gpt-5.4"},
	}
	// Stable order mirrors install.ListEnabledHarnesses: opencode before codex.
	workflowconfig.InjectHarnessForNew(cfg, []string{"opencode", "codex"})
	if cfg.Agents["planning_agent"].Harness != "opencode" {
		t.Fatalf("want first enabled (opencode), got %q", cfg.Agents["planning_agent"].Harness)
	}
}

func TestInjectHarnessForNew_NeverDisabled(t *testing.T) {
	cfg := &workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"planning_agent": {Model: "composer-2.5"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Model: "composer-2.5"},
	}
	workflowconfig.InjectHarnessForNew(cfg, nil)
	if cfg.Agents["planning_agent"].Harness != "" {
		t.Fatalf("empty enabled must not inject, got %q", cfg.Agents["planning_agent"].Harness)
	}
	if cfg.FallbackModel.Harness != "" {
		t.Fatalf("empty enabled must not inject fallback, got %q", cfg.FallbackModel.Harness)
	}
}

func TestInjectHarnessForNew_PreservesExplicit(t *testing.T) {
	cfg := &workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"planning_agent": {Harness: "codex", Model: "gpt-5.4"},
			"qa_agent":       {Model: "composer-2.5"},
		},
		FallbackModel: workflowconfig.AgentModelConfig{Harness: "opencode", Model: "anthropic/claude-sonnet-4"},
	}
	workflowconfig.InjectHarnessForNew(cfg, []string{"cursor"})
	if cfg.Agents["planning_agent"].Harness != "codex" {
		t.Fatalf("explicit codex overwritten: %q", cfg.Agents["planning_agent"].Harness)
	}
	if cfg.FallbackModel.Harness != "opencode" {
		t.Fatalf("explicit opencode overwritten: %q", cfg.FallbackModel.Harness)
	}
	if cfg.Agents["qa_agent"].Harness != "cursor" {
		t.Fatalf("missing harness should inject cursor, got %q", cfg.Agents["qa_agent"].Harness)
	}
}
