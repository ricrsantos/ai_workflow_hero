package uninstall

import (
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

			if !yes {
				// Require --yes to proceed (non-interactive safety guard).
				e := clierr.NewWithSuggestion(
					"uninstall requires confirmation",
					"pass --yes to confirm removal of Hero-owned files.",
				)
				clierr.Format(stderr, e)
				return e
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

	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm removal of Hero-owned files")
	return cmd
}
