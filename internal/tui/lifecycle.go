package tui

import (
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
)

// lifecycleEventMsg is delivered by the private relay owned by this TUI.
type lifecycleEventMsg struct {
	event conversation.Event
}

// relayLifecycleEvents pushes CLI-as-API child notifications into Bubble Tea.
// The relay goroutine never mutates model state; all routing and de-duplication
// stays in Update.
func relayLifecycleEvents(p *tea.Program, events <-chan conversation.Event) {
	if p == nil || events == nil {
		return
	}
	go func() {
		for event := range events {
			p.Send(lifecycleEventMsg{event: event})
		}
	}()
}

func (m model) handleLifecycleEvent(event conversation.Event) (model, tea.Cmd) {
	if m.telegram == nil {
		return m, nil
	}
	if event.EventID > 0 {
		if m.lifecycleEventIDs == nil {
			m.lifecycleEventIDs = make(map[int64]struct{})
		}
		if _, seen := m.lifecycleEventIDs[event.EventID]; seen {
			return m, nil
		}
		m.lifecycleEventIDs[event.EventID] = struct{}{}
	}
	text := formatTelegramEvent(event)
	if text == "" {
		return m, nil
	}
	if !m.telegram.connected || !m.telegram.paired {
		m.pendingLifecycleEvents = append(m.pendingLifecycleEvents, event)
		slog.Debug("queued lifecycle event until Telegram is ready", "kind", event.Kind, "event_id", event.EventID)
		return m, nil
	}
	return m, m.telegramOutboundCmd(text)
}

func (m model) flushPendingLifecycleEvents() (model, tea.Cmd) {
	if m.telegram == nil || !m.telegram.connected || !m.telegram.paired || len(m.pendingLifecycleEvents) == 0 {
		return m, nil
	}
	events := m.pendingLifecycleEvents
	m.pendingLifecycleEvents = nil
	cmds := make([]tea.Cmd, 0, len(events))
	for _, event := range events {
		if text := formatTelegramEvent(event); text != "" {
			cmds = append(cmds, m.telegramOutboundCmd(text))
		}
	}
	return m, combineTimerCmds(cmds...)
}

func (m model) flushPendingTelegramNotifications() (model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m, cmd = m.flushPendingLifecycleEvents()
	cmds = append(cmds, cmd)
	m, cmd = m.flushPendingHarnessPermissionNotices()
	cmds = append(cmds, cmd)
	return m, combineTimerCmds(cmds...)
}
