package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
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
	if svc == nil {
		return fmt.Errorf("cycle service is nil")
	}
	slog.Info("starting hero tui")
	m := newModel(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		slog.Error("tui exited with error", "error", err)
		return err
	}
	slog.Info("hero tui closed")
	return nil
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

	adapter, err := bootHarness(context.Background(), stdout, stderr, svc.ProjectDir, defaultHarnessBootDeps())
	if err != nil {
		if _, ok := err.(*harnessBootError); ok {
			e := clierr.New("harness validation failed")
			return e
		}
		e := clierr.New(err.Error())
		clierr.Format(stderr, e)
		return e
	}
	svc.Harness = adapter

	if err := Run(svc); err != nil {
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
