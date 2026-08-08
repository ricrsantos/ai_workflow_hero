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
	"github.com/charmbracelet/lipgloss"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

// harnessBootDeps groups injectable dependencies for harness boot (tests).
type harnessBootDeps struct {
	readHeroJSON  func(projectDir string) (install.HeroJSON, error)
	writeCLITools func(projectDir string, tools []string) error
	promptTool    func(stdout io.Writer, projectDir string) (string, error)
	newAdapter    func(projectDir, toolID string) (harness.HarnessAdapter, error)
	versionLabel  func(projectDir, toolID string) string
}

func defaultHarnessBootDeps() harnessBootDeps {
	return harnessBootDeps{
		readHeroJSON:  readHeroJSON,
		writeCLITools: writeCLITools,
		promptTool:    promptHarnessTool,
		newAdapter:    newHarnessAdapter,
		versionLabel:  cursorVersionLabel,
	}
}

// bootHarness selects (when needed), validates, and persists the harness before TUI start (design D5).
func bootHarness(ctx context.Context, stdout, stderr io.Writer, projectDir string, deps harnessBootDeps) (harness.HarnessAdapter, error) {
	hero, err := deps.readHeroJSON(projectDir)
	if err != nil {
		slog.Error("harness boot read hero.json failed", "error", err)
		return nil, fmt.Errorf("read hero.json: %w", err)
	}

	tools := nonEmptyTools(hero.CLI.Tools)
	selected := ""
	persist := false

	if len(tools) == 0 {
		output.Progress(stdout, "No harness configured for this project.")
		selected, err = deps.promptTool(stdout, projectDir)
		if err != nil {
			slog.Error("harness boot prompt failed", "error", err)
			return nil, err
		}
		persist = true
	} else {
		selected = tools[0]
	}

	adapter, err := deps.newAdapter(projectDir, selected)
	if err != nil {
		slog.Error("harness boot adapter init failed", "tool", selected, "error", err)
		return nil, err
	}

	if err := adapter.IsAvailable(ctx); err != nil {
		slog.Error("harness boot validation failed", "tool", selected, "error", err)
		formatHarnessBootFailure(stderr, selected, err)
		return nil, &harnessBootError{tool: selected, cause: err}
	}

	if persist {
		if err := deps.writeCLITools(projectDir, []string{selected}); err != nil {
			slog.Error("harness boot persist cli.tools failed", "error", err)
			return nil, fmt.Errorf("persist cli.tools: %w", err)
		}
		slog.Info("harness boot persisted cli.tools", "tool", selected)
	}

	if persist {
		label := deps.versionLabel(projectDir, selected)
		if label != "" {
			output.Successf(stdout, "Cursor harness ready (%s)", label)
		} else {
			output.Success(stdout, "Cursor harness ready")
		}
	}

	slog.Info("harness boot ready", "tool", selected, "adapter", adapter.Name())
	return adapter, nil
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
	default:
		if toolID == "" {
			return "Harness"
		}
		return toolID
	}
}

func nonEmptyTools(tools []string) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		if strings.TrimSpace(t) != "" {
			out = append(out, strings.TrimSpace(t))
		}
	}
	return out
}

func readHeroJSON(projectDir string) (install.HeroJSON, error) {
	path := filepath.Join(projectDir, cursoradapter.HeroJSONPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return install.HeroJSON{}, err
	}
	var hero install.HeroJSON
	if err := json.Unmarshal(data, &hero); err != nil {
		return install.HeroJSON{}, err
	}
	return hero, nil
}

func writeCLITools(projectDir string, tools []string) error {
	path := filepath.Join(projectDir, cursoradapter.HeroJSONPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var hero install.HeroJSON
	if err := json.Unmarshal(data, &hero); err != nil {
		return err
	}
	hero.CLI.Tools = tools
	encoded, err := json.MarshalIndent(hero, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

func newHarnessAdapter(projectDir, toolID string) (harness.HarnessAdapter, error) {
	switch toolID {
	case "cursor":
		return cursoradapter.NewAdapter(projectDir), nil
	default:
		return nil, fmt.Errorf("unsupported harness %q", toolID)
	}
}

func promptHarnessTool(_ io.Writer, projectDir string) (string, error) {
	var selected string
	label := cursorSelectLabel(projectDir)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select harness:").
				Description("[Only supported harness in V1]").
				Options(huh.NewOption(label, "cursor")).
				Value(&selected),
		),
	).WithTheme(harnessBootTheme())
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("harness selection cancelled: %w", err)
	}
	if strings.TrimSpace(selected) == "" {
		return "", fmt.Errorf("harness selection required")
	}
	return selected, nil
}

func cursorSelectLabel(projectDir string) string {
	det, err := harness.DetectMarkers(projectDir, nil)
	if err != nil {
		return "cursor"
	}
	for _, m := range det.Present {
		if m.ToolID == "cursor" {
			return "cursor (detected: .cursor/)"
		}
	}
	return "cursor"
}

func cursorVersionLabel(projectDir, toolID string) string {
	if toolID != "cursor" {
		return ""
	}
	spec, err := cursoradapter.ResolveAgentCLI(nil)
	if err != nil {
		return ""
	}
	adapter := cursoradapter.NewAdapter(projectDir)
	res, err := adapter.Runner.Run(context.Background(), projectDir, spec.Path, spec.BuildArgs("--version"))
	if err != nil {
		return ""
	}
	ver := strings.TrimSpace(string(res.Stdout))
	if ver == "" {
		ver = strings.TrimSpace(string(res.Stderr))
	}
	bin := cursoradapter.AgentCLI
	if len(spec.Base) > 0 {
		bin = cursoradapter.CursorCLI + " agent"
	}
	if ver == "" {
		return bin
	}
	return fmt.Sprintf("%s %s", bin, ver)
}

func harnessBootTheme() *huh.Theme {
	t := huh.ThemeBase()
	clean := lipgloss.NewStyle()
	t.Focused.Base = clean
	t.Focused.Card = clean
	t.Blurred.Base = clean
	t.Blurred.Card = clean
	t.Focused.Title = lipgloss.NewStyle()
	t.Blurred.Title = lipgloss.NewStyle()
	return t
}
