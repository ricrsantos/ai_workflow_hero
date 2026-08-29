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
	m.modelPickerHarness = ""
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	m.harnessDraft = make(map[string]bool, len(install.SupportedHarnessIDs))
	m.harnessPermissionDraft = make(map[string]map[harness.PermissionProfile]bool, len(install.SupportedHarnessIDs))

	items := make([]paletteItem, 0, len(install.SupportedHarnessIDs)*3)
	for _, id := range install.SupportedHarnessIDs {
		enabled := install.IsHarnessEnabled(hero, id)
		m.harnessDraft[id] = enabled
		profile := install.HarnessPermissionProfile(hero, id)
		m.harnessPermissionDraft[id] = map[harness.PermissionProfile]bool{
			harness.PermissionProfileAsk:         profile == harness.PermissionProfileAsk,
			harness.PermissionProfileAutoProject: profile == harness.PermissionProfileAutoProject,
		}
		items = append(items, paletteItem{
			label:     harnessDisplayName(id),
			hint:      "(" + m.harnessAvailabilityLabel(id) + ")",
			action:    actionToggleHarness,
			harnessID: id,
		})
		items = append(items,
			paletteItem{label: harness.PermissionProfileLabel(harness.PermissionProfileAsk), action: actionToggleHarnessPermission, harnessID: id, modelSlug: string(harness.PermissionProfileAsk)},
			paletteItem{label: harness.PermissionProfileLabel(harness.PermissionProfileAutoProject), action: actionToggleHarnessPermission, harnessID: id, modelSlug: string(harness.PermissionProfileAutoProject)},
		)
	}
	m.paletteItems = items
	m = m.ensurePaletteVisible()
	return m, nil
}

func (m model) toggleHarnessPickerDraft() (model, tea.Cmd) {
	items := m.filteredPaletteItems()
	if len(items) == 0 || m.paletteIndex < 0 || m.paletteIndex >= len(items) {
		return m, nil
	}
	item := items[m.paletteIndex]
	id := strings.TrimSpace(strings.ToLower(item.harnessID))
	if id == "" {
		return m, nil
	}
	switch item.action {
	case actionToggleHarness:
		if m.harnessDraft == nil {
			m.harnessDraft = make(map[string]bool)
		}
		m.harnessDraft[id] = !m.harnessDraft[id]
	case actionToggleHarnessPermission:
		// Permission controls are shown below every Harness, but a disabled
		// Harness cannot be configured until it is enabled in this same draft.
		if !m.harnessDraft[id] {
			return m, nil
		}
		if m.harnessPermissionDraft == nil {
			m.harnessPermissionDraft = make(map[string]map[harness.PermissionProfile]bool)
		}
		if m.harnessPermissionDraft[id] == nil {
			m.harnessPermissionDraft[id] = make(map[harness.PermissionProfile]bool)
		}
		profile := harness.NormalizePermissionProfile(harness.PermissionProfile(item.modelSlug))
		m.harnessPermissionDraft[id][profile] = !m.harnessPermissionDraft[id][profile]
	}
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
	var permissionChanges []string
	for _, id := range install.SupportedHarnessIDs {
		if !m.harnessDraft[id] {
			continue // Keep disabled Harness profiles intact and inactive.
		}
		profile := m.draftHarnessPermissionProfile(id)
		if install.HarnessPermissionProfile(hero, id) == profile {
			continue
		}
		if err := install.SetHarnessPermissionProfile(projectDir, id, profile); err != nil {
			m = m.closePalette()
			m = m.setStatusResult(false, slashHarness, err.Error())
			return m, nil
		}
		permissionChanges = append(permissionChanges, harnessDisplayName(id)+" permissions: "+harness.PermissionProfileLabel(profile))
		switch id {
		case "opencode":
			if err := stopOpenCodeServe(context.Background(), projectDir, m.svc.Store, m.svc.Registry); err != nil {
				slog.Warn("stop opencode serve after permission update failed", "error", err)
			}
		case "codex":
			if err := stopCodexAppServer(context.Background(), projectDir, m.svc.Store, m.svc.Registry); err != nil {
				slog.Warn("stop codex app-server after permission update failed", "error", err)
			}
		}
	}
	if len(permissionChanges) > 0 {
		statusText := m.statusText
		if statusText != "" {
			permissionChanges = append([]string{statusText}, permissionChanges...)
		}
		m = m.setStatusResult(true, slashHarness, strings.Join(permissionChanges, "; "))
	}
	m = m.closePalette()
	return m, nil
}

// draftHarnessPermissionProfile resolves the checkbox pair to Hero's portable
// profile. No selection is intentionally conservative (Ask); when both are
// selected, automatic project approval wins as agreed by the Harness Manager UX.
func (m model) draftHarnessPermissionProfile(id string) harness.PermissionProfile {
	draft := m.harnessPermissionDraft[id]
	if draft[harness.PermissionProfileAutoProject] {
		return harness.PermissionProfileAutoProject
	}
	return harness.PermissionProfileAsk
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
