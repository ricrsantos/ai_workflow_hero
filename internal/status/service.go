package status

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
)

// StageStatus represents the status of a single workflow stage.
type StageStatus struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	Iteration      string `json:"iteration"`
	HumanApproval  string `json:"humanApproval"`
}

// WorkflowStatus represents the current cycle's status.
type WorkflowStatus struct {
	Stages []StageStatus `json:"stages"`
}

// Options holds status command options.
type Options struct {
	ProjectDir string
}

// Run reads the current workflow.md and returns status.
func Run(opts Options) (WorkflowStatus, error) {
	workflowPath := filepath.Join(opts.ProjectDir, cursoradapter.HeroCurrentCycleDir, "workflow.md")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkflowStatus{}, nil // not initialized
		}
		return WorkflowStatus{}, fmt.Errorf("read workflow.md: %w", err)
	}

	return parseWorkflowMD(string(data))
}

// parseWorkflowMD parses the stages table from workflow.md.
func parseWorkflowMD(content string) (WorkflowStatus, error) {
	var ws WorkflowStatus
	scanner := bufio.NewScanner(strings.NewReader(content))

	inTable := false
	headerPassed := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !inTable {
			// Look for the table header line.
			if strings.HasPrefix(trimmed, "| Stage") {
				inTable = true
				continue
			}
			continue
		}

		// Skip separator row.
		if strings.HasPrefix(trimmed, "|---") || strings.HasPrefix(trimmed, "|----") || strings.HasPrefix(trimmed, "| ---") {
			headerPassed = true
			continue
		}

		if !headerPassed {
			continue
		}

		if !strings.HasPrefix(trimmed, "|") {
			break // end of table
		}

		parts := strings.Split(trimmed, "|")
		// parts[0] is empty (before first |), parts[len-1] is empty (after last |)
		if len(parts) < 5 {
			continue
		}

		stage := StageStatus{
			Name:          strings.TrimSpace(parts[1]),
			Status:        strings.TrimSpace(parts[2]),
			Iteration:     strings.TrimSpace(parts[3]),
			HumanApproval: strings.TrimSpace(parts[4]),
		}
		if stage.Name == "" {
			continue
		}
		ws.Stages = append(ws.Stages, stage)
	}

	return ws, nil
}

// PrintTable writes a human-readable table to w.
func PrintTable(w io.Writer, ws WorkflowStatus) {
	if len(ws.Stages) == 0 {
		output.Progressf(w, "No active cycle. Run /hero:init to start.")
		return
	}
	headers := []string{"Stage", "Status", "Iteration", "Human Approval"}
	var rows [][]string
	for _, s := range ws.Stages {
		rows = append(rows, []string{s.Name, s.Status, s.Iteration, s.HumanApproval})
	}
	output.Table(w, headers, rows)
}
