package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

const telegramInstallCommand = "hero plugin install telegram"

// settingsRowKind classifies one focusable row in the Settings screen.
// Status chrome (plugin badge, daemon line) is display-only.
type settingsRowKind int

const (
	rowVerbosity settingsRowKind = iota
	rowTelegramCopyCommand
	rowTelegramAbbrev
	rowTelegramAction
)

type settingsRow struct {
	kind      settingsRowKind
	label     string
	desc      string
	action    string // rowTelegramAction: pair | replace | clear | test
	verbosity install.ChatVerbosity
}

// settingsRows returns the ordered, selectable Settings rows.
func (m model) settingsRows() []settingsRow {
	rows := make([]settingsRow, 0, 8)
	for _, o := range verbosityOptions {
		rows = append(rows, settingsRow{
			kind:      rowVerbosity,
			label:     o.label,
			desc:      o.description,
			verbosity: o.value,
		})
	}
	if m.telegram == nil || !m.telegram.installed {
		rows = append(rows, settingsRow{
			kind:  rowTelegramCopyCommand,
			label: "Copy command",
			desc:  "Install with: " + telegramInstallCommand,
		})
		return rows
	}
	t := m.telegram
	rows = append(rows, settingsRow{
		kind:  rowTelegramAbbrev,
		label: "Project ID",
		desc:  fmt.Sprintf("%s (live: %s)", t.abbrev, displayAddress(t.address)),
	})
	if t.paired {
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Replace", desc: "Re-pair with a different chat", action: "replace"})
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Clear", desc: "Remove stored credentials", action: "clear"})
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Test", desc: "Send a test message", action: "test"})
	} else {
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Pair", desc: "Pair with your bot", action: "pair"})
	}
	return rows
}

func displayAddress(addr string) string {
	if addr == "" {
		return "—"
	}
	return addr
}

func (m model) renderTelegramPluginSection(rows []settingsRow, width int) string {
	if m.telegram == nil || !m.telegram.installed {
		return m.renderTelegramNotInstalled(rows, width)
	}
	return m.renderTelegramInstalled(rows, width)
}

func (m model) renderTelegramNotInstalled(rows []settingsRow, width int) string {
	focused := false
	for i, row := range rows {
		if row.kind == rowTelegramCopyCommand && i == m.settings.cursor {
			focused = true
			break
		}
	}
	var b strings.Builder
	b.WriteString(mutedStyle.Render("  Status:  "))
	b.WriteString(settingsBadgeStyle.Render("Not installed"))
	b.WriteString("\n\n")
	b.WriteString(settingsCommandTextStyle.Render(truncateDisplayWidth("$ "+telegramInstallCommand, width)))
	b.WriteString("\n\n  ")
	b.WriteString(renderSettingsButton("Copy command", focused))
	return b.String()
}

func (m model) renderTelegramInstalled(rows []settingsRow, width int) string {
	t := m.telegram
	var b strings.Builder
	b.WriteString(mutedStyle.Render("  Status:  "))
	if t.paired {
		b.WriteString(settingsBadgeOKStyle.Render("Configured"))
	} else {
		b.WriteString(settingsBadgeStyle.Render("Not configured"))
	}
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("  Plugin:  "))
	b.WriteString(configValueStyle.Render(fmt.Sprintf("Installed · v%s (protocol v%d)", t.pluginVersion, t.protocolVersion)))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("  Daemon:  "))
	switch {
	case t.connected:
		b.WriteString(successStyle.Render("Connected"))
	case t.retrying:
		b.WriteString(warnStyle.Render("Disconnected — retrying…"))
	default:
		b.WriteString(mutedStyle.Render("Disconnected"))
	}
	b.WriteString("\n\n")

	abbrevFocused := false
	var actions []settingsRow
	actionFocus := -1
	for i, row := range rows {
		switch row.kind {
		case rowTelegramAbbrev:
			abbrevFocused = i == m.settings.cursor
			b.WriteString(m.renderProjectIDRow(row, abbrevFocused, width))
			b.WriteByte('\n')
		case rowTelegramAction:
			if i == m.settings.cursor {
				actionFocus = len(actions)
			}
			actions = append(actions, row)
		}
	}
	if len(actions) > 0 {
		b.WriteByte('\n')
		b.WriteString("  ")
		var parts []string
		for i, a := range actions {
			parts = append(parts, renderSettingsButton(a.label, i == actionFocus))
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, joinButtonGap(parts)...))
	}
	return b.String()
}

