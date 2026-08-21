package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestSyncAgentDefinitionUpdatesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".codex", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(agentsDir, "planning_agent.md")
	initial := `---
name: planning_agent
description: planning
model: stale-model
---
# body
`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := workflowconfig.AgentModelConfig{
		Harness:         "codex",
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		Thinking:        "true",
	}
	if err := codex.SyncAgentDefinition(dir, "planning_agent", cfg); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{"model: gpt-5.4", "reasoningEffort: high", "thinking: true"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestAgentsUsingHarnessSorted(t *testing.T) {
	cfg := workflowconfig.ConfigFile{
		Agents: map[string]workflowconfig.AgentModelConfig{
			"qa_agent":       {Harness: "codex", Model: "gpt-5.4"},
			"planning_agent": {Harness: "codex", Model: "gpt-5.4"},
			"backend_agent":  {Harness: "cursor", Model: "composer-2.5"},
		},
	}
	got := codex.AgentsUsingHarness(cfg, "codex")
	if len(got) != 2 || got[0] != "planning_agent" || got[1] != "qa_agent" {
		t.Fatalf("got=%v", got)
	}
}

func TestPrepareHeroStartSyncsResetsAndProbes(t *testing.T) {
	dir := setupCodexPrepareProject(t)
	peer := newMockPeer()
	var turns int32
	peer.onTurn = func(params map[string]any) {
		atomic.AddInt32(&turns, 1)
		if model, _ := params["model"].(string); model != "gpt-5.4" {
			t.Errorf("probe model=%v want gpt-5.4", params["model"])
		}
	}
	a := codex.NewAdapter(dir, nil)
	a.LookPath = func(string) (string, error) { return "codex", nil }
	a.Runner = peer

	old := codex.AppServerResetDelayForTest()
	codex.SetAppServerResetDelayForTest(0)
	t.Cleanup(func() { codex.SetAppServerResetDelayForTest(old) })

	if err := codex.PrepareHeroStartWithAdapter(context.Background(), dir, nil, a); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&turns) < 1 {
		t.Fatal("expected probe turn")
	}
	for _, agent := range []struct {
		name  string
		model string
	}{
		{"planning_agent", "gpt-5.4"},
		{"qa_agent", "gpt-5.3-codex"},
	} {
		data, err := os.ReadFile(filepath.Join(dir, ".codex", "agents", agent.name+".md"))
		if err != nil {
			t.Fatal(err)
		}
		want := "model: " + agent.model
		if !strings.Contains(string(data), want) {
			t.Fatalf("%s not synced (%s missing): %s", agent.name, want, data)
		}
	}
}

func TestPrepareHeroStartProbeFailure(t *testing.T) {
	dir := setupCodexPrepareProject(t)
	peer := newMockPeer()
	peer.authNil = true
	a := codex.NewAdapter(dir, nil)
	a.LookPath = func(string) (string, error) { return "codex", nil }
	a.Runner = peer

	old := codex.AppServerResetDelayForTest()
	codex.SetAppServerResetDelayForTest(0)
	t.Cleanup(func() { codex.SetAppServerResetDelayForTest(old) })

	err := codex.PrepareHeroStartWithAdapter(context.Background(), dir, nil, a)
	if err == nil {
		t.Fatal("expected probe failure")
	}
	msg := err.Error()
	for _, want := range []string{
		`codex agent "planning_agent" model probe failed`,
		"Exit Hero TUI",
		"run `hero` again",
		"retry /hero-start",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in error: %v", want, err)
		}
	}
}

func TestPrepareHeroStartNoCodexAgents(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), []byte(`title: t
objective: t
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
fallback_model:
  harness: cursor
  model: composer-2.5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := codex.PrepareHeroStart(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
}

func TestResetAppServerWaitsDelay(t *testing.T) {
	peer := newMockPeer()
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "codex", nil }
	a.Runner = peer

	old := codex.AppServerResetDelayForTest()
	codex.SetAppServerResetDelayForTest(50 * time.Millisecond)
	t.Cleanup(func() { codex.SetAppServerResetDelayForTest(old) })

	start := time.Now()
	if err := a.ResetAppServer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("reset too fast: %v", elapsed)
	}
}

func setupCodexPrepareProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	heroCfgDir := filepath.Join(dir, ".workflow-hero", "config")
	agentsDir := filepath.Join(dir, ".codex", "agents")
	for _, d := range []string{cfgDir, heroCfgDir, agentsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), []byte(`title: t
objective: t
agents:
  planning_agent:
    harness: codex
    model: gpt-5.4
  qa_agent:
    harness: codex
    model: gpt-5.3-codex
fallback_model:
  harness: cursor
  model: composer-2.5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(heroCfgDir, "hero.json"), []byte(`{
  "harnesses": {
    "codex": { "enabled": true, "model": "gpt-5.4" },
    "cursor": { "enabled": true, "model": "composer-2.5" }
  },
  "freechat_default": { "harness": "codex", "model": "gpt-5.4" }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "planning_agent.md"), []byte(`---
name: planning_agent
description: planning
model: stale-model
---
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "qa_agent.md"), []byte(`---
name: qa_agent
description: qa
model: stale-model
---
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
