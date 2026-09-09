package claude

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

// PrepareHeroStart synchronizes the marker-delimited Claude agent fields and
// validates CLI compatibility. It performs no turn, daemon, or persistent
// process work, and is a no-op when no configured agent uses Claude.
func PrepareHeroStart(ctx context.Context, projectDir string) error {
	return PrepareHeroStartWithAdapter(ctx, projectDir, NewAdapter(projectDir))
}

// PrepareHeroStartWithAdapter provides injected protocol probes for tests.
func PrepareHeroStartWithAdapter(ctx context.Context, projectDir string, adapter *Adapter) error {
	cfg, _, err := workflowconfig.LoadCurrent(projectDir)
	if err != nil {
		return err
	}
	agents := AgentsUsingHarness(cfg, adapterName)
	if len(agents) == 0 {
		return nil
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return fmt.Errorf("read hero.json: %w", err)
	}
	if !installHarnessEnabled(hero, adapterName) {
		return nil
	}
	if adapter == nil {
		return fmt.Errorf("Claude prepare adapter is required")
	}

	slog.Info("claude prepare hero-start", "agents", len(agents), "project", projectDir)
	// Probe before any filesystem plan is written. A missing/incompatible CLI
	// thus cannot change a user's projected agent file.
	if err := adapter.IsAvailable(ctx); err != nil {
		slog.Error("claude compatibility probe failed", "error", err)
		return fmt.Errorf("Claude Code compatibility probe failed: %v. Exit Hero TUI, install Claude Code %s or newer with the required stream-json flags, then run `hero` again and retry /hero-start", err, MinimumCLIVersion)
	}

	plans := make([]agentDefinitionPlan, 0, len(agents))
	for _, name := range agents {
		plan, err := planAgentDefinition(projectDir, name, cfg.Agents[name])
		if err != nil {
			slog.Error("claude agent preparation failed", "agent", name, "error", err)
			return fmt.Errorf("Claude agent %q preparation failed: %v. Repair the Hero managed fields in .claude/agents/%s.md, then retry /hero-start", name, err, name)
		}
		plans = append(plans, plan)
	}
	for _, plan := range plans {
		if err := writeAgentDefinition(plan); err != nil {
			slog.Error("claude agent synchronization failed", "path", plan.path, "error", err)
			return fmt.Errorf("synchronize Claude managed agent fields: %w", err)
		}
	}
	slog.Info("claude hero-start preparation completed", "agents", len(plans))
	return nil
}

func installHarnessEnabled(hero install.HeroJSON, harnessID string) bool {
	for _, id := range install.ListEnabledHarnesses(hero) {
		if strings.EqualFold(id, harnessID) {
			return true
		}
	}
	return false
}
