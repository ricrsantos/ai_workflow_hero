package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/tui"
	"github.com/ricrsantos/ai_workflow_hero/internal/upgrade"
)

func TestIntegration_UpgradeLegacyHeroJSON_CursorOnlyEnabled(t *testing.T) {
	dir := makeGitRepo(t)
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{
  "cli": {"version": "1.0.0", "installedAt": "2026-01-01T00:00:00Z", "tools": ["cursor"]},
  "assets": {"version": "1.0.0", "installedAt": "2026-01-01T00:00:00Z"},
  "harnesses": {"cursor": {"model": "composer-2.5", "enable_fast_model": false}}
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "checksums.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{
		filepath.Join(dir, cursoradapter.HeroTemplatesDir),
		filepath.Join(dir, cursoradapter.HeroModelsDir),
		filepath.Join(dir, cursoradapter.HeroDocsDir),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var sb strings.Builder
	if _, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "2.0.0",
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("upgrade: %v", err)
	}

	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(hero, "cursor") {
		t.Fatal("cursor should be enabled after 1.x upgrade")
	}
	if install.IsHarnessEnabled(hero, "opencode") {
		t.Fatal("opencode should stay disabled after 1.x upgrade")
	}
	if _, err := os.Stat(filepath.Join(dir, ".opencode")); !os.IsNotExist(err) {
		t.Fatal("upgrade must not auto-provision .opencode/")
	}
}

func TestIntegration_MixedHarnessFallbackResolve(t *testing.T) {
	reg := mixedHarnessRegistry{}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode"}),
	}
	pair, attempts, err := harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"cursor", "composer-2.5",
		"opencode", "anthropic/claude-sonnet-4")
	if err != nil {
		t.Fatal(err)
	}
	if pair.HarnessID != "opencode" {
		t.Fatalf("harness=%q", pair.HarnessID)
	}
	if len(attempts) != 1 {
		t.Fatalf("attempts=%v", attempts)
	}
}

// TestIntegration_MixedCursorOpenCodeCodexResolve covers C6 §8H.3: three-harness
// workflow-config resolution via mock adapters (registry wiring only).
func TestIntegration_MixedCursorOpenCodeCodexResolve(t *testing.T) {
	reg := mockThreeHarnessRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor":   availableAdapter{id: "cursor"},
		"opencode": availableAdapter{id: "opencode"},
		"codex":    unavailableAdapter{id: "codex"},
	}}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode", "codex"}),
	}
	enabled := install.ListEnabledHarnesses(hero)
	if len(enabled) != 3 {
		t.Fatalf("enabled=%v want cursor+opencode+codex", enabled)
	}

	pair, attempts, err := harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"codex", "gpt-5.4",
		"opencode", "opencode/deepseek-v4-flash-free")
	if err != nil {
		t.Fatal(err)
	}
	if pair.HarnessID != "opencode" || pair.Model != "opencode/deepseek-v4-flash-free" {
		t.Fatalf("pair=%+v", pair)
	}
	if len(attempts) != 1 || attempts[0].HarnessID != "codex" {
		t.Fatalf("attempts=%v", attempts)
	}

	// Preferred Cursor still wins when available (Codex not in this pair).
	pair, attempts, err = harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"cursor", "composer-2.5",
		"codex", "gpt-5.4")
	if err != nil {
		t.Fatal(err)
	}
	if pair.HarnessID != "cursor" || len(attempts) != 0 {
		t.Fatalf("pair=%+v attempts=%v", pair, attempts)
	}
}

type mixedHarnessRegistry struct{}

func (mixedHarnessRegistry) Adapter(id string) (harness.HarnessAdapter, error) {
	switch id {
	case "cursor":
		return unavailableAdapter{id: "cursor"}, nil
	case "opencode":
		return availableAdapter{id: "opencode"}, nil
	case "codex":
		return unavailableAdapter{id: "codex"}, nil
	default:
		return nil, harnessmgrErr("unknown")
	}
}

