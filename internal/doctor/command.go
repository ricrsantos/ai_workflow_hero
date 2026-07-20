package doctor

import (
	"encoding/json"
	"os"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/spf13/cobra"
)

// NewCommand creates the `hero doctor` cobra command.
func NewCommand(version string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check Hero installation integrity",
		Long: `Verify that Hero is correctly installed in the current project.

Checks:
  - Git repository presence
  - Required Hero files and directories (ADR-011 inventory)
  - Version consistency between hero.json and this binary
  - Config file JSON/YAML validity`,
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

			report := Run(Options{
				ProjectDir:    projectDir,
				BinaryVersion: version,
			})

			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(report)
			} else {
				PrintTable(stdout, report)
			}

			if !report.OK {
				return &clierr.HeroError{Description: "doctor found issues"}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output results as JSON")
	return cmd
}