func joinButtonGap(buttons []string) []string {
	if len(buttons) == 0 {
		return buttons
	}
	out := make([]string, 0, len(buttons)*2-1)
	for i, btn := range buttons {
		if i > 0 {
			out = append(out, "  ")
		}
		out = append(out, btn)
	}
	return out
}

func renderSettingsButton(label string, focused bool) string {
	text := "| " + label + " |"
	if focused {
		return navSidebarFocusedStyle.Render(text)
	}
	return mutedStyle.Render(text)
}

func (m model) renderProjectIDRow(row settingsRow, focused bool, width int) string {
	marker := "  "
	if focused {
		marker = "> "
	}
	label := "Project ID: "
	value := row.desc
	if m.settings.editingAbbrev && focused {
		value = m.settings.abbrevDraft + "█"
	}
	used := lipgloss.Width(marker + label)
	remain := width - used
	if remain < 8 {
		remain = 8
	}
	line := marker + label + truncateDisplayWidth(value, remain)
	if focused {
		return navSidebarFocusedStyle.Render(truncateNavText(line, width))
	}
	return marker + configLabelStyle.Render(label) + configValueStyle.Render(truncateDisplayWidth(value, remain))
}

// openTelegramPairingModal opens the pairing modal (telegram-tui R2).
func (m model) openTelegramPairingModal() model {
	if m.telegram == nil || !m.telegram.installed {
		return m
	}
	if !m.telegram.connected {
		m = m.appendTelegramNotice("⚠ Telegram daemon is not connected — pair after it reconnects.")
		return m
	}
	m.telegram.pairing = true
	m.telegram.pairState = "token"
	m.telegram.pairToken = ""
	m.telegram.pairCode = ""
	return m
}

// renderTelegramPairingModalOpen marks the modal as open after the daemon
// issues a code (used by handleTelegramEvent when not on the Settings screen).
func (m model) renderTelegramPairingModalOpen() model {
	m.telegram.pairing = true
	return m
}

// cancelTelegramPairing invalidates the active code (esc) and closes the modal.
func (m model) cancelTelegramPairing() (model, tea.Cmd) {
	m.telegram.pairing = false
	m.telegram.pairState = ""
	m.telegram.pairToken = ""
	m.telegram.pairCode = ""
	if m.telegram.client != nil {
		_ = m.telegram.client.Send(ipc.Message{Type: ipc.TypePairCancel})
	}
	return m, nil
}

func (m model) handleTelegramPairingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.telegram.pairState {
	case "token":
		switch msg.String() {
		case "esc":
			return m.cancelTelegramPairing()
		case "enter":
			token := strings.TrimSpace(m.telegram.pairToken)
			m.telegram.pairToken = ""
			m.telegram.pairState = "waiting"
			if m.telegram.client != nil {
				if token != "" {
					_ = m.telegram.client.Send(ipc.Message{Type: ipc.TypeSetCredentials, Token: token})
				}
				_ = m.telegram.client.Send(ipc.Message{Type: ipc.TypePairStart})
			}
			return m, nil
		case "backspace":
			if len(m.telegram.pairToken) > 0 {
				m.telegram.pairToken = m.telegram.pairToken[:len(m.telegram.pairToken)-1]
			}
			return m, nil
		default:
			if len(msg.Runes) > 0 && !msg.Alt {
				m.telegram.pairToken += string(msg.Runes)
			}
			return m, nil
		}
	case "waiting":
		switch msg.String() {
		case "esc":
			return m.cancelTelegramPairing()
		case "enter":
			return m, nil
		}
		return m, nil
	case "success", "expired":
		m.telegram.pairing = false
		m.telegram.pairState = ""
		return m, nil
	default:
		return m.cancelTelegramPairing()
	}
}

