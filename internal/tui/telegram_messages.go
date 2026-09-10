package tui

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

const (
	telegramInterruptCommand         = "/interrupt"
	telegramKillCommand              = "/kill"
	telegramKillOutboundText         = "Killing TUI."
	telegramHarnessPermissionCommand = "/hero-permission"
)

// telegramForceKillProcess terminates the TUI process immediately. Tests
// replace it so unit coverage never SIGKILLs the test binary.
var telegramForceKillProcess = func() {
	pid := os.Getpid()
	_ = syscall.Kill(pid, syscall.SIGKILL)
	// Unreachable after a successful SIGKILL; keep a hard fallback so a
	// last-resort /kill never returns into a stuck Update loop.
	os.Exit(1)
}

func isTelegramKillCommand(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), telegramKillCommand)
}

// telegramOriginLabel renders the directional transcript label for a
// Telegram-routed message (UI-C09-001 §3). User messages use ←; answering agent
// messages use →. It returns ok=false for local turns.
func telegramOriginLabel(msg convMessage) (string, bool) {
	if msg.origin == "" {
		return "", false
	}
	addr := strings.TrimPrefix(msg.origin, "telegram:")
	if addr == "" {
		addr = "?"
	}
	switch msg.role {
	case convRoleUser:
		return "← [Telegram · " + addr + "]", true
	case convRoleAgent:
		return "→ [Telegram · " + addr + "]", true
	default:
		return "", false
	}
}

// handleTelegramMsg processes a daemon-pushed frame. Production delivers these
// via tea.Program.Send (launch relay), so this handler must not depend on
// re-issuing waitTelegramMsg.
func (m model) handleTelegramMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.telegram == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case telegramConnectedMsg:
		m.telegram.connected = true
		m.telegram.retrying = false
		m.telegram.daemonErr = ""
		m, notificationCmd := m.flushPendingTelegramNotifications()
		return m, combineTimerCmds(notificationCmd, m.ensureTimerLoop())

	case telegramRegisteredMsg:
		m.telegram.address = msg.address
		m.telegram.paired = msg.paired
		slog.Info("telegram client registered", "address", msg.address, "paired", msg.paired)
		m, notificationCmd := m.flushPendingTelegramNotifications()
		return m, combineTimerCmds(notificationCmd, m.ensureTimerLoop())

	case telegramDisconnectedMsg:
		wasConnected := m.telegram.connected
		m.telegram.connected = false
		m.telegram.retrying = true
		m.telegram.daemonErr = msg.err
		if wasConnected || m.telegram.address != "" {
			m = m.appendTelegramNotice("⚠ Telegram daemon disconnected; retrying…")
		}
		slog.Debug("telegram client disconnected", "error", msg.err)
		return m, nil

	case telegramEventMsg:
		m = m.handleTelegramEvent(msg)
		m, notificationCmd := m.flushPendingTelegramNotifications()
		if m.restartRequested {
			restartCmds := []tea.Cmd{tea.Quit}
			if m.streaming {
				restartCmds = append(restartCmds, m.cancelStreamCmd())
			}
			return m, combineTimerCmds(append([]tea.Cmd{
				notificationCmd,
				m.telegramOutboundCmd("Hero update installed; restarting this TUI."),
			}, restartCmds...)...)
		}
		return m, notificationCmd

	case telegramInboundMsg:
		return m.handleTelegramInbound(msg)
	}
	return m, nil
}

func (m model) handleTelegramEvent(msg telegramEventMsg) model {
	switch msg.eventType {
	case ipc.EventPairingProgress:
		if msg.data == "missing-token" {
			m.telegram.pairing = true
			m.telegram.pairState = "token"
			m.telegram.pairCode = ""
			return m
		}
		m.telegram.pairing = true
		m.telegram.pairCode = msg.data
		m.telegram.pairState = "waiting"
		m.telegram.pairDeadline = time.Now().Add(10 * time.Minute)
	case ipc.EventPairingSuccess:
		m.telegram.pairing = false
		m.telegram.pairState = ""
		m.telegram.pairCode = ""
		m.telegram.pairToken = ""
		m.telegram.pairDeadline = time.Time{}
		m.telegram.paired = true
		m = m.appendTelegramNotice("✓ Telegram paired.")
		m = m.setStatusResult(true, "telegram", "Telegram paired.")
	case ipc.EventPairingExpired:
		if m.telegram.pairing {
			m.telegram.pairing = false
			m.telegram.pairState = ""
			m.telegram.pairCode = ""
			m.telegram.pairDeadline = time.Time{}
			m = m.appendTelegramNotice("⚠ Pairing code expired. Start pairing again.")
			m = m.setStatusWarning("telegram", "Pairing code expired. Start pairing again.")
		}
	case ipc.EventUpdateRestart:
		m.restartRequested = true
		m = m.appendTelegramNotice("Hero update received; preparing a TUI restart…")
	case ipc.EventDaemonUp:
		m.telegram.connected = true
		m.telegram.retrying = false
		m = m.appendTelegramNotice("✓ Telegram daemon reconnected.")
	case ipc.EventCleared:
		m.telegram.paired = false
		m.telegram.pairState = ""
		m = m.appendTelegramNotice("Telegram credentials cleared.")
	}
	return m
}

