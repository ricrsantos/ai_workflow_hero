package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

type listModelsMsg struct {
	harnessID string
	models    []string
	err       error
}

func (m model) openModelPicker() (model, tea.Cmd) {
	enabled := m.enabledHarnessIDs()
	if len(enabled) == 0 {
		m = m.setStatusResult(false, "/hero-model", "Enable a harness with /hero-harness first.")
		return m, nil
	}
	if len(enabled) == 1 {
		return m.beginModelPickerForHarness(enabled[0])
	}

	m = m.openPaletteOverlay()
	m.pickingModel = true
	m.pickingHarness = false
	m.modelPickerHarness = ""
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0

	items := make([]paletteItem, 0, len(enabled))
	for _, id := range enabled {
		items = append(items, paletteItem{
			label:     harnessDisplayName(id),
			action:    actionPickModelHarness,
			harnessID: id,
		})
	}
	m.paletteItems = items
	if cur := strings.TrimSpace(m.chatHarnessID); cur != "" {
		for i, item := range items {
			if item.harnessID == cur {
				m.paletteIndex = i
				break
			}
		}
	}
	m = m.ensurePaletteVisible()
	return m, nil
}

func (m model) beginModelPickerForHarness(harnessID string) (model, tea.Cmd) {
	m = m.openPaletteOverlay()
	m.pickingModel = true
	m.pickingHarness = false
	m.modelPickerHarness = ""
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	return m.pickModelHarness(harnessID)
}

func (m model) enabledHarnessIDs() []string {
	if m.svc == nil {
		return nil
	}
	hero, err := install.LoadHeroJSON(m.svc.ProjectDir)
	if err != nil {
		return nil
	}
	return install.ListEnabledHarnesses(hero)
}

func (m model) pickModelHarness(harnessID string) (model, tea.Cmd) {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if harnessID == "" {
		m = m.setStatusResult(false, "/hero-model", "harness required")
		return m, nil
	}
	if !m.harnessEnabled(harnessID) {
		name := harnessDisplayName(harnessID)
		m = m.setStatusResult(false, "/hero-model",
			fmt.Sprintf("%s is not enabled — use /hero-harness first.", name))
		return m, nil
	}
	cached := m.modelsForHarness(harnessID)
	if len(cached) > 0 {
		return m.showModelList(harnessID, cached), nil
	}
	m = m.setStatusRunning("/hero-model")
	return m, m.listModelsForHarnessCmd(harnessID)
}

func (m model) showModelList(harnessID string, models []string) model {
	m.pickingModel = true
	m.pickingHarness = false
	m.modelPickerHarness = harnessID
	m.screen = screenPalette
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	items := make([]paletteItem, 0, len(models))
	for _, slug := range models {
		slug = strings.TrimSpace(slug)
		if slug == "" {
			continue
		}
		hint := ""
		if slug == m.chatModelSlug && harnessID == m.chatHarnessID {
			hint = "current"
		}
		items = append(items, paletteItem{
			label:     slug,
			hint:      hint,
			action:    actionSelectModel,
			modelSlug: slug,
			harnessID: harnessID,
		})
	}
	m.paletteItems = items
	for i, item := range items {
		if item.modelSlug == m.chatModelSlug && item.harnessID == m.chatHarnessID {
			m.paletteIndex = i
			break
		}
	}
	return m.ensurePaletteVisible()
}

func (m model) handleListModelsMsg(msg listModelsMsg) (model, tea.Cmd) {
	if !m.pickingModel {
		return m, nil
	}
	if msg.err != nil {
		m.actionBusy = false
		m = m.setStatusResult(false, "/hero-model",
			fmt.Sprintf("Could not list %s models: %s", harnessDisplayName(msg.harnessID), msg.err.Error()))
		return m, nil
	}
	if len(msg.models) == 0 {
		m.actionBusy = false
		m = m.setStatusResult(false, "/hero-model",
			fmt.Sprintf("No models available for %s", harnessDisplayName(msg.harnessID)))
		return m, nil
	}
	for _, slug := range msg.models {
		m.modelOptions = append(m.modelOptions, harnessmgr.ModelOption{Model: slug, Harness: msg.harnessID})
	}
	m.availableModels = flattenModelOptions(m.modelOptions)
	m.actionBusy = false
	m.statusKind = statusIdle
	m.statusText = ""
	return m.showModelList(msg.harnessID, msg.models), nil
}

func (m model) listModelsForHarnessCmd(harnessID string) tea.Cmd {
	return func() tea.Msg {
		models, err := listModelsForHarnessFn(context.Background(), m, harnessID)
		return listModelsMsg{harnessID: harnessID, models: models, err: err}
	}
}

func (m model) modelsForHarness(harnessID string) []string {
	var out []string
	seen := map[string]bool{}
	for _, opt := range m.modelOptions {
		if opt.Harness != harnessID {
			continue
		}
		slug := strings.TrimSpace(opt.Model)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, slug)
	}
	if len(out) > 0 {
		return out
	}
	// Test helper path: availableModels without per-harness options.
	if len(m.modelOptions) == 0 {
		return append([]string(nil), m.availableModels...)
	}
	return nil
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
	harnessID := m.modelPickerHarness
	if harnessID == "" {
		harnessID = m.conversationHarnessTool()
	}
	return m.selectChatModelPair(slug, harnessID)
}

func splitModelPairLabel(label string) (modelSlug, harnessID string) {
	parts := strings.Split(label, " · ")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(label), ""
}

var listModelsForHarnessFn = defaultListModelsForHarness

func defaultListModelsForHarness(ctx context.Context, m model, harnessID string) ([]string, error) {
	if m.svc == nil || m.svc.Registry == nil {
		return nil, fmt.Errorf("harness registry unavailable")
	}
	return harnessmgr.ListModelsFor(ctx, m.svc.Registry, harnessID)
}
