package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// navScreens is the fixed left-rail menu (replaces the old horizontal tab bar).
var navScreens = []struct {
	screen screen
	label  string
}{
	{screenConversation, "Chat"},
	{screenStatus, "Status"},
	{screenArtifacts, "Artifacts"},
	{screenCosts, "Costs"},
	{screenEvents, "Events"},
}

// navSidebarAgentRows is the minimum body rows reserved for live agent labels.
const navSidebarAgentRows = 2

// sidebarVisible reports whether the left nav frame fits beside the main pane.
func (m model) sidebarVisible() bool {
	if m.width < navSidebarMinWidth {
		return false
	}
	return m.width-navSidebarWidth >= navSidebarMinMain
}

// contentWidth is the main pane width (terminal minus sidebar when shown).
func (m model) contentWidth() int {
	w := m.width
	if m.sidebarVisible() {
		w -= navSidebarWidth
	}
	if w < 1 {
		w = 1
	}
	return w
}

// agentsSidebarLines returns the agents summary (count + label row). Always
// navSidebarAgentRows lines so the sidebar section keeps a stable height.
func (m model) agentsSidebarLines(innerW int) []string {
	n := len(m.liveAgents)
	labels := make([]string, 0, n)
	for _, a := range m.liveAgents {
		labels = append(labels, a.Label)
	}
	line1 := fmt.Sprintf("agents: %d", n)
	line2 := wrapAgentLabels(labels, innerW)
	if line2 != "" {
		if idx := strings.Index(line2, "\n"); idx >= 0 {
			line2 = line2[:idx]
		}
		line2 = truncateNavText(line2, innerW)
	}
	out := make([]string, navSidebarAgentRows)
	out[0] = line1
	if line2 != "" {
		out[1] = line2
	}
	return out
}

func navSidebarSeparator(innerW int) string {
	if innerW < 1 {
		innerW = 1
	}
	return navSidebarSepStyle.Render(strings.Repeat("─", innerW))
}

// renderNavSidebar draws the left "AI Hero" frame with agents + screen entries.
// Height is the middle band (above the full-width bottom chrome).
func (m model) renderNavSidebar(height int) string {
	if !m.sidebarVisible() || height <= 0 {
		return ""
	}
	box := navSidebarBoxStyle
	// Width/Height on bordered styles are inner content sizes; borders add outside.
	innerW := navSidebarWidth - box.GetHorizontalFrameSize()
	if innerW < 1 {
		innerW = 1
	}
	innerH := height - box.GetVerticalFrameSize()
	if innerH < 1 {
		innerH = 1
	}

	var lines []string
	lines = append(lines, navSidebarTitleStyle.Render(truncateNavText(" AI Hero", innerW)))
	lines = append(lines, navSidebarSeparator(innerW))
	for _, agentLine := range m.agentsSidebarLines(innerW) {
		lines = append(lines, navSidebarItemStyle.Render(truncateNavText(agentLine, innerW)))
	}
	lines = append(lines, navSidebarSeparator(innerW))

	for _, item := range navScreens {
		marker := "  "
		rowStyle := navSidebarItemStyle
		if item.screen == m.screen {
			marker = "> "
			rowStyle = navSidebarActiveStyle
		}
		lines = append(lines, rowStyle.Render(truncateNavText(marker+item.label, innerW)))
	}

	footer := navSidebarFooterStyle.Render(truncateNavText(" alt+1-5", innerW))
	for len(lines) < innerH-1 {
		lines = append(lines, strings.Repeat(" ", innerW))
	}
	if len(lines) > innerH-1 {
		lines = lines[:innerH-1]
	}
	lines = append(lines, footer)

	return box.Width(innerW).Height(innerH).Render(strings.Join(lines, "\n"))
}

// truncateNavText shortens s to at most width columns (ANSI-aware via lipgloss).
func truncateNavText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		// Pad so every row fills the inner width (avoids uneven box fill).
		pad := width - lipgloss.Width(s)
		if pad > 0 {
			return s + strings.Repeat(" ", pad)
		}
		return s
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		cand := string(runes) + "…"
		if lipgloss.Width(cand) <= width {
			pad := width - lipgloss.Width(cand)
			if pad > 0 {
				cand += strings.Repeat(" ", pad)
			}
			return cand
		}
	}
	return "…"
}
