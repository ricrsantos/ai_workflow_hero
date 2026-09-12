package cycle

import (
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/spf13/cobra"
)

// StructuredDiagnosticJSONForTest exposes JSON diagnostic encoding for tests.
func StructuredDiagnosticJSONForTest(d *reports.DiagnosticError) string {
	return structuredDiagnosticJSON(d)
}

// StageCloseCommandForTest returns the stage close cobra command for flag tests.
func StageCloseCommandForTest() *cobra.Command {
	return newStageCloseCommand()
}

// AddTodoCommandForTest returns the add-todo cobra command for flag tests.
func AddTodoCommandForTest() *cobra.Command {
	return newAddTodoCommand()
}

// CompleteTodoCommandForTest returns the complete-todo cobra command for flag tests.
func CompleteTodoCommandForTest() *cobra.Command {
	return newCompleteTodoCommand()
}