// mockThreeHarnessRegistry is an injectable registry for Cursor+OpenCode+Codex tests.
type mockThreeHarnessRegistry struct {
	adapters map[string]harness.HarnessAdapter
}

func (r mockThreeHarnessRegistry) Adapter(id string) (harness.HarnessAdapter, error) {
	a, ok := r.adapters[id]
	if !ok {
		return nil, harnessmgrErr("unknown")
	}
	return a, nil
}

func (r mockThreeHarnessRegistry) EnabledIDs(hero install.HeroJSON) []string {
	return install.ListEnabledHarnesses(hero)
}

func (r mockThreeHarnessRegistry) SupportedIDs() []string {
	return install.SupportedHarnessIDs
}

func (mixedHarnessRegistry) EnabledIDs(hero install.HeroJSON) []string {
	return install.ListEnabledHarnesses(hero)
}

func (mixedHarnessRegistry) SupportedIDs() []string {
	return install.SupportedHarnessIDs
}

type harnessmgrErr string

func (e harnessmgrErr) Error() string { return string(e) }

type unavailableAdapter struct{ id string }

func (a unavailableAdapter) Name() string { return a.id }
func (a unavailableAdapter) IsAvailable(context.Context) error {
	return harnessmgrErr("unavailable")
}
func (a unavailableAdapter) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, nil
}
func (a unavailableAdapter) ResumeSession(context.Context, string) error { return nil }
func (a unavailableAdapter) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, nil
}
func (a unavailableAdapter) Cancel(context.Context, string) error { return nil }
func (a unavailableAdapter) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{}, nil
}
func (a unavailableAdapter) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}

type availableAdapter struct{ id string }

func (a availableAdapter) Name() string                      { return a.id }
func (a availableAdapter) IsAvailable(context.Context) error { return nil }
func (a availableAdapter) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return &harness.Session{ID: "s"}, nil
}
func (a availableAdapter) ResumeSession(context.Context, string) error { return nil }
func (a availableAdapter) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return &harness.ExecutionResult{}, nil
}
func (a availableAdapter) Cancel(context.Context, string) error { return nil }
func (a availableAdapter) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{}, nil
}
func (a availableAdapter) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}

func TestIntegration_OrphanServeRegistryReap(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := st.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:   "opencode",
		PID:       999999,
		Port:      4096,
		URL:       "http://127.0.0.1:4096",
		CreatedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	if err := tui.ReapOpenCodeOrphansForTest(context.Background(), dir, st); err != nil {
		t.Fatal(err)
	}
	entries, err := st.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected orphan reap to clear registry, got %v", entries)
	}
}

func TestIntegration_OpenspecChangePersisted(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "2.0.0")
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.PrepareCycle(); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("hero-2-0-multi-harness"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.OpenspecChange != "hero-2-0-multi-harness" {
		t.Fatalf("openspec_change=%q", st.OpenspecChange)
	}
}

func TestIntegration_HeroJSONHarnessStateAfterInstall(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "2.0.0")
	data, err := os.ReadFile(filepath.Join(dir, cursoradapter.HeroJSONPath))
	if err != nil {
		t.Fatal(err)
	}
	var hero install.HeroJSON
	if err := json.Unmarshal(data, &hero); err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(hero, "cursor") {
		t.Fatal("cursor not enabled")
	}
}

