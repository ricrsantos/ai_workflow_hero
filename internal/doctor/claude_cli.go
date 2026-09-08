package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	claudeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/claude"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

// ClaudeCLIProbe checks the non-interactive Claude Code CLI contract for
// Doctor. Probes may run --version/--help, but must never start a turn or log
// in, which keeps diagnostics deterministic and credential-free.
type ClaudeCLIProbe func(ctx context.Context, projectDir string) error

func defaultClaudeCLIProbe(ctx context.Context, projectDir string) error {
	return claudeadapter.NewAdapter(projectDir).IsAvailable(ctx)
}

func addClaudeCLIChecks(ctx context.Context, projectDir string, probe ClaudeCLIProbe, addCheck func(name, status, message string)) {
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil || !install.IsHarnessEnabled(hero, "claude") {
		return
	}
	if probe == nil {
		probe = defaultClaudeCLIProbe
	}
	if err := probe(ctx, projectDir); err != nil {
		addCheck("claude-cli", "warn", claudeCLIProblem(err))
		return
	}
	addCheck("claude-cli", "ok", "Claude Code CLI available on PATH (2.1.261+; stream-json and permission flags verified)")
}

func claudeCLIProblem(err error) string {
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
	case strings.Contains(lower, "missing required flag"), strings.Contains(lower, "required flag"), strings.Contains(lower, "permission-prompt"):
		return text + "; install Claude Code 2.1.261+ with the required stream-json/permission flags, then run /hero-continue"
	case strings.Contains(lower, "auth"), strings.Contains(lower, "login"):
		return "Claude Code authentication is required; authenticate with `claude login`, then run /hero-continue"
	default:
		return fmt.Sprintf("Claude Code unavailable: %s; fix Claude Code setup and run /hero-continue", text)
	}
}

func addClaudeProjectionChecks(projectDir string, addCheck func(name, status, message string)) {
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil || !install.IsHarnessEnabled(hero, "claude") {
		return
	}
	path := filepath.Join(projectDir, ".claude")
	info, statErr := os.Stat(path)
	if statErr == nil && info.IsDir() {
		addCheck("claude-projection", "ok", ".claude/ projection present")
		return
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		addCheck("claude-projection", "warn", fmt.Sprintf("cannot inspect .claude/ projection: %v", statErr))
		return
	}
	addCheck("claude-projection", "warn", "Claude is enabled but .claude/ projection is missing — enable Claude with /harness, then run /hero-continue")
}

func addClaudeRuntimeChecks(projectDir string, addCheck func(name, status, message string)) {
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil || !install.IsHarnessEnabled(hero, "claude") {
		return
	}
	// Claude deliberately has no daemon or process registry. The active TUI
	// owns its child, session binding, health, and permission pause; Doctor can
	// report that boundary without starting or probing a second turn.
	addCheck("claude-process", "ok", "Claude process is turn-scoped; no persistent process is managed by Hero")
	addCheck("claude-session", "ok", "Claude sessions are bound to their harness and stage; active turns are visible in the TUI")
	addCheck("claude-health", "ok", "Claude health and permission pauses are monitored by the active TUI turn")
}

// claudeMarkerTools returns marker configuration without mutating hero.json.
// Explicit harness state wins over legacy cli.tools for Claude, so a disabled
// Claude marker remains warn-only after an upgrade.
func claudeMarkerTools(hero install.HeroJSON) []string {
	tools := make(map[string]bool)
	for _, id := range hero.CLI.Tools {
		id = strings.TrimSpace(strings.ToLower(id))
		if id != "" {
			tools[id] = true
		}
	}
	if cfg, ok := hero.Harnesses["claude"]; ok {
		if cfg.Enabled {
			tools["claude"] = true
		} else {
			delete(tools, "claude")
		}
	}
	out := make([]string, 0, len(tools))
	for id := range tools {
		out = append(out, id)
	}
	return out
}
