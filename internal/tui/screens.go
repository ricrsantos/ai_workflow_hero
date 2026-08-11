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
		h := m.height - (5 + statusBarMaxLines)
		if h < 3 {
			h = 3
		}
		return m.renderConversation(h)
	case screenPalette:
		return m.renderPalette()
	case screenOutput:
		return m.renderOutput()
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
		b.WriteString(mutedStyle.Render(emptyCycleScreenMessage("artifacts", m.artifacts.CycleNumber)))
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
		b.WriteString(mutedStyle.Render(emptyCycleScreenMessage("metrics", m.metrics.CycleNumber)))
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
		b.WriteString(mutedStyle.Render(emptyCycleScreenMessage("events", m.events.CycleNumber)))
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

// emptyCycleScreenMessage explains an empty Artifacts/Costs/Events view.
// CycleNumber 0 means there is no active cycle (not a literal "C0").
func emptyCycleScreenMessage(kind string, cycleNumber int) string {
	if cycleNumber <= 0 {
		return "No active cycle. Run /hero-new to start."
	}
	return fmt.Sprintf("No %s for cycle C%d.", kind, cycleNumber)
}

func (m model) renderPalette() string {
	var b strings.Builder
	if m.pickingModel {
		b.WriteString(headerStyle.Render("Models"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("Type to filter · ↑↓ navigate · enter select · esc close"))
	} else {
		b.WriteString(headerStyle.Render("Commands"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("Type to filter · ↑↓ navigate · PgUp/PgDn · enter run · esc close"))
	}
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
		if m.pickingModel {
			b.WriteString(mutedStyle.Render("No matching models."))
		} else {
			b.WriteString(mutedStyle.Render("No matching commands."))
		}
		return b.String()
	}

	m = m.ensurePaletteVisible()
	listH := m.paletteListHeight()
	start := m.paletteOffset
	end := start + listH
	if end > len(items) {
		end = len(items)
	}

	var list strings.Builder
	if start > 0 {
		list.WriteString(mutedStyle.Render(fmt.Sprintf("  ▲ %d more above", start)))
		list.WriteByte('\n')
	}
	for i := start; i < end; i++ {
		item := items[i]
		line := fmt.Sprintf(" %s — %s", item.label, item.hint)
		if i == m.paletteIndex {
			list.WriteString(selectedStyle.Render("▸ "+line))
		} else {
			list.WriteString("  " + line)
		}
		list.WriteByte('\n')
	}
	below := len(items) - end
	if below > 0 {
		list.WriteString(mutedStyle.Render(fmt.Sprintf("  ▼ %d more below", below)))
		list.WriteByte('\n')
	}

	title := fmt.Sprintf(" %d–%d of %d ", start+1, end, len(items))
	panelWidth := m.width - 2
	if panelWidth < 24 {
		panelWidth = 24
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Width(panelWidth).
		Render(strings.TrimRight(list.String(), "\n"))
	b.WriteString(mutedStyle.Render(title))
	b.WriteByte('\n')
	b.WriteString(panel)
	return b.String()
}

// paletteListHeight is how many command rows fit in the scrollable panel
// (scroll cues ▲/▼ are reserved via chrome, not counted here).
func (m model) paletteListHeight() int {
	// title+tabs, rules (3), Commands header, hint, prompt, blank, range line,
	// border (2), optional ▲/▼ (2), status bar, footer.
	chrome := 14 + m.statusBarLineCount() + 2 // status content + separators around it
	h := m.height - chrome
	if h < 4 {
		h = 4
	}
	return h
}

// ensurePaletteVisible keeps paletteIndex inside the scrolled window.
func (m model) ensurePaletteVisible() model {
	items := m.filteredPaletteItems()
	n := len(items)
	if n == 0 {
		m.paletteOffset = 0
		m.paletteIndex = 0
		return m
	}
	if m.paletteIndex < 0 {
		m.paletteIndex = 0
	}
	if m.paletteIndex >= n {
		m.paletteIndex = n - 1
	}
	vh := m.paletteListHeight()
	if m.paletteIndex < m.paletteOffset {
		m.paletteOffset = m.paletteIndex
	}
	if m.paletteIndex >= m.paletteOffset+vh {
		m.paletteOffset = m.paletteIndex - vh + 1
	}
	maxOff := n - vh
	if maxOff < 0 {
		maxOff = 0
	}
	if m.paletteOffset < 0 {
		m.paletteOffset = 0
	}
	if m.paletteOffset > maxOff {
		m.paletteOffset = maxOff
	}
	return m
}

func (m model) renderFrame() string {
	var top strings.Builder
	top.WriteString(titleStyle.Render("AI Hero"))
	top.WriteString(mutedStyle.Render("  ·  "))
	top.WriteString(screenTabBar(m.screen))
	top.WriteByte('\n')
	top.WriteString(strings.Repeat("─", max(20, m.width)))
	top.WriteByte('\n')

	var bottom strings.Builder
	bottom.WriteString(strings.Repeat("─", max(20, m.width)))
	bottom.WriteByte('\n')
	bottom.WriteString(m.renderStatusBar())
	bottom.WriteByte('\n')
	bottom.WriteString(strings.Repeat("─", max(20, m.width)))
	bottom.WriteByte('\n')
	bottom.WriteString(footerStyle.Render(m.footerHints()))

	topStr := top.String()
	bottomStr := bottom.String()
	chrome := countContentLines(topStr) + countContentLines(bottomStr)
	contentH := m.height - chrome
	if contentH < 3 {
		contentH = 3
	}

	var content string
	if m.screen == screenConversation {
		content = m.renderConversation(contentH)
	} else {
		// Leave other screens as-is (no Height/Width wrap — breaks bordered panes).
		content = m.renderContent()
	}
	content = strings.TrimRight(content, "\n") + "\n"

	return topStr + content + bottomStr
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
		return "esc close · enter select · ↑↓ scroll"
	}
	if m.screen == screenOutput {
		return "esc close · ↑↓ scroll · PgUp/PgDn"
	}
	if m.screen == screenConversation {
		if m.streaming {
			return "↑↓ scroll · ctrl+c interrupt"
		}
		return "tab mode · enter send · ↑↓ scroll · /hero-model · alt+1-6 screens · ctrl+q quit"
	}
	return "alt+1-6 screens · / commands · ctrl+r refresh · d dispatch · ctrl+q quit"
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
