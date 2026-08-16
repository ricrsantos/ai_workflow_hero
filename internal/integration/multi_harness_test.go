package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
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

type mixedHarnessRegistry struct{}

func (mixedHarnessRegistry) Adapter(id string) (harness.HarnessAdapter, error) {
	switch id {
	case "cursor":
		return unavailableAdapter{id: "cursor"}, nil
	case "opencode":
		return availableAdapter{id: "opencode"}, nil
	default:
		return nil, harnessmgrErr("unknown")
	}
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
