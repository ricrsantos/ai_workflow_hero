package tui

import (
	"fmt"
	"strconv"
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
	rowTelegramAutoReport
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
	rows = append(rows, settingsRow{
		kind:  rowTelegramAutoReport,
		label: "Auto report",
		desc:  telegramAutoReportLabel(t.autoReportMinutes),
	})
	if !t.connected {
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Retry", desc: "Start or reconnect the Telegram daemon", action: "retry"})
	}
	if t.paired {
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Replace", desc: "Re-pair with a different chat", action: "replace"})
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Clear", desc: "Remove stored credentials", action: "clear"})
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Test", desc: "Send a test message", action: "test"})
	} else if t.connected {
		rows = append(rows, settingsRow{kind: rowTelegramAction, label: "Pair", desc: "Pair with your bot", action: "pair"})
	}
	return rows
}

func telegramAutoReportLabel(minutes int) string {
	minutes = install.NormalizeTelegramAutoReportMinutes(minutes)
	if minutes == 0 {
		return "Disabled"
	}
	return fmt.Sprintf("Every %d min", minutes)
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
	if !t.connected {
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("  Start the daemon with Retry, then Pair."))
		if errText := strings.TrimSpace(t.daemonErr); errText != "" {
			b.WriteByte('\n')
			b.WriteString(warnStyle.Render("  " + truncateDisplayWidth(errText, max(8, width-2))))
		}
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
		case rowTelegramAutoReport:
			b.WriteString(m.renderTelegramAutoReportRow(row, i == m.settings.cursor, width))
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

func (m model) renderTelegramAutoReportRow(row settingsRow, focused bool, width int) string {
	marker := "  "
	if focused {
		marker = "> "
	}
	label := "Auto report: "
	value := row.desc
	if m.settings.editingAutoReport && focused {
		value = m.settings.autoReportDraft + "█"
	}
	used := lipgloss.Width(marker + label)
	remain := max(8, width-used)
	line := marker + label + truncateDisplayWidth(value, remain)
	if focused {
		return navSidebarFocusedStyle.Render(truncateNavText(line, width))
	}
	return marker + configLabelStyle.Render(label) + configValueStyle.Render(truncateDisplayWidth(value, remain))
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

// openTelegramPairingModal starts pairing immediately and opens the instruction
// modal (telegram-tui R2; UI-C09-001 §2). The token field is only shown later
// if the daemon reports that the vault is empty.
func (m model) openTelegramPairingModal() model {
	if m.telegram == nil || !m.telegram.installed {
		return m
	}
	if !m.telegram.connected {
		notice := "Telegram daemon is not connected — pair after it reconnects."
		m = m.setStatusWarning("telegram", notice)
		m = m.appendTelegramNotice("⚠ " + notice)
		return m
	}
	m.telegram.pairing = true
	m.telegram.pairState = "waiting"
	m.telegram.pairToken = ""
	m.telegram.pairCode = ""
	m.telegram.pairDeadline = time.Time{}
	if m.telegram.client != nil {
		_ = m.telegram.client.Send(ipc.Message{Type: ipc.TypePairStart})
	}
	return m
}

// cancelTelegramPairing invalidates the active code (esc) and closes the modal.
func (m model) cancelTelegramPairing() (model, tea.Cmd) {
	m.telegram.pairing = false
	m.telegram.pairState = ""
	m.telegram.pairToken = ""
	m.telegram.pairCode = ""
	m.telegram.pairDeadline = time.Time{}
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
	case "retry":
		m.telegram.retrying = true
		m.telegram.daemonErr = ""
		m = m.setStatusWarning("telegram", "Starting Telegram daemon…")
		if m.telegram.client != nil && m.telegram.client.daemonPath != "" {
			return m, spawnTelegramDaemonCmd(m.telegram.client.daemonPath, m.telegram.client.socketPath)
		}
		return m, nil
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

// renderTelegramPairingModal renders the focused pairing dialog (telegram-tui
// R2; UI-C09-001 §2). It never renders a token or chat id.
func (m model) renderTelegramPairingModal() string {
	if m.telegram == nil {
		return ""
	}
	boxWidth := minInt(72, max(40, m.width-8))
	if m.width < 48 || m.height < 16 {
		boxWidth = max(24, m.width-4)
	}
	innerWidth := cycleWelcomeInnerWidth(boxWidth)
	t := m.telegram
	rows := []string{
		cycleWelcomeLine(cycleWelcomeTitleStyle, "Pair Telegram", innerWidth),
		cycleWelcomeLine(cycleWelcomeDetailStyle, "[Esc: cancel]", innerWidth),
		cycleWelcomeBlank(innerWidth),
	}
	switch t.pairState {
	case "token":
		rows = append(rows,
			cycleWelcomeWrapped(cycleWelcomeBodyStyle, "The daemon needs a bot token before pairing can start. Paste it here; it is stored only in the OS vault and is never shown.", innerWidth),
			cycleWelcomeBlank(innerWidth),
			cycleWelcomeLine(cycleWelcomeLeadStyle, "Token: "+strings.Repeat("•", len(t.pairToken))+"█", innerWidth),
			cycleWelcomeBlank(innerWidth),
			cycleWelcomeLine(cycleWelcomeDetailStyle, "enter submit · leave empty to retry a stored token", innerWidth),
		)
	case "success":
		rows = append(rows, cycleWelcomeLine(cycleWelcomeLeadStyle, "✓ Telegram paired.", innerWidth))
	case "expired":
		rows = append(rows, cycleWelcomeWrapped(cycleWelcomeBodyStyle, "⚠ Pairing code expired. Start pairing again.", innerWidth))
	default:
		rows = append(rows, m.renderTelegramPairingWaitingRows(innerWidth)...)
	}
	rows = append(rows,
		cycleWelcomeBlank(innerWidth),
		renderPairingCancelButton(innerWidth),
	)
	return m.placeCycleWelcomeBox(strings.Join(rows, "\n"), innerWidth)
}

func (m model) renderTelegramPairingWaitingRows(innerWidth int) []string {
	t := m.telegram
	rows := []string{
		cycleWelcomeWrapped(cycleWelcomeBodyStyle, "1. Open the configured Telegram bot.", innerWidth),
	}
	if t.pairCode != "" {
		rows = append(rows, cycleWelcomeLine(cycleWelcomeLeadStyle, "2. Send: /start "+t.pairCode, innerWidth))
	} else {
		rows = append(rows, cycleWelcomeWrapped(cycleWelcomeDetailStyle, "2. Waiting for a pairing code from the daemon…", innerWidth))
	}
	rows = append(rows,
		cycleWelcomeWrapped(cycleWelcomeBodyStyle, "3. Return here; pairing will complete automatically.", innerWidth),
		cycleWelcomeBlank(innerWidth),
	)
	if !t.pairDeadline.IsZero() {
		rows = append(rows, cycleWelcomeLine(cycleWelcomeDetailStyle, "Code expires in "+pairingRemaining(t.pairDeadline), innerWidth))
	}
	rows = append(rows, cycleWelcomeLine(cycleWelcomeLeadStyle, "Waiting for confirmation…", innerWidth))
	return rows
}

func renderPairingCancelButton(width int) string {
	cancel := cycleWelcomeSelectedStyle.Render("[Cancel]")
	row := lipgloss.PlaceHorizontal(
		width,
		lipgloss.Center,
		cancel,
		lipgloss.WithWhitespaceBackground(colorBgSurface),
	)
	return cycleWelcomeFillStyle.Width(width).Render(row)
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
	case rowTelegramAutoReport:
		m.settings.editingAutoReport = true
		m.settings.autoReportDraft = fmt.Sprintf("%d", m.telegram.autoReportMinutes)
		return m, nil
	case rowTelegramAction:
		return m.handleTelegramSettingsAction(row.action)
	}
	return m, nil
}

type telegramAbbrevSavedMsg struct{ err error }

type telegramAutoReportSavedMsg struct {
	minutes int
	err     error
}

// commitTelegramAbbrev applies the edited abbreviation, re-registers with the
// daemon, and writes telegram.project_abbrev to hero.json (PRD-C09-001 §3.2).
func (m model) commitTelegramAbbrev() (model, tea.Cmd) {
	if m.telegram == nil {
		return m, nil
	}
	normalized := normalizeTelegramAbbrev(m.settings.abbrevDraft)
	m.settings.editingAbbrev = false
	m.settings.abbrevDraft = ""
	m.telegram.abbrev = normalized
	if m.telegram.client != nil {
		m.telegram.client.setAbbrev(normalized)
	}
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	return m, saveTelegramAbbrevCmd(projectDir, normalized)
}

func saveTelegramAbbrevCmd(projectDir, abbrev string) tea.Cmd {
	return func() tea.Msg {
		if projectDir == "" {
			return telegramAbbrevSavedMsg{err: fmt.Errorf("project unavailable")}
		}
		return telegramAbbrevSavedMsg{err: install.SetTelegramProjectAbbrev(projectDir, abbrev)}
	}
}

func saveTelegramAutoReportCmd(projectDir string, minutes int) tea.Cmd {
	return func() tea.Msg {
		if projectDir == "" {
			return telegramAutoReportSavedMsg{err: fmt.Errorf("project unavailable")}
		}
		return telegramAutoReportSavedMsg{minutes: minutes, err: install.SetTelegramAutoReportMinutes(projectDir, minutes)}
	}
}

// telegramSettingsKey handles key input while editing the abbreviation.
func (m model) handleTelegramAbbrevKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.settings.editingAbbrev = false
		m.settings.abbrevDraft = ""
		return m, nil
	case "enter":
		return m.commitTelegramAbbrev()
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

func (m model) handleTelegramAutoReportKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.settings.editingAutoReport = false
		m.settings.autoReportDraft = ""
		return m, nil
	case "enter":
		minutes, err := strconv.Atoi(strings.TrimSpace(m.settings.autoReportDraft))
		if err != nil || minutes < 0 || minutes > 300 {
			m.settings.err = "Auto report must be 0 (disabled) or 1–300 minutes."
			return m, nil
		}
		m.settings.editingAutoReport = false
		m.settings.autoReportDraft = ""
		projectDir := ""
		if m.svc != nil {
			projectDir = m.svc.ProjectDir
		}
		return m, saveTelegramAutoReportCmd(projectDir, minutes)
	case "backspace":
		if len(m.settings.autoReportDraft) > 0 {
			m.settings.autoReportDraft = m.settings.autoReportDraft[:len(m.settings.autoReportDraft)-1]
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			for _, r := range msg.Runes {
				if r < '0' || r > '9' {
					return m, nil
				}
			}
			m.settings.autoReportDraft += string(msg.Runes)
		}
		return m, nil
	}
}

func pairingRemaining(t time.Time) string {
	if t.IsZero() {
		return "00:00"
	}
	rem := time.Until(t)
	if rem < 0 {
		return "00:00"
	}
	total := int(rem.Seconds())
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}
