package install

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/spf13/cobra"
)

// ToolsFlagRemovedMsg is the primary error line when --tools is passed (UI-C04-001 §2).
const ToolsFlagRemovedMsg = "Flag --tools is not supported in Hero 2.0."

// ToolsFlagRemovedSuggestion is the follow-up hint for --tools removal.
const ToolsFlagRemovedSuggestion = "run `hero install` and select harnesses interactively,\nor enable them later in the TUI with /hero-harness."

// NewCommand creates the `hero install` cobra command.
func NewCommand(version string, assetsFS fs.FS) *cobra.Command {
	var (
		tools   string
		name    string
		summary string
		yes     bool
		gitInit bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Hero into the current project",
		Long: `Install Hero into the current project directory.

Hero requires the project to be a git repository. If it is not, Hero will
offer to run 'git init' on your behalf (or you can pass --git-init).

Select at least one harness (Cursor and/or OpenCode) during the interactive install.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, version, assetsFS, tools, name, summary, yes, gitInit)
		},
	}

	cmd.Flags().StringVar(&tools, "tools", "", "deprecated in Hero 2.0 — use interactive harness selection")
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&summary, "summary", "", "Project summary (optional)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive prompts (--name required; --summary optional)")
	cmd.Flags().BoolVar(&gitInit, "git-init", false, "Initialize a git repository if none exists")

	return cmd
}

func runInstall(cmd *cobra.Command, version string, assetsFS fs.FS, tools, name, summary string, yes, gitInit bool) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	if cmd.Flags().Changed("tools") {
		e := clierr.NewWithSuggestion(ToolsFlagRemovedMsg, ToolsFlagRemovedSuggestion)
		clierr.Format(stderr, e)
		return e
	}

	projectDir, err := os.Getwd()
	if err != nil {
		e := clierr.New("could not determine current directory: " + err.Error())
		clierr.Format(stderr, e)
		return e
	}

	if !isGitRepo(projectDir) {
		if gitInit {
			// handled in Run
		} else if yes {
			e := clierr.NewWithSuggestion(
				"this directory is not a git repository",
				"run `hero install --git-init` to let Hero initialize\ngit automatically, or run `git init` manually and retry.",
			)
			clierr.Format(stderr, e)
			return e
		} else {
			var doGitInit bool
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("This directory is not a git repository. Initialize one now?").
						Value(&doGitInit),
				),
			).WithTheme(heroInstallTheme())
			if err := form.Run(); err != nil {
				e := clierr.New("prompt error: " + err.Error())
				clierr.Format(stderr, e)
				return e
			}
			if !doGitInit {
				e := clierr.NewWithSuggestion(
					"installation aborted: git repository is required",
					"run `git init` in this directory and retry.",
				)
				clierr.Format(stderr, e)
				return e
			}
			gitInit = true
		}
	}

	summaryFlagSet := cmd.Flags().Changed("summary")
	needName := strings.TrimSpace(name) == ""
	needSummaryPrompt := !summaryFlagSet && !yes

	printSetupHeader(stdout)

	if needName || needSummaryPrompt {
		groups := []*huh.Group{}
		if needName {
			groups = append(groups, huh.NewGroup(
				huh.NewInput().
					Title("Project name:").
					Prompt("> ").
					Value(&name).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("project name cannot be empty")
						}
						return nil
					}),
			))
		}
		if needSummaryPrompt {
			groups = append(groups, huh.NewGroup(
				huh.NewInput().
					Title("Project summary (Opcional):").
					Prompt("> ").
					Value(&summary),
			))
		}
		form := huh.NewForm(groups...).WithTheme(heroInstallTheme())
		if err := form.Run(); err != nil {
			e := clierr.New("prompt error: " + err.Error())
			clierr.Format(stderr, e)
			return e
		}
	}

	selected, err := promptHarnessMultiSelect(stdout)
	if err != nil {
		e := clierr.New(err.Error())
		clierr.Format(stderr, e)
		return e
	}

	name = strings.TrimSpace(name)
	summary = strings.TrimSpace(summary)
	if name == "" {
		e := clierr.NewWithSuggestion(
			"project name is required",
			"pass --name \"Your Project\" or run without --yes to be prompted.",
		)
		clierr.Format(stderr, e)
		return e
	}

	opts := Options{
		ProjectDir: projectDir,
		Name:       name,
		Summary:    summary,
		Tools:      selected,
		Version:    version,
		GitInit:    gitInit,
		AssetsFS:   assetsFS,
	}

	if err := Run(opts, stdout, stderr); err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			e := clierr.NewWithSuggestion(
				"this directory is not a git repository",
				"run `hero install --git-init` to let Hero initialize\ngit automatically, or run `git init` manually and retry.",
			)
			clierr.Format(stderr, e)
			return e
		}
		e := clierr.New("installation failed: " + err.Error())
		clierr.Format(stderr, e)
		return e
	}

	fmt.Fprintln(stdout)
	output.Successf(stdout, "Hero installed successfully (%s enabled).", EnabledHarnessSummary(selected))
	output.Progressf(stdout, "Full user guide: %s", cursoradapter.WorkflowHelpPath)
	return nil
}

func promptHarnessMultiSelect(stdout interface{ Write([]byte) (int, error) }) ([]string, error) {
	var selected []string
	options := []huh.Option[string]{
		huh.NewOption("Cursor", "cursor"),
		huh.NewOption("OpenCode", "opencode"),
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select the AI Harnesses you want to use (at least one):").
				Description("Supported harnesses for Hero 2.0").
				Options(options...).
				Value(&selected).
				Validate(func(v []string) error {
					if len(v) == 0 {
						return fmt.Errorf("select at least one harness")
					}
					return nil
				}),
		),
	).WithTheme(heroInstallTheme())
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("harness selection cancelled: %w", err)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("select at least one harness")
	}
	return selected, nil
}
