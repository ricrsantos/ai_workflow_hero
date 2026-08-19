package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const chatSlashOverlayMax = 8

func chatSlashToken(text string) string {
	s := strings.TrimSpace(text)
	if !strings.HasPrefix(s, "/") {
		return ""
	}
	if i := strings.IndexAny(s, " \t\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func (m model) liveOrchestratorSession() bool {
	return m.orchestrationLive
}

// chatFollowUpControlSlash reports whether a Chat submit should go to the live
// orchestrator as a follow-up instead of TUI Execute (which gates on SQLite
// PendingApproval and starts a fresh session).
func (m model) chatFollowUpControlSlash(text string) bool {
	if !m.liveOrchestratorSession() {
		return false
	}
	switch strings.ToLower(chatSlashToken(text)) {
	case "/hero-approve", "/hero-reject", "/hero-cancel", "/hero-finish",
		"/hero-continue", "/hero-back":
		return true
	default:
		return false
	}
}

func (m model) chatSlashOverlayActive() bool {
	if m.streaming || m.slashOverlayDismissed {
		return false
	}
	if strings.Contains(m.input, "\n") {
		return false
	}
	if strings.ContainsAny(strings.TrimSpace(m.input), " \t") {
		return false
	}
	token := chatSlashToken(m.input)
	if token == "" {
		return false
	}
	return len(m.filteredChatSlashItems()) > 0
}

func (m model) filteredChatSlashItems() []paletteItem {
	token := strings.ToLower(chatSlashToken(m.input))
	if token == "" {
		return nil
	}
	rest := strings.TrimPrefix(token, "/")
	var out []paletteItem
	for _, item := range m.paletteItems {
		if !chatSlashOverlayItem(item) {
			continue
		}
		label := strings.ToLower(item.label)
		if token == "/" || strings.HasPrefix(label, token) || (rest != "" && strings.Contains(label, rest)) {
			out = append(out, item)
		}
	}
	return out
}

func chatSlashOverlayItem(item paletteItem) bool {
	switch item.action {
	case actionSelectModel, actionPickModelHarness, actionToggleHarness, actionApplyHarness,
		actionSelectHarnessReset:
		return false
	default:
		return true
	}
}

// chatComposerControlSlash is true for agent-reply slashes that stay in the
// composer (insert, then Enter to send). All other overlay items run immediately
// like the full-screen palette.
func chatComposerControlSlash(item paletteItem) bool {
	switch item.action {
	case actionApprove, actionReject, actionCancel, actionContinue, actionFinish, actionBack:
		return true
	default:
		return false
	}
}

func (m model) clampedSlashOverlayIndex() int {
	n := len(m.filteredChatSlashItems())
	if n == 0 {
		return 0
	}
	if m.slashOverlayIndex < 0 {
		return 0
	}
	if m.slashOverlayIndex >= n {
		return n - 1
	}
	return m.slashOverlayIndex
}

func (m model) afterChatInputEdit(prevToken string) model {
	token := chatSlashToken(m.input)
	if token != prevToken {
		m.slashOverlayIndex = 0
		m.slashOverlayDismissed = false
	}
	return m
}

func (m model) insertChatSlashSelection() model {
	items := m.filteredChatSlashItems()
	if len(items) == 0 {
		return m
	}
	idx := m.clampedSlashOverlayIndex()
	label := items[idx].label
	m.input = label
	m.inputCursor = runeLen(label)
	m.slashOverlayDismissed = true
	m.slashOverlayIndex = 0
	return m.ensureInputCaretVisible()
}

func (m model) applyChatSlashSelection() (model, tea.Cmd) {
	items := m.filteredChatSlashItems()
	if len(items) == 0 {
		return m, nil
	}
	item := items[m.clampedSlashOverlayIndex()]
	if chatComposerControlSlash(item) {
		return m.insertChatSlashSelection(), nil
	}
	m = m.clearChatInput()
	// Overlay is not the full-screen palette; keep Chat as the restore target
	// so closePalette() does not jump to a stale prevScreen (e.g. Status).
	m.prevScreen = screenConversation
	return m.runPaletteAction(item)
}

func (m model) clearChatInput() model {
	m.input = ""
	m.inputCursor = 0
	m.inputScrollOffset = 0
	m.slashOverlayDismissed = false
	m.slashOverlayIndex = 0
	return m
}

func (m model) renderChatSlashOverlay() string {
	if !m.chatSlashOverlayActive() {
		return ""
	}
	items := m.filteredChatSlashItems()
	idx := m.clampedSlashOverlayIndex()
	end := len(items)
	if end > chatSlashOverlayMax {
		end = chatSlashOverlayMax
	}
	start := 0
	if idx >= end {
		start = idx - chatSlashOverlayMax + 1
		end = start + chatSlashOverlayMax
		if end > len(items) {
			end = len(items)
			start = end - chatSlashOverlayMax
			if start < 0 {
				start = 0
			}
		}
	}

	var list strings.Builder
	for i := start; i < end; i++ {
		item := items[i]
		line := " " + item.label
		if item.hint != "" {
			line += " — " + item.hint
		}
		if i == idx {
			list.WriteString(selectedStyle.Render("▸ " + line))
		} else {
			list.WriteString(mutedStyle.Render("  " + line))
		}
		if i < end-1 {
			list.WriteByte('\n')
		}
	}
	width := m.chatBoxWidth()
	if width < 24 {
		width = 24
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Width(width).
		Render(list.String())
	return panel + "\n"
}
