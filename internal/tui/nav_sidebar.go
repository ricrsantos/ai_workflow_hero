package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type navScreenItem struct {
	screen screen
	label  string
}

type shellFocus int

const (
	shellFocusContent shellFocus = iota
	shellFocusNavbar
)

// navScreens is the fixed left-rail menu (replaces the old horizontal tab bar).
var navScreens = []navScreenItem{
	{screenConversation, "Chat"},
	{screenStatus, "Status"},
	{screenArtifacts, "Artifacts"},
	{screenCosts, "Costs"},
	{screenEvents, "Events"},
	{screenConfig, "Config"},
}

// Navigation bindings are shared by the sidebar, palette, and screen handlers.
// The visible screen list below decides whether a numbered shortcut is valid.
var navShortcutKeys = [...]key.Binding{
	key.NewBinding(key.WithKeys("alt+1"), key.WithHelp("alt+1", "Chat")),
	key.NewBinding(key.WithKeys("alt+2"), key.WithHelp("alt+2", "Status")),
	key.NewBinding(key.WithKeys("alt+3"), key.WithHelp("alt+3", "Artifacts")),
	key.NewBinding(key.WithKeys("alt+4"), key.WithHelp("alt+4", "Costs")),
	key.NewBinding(key.WithKeys("alt+5"), key.WithHelp("alt+5", "Events")),
	key.NewBinding(key.WithKeys("alt+6"), key.WithHelp("alt+6", "Config")),
}

var (
	shellFocusKey = key.NewBinding(
		key.WithKeys("tab", "shift+tab"),
		key.WithHelp("tab", "switch focus"),
	)
	navUpKey = key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous item"),
	)
	navDownKey = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next item"),
	)
	navSelectKey = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open screen"),
	)
)

var navEventsAlias = key.NewBinding(
	key.WithKeys("alt+n"),
	key.WithHelp("alt+n", "Events"),
)

// navSidebarAgentRows is the minimum body rows reserved for live agent labels.
const navSidebarAgentRows = 2

// The timer block is kept at the bottom of the sidebar, below navigation and
// its shortcut hint. The separator belongs to the block so it reads as a
// dedicated subdivision like the live-agent rows above.
const navSidebarTimerRows = 4

// navSidebarTimerLabelWidth reserves one shared value column for the Session
// and AI counters in the sidebar.
const navSidebarTimerLabelWidth = 7

func (m model) visibleNavScreens() []navScreenItem {
	items := make([]navScreenItem, 0, len(navScreens))
	for _, item := range navScreens {
		if m.freeChatMode && item.screen != screenConversation {
			continue
		}
		if item.screen == screenConfig && !m.hasActiveCycle() {
			continue
		}
		items = append(items, item)
	}
	return items
}

func navShortcutIndex(msg tea.KeyMsg) (int, bool) {
	for i, binding := range navShortcutKeys {
		if key.Matches(msg, binding) {
			return i + 1, true
		}
	}
	return 0, false
}

func isLegacyEventsShortcut(msg tea.KeyMsg) bool {
	return key.Matches(msg, navEventsAlias)
}

func isNavKey(msg tea.KeyMsg) bool {
	_, numbered := navShortcutIndex(msg)
	return numbered || isLegacyEventsShortcut(msg)
}

func (m model) navScreenAt(index int) (screen, bool) {
	items := m.visibleNavScreens()
	if index < 1 || index > len(items) {
		return 0, false
	}
	return items[index-1].screen, true
}

func (m model) activeNavScreen() screen {
	if m.screen == screenPalette || m.screen == screenOutput {
		return m.prevScreen
	}
	return m.screen
}

func (m model) activeNavIndex() int {
	active := m.activeNavScreen()
	for i, item := range m.visibleNavScreens() {
		if item.screen == active {
			return i
		}
	}
	return 0
}

func (m model) toggleShellFocus() (tea.Model, tea.Cmd) {
	if !m.sidebarVisible() {
		return m, nil
	}
	if m.shellFocus == shellFocusNavbar {
		m.shellFocus = shellFocusContent
		m.chatInputFocused = m.screen == screenConversation
		return m, nil
	}
	if m.screen == screenConfig && m.config.editing {
		m = m.commitConfigEdit()
	}
	m.shellFocus = shellFocusNavbar
	m.navCursor = m.activeNavIndex()
	m.chatInputFocused = false
	return m, nil
}

func (m model) handleNavbarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.visibleNavScreens()
	if len(items) == 0 {
		return m, nil
	}
	if m.navCursor < 0 || m.navCursor >= len(items) {
		m.navCursor = m.activeNavIndex()
	}
	switch {
	case key.Matches(msg, navUpKey):
		m.navCursor = (m.navCursor - 1 + len(items)) % len(items)
		return m, nil
	case key.Matches(msg, navDownKey):
		m.navCursor = (m.navCursor + 1) % len(items)
		return m, nil
	case key.Matches(msg, navSelectKey):
		return m.activateNavbarScreen(items[m.navCursor].screen)
	case msg.String() == "alt+q":
		if m.screen == screenConfig && m.config.dirty {
			m.shellFocus = shellFocusContent
			m.config.leaveDialog = true
			m.config.leaveScreen = screenConfig
			m.config.leaveQuit = true
			return m, nil
		}
		return m.handleKey(msg)
	}
	if isLegacyEventsShortcut(msg) && !m.freeChatMode {
		for i, item := range items {
			if item.screen == screenEvents {
				m.navCursor = i
				break
			}
		}
		return m.activateNavbarScreen(screenEvents)
	}
	if index, ok := navShortcutIndex(msg); ok {
		target, exists := m.navScreenAt(index)
		if !exists {
			return m, nil
		}
		for i, item := range items {
			if item.screen == target {
				m.navCursor = i
				break
			}
		}
		return m.activateNavbarScreen(target)
	}
	return m, nil
}

