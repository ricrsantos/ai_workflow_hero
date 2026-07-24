package upgrade

import (
	"io/fs"
	"os"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/spf13/cobra"
)

// NewCommand creates the `hero upgrade` cobra command.
func NewCommand(version string, assetsFS fs.FS) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade Hero assets in the current project",
		Long: `Re-copy updated Hero assets from this binary version into the current project.

Files you have customized (detected via checksum comparison) will not be
overwritten — Hero will warn you and list them for manual merging.

Migration: workflow-config.yml files that still use the legacy generic_model
field are automatically converted to the fallback_model block (model,
reasoning_effort, enable_fast_model, thinking). If both keys are present,
Hero warns and leaves the file unchanged for manual merge.`,
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
				// In a real interactive scenario we'd prompt; for now accept --yes.
				// Non-interactive use requires --yes.
			}

			output.Progressf(stdout, "Upgrading Hero assets to version %s...", version)

			opts := Options{
				ProjectDir: projectDir,
				Version:    version,
				AssetsFS:   assetsFS,
			}

			result, err := Run(opts, stdout, stderr)
			if err != nil {
				e := clierr.New("upgrade failed: " + err.Error())
				clierr.Format(stderr, e)
				return e
			}

			for _, f := range result.Updated {
				output.Successf(stdout, "Updated: %s", f)
			}
			for _, f := range result.Migrated {
				output.Successf(stdout, "Migrated workflow-config: %s (generic_model → fallback_model)", f)
			}
			if len(result.Skipped) > 0 {
				output.Warningf(stdout, "%d file(s) skipped (customized). Review and merge manually.", len(result.Skipped))
			}

			output.Successf(stdout, "Hero upgraded to version %s.", version)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
