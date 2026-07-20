package status

import (
	"encoding/json"
	"os"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/spf13/cobra"
)

// NewCommand creates the `hero status` cobra command.
func NewCommand() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current cycle workflow status",
		Long: `Read the current development cycle's workflow.md and display stage status.

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

			ws, err := Run(Options{ProjectDir: projectDir})
			if err != nil {
				e := clierr.New("status failed: " + err.Error())
				clierr.Format(stderr, e)
				return e
			}

			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(ws)
			} else {
				PrintTable(stdout, ws)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output status as JSON")
	return cmd
}
