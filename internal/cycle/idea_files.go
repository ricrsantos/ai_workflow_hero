package cycle

import (
	"fmt"

	"github.com/ricrsantos/ai_workflow_hero/internal/ideadocs"
	"github.com/spf13/cobra"
)

type ideaFilesView struct {
	Paths []string `json:"paths"`
}

func newCycleIdeaFilesCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "idea-files",
		Short: "List active design notes under docs/idea (excluding archive/ and tobe/)",
		Long: `List project-relative paths to active idea files for the Research stage.

Excludes top-level docs/idea/archive/ and docs/idea/tobe/. Used by discover_agent
at session start; table output prints one path per line.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: withService(func(cmd *cobra.Command, svc *Service) error {
			paths, err := ideadocs.ListActive(svc.ProjectDir)
			if err != nil {
				return err
			}
			if asJSON {
				return encodeJSON(cmd.OutOrStdout(), ideaFilesView{Paths: paths})
			}
			for _, p := range paths {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), p); err != nil {
					return err
				}
			}
			return nil
		}),
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output paths as JSON")
	return cmd
}
