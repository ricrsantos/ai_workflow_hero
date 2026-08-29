package cycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/spf13/cobra"
)

// NewCommands returns all CLI-as-API cycle-related commands.
func NewCommands() []*cobra.Command {
	return []*cobra.Command{
		newMetricsCommand(),
		newEventsCommand(),
		newApproveCommand(),
		newRejectCommand(),
		newCancelCommand(),
		newFinishCommand(),
		newContinueCommand(),
		newStageCommand(),
		newCycleCommand(),
		newRunCommand(),
	}
}

func withService(run func(cmd *cobra.Command, svc *Service) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		stderr := cmd.ErrOrStderr()
		svc, err := OpenService("")
		if err != nil {
			e := clierr.New(err.Error())
			clierr.Format(stderr, e)
			return e
		}
		defer svc.Close()
		if err := run(cmd, svc); err != nil {
			e := mapCLIError(err)
			clierr.Format(stderr, e)
			return e
		}
		return nil
	}
}

func mapCLIError(err error) *clierr.HeroError {
	if errors.Is(err, store.ErrNoActiveCycle) {
		return clierr.NewWithSuggestion("no active cycle.", "Run `hero cycle new` or `/hero-new` first.")
	}
	if errors.Is(err, store.ErrBusy) {
		return clierr.NewWithSuggestion("cycle is locked by another session.", "Wait for the other session to finish or clear the lock.")
	}
	return clierr.New(err.Error())
}

func newMetricsCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:           "metrics",
		Short:         "Show cycle metrics from the operational store",
		Long:          `Show per-stage token/cost estimates from SQLite. Table by default; --json for machine output.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			view, err := svc.Metrics()
			if err != nil {
				return err
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), view)
			}
			printMetricsTable(cmd.OutOrStdout(), view)
			return nil
		}),
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output metrics as JSON")
	return cmd
}

func newEventsCommand() *cobra.Command {
	var asJSON bool
	var limit int
	var eventType string
	cmd := &cobra.Command{
		Use:           "events",
		Short:         "Show recent cycle events from the operational store",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			view, err := svc.Events(eventType, limit)
			if err != nil {
				return err
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), view)
			}
			printEventsTable(cmd.OutOrStdout(), view)
			return nil
		}),
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output events as JSON")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum events to return")
	cmd.Flags().StringVar(&eventType, "type", "", "Filter by event type")
	return cmd
}

func newApproveCommand() *cobra.Command {
	var summary, metricsJSON string
	cmd := &cobra.Command{
		Use:           "approve",
		Short:         "Approve the pending stage",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if err := svc.Approve(summary, metricsJSON); err != nil {
				return err
			}
			output.Success(cmd.OutOrStdout(), "Stage approved.")
			return nil
		}),
	}
	cmd.Flags().StringVar(&summary, "summary", "", "Optional approval summary")
	cmd.Flags().StringVar(&metricsJSON, "metrics-json", "", "Metrics payload JSON (object or array)")
	return cmd
}

func newRejectCommand() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:           "reject",
		Short:         "Reject the pending stage",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if err := svc.Reject(reason); err != nil {
				return err
			}
			output.Success(cmd.OutOrStdout(), "Stage rejected.")
			return nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Rejection reason")
	return cmd
}

func newCancelCommand() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:           "cancel",
		Short:         "Cancel the active cycle",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if err := svc.Cancel(reason); err != nil {
				return err
			}
			output.Success(cmd.OutOrStdout(), "Cycle cancelled.")
			return nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Cancellation reason")
	return cmd
}

func newFinishCommand() *cobra.Command {
	var metricsJSON string
	cmd := &cobra.Command{
		Use:           "finish",
		Short:         "Finish the active cycle",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if err := svc.Finish(metricsJSON); err != nil {
				return err
			}
			output.Success(cmd.OutOrStdout(), "Cycle finished.")
			return nil
		}),
	}
	cmd.Flags().StringVar(&metricsJSON, "metrics-json", "", "Metrics payload JSON (object or array)")
	return cmd
}

func newContinueCommand() *cobra.Command {
	var extra int
	cmd := &cobra.Command{
		Use:           "continue",
		Short:         "Grant extra iterations to an escalated stage",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if err := svc.Continue(extra); err != nil {
				return err
			}
			output.Successf(cmd.OutOrStdout(), "Granted +%d extra iteration(s).", extra)
			return nil
		}),
	}
	cmd.Flags().IntVar(&extra, "extra", 1, "Number of extra iterations to grant")
	return cmd
}

func newStageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "stage",
		Short:         "Start or close a cycle stage",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newStageStartCommand(), newStageCloseCommand())
	return cmd
}

func newStageStartCommand() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:           "start",
		Short:         "Mark a stage as Running (increments iteration)",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if err := svc.StartStage(name); err != nil {
				return err
			}
			output.Successf(cmd.OutOrStdout(), "Stage %s started.", name)
			return nil
		}),
	}
	cmd.Flags().StringVar(&name, "name", "", "Stage name (e.g. research, qa)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newStageCloseCommand() *cobra.Command {
	var name, summary, metricsJSON string
	var failed bool
	cmd := &cobra.Command{
		Use:           "close",
		Short:         "Close a running stage (PendingApproval or Completed)",
		Long:          `Close the running stage. When require_human_approval is false, completes and advances. When true, moves to PendingApproval for hero approve.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if err := svc.CloseStage(name, summary, metricsJSON, failed); err != nil {
				return err
			}
			output.Successf(cmd.OutOrStdout(), "Stage %s closed.", name)
			return nil
		}),
	}
	cmd.Flags().StringVar(&name, "name", "", "Stage name (e.g. research, qa)")
	cmd.Flags().StringVar(&summary, "summary", "", "Stage summary")
	cmd.Flags().StringVar(&metricsJSON, "metrics-json", "", "Metrics payload JSON (object or array)")
	cmd.Flags().BoolVar(&failed, "failed", false, "Mark the stage as Failed")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newCycleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "cycle",
		Short:         "Cycle lifecycle commands",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newCycleNewCommand(), newCycleSyncConfigCommand(), newCycleArchiveCommand(), newCycleResumeCommand(), newCycleOpenspecChangeCommand(), newCycleIdeaFilesCommand())
	return cmd
}

