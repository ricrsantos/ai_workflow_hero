package main

import (
	"os"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/doctor"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/status"
	"github.com/ricrsantos/ai_workflow_hero/internal/tui"
	"github.com/ricrsantos/ai_workflow_hero/internal/uninstall"
	"github.com/ricrsantos/ai_workflow_hero/internal/update_models"
	"github.com/ricrsantos/ai_workflow_hero/internal/upgrade"
	"github.com/ricrsantos/ai_workflow_hero/internal/variables"
	"github.com/spf13/cobra"
)

// version is injected at build time via -ldflags "-X main.version=<tag>".
var version = "1.0.5"

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		// clierr.HeroError has already been formatted and printed in RunE.
		// For any other unexpected error, print a generic message.
		if _, ok := err.(*clierr.HeroError); !ok {
			clierr.Format(os.Stderr, clierr.New(err.Error()))
		}
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var (
		verbose bool
		debug   bool
	)

	root := &cobra.Command{
		Use:   "hero",
		Short: "AI Workflow Hero — framework for AI-augmented software development",
		Long: `Hero is an open-source framework for AI-augmented software development.

It bootstraps a project with commands, skills, prompts, and templates for
Cursor AI, enabling reproducible, multi-agent development cycles.

Running hero with no arguments opens the interactive TUI (requires Hero
installed in the current project via hero install).

Stages: Configuration → Research → Planning → Implementation → QA → Judge → QA End-to-End`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunDefault(cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// verbose/debug flags are available globally; features can query them.
			_ = verbose
			_ = debug
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output (includes stack traces)")

	// Register all subcommands.
	root.AddCommand(
		install.NewCommand(version, assets.FS),
		upgrade.NewCommand(version, assets.FS),
		uninstall.NewCommand(),
		doctor.NewCommand(version),
		status.NewCommand(),
		variables.NewCommand(),
		update_models.NewCommand(),
		tui.NewCommand(),
		newVersionCommand(),
	)
	for _, c := range cycle.NewCommands() {
		root.AddCommand(c)
	}

	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Hero CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("hero version %s\n", version)
		},
	}
}
