package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

// settingsRowKind classifies one row in the Settings screen. Verbosity profiles
// keep their existing behavior; Telegram rows are appended only when the plugin
// is installed (hero-tui R1; telegram-tui R1).
type settingsRowKind int

const (
	rowVerbosity settingsRowKind = iota
	rowTelegramInfo
	rowTelegramDaemon
	rowTelegramState
	rowTelegramAbbrev
	rowTelegramAction
	rowTelegramNotInstalled
)

type settingsRow struct {
	kind   settingsRowKind
	label  string
	desc   string
	action string // rowTelegramAction: pair | replace | clear | test
}

// settingsRows returns the ordered, selectable Settings rows.
func (m model) settingsRows() []settingsRow {
	rows := make([]settingsRow, 0, 8)
	for _, o := range verbosityOptions {
		rows = append(rows, settingsRow{kind: rowVerbosity, label: o.label, desc: o.description})
	}
	if m.telegram == nil || !m.telegram.installed {
		rows = append(rows, settingsRow{
			kind:  rowTelegramNotInstalled,
			label: "Telegram",
			desc:  "Not installed — Install with: hero plugin install telegram",
		})
		return rows
	}
	t := m.telegram
	rows = append(rows, settingsRow{
		kind:  rowTelegramInfo,
		label: "Telegram",
		desc:  fmt.Sprintf("Installed v%s (protocol v%d)", t.pluginVersion, t.protocolVersion),
	})
	daemonDesc := "Daemon: disconnected"
	if t.connected {
		daemonDesc = "Daemon: connected · " + t.address
	} else if t.retrying {
		daemonDesc = "Daemon: disconnected — retrying…"
	}
	rows = append(rows, settingsRow{kind: rowTelegramDaemon, label: "Daemon", desc: daemonDesc})
	stateDesc := "State: Not configured"
	if t.paired {
		stateDesc = "State: Configured"
	}
	rows = append(rows, settingsRow{kind: rowTelegramState, label: "Pairing", desc: stateDesc})
	rows = append(rows, settingsRow{
		kind:  rowTelegramAbbrev,
		label: "Project address",
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
	case rowTelegramAbbrev:
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
