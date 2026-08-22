package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

// syncDisplayModelFromDisk reloads freechat defaults from hero.json and, when a
// workflow/runtime agent is active, the agent pair from workflow-config.yml.
// It updates only in-memory Chat labels — it does not Execute the harness.
func (m model) syncDisplayModelFromDisk() (model, error) {
	if m.svc == nil {
		return m, fmt.Errorf("project unavailable")
	}
	projectDir := m.svc.ProjectDir
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return m, fmt.Errorf("read hero.json: %w", err)
	}
	h, slug := install.GetFreechatDefault(hero)
	if strings.TrimSpace(h) != "" {
		m.chatHarnessID = strings.TrimSpace(strings.ToLower(h))
	}
	if strings.TrimSpace(slug) != "" {
		m.chatModelSlug = strings.TrimSpace(slug)
	}
	m = m.loadFreechatProps()

	agent := strings.TrimSpace(m.runtimeAgentName)
	switch {
	case m.researchLive:
		agent = agentDiscover
	case m.orchestrationLive && agent == "":
		agent = agentOrchestration
	case m.workflowAgentActive():
		// keep runtimeAgentName
	default:
		agent = ""
	}
	if agent != "" {
		m = m.withRuntimeAgent(agent)
		m = m.applyAgentRuntimePair(agent, "")
	}
	return m, nil
}

// beginHeroConfigUpdate handles /hero-config-update: reload config into TUI labels.
func (m model) beginHeroConfigUpdate() (model, tea.Cmd) {
	m, _ = m.enterConversation()
	next, err := m.syncDisplayModelFromDisk()
	if err != nil {
		m = m.setStatusResult(false, "/hero-config-update", err.Error())
		return m, nil
	}
	msg := fmt.Sprintf("config reloaded · %s · %s",
		next.conversationModelLabel(), next.conversationHarnessTool())
	next = next.setStatusResult(true, "/hero-config-update", msg)
	return next, nil
}
