package harnessmgr

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

// ExecutePair describes a resolved harness execution target.
type ExecutePair struct {
	HarnessID string
	Model     string
	Adapter   harness.HarnessAdapter
}

// FallbackAttempt records one step in the fallback chain (ADR-033).
type FallbackAttempt struct {
	AgentName string
	HarnessID string
	Model     string
	Err       error
}

// ResolveExecutePair resolves agent pair → fallback_model pair (ADR-033).
// Freechat default is not a third Execute fallback; use it as the agent pair for freechat only.
func ResolveExecutePair(
	ctx context.Context,
	reg Registry,
	hero install.HeroJSON,
	agentHarness, agentModel, fallbackHarness, fallbackModel string,
) (ExecutePair, []FallbackAttempt, error) {
	if reg == nil {
		return ExecutePair{}, nil, fmt.Errorf("harness registry unavailable")
	}
	attempts := []FallbackAttempt{}

	try := func(label, harnessID, model string) (ExecutePair, bool) {
		harnessID = strings.TrimSpace(strings.ToLower(harnessID))
		model = strings.TrimSpace(model)
		if harnessID == "" || model == "" {
			return ExecutePair{}, false
		}
		if !install.IsHarnessEnabled(hero, harnessID) {
			attempts = append(attempts, FallbackAttempt{AgentName: label, HarnessID: harnessID, Model: model,
				Err: fmt.Errorf("harness %s is not enabled", harnessID)})
			return ExecutePair{}, false
		}
		adapter, err := reg.Adapter(harnessID)
		if err != nil {
			attempts = append(attempts, FallbackAttempt{AgentName: label, HarnessID: harnessID, Model: model, Err: err})
			return ExecutePair{}, false
		}
		if err := adapter.IsAvailable(ctx); err != nil {
			attempts = append(attempts, FallbackAttempt{AgentName: label, HarnessID: harnessID, Model: model, Err: err})
			return ExecutePair{}, false
		}
		slug := model
		if harnessID == "cursor" {
			slug = install.ResolveHarnessModelSlug(install.HarnessConfig{Model: model})
		}
		return ExecutePair{HarnessID: harnessID, Model: slug, Adapter: adapter}, true
	}

	if pair, ok := try("agent", agentHarness, agentModel); ok {
		return pair, attempts, nil
	}
	if pair, ok := try("fallback_model", fallbackHarness, fallbackModel); ok {
		return pair, attempts, nil
	}
	return ExecutePair{}, attempts, fmt.Errorf("no available harness/model pair")
}

// FormatFallbackWarning formats the TUI fallback warning (UI-C04-001 §6).
func FormatFallbackWarning(fromAgent, fromHarness, fromModel, toHarness, toModel string) string {
	return fmt.Sprintf("⚠ Fallback: %s %s/%s unavailable\n→ Using fallback_model %s/%s",
		fromAgent, fromHarness, fromModel, toHarness, toModel)
}

// FormatHardStop formats the hard-stop message when agent + fallback both fail (UI-C04-001 §6).
func FormatHardStop(agentName, harnessID, model string, attempts []FallbackAttempt) string {
	var b strings.Builder
	fmt.Fprintf(&b, "✗ Cannot run %s: harness %s is not available", agentName, harnessID)
	if len(attempts) > 1 {
		last := attempts[len(attempts)-1]
		if last.HarnessID != "" {
			fmt.Fprintf(&b, "\n  Fallback %s/%s also failed.", last.HarnessID, last.Model)
		}
	}
	b.WriteString("\n\n  Suggestion: install/enable the harness or fix workflow-config.yml,")
	b.WriteString("\n  then run /hero-continue.")
	return b.String()
}

// ListModels aggregates model ids from enabled harness adapters.
func ListModels(ctx context.Context, reg Registry, hero install.HeroJSON) ([]ModelOption, error) {
	var out []ModelOption
	for _, id := range reg.EnabledIDs(hero) {
		adapter, err := reg.Adapter(id)
		if err != nil {
			continue
		}
		lister, ok := adapter.(harness.ModelLister)
		if !ok {
			continue
		}
		models, err := lister.ListModels(ctx)
		if err != nil {
			slogWarnListModels(id, err)
			continue
		}
		for _, m := range models {
			out = append(out, ModelOption{Model: m, Harness: id})
		}
	}
	return out, nil
}

// ModelOption is one row in the /hero-model pair picker.
type ModelOption struct {
	Model   string
	Harness string
}

func slogWarnListModels(harnessID string, err error) {
	slog.Warn("list models failed", "harness", harnessID, "error", err)
}
