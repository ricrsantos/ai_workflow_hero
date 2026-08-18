package workflowconfig

import (
	"os"
	"path/filepath"
	"testing"
)

const projectionYAML = `title: t
objective: t
agents:
  qa_agent:
    harness: opencode
    model: opencode-go/deepseek-v4-pro
    reasoning_effort: low
    thinking: "false"
    enable_fast_model: false
  orchestration_agent:
    harness: cursor
    model: composer-2.5
    enable_fast_model: true
fallback_model:
  harness: cursor
  model: grok-4.6
  enable_fast_model: false
`

func writeWorkflowConfig(t *testing.T, dir, content string) string {
	t.Helper()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "workflow-config.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEffectivePropertiesDerivesNormalizedKeys(t *testing.T) {
	props := EffectiveProperties(AgentModelConfig{
		Model:           "composer-2.5",
		EnableFastModel: true,
		Thinking:        "max",
		ReasoningEffort: "high",
	})
	if props["fs"] != "true" || props["th"] != "max" || props["ef"] != "high" {
		t.Fatalf("projection: %v", props)
	}
	// Fast explicitly off must stay visible as false — never empty.
	props = EffectiveProperties(AgentModelConfig{Model: "grok-4.6", EnableFastModel: false})
	if props["fs"] != "false" {
		t.Fatalf("fs must be false when the block disables fast: %v", props)
	}
	if _, ok := props["th"]; ok {
		t.Fatal("unset th must be absent")
	}
	// Empty block projects nothing.
	if props := EffectiveProperties(AgentModelConfig{}); props != nil {
		t.Fatalf("empty block must project nil: %v", props)
	}
}

func TestAgentPropertiesUsesAgentBlockThenFallback(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowConfig(t, dir, projectionYAML)

	props, usedFallback, err := AgentProperties(dir, "qa_agent")
	if err != nil {
		t.Fatal(err)
	}
	if usedFallback {
		t.Fatal("qa_agent block exists; fallback must not be used")
	}
	if props["ef"] != "low" || props["fs"] != "false" || props["th"] != "false" {
		t.Fatalf("qa_agent projection: %v", props)
	}

	props, usedFallback, err = AgentProperties(dir, "missing_agent")
	if err != nil {
		t.Fatal(err)
	}
	if !usedFallback {
		t.Fatal("missing agent must fall back to fallback_model")
	}
	if props["fs"] != "false" {
		t.Fatalf("fallback projection: %v", props)
	}
	if _, ok := props["ef"]; ok {
		t.Fatal("fallback has no reasoning_effort; ef must be absent")
	}
	_ = path
}

func TestWorkflowProjectionNeverTouchesFreechatJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowConfig(t, dir, projectionYAML)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// A freechat hero.json with conflicting properties must be ignored.
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	heroJSON := `{"freechat_default":{"harness":"opencode","model":"m"},"model_properties":{"opencode":{"m":{"ef":"high","fs":"true"}}}}`
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte(heroJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	props, _, err := AgentProperties(dir, "qa_agent")
	if err != nil {
		t.Fatal(err)
	}
	if props["ef"] != "low" {
		t.Fatalf("freechat ef=high must never override workflow ef=low: %v", props)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("stage YAML must be byte-for-byte unchanged")
	}
}

func TestAgentConfigForMatchesAgentPairFor(t *testing.T) {
	dir := t.TempDir()
	writeWorkflowConfig(t, dir, projectionYAML)

	pair, usedFallback, err := AgentPairFor(dir, "qa_agent")
	if err != nil {
		t.Fatal(err)
	}
	cfg, usedFallbackCfg, err := AgentConfigFor(dir, "qa_agent")
	if err != nil {
		t.Fatal(err)
	}
	if usedFallback != usedFallbackCfg {
		t.Fatal("fallback flags must match")
	}
	if cfg.Harness != pair.Harness || cfg.Model != pair.Model {
		t.Fatalf("config/pair mismatch: %+v vs %+v", cfg, pair)
	}
}
