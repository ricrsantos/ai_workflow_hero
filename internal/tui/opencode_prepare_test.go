package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestPrepareCodexOnStartSkipsMockRegistryAdapter(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeMixedDiscoverAgentYAML(t, dir)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": true})
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"codex":  &streamingHarness{deltas: []string{"ok"}},
		"cursor": &streamingHarness{deltas: []string{"ok"}},
	}}

	if err := prepareCodexOnStart(context.Background(), dir, svc.Store, svc.Registry); err != nil {
		t.Fatalf("mock Codex adapter must skip app-server prepare, got %v", err)
	}
}

func TestPrepareOpenCodeOnStartSkipsMockRegistryAdapter(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "opencode": true})
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"opencode": &streamingHarness{deltas: []string{"ok"}},
		"cursor":   &streamingHarness{deltas: []string{"ok"}},
	}}

	if err := prepareOpenCodeOnStart(context.Background(), dir, svc.Store, svc.Registry); err != nil {
		t.Fatalf("mock OpenCode adapter must skip serve prepare, got %v", err)
	}
}

func TestPrepareCodexOnStartFallsBackWithoutRegistry(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": false})
	if err := prepareCodexOnStart(context.Background(), dir, svc.Store, nil); err != nil {
		t.Fatalf("no-op prepare without registry: %v", err)
	}
}

func TestPrepareClaudeOnStartSkipsMockRegistryAdapter(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "claude": true})
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"claude": &streamingHarness{deltas: []string{"ok"}},
		"cursor": &streamingHarness{deltas: []string{"ok"}},
	}}

	if err := prepareClaudeOnStart(context.Background(), dir, svc.Registry); err != nil {
		t.Fatalf("mock Claude adapter must not run a live CLI probe, got %v", err)
	}
}

func TestHeroStartNeedsClaudePrepareOnlyForEnabledClaudeAgents(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "claude": true})
	config := []byte(`agents:
  generic_agent:
    harness: claude
    model: sonnet
fallback_model:
  harness: cursor
  model: composer-2.5
`)
	path := filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	if err := os.WriteFile(path, config, 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	if !m.heroStartNeedsClaudePrepare() {
		t.Fatal("enabled Claude workflow agent must request asynchronous prepare")
	}
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "claude": false})
	if m.heroStartNeedsClaudePrepare() {
		t.Fatal("disabled Claude must not request prepare")
	}
}
