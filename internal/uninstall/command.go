package uninstall

import (
	"io"
	"os"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/spf13/cobra"
)

// NewCommand creates the `hero uninstall` cobra command.
func NewCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Hero from the current project",
		Long: `Remove Hero-owned files and directories from the current project.

The following are removed:
  .cursor/agents/
  .cursor/commands/hero-*.md
  .cursor/skills/workflow-hero/
  .cursor/skills/grilling/
  .workflow-hero/

The following are preserved:
  AGENTS.md, context/, docs/, openspec/`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			projectDir, err := os.Getwd()
			if err != nil {
				e := clierr.New("could not determine current directory: " + err.Error())
				clierr.Format(stderr, e)
				return e
			}

			proceed, herr := resolveUninstallProceed(yes, stdout, stderr)
			if herr != nil {
				clierr.Format(stderr, herr)
				return herr
			}
			if !proceed {
				output.Progress(stdout, "Uninstall cancelled.")
				return nil
			}

			output.Progressf(stdout, "Uninstalling Hero from %s...", projectDir)

			if err := Run(Options{ProjectDir: projectDir}, stdout, stderr); err != nil {
				e := clierr.New("uninstall failed: " + err.Error())
				clierr.Format(stderr, e)
				return e
			}

			output.Successf(stdout, "Hero uninstalled. Project artifacts (AGENTS.md, context/, docs/, openspec/) are preserved.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the uninstall confirmation prompt")
	return cmd
}

func resolveUninstallProceed(skipPrompt bool, stdout, stderr interface {
	io.Writer
}) (bool, *clierr.HeroError) {
	if skipPrompt {
		return true, nil
	}
	if !clierr.IsTerminal(stdout) && !clierr.IsTerminal(stderr) {
		return false, clierr.NewWithSuggestion(
			"uninstall requires confirmation",
			"pass --yes to confirm removal of Hero-owned files.",
		)
	}
	confirmed, err := promptUninstallConfirm()
	if err != nil {
		return false, clierr.New("prompt error: " + err.Error())
	}
	return confirmed, nil
}

// ConfirmTitleForTest exposes the uninstall confirmation title for tests.
func ConfirmTitleForTest() string { return uninstallConfirmTitle }

// ConfirmBodyForTest exposes the uninstall confirmation body for tests.
func ConfirmBodyForTest() string { return uninstallConfirmBody }

// ResolveUninstallProceedForTest mirrors resolveUninstallProceed for unit tests.
func ResolveUninstallProceedForTest(skipPrompt bool, stdout, stderr io.Writer) (bool, *clierr.HeroError) {
	return resolveUninstallProceed(skipPrompt, stdout, stderr)
}
