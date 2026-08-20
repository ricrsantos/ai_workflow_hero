package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

// modelRefreshDoneMsg is delivered when the /hero-model background refresh
// finishes. It never moves open rows or replaces an active property draft.
type modelRefreshDoneMsg struct {
	summaries []modelprops.RefreshSummary
}

type pendingModelSelect struct {
	harnessID string
	modelSlug string
}

// initModelProps builds the C5 model-property service and the effective freechat
// property map. Only local sources (hero.json, project cache, catalog) are read —
// no harness API is touched at boot, so OpenCode stays lazy.
func (m model) initModelProps(projectDir string) model {
	m.propsSvc = modelprops.NewService(projectDir, nil, nil, assets.FS)
	if m.svc != nil {
		m.propsSvc.Store = m.svc.Store
		m.propsSvc.Registry = m.svc.Registry
	}
	if projectDir == "" {
		return m
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		slog.Debug("tui model props init hero read failed", "error", err)
		return m
	}
	h, model := install.GetFreechatDefault(hero)
	if strings.TrimSpace(model) == "" {
		// C4 projects may have only harnesses.<harness>.model populated;
		// boot already supplied that pair to the TUI, so still restore its
		// per-pair C5 properties without inventing a model.
		h = strings.TrimSpace(m.chatHarnessID)
		model = strings.TrimSpace(m.chatModelSlug)
	}
	if strings.TrimSpace(h) == "" || strings.TrimSpace(model) == "" {
		return m
	}
	snap := m.propsSvc.Snapshot(h, model)
	saved := install.EffectivePairProperties(hero, h, model)
	values, invalidated := modelprops.EffectiveValues(snap, saved)
	m.freechatSnapshot = snap
	m.freechatProps = values
	if len(invalidated) > 0 {
		m = m.setPropsWarning(modelprops.WarningInvalidated)
	} else if snap.Warning != "" {
		m = m.setPropsWarning(snap.Warning)
	}
	return m
}

// startModelPropsRefresh launches the background refresh for every enabled
// harness (invoked only when /hero-model opens; PRD-C05-001 §4.2.5).
func (m model) startModelPropsRefresh(enabled []string) tea.Cmd {
	svc := m.propsSvc
	if svc == nil {
		return nil
	}
	return func() tea.Msg {
		summaries := svc.Refresh(context.Background(), enabled)
		return modelRefreshDoneMsg{summaries: summaries}
	}
}

// loadFreechatProps refreshes the in-memory freechat property view from disk.
func (m model) loadFreechatProps() model {
	if m.svc == nil || m.propsSvc == nil {
		return m
	}
	hero, err := install.LoadHeroJSON(m.svc.ProjectDir)
	if err != nil {
		return m
	}
	h, model := install.GetFreechatDefault(hero)
	if strings.TrimSpace(model) == "" {
		h = strings.TrimSpace(m.chatHarnessID)
		model = strings.TrimSpace(m.chatModelSlug)
	}
	if strings.TrimSpace(h) == "" || strings.TrimSpace(model) == "" {
		return m
	}
	snap := m.propsSvc.Snapshot(h, model)
	saved := install.EffectivePairProperties(hero, h, model)
	values, invalidated := modelprops.EffectiveValues(snap, saved)
	m.freechatSnapshot = snap
	m.freechatProps = values
	if len(invalidated) > 0 {
		m = m.setPropsWarning(modelprops.WarningInvalidated)
	} else if snap.Warning != "" {
		m = m.setPropsWarning(snap.Warning)
	}
	return m
}

// effectiveDisplayProperties returns the property values shown on the Chat
// status line plus the validity flag per key (validated → green).
func (m model) effectiveDisplayProperties() (values map[string]string, validated map[string]bool) {
	values = make(map[string]string, len(harness.PropertyKeys()))
	validated = make(map[string]bool, len(harness.PropertyKeys()))
	for _, key := range harness.PropertyKeys() {
		values[key] = "na"
	}
	if m.workflowAgentActive() {
		for key, v := range m.workflowPropertyProjection() {
			if v == "" {
				continue
			}
			values[key] = v
			// Workflow YAML values are unvalidated for display (gray).
			validated[key] = false
		}
		return values, validated
	}
	if len(m.freechatProps) == 0 {
		return values, validated
	}
	for _, key := range harness.PropertyKeys() {
		v := m.freechatProps[key]
		if v == "" || v == "na" {
			continue
		}
		values[key] = v
		validated[key] = m.isValidatedFreechatValue(key, v)
	}
	return values, validated
}

// isValidatedFreechatValue reports whether a freechat value is accepted by the
// current capability snapshot (green) or unvalidated/unavailable (gray).
func (m model) isValidatedFreechatValue(key, value string) bool {
	if value == "" || value == "na" {
		return false
	}
	if m.workflowAgentActive() {
		return false
	}
	cap, ok := m.freechatSnapshot.Properties[key]
	if !ok || !cap.Available {
		return false
	}
	for _, accepted := range cap.AcceptedValues {
		if accepted == value {
			return true
		}
	}
	return false
}