// handleTelegramInbound routes an addressed inbound frame through the same
// slash-vs-plain classification as the composer (conversation-service R1) and
// acknowledges queued deliveries (telegram-ipc R3).
func (m model) handleTelegramInbound(msg telegramInboundMsg) (model, tea.Cmd) {
	ack := m.telegramAckCmd(msg.inboundID)
	if telegram.IsHelpCommand(msg.text) {
		return m, combineTimerCmds(ack, m.telegramOutboundCmd(telegram.CommandHelpText()))
	}
	if isTelegramKillCommand(msg.text) {
		// Defense in depth: production /kill is handled in the IPC client
		// goroutine before Program.Send. This path covers tests and any
		// inbound that still reaches Update.
		m.applyTelegramKill(msg.inboundID)
		return m, nil
	}
	if isTelegramAutoUpdateCommand(msg.text) {
		next, cmd := m.handleTelegramAutoUpdate()
		return next, combineTimerCmds(ack, cmd)
	}
	if strings.EqualFold(strings.TrimSpace(msg.text), telegramStatusCommand) {
		return m, combineTimerCmds(ack, m.telegramOutboundCmd(m.telegramStatusText(time.Now())))
	}
	if strings.EqualFold(strings.TrimSpace(msg.text), telegramInterruptCommand) {
		next, cmd := m.handleTelegramInterrupt()
		return next, combineTimerCmds(ack, cmd)
	}
	if permissionID, approved, matched, valid := parseTelegramHarnessPermission(msg.text); matched {
		if !valid {
			return m, combineTimerCmds(ack, m.telegramOutboundCmd("Use /hero-permission <id> allow or /hero-permission <id> deny."))
		}
		next, cmd := m.handleTelegramHarnessPermission(msg, permissionID, approved)
		return next, combineTimerCmds(ack, cmd)
	}
	if next, cmd, handled := m.handleTelegramConfigCommand(msg.text, msg.address); handled {
		return next, combineTimerCmds(ack, cmd)
	}
	if m.telegram != nil && m.telegram.configWizard != nil {
		if m.telegram.modelSelection != nil {
			if strings.EqualFold(strings.TrimSpace(msg.text), slashModel) || strings.EqualFold(strings.TrimSpace(msg.text), "/hero-model") {
				selection := m.telegram.modelSelection
				next, cmd := m.startTelegramModelSelectionFor(msg.address, selection.configAgent, selection.configSubagent)
				return next, combineTimerCmds(ack, cmd)
			}
			next, cmd := m.handleTelegramModelSelection(msg.address, msg.text)
			return next, combineTimerCmds(ack, cmd)
		}
		next, cmd := m.handleTelegramConfigInput(msg.address, msg.text)
		return next, combineTimerCmds(ack, cmd)
	}
	if m.telegram != nil && m.telegram.modelSelection != nil {
		if strings.EqualFold(strings.TrimSpace(msg.text), slashModel) || strings.EqualFold(strings.TrimSpace(msg.text), "/hero-model") {
			next, cmd := m.startTelegramModelSelection(msg.address)
			return next, combineTimerCmds(ack, cmd)
		}
		next, cmd := m.handleTelegramModelSelection(msg.address, msg.text)
		return next, combineTimerCmds(ack, cmd)
	}

	// Classify through the shared service so the remote transport obeys the
	// same slash-vs-text rule as the composer (ADR-061).
	isCommand := msg.isCommand
	if m.convService != nil {
		isCommand = m.convService.Classify(msg.text).Kind == conversation.KindSlash
	}

	origin := "telegram:" + msg.address
	var next model
	var cmd tea.Cmd
	if isCommand {
		next, cmd = m.submitRemoteCommand(msg.text, origin)
	} else {
		next, cmd = m.submitRemoteTurn(msg.text, origin)
	}
	return next, combineTimerCmds(ack, cmd)
}

