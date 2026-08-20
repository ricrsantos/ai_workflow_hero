package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/spf13/cobra"
)

// LaunchRefusal describes why the TUI cannot start.
type LaunchRefusal struct {
	Description string
	Suggestion  string
}

func (r LaunchRefusal) Error() string {
	return r.Description
}

// CanLaunch reports whether the TUI may start for the given stdout writer.
func CanLaunch(stdout io.Writer) *LaunchRefusal {
	if os.Getenv("NO_COLOR") != "" {
		return &LaunchRefusal{
			Description: "interactive TUI requires terminal colors.",
			Suggestion:  "Unset NO_COLOR and run `hero` in an interactive terminal, or use `hero status` / `hero metrics` for plain output.",
		}
	}
	if !clierr.IsTerminal(stdout) {
		return &LaunchRefusal{
			Description: "interactive TUI requires a terminal (TTY).",
			Suggestion:  "Run `hero` in an interactive terminal, or use `hero status --json` for scripting.",
		}
	}
	return nil
}

// Run launches the Bubble Tea application using an opened cycle service.
func Run(svc *cycle.Service) error {
	return RunWithChat(svc, nil, "", "", "")
}

// RunWithChat launches the TUI with optional harness model catalog from boot.
func RunWithChat(svc *cycle.Service, models []harnessmgr.ModelOption, modelSlug, harnessID, modelWarn string) error {
	if svc == nil {
		return fmt.Errorf("cycle service is nil")
	}
	// slog default writes stderr and corrupts the alt-screen TUI; redirect while running.
	restoreLog := redirectSlogForTUI(svc.ProjectDir)
	defer restoreLog()

	slog.Info("starting hero tui")

	var stopOnce sync.Once
	stopServe := func() {
		stopOnce.Do(func() {
			if svc != nil {
				var st *store.Store
				if svc.Store != nil {
					st = svc.Store
				}
				if err := stopOpenCodeServe(context.Background(), svc.ProjectDir, st, svc.Registry); err != nil {
					slog.Warn("stop opencode serve on tui exit failed", "error", err)
				}
			}
		})
	}
	defer stopServe()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigCh)
	go func() {
		sig, ok := <-sigCh
		if !ok {
			return
		}
		slog.Info("signal received, stopping managed opencode serve", "signal", sig.String())
		stopServe()
	}()

	m := newModelWithChat(svc, models, modelSlug, harnessID, modelWarn)
	err := opencodeadapter.RunServeWatchdog(context.Background(), svc.ProjectDir, svc.Store, func() error {
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, runErr := p.Run()
		return runErr
	})
	if err != nil {
		slog.Error("tui exited with error", "error", err)
		return err
	}
	slog.Info("hero tui closed")
	return nil
}

// redirectSlogForTUI sends slog to .workflow-hero/tui.log (or Discard) so INFO/ERROR
// lines do not paint over the Bubble Tea frame.
func redirectSlogForTUI(projectDir string) func() {
	prev := slog.Default()
	if projectDir == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() { slog.SetDefault(prev) }
	}
	dir := filepath.Join(projectDir, ".workflow-hero")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() { slog.SetDefault(prev) }
	}
	f, err := os.OpenFile(filepath.Join(dir, "tui.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() { slog.SetDefault(prev) }
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return func() {
		slog.SetDefault(prev)
		_ = f.Close()
	}
}

// RunDefault is the shared entry for `hero` (no args) and `hero tui`.
// It refuses to start when Hero is not installed in the project and ensures
// the operational SQLite store exists automatically when it is.
func RunDefault(stdout, stderr io.Writer) error {
	if refusal := CanLaunch(stdout); refusal != nil {
		e := clierr.NewWithSuggestion(refusal.Description, refusal.Suggestion)
		clierr.Format(stderr, e)
		return e
	}
	svc, err := cycle.OpenService("")
	if err != nil {
		if errors.Is(err, cycle.ErrNotInstalled) {
			e := clierr.NewWithSuggestion(
				err.Error(),
				"Install Hero in this project first, then run `hero` again to open the TUI.",
			)
			clierr.Format(stderr, e)
			return e
		}
		e := clierr.New(err.Error())
		clierr.Format(stderr, e)
		return e
	}
	defer svc.Close()

	adapter, err := bootHarness(context.Background(), stdout, stderr, svc.ProjectDir, svc.Store, defaultHarnessBootDeps())
	if err != nil {
		if _, ok := err.(*harnessBootError); ok {
			e := clierr.New("harness validation failed")
			return e
		}
		e := clierr.New(err.Error())
		clierr.Format(stderr, e)
		return e
	}
	if adapter.Registry != nil {
		svc.Registry = adapter.Registry
	}

	if err := RunWithChat(svc, adapter.Models, adapter.ModelSlug, adapter.HarnessID, adapter.ModelWarn); err != nil {
		e := clierr.New(err.Error())
		clierr.Format(stderr, e)
		return e
	}
	return nil
}

// NewCommand returns the `hero tui` command (alias of the default `hero` entry).
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "tui",
		Short:         "Launch the Hero interactive terminal UI (same as running `hero` with no arguments)",
		Long:          `Launch a keyboard-driven Bubble Tea UI for cycle status, approvals, artifacts, costs, and events. Equivalent to running hero with no subcommand.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunDefault(cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
}
