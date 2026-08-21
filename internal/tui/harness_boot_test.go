package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestBootHarness_EnabledCursorNoPrompt(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, []string{"cursor"}, true)

	prompted := false
	deps := defaultHarnessBootDeps()
	deps.promptHarness = func(_ io.Writer) ([]string, error) {
		prompted = true
		return nil, errors.New("should not prompt")
	}
	deps.newRegistry = func(projectDir string, st *store.Store) harnessmgr.Registry {
		return &bootRegistry{adapter: &bootOKHarness{}}
	}
	deps.openStore = func(projectDir string) (*store.Store, error) {
		return store.OpenProject(projectDir)
	}

	result, err := bootHarness(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, dir, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if prompted {
		t.Fatal("expected no prompt when harness enabled")
	}
	if result.HarnessID == "" {
		t.Fatalf("result=%+v", result)
	}
}

func TestBootHarness_PromptsWhenNoneEnabled(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, nil, false)

	deps := defaultHarnessBootDeps()
	deps.promptHarness = func(_ io.Writer) ([]string, error) { return []string{"cursor"}, nil }
	deps.newRegistry = func(projectDir string, st *store.Store) harnessmgr.Registry {
		return &bootRegistry{adapter: &bootOKHarness{}}
	}
	deps.openStore = func(projectDir string) (*store.Store, error) {
		return store.OpenProject(projectDir)
	}

	_, err := bootHarness(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, dir, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	hero, _ := install.LoadHeroJSON(dir)
	if !install.IsHarnessEnabled(hero, "cursor") {
		t.Fatal("expected cursor enabled after prompt")
	}
}

func TestBootHarness_WarnsWhenUnavailable(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, []string{"cursor"}, true)

	deps := defaultHarnessBootDeps()
	deps.newRegistry = func(projectDir string, st *store.Store) harnessmgr.Registry {
		return &bootRegistry{adapter: &bootUnavailableHarness{}}
	}
	deps.openStore = func(projectDir string) (*store.Store, error) {
		return store.OpenProject(projectDir)
	}

	var stderr bytes.Buffer
	result, err := bootHarness(context.Background(), &bytes.Buffer{}, &stderr, dir, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AvailWarnings) == 0 {
		t.Fatal("expected availability warning")
	}
}

func TestBootHarness_OpenCodeDefaultNotFalseCatalogWarn(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := install.HeroJSON{
		CLI:       install.CLIInfo{Version: "2.0.0", Tools: []string{"cursor", "opencode"}},
		Assets:    install.AssetsInfo{Version: "2.0.0"},
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode"}),
		FreechatDefault: install.FreechatDefault{
			Harness: "opencode",
			Model:   "opencode-go/glm-5.3",
		},
	}
	data, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := defaultHarnessBootDeps()
	deps.newRegistry = func(projectDir string, st *store.Store) harnessmgr.Registry {
		return &bootRegistry{adapter: &bootOKHarness{}}
	}
	deps.listModels = func(ctx context.Context, reg harnessmgr.Registry, hero install.HeroJSON) ([]harnessmgr.ModelOption, error) {
		return []harnessmgr.ModelOption{{Model: "composer-2.5", Harness: "cursor"}}, nil
	}
	deps.openStore = func(projectDir string) (*store.Store, error) {
		return store.OpenProject(projectDir)
	}

	result, err := bootHarness(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, dir, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.ModelWarn != "" {
		t.Fatalf("unexpected model warn for opencode default: %q", result.ModelWarn)
	}
	if result.ModelSlug != "opencode-go/glm-5.3" || result.HarnessID != "opencode" {
		t.Fatalf("result=%+v", result)
	}
}

func TestValidateBootDefaultModel(t *testing.T) {
	catalog := []harnessmgr.ModelOption{{Model: "composer-2.5", Harness: "cursor"}}
	if got := validateBootDefaultModel("cursor", "composer-2.5", catalog); got != "" {
		t.Fatalf("valid cursor pair: %q", got)
	}
	if got := validateBootDefaultModel("cursor", "missing", catalog); got == "" {
		t.Fatal("expected warn for missing cursor model")
	}
	if got := validateBootDefaultModel("opencode", "opencode-go/glm-5.3", catalog); got != "" {
		t.Fatalf("opencode default must not validate against cursor-only catalog: %q", got)
	}
	if got := validateBootDefaultModel("codex", "gpt-5.4", catalog); got != "" {
		t.Fatalf("codex default must not validate against cursor-only catalog: %q", got)
	}
}

func TestBootHarness_CodexEnabledWarnsWhenUnavailable(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, []string{"codex"}, true)

	deps := defaultHarnessBootDeps()
	deps.newRegistry = func(projectDir string, st *store.Store) harnessmgr.Registry {
		return &bootRegistry{adapter: &bootUnavailableCodexHarness{}}
	}
	deps.openStore = func(projectDir string) (*store.Store, error) {
		return store.OpenProject(projectDir)
	}
	deps.listModels = func(ctx context.Context, reg harnessmgr.Registry, hero install.HeroJSON) ([]harnessmgr.ModelOption, error) {
		return nil, nil
	}

	var stderr bytes.Buffer
	result, err := bootHarness(context.Background(), &bytes.Buffer{}, &stderr, dir, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Enabled) != 1 || result.Enabled[0] != "codex" {
		t.Fatalf("enabled=%v", result.Enabled)
	}
	if len(result.AvailWarnings) == 0 {
		t.Fatal("expected availability warning (enabled ≠ available)")
	}
	if !strings.Contains(stderr.String(), "Codex") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestBootHarness_ReapsCodexOrphans(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, []string{"cursor"}, true)
	reaped := false
	deps := defaultHarnessBootDeps()
	deps.reapOrphans = func(ctx context.Context, projectDir string, st *store.Store) error {
		reaped = true
		if projectDir != dir {
			t.Fatalf("projectDir=%q", projectDir)
		}
		if st == nil {
			t.Fatal("expected store")
		}
		return nil
	}
	deps.newRegistry = func(projectDir string, st *store.Store) harnessmgr.Registry {
		return &bootRegistry{adapter: &bootOKHarness{}}
	}
	deps.openStore = func(projectDir string) (*store.Store, error) {
		return store.OpenProject(projectDir)
	}

	if _, err := bootHarness(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, dir, nil, deps); err != nil {
		t.Fatal(err)
	}
	if !reaped {
		t.Fatal("expected orphan reap on TUI boot")
	}
}

