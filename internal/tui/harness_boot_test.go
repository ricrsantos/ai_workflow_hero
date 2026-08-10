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
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestBootHarness_PersistsCursorOnFirstLaunch(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, nil)

	var adapter harness.HarnessAdapter = &bootOKHarness{}
	deps := defaultHarnessBootDeps()
	deps.promptTool = func(_ io.Writer, _ string) (string, error) { return "cursor", nil }
	deps.newAdapter = func(_ string, toolID string) (harness.HarnessAdapter, error) {
		if toolID != "cursor" {
			t.Fatalf("tool = %q", toolID)
		}
		return adapter, nil
	}
	deps.versionLabel = func(_, _ string) string { return "cursor-agent v1.2.3" }

	var stdout bytes.Buffer
	result, err := bootHarness(context.Background(), &stdout, &bytes.Buffer{}, dir, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.Adapter != adapter {
		t.Fatal("expected injected adapter")
	}
	if !strings.Contains(stdout.String(), "Cursor harness ready") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	hero, err := readHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(hero.CLI.Tools) != 1 || hero.CLI.Tools[0] != "cursor" {
		t.Fatalf("cli.tools = %v", hero.CLI.Tools)
	}
}

func TestBootHarness_SkipsPromptWhenConfigured(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, []string{"cursor"})

	prompted := false
	deps := defaultHarnessBootDeps()
	deps.promptTool = func(_ io.Writer, _ string) (string, error) {
		prompted = true
		return "", errors.New("should not prompt")
	}
	deps.newAdapter = func(_ string, toolID string) (harness.HarnessAdapter, error) {
		return &bootOKHarness{}, nil
	}

	_, err := bootHarness(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, dir, deps)
	if err != nil {
		t.Fatal(err)
	}
	if prompted {
		t.Fatal("expected no prompt when cli.tools is set")
	}
}

func TestBootHarness_ValidationFailureAbortsWithLoginHint(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, []string{"cursor"})

	deps := defaultHarnessBootDeps()
	deps.newAdapter = func(_ string, _ string) (harness.HarnessAdapter, error) {
		return &bootAuthFailHarness{}, nil
	}

	var stderr bytes.Buffer
	_, err := bootHarness(context.Background(), &bytes.Buffer{}, &stderr, dir, deps)
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if _, ok := err.(*harnessBootError); !ok {
		t.Fatalf("err type %T", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "Cursor harness unavailable") {
		t.Fatalf("stderr = %q", out)
	}
	if !strings.Contains(out, cursoradapter.LoginHint) {
		t.Fatalf("missing login hint: %q", out)
	}
	if !strings.Contains(out, "Then start Hero again: hero") {
		t.Fatalf("missing restart hint: %q", out)
	}
}

func TestBootHarness_CLIUnavailableAborts(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, []string{"cursor"})

	deps := defaultHarnessBootDeps()
	deps.newAdapter = func(_ string, _ string) (harness.HarnessAdapter, error) {
		return &bootUnavailableHarness{}, nil
	}

	var stderr bytes.Buffer
	_, err := bootHarness(context.Background(), &bytes.Buffer{}, &stderr, dir, deps)
	if err == nil {
		t.Fatal("expected failure")
	}
	if strings.Contains(stderr.String(), cursoradapter.LoginHint) {
		t.Fatalf("should not suggest login for missing CLI: %q", stderr.String())
	}
}

func TestBootHarness_ListsModelsAndWarnsWhenMissing(t *testing.T) {
	dir := writeHeroJSONForBootTest(t, []string{"cursor"})
	// Seed harnesses.cursor.model to something not in the catalog.
	hero, err := readHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	hero.Harnesses = map[string]install.HarnessConfig{
		"cursor": {Model: "missing-model", EnableFastModel: false},
	}
	encoded, _ := json.MarshalIndent(hero, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, cursoradapter.HeroJSONPath), append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := defaultHarnessBootDeps()
	deps.newAdapter = func(_ string, _ string) (harness.HarnessAdapter, error) {
		return &bootOKHarness{}, nil
	}
	deps.listModels = func(_ context.Context, _ harness.HarnessAdapter) ([]string, error) {
		return []string{"composer-2.5", "auto"}, nil
	}

	result, err := bootHarness(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, dir, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 2 {
		t.Fatalf("models=%v", result.Models)
	}
	if result.ModelSlug != "missing-model" {
		t.Fatalf("slug=%q", result.ModelSlug)
	}
	if result.ModelWarn == "" {
		t.Fatal("expected model warn")
	}
}

func TestCursorSelectLabel_DetectsCursorDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := cursorSelectLabel(dir)
	if got != "cursor (detected: .cursor/)" {
		t.Fatalf("label = %q", got)
	}
}

func TestNonEmptyTools_FiltersBlanks(t *testing.T) {
	got := nonEmptyTools([]string{"", "cursor", "  "})
	if len(got) != 1 || got[0] != "cursor" {
		t.Fatalf("got %v", got)
	}
}

type bootOKHarness struct{}

func (bootOKHarness) Name() string { return "cursor" }
func (bootOKHarness) IsAvailable(context.Context) error { return nil }
func (bootOKHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, errors.New("not implemented")
}
func (bootOKHarness) ResumeSession(context.Context, string) error { return nil }
func (bootOKHarness) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, nil
}
func (bootOKHarness) Cancel(context.Context, string) error { return nil }
func (bootOKHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return nil, nil
}
func (bootOKHarness) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}

type bootAuthFailHarness struct{}

func (bootAuthFailHarness) Name() string { return "cursor" }
func (bootAuthFailHarness) IsAvailable(context.Context) error {
	return &cursoradapter.AuthError{Detail: "authentication required"}
}
func (bootAuthFailHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, nil
}
func (bootAuthFailHarness) ResumeSession(context.Context, string) error { return nil }
func (bootAuthFailHarness) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, nil
}
func (bootAuthFailHarness) Cancel(context.Context, string) error { return nil }
func (bootAuthFailHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return nil, nil
}
func (bootAuthFailHarness) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
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

func writeHeroJSONForBootTest(t *testing.T, tools []string) string {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := install.HeroJSON{
		CLI: install.CLIInfo{
			Version: "1.0.0",
			Tools:   tools,
		},
		Assets: install.AssetsInfo{Version: "1.0.0"},
	}
	data, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
