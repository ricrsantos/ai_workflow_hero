package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

var settingsKeys = struct {
	Next     key.Binding
	Previous key.Binding
	Save     key.Binding
	Leave    key.Binding
}{
	Next:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next profile")),
	Previous: key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "previous profile")),
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
	switch {
	case key.Matches(msg, settingsKeys.Next):
		m.settings.cursor = (m.settings.cursor + 1) % n
		return m, nil
	case key.Matches(msg, settingsKeys.Previous):
		m.settings.cursor = (m.settings.cursor - 1 + n) % n
		return m, nil
	case key.Matches(msg, settingsKeys.Save):
		if m.settings.cursor < 0 || m.settings.cursor >= n {
			return m, nil
		}
		row := rows[m.settings.cursor]
		if row.kind == rowVerbosity {
			m.settings.verbosity = verbosityOptions[m.settings.cursor].value
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
	rows := m.settingsRows()
	var b strings.Builder
	b.WriteString(headerStyle.Render("Settings"))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("Choose how much live AI activity appears in Chat."))
	b.WriteString("\n\n")
	for i, row := range rows {
		if row.kind == rowVerbosity {
			option := verbosityOptions[i]
			selected := i == m.settings.cursor
			active := option.value == install.NormalizeChatVerbosity(m.settings.verbosity)
			marker := "  "
			labelStyle := mutedStyle
			if active {
				marker = "✓ "
				labelStyle = successStyle.Bold(true)
			}
			if selected {
				marker = "› "
				labelStyle = selectedStyle
			}
			b.WriteString(labelStyle.Render(marker + option.label))
			b.WriteByte('\n')
			b.WriteString(mutedStyle.Render("    " + option.description))
			b.WriteByte('\n')
			continue
		}
		b.WriteString(m.renderTelegramSettingsRow(row, i))
	}
	b.WriteByte('\n')
	if m.settings.saving {
		b.WriteString(infoStyle.Render("Saving preference…"))
	} else if m.settings.err != "" {
		b.WriteString(errorStyle.Render("Could not save: " + m.settings.err))
	} else if m.settings.editingAbbrev {
		b.WriteString(infoStyle.Render("enter save abbreviation · esc cancel"))
	} else {
		b.WriteString(infoStyle.Render("↑↓ choose · enter apply · esc Chat"))
	}
	return b.String()
}

func (m model) renderTelegramSettingsRow(row settingsRow, index int) string {
	var b strings.Builder
	selected := index == m.settings.cursor
	marker := "  "
	labelStyle := mutedStyle
	if selected {
		marker = "› "
		labelStyle = selectedStyle
	}
	b.WriteString(labelStyle.Render(marker + row.label))
	b.WriteByte('\n')
	if m.settings.editingAbbrev && row.kind == rowTelegramAbbrev {
		b.WriteString(mutedStyle.Render("    " + m.settings.abbrevDraft + "_"))
	} else {
		b.WriteString(mutedStyle.Render("    " + row.desc))
	}
	b.WriteByte('\n')
	return b.String()
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
