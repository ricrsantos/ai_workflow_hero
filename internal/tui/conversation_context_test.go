package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdatePathsDoNotBlockOnSlowConversationContextIO(t *testing.T) {
	const delay = 200 * time.Millisecond
	const updateBudget = 40 * time.Millisecond

	svc := newTestService(t)
	m := withDefaultChatModel(NewTestModel(svc))
	m.testContextIODelay = delay

	assertUpdateFast := func(t *testing.T, name string, run func(model) (model, tea.Cmd), slowCmd func(model) tea.Msg) {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			next, cmd := run(m)
			elapsed := time.Since(start)
			if elapsed > updateBudget {
				t.Fatalf("%s: Update blocked for %v", name, elapsed)
			}
			if cmd == nil {
				t.Fatalf("%s: expected async tea.Cmd", name)
			}
			_ = next
			if slowCmd == nil {
				return
			}
			cmdStart := time.Now()
			msg := slowCmd(m)
			if msg == nil {
				t.Fatalf("%s: slow cmd returned nil message", name)
			}
			if time.Since(cmdStart) < delay/2 {
				t.Fatalf("%s: slow cmd returned before injected IO delay (%v)", name, time.Since(cmdStart))
			}
		})
	}

	slowReset := func(m model) tea.Msg {
		_, cmd := m.resetChatSession()
		return cmd()
	}
	slowSync := func(m model) tea.Msg {
		m, seq := m.nextConversationContextSync()
		return m.syncConversationContextCmd(seq, syncContextFollowUp)()
	}
	slowHeroNew := func(m model) tea.Msg {
		return m.heroNewPrepareCmd()()
	}

	assertUpdateFast(t, "enterConversation", func(m model) (model, tea.Cmd) {
		return m.enterConversation()
	}, func(m model) tea.Msg {
		m, seq := m.nextConversationContextSync()
		return m.syncConversationContextCmd(seq, syncContextEnter)()
	})

	assertUpdateFast(t, "resetChatSession", func(m model) (model, tea.Cmd) {
		return m.resetChatSession()
	}, slowReset)

	assertUpdateFast(t, "beginNewChat", func(m model) (model, tea.Cmd) {
		return m.beginNewChat()
	}, slowReset)

	assertUpdateFast(t, "openHeroSessionFromHistory", func(m model) (model, tea.Cmd) {
		return m.openHeroSessionFromHistory("sess-test")
	}, slowReset)

	assertUpdateFast(t, "submitChatFollowUp", func(m model) (model, tea.Cmd) {
		m.screen = screenConversation
		m.chatInputFocused = true
		return m.submitChatFollowUp("hello")
	}, slowSync)

	assertUpdateFast(t, "beginHeroNew", func(m model) (model, tea.Cmd) {
		return m.beginHeroNew()
	}, slowHeroNew)
}
