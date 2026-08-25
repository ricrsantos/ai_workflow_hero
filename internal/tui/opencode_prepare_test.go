package tui

import (
	"context"
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