func (m model) activateNavbarScreen(target screen) (model, tea.Cmd) {
	if m.screen == screenConfig && target != screenConfig && m.config.dirty {
		m.shellFocus = shellFocusContent
		m.config.leaveDialog = true
		m.config.leaveScreen = target
		m.config.leaveQuit = false
		return m, nil
	}
	if m.screen == screenPalette {
		m = m.closePalette()
	}
	if m.screen == screenOutput {
		m.outputLines = nil
		m.outputRaw = ""
		m.outputOffset = 0
		m.outputTitle = ""
		m.outputErr = false
	}
	next, cmd := m.navigateToScreen(target)
	next.shellFocus = shellFocusNavbar
	next.navCursor = next.activeNavIndex()
	next.chatInputFocused = false
	return next, cmd
}

func (m model) navigateToScreen(target screen) (model, tea.Cmd) {
	if target == screenConversation {
		return m.enterConversation()
	}
	if target == screenConfig {
		return m.openConfig()
	}
	return m.goListScreen(target)
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

func navSidebarTimerLine(label, elapsed string, innerW int) string {
	line := fmt.Sprintf(" %-*s %s", navSidebarTimerLabelWidth, label, elapsed)
	return navSidebarTimerStyle.Render(truncateNavText(line, innerW))
}

func (m model) timerSidebarLines(innerW int) []string {
	lines := make([]string, 0, navSidebarTimerRows)
	lines = append(lines,
		navSidebarSeparator(innerW),
		navSidebarTimerLine("Session", formatElapsed(m.sessionTimer.displayed), innerW),
		navSidebarTimerLine("AI wk", formatElapsed(m.aiTimer.displayed), innerW),
		navSidebarTimerLine("AI rp", formatElapsed(m.aiResponseTimer.displayed), innerW),
	)
	return lines
}

func (m model) navSidebarNavigationLines(innerW int) []string {
	lines := make([]string, 0, 1+1+navSidebarAgentRows+1+len(m.visibleNavScreens()))
	lines = append(lines, navSidebarTitleStyle.Render(truncateNavText(" AI Hero", innerW)))
	lines = append(lines, navSidebarSeparator(innerW))
	for _, agentLine := range m.agentsSidebarLines(innerW) {
		lines = append(lines, navSidebarItemStyle.Render(truncateNavText(agentLine, innerW)))
	}
	lines = append(lines, navSidebarSeparator(innerW))

	active := m.activeNavScreen()
	for i, item := range m.visibleNavScreens() {
		marker := "  "
		rowStyle := navSidebarItemStyle
		if item.screen == active {
			marker = "> "
			rowStyle = navSidebarActiveStyle
		}
		if m.shellFocus == shellFocusNavbar && i == m.navCursor {
			rowStyle = navSidebarFocusedStyle
		}
		lines = append(lines, rowStyle.Render(truncateNavText(marker+item.label, innerW)))
	}
	return lines
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

	rangeLabel := "alt+1-5"
	if m.hasActiveCycle() {
		rangeLabel = "alt+1-6"
	}
	rangeHint := navSidebarFooterStyle.Render(truncateNavText(" "+rangeLabel, innerW))
	navigationLines := m.navSidebarNavigationLines(innerW)
	timerLines := m.timerSidebarLines(innerW)
	if len(timerLines) > innerH {
		// Keep the most recent values visible in a very short terminal, even when the
		// separator cannot fit.
		timerLines = timerLines[len(timerLines)-innerH:]
	}
	upperH := innerH - len(timerLines)
	if upperH < 0 {
		upperH = 0
	}
	// Keep the shortcut hint anchored to the bottom of the navigation area.
	// This leaves any spare rows between the menu and the hint, and keeps the
	// hint directly above the timer separator.
	lines := make([]string, 0, innerH)
	if upperH > 0 {
		lines = navigationLines
		menuH := upperH - 1
		if menuH < 0 {
			menuH = 0
		}
		if len(lines) > menuH {
			lines = lines[:menuH]
		}
		for len(lines) < menuH {
			lines = append(lines, strings.Repeat(" ", innerW))
		}
		lines = append(lines, rangeHint)
	}
	lines = append(lines, timerLines...)

	var content strings.Builder
	for i, line := range lines {
		if i > 0 {
			content.WriteByte('\n')
		}
		content.WriteString(line)
	}
	return box.Width(innerW).Height(innerH).Render(content.String())
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
