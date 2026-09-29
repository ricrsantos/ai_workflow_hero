package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

var helpKey = key.NewBinding(key.WithKeys("alt+f1"), key.WithHelp("Alt+F1", "Help"))
var helpCloseKey = key.NewBinding(key.WithKeys("alt+f1", "esc"), key.WithHelp("Alt+F1 / Esc", "close"))

func (m model) openKeyboardHelp() model {
	m.helpOpen = true
	m.helpOffset = 0
	return m
}

type helpEntry struct {
	keys   string
	action string
}

type helpGroup struct {
	title   string
	entries []helpEntry
}

func (m model) helpGroups() []helpGroup {
	nav := make([]helpEntry, 0, len(m.visibleNavScreens())+4)
	for i, item := range m.visibleNavScreens() {
		nav = append(nav, helpEntry{fmt.Sprintf("Alt+%d", i+1), item.label})
	}
	if !m.freeChatMode {
		nav = append(nav, helpEntry{"Alt+N", "Events"})
	}
	nav = append(nav,
		helpEntry{"Tab / Shift+Tab", "Switch content and sidebar focus"},
		helpEntry{"Esc", "Focus sidebar / return to Chat"},
		helpEntry{"↑ / ↓, Enter", "Select and open a sidebar screen"},
	)
	return []helpGroup{
		{"GENERAL", []helpEntry{
			{"/help", "Open this help from Chat or commands"},
			{"Alt+F1 / Esc", "Close this help"},
			{"/", "Open commands (Chat: autocomplete)"},
			{"Alt+Q", "Quit Hero"},
			{"Ctrl+C", "Interrupt the active agent"},
			{"F5", "Refresh project data"},
			{"Alt+↑ / ↓", "Scroll long status messages"},
			{"Alt+PgUp/PgDn", "Scroll status by page"},
			{"Alt+Home/End", "Start / end of status message"},
		}},
		{"NAVIGATION", nav},
		{"CHAT", []helpEntry{
			{"Enter", "New line; run a selected slash command"},
			{"Alt+Enter", "Send message"},
			{"Alt+M", "Switch Build / Plan mode"},
			{"↑ / ↓", "Move caret or scroll conversation"},
			{"PgUp / PgDn", "Scroll conversation by page"},
			{"Alt+R / I", "Copy reply / composer"},
			{"Alt+A", "Attach an image or file"},
			{"Alt+V", "Attach image from clipboard"},
			{"Alt+C / G", "Focus attachment chips / asset cards"},
			{"Esc", "Return from chips or cards to composer"},
			{"Enter / O", "Open focused image"},
			{"C / A / S / X", "Copy / attach / save / remove focused image"},
		}},
		{"HISTORY", []helpEntry{
			{"↑ / ↓", "Choose a session"},
			{"← / →", "Active / archived sessions"},
			{"/", "Search sessions"},
			{"Enter", "Open session or details"},
			{"R / A / D", "Rename / archive or restore / delete"},
			{"Esc", "Close detail, search, or dialog"},
		}},
		{"CONFIG & SETTINGS", []helpEntry{
			{"↑ / ↓", "Select a field or setting"},
			{"Enter", "Edit, select, or apply"},
			{"Space", "Toggle a Config option"},
			{"Alt+S", "Save Config"},
			{"Alt+Enter", "Save Config and start cycle"},
			{"Alt+R", "Reload Config"},
			{"R", "Retry failed stage (when available)"},
			{"← / →, Home/End", "Move within a Config field"},
			{"Backspace/Delete", "Delete text in a Config field"},
			{"D", "Discard Config changes in leave dialog"},
			{"Esc", "Cancel edit or leave screen"},
		}},
		{"COMMANDS & PICKERS", []helpEntry{
			{"/", "Open command palette; type to filter"},
			{"↑ / ↓", "Select a command or picker item"},
			{"Tab", "Complete or run a slash suggestion"},
			{"Space", "Toggle a harness in its picker"},
			{"Enter", "Run command or choose item"},
			{"Esc", "Close picker or command palette"},
			{"J / K, ← / H", "Navigate / go back in file picker"},
		}},
		{"DIALOGS & LISTS", []helpEntry{
			{"↑ / ↓", "Move selection"},
			{"Enter", "Confirm selection"},
			{"Esc", "Cancel or go back"},
			{"PgUp / PgDn", "Scroll long lists"},
		}},
	}
}

func (m model) helpRows() []string {
	w := max(1, m.width)
	rows := []string{
		truncateDisplayWidth(titleStyle.Render("  KEYBOARD SHORTCUTS")+footerStyle.Render("  /  HERO"), w),
		truncateDisplayWidth(footerStyle.Render("  /help opens · Esc closes · ↑↓ scroll · PgUp/PgDn jump"), w),
		"",
	}
	for _, group := range m.helpGroups() {
		rows = append(rows, truncateDisplayWidth("  "+helpSectionStyle.Render(group.title), w))
		for _, entry := range group.entries {
			line := "  " + helpKeyStyle.Render(fmt.Sprintf("%-19s", entry.keys)) + footerStyle.Render(entry.action)
			rows = append(rows, truncateDisplayWidth(line, w))
		}
		rows = append(rows, "")
	}
	return rows
}

func (m model) helpMaxOffset() int {
	return max(0, len(m.helpRows())-m.frameContentHeight())
}

func (m model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, helpCloseKey) {
		m.helpOpen = false
		return m, nil
	}
	switch msg.String() {
	case "up":
		m.helpOffset--
	case "down":
		m.helpOffset++
	case "pgup":
		m.helpOffset -= max(1, m.frameContentHeight()-2)
	case "pgdown":
		m.helpOffset += max(1, m.frameContentHeight()-2)
	case "home":
		m.helpOffset = 0
	case "end":
		m.helpOffset = m.helpMaxOffset()
	}
	m.helpOffset = min(max(0, m.helpOffset), m.helpMaxOffset())
	return m, nil
}

func (m model) renderHelpFrame() string {
	rows := m.helpRows()
	h := m.frameContentHeight()
	start := min(m.helpOffset, max(0, len(rows)-h))
	end := min(len(rows), start+h)
	content := fitContentHeight(strings.Join(rows[start:end], "\n"), h, false)
	if h > 0 {
		content += "\n"
	}
	return content + m.renderStatusBar() + "\n" + m.renderBorderRule() + "\n" + m.renderFooter()
}
