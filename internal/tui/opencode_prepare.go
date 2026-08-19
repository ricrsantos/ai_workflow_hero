package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

type heroStartPrepareDoneMsg struct {
	slug string
	err  string
}

func (m model) beginHeroStartPrepare(slug string) (model, tea.Cmd) {
	m.heroStartPreparing = true
	m.waitAnimFrame = 0
	return m, tea.Batch(convWaitTickCmd(), m.heroStartPrepareCmd(slug))
}

func (m model) heroStartPrepareCmd(slug string) tea.Cmd {
	projectDir := ""
	var st *store.Store
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
		st = m.svc.Store
	}
	return func() tea.Msg {
		err := opencodeadapter.PrepareHeroStart(context.Background(), projectDir, st)
		if err != nil {
			return heroStartPrepareDoneMsg{slug: slug, err: err.Error()}
		}
		return heroStartPrepareDoneMsg{slug: slug}
	}
}

func (m model) handleHeroStartPrepareDone(msg heroStartPrepareDoneMsg) (model, tea.Cmd) {
	m.heroStartPreparing = false
	if msg.err != "" {
		m = m.setStatusResult(false, "/hero-start", msg.err)
		return m, nil
	}
	return m.beginHeroRuntimeConversation("start", msg.slug, heroRuntimeOpts{})
}
