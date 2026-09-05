package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

var settingsKeys = struct {
	Next     key.Binding
	Previous key.Binding
	Save     key.Binding
	Leave    key.Binding
}{
	Next:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next")),
	Previous: key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "previous")),
	Save:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
	Leave:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to chat")),
}

type settingsScreen struct {
	verbosity install.ChatVerbosity
	cursor    int
	saving    bool
	err       string

	// Telegram abbreviation inline edit (telegram-tui R1).
	editingAbbrev bool
	abbrevDraft   string
}

type settingsSavedMsg struct{ err error }

type verbosityOption struct {
	value       install.ChatVerbosity
	label       string
	description string
}

var verbosityOptions = []verbosityOption{
	{install.ChatVerbosityCompact, "Compact", "Responses, errors, and required approvals."},
	{install.ChatVerbosityStandard, "Standard", "Compact plus tools and Task lifecycle."},
	{install.ChatVerbosityDetailed, "Detailed", "Standard plus reasoning, activities, and subagent output."},
	{install.ChatVerbosityDebug, "Debug", "Everything Hero currently shows, including diagnostics."},
}

func (m model) openSettings() (model, tea.Cmd) {
	m.chatInputFocused = false
	m.screen = screenSettings
	m.settings.err = ""
	m.settings.cursor = verbosityOptionIndex(m.settings.verbosity)
	m.settings.editingAbbrev = false
	m.settings.abbrevDraft = ""
	return m, nil
}

func (m model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// The Telegram pairing modal is focused and owns all keys until closed.
	if m.telegram != nil && m.telegram.pairing {
		return m.handleTelegramPairingKey(msg)
	}
	if m.settings.saving {
		return m, nil
	}
	if m.settings.editingAbbrev {
		return m.handleTelegramAbbrevKey(msg)
	}

	rows := m.settingsRows()
	n := len(rows)
	if n == 0 {
		return m, nil
	}
	if m.settings.cursor < 0 || m.settings.cursor >= n {
		m.settings.cursor = 0
	}
	switch {
	case key.Matches(msg, settingsKeys.Next):
		m.settings.cursor = (m.settings.cursor + 1) % n
		return m, nil
	case key.Matches(msg, settingsKeys.Previous):
		m.settings.cursor = (m.settings.cursor - 1 + n) % n
		return m, nil
	case key.Matches(msg, settingsKeys.Save):
		row := rows[m.settings.cursor]
		if row.kind == rowVerbosity {
			m.settings.verbosity = row.verbosity
			m.settings.err = ""
			m.settings.saving = true
			return m, m.saveSettingsCmd()
		}
		return m.telegramSettingsEnter(row)
	case key.Matches(msg, settingsKeys.Leave):
		return m.enterConversation()
	default:
		return m.handleKey(msg)
	}
}

func (m model) saveSettingsCmd() tea.Cmd {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	value := m.settings.verbosity
	return func() tea.Msg {
		if projectDir == "" {
			return settingsSavedMsg{err: fmt.Errorf("project unavailable")}
		}
		return settingsSavedMsg{err: install.SetChatVerbosity(projectDir, value)}
	}
}

func (m model) handleSettingsMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	saved, ok := msg.(settingsSavedMsg)
	if !ok {
		return m, nil
	}
	m.settings.saving = false
	if saved.err != nil {
		m.settings.err = saved.err.Error()
		return m, nil
	}
	m = m.setStatusResult(true, "settings", "Chat verbosity: "+verbosityOptionLabel(m.settings.verbosity))
	return m, nil
}

