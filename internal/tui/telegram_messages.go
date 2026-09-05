package tui

import (
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

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
		return m, nil

	case telegramRegisteredMsg:
		m.telegram.address = msg.address
		m.telegram.paired = msg.paired
		slog.Info("telegram client registered", "address", msg.address, "paired", msg.paired)
		return m, nil

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
		return m.handleTelegramEvent(msg), nil

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
	return next, cmd
}

// submitRemoteTurn starts a harness turn for a Telegram-originated plain-text
// message. It mirrors the composer follow-up path without touching the composer.
func (m model) submitRemoteTurn(text, origin string) (model, tea.Cmd) {
	m.nextUserOrigin = origin
	if m.streaming {
		// Queue the turn behind the active one rather than interrupting it.
		return m, m.appendTelegramPendingTurnCmd(text, origin)
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
	return m, m.conversationExecuteCmds()
}

// appendTelegramPendingTurnCmd defers a Telegram turn until the active harness
// turn finishes (best-effort; not persisted beyond this TUI session).
func (m model) appendTelegramPendingTurnCmd(text, origin string) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return telegramInboundMsg{text: text, isCommand: false, address: strings.TrimPrefix(origin, "telegram:")}
	})
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
	if text := strings.TrimSpace(output); text != "" {
		return text
	}
	return strings.TrimSpace(errText)
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
	return m.telegramOutboundCmd(telegramTurnReplyText(origin, output, errText, turnComplete))
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
