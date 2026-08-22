package tui

import (
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

// renderNavSidebar draws the left "AI Hero" frame with screen entries.
// Height is the middle band (above the full-width bottom chrome).
func (m model) renderNavSidebar(height int) string {
	if !m.sidebarVisible() || height <= 0 {
		return ""
	}
	box := navSidebarBoxStyle
	innerW := navSidebarWidth - box.GetHorizontalFrameSize()
	if innerW < 1 {
		innerW = 1
	}
	innerH := height - box.GetVerticalFrameSize()
	if innerH < 1 {
		innerH = 1
	}

	var b strings.Builder
	b.WriteString(navSidebarTitleStyle.Render(truncateNavText(" AI Hero", innerW)))
	b.WriteByte('\n')

	// title + footer consume two rows of the inner area.
	bodyLines := innerH - 2
	if bodyLines < 0 {
		bodyLines = 0
	}
	for i, item := range navScreens {
		if i >= bodyLines {
			break
		}
		marker := "  "
		rowStyle := navSidebarItemStyle
		if item.screen == m.screen {
			marker = "> "
			rowStyle = navSidebarActiveStyle
		}
		b.WriteString(rowStyle.Render(truncateNavText(marker+item.label, innerW)))
		b.WriteByte('\n')
	}
	// Pad remaining body rows so the footer sits at the bottom of the box.
	writtenBody := len(navScreens)
	if writtenBody > bodyLines {
		writtenBody = bodyLines
	}
	for i := writtenBody; i < bodyLines; i++ {
		b.WriteByte('\n')
	}

	footer := navSidebarFooterStyle.Render(truncateNavText(" alt+1-5", innerW))
	body := strings.TrimSuffix(b.String(), "\n") + "\n" + footer
	// Height is the content box only; borders are added outside (lipgloss v1).
	return box.Width(navSidebarWidth).Height(innerH).Render(body)
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
