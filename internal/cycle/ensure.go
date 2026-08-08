package cycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// EnsureResult describes what EnsureOperationalStore did.
type EnsureResult struct {
	LegacyImported bool
	LegacyCycleNum int
}

// EnsureOperationalStore creates/migrates `.workflow-hero/hero.db` and, when the
// store has no cycles yet, imports legacy workflow.md/metrics.md once if present.
// Callers must Close the returned store.
func EnsureOperationalStore(projectDir string) (*store.Store, EnsureResult, error) {
	var result EnsureResult
	st, err := store.OpenProject(projectDir)
	if err != nil {
		return nil, result, fmt.Errorf("open store: %w", err)
	}
	imported, cycleNum, err := importLegacyIfNeeded(st, projectDir)
	if err != nil {
		_ = st.Close()
		return nil, result, err
	}
	result.LegacyImported = imported
	result.LegacyCycleNum = cycleNum
	return st, result, nil
}

func importLegacyIfNeeded(st *store.Store, projectDir string) (imported bool, cycleNum int, err error) {
	cycles, err := st.ListCycles()
	if err != nil {
		return false, 0, fmt.Errorf("list cycles: %w", err)
	}
	if len(cycles) > 0 {
		return false, 0, nil
	}

	cycleDir := filepath.Join(projectDir, cursoradapter.HeroCurrentCycleDir)
	workflowPath := filepath.Join(cycleDir, "workflow.md")
	if _, err := os.Stat(workflowPath); err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("stat workflow.md: %w", err)
	}

	cycleNumber := legacyCycleNumberFromProject(projectDir)
	configSnapshot := legacyConfigSnapshot(cycleDir)

	importRes, err := st.ImportLegacyCycle(cycleDir, cycleNumber, configSnapshot)
	if err != nil {
		return false, 0, fmt.Errorf("import legacy cycle: %w", err)
	}
	if importRes.Imported {
		return true, importRes.CycleNumber, nil
	}
	return false, 0, nil
}

func legacyCycleNumberFromProject(projectDir string) int {
	projectPath := filepath.Join(projectDir, cursoradapter.ProjectJSONPath)
	data, err := os.ReadFile(projectPath)
	if err != nil {
		return 0
	}
	var project struct {
		Workflow struct {
			Cycle int `json:"cycle"`
		} `json:"workflow"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		return 0
	}
	if project.Workflow.Cycle <= 0 {
		return 0
	}
	return project.Workflow.Cycle
}

func legacyConfigSnapshot(cycleDir string) string {
	configPath := filepath.Join(cycleDir, "workflow-config.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "{}"
	}
	return string(data)
}
