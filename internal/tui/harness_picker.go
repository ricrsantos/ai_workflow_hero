package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func (m model) openHarnessPicker() (model, tea.Cmd) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	if projectDir == "" {
		m = m.setStatusResult(false, slashHarness, "project unavailable")
		return m, nil
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		m = m.setStatusResult(false, slashHarness, err.Error())
		return m, nil
	}

	m = m.openPaletteOverlay()
	m.pickingHarness = true
	m.pickingModel = false
	m.pickingHarnessPermission = false
	m.pickingPermissionProfile = false
	m.modelPickerHarness = ""
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	m.harnessDraft = make(map[string]bool, len(install.SupportedHarnessIDs))

	items := make([]paletteItem, 0, len(install.SupportedHarnessIDs))
	for _, id := range install.SupportedHarnessIDs {
		enabled := install.IsHarnessEnabled(hero, id)
		m.harnessDraft[id] = enabled
		items = append(items, paletteItem{
			label:     harnessDisplayName(id),
			hint:      "(" + m.harnessAvailabilityLabel(id) + ")",
			action:    actionToggleHarness,
			harnessID: id,
		})
	}
	m.paletteItems = items
	m = m.ensurePaletteVisible()
	return m, nil
}

func (m model) toggleHarnessDraft() (model, tea.Cmd) {
	items := m.filteredPaletteItems()
	if len(items) == 0 || m.paletteIndex < 0 || m.paletteIndex >= len(items) {
		return m, nil
	}
	id := strings.TrimSpace(strings.ToLower(items[m.paletteIndex].harnessID))
	if id == "" {
		return m, nil
	}
	if m.harnessDraft == nil {
		m.harnessDraft = make(map[string]bool)
	}
	m.harnessDraft[id] = !m.harnessDraft[id]
	return m, nil
}

func (m model) applyHarnessDraft() (model, tea.Cmd) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	if projectDir == "" {
		m = m.closePalette()
		m = m.setStatusResult(false, slashHarness, "project unavailable")
		return m, nil
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, slashHarness, err.Error())
		return m, nil
	}

	enabledCount := 0
	for _, id := range install.SupportedHarnessIDs {
		if m.harnessDraft[id] {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		m = m.setStatusResult(false, slashHarness, "Select at least one harness.")
		return m, nil
	}

	var enabledNames, disabledNames []string
	for _, id := range install.SupportedHarnessIDs {
		want := m.harnessDraft[id]
		have := install.IsHarnessEnabled(hero, id)
		if want && !have {
			if m.freeChatMode {
				if err := install.SetHarnessEnabled(projectDir, id, true); err != nil {
					m = m.closePalette()
					m = m.setStatusResult(false, slashHarness, err.Error())
					return m, nil
				}
			} else if err := install.EnableHarnessWithProjection(projectDir, id, assets.FS); err != nil {
				m = m.closePalette()
				m = m.setStatusResult(false, slashHarness, err.Error())
				return m, nil
			}
			enabledNames = append(enabledNames, harnessDisplayName(id))
		}
	}
	hero, err = install.LoadHeroJSON(projectDir)
	if err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, slashHarness, err.Error())
		return m, nil
	}
	for _, id := range install.SupportedHarnessIDs {
		want := m.harnessDraft[id]
		have := install.IsHarnessEnabled(hero, id)
		if !want && have {
			if err := install.SetHarnessEnabled(projectDir, id, false); err != nil {
				msg := err.Error()
				if strings.Contains(msg, "last enabled harness") {
					name := harnessDisplayName(id)
					msg = fmt.Sprintf("Cannot disable the last enabled harness (%s).\n\n  Suggestion: enable another harness first, then disable %s.", name, name)
				}
				m = m.closePalette()
				m = m.setStatusResult(false, slashHarness, msg)
				return m, nil
			}
			if id == "opencode" {
				var st *store.Store
				var reg harnessmgr.Registry
				if m.svc != nil {
					st = m.svc.Store
					reg = m.svc.Registry
				}
				if err := stopOpenCodeServe(context.Background(), projectDir, st, reg); err != nil {
					slog.Warn("stop opencode serve on disable failed", "error", err)
				}
			}
			if id == "codex" {
				var st *store.Store
				var reg harnessmgr.Registry
				if m.svc != nil {
					st = m.svc.Store
					reg = m.svc.Registry
				}
				if err := stopCodexAppServer(context.Background(), projectDir, st, reg); err != nil {
					slog.Warn("stop codex app-server on disable failed", "error", err)
				}
			}
			disabledNames = append(disabledNames, harnessDisplayName(id))
		}
	}

	if len(enabledNames) == 0 && len(disabledNames) == 0 {
		m = m.setStatusResult(true, slashHarness, "Harness selection unchanged")
	} else {
		var parts []string
		for _, name := range enabledNames {
			msg := name + " enabled"
			if !m.freeChatMode {
				switch name {
				case "OpenCode":
					msg = "OpenCode enabled (projected .opencode/)"
				case "Codex":
					msg = "Codex enabled (projected .codex/)"
				}
			}
			parts = append(parts, msg)
		}
		for _, name := range disabledNames {
			parts = append(parts, name+" disabled (files kept)")
		}
		m = m.setStatusResult(true, slashHarness, strings.Join(parts, "; "))
	}
	return m.openHarnessPermissionPicker()
}

