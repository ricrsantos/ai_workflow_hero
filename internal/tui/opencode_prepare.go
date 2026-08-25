package tui

import (
	"context"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	codexadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
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
	var reg harnessmgr.Registry
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
		st = m.svc.Store
		reg = m.svc.Registry
	}
	return func() tea.Msg {
		// OpenCode first (existing path), then Codex — each no-ops when unused.
		if err := prepareOpenCodeOnStart(ctx, projectDir, st, reg); err != nil {
			slog.Error("hero-start prepare failed", "harness", "opencode", "error", err)
			return heroStartPrepareDoneMsg{requestID: requestID, slug: slug, commandBody: commandBody, agentBody: agentBody, err: err.Error()}
		}
		if err := prepareCodexOnStart(ctx, projectDir, st, reg); err != nil {
			slog.Error("hero-start prepare failed", "harness", "codex", "error", err)
			return heroStartPrepareDoneMsg{requestID: requestID, slug: slug, commandBody: commandBody, agentBody: agentBody, err: err.Error()}
		}
		return heroStartPrepareDoneMsg{requestID: requestID, slug: slug, commandBody: commandBody, agentBody: agentBody}
	}
}

// prepareOpenCodeOnStart resets the registry's OpenCode adapter when present so
// /hero-start does not leave a stale serve URL on the Chat singleton.
func prepareOpenCodeOnStart(ctx context.Context, projectDir string, st *store.Store, reg harnessmgr.Registry) error {
	if reg != nil {
		a, err := reg.Adapter("opencode")
		if err == nil {
			if oa, ok := a.(*opencodeadapter.Adapter); ok {
				return opencodeadapter.PrepareHeroStartWithAdapter(ctx, projectDir, st, oa)
			}
			// Test/mock adapters have nothing to reset.
			return nil
		}
	}
	return opencodeadapter.PrepareHeroStart(ctx, projectDir, st)
}

// prepareCodexOnStart resets the registry's Codex adapter when present so
// /hero-start does not leave a stale app-server RPC on the Chat singleton.
func prepareCodexOnStart(ctx context.Context, projectDir string, st *store.Store, reg harnessmgr.Registry) error {
	if reg != nil {
		a, err := reg.Adapter("codex")
		if err == nil {
			if ca, ok := a.(*codexadapter.Adapter); ok {
				return codexadapter.PrepareHeroStartWithAdapter(ctx, projectDir, st, ca)
			}
			// Test/mock adapters have nothing to reset.
			return nil
		}
	}
	return codexadapter.PrepareHeroStart(ctx, projectDir, st)
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