func newCycleOpenspecChangeCommand() *cobra.Command {
	var clear bool
	var nameArg string
	cmd := &cobra.Command{
		Use:   "openspec-change [name]",
		Short: "Set or clear the OpenSpec change name on the active cycle",
		Long: `Persist the OpenSpec change directory name on the active cycle (ADR-023).

Examples:
  hero cycle openspec-change slash-parity-tui-harness
  hero cycle openspec-change --clear`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			nameArg = ""
			if len(args) > 0 {
				nameArg = args[0]
			}
			if clear && nameArg != "" {
				return fmt.Errorf("provide either a change name or --clear, not both")
			}
			if !clear && nameArg == "" {
				return fmt.Errorf("provide a change name or --clear")
			}
			return nil
		},
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if clear {
				if err := svc.ClearOpenspecChange(); err != nil {
					return err
				}
				output.Success(cmd.OutOrStdout(), "Cleared openspec_change on active cycle.")
				return nil
			}
			if err := svc.SetOpenspecChange(nameArg); err != nil {
				return err
			}
			output.Successf(cmd.OutOrStdout(), "Set openspec_change to %q.", nameArg)
			return nil
		}),
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear openspec_change on the active cycle")
	return cmd
}

func newCycleNewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Prepare a new active cycle from workflow-config.yml (empty title/objective until sync-config)",
		Long: `Create an active cycle in SQLite after /hero-new prepares workflow-config.yml.

Title and objective stay empty until hero cycle sync-config (normally run by /hero-start).
Stages are imported from the current workflow-config.yml.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			res, err := svc.PrepareCycle()
			if err != nil {
				return err
			}
			output.Successf(cmd.OutOrStdout(), "Prepared cycle C%d (%d stages). Edit workflow-config.yml title/objective, then run /hero-start.", res.Cycle.Number, len(res.Stages))
			return nil
		}),
	}
	return cmd
}

func newCycleSyncConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "sync-config",
		Short:         "Sync active cycle title/objective from workflow-config.yml",
		Long:          `Updates the active cycle's title, objective, and config snapshot from .workflow-hero/cycles/current/workflow-config.yml. /hero-start runs this before orchestration.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if err := svc.SyncCycleConfig(); err != nil {
				return err
			}
			st, err := svc.Status()
			if err != nil {
				return err
			}
			output.Successf(cmd.OutOrStdout(), "Synced cycle C%d — %s.", st.CycleNumber, st.Title)
			return nil
		}),
	}
	return cmd
}