// TestIntegration_OpenCodePrepareUnchangedWithoutCodexAgents (C6 §8H.3): when
// Cursor+OpenCode+Codex are enabled but no agent uses harness:codex, OpenCode
// Prepare still syncs/probes and Codex Prepare remains a no-op.
func TestIntegration_OpenCodePrepareUnchangedWithoutCodexAgents(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	heroCfgDir := filepath.Join(dir, ".workflow-hero", "config")
	agentsDir := filepath.Join(dir, ".opencode", "agents")
	codexAgents := filepath.Join(dir, ".codex", "agents")
	for _, d := range []string{cfgDir, heroCfgDir, agentsDir, codexAgents} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), []byte(`title: mixed
objective: three harnesses enabled; agents use cursor+opencode only
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
  backend_agent:
    harness: opencode
    model: opencode-go/deepseek-v4-pro
fallback_model:
  harness: cursor
  model: composer-2.5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(heroCfgDir, "hero.json"), []byte(`{
  "harnesses": {
    "cursor": { "enabled": true, "model": "composer-2.5" },
    "opencode": { "enabled": true, "model": "opencode-go/deepseek-v4-pro" },
    "codex": { "enabled": true, "model": "gpt-5.4" }
  },
  "freechat_default": { "harness": "cursor", "model": "composer-2.5" }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "backend_agent.md"), []byte(`---
name: backend_agent
description: backend
model: stale/model
---
`), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(codexAgents, "should_not_touch.md")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var serveStarts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "sess-probe"})
		case "/session/sess-probe/message":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"parts": []map[string]string{{"type": "text", "text": "ok"}},
			})
		default:
			if r.URL.Path == "/config/providers" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := opencode.NewAdapter(dir, nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &integrationPrepareRunner{}
	a.HTTP = srv.Client()
	a.ResolveServeURL = func(opencode.ProcessHandle) (string, int, error) {
		atomic.AddInt32(&serveStarts, 1)
		return srv.URL, 1, nil
	}
	oldDelay := opencode.ServeResetDelayForTest()
	opencode.SetServeResetDelayForTest(0)
	t.Cleanup(func() { opencode.SetServeResetDelayForTest(oldDelay) })

	if err := opencode.PrepareHeroStartWithAdapter(context.Background(), dir, nil, a); err != nil {
		t.Fatalf("OpenCode Prepare: %v", err)
	}
	if atomic.LoadInt32(&serveStarts) < 1 {
		t.Fatal("expected OpenCode serve restart during Prepare")
	}
	data, err := os.ReadFile(filepath.Join(agentsDir, "backend_agent.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "model: opencode-go/deepseek-v4-pro") {
		t.Fatalf("OpenCode agent sync missing: %s", data)
	}

	if err := codex.PrepareHeroStart(context.Background(), dir, nil); err != nil {
		t.Fatalf("Codex Prepare no-op: %v", err)
	}
	kept, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "keep\n" {
		t.Fatalf("Codex Prepare must not mutate .codex/agents when unused: %q", kept)
	}
	entries, err := os.ReadDir(codexAgents)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "should_not_touch.md" {
		t.Fatalf("unexpected .codex/agents after no-op Prepare: %v", entries)
	}
}

type integrationPrepareRunner struct{}

func (integrationPrepareRunner) Start(context.Context, string, string, ...string) (opencode.ProcessHandle, error) {
	return integrationPrepareHandle{pid: 4242}, nil
}

type integrationPrepareHandle struct{ pid int }

func (h integrationPrepareHandle) PID() int    { return h.pid }
func (h integrationPrepareHandle) Wait() error { return nil }
func (h integrationPrepareHandle) Kill() error { return nil }

// TestIntegration_DefaultRegistryWiresThreeAdapters (C6 §8H.1–8H.2): registry
// resolves cursor/opencode/codex without changing adapter Execute contracts.
func TestIntegration_DefaultRegistryWiresThreeAdapters(t *testing.T) {
	reg := harnessmgr.NewRegistry(t.TempDir(), nil)
	ids := reg.SupportedIDs()
	if len(ids) != 3 {
		t.Fatalf("SupportedIDs=%v", ids)
	}
	for _, id := range []string{"cursor", "opencode", "codex"} {
		a, err := reg.Adapter(id)
		if err != nil {
			t.Fatalf("Adapter(%s): %v", id, err)
		}
		if a.Name() != id {
			t.Fatalf("Name()=%q want %q", a.Name(), id)
		}
	}
}
