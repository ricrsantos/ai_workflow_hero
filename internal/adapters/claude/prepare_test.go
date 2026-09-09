package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestPrepareHeroStartSyncsOnlyManagedClaudeFields(t *testing.T) {
	dir := setupClaudePrepareProject(t, map[string]string{
		"planning_agent": managedClaudeAgent("planning-agent user metadata", "PLANNING BODY"),
	})
	a, launcher := newTestAdapter(&fakeProcess{})
	if err := PrepareHeroStartWithAdapter(context.Background(), dir, a); err != nil {
		t.Fatal(err)
	}
	if launcher.starts != 0 {
		t.Fatalf("prepare started %d Claude turns; want none", launcher.starts)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "agents", "planning_agent.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"description: planning-agent user metadata",
		"custom: preserve-me",
		"model: sonnet",
		"effort: high",
		"  - workflow-hero",
		"PLANNING BODY",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prepared agent missing %q:\n%s", want, got)
		}
	}
}

func TestPrepareHeroStartFailsBeforeWritingOnCompatibilityOrMarkerError(t *testing.T) {
	t.Run("compatibility", func(t *testing.T) {
		dir := setupClaudePrepareProject(t, map[string]string{
			"planning_agent": managedClaudeAgent("preserve", "BODY"),
		})
		path := filepath.Join(dir, ".claude", "agents", "planning_agent.md")
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := newTestAdapter(&fakeProcess{})
		a.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
		err = PrepareHeroStartWithAdapter(context.Background(), dir, a)
		if err == nil || !strings.Contains(err.Error(), "compatibility probe failed") || !strings.Contains(err.Error(), "Exit Hero TUI") {
			t.Fatalf("err=%v", err)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatal("compatibility failure rewrote the Claude agent")
		}
	})

	t.Run("invalid second marker", func(t *testing.T) {
		dir := setupClaudePrepareProject(t, map[string]string{
			"planning_agent": managedClaudeAgent("preserve", "BODY"),
			"qa_agent":       "---\nname: qa_agent\nmodel: inherit\n---\nUSER BODY\n",
		})
		path := filepath.Join(dir, ".claude", "agents", "planning_agent.md")
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := newTestAdapter(&fakeProcess{})
		err = PrepareHeroStartWithAdapter(context.Background(), dir, a)
		if err == nil || !strings.Contains(err.Error(), `Claude agent "qa_agent" preparation failed`) {
			t.Fatalf("err=%v", err)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatal("invalid marked agent caused an earlier agent rewrite")
		}
	})
}

func TestAgentsUsingHarnessClaudeSorted(t *testing.T) {
	cfg := workflowconfig.ConfigFile{Agents: map[string]workflowconfig.AgentModelConfig{
		"qa_agent":       {Harness: "claude"},
		"planning_agent": {Harness: "claude"},
		"generic_agent":  {Harness: "codex"},
	}}
	got := AgentsUsingHarness(cfg, "claude")
	if strings.Join(got, ",") != "planning_agent,qa_agent" {
		t.Fatalf("agents=%v", got)
	}
}

func setupClaudePrepareProject(t *testing.T, agents map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude", "agents")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var config strings.Builder
	config.WriteString("agents:\n")
	for name := range agents {
		config.WriteString("  " + name + ":\n    harness: claude\n    model: sonnet\n    reasoning_effort: high\n")
	}
	config.WriteString("fallback_model:\n  harness: cursor\n  model: composer-2.5\n")
	configDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "workflow-config.yml"), []byte(config.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "config", "hero.json"), []byte(`{"harnesses":{"claude":{"enabled":true,"model":"sonnet"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range agents {
		if err := os.WriteFile(filepath.Join(claudeDir, name+".md"), []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func managedClaudeAgent(description, body string) string {
	return "---\nname: agent\ndescription: " + description + "\ncustom: preserve-me\n" +
		ManagedAgentFieldsBegin + "\nmodel: inherit\nskills:\n  - user-skill\n" + ManagedAgentFieldsEnd + "\n---\n" + body + "\n"
}
