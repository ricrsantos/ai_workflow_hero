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

// handleTelegramMsg processes a daemon-pushed frame and keeps the listener
// alive by re-issuing waitTelegramMsg. It never blocks the Update loop: all
// connection work happened in the client goroutine (telegram-ipc R3).
func (m model) handleTelegramMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.telegram == nil {
		return m, nil
	}
	next := waitTelegramMsg(m.telegramMsgCh)

	switch msg := msg.(type) {
	case telegramConnectedMsg:
		m.telegram.connected = true
		m.telegram.retrying = false
		m.telegram.daemonErr = ""
		return m, next

	case telegramRegisteredMsg:
		m.telegram.address = msg.address
		m.telegram.paired = msg.paired
		slog.Info("telegram client registered", "address", msg.address, "paired", msg.paired)
		return m, next

	case telegramDisconnectedMsg:
		wasConnected := m.telegram.connected
		m.telegram.connected = false
		m.telegram.retrying = true
		m.telegram.daemonErr = msg.err
		if wasConnected || m.telegram.address != "" {
			m = m.appendTelegramNotice("⚠ Telegram daemon disconnected; retrying…")
		}
		slog.Debug("telegram client disconnected", "error", msg.err)
		return m, next

	case telegramEventMsg:
		return m.handleTelegramEvent(msg), next

	case telegramInboundMsg:
		nextM, cmd := m.handleTelegramInbound(msg)
		if cmd == nil {
			return nextM, next
		}
		return nextM, tea.Batch(cmd, next)
	}
	return m, next
}

func (m model) handleTelegramEvent(msg telegramEventMsg) model {
	switch msg.eventType {
	case ipc.EventPairingProgress:
		if msg.data == "missing-token" {
			m.telegram.pairState = "token"
			m.telegram.pairCode = ""
			return m
		}
		m.telegram.pairCode = msg.data
		m.telegram.pairState = "waiting"
		m.telegram.pairDeadline = time.Now().Add(10 * time.Minute)
		if m.screen == screenSettings {
			m = m.renderTelegramPairingModalOpen()
		}
	case ipc.EventPairingSuccess:
		m.telegram.pairing = false
		m.telegram.pairState = "success"
		m.telegram.paired = true
		if m.screen == screenSettings {
			m = m.appendTelegramNotice("✓ Telegram pairing succeeded.")
		}
	case ipc.EventPairingExpired:
		if m.telegram.pairing {
			m.telegram.pairing = false
			m.telegram.pairState = "expired"
			if m.screen == screenSettings {
				m = m.appendTelegramNotice("Pairing code expired.")
			}
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
	// Acknowledge queued deliveries first so the daemon can mark them processed.
	if msg.inboundID != "" && m.telegram != nil && m.telegram.client != nil {
		_ = m.telegram.client.Send(ipc.Message{Type: ipc.TypeAckDelivery, AckID: msg.inboundID})
	}

	// Classify through the shared service so the remote transport obeys the
	// same slash-vs-text rule as the composer (ADR-061).
	isCommand := msg.isCommand
	if m.convService != nil {
		isCommand = m.convService.Classify(msg.text).Kind == conversation.KindSlash
	}

	origin := "telegram:" + msg.address
	if isCommand {
		return m.submitRemoteCommand(msg.text, origin)
	}
	return m.submitRemoteTurn(msg.text, origin)
}

// submitRemoteCommand routes a Telegram-originated slash command through the
// exact Hero slash dispatcher used by the composer.
func (m model) submitRemoteCommand(text, origin string) (model, tea.Cmd) {
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
