package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func (m model) openModelPicker() (model, tea.Cmd) {
	opts := append([]harnessmgr.ModelOption(nil), m.modelOptions...)
	if len(opts) == 0 {
		for _, slug := range m.availableModels {
			opts = append(opts, harnessmgr.ModelOption{Model: slug, Harness: m.conversationHarnessTool()})
		}
	}
	if len(opts) == 0 {
		m = m.setStatusResult(false, "/hero-model",
			"No models available — enable a harness or authenticate with the harness CLI")
		return m, nil
	}
	m.prevScreen = m.screen
	m.screen = screenPalette
	m.pickingModel = true
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	items := make([]paletteItem, 0, len(opts))
	for _, opt := range opts {
		hint := harnessDisplayName(opt.Harness)
		if opt.Model == m.chatModelSlug && opt.Harness == m.chatHarnessID {
			hint = harnessDisplayName(opt.Harness) + " · current"
		}
		items = append(items, paletteItem{
			label:     opt.Model,
			hint:      hint,
			action:    actionSelectModel,
			modelSlug: opt.Model,
			harnessID: opt.Harness,
		})
	}
	m.paletteItems = items
	for i, item := range items {
		if item.modelSlug == m.chatModelSlug && item.harnessID == m.chatHarnessID {
			m.paletteIndex = i
			break
		}
	}
	m = m.ensurePaletteVisible()
	return m, nil
}

func (m model) selectChatModelPair(modelSlug, harnessID string) (model, tea.Cmd) {
	modelSlug = strings.TrimSpace(modelSlug)
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if modelSlug == "" || harnessID == "" {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-model", "model and harness required")
		return m, nil
	}
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	if projectDir != "" {
		if err := install.SetFreechatDefault(projectDir, harnessID, modelSlug); err != nil {
			m = m.closePalette()
			m = m.setStatusResult(false, "/hero-model", err.Error())
			return m, nil
		}
	}
	m.chatModelSlug = modelSlug
	m.chatHarnessID = harnessID
	m = m.closePalette()
	m = m.setStatusResult(true, "/hero-model", fmt.Sprintf("Model set to %s · %s", modelSlug, harnessID))
	return m, nil
}

func (m model) selectChatModel(slug string) (model, tea.Cmd) {
	return m.selectChatModelPair(slug, m.conversationHarnessTool())
}

func splitModelPairLabel(label string) (modelSlug, harnessID string) {
	parts := strings.Split(label, " · ")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(label), ""
}
