package telegram

import (
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

const telegramMaxActionableIDs = 3

// CompactFindingsStatus renders finding counts and the first actionable finding
// IDs from the additive status JSON (UI-C15-001 §13). It returns an empty
// string when there is no findings or escalation data to show.
func CompactFindingsStatus(view cycle.StatusView) string {
	if view.Findings == nil && len(view.AvailableActions) == 0 {
		return ""
	}

	var lines []string
	if view.Findings != nil {
		c := view.Findings.Counts
		total := c.Open + c.Reopened + c.Done + c.DeferredTodo
		if total == 0 {
			lines = append(lines, "Findings none")
		} else {
			lines = append(lines, fmt.Sprintf(
				"Findings: open %d · reopened %d · done %d · ToDo %d",
				c.Open, c.Reopened, c.Done, c.DeferredTodo,
			))
			if ids := firstActionableFindingIDs(view.Findings.Items, telegramMaxActionableIDs); len(ids) > 0 {
				lines = append(lines, "Actionable: "+strings.Join(ids, ", "))
			}
		}
	}
	if actions := formatTelegramAvailableActions(view.AvailableActions); actions != "" {
		lines = append(lines, actions)
	}
	return strings.Join(lines, "\n")
}

func firstActionableFindingIDs(items []cycle.StatusFindingRow, limit int) []string {
	if limit <= 0 {
		return nil
	}
	out := make([]string, 0, limit)
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "open", "reopened":
			if id := strings.TrimSpace(item.ID); id != "" {
				out = append(out, id)
				if len(out) >= limit {
					return out
				}
			}
		}
	}
	return out
}

func formatTelegramAvailableActions(actions []string) string {
	if len(actions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(actions))
	for _, a := range actions {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if !strings.HasPrefix(a, "/") {
			a = "/" + a
		}
		parts = append(parts, a)
	}
	if len(parts) == 0 {
		return ""
	}
	return "Actions: " + strings.Join(parts, ", ")
}