func (m model) renderSettings() string {
	if m.frameContentHeight() < 12 {
		return warnStyle.Render("Settings needs a taller terminal window.")
	}
	width := m.settingsPanelWidth()
	applied := install.NormalizeChatVerbosity(m.settings.verbosity)
	rows := m.settingsRows()

	var b strings.Builder
	b.WriteString(settingsTitleStyle.Render("Settings"))
	b.WriteByte('\n')
	b.WriteString(m.renderSettingsRule())
	b.WriteString("\n\n")
	b.WriteString(renderSettingsSection("CHAT VERBOSITY", "select one", width))
	b.WriteString("\n\n")
	for i, row := range rows {
		if row.kind != rowVerbosity {
			continue
		}
		b.WriteString(m.renderVerbosityRadio(row, i == m.settings.cursor, row.verbosity == applied, width))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(m.renderSettingsRule())
	b.WriteString("\n\n")
	b.WriteString(renderSettingsSection("TELEGRAM PLUGIN", "optional plugin", width))
	b.WriteString("\n\n")
	b.WriteString(m.renderTelegramPluginSection(rows, width))
	b.WriteByte('\n')
	b.WriteByte('\n')
	if m.settings.saving {
		b.WriteString(infoStyle.Render("Saving preference…"))
	} else if m.settings.err != "" {
		b.WriteString(errorStyle.Render("Could not save: " + m.settings.err))
	} else {
		b.WriteString(m.renderSettingsFooterHints())
	}
	return b.String()
}

func (m model) settingsPanelWidth() int {
	w := m.contentWidth() - 2
	if w < 24 {
		w = 24
	}
	return w
}

func (m model) renderSettingsRule() string {
	return settingsRuleStyle.Render(strings.Repeat("─", m.settingsPanelWidth()))
}

func renderSettingsSection(title, hint string, width int) string {
	left := settingsSectionStyle.Render(title)
	right := settingsSectionHintStyle.Render(hint)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func settingsRadioLabelWidth() int {
	w := 0
	for _, o := range verbosityOptions {
		if n := lipgloss.Width(o.label); n > w {
			w = n
		}
	}
	return w
}

func (m model) renderVerbosityRadio(row settingsRow, focused, applied bool, width int) string {
	box := settingsRadioIdle.Width(width)
	switch {
	case applied:
		box = settingsRadioApplied.Width(width)
	case focused:
		box = settingsRadioFocus.Width(width)
	}
	inner := width - box.GetHorizontalFrameSize()
	if inner < 8 {
		inner = 8
	}

	caret := "  "
	if focused && applied {
		caret = settingsRadioCaretStyle.Render("> ")
	}
	glyph := settingsRadioEmptyStyle.Render("○ ")
	if applied {
		glyph = settingsRadioFilledStyle.Render("• ")
	}
	label := settingsRadioNameStyle.Render(padRight(row.label, settingsRadioLabelWidth()))
	used := lipgloss.Width(caret) + lipgloss.Width(glyph) + lipgloss.Width(padRight(row.label, settingsRadioLabelWidth())) + 1
	descW := inner - used
	if descW < 4 {
		descW = 4
	}
	desc := mutedStyle.Render(truncateDisplayWidth(row.desc, descW))
	return box.Render(caret + glyph + label + " " + desc)
}

func (m model) renderSettingsFooterHints() string {
	enter := "apply"
	if m.settings.editingAbbrev {
		return joinSettingsHints(
			settingsHintChip("enter", "save abbreviation"),
			settingsHintChip("esc", "cancel"),
		)
	}
	if row, ok := m.focusedSettingsRow(); ok {
		switch row.kind {
		case rowTelegramCopyCommand:
			enter = "copy"
		case rowTelegramAbbrev:
			enter = "edit"
		case rowTelegramAction:
			enter = row.label
		}
	}
	return joinSettingsHints(
		settingsHintChip("↑/↓", "navigate"),
		settingsHintChip("enter", enter),
		settingsHintChip("esc", "back to chat"),
	)
}

func settingsHintChip(key, help string) string {
	return settingsKeyChipStyle.Render(key) + " " + mutedStyle.Render(help)
}

func joinSettingsHints(parts ...string) string {
	return strings.Join(parts, "  ")
}

func (m model) focusedSettingsRow() (settingsRow, bool) {
	rows := m.settingsRows()
	if m.settings.cursor < 0 || m.settings.cursor >= len(rows) {
		return settingsRow{}, false
	}
	return rows[m.settings.cursor], true
}

func verbosityOptionIndex(value install.ChatVerbosity) int {
	value = install.NormalizeChatVerbosity(value)
	for i, option := range verbosityOptions {
		if option.value == value {
			return i
		}
	}
	return len(verbosityOptions) - 1
}

func verbosityOptionLabel(value install.ChatVerbosity) string {
	for _, option := range verbosityOptions {
		if option.value == install.NormalizeChatVerbosity(value) {
			return option.label
		}
	}
	return "Debug"
}
