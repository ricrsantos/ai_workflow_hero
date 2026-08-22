package tui

import (
	"context"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	codexadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

type heroStartPrepareDoneMsg struct {
	requestID   uint64
	slug        string
	commandBody string
	agentBody   string
	err         string
}

func (m model) beginHeroStartPrepare(requestID uint64, slug, commandBody, agentBody string) (model, tea.Cmd) {
	m.heroStartPreparing = true
	m.waitAnimFrame = 0
	ctx, cancel := context.WithCancel(context.Background())
	m.heroStartCancel = cancel
	return m, tea.Batch(convWaitTickCmd(), m.heroStartPrepareCmd(ctx, requestID, slug, commandBody, agentBody))
}

func (m model) heroStartPrepareCmd(ctx context.Context, requestID uint64, slug, commandBody, agentBody string) tea.Cmd {
	projectDir := ""
	var st *store.Store
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
		st = m.svc.Store
	}
	return func() tea.Msg {
		// OpenCode first (existing path), then Codex — each no-ops when unused.
		if err := opencodeadapter.PrepareHeroStart(ctx, projectDir, st); err != nil {
			slog.Error("hero-start prepare failed", "harness", "opencode", "error", err)
			return heroStartPrepareDoneMsg{requestID: requestID, slug: slug, commandBody: commandBody, agentBody: agentBody, err: err.Error()}
		}
		if err := codexadapter.PrepareHeroStart(ctx, projectDir, st); err != nil {
			slog.Error("hero-start prepare failed", "harness", "codex", "error", err)
			return heroStartPrepareDoneMsg{requestID: requestID, slug: slug, commandBody: commandBody, agentBody: agentBody, err: err.Error()}
		}
		return heroStartPrepareDoneMsg{requestID: requestID, slug: slug, commandBody: commandBody, agentBody: agentBody}
	}
}

func (m model) handleHeroStartPrepareDone(msg heroStartPrepareDoneMsg) (model, tea.Cmd) {
	if !m.heroStartPreparing || msg.requestID != m.heroStartRequestID {
		return m, nil
	}
	m.heroStartPreparing = false
	m.heroStartCancel = nil
	if msg.err != "" {
		m.actionBusy = false
		m = m.setStatusResult(false, "/hero-start", msg.err)
		m.convError = msg.err
		m.chatInputFocused = true
		return m, nil
	}
	return m.beginHeroRuntimeConversation("start", msg.slug, heroRuntimeOpts{
		preloadedCommandBody: msg.commandBody,
		preloadedAgentBody:   msg.agentBody,
		preloadedPrompt:      true,
	})
}

func (m model) cancelHeroStartPreparation() (model, tea.Cmd) {
	if m.heroStartCancel != nil {
		m.heroStartCancel()
	}
	m.heroStartCancel = nil
	m.heroStartBootstrapping = false
	m.heroStartPreparing = false
	m.actionBusy = false
	m.chatInputFocused = true
	m = m.setStatusResult(false, "/hero-start", "cancelled")
	return m, nil
}