// propertyPickerHeader is the C5 property screen header (UI-C05-001 §3).
func propertyPickerHeader(harnessID string) string {
	return "/hero-model · " + harnessDisplayName(harnessID) + " · properties"
}

// friendlyPropertyName maps normalized keys to friendly labels (PRD §4.4.1).
func friendlyPropertyName(key string) string {
	switch key {
	case harness.PropertyFast:
		return "Fast Mode"
	case harness.PropertyThink:
		return "Thinking"
	case harness.PropertyEffort:
		return "Reasoning effort"
	default:
		return key
	}
}

// propertyStatusLabelPrefix maps internal C5 keys to the compact Chat status-line labels.
func propertyStatusLabelPrefix(key string) string {
	switch key {
	case harness.PropertyFast:
		return "fast"
	case harness.PropertyThink:
		return "thinking"
	case harness.PropertyEffort:
		return "effort"
	default:
		return key
	}
}

// selectChatModelPair continues the C4 flow into the C5 property step: when at
// least one property is selectable the property picker opens; otherwise the pair
// is committed immediately through the same atomic save path.
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
	if projectDir == "" {
		// No project service (unit tests): keep the C4 in-memory behavior.
		m.chatModelSlug = modelSlug
		m.chatHarnessID = harnessID
		m = m.closePalette()
		m = m.setStatusResult(true, "/hero-model", fmt.Sprintf("Model set to %s · %s", modelSlug, harnessID))
		return m, nil
	}
	if m.propsSvc == nil {
		m.propsSvc = modelprops.NewService(projectDir, nil, nil, assets.FS)
		if m.svc != nil {
			m.propsSvc.Store = m.svc.Store
			m.propsSvc.Registry = m.svc.Registry
		}
	}

	if m.propsRefreshBusy {
		m.propsPendingSelect = &pendingModelSelect{harnessID: harnessID, modelSlug: modelSlug}
		m.propsAwaitingRefresh = true
		m.waitAnimFrame = 0
		return m, convWaitTickCmd()
	}
	return m.applyModelSelection(modelSlug, harnessID, false, nil)
}

func (m model) applyModelSelection(modelSlug, harnessID string, fromRefreshWait bool, summaries []modelprops.RefreshSummary) (model, tea.Cmd) {
	snap := m.propsSvc.Snapshot(harnessID, modelSlug)
	if fromRefreshWait {
		snap = m.propsSvc.SnapshotCacheOnly(harnessID, modelSlug)
		if !snap.HasSelectableProperty() && refreshFailedForHarness(summaries, harnessID) {
			fallback := m.propsSvc.Snapshot(harnessID, modelSlug)
			if fallback.HasSelectableProperty() {
				if fallback.Warning == "" {
					fallback.Warning = modelprops.WarningStaleCache
				}
				snap = fallback
			} else if snap.Warning == "" {
				snap.Warning = modelprops.WarningMissingCatalog
			}
		}
	}
	return m.finishModelSelection(modelSlug, harnessID, snap)
}

func refreshFailedForHarness(summaries []modelprops.RefreshSummary, harnessID string) bool {
	for _, summary := range summaries {
		if strings.EqualFold(strings.TrimSpace(summary.HarnessID), harnessID) && summary.Err != nil {
			return true
		}
	}
	return false
}

func (m model) finishModelSelection(modelSlug, harnessID string, snap modelprops.Snapshot) (model, tea.Cmd) {
	projectDir := m.svc.ProjectDir
	if !snap.HasSelectableProperty() {
		// Skip the submenu: save the pair immediately through the complete-save path.
		saved := map[string]string{}
		if hero, err := install.LoadHeroJSON(projectDir); err == nil {
			saved = install.EffectivePairProperties(hero, harnessID, modelSlug)
		}
		_, invalidated := modelprops.EffectiveValues(snap, saved)
		if err := install.CommitModelSelection(projectDir, harnessID, modelSlug, nil); err != nil {
			m = m.closePalette()
			m = m.setStatusResult(false, "/hero-model", err.Error())
			return m, nil
		}
		m.chatModelSlug = modelSlug
		m.chatHarnessID = harnessID
		m = m.loadFreechatProps()
		m = m.closePalette()
		if len(invalidated) > 0 {
			m = m.setPropsWarning(modelprops.WarningInvalidated)
		} else if snap.Warning != "" {
			m = m.setPropsWarning(snap.Warning)
		} else {
			m = m.setStatusResult(true, "/hero-model", fmt.Sprintf("Model set to %s · %s", modelSlug, harnessID))
		}
		return m, nil
	}

	// Open the property picker with a draft restored from saved choices.
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		hero = install.HeroJSON{}
	}
	saved := install.EffectivePairProperties(hero, harnessID, modelSlug)
	values, invalidated := modelprops.EffectiveValues(snap, saved)
	values = mergeLockedPropertyDraft(snap, values)
	m = m.openPaletteOverlay()
	m.pickingProps = true
	m.pickingModel = false
	m.pickingHarness = false
	m.propsDraftHarness = harnessID
	m.propsDraftModel = modelSlug
	m.propsSnapshot = snap
	m.propsDraft = values
	m.propsEdited = map[string]bool{}
	m.propsValueList = false
	m.propsValueKey = ""
	m.propsValueIndex = 0
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	m.paletteItems = nil
	if len(invalidated) > 0 {
		m = m.setPropsWarning(modelprops.WarningInvalidated)
	} else if snap.Warning != "" {
		m = m.setPropsWarning(snap.Warning)
	}
	slog.Info("tui model property picker opened", "harness", harnessID, "model", modelSlug)
	return m, nil
}