// handleTelegramInterrupt mirrors Chat Ctrl+C. It cancels every in-flight
// Execute, including concurrent stage executions and /hero-start preflight,
// without sending the command through a harness turn.
func (m model) handleTelegramInterrupt() (model, tea.Cmd) {
	if m.heroStartBootstrapping || m.heroStartPreparing {
		next, cmd := m.cancelHeroStartPreparation()
		return next, combineTimerCmds(cmd, next.telegramOutboundCmd("Interrupted."))
	}
	if m.streaming {
		return m, combineTimerCmds(m.cancelStreamCmd(), m.telegramOutboundCmd("Interrupt requested."))
	}
	return m, m.telegramOutboundCmd("No process is running.")
}

// applyTelegramKill is the last-resort remote shutdown. It best-effort acks
// delivery, sends a short outbound notice, then force-kills this process.
// It must not depend on Bubble Tea completing a Cmd.
func (m model) applyTelegramKill(inboundID string) {
	if m.telegram != nil && m.telegram.recordOutbound != nil {
		m.telegram.recordOutbound(telegramKillOutboundText)
	}
	if m.telegram != nil && m.telegram.client != nil {
		m.telegram.client.applyTelegramKill(inboundID)
		return
	}
	telegramForceKillProcess()
}

func parseTelegramHarnessPermission(text string) (id string, approved, matched, valid bool) {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.EqualFold(fields[0], telegramHarnessPermissionCommand) {
		return "", false, false, false
	}
	if len(fields) != 3 {
		return "", false, true, false
	}
	id = strings.TrimSpace(fields[1])
	if id == "" {
		return "", false, true, false
	}
	switch strings.ToLower(strings.TrimSpace(fields[2])) {
	case "allow":
		return id, true, true, true
	case "deny":
		return id, false, true, true
	default:
		return "", false, true, false
	}
}

func (m model) handleTelegramHarnessPermission(msg telegramInboundMsg, id string, approved bool) (model, tea.Cmd) {
	if m.telegram != nil && strings.TrimSpace(m.telegram.address) != "" &&
		strings.TrimSpace(msg.address) != "" && !strings.EqualFold(m.telegram.address, msg.address) {
		return m, m.telegramOutboundCmd("Permission response rejected: this Telegram instance is no longer the active target.")
	}
	if !m.hasHarnessPermission(id) {
		return m, m.telegramOutboundCmd(fmt.Sprintf("No pending harness permission matches %q. It may have expired or been interrupted.", id))
	}
	reason := ""
	if !approved {
		reason = "telegram denied"
	}
	m = m.replyHarnessPermissionID(id, approved, reason)
	decision := "denied"
	if approved {
		decision = "allowed"
	}
	return m, m.telegramOutboundCmd(fmt.Sprintf("Harness permission %s %s.", id, decision))
}

func telegramHarnessPermissionText(req harness.PermissionRequest) string {
	id := strings.TrimSpace(req.ID)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Harness permission"
	}
	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > 1600 {
		description = string([]rune(description)[:1600]) + "…"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Harness permission requested: %s", title)
	if description != "" {
		fmt.Fprintf(&b, "\n%s", description)
	}
	if id == "" {
		b.WriteString("\nThis request has no remote id; answer it in the TUI with y/n.")
		return b.String()
	}
	fmt.Fprintf(&b, "\nID: %s\nReply /hero-permission %s allow or /hero-permission %s deny.", id, id, id)
	return b.String()
}

func (m model) telegramHarnessPermissionCmd(req harness.PermissionRequest) (model, tea.Cmd) {
	if m.telegram == nil {
		return m, nil
	}
	if !m.telegram.connected || !m.telegram.paired {
		id := harnessPermissionKey(req)
		for _, pending := range m.pendingHarnessPermissionNotices {
			if harnessPermissionKey(pending) == id {
				return m, nil
			}
		}
		m.pendingHarnessPermissionNotices = append(m.pendingHarnessPermissionNotices, req)
		return m, nil
	}
	return m, m.telegramOutboundCmd(telegramHarnessPermissionText(req))
}

func (m model) removePendingHarnessPermissionNotice(id string) model {
	id = strings.TrimSpace(id)
	if id == "" {
		return m
	}
	filtered := m.pendingHarnessPermissionNotices[:0]
	for _, req := range m.pendingHarnessPermissionNotices {
		if harnessPermissionKey(req) != id {
			filtered = append(filtered, req)
		}
	}
	m.pendingHarnessPermissionNotices = filtered
	return m
}

