package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func (m model) openModelPicker() (model, tea.Cmd) {
	if len(m.availableModels) == 0 {
		m = m.setStatusResult(false, "/hero-model",
			"No models available — run `agent models` or authenticate with `cursor agent login`")
		return m, nil
	}
	m.prevScreen = m.screen
	m.screen = screenPalette
	m.pickingModel = true
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	items := make([]paletteItem, 0, len(m.availableModels))
	for _, slug := range m.availableModels {
		hint := ""
		if slug == m.chatModelSlug {
			hint = "current"
		}
		items = append(items, paletteItem{
			label:  slug,
			hint:   hint,
			action: actionSelectModel,
		})
	}
	m.paletteItems = items
	// Prefer current model selection in the list.
	for i, item := range items {
		if item.label == m.chatModelSlug {
			m.paletteIndex = i
			break
		}
	}
	m = m.ensurePaletteVisible()
	return m, nil
}

func (m model) selectChatModel(slug string) (model, tea.Cmd) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-model", "empty model slug")
		return m, nil
	}
	projectDir := ""
	tool := m.conversationHarnessTool()
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	if projectDir != "" {
		if err := install.SaveHarnessModel(projectDir, tool, slug); err != nil {
			m = m.closePalette()
			m = m.setStatusResult(false, "/hero-model", err.Error())
			return m, nil
		}
	}
	m.chatModelSlug = slug
	m = m.closePalette()
	m = m.setStatusResult(true, "/hero-model", fmt.Sprintf("Model set to %s", slug))
	return m, nil
}