func mergeLockedPropertyDraft(snap modelprops.Snapshot, values map[string]string) map[string]string {
	if values == nil {
		values = map[string]string{}
	}
	for _, key := range harness.PropertyKeys() {
		cap, ok := snap.Properties[key]
		if !ok || cap.Available {
			continue
		}
		if v := strings.TrimSpace(cap.DefaultValue); v != "" {
			values[key] = v
		}
	}
	return values
}

func (m model) clearPropsPendingSelect() model {
	m.propsPendingSelect = nil
	m.propsAwaitingRefresh = false
	return m
}

// commitPropertyDraft atomically saves the pair and the complete property draft
// (Enter on the main property picker).
func (m model) commitPropertyDraft() (model, tea.Cmd) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	if projectDir == "" {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-model", "project unavailable")
		return m, nil
	}
	if err := install.CommitModelSelection(projectDir, m.propsDraftHarness, m.propsDraftModel, m.propsDraft); err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-model", err.Error())
		return m, nil
	}
	m.chatModelSlug = m.propsDraftModel
	m.chatHarnessID = m.propsDraftHarness
	m = m.loadFreechatProps()
	m = m.closePalette()
	m = m.setStatusResult(true, "/hero-model",
		fmt.Sprintf("Model set to %s · %s", m.propsDraftModel, m.propsDraftHarness))
	return m, nil
}

// cancelPropertyDraft discards the complete model/property selection (Escape).
func (m model) cancelPropertyDraft() (model, tea.Cmd) {
	// Leave pickingProps true until closePalette() runs so wasPicking triggers
	// reloadPaletteItems() and the Chat `/` overlay keeps working.
	m.propsValueList = false
	m.propsValueKey = ""
	m.propsDraft = nil
	m.propsEdited = nil
	m.propsDraftModel = ""
	m.propsDraftHarness = ""
	m = m.clearPropsPendingSelect()
	m = m.closePalette()
	m = m.setStatusResult(true, "/hero-model", "Selection cancelled — no changes saved.")
	return m, nil
}

// setPropsWarning shows a yellow C5 warning in the same status area used for
// execution errors (UI-C05-001 §5). Cleared by the next user action.
func (m model) setPropsWarning(text string) model {
	m.propsWarningText = strings.TrimSpace(text)
	m.statusKind = statusWarn
	if m.statusLabel == "" {
		m.statusLabel = "/hero-model"
	}
	m.statusText = strings.TrimSpace(text)
	m.actionBusy = false
	return m
}

// clearPropsWarning clears the pending C5 warning (called on the next user action).
func (m model) clearPropsWarning() model {
	if m.statusKind == statusWarn {
		m.statusKind = statusIdle
	}
	m.propsWarningText = ""
	return m
}

// workflowAgentActive reports whether a workflow/runtime command is driving the
// current execution. It mirrors the C4 runtimeAgentName contract: stage-agent
// paths set it explicitly, ordinary Chat and /hero-new leave it empty.
func (m model) workflowAgentActive() bool {
	return strings.TrimSpace(m.runtimeAgentName) != ""
}

// workflowPropertyProjection returns the active agent's YAML-derived property
// map, computing it on demand for C4 paths that set runtimeAgentName directly.
func (m model) workflowPropertyProjection() map[string]string {
	if len(m.workflowProps) > 0 {
		return m.workflowProps
	}
	if m.svc != nil && m.workflowAgentActive() {
		if props, _, err := workflowconfig.AgentProperties(m.svc.ProjectDir, m.runtimeAgentName); err == nil {
			return props
		}
	}
	return nil
}

// withRuntimeAgent sets the active runtime agent and projects its YAML property
// values for the C5 status line (gray/unvalidated; ADR-042).
func (m model) withRuntimeAgent(agentName string) model {
	m.runtimeAgentName = strings.TrimSpace(agentName)
	m.workflowProps = nil
	if m.workflowAgentActive() && m.svc != nil {
		if props, _, err := workflowconfig.AgentProperties(m.svc.ProjectDir, m.runtimeAgentName); err == nil {
			m.workflowProps = props
		} else {
			slog.Debug("tui workflow property projection failed", "agent", m.runtimeAgentName, "error", err)
		}
	}
	return m
}