func (r *bootRegistry) SupportedIDs() []string { return []string{"cursor", "opencode", "codex"} }

type bootRegistry struct {
	adapter harness.HarnessAdapter
}

func (r *bootRegistry) Adapter(id string) (harness.HarnessAdapter, error) {
	if r.adapter != nil {
		return r.adapter, nil
	}
	return nil, errors.New("no adapter")
}
func (r *bootRegistry) EnabledIDs(install.HeroJSON) []string {
	return []string{"cursor"}
}

type bootOKHarness struct{}

func (bootOKHarness) Name() string                      { return "cursor" }
func (bootOKHarness) IsAvailable(context.Context) error { return nil }
func (bootOKHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return &harness.Session{ID: "s1"}, nil
}
func (bootOKHarness) ResumeSession(context.Context, string) error { return nil }
func (bootOKHarness) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return &harness.ExecutionResult{}, nil
}
func (bootOKHarness) Cancel(context.Context, string) error { return nil }
func (bootOKHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{}, nil
}
func (bootOKHarness) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}

type bootUnavailableHarness struct{}

func (bootUnavailableHarness) Name() string { return "cursor" }
func (bootUnavailableHarness) IsAvailable(context.Context) error {
	return errors.New("cursor agent CLI not found on PATH")
}
func (bootUnavailableHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, nil
}
func (bootUnavailableHarness) ResumeSession(context.Context, string) error { return nil }
func (bootUnavailableHarness) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, nil
}
func (bootUnavailableHarness) Cancel(context.Context, string) error { return nil }
func (bootUnavailableHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return nil, nil
}
func (bootUnavailableHarness) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}

type bootUnavailableCodexHarness struct{}

func (bootUnavailableCodexHarness) Name() string { return "codex" }
func (bootUnavailableCodexHarness) IsAvailable(context.Context) error {
	return errors.New("codex CLI not found on PATH")
}
func (bootUnavailableCodexHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, nil
}
func (bootUnavailableCodexHarness) ResumeSession(context.Context, string) error { return nil }
func (bootUnavailableCodexHarness) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, nil
}
func (bootUnavailableCodexHarness) Cancel(context.Context, string) error { return nil }
func (bootUnavailableCodexHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return nil, nil
}
func (bootUnavailableCodexHarness) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}

func writeHeroJSONForBootTest(t *testing.T, tools []string, withEnabled bool) string {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := install.HeroJSON{
		CLI: install.CLIInfo{
			Version: "2.0.0",
			Tools:   tools,
		},
		Assets: install.AssetsInfo{Version: "2.0.0"},
	}
	if withEnabled && len(tools) > 0 {
		hero.Harnesses = install.HarnessesFromSelection(tools)
		hero.FreechatDefault = install.DefaultFreechatDefault(hero.Harnesses)
	}
	data, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dir, "") {
		_ = cursoradapter.HeroJSONPath
	}
	return dir
}
