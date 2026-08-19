package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func (m model) openHarnessPicker() (model, tea.Cmd) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	if projectDir == "" {
		m = m.setStatusResult(false, "/hero-harness", "project unavailable")
		return m, nil
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		m = m.setStatusResult(false, "/hero-harness", err.Error())
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
		m = m.setStatusResult(false, "/hero-harness", "project unavailable")
		return m, nil
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-harness", err.Error())
		return m, nil
	}

	enabledCount := 0
	for _, id := range install.SupportedHarnessIDs {
		if m.harnessDraft[id] {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		m = m.setStatusResult(false, "/hero-harness", "Select at least one harness.")
		return m, nil
	}

	var enabledNames, disabledNames []string
	for _, id := range install.SupportedHarnessIDs {
		want := m.harnessDraft[id]
		have := install.IsHarnessEnabled(hero, id)
		if want && !have {
			if err := install.EnableHarnessWithProjection(projectDir, id, assets.FS); err != nil {
				m = m.closePalette()
				m = m.setStatusResult(false, "/hero-harness", err.Error())
				return m, nil
			}
			enabledNames = append(enabledNames, harnessDisplayName(id))
		}
	}
	hero, err = install.LoadHeroJSON(projectDir)
	if err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-harness", err.Error())
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
				m = m.setStatusResult(false, "/hero-harness", msg)
				return m, nil
			}
			if id == "opencode" {
				var st *store.Store
				if m.svc != nil {
					st = m.svc.Store
				}
				if err := stopOpenCodeServe(context.Background(), projectDir, st); err != nil {
					slog.Warn("stop opencode serve on disable failed", "error", err)
				}
			}
			disabledNames = append(disabledNames, harnessDisplayName(id))
		}
	}

	m = m.closePalette()
	if len(enabledNames) == 0 && len(disabledNames) == 0 {
		m = m.setStatusResult(true, "/hero-harness", "Harness selection unchanged")
		return m, nil
	}
	var parts []string
	for _, name := range enabledNames {
		msg := name + " enabled"
		if name == "OpenCode" {
			msg = "OpenCode enabled (projected .opencode/)"
		}
		parts = append(parts, msg)
	}
	for _, name := range disabledNames {
		parts = append(parts, name+" disabled (files kept)")
	}
	m = m.setStatusResult(true, "/hero-harness", strings.Join(parts, "; "))
	return m, nil
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
