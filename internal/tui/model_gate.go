package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) hasDefaultModel() bool {
	return m.defaultHarnessModelSlug() != ""
}

// ensureDefaultModel blocks harness commands until the user picks /hero-model once.
func (m model) ensureDefaultModel(actionLabel string) (model, tea.Cmd, bool) {
	if m.hasDefaultModel() {
		return m, nil, true
	}
	m = m.setStatusResult(false, actionLabel, defaultModelRequiredMessage(actionLabel))
	return m, nil, false
}

// defaultExecuteModel requires /hero-model and returns that slug for TUI Execute.
func (m model) defaultExecuteModel(actionLabel string) (model, tea.Cmd, string, bool) {
	m, cmd, ok := m.ensureDefaultModel(actionLabel)
	if !ok {
		return m, cmd, "", false
	}
	return m, nil, m.defaultHarnessModelSlug(), true
}

// orchestratorExecuteModel resolves agents.orchestration_agent (then fallback_model,
// then /hero-model) for TUI Runtime Execute of orchestrator commands.
func (m model) orchestratorExecuteModel(actionLabel string) (model, tea.Cmd, string, bool) {
	m = m.applyAgentRuntimePair(agentOrchestration, "")
	if slug := strings.TrimSpace(m.runtimeModelSlug); slug != "" {
		return m, nil, slug, true
	}
	return m.defaultExecuteModel(actionLabel)
}

func defaultModelRequiredMessage(actionLabel string) string {
	switch actionLabel {
	case "/hero-new":
		return "Select a default model with /hero-model first, then run /hero-new again."
	case "chat":
		return "Select a default model with /hero-model first, then send your message again."
	default:
		if strings.HasPrefix(actionLabel, "/hero-") {
			return "Select a default model with /hero-model first, then run " + actionLabel + " again."
		}
		return "Select a default model with /hero-model before continuing."
	}
}
