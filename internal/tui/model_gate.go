package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) hasDefaultModel() bool {
	return m.conversationModelSlug() != ""
}

// ensureDefaultModel blocks harness commands until the user picks /hero-model once.
func (m model) ensureDefaultModel(actionLabel string) (model, tea.Cmd, bool) {
	if m.hasDefaultModel() {
		return m, nil, true
	}
	m = m.setStatusResult(false, actionLabel, defaultModelRequiredMessage(actionLabel))
	return m, nil, false
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
