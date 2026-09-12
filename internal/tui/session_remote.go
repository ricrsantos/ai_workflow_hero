package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
)

type historyImportMsg struct {
	sessionID string
	imported  int
	err       error
}

func bindRemoteHistoryReader(svc *conversation.SessionService, registry harnessmgr.Registry, harnessID string) {
	if svc == nil {
		return
	}
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if registry == nil || harnessID == "" {
		svc.Remote = nil
		return
	}
	adapter, err := registry.Adapter(harnessID)
	if err != nil || adapter == nil {
		svc.Remote = nil
		return
	}
	reader, ok := adapter.(harness.RemoteHistoryReader)
	if !ok || !reader.SupportsRemoteHistory() {
		svc.Remote = nil
		return
	}
	svc.Remote = reader
}

func (m model) attachRemoteHistoryReader(harnessID string) model {
	if m.sessionService == nil {
		return m
	}
	var registry harnessmgr.Registry
	if m.svc != nil {
		registry = m.svc.Registry
	}
	bindRemoteHistoryReader(m.sessionService, registry, harnessID)
	return m
}

func sessionHasLocalEvents(svc *conversation.SessionService, sessionID string) bool {
	if svc == nil || svc.Store == nil {
		return false
	}
	events, err := svc.Store.ListSessionEventsNewest(sessionID, 0, 1)
	return err == nil && len(events) > 0
}

func (m model) finishHistoryOpen(sessionID string, restored bool) (model, tea.Cmd) {
	m.history.detailBusy = false
	m.history.narrowDetail = false
	if restored {
		m = m.setStatusResult(true, "history", "✓ Session restored.")
	}
	return m.openHeroSessionFromHistory(sessionID)
}

func (m model) historyImportCmd(sessionID string) tea.Cmd {
	svc := m.sessionService
	return func() tea.Msg {
		if svc == nil {
			return historyImportMsg{sessionID: sessionID, err: fmt.Errorf("sessions unavailable")}
		}
		ctx := context.Background()
		n, err := svc.ImportRemoteHistory(ctx, sessionID, true)
		if err != nil {
			slog.Error("history remote import failed", "error", redact.Error(err))
			return historyImportMsg{sessionID: sessionID, err: err}
		}
		slog.Info("history remote import completed", "imported", n)
		return historyImportMsg{sessionID: sessionID, imported: n}
	}
}
