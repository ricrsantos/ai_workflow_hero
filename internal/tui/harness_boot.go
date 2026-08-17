package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// harnessBootResult is the outcome of bootHarness before the Bubble Tea loop.
type harnessBootResult struct {
	Registry      harnessmgr.Registry
	Enabled       []string
	Models        []harnessmgr.ModelOption
	ModelSlug     string
	HarnessID     string
	ModelWarn     string
	AvailWarnings []string
}

type harnessBootDeps struct {
	readHeroJSON  func(projectDir string) (install.HeroJSON, error)
	writeHeroJSON func(projectDir string, hero install.HeroJSON) error
	promptHarness func(stdout io.Writer) ([]string, error)
	newRegistry   func(projectDir string, st *store.Store) harnessmgr.Registry
	reapOrphans   func(ctx context.Context, projectDir string, st *store.Store) error
	listModels    func(ctx context.Context, reg harnessmgr.Registry, hero install.HeroJSON) ([]harnessmgr.ModelOption, error)
	openStore     func(projectDir string) (*store.Store, error)
}

func defaultHarnessBootDeps() harnessBootDeps {
	return harnessBootDeps{
		readHeroJSON:  readHeroJSON,
		writeHeroJSON: writeHeroJSONFile,
		promptHarness: promptInstallLikeHarnesses,
		newRegistry: func(projectDir string, st *store.Store) harnessmgr.Registry {
			return harnessmgr.NewRegistry(projectDir, st)
		},
		reapOrphans: reapOpenCodeOrphans,
		listModels:  harnessmgr.ListModels,
		openStore:   store.OpenProject,
	}
}

func bootHarness(ctx context.Context, stdout, stderr io.Writer, projectDir string, deps harnessBootDeps) (harnessBootResult, error) {
	hero, err := deps.readHeroJSON(projectDir)
	if err != nil {
		slog.Error("harness boot read hero.json failed", "error", err)
		return harnessBootResult{}, fmt.Errorf("read hero.json: %w", err)
	}
	install.MigrateHarnessState(&hero)

	enabled := install.ListEnabledHarnesses(hero)
	if len(enabled) == 0 {
		output.Progress(stdout, "No harness configured for this project.")
		selected, err := deps.promptHarness(stdout)
		if err != nil {
			return harnessBootResult{}, err
		}
		for _, id := range selected {
			_ = install.SetHarnessEnabled(projectDir, id, true)
		}
		hero, _ = deps.readHeroJSON(projectDir)
		install.MigrateHarnessState(&hero)
		enabled = install.ListEnabledHarnesses(hero)
		if err := deps.writeHeroJSON(projectDir, hero); err != nil {
			return harnessBootResult{}, fmt.Errorf("persist harness selection: %w", err)
		}
	}

	st, err := deps.openStore(projectDir)
	if err != nil {
		return harnessBootResult{}, fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	if err := deps.reapOrphans(ctx, projectDir, st); err != nil {
		slog.Warn("orphan serve reap failed", "error", err)
	}

	reg := deps.newRegistry(projectDir, st)
	warnings := []string{}
	for _, id := range enabled {
		adapter, err := reg.Adapter(id)
		if err != nil {
			continue
		}
		if err := adapter.IsAvailable(ctx); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s harness unavailable: %s", harnessDisplayName(id), harnessUnavailableReason(err)))
			slog.Warn("harness boot unavailable", "harness", id, "error", err)
		}
	}
	if len(warnings) > 0 {
		for _, w := range warnings {
			output.Warning(stderr, w)
		}
		output.Progress(stderr, "Fix harness setup, then retry commands that need it.")
	}

	h, m := install.GetFreechatDefault(hero)
	models, listErr := deps.listModels(ctx, reg, hero)
	if listErr != nil {
		slog.Warn("harness boot list models failed", "error", listErr)
	}
	modelWarn := validateBootDefaultModel(h, m, models)

	slog.Info("harness boot ready", "enabled", enabled, "default_harness", h, "models", len(models))
	return harnessBootResult{
		Registry:      reg,
		Enabled:       enabled,
		Models:        models,
		ModelSlug:     m,
		HarnessID:     h,
		ModelWarn:     modelWarn,
		AvailWarnings: warnings,
	}, nil
}

