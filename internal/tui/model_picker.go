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
		m = m.setStatusResult(false, "/model", "Enable a harness with /harness first.")
		return m, nil
	}
	// Apply any cache/catalog result completed since the previous opening before
	// starting the next refresh.  An open selector is never mutated underneath
	// the user's cursor; this happens only at the explicit opening boundary.
	if m.propsSvc != nil && m.svc != nil {
		m.propsSvc.Store = m.svc.Store
		m.propsSvc.Registry = m.svc.Registry
	}
	m = m.loadFreechatProps()
	m = m.clearPropsPendingSelect()
	// C5: opening /hero-model starts the background refresh for every enabled
	// harness (PRD-C05-001 §4.2.5). It never blocks the picker and never runs
	// at TUI boot, so OpenCode stays lazy until this explicit user action.
	var refresh tea.Cmd
	if m.propsSvc != nil && !m.propsRefreshBusy {
		m.propsRefreshBusy = true
		refresh = m.startModelPropsRefresh(enabled)
	}
	if len(enabled) == 1 {
		m, cmd := m.beginModelPickerForHarness(enabled[0])
		if refresh != nil && cmd != nil {
			return m, tea.Batch(refresh, cmd)
		}
		if refresh != nil {
			return m, refresh
		}
		return m, cmd
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
	return m, refresh
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
		m = m.setStatusResult(false, "/model", "harness required")
		return m, nil
	}
	if !m.harnessEnabled(harnessID) {
		name := harnessDisplayName(harnessID)
		m = m.setStatusResult(false, "/model",
			fmt.Sprintf("%s is not enabled — use /harness first.", name))
		return m, nil
	}
	cached := m.modelsForHarness(harnessID)
	if len(cached) > 0 {
		return m.showModelList(harnessID, cached), nil
	}
	m = m.setStatusRunning("/model")
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
		m = m.setStatusResult(false, "/model",
			fmt.Sprintf("Could not list %s models: %s", harnessDisplayName(msg.harnessID), msg.err.Error()))
		return m, nil
	}
	if len(msg.models) == 0 {
		m.actionBusy = false
		m = m.setStatusResult(false, "/model",
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
	// A completed explicit refresh is authoritative on the next opening.  Read
	// only the persisted list here so an in-memory boot list does not mask it.
	if m.propsSvc != nil {
		if cached := m.propsSvc.CachedModels(harnessID); len(cached) > 0 {
			return cached
		}
	}
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
	// C5 local-first fallback: a persisted project model-list cache wins over
	// the embedded/installed catalog, and neither path starts a harness.
	if m.propsSvc != nil {
		if cached := m.propsSvc.Models(harnessID); len(cached) > 0 {
			return cached
		}
	}
	// Test helper path: availableModels without per-harness options.
	if len(m.modelOptions) == 0 {
		return append([]string(nil), m.availableModels...)
	}
	return nil
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
