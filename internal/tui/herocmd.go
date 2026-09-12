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

// parseHeroAddTodoInline returns optional finding IDs from /hero-add-todo [id...].
func parseHeroAddTodoInline(text string) ([]string, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "/hero-add-todo" {
		return nil, true
	}
	const prefix = "/hero-add-todo "
	if !strings.HasPrefix(lower, prefix) {
		return nil, false
	}
	args := strings.Fields(strings.TrimSpace(text[len(prefix):]))
	return args, true
}

// parseHeroCompleteTodoInline returns optional ToDo IDs from /hero-complete-todo [id...].
func parseHeroCompleteTodoInline(text string) ([]string, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "/hero-complete-todo" {
		return nil, true
	}
	const prefix = "/hero-complete-todo "
	if !strings.HasPrefix(lower, prefix) {
		return nil, false
	}
	args := strings.Fields(strings.TrimSpace(text[len(prefix):]))
	return args, true
}
