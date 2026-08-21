package codex

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

const heroStartProbePrompt = "Reply with exactly: ok"

// PrepareHeroStart syncs .codex/agents frontmatter from workflow-config.yml,
// resets the managed Codex app-server, and probes one configured agent.
// When no agents use Codex, it is a no-op (design D9; UI-C06-001 §6).
func PrepareHeroStart(ctx context.Context, projectDir string, st *store.Store) error {
	adapter := NewAdapter(projectDir, st)
	return PrepareHeroStartWithAdapter(ctx, projectDir, st, adapter)
}

// PrepareHeroStartWithAdapter is the injectable PrepareHeroStart implementation for tests.
func PrepareHeroStartWithAdapter(ctx context.Context, projectDir string, st *store.Store, adapter *Adapter) error {
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

	slog.Info("codex prepare hero-start", "agents", len(agents), "project", projectDir)

	for _, name := range agents {
		agentCfg, ok := cfg.Agents[name]
		if !ok {
			continue
		}
		if err := SyncAgentDefinition(projectDir, name, agentCfg); err != nil {
			slog.Error("codex sync agent definition failed", "agent", name, "error", err)
			return err
		}
	}

	if err := adapter.ResetAppServer(ctx); err != nil {
		slog.Error("codex reset app-server failed", "error", err)
		return fmt.Errorf("reset codex app-server: %w", err)
	}

	probeAgent := agents[0]
	probeModel := ""
	if agentCfg, ok := cfg.Agents[probeAgent]; ok {
		probeModel = strings.TrimSpace(agentCfg.Model)
	}
	_, err = adapter.Execute(ctx, harness.ExecuteRequest{
		ProjectDir: projectDir,
		AgentName:  probeAgent,
		Prompt:     heroStartProbePrompt,
		Model:      probeModel,
		Stream:     false,
	})
	if err != nil {
		slog.Error("codex agent probe failed", "agent", probeAgent, "error", err)
		return fmt.Errorf(
			"codex agent %q model probe failed: %v. Exit Hero TUI, run `hero` again, and retry /hero-start",
			probeAgent,
			err,
		)
	}
	slog.Info("codex prepare hero-start complete", "probe_agent", probeAgent)
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
