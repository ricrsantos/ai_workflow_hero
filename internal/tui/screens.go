package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func (m model) renderContent() string {
	switch m.screen {
	case screenStatus:
		return m.renderStatus()
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
		b.WriteString(mutedStyle.Render("No active cycle. Run /hero-new, then /hero-start."))
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

func (m model) renderArtifacts() string {
	var b strings.Builder
	if len(m.artifacts.Artifacts) == 0 {
		b.WriteString(mutedStyle.Render(emptyCycleScreenMessage("artifacts", m.artifacts.CycleNumber)))
		return b.String()
	}
	title := "Artifacts"
	if m.artifacts.CycleNumber > 0 {
		title = fmt.Sprintf("Artifacts — C%d", m.artifacts.CycleNumber)
	}
	b.WriteString(headerStyle.Render(title))
	b.WriteByte('\n')
	b.WriteByte('\n')

	kindW, labelW := artifactColumnWidths(m.artifacts.Artifacts)
	header := fmt.Sprintf(" %s %s %s %s",
		padRight("Time", 8),
		padRight("Kind", kindW),
		padRight("Label", labelW),
		"Path",
	)
	b.WriteString(mutedStyle.Render(header))
	b.WriteByte('\n')
	for _, a := range m.artifacts.Artifacts {
		label := a.Label
		if label == "" {
			label = a.Kind
		}
		line := fmt.Sprintf(" %s %s %s %s",
			padRight(formatArtifactTime(a.CreatedAt), 8),
			padRight(a.Kind, kindW),
			padRight(label, labelW),
			a.Path,
		)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func artifactColumnWidths(arts []store.Artifact) (kindW, labelW int) {
	kindW, labelW = len("Kind"), len("Label")
	for _, a := range arts {
		kindW = max(kindW, len(a.Kind))
		label := a.Label
		if label == "" {
			label = a.Kind
		}
		labelW = max(labelW, len(label))
	}
	return kindW + 1, labelW + 1
}

func formatArtifactTime(ts string) string {
	if strings.TrimSpace(ts) == "" {
		return "—"
	}
	return formatEventTimeLocal(ts)
}

func (m model) renderCosts() string {
	var b strings.Builder
	if len(m.metrics.Rows) == 0 {
		b.WriteString(mutedStyle.Render(emptyCycleScreenMessage("metrics", m.metrics.CycleNumber)))
		return b.String()
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("Costs — C%d %s", m.metrics.CycleNumber, m.metrics.Title)))
	b.WriteByte('\n')
	b.WriteByte('\n')

	stageW, agentW, modelW := costColumnWidths(m.metrics.Rows)
	header := fmt.Sprintf(" %s %s %s %s %s %s %s",
		padRight("Stage", stageW),
		padRight("Agent", agentW),
		padRight("Model", modelW),
		padLeft("In", 8),
		padLeft("Out", 8),
		padLeft("Cost", 10),
		padLeft("Time", 7),
	)
	b.WriteString(mutedStyle.Render(header))
	b.WriteByte('\n')

	for _, r := range m.metrics.Rows {
		line := fmt.Sprintf(" %s %s %s %s %s %s %s",
			padRight(r.Stage, stageW),
			padRight(r.Agent, agentW),
			padRight(r.Model, modelW),
			padLeft(fmt.Sprintf("%d", r.InputTokens), 8),
			padLeft(fmt.Sprintf("%d", r.OutputTokens), 8),
			padLeft(formatCostCell(r.CostUSD), 10),
			padLeft(formatDurationMMSS(r.DurationMS), 7),
		)
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
		line := fmt.Sprintf(" %s  %-18s %s", formatEventTimeLocal(e.TS), e.Type, payload)
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
			list.WriteString(selectedStyle.Render("▸ " + line))
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
		content = m.renderContent()
		if m.screenHasContentScroll() {
			content = m.clipScrolledContent(content, contentH)
		}
	}
	content = strings.TrimRight(content, "\n") + "\n"

	return topStr + content + bottomStr
}

func screenTabBar(active screen) string {
	names := []string{"Chat", "Status", "Artifacts", "Costs", "Events"}
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
		if m.chatSlashOverlayActive() {
			return "enter select · tab select · esc close · ↑↓"
		}
		return "tab mode · enter send · ↑↓ scroll · /hero-model · alt+1-5 screens · ctrl+q quit"
	}
	if m.screenHasContentScroll() {
		return "↑↓ scroll · alt+1-5 screens · / commands · ctrl+r refresh · ctrl+q quit"
	}
	return "alt+1-5 screens · / commands · ctrl+r refresh · ctrl+q quit"
}

func pendingApprovalStage(st cycle.StatusView) string {
	for _, s := range st.Stages {
		if s.Status == "PendingApproval" || s.HumanApproval == "Pending" {
			return s.Name
		}
	}
	return ""
}

func escalatedStage(st cycle.StatusView) string {
	for _, s := range st.Stages {
		if s.Status == "Escalated" {
			return s.Name
		}
	}
	return ""
}

func formatEventTimeLocal(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return truncateTS(ts)
		}
	}
	return t.Local().Format("15:04:05")
}

func truncateTS(ts string) string {
	if len(ts) >= 19 {
		return ts[11:19]
	}
	return ts
}

func costColumnWidths(rows []cycle.MetricsRow) (stageW, agentW, modelW int) {
	stageW, agentW, modelW = len("Stage"), len("Agent"), len("Model")
	for _, r := range rows {
		stageW = max(stageW, len(r.Stage))
		agentW = max(agentW, len(r.Agent))
		modelW = max(modelW, len(r.Model))
	}
	return stageW + 1, agentW + 1, modelW + 1
}

func formatCostCell(cost float64) string {
	return fmt.Sprintf("$%.4f", cost)
}

// formatDurationMMSS renders stage duration as mm:ss (truncated to whole seconds).
func formatDurationMMSS(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	totalSec := ms / 1000
	mins := totalSec / 60
	secs := totalSec % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) screenHasContentScroll() bool {
	switch m.screen {
	case screenStatus, screenArtifacts, screenCosts, screenEvents:
		return true
	default:
		return false
	}
}

func (m model) frameContentHeight() int {
	// Match renderFrame chrome: title+tabs, rule, status rules, status bar, footer.
	h := m.height - (2 + 1 + m.statusBarLineCount() + 1 + 1)
	if h < 3 {
		h = 3
	}
	return h
}

func (m model) maxContentOffset() int {
	n := countContentLines(m.renderContent())
	maxOff := n - m.frameContentHeight()
	if maxOff < 0 {
		return 0
	}
	return maxOff
}

func (m model) clampContentOffset() model {
	if !m.screenHasContentScroll() {
		return m
	}
	maxOff := m.maxContentOffset()
	if m.contentOffset > maxOff {
		m.contentOffset = maxOff
	}
	if m.contentOffset < 0 {
		m.contentOffset = 0
	}
	return m
}

func (m model) scrollContent(delta int) model {
	m.contentOffset += delta
	return m.clampContentOffset()
}

func (m model) clipScrolledContent(content string, height int) string {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return "\n"
	}
	if height < 1 {
		height = 1
	}
	lines := strings.Split(content, "\n")
	n := len(lines)
	if n <= height {
		return content + "\n"
	}
	offset := m.contentOffset
	maxOff := n - height
	if offset > maxOff {
		offset = maxOff
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + height
	if end > n {
		end = n
	}
	return strings.Join(lines[offset:end], "\n") + "\n"
}
