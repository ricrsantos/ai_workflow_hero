package opencode_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestSyncAgentDefinitionUpdatesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(agentsDir, "backend_agent.md")
	initial := `---
name: backend_agent
description: backend work
model: old/model
reasoningEffort: low
thinking: true
---
# body
`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := workflowconfig.AgentModelConfig{
		Harness:         "opencode",
		Model:           "opencode-go/gpt-5.6-luna",
		ReasoningEffort: "max",
		Thinking:        "false",
		EnableFastModel: true,
	}
	if err := opencode.SyncAgentDefinition(dir, "backend_agent", cfg); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		"model: opencode-go/gpt-5.6-luna",
		"reasoningEffort: max",
		"thinking: false",
		"fast: true",
		"name: backend_agent",
		"description: backend work",
		"# body",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "old/model") {
		t.Fatalf("stale model remained: %s", text)
	}
	if strings.Contains(text, "thinking: true") {
		t.Fatalf("stale thinking remained: %s", text)
	}
}

func TestSyncAgentDefinitionOmitsNAProperties(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(agentsDir, "qa_agent.md")
	initial := `---
name: qa_agent
description: qa
model: old/model
reasoningEffort: max
thinking: false
fast: true
---
`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := workflowconfig.AgentModelConfig{
		Harness:         "opencode",
		Model:           "opencode/deepseek-v4-flash-free",
		ReasoningEffort: "na",
		Thinking:        "na",
		EnableFastModel: false,
	}
	if err := opencode.SyncAgentDefinition(dir, "qa_agent", cfg); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if strings.Contains(text, "reasoningEffort:") {
		t.Fatalf("na reasoning_effort must be omitted: %s", text)
	}
	if strings.Contains(text, "thinking:") {
		t.Fatalf("na thinking must be omitted: %s", text)
	}
	if strings.Contains(text, "fast:") {
		t.Fatalf("disabled fast must be omitted: %s", text)
	}
}

func TestAgentsUsingHarnessSorted(t *testing.T) {
	cfg := workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"qa_agent":            {Harness: "opencode"},
			"backend_agent":       {Harness: "opencode"},
			"orchestration_agent": {Harness: "cursor"},
		},
	}
	got := opencode.AgentsUsingHarness(cfg, "opencode")
	want := []string{"backend_agent", "qa_agent"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	_ = install.OpenCodePathsFor(t.TempDir())
}
