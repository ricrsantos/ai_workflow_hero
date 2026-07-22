package install

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/spf13/cobra"
)

// NewCommand creates the `hero install` cobra command.
// version is the CLI version injected at build time.
// assetsFS is the embedded asset filesystem.
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

Only --tools cursor is supported in V1.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, version, assetsFS, tools, name, summary, yes, gitInit)
		},
	}

	cmd.Flags().StringVar(&tools, "tools", "", "IDE tool to install for (required; only 'cursor' supported)")
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&summary, "summary", "", "Project summary (optional)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive prompts (--name required; --summary optional)")
	cmd.Flags().BoolVar(&gitInit, "git-init", false, "Initialize a git repository if none exists")

	_ = cmd.MarkFlagRequired("tools")

	return cmd
}

func runInstall(cmd *cobra.Command, version string, assetsFS fs.FS, tools, name, summary string, yes, gitInit bool) error {
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	// Validate --tools flag.
	if strings.ToLower(tools) != "cursor" {
		e := clierr.NewWithSuggestion(
			fmt.Sprintf("unsupported tool: %q", tools),
			"Only --tools cursor is supported in V1.",
		)
		clierr.Format(stderr, e)
		return e
	}

	projectDir, err := os.Getwd()
	if err != nil {
		e := clierr.New("could not determine current directory: " + err.Error())
		clierr.Format(stderr, e)
		return e
	}

	// Check git and optionally offer to init.
	if !isGitRepo(projectDir) {
		if gitInit {
			// Will be handled in Run.
		} else if yes {
			// --yes without --git-init still aborts if no git repo.
			e := clierr.NewWithSuggestion(
				"this directory is not a git repository",
				"run `hero install --tools cursor --git-init` to let Hero initialize\ngit automatically, or run `git init` manually and retry.",
			)
			clierr.Format(stderr, e)
			return e
		} else {
			// Interactive: ask user.
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

	if yes && needName {
		e := clierr.NewWithSuggestion(
			"project name is required",
			"pass --name \"Your Project\" (and optionally --summary \"...\") with --yes.",
		)
		clierr.Format(stderr, e)
		return e
	}

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
		Tools:      []string{tools},
		Version:    version,
		GitInit:    gitInit,
		AssetsFS:   assetsFS,
	}

	if err := Run(opts, stdout, stderr); err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			e := clierr.NewWithSuggestion(
				"this directory is not a git repository",
				"run `hero install --tools cursor --git-init` to let Hero initialize\ngit automatically, or run `git init` manually and retry.",
			)
			clierr.Format(stderr, e)
			return e
		}
		e := clierr.New("installation failed: " + err.Error())
		clierr.Format(stderr, e)
		return e
	}

	fmt.Fprintln(stdout)
	output.Successf(stdout, "Hero installed successfully.")
	return nil
}
