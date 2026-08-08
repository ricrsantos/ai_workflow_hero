package status

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/spf13/cobra"
)

// NewCommand creates the `hero status` cobra command.
func NewCommand() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current cycle workflow status",
		Long: `Read the active cycle stage machine from SQLite and display status.

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

// Options holds status command options.
type Options struct {
	ProjectDir string
}

// WorkflowStatus is kept for JSON compatibility with prior status output.
type WorkflowStatus = cycle.StatusView

// StageStatus alias for compatibility.
type StageStatus = cycle.StatusStage

// Run reads status from the SQLite operational store.
func Run(opts Options) (cycle.StatusView, error) {
	svc, err := cycle.OpenService(opts.ProjectDir)
	if err != nil {
		return cycle.StatusView{}, err
	}
	defer svc.Close()
	return svc.Status()
}

// PrintTable writes a human-readable table to w.
func PrintTable(w io.Writer, ws cycle.StatusView) {
	if len(ws.Stages) == 0 {
		output.Progressf(w, "No active cycle. Run hero cycle new (or /hero-new) to start.")
		return
	}
	if ws.CycleNumber > 0 {
		fmt.Fprintf(w, "Cycle C%d — %s (%s)\n", ws.CycleNumber, ws.Title, ws.Status)
		if ws.OpenspecChange != "" {
			fmt.Fprintf(w, "OpenSpec change: %s\n", ws.OpenspecChange)
		}
		fmt.Fprintln(w)
	}
	headers := []string{"Stage", "Status", "Iteration", "Human Approval"}
	var rows [][]string
	for _, s := range ws.Stages {
		rows = append(rows, []string{s.Name, s.Status, s.Iteration, s.HumanApproval})
	}
	output.Table(w, headers, rows)
}
