package update_models

import (
	"os"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/spf13/cobra"
)

// NewCommand creates the `hero update-models` cobra command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-models",
		Short: "Fetch updated model pricing data from the Hero repository",
		Long: `Fetch the latest model pricing YAML files from the Hero GitHub repository
and rewrite .workflow-hero/models/*.yml with the updated data.

This command never scrapes pricing pages — it fetches pre-structured data
maintained by the Hero project (ADR-003).`,
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

			output.Progressf(stdout, "Fetching updated model pricing data...")

			if err := Run(Options{ProjectDir: projectDir}, stdout, stderr); err != nil {
				e := clierr.NewWithSuggestion(
					"update-models failed: "+err.Error(),
					"Check your internet connection and try again.",
				)
				clierr.Format(stderr, e)
				return e
			}

			output.Successf(stdout, "Model pricing data updated.")
			return nil
		},
	}

	return cmd
}
