package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

func (m model) renderContent() string {
	switch m.screen {
	case screenStatus:
		return m.renderStatus()
	case screenApprovals:
		return m.renderApprovals()
	case screenArtifacts:
		return m.renderArtifacts()
	case screenCosts:
		return m.renderCosts()
	case screenEvents:
		return m.renderEvents()
	case screenConversation:
		return m.renderConversation()
	case screenPalette:
		return m.renderPalette()
	default:
		return ""
	}
}

func (m model) renderStatus() string {
	var b strings.Builder
	if m.status.CycleNumber == 0 && len(m.status.Stages) == 0 {
		b.WriteString(mutedStyle.Render("No active cycle. Run /hero-new, then /hero-start (or `hero cycle new`)."))
		return b.String()
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("Cycle C%d — %s", m.status.CycleNumber, m.status.Title)))
	b.WriteByte('\n')
	if m.status.Objective != "" {
		b.WriteString(mutedStyle.Render(m.status.Objective))
		b.WriteByte('\n')
	}
	if m.status.Status != "" {
		b.WriteString(infoStyle.Render("Status: "+m.status.Status) + "\n")
	}
	b.WriteByte('\n')
	b.WriteString(headerStyle.Render("Stages"))
	b.WriteByte('\n')
	for _, st := range m.status.Stages {
		icon := stageStatusIcon(st.Status)
		line := fmt.Sprintf(" %s %-18s %-16s %-8s %s",
			icon, st.Name, st.Status, st.Iteration, st.HumanApproval)
		b.WriteString(stageStatusStyle(st.Status).Render(line))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m model) renderApprovals() string {
	var b strings.Builder
	pending := pendingApprovalStage(m.status)
	if pending == "" {
		b.WriteString(mutedStyle.Render("No stage pending approval."))
		b.WriteString("\n\n")
		b.WriteString(footerStyle.Render("Keys: a approve · r reject · c cancel · f finish · d dispatch"))
		return b.String()
	}
	b.WriteString(headerStyle.Render("Pending approval"))
	b.WriteByte('\n')
	b.WriteString(warnStyle.Render(fmt.Sprintf("⚠ Stage %q awaits your decision.", pending)))
	b.WriteByte('\n')
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("Keys: a approve · r reject · c cancel cycle · f finish · d dispatch"))
	return b.String()
}

func (m model) renderArtifacts() string {
	var b strings.Builder
	if len(m.artifacts.Artifacts) == 0 {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("No artifacts for cycle C%d.", m.artifacts.CycleNumber)))
		return b.String()
	}
	b.WriteString(headerStyle.Render("Artifacts"))
	b.WriteByte('\n')
	for _, a := range m.artifacts.Artifacts {
		label := a.Label
		if label == "" {
			label = a.Kind
		}
		line := fmt.Sprintf(" %-20s %-10s %s", label, a.Kind, a.Path)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func (m model) renderCosts() string {
	var b strings.Builder
	if len(m.metrics.Rows) == 0 {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("No metrics for cycle C%d.", m.metrics.CycleNumber)))
		return b.String()
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("Costs — C%d %s", m.metrics.CycleNumber, m.metrics.Title)))
	b.WriteByte('\n')
	for _, r := range m.metrics.Rows {
		line := fmt.Sprintf(" %-12s %-16s %-14s in:%-6d out:%-6d $%.4f %dms",
			r.Stage, r.Agent, r.Model, r.InputTokens, r.OutputTokens, r.CostUSD, r.DurationMS)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(infoStyle.Render(fmt.Sprintf("Total: %d in / %d out tokens · ~$%.4f USD",
		m.metrics.TotalIn, m.metrics.TotalOut, m.metrics.TotalCost)))
	return b.String()
}

func (m model) renderEvents() string {
	var b strings.Builder
	if len(m.events.Events) == 0 {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("No events for cycle C%d.", m.events.CycleNumber)))
		return b.String()
	}
	b.WriteString(headerStyle.Render("Recent events"))
	b.WriteByte('\n')
	for _, e := range m.events.Events {
		payload := e.PayloadJSON
		if len(payload) > 48 {
			payload = payload[:45] + "..."
		}
		line := fmt.Sprintf(" %s  %-18s %s", truncateTS(e.TS), e.Type, payload)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func (m model) renderPalette() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Commands"))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("Type to filter · ↑↓ navigate · enter run · esc close"))
	b.WriteByte('\n')
	prompt := "/"
	if m.paletteFilter != "" {
		prompt = "/" + m.paletteFilter
	}
	b.WriteString(infoStyle.Render(prompt))
	b.WriteByte('\n')
	b.WriteByte('\n')
	items := m.filteredPaletteItems()
	if len(items) == 0 {
		b.WriteString(mutedStyle.Render("No matching commands."))
		return b.String()
	}
	for i, item := range items {
		line := fmt.Sprintf(" %s — %s", item.label, item.hint)
		if i == m.paletteIndex {
			b.WriteString(selectedStyle.Render("▸ "+line) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}
	return b.String()
}

func (m model) renderFrame() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Hero TUI"))
	b.WriteString(mutedStyle.Render("  ·  "))
	b.WriteString(screenTabBar(m.screen))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", max(20, m.width)))
	b.WriteByte('\n')
	content := m.renderContent()
	b.WriteString(lipgloss.NewStyle().Width(m.width).Render(content))
	if m.flash != "" {
		b.WriteByte('\n')
		if m.flashErr {
			b.WriteString(errorStyle.Render("✗ " + m.flash))
		} else {
			b.WriteString(successStyle.Render("✓ " + m.flash))
		}
	}
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", max(20, m.width)))
	b.WriteByte('\n')
	b.WriteString(footerStyle.Render(m.footerHints()))
	return b.String()
}

func screenTabBar(active screen) string {
	names := []string{"Status", "Approvals", "Artifacts", "Costs", "Events", "Chat"}
	var parts []string
	for i, name := range names {
		if screen(i) == active {
			parts = append(parts, infoStyle.Render(name))
		} else {
			parts = append(parts, mutedStyle.Render(name))
		}
	}
	return strings.Join(parts, " │ ")
}

func (m model) footerHints() string {
	if m.screen == screenPalette {
		return "esc close · enter select"
	}
	if m.screen == screenConversation {
		if m.streaming {
			return "ctrl+c interrupt · esc wait"
		}
		return "enter submit · esc clear · /hero-help · 1-6 screens · q quit"
	}
	return "1-6 screens · / commands · ctrl+r refresh · d dispatch · q quit"
}

func pendingApprovalStage(st cycle.StatusView) string {
	for _, s := range st.Stages {
		if s.Status == "PendingApproval" || s.HumanApproval == "Pending" {
			return s.Name
		}
	}
	return ""
}

func truncateTS(ts string) string {
	if len(ts) >= 19 {
		return ts[11:19]
	}
	return ts
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