// handleTelegramSettingsAction dispatches a Telegram action row.
func (m model) handleTelegramSettingsAction(action string) (model, tea.Cmd) {
	switch action {
	case "pair", "replace":
		return m.openTelegramPairingModal(), nil
	case "clear":
		if m.telegram.client != nil {
			_ = m.telegram.client.Send(ipc.Message{Type: ipc.TypeClear})
		}
		m = m.appendTelegramNotice("Clearing Telegram credentials…")
		return m, nil
	case "test":
		if m.telegram.client != nil {
			_ = m.telegram.client.Send(ipc.Message{Type: ipc.TypeTest})
		}
		m = m.appendTelegramNotice("Test message sent to Telegram.")
		return m, nil
	}
	return m, nil
}

// renderTelegramPairingModal renders the focused pairing modal (telegram-tui
// R2): numbered steps, a masked token field, visible code + countdown, and
// waiting/success/expiry states. It never renders the token or chat id.
func (m model) renderTelegramPairingModal() string {
	t := m.telegram
	var b strings.Builder
	b.WriteString(headerStyle.Render("Pair with Telegram"))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("1. Open your Telegram bot and send it the pairing code."))
	b.WriteByte('\n')
	switch t.pairState {
	case "token":
		b.WriteString(mutedStyle.Render("2. Enter your bot token (stored only in the OS vault)."))
		b.WriteByte('\n')
		b.WriteString(infoStyle.Render("   Token: " + strings.Repeat("•", len(t.pairToken))))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("   (enter submits; press enter empty to reuse a stored token)"))
	case "waiting":
		if t.pairCode != "" {
			b.WriteString(infoStyle.Render("2. Pairing code: " + t.pairCode))
		} else {
			b.WriteString(infoStyle.Render("2. Waiting for the daemon to issue a code…"))
		}
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("   Countdown: " + pairingRemaining(t.pairDeadline)))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("3. Send /start <code> (or the bare code) to the bot."))
	case "success":
		b.WriteString(successStyle.Render("✓ Paired successfully."))
	case "expired":
		b.WriteString(errorStyle.Render("Pairing code expired."))
	}
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("esc cancel"))
	return b.String()
}

// telegramSettingsAction dispatches enter on a Telegram settings row.
func (m model) telegramSettingsEnter(row settingsRow) (model, tea.Cmd) {
	switch row.kind {
	case rowTelegramCopyCommand:
		m = m.setStatusResult(true, "settings", "Copied: "+telegramInstallCommand)
		return m, copyToClipboardCmd(telegramInstallCommand)
	case rowTelegramAbbrev:
		if m.telegram == nil {
			return m, nil
		}
		m.settings.editingAbbrev = true
		m.settings.abbrevDraft = m.telegram.abbrev
		return m, nil
	case rowTelegramAction:
		return m.handleTelegramSettingsAction(row.action)
	}
	return m, nil
}

// commitTelegramAbbrev persists the edited abbreviation and re-registers with
// the daemon so the allocated address reflects the new base.
func (m model) commitTelegramAbbrev() model {
	normalized := normalizeTelegramAbbrev(m.settings.abbrevDraft)
	m.settings.editingAbbrev = false
	m.settings.abbrevDraft = ""
	if normalized == m.telegram.abbrev {
		return m
	}
	m.telegram.abbrev = normalized
	if m.telegram.client != nil {
		m.telegram.client.setAbbrev(normalized)
	}
	return m
}

// telegramSettingsKey handles key input while editing the abbreviation.
func (m model) handleTelegramAbbrevKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.settings.editingAbbrev = false
		m.settings.abbrevDraft = ""
		return m, nil
	case "enter":
		return m.commitTelegramAbbrev(), nil
	case "backspace":
		if len(m.settings.abbrevDraft) > 0 {
			m.settings.abbrevDraft = m.settings.abbrevDraft[:len(m.settings.abbrevDraft)-1]
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			m.settings.abbrevDraft += string(msg.Runes)
		}
		return m, nil
	}
}

func pairingRemaining(t time.Time) string {
	rem := time.Until(t)
	if rem < 0 {
		return "0:00"
	}
	m := int(rem.Minutes())
	s := int(rem.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
