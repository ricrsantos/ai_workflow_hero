package tui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/todos"
)

func formatCyclesList(svc *cycle.Service) (string, error) {
	if svc == nil {
		return "", fmt.Errorf("cycle service unavailable")
	}
	view, err := svc.Cycles()
	if err != nil {
		return "", err
	}
	if view.Total == 0 {
		return "No cycles found. Run /hero-new to start.", nil
	}
	var buf bytes.Buffer
	cycle.FormatCycles(&buf, view)
	return strings.TrimRight(buf.String(), "\n"), nil
}

func formatTodosList(projectDir string) (string, error) {
	items, err := todos.ReadProject(projectDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("context/current-state.md not found — run /hero-sync first")
		}
		return "", err
	}
	return todos.Format(items), nil
}
