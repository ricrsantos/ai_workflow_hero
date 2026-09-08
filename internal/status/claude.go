package status

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	claudeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/claude"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// ClaudeCLIProbe checks Claude's non-interactive compatibility surface. It is
// injectable so status tests never depend on a user's PATH or credentials.
type ClaudeCLIProbe func(context.Context, string) error

// ClaudeStatus is the deterministic Status view for the opt-in Claude
// harness. Claude process/session fields describe the TUI-owned turn boundary;
// this command never starts a turn or exposes credentials.
type ClaudeStatus struct {
	Enabled          bool   `json:"enabled"`
	Model            string `json:"model,omitempty"`
	Projection       string `json:"projection"`
	CLI              string `json:"cli"`
	Available        bool   `json:"available"`
	Diagnostic       string `json:"diagnostic,omitempty"`
	Process          string `json:"process"`
	Session          string `json:"session"`
	Health           string `json:"health"`
	PermissionPaused bool   `json:"permissionPaused"`
}

func defaultClaudeCLIProbe(ctx context.Context, projectDir string) error {
	return claudeadapter.NewAdapter(projectDir).IsAvailable(ctx)
}

// ClaudeStatusFor reads Hero configuration and non-interactive availability.
// Disabled Claude is deliberately probe-free, preserving warn-free upgrades.
func ClaudeStatusFor(opts Options) ClaudeStatus {
	result := ClaudeStatus{
		Projection:       "not-enabled",
		CLI:              "disabled",
		Process:          "idle (turn-scoped)",
		Session:          "unbound",
		Health:           "disabled",
		PermissionPaused: false,
	}
	hero, err := install.LoadHeroJSON(opts.ProjectDir)
	if err != nil {
		result.Diagnostic = "Hero configuration unavailable"
		return result
	}
	cfg := install.HarnessConfigForTool(hero, "claude")
	result.Enabled = install.IsHarnessEnabled(hero, "claude")
	result.Model = strings.TrimSpace(cfg.Model)
	if !result.Enabled {
		return result
	}

	if info, statErr := os.Stat(filepath.Join(opts.ProjectDir, ".claude")); statErr == nil && info.IsDir() {
		result.Projection = "present"
	} else {
		result.Projection = "missing"
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			result.Diagnostic = "cannot inspect .claude/ projection"
		} else {
			result.Diagnostic = "Claude is enabled but .claude/ projection is missing; enable Claude with /harness"
		}
	}

	probe := opts.ClaudeCLIProbe
	if probe == nil {
		probe = defaultClaudeCLIProbe
	}
	if probeErr := probe(context.Background(), opts.ProjectDir); probeErr != nil {
		result.CLI = "unavailable"
		result.Available = false
		result.Diagnostic = joinClaudeDiagnostics(result.Diagnostic, claudeStatusCLIProblem(probeErr))
	} else {
		result.CLI = "available"
		result.Available = true
	}

	result = applyClaudeRuntimeStatus(opts.ProjectDir, result)
	return result
}

func claudeStatusCLIProblem(err error) string {
	if err == nil {
		return "Claude Code CLI unavailable"
	}
	text := strings.TrimSpace(err.Error())
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "not on path"), strings.Contains(lower, "not found"):
		return "Claude Code CLI not found on PATH; install Claude Code 2.1.261+ and run /hero-continue"
	case strings.Contains(lower, "require 2.1.261"), strings.Contains(lower, "unsupported"):
		return text + "; upgrade Claude Code to 2.1.261+ and run /hero-continue"
	case strings.Contains(lower, "required flag"), strings.Contains(lower, "permission-prompt"):
		return text + "; install Claude Code 2.1.261+ with the required flags, then run /hero-continue"
	case strings.Contains(lower, "auth"), strings.Contains(lower, "login"):
		return "Claude Code authentication is required; authenticate with `claude login`, then run /hero-continue"
	default:
		return fmt.Sprintf("Claude Code unavailable: %s; fix Claude Code setup and run /hero-continue", text)
	}
}

func joinClaudeDiagnostics(current, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" {
		return next
	}
	if next == "" {
		return current
	}
	return current + "; " + next
}

func applyClaudeRuntimeStatus(projectDir string, result ClaudeStatus) ClaudeStatus {
	st, err := store.OpenProject(projectDir)
	if err != nil {
		return result
	}
	defer st.Close()
	cycleRow, err := st.GetActiveCycle()
	if err != nil {
		return result
	}
	stages, err := st.ListStages(cycleRow.ID)
	if err != nil {
		return result
	}
	for _, stage := range stages {
		if !strings.EqualFold(strings.TrimSpace(stage.HarnessID), "claude") {
			continue
		}
		result.Session = strings.TrimSpace(stage.HarnessSessionID)
		result.PermissionPaused = stage.HarnessPermissionPaused
		if result.Session == "" {
			result.Session = "bound stage (native id pending)"
		}
		switch stage.Status {
		case store.StageRunning:
			result.Process = "active turn (TUI-owned)"
			result.Health = "running (TUI-owned)"
		case store.StageFailed:
			result.Process = "stopped"
			result.Health = "failed"
		case store.StageCompleted:
			result.Process = "stopped"
			result.Health = "completed"
		default:
			result.Process = "idle (turn-scoped)"
			result.Health = strings.ToLower(stage.Status)
		}
		break
	}
	return result
}

// PrintClaudeStatus writes the human-readable Claude section used by
// `hero status`. It intentionally avoids session secrets and process details.
func PrintClaudeStatus(w io.Writer, result ClaudeStatus) {
	state := "disabled"
	if result.Enabled {
		state = "enabled"
	}
	model := result.Model
	if model == "" {
		model = "not set"
	}
	fmt.Fprintf(w, "Claude: %s (model: %s; projection: %s; CLI: %s)\n", state, model, result.Projection, result.CLI)
	fmt.Fprintf(w, "Claude runtime: process=%s; session=%s; health=%s; permission pause=%t\n", result.Process, result.Session, result.Health, result.PermissionPaused)
	if result.Diagnostic != "" {
		fmt.Fprintf(w, "Claude diagnostic: %s\n", result.Diagnostic)
	}
}