func (m model) flushPendingHarnessPermissionNotices() (model, tea.Cmd) {
	if m.telegram == nil || !m.telegram.connected || !m.telegram.paired || len(m.pendingHarnessPermissionNotices) == 0 {
		return m, nil
	}
	notices := m.pendingHarnessPermissionNotices
	m.pendingHarnessPermissionNotices = nil
	cmds := make([]tea.Cmd, 0, len(notices))
	for _, req := range notices {
		if text := telegramHarnessPermissionText(req); text != "" {
			cmds = append(cmds, m.telegramOutboundCmd(text))
		}
	}
	return m, combineTimerCmds(cmds...)
}

func (m model) telegramAckCmd(inboundID string) tea.Cmd {
	if inboundID == "" || m.telegram == nil || m.telegram.client == nil {
		return nil
	}
	client := m.telegram.client
	id := inboundID
	return func() tea.Msg {
		if err := client.Send(ipc.Message{Type: ipc.TypeAckDelivery, AckID: id}); err != nil {
			slog.Debug("telegram ack failed", "error", err)
		}
		return nil
	}
}

// submitRemoteCommand routes a Telegram-originated slash command through the
// exact Hero slash dispatcher used by the composer.
func (m model) submitRemoteCommand(text, origin string) (model, tea.Cmd) {
	if strings.EqualFold(strings.TrimSpace(text), slashModel) || strings.EqualFold(strings.TrimSpace(text), "/hero-model") {
		return m.startTelegramModelSelection(strings.TrimPrefix(origin, "telegram:"))
	}
	m.nextUserOrigin = origin
	m = m.clearChatInput()
	next, cmd, ok := m.dispatchExactHeroSlash(text)
	if !ok {
		// Not a recognized control slash: run it as an ordinary harness turn so
		// the Runtime sees the same text it would from the composer.
		m = m.setRemoteOrigin(origin)
		return m.submitRemoteTurn(text, origin)
	}
	if next.streaming {
		cmd = combineTimerCmds(cmd, next.telegramOutboundCmd(next.telegramAutoReportText(time.Now())))
	}
	return next, cmd
}

type telegramPendingTurn struct {
	text   string
	origin string
}

// submitRemoteTurn starts a harness turn for a Telegram-originated plain-text
// message. It mirrors the composer follow-up path without touching the composer.
func (m model) submitRemoteTurn(text, origin string) (model, tea.Cmd) {
	m.nextUserOrigin = origin
	if m.streaming {
		return m.enqueueTelegramPendingTurn(text, origin)
	}

	if m.researchLive {
		m = m.prepareDiscoverFollowUp()
	} else if m.orchestrationLive || m.workflowAgentActive() {
		if strings.TrimSpace(m.runtimeModelSlug) == "" {
			var cmd tea.Cmd
			var ok bool
			m, cmd, _, ok = m.orchestratorExecuteModel("chat")
			if !ok {
				return m, cmd
			}
		}
	} else {
		var cmd tea.Cmd
		var ok bool
		m, cmd, ok = m.ensureDefaultModel("chat")
		if !ok {
			return m, cmd
		}
	}
	m.runtimeCommandName = ""
	m = m.syncConversationContext()
	m = m.beginConversationExecute(text, controlSlashFollowUpPrompt(text))
	return m, combineTimerCmds(m.conversationExecuteCmds(), m.telegramOutboundCmd(m.telegramAutoReportText(time.Now())))
}

// enqueueTelegramPendingTurn defers one remote turn until the active Execute
// finishes. The PRD asks for an immediate status when the turn is queued, not
// a retry loop that re-sends that status while the harness is still running.
func (m model) enqueueTelegramPendingTurn(text, origin string) (model, tea.Cmd) {
	text = strings.TrimSpace(text)
	origin = strings.TrimSpace(origin)
	if text == "" {
		return m, nil
	}
	for _, pending := range m.telegramPendingTurns {
		if pending.text == text && pending.origin == origin {
			return m, nil
		}
	}
	m.telegramPendingTurns = append(m.telegramPendingTurns, telegramPendingTurn{text: text, origin: origin})
	return m, m.telegramOutboundCmd(m.telegramAutoReportText(time.Now()))
}

