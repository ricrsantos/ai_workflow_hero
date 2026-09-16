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
	if event.EventID > 0 {
		if m.lifecycleEventIDs == nil {
			m.lifecycleEventIDs = make(map[int64]struct{})
		}
		if _, seen := m.lifecycleEventIDs[event.EventID]; seen {
			return m, nil
		}
		m.lifecycleEventIDs[event.EventID] = struct{}{}
	}
	if !m.lifecycleCycleKnown(event.CycleID) {
		slog.Debug("ignored lifecycle event for unknown cycle",
			"kind", event.Kind, "cycle_id", event.CycleID, "event_id", event.EventID)
		return m, nil
	}
	var progressCmd tea.Cmd
	switch event.Kind {
	case conversation.EventStageStarted, conversation.EventApprovalRequired:
		if len(m.executes) == 0 && !m.streaming {
			m, progressCmd = m.ensureStageProgress()
		}
	}
	if m.telegram == nil {
		return m, progressCmd
	}
	text := formatTelegramEvent(event)
	if text == "" {
		return m, progressCmd
	}
	if !m.telegram.connected || !m.telegram.paired {
		m.pendingLifecycleEvents = append(m.pendingLifecycleEvents, event)
		slog.Debug("queued lifecycle event until Telegram is ready", "kind", event.Kind, "event_id", event.EventID)
		return m, progressCmd
	}
	return m, combineTimerCmds(progressCmd, m.telegramOutboundCmd(text))
}

// lifecycleCycleKnown guards against lifecycle events that reference a cycle
// this TUI does not own. The private relay is project-scoped, but a stale or
// replayed event must not drive ensureStageProgress or Telegram delivery.
// Events without a cycle id (direct in-process calls, tests) are allowed.
func (m model) lifecycleCycleKnown(cycleID int64) bool {
	if cycleID <= 0 || m.svc == nil || m.svc.Store == nil {
		return true
	}
	_, err := m.svc.Store.GetCycle(cycleID)
	return err == nil
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
