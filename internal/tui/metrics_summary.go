package tui

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// turnCostUSD prices one completed turn from the model catalog. Unknown or
// unpriced models return 0 — Hero shows a blank cost rather than inventing one.
//
// This is the TUI runtime's replacement for the orchestrator's old Metrics
// Procedure: the harness reports the tokens, the catalog supplies the rate,
// and no agent is asked to estimate either.
func (m model) turnCostUSD(modelID string, inputTokens, outputTokens int64) float64 {
	if m.propsSvc == nil {
		return 0
	}
	cost, warning := modelprops.EstimateCatalogCostUSD(m.propsSvc.Catalog, modelID, inputTokens, outputTokens)
	if warning != "" {
		slog.Debug("tui stage cost left unset", "model", modelID, "reason", warning)
	}
	return cost
}

// stageMetricsSummary renders the per-stage metrics block that the
// orchestration agent used to print from its own estimates. The TUI owns it
// now because only the TUI sees real harness usage.
//
// Returns "" when the stage recorded nothing, so a stage that never ran an
// agent does not emit an empty block.
func (m model) stageMetricsSummary(stage string) string {
	stage = strings.TrimSpace(stage)
	if stage == "" || m.svc == nil || m.svc.Store == nil {
		return ""
	}
	cycleRow, err := m.svc.SessionCycle()
	if err != nil || cycleRow == nil {
		return ""
	}
	rows, err := m.svc.Store.ListMetrics(cycleRow.ID)
	if err != nil {
		slog.Error("tui stage metrics lookup failed", "stage", stage, "error", redact.Error(err))
		return ""
	}
	var stageRows []store.Metric
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.StageName), stage) {
			stageRows = append(stageRows, r)
		}
	}
	if len(stageRows) == 0 {
		return ""
	}
	// Deterministic order so a multi-agent stage reads the same way every run.
	sort.Slice(stageRows, func(i, j int) bool { return stageRows[i].Agent < stageRows[j].Agent })

	var in, out, ms int64
	var cost float64
	models := make([]string, 0, len(stageRows))
	seen := map[string]struct{}{}
	for _, r := range stageRows {
		in += r.InputTokens
		out += r.OutputTokens
		ms += r.DurationMS
		cost += r.CostUSD
		id := strings.TrimSpace(r.Model)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	if in == 0 && out == 0 && ms == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "→ Metrics: %s\n", stage)
	if len(models) > 0 {
		fmt.Fprintf(&b, "  Model: %s\n", strings.Join(models, ", "))
	}
	fmt.Fprintf(&b, "  Input: %s tokens | Output: %s tokens | Total: %s tokens\n",
		formatTokenCount(in), formatTokenCount(out), formatTokenCount(in+out))
	fmt.Fprintf(&b, "  Duration: %s\n", formatMetricsDuration(ms))
	if cost > 0 {
		fmt.Fprintf(&b, "  Cost: ~$%.4f\n", cost)
	} else {
		// An unpriced model is a real state (ChatGPT-subsidized Codex, model
		// missing from the catalog). Saying so beats printing $0.0000.
		b.WriteString("  Cost: not priced for this model\n")
	}
	b.WriteString("→ Full details: run `hero metrics` (or `hero metrics --json`)\n")
	return b.String()
}

func formatMetricsDuration(ms int64) string {
	if ms <= 0 {
		return "0s"
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d / time.Minute)
	secs := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%02ds", mins, secs)
}