func reapOpenCodeOrphans(ctx context.Context, projectDir string, st *store.Store) error {
	if st == nil {
		return nil
	}
	entries, err := st.ListServeRegistry()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Harness != "opencode" {
			continue
		}
		if !processIsOpenCodeServe(e.PID) {
			_ = st.DeleteServeRegistry(e.ID)
			continue
		}
		slog.Info("reaping orphan opencode serve", "pid", e.PID)
		adapter := opencodeadapter.NewAdapter(projectDir, st)
		_ = adapter.StopServe(ctx)
		_ = st.DeleteServeRegistry(e.ID)
	}
	return nil
}

func processIsOpenCodeServe(pid int) bool {
	if pid <= 0 {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	cmd := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.Contains(cmd, "opencode") && strings.Contains(cmd, "serve")
}

func writeHeroJSONFile(projectDir string, hero install.HeroJSON) error {
	path := filepath.Join(projectDir, cursoradapter.HeroJSONPath)
	encoded, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

func promptInstallLikeHarnesses(_ io.Writer) ([]string, error) {
	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select the AI Harnesses you want to use (at least one):").
				Options(
					huh.NewOption("Cursor", "cursor"),
					huh.NewOption("OpenCode", "opencode"),
				).
				Value(&selected).
				Validate(func(v []string) error {
					if len(v) == 0 {
						return fmt.Errorf("select at least one harness")
					}
					return nil
				}),
		),
	).WithTheme(harnessBootTheme())
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("harness selection cancelled: %w", err)
	}
	return selected, nil
}

func listHarnessModels(ctx context.Context, adapter harness.HarnessAdapter) ([]string, error) {
	lister, ok := adapter.(harness.ModelLister)
	if !ok {
		return nil, fmt.Errorf("harness %q does not support model listing", adapter.Name())
	}
	return lister.ListModels(ctx)
}

type harnessBootError struct {
	tool  string
	cause error
}

func (e *harnessBootError) Error() string {
	if e == nil || e.cause == nil {
		return "harness boot failed"
	}
	return e.cause.Error()
}

func (e *harnessBootError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func formatHarnessBootFailure(stderr io.Writer, tool string, err error) {
	reason := harnessUnavailableReason(err)
	name := harnessDisplayName(tool)
	if output.IsTerminal(stderr) {
		fmt.Fprintf(stderr, "\033[31m✗\033[0m %s harness unavailable: %s\n", name, reason)
	} else {
		fmt.Fprintf(stderr, "[ERROR] %s harness unavailable: %s\n", name, reason)
	}
	if isAuthHarnessError(err) {
		output.Progress(stderr, fmt.Sprintf("Run: %s", cursoradapter.LoginHint))
	}
	output.Progress(stderr, "Then start Hero again: hero")
}

func harnessUnavailableReason(err error) string {
	if err == nil {
		return "unknown error"
	}
	var auth *cursoradapter.AuthError
	if errors.As(err, &auth) {
		if strings.TrimSpace(auth.Detail) != "" {
			return auth.Detail
		}
		return "authentication required"
	}
	return strings.TrimSpace(err.Error())
}

func isAuthHarnessError(err error) bool {
	var auth *cursoradapter.AuthError
	return errors.As(err, &auth)
}

func harnessDisplayName(toolID string) string {
	switch toolID {
	case "cursor":
		return "Cursor"
	case "opencode":
		return "OpenCode"
	default:
		if toolID == "" {
			return "Harness"
		}
		return toolID
	}
}

func readHeroJSON(projectDir string) (install.HeroJSON, error) {
	return install.LoadHeroJSON(projectDir)
}

// validateBootDefaultModel checks the persisted freechat pair against the boot-time catalog.
// Aggregate ListModels skips OpenCode so boot does not start opencode serve (UI-C04-001 §7);
// OpenCode defaults are validated on demand via /hero-model and first Execute.
func validateBootDefaultModel(harnessID, model string, catalog []harnessmgr.ModelOption) string {
	model = strings.TrimSpace(model)
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if model == "" || harnessID == "" {
		return ""
	}
	if harnessID == "opencode" {
		return ""
	}
	if len(catalog) == 0 {
		return ""
	}
	for _, opt := range catalog {
		if opt.Model == model && strings.EqualFold(opt.Harness, harnessID) {
			return ""
		}
	}
	return fmt.Sprintf("configured model %q not in harness catalog", model)
}

func newHarnessAdapter(projectDir, toolID string) (harness.HarnessAdapter, error) {
	return harnessmgr.NewRegistry(projectDir, nil).Adapter(toolID)
}

func harnessBootTheme() *huh.Theme {
	t := huh.ThemeBase()
	return t
}
