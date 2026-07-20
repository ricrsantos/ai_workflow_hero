package variables

import (
	"encoding/json"
	"os"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/spf13/cobra"
)

// NewCommand creates the `hero variables` cobra command.
func NewCommand() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "variables",
		Short: "Show project and Hero configuration variables",
		Long: `Display key fields from project.json and hero.json.

Outputs a table by default, or JSON with --json.`,
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

			vars, err := Run(Options{ProjectDir: projectDir})
			if err != nil {
				e := clierr.New("variables failed: " + err.Error())
				clierr.Format(stderr, e)
				return e
			}

			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(vars)
			} else {
				PrintTable(stdout, vars)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output variables as JSON")
	return cmd
}
