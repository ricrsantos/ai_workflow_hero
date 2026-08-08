package doctor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

// CursorCLIProbe checks Cursor Agent CLI availability for doctor diagnostics (PRD-C03-001 §4.10).
type CursorCLIProbe func(ctx context.Context, projectDir string) error

func defaultCursorCLIProbe(ctx context.Context, projectDir string) error {
	adapter := cursoradapter.NewAdapter(projectDir)
	return adapter.IsAvailable(ctx)
}

func addCursorCLIChecks(ctx context.Context, projectDir string, probe CursorCLIProbe, addCheck func(name, status, message string)) {
	if probe == nil {
		probe = defaultCursorCLIProbe
	}
	err := probe(ctx, projectDir)
	if err == nil {
		addCheck("cursor-cli", "ok", "cursor agent CLI available on PATH")
		return
	}

	var auth *cursoradapter.AuthError
	if errors.As(err, &auth) {
		msg := fmt.Sprintf("Cursor Agent CLI authentication required — run `%s`", cursoradapter.LoginHint)
		if strings.TrimSpace(auth.Detail) != "" {
			msg = fmt.Sprintf("Cursor Agent CLI authentication required (%s) — run `%s`", auth.Detail, cursoradapter.LoginHint)
		}
		addCheck("cursor-cli-auth", "warn", msg)
		return
	}

	msg := strings.TrimSpace(err.Error())
	if strings.Contains(msg, "not found on PATH") {
		msg += " — install Cursor Agent CLI before running `hero` TUI"
	}
	addCheck("cursor-cli", "warn", msg)
}