// drainTelegramPendingTurn starts the oldest queued remote turn after the TUI
// is no longer streaming. It does not re-inject a synthetic inbound frame.
func (m model) drainTelegramPendingTurn() (model, tea.Cmd) {
	if m.streaming || len(m.telegramPendingTurns) == 0 {
		return m, nil
	}
	next := m.telegramPendingTurns[0]
	m.telegramPendingTurns = append([]telegramPendingTurn(nil), m.telegramPendingTurns[1:]...)
	return m.submitRemoteTurn(next.text, next.origin)
}

func (m model) afterExecuteTelegramDrain(cmd tea.Cmd) (model, tea.Cmd) {
	if m.streaming {
		return m, cmd
	}
	next, drainCmd := m.drainTelegramPendingTurn()
	return next, combineTimerCmds(cmd, drainCmd)
}

// setRemoteOrigin re-applies the Telegram origin to the next turn.
func (m model) setRemoteOrigin(origin string) model {
	m.nextUserOrigin = origin
	return m
}

// appendTelegramNotice adds a muted informational line to the Chat transcript
// (used for daemon outage/recovery and pending/cancel notices; UI-C09-001 §4).
func (m model) appendTelegramNotice(text string) model {
	m.transcript = append(m.transcript, convMessage{role: convRoleWarning, content: text})
	return m
}

func telegramAddressOf(origin string) (string, bool) {
	if !strings.HasPrefix(origin, "telegram:") {
		return "", false
	}
	addr := strings.TrimPrefix(origin, "telegram:")
	if addr == "" {
		return "", false
	}
	return addr, true
}

func (m model) telegramTurnOrigin(meta convExecute) string {
	if _, ok := telegramAddressOf(meta.Origin); ok {
		return meta.Origin
	}
	if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
		return m.transcript[m.agentMsgIndex].origin
	}
	return ""
}

func telegramTurnReplyText(origin, output, errText string, turnComplete bool) string {
	if !turnComplete {
		return ""
	}
	if _, ok := telegramAddressOf(origin); !ok {
		return ""
	}
	return telegramTurnReplyBody(output, errText)
}

func telegramTurnReplyBody(output, errText string) string {
	if text := strings.TrimSpace(output); text != "" {
		return text
	}
	return strings.TrimSpace(errText)
}

func (m model) telegramTurnReplyText(origin, output, errText string, turnComplete bool) string {
	if !turnComplete {
		return ""
	}
	if _, ok := telegramAddressOf(origin); !ok && (m.telegram == nil || !m.telegram.alwaysSend) {
		return ""
	}
	return telegramTurnReplyBody(output, errText)
}

func appendTelegramConfigHint(reply string, cycleNumber int) string {
	hint := "\n\nCiclo preparado. Use /hero-config para configurar título, objetivo, escopo, stages e modelos; use /hero-config-show para consultar a configuração."
	if cycleNumber > 0 {
		hint = fmt.Sprintf("\n\nCiclo C%d preparado. Use /hero-config para configurar título, objetivo, escopo, stages e modelos; use /hero-config-show para consultar a configuração.", cycleNumber)
	}
	if strings.TrimSpace(reply) == "" {
		return strings.TrimSpace(hint)
	}
	return strings.TrimSpace(reply) + hint
}

// telegramMaxOutboundRunes leaves headroom for the daemon's address prefix
// (Telegram Bot API messages are capped at 4096 characters).
const telegramMaxOutboundRunes = 3900

func splitTelegramOutbound(text string) []string {
	runes := []rune(text)
	if len(runes) <= telegramMaxOutboundRunes {
		if text == "" {
			return nil
		}
		return []string{text}
	}
	out := make([]string, 0, (len(runes)+telegramMaxOutboundRunes-1)/telegramMaxOutboundRunes)
	for len(runes) > 0 {
		n := telegramMaxOutboundRunes
		if n > len(runes) {
			n = len(runes)
		}
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

func (m model) telegramTurnReplyCmd(origin, output, errText string, turnComplete bool) tea.Cmd {
	return m.telegramOutboundCmd(m.telegramTurnReplyText(origin, output, errText, turnComplete))
}

func (m model) telegramOutboundCmd(text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if text == "" || m.telegram == nil || !m.telegram.connected {
		return nil
	}
	if m.telegram.recordOutbound != nil {
		m.telegram.recordOutbound(text)
	}
	if m.telegram.client == nil {
		return nil
	}
	client := m.telegram.client
	chunks := splitTelegramOutbound(text)
	return func() tea.Msg {
		for _, chunk := range chunks {
			if err := client.Send(ipc.Message{Type: ipc.TypeOutbound, OutboundText: chunk}); err != nil {
				slog.Debug("telegram conversation reply failed", "error", err)
				return nil
			}
		}
		return nil
	}
}