// openHarnessPermissionPicker lists every enabled harness after a topology
// change, so newly enabled harnesses must pass through the same explicit
// project-local approval choice as existing ones.
func (m model) openHarnessPermissionPicker() (model, tea.Cmd) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, slashHarness, err.Error())
		return m, nil
	}
	m = m.openPaletteOverlay()
	m.pickingHarness = false
	m.pickingHarnessPermission = true
	m.pickingPermissionProfile = false
	m.permissionPickerHarness = ""
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	m.paletteItems = nil
	for _, id := range install.SupportedHarnessIDs {
		if !install.IsHarnessEnabled(hero, id) {
			continue
		}
		m.paletteItems = append(m.paletteItems, paletteItem{
			label:     harnessDisplayName(id),
			hint:      harness.PermissionProfileLabel(install.HarnessPermissionProfile(hero, id)),
			action:    actionPickHarnessPermission,
			harnessID: id,
		})
	}
	m = m.ensurePaletteVisible()
	return m, nil
}

func (m model) pickHarnessPermission(harnessID string) (model, tea.Cmd) {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if harnessID == "" {
		return m, nil
	}
	m.pickingHarnessPermission = false
	m.pickingPermissionProfile = true
	m.permissionPickerHarness = harnessID
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	m.paletteItems = []paletteItem{
		{label: harness.PermissionProfileLabel(harness.PermissionProfileAsk), hint: "require confirmation for native approvals", action: actionSelectHarnessPermissionProfile, harnessID: harnessID, modelSlug: string(harness.PermissionProfileAsk)},
		{label: harness.PermissionProfileLabel(harness.PermissionProfileAutoProject), hint: "pre-approve workspace operations only", action: actionSelectHarnessPermissionProfile, harnessID: harnessID, modelSlug: string(harness.PermissionProfileAutoProject)},
	}
	m = m.ensurePaletteVisible()
	return m, nil
}

func (m model) applyHarnessPermissionProfile() (model, tea.Cmd) {
	if m.svc == nil {
		m = m.closePalette()
		m = m.setStatusResult(false, slashHarness, "project unavailable")
		return m, nil
	}
	items := m.filteredPaletteItems()
	if len(items) == 0 || m.paletteIndex < 0 || m.paletteIndex >= len(items) {
		return m, nil
	}
	item := items[m.paletteIndex]
	profile := harness.NormalizePermissionProfile(harness.PermissionProfile(item.modelSlug))
	if err := install.SetHarnessPermissionProfile(m.svc.ProjectDir, m.permissionPickerHarness, profile); err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, slashHarness, err.Error())
		return m, nil
	}
	// Long-lived servers read native permission configuration at startup. Stop
	// them now; the next Execute lazily starts one with the newly saved profile.
	if m.svc != nil {
		switch m.permissionPickerHarness {
		case "opencode":
			if err := stopOpenCodeServe(context.Background(), m.svc.ProjectDir, m.svc.Store, m.svc.Registry); err != nil {
				slog.Warn("stop opencode serve after permission update failed", "error", err)
			}
		case "codex":
			if err := stopCodexAppServer(context.Background(), m.svc.ProjectDir, m.svc.Store, m.svc.Registry); err != nil {
				slog.Warn("stop codex app-server after permission update failed", "error", err)
			}
		}
	}
	m = m.setStatusResult(true, slashHarness, harnessDisplayName(m.permissionPickerHarness)+" permissions: "+harness.PermissionProfileLabel(profile))
	return m.openHarnessPermissionPicker()
}

func (m model) harnessEnabled(id string) bool {
	if m.svc == nil {
		return id == "cursor"
	}
	hero, err := install.LoadHeroJSON(m.svc.ProjectDir)
	if err != nil {
		return false
	}
	return install.IsHarnessEnabled(hero, id)
}

func (m model) harnessAvailabilityLabel(id string) string {
	if m.svc != nil && m.svc.Registry != nil {
		return harnessAvailabilityLabel(m.svc.Registry, id)
	}
	return "available"
}
