package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func (m model) renderContent() string {
	switch m.screen {
	case screenConfig:
		return m.renderConfig()
	case screenSettings:
		return m.renderSettings()
	case screenStatus:
		return m.renderStatus()
	case screenArtifacts:
		return m.renderArtifacts()
	case screenCosts:
		return m.renderCosts()
	case screenEvents:
		return m.renderEvents()
	case screenConversation:
		return m.renderConversation(m.frameContentHeight())
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
	if m.pickingProps {
		return m.renderPropertyPicker()
	}
	if m.harnessResetAwaitingOpen {
		var b strings.Builder
		frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
		b.WriteString(headerStyle.Render("/harness-reset · loading harnesses"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render(frame + " Preparing harness list…"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("esc cancel"))
		return b.String()
	}
	if m.propsAwaitingRefresh {
		var b strings.Builder
		frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
		b.WriteString(headerStyle.Render("/model · loading model properties"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render(frame + " Waiting for harness refresh…"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("esc cancel"))
		return b.String()
	}
	var b strings.Builder
	compact := m.frameContentHeight() < 10
	switch {
	case m.pickingModel && m.modelPickerHarness != "":
		b.WriteString(headerStyle.Render("/model · " + harnessDisplayName(m.modelPickerHarness)))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("↑↓ navigate · enter select · esc back"))
	case m.pickingModel:
		b.WriteString(headerStyle.Render("/model · select harness"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("↑↓ navigate · enter · esc close"))
	case m.pickingHarness:
		b.WriteString(headerStyle.Render("Harnesses"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("↑↓ navigate · space toggle · enter save · esc cancel"))
	case m.pickingHarnessReset:
		b.WriteString(headerStyle.Render("/harness-reset · select harness"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("↑↓ navigate · enter reset · esc cancel"))
	default:
		b.WriteString(headerStyle.Render("Commands"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("Type to filter · ↑↓ navigate · PgUp/PgDn · enter run · esc close"))
	}
	if !compact {
		b.WriteByte('\n')
		prompt := "/"
		if m.paletteFilter != "" {
			prompt = "/" + m.paletteFilter
		}
		b.WriteString(infoStyle.Render(prompt))
		b.WriteByte('\n')
		b.WriteByte('\n')
	} else {
		// The fixed footer can occupy several rows on a narrow/short terminal.
		// Keep the palette's header and scroll markers intact instead of
		// letting its prompt chrome push the markers into the footer.
		b.WriteByte('\n')
	}

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
		var line string
		switch {
		case m.pickingHarness:
			switch item.action {
			case actionToggleHarness:
				line = formatHarnessCheckboxLine(item.label, item.hint, m.harnessDraft[item.harnessID])
			case actionToggleHarnessPermission:
				enabled := m.harnessDraft[item.harnessID]
				checked := m.harnessPermissionDraft[item.harnessID][harness.PermissionProfile(item.modelSlug)]
				line = formatHarnessPermissionCheckboxLine(item.label, checked, enabled)
			}
		case m.pickingHarnessReset:
			line = " " + item.label
			if item.hint != "" {
				line += "  " + item.hint
			}
		case m.pickingModel && item.modelSlug != "":
			line = " " + item.label
			if item.hint != "" {
				line += "  " + item.hint
			}
		case m.pickingModel:
			line = " " + item.label
		default:
			line = fmt.Sprintf(" %s — %s", item.label, item.hint)
		}
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
	panelWidth := m.contentWidth() - 2
	if panelWidth < 24 {
		panelWidth = 24
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Width(panelWidth).
		Render(strings.TrimRight(list.String(), "\n"))
	if !compact || m.frameContentHeight() >= 8 {
		b.WriteString(mutedStyle.Render(title))
		b.WriteByte('\n')
	}
	b.WriteString(panel)
	return b.String()
}

func formatHarnessCheckboxLine(name, availHint string, checked bool) string {
	box := "[ ]"
	if checked {
		box = "[x]"
	}
	return fmt.Sprintf(" %s %s %s", box, strings.TrimSpace(name), strings.TrimSpace(availHint))
}

func formatHarnessPermissionCheckboxLine(name string, checked, enabled bool) string {
	box := "[ ]"
	if checked {
		box = "[x]"
	}
	line := "     " + box + " " + strings.TrimSpace(name)
	if !enabled {
		return mutedStyle.Render(line)
	}
	return line
}

func (m model) paletteListHeight() int {
	contentH := m.frameContentHeight()
	static := 5 // header, hint, prompt, blank, range label
	if contentH < 10 {
		static = 3 // header, hint, range label (prompt/blank omitted)
	}
	if contentH < 8 {
		static = 2 // header and hint only
	}
	// Reserve the panel border and both scroll markers. When a marker is not
	// needed this leaves a blank row, which is preferable to clipping it.
	h := contentH - static - 2 - 2
	if h < 1 {
		h = 1
	}
	return h
}

// ensurePaletteVisible keeps paletteIndex inside the scrolled window.
func (m model) ensurePaletteVisible() model {
	if m.pickingProps {
		// The C5 property picker keeps its own three-row cursor; paletteItems is
		// empty while it is open, so clamping against the list would reset the
		// property row selection to 0 on every render/refresh.
		return m
	}
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
	if m.cycleWelcomeDialog {
		return m.renderCycleWelcomeDialog()
	}
	var bottom strings.Builder
	bottom.WriteString(m.renderStatusBar())
	bottom.WriteByte('\n')
	bottom.WriteString(m.renderBorderRule())
	bottom.WriteByte('\n')
	bottom.WriteString(m.renderFooter())
	bottomStr := bottom.String()

	contentH := m.frameContentHeight()

	var content string
	if m.screen == screenConversation {
		content = m.renderConversation(contentH)
	} else {
		content = m.renderContent()
		if m.screenHasContentScroll() {
			content = m.clipScrolledContent(content, contentH)
		}
	}
	// Keep the footer anchored to the bottom of the terminal. Chat can have
	// more fixed pane chrome than fits in a very short terminal; retain its
	// bottom rows (composer/status) while trimming the excess instead of
	// allowing the content to push the footer out of the frame.
	if contentH > 0 {
		content = fitContentHeight(content, contentH, m.screen == screenConversation)
	} else {
		content = ""
	}

	mid := content
	if sidebar := m.renderNavSidebar(contentH); sidebar != "" {
		mid = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	}
	if mid != "" {
		mid += "\n"
	}
	return mid + bottomStr
}

// fitContentHeight makes the content area exactly height rows. A fixed frame
// is important here: if content is shorter, the footer must still stay at the
// terminal bottom; if it is taller, the footer must not be overwritten by it.
func fitContentHeight(content string, height int, keepBottom bool) string {
	if height <= 0 {
		return ""
	}
	content = strings.TrimRight(content, "\n")
	var lines []string
	if content != "" {
		lines = strings.Split(content, "\n")
	}
	if len(lines) > height {
		if keepBottom {
			lines = lines[len(lines)-height:]
		} else {
			lines = lines[:height]
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

const fixedFooterHints = "tab focus · alt+m mode · / commands · enter newline or command · alt+enter send · alt+r/i copy · ↑↓ scroll · alt+q quit"

func (m model) footerHints() string {
	if m.cycleWelcomeDialog {
		return "tab/←→ select · enter confirm · esc close"
	}
	if m.shellFocus == shellFocusNavbar {
		return "tab screen · ↑↓ navbar · enter open · " + m.navScreenRangeLabel() + " screens · alt+q quit"
	}
	if m.screen == screenConfig {
		switch {
		case m.config.leaveDialog && m.config.saving:
			return "saving configuration…"
		case m.config.leaveDialog:
			return "enter save · d discard · esc cancel"
		case m.config.editing:
			return "enter apply · esc cancel · ←→ move · home/end · backspace/delete"
		default:
			return "tab navbar · ↑↓ fields · space toggle · enter edit/select · alt+s save · alt+enter save and start · alt+r reload · esc leave"
		}
	}
	if m.screen == screenSettings {
		if m.settings.saving {
			return "saving settings…"
		}
		return "tab navbar · ↑↓ choose · enter apply · esc chat · alt+q quit"
	}
	return fixedFooterHints
}

// renderBorderRule draws a full-width horizontal rule in the same border color
// used by chat/sidebar rounded boxes.
func (m model) renderBorderRule() string {
	w := max(20, m.width)
	return lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", w))
}

// footerHintLines wraps the footer at the terminal width. The footer used to
// be rendered as a single unbounded line, so long Chat hints were clipped by
// the terminal and could visually collide with the content above them.
func (m model) footerHintLines() []string {
	width := m.width
	if width < 1 {
		width = 1
	}
	hints := strings.TrimSpace(m.footerHints())
	if hints == "" {
		// Keep one reserved footer row for picker states that render their own
		// keyboard guidance in the content area.
		return []string{""}
	}
	// Keep each hint atomic while wrapping. In particular, splitting the
	// `↑↓ scroll` or `alt+enter send` pairs makes the fixed footer look
	// incomplete even though all text is technically present.
	parts := strings.Split(hints, " · ")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(lines) == 0 {
			lines = append(lines, part)
			continue
		}
		candidate := lines[len(lines)-1] + " · " + part
		if lipgloss.Width(candidate) <= width {
			lines[len(lines)-1] = candidate
			continue
		}
		lines = append(lines, part)
	}
	return lines
}

func (m model) footerLineCount() int {
	return len(m.footerHintLines())
}

func (m model) renderFooter() string {
	lines := m.footerHintLines()
	for i, line := range lines {
		// Render each row independently. Lipgloss pads multi-line strings to
		// their widest row, which makes shorter wrapped hints look like they
		// overlap the terminal edge.
		lines[i] = footerStyle.Render(line)
	}
	return strings.Join(lines, "\n")
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
	case screenConfig, screenSettings, screenStatus, screenArtifacts, screenCosts, screenEvents:
		return true
	default:
		return false
	}
}

func (m model) frameContentHeight() int {
	// Match renderFrame chrome: status bar, one border rule under status, and
	// the responsive footer. No rule above the status area.
	h := m.height - (m.statusBarLineCount() + 1 + m.footerLineCount())
	if h < 0 {
		h = 0
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
