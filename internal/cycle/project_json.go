package cycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

type projectJSONWorkflow struct {
	Cycle int `json:"cycle"`
}

type projectJSONFile struct {
	Workflow projectJSONWorkflow `json:"workflow"`
}

// syncProjectWorkflowCycle updates project.json → workflow.cycle after cycle prepare.
func syncProjectWorkflowCycle(projectDir string, cycleNumber int) error {
	if cycleNumber <= 0 {
		return nil
	}
	path := filepath.Join(projectDir, cursoradapter.ProjectJSONPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read project.json: %w", err)
	}
	var doc projectJSONFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse project.json: %w", err)
	}
	doc.Workflow.Cycle = cycleNumber
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}
