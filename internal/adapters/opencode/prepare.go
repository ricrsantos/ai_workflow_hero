package opencode

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

const heroStartProbePrompt = "Reply with exactly: ok"

// PrepareHeroStart syncs .opencode/agents frontmatter from workflow-config.yml,
// resets the managed opencode serve process, and probes one configured agent.
// When no agents use OpenCode, it is a no-op.
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

	for _, name := range agents {
		agentCfg, ok := cfg.Agents[name]
		if !ok {
			continue
		}
		if err := SyncAgentDefinition(projectDir, name, agentCfg); err != nil {
			return err
		}
	}

	if err := adapter.ResetServe(ctx); err != nil {
		return fmt.Errorf("reset opencode serve: %w", err)
	}

	probeAgent := agents[0]
	_, err = adapter.Execute(ctx, harness.ExecuteRequest{
		ProjectDir: projectDir,
		AgentName:  probeAgent,
		Prompt:     heroStartProbePrompt,
		Stream:     false,
	})
	if err != nil {
		return fmt.Errorf(
			"opencode agent %q model probe failed: %v. Exit Hero TUI, run `hero` again, and retry /hero-start",
			probeAgent,
			err,
		)
	}
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