func newCycleArchiveCommand() *cobra.Command {
	var force, skipOpenspec bool
	var openspecChange string
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive the current cycle (OpenSpec archive first when configured)",
		Long: `Archive the active (or latest completed) Hero cycle.

When an OpenSpec change name is stored, auto-detected (single active change), or
passed via --openspec-change, runs openspec archive <name> -y before moving the
cycle folder. On OpenSpec failure, Hero archive is blocked unless --force or
--skip-openspec is set; forced runs print manual OpenSpec recovery instructions.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			res, err := svc.ArchiveWithOptions(ArchiveOptions{
				Force:          force,
				SkipOpenspec:   skipOpenspec,
				OpenspecChange: openspecChange,
			})
			if err != nil {
				return err
			}
			if res.OpenspecForced && res.OpenspecChange != "" {
				output.Warning(cmd.OutOrStdout(), ManualOpenspecArchiveInstructions(res.OpenspecChange))
			}
			output.Successf(cmd.OutOrStdout(), "Archived cycle C%d to %s.", res.CycleNumber, res.ArchiveDir)
			return nil
		}),
	}
	cmd.Flags().BoolVar(&force, "force", false, "Archive Hero cycle even when OpenSpec archive fails")
	cmd.Flags().BoolVar(&skipOpenspec, "skip-openspec", false, "Alias for --force when OpenSpec archive fails")
	cmd.Flags().StringVar(&openspecChange, "openspec-change", "", "OpenSpec change directory name for this archive")
	return cmd
}

func newCycleResumeCommand() *cobra.Command {
	var number int
	cmd := &cobra.Command{
		Use:           "resume",
		Short:         "Resume a paused/cancelled cycle",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			if err := svc.Resume(number); err != nil {
				return err
			}
			output.Success(cmd.OutOrStdout(), "Cycle resumed.")
			return nil
		}),
	}
	cmd.Flags().IntVar(&number, "number", 0, "Cycle number to resume (default: latest non-archived)")
	return cmd
}

func newRunCommand() *cobra.Command {
	var stage string
	cmd := &cobra.Command{
		Use:           "run",
		Short:         "Dispatch stage execution via the harness adapter",
		Long:          `Best-effort Cursor dispatch. Falls back to chat guidance when push is unavailable.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			res, err := svc.Run(stage)
			if err != nil {
				return err
			}
			if res.Dispatched {
				output.Success(cmd.OutOrStdout(), res.Message)
			} else {
				output.Warning(cmd.OutOrStdout(), res.Message)
			}
			return nil
		}),
	}
	cmd.Flags().StringVar(&stage, "stage", "", "Stage name to dispatch")
	return cmd
}

func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printMetricsTable(w io.Writer, view MetricsView) {
	if len(view.Rows) == 0 {
		output.Progressf(w, "No metrics for cycle C%d.", view.CycleNumber)
		return
	}
	headers := []string{"Stage", "Agent", "Model", "In", "Out", "Cost USD", "Duration ms"}
	var rows [][]string
	for _, r := range view.Rows {
		rows = append(rows, []string{
			r.Stage, r.Agent, r.Model,
			fmt.Sprintf("%d", r.InputTokens),
			fmt.Sprintf("%d", r.OutputTokens),
			fmt.Sprintf("%.6f", r.CostUSD),
			fmt.Sprintf("%d", r.DurationMS),
		})
	}
	output.Table(w, headers, rows)
	fmt.Fprintf(w, "\nTotal: %d in / %d out tokens, ~$%.6f USD\n", view.TotalIn, view.TotalOut, view.TotalCost)
}

func printEventsTable(w io.Writer, view EventsView) {
	if len(view.Events) == 0 {
		output.Progressf(w, "No events for cycle C%d.", view.CycleNumber)
		return
	}
	headers := []string{"Time", "Type", "Payload"}
	var rows [][]string
	for _, e := range view.Events {
		payload := e.PayloadJSON
		if len(payload) > 60 {
			payload = payload[:57] + "..."
		}
		rows = append(rows, []string{e.TS, e.Type, payload})
	}
	output.Table(w, headers, rows)
}
