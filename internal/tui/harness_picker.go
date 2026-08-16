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

	m.prevScreen = m.screen
	m.screen = screenPalette
	m.pickingHarness = true
	m.pickingModel = false
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0

	items := make([]paletteItem, 0, len(install.SupportedHarnessIDs))
	for _, id := range install.SupportedHarnessIDs {
		enabled := install.IsHarnessEnabled(hero, id)
		avail := "available"
		if m.svc != nil && m.svc.Registry != nil {
			if a, err := m.svc.Registry.Adapter(id); err == nil {
				if err := a.IsAvailable(context.Background()); err != nil {
					avail = "unavailable"
				}
			}
		}
		state := "disabled"
		if enabled {
			state = "enabled"
		}
		actionLabel := "enable"
		if enabled {
			actionLabel = "disable"
		}
		items = append(items, paletteItem{
			label:     harnessDisplayName(id),
			hint:      fmt.Sprintf("%s · %s · %s", state, avail, actionLabel),
			action:    actionToggleHarness,
			harnessID: id,
		})
	}
	m.paletteItems = items
	m = m.ensurePaletteVisible()
	return m, nil
}

func (m model) toggleHarness(harnessID string) (model, tea.Cmd) {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	if projectDir == "" || harnessID == "" {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-harness", "harness unavailable")
		return m, nil
	}

	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-harness", err.Error())
		return m, nil
	}

	enabled := install.IsHarnessEnabled(hero, harnessID)
	name := harnessDisplayName(harnessID)

	if enabled {
		if err := install.SetHarnessEnabled(projectDir, harnessID, false); err != nil {
			m = m.closePalette()
			msg := err.Error()
			if strings.Contains(msg, "last enabled harness") {
				msg = fmt.Sprintf("Cannot disable the last enabled harness (%s).\n\n  Suggestion: enable another harness first, then disable %s.", name, name)
			}
			m = m.setStatusResult(false, "/hero-harness", msg)
			return m, nil
		}
		if harnessID == "opencode" {
			var st *store.Store
			if m.svc != nil {
				st = m.svc.Store
			}
			if err := stopOpenCodeServe(context.Background(), projectDir, st); err != nil {
				slog.Warn("stop opencode serve on disable failed", "error", err)
			}
		}
		m = m.closePalette()
		m = m.setStatusResult(true, "/hero-harness", fmt.Sprintf("%s disabled (files kept)", name))
		return m, nil
	}

	if err := install.EnableHarnessWithProjection(projectDir, harnessID, assets.FS); err != nil {
		m = m.closePalette()
		m = m.setStatusResult(false, "/hero-harness", err.Error())
		return m, nil
	}
	m = m.closePalette()
	msg := fmt.Sprintf("%s enabled", name)
	if harnessID == "opencode" {
		msg = fmt.Sprintf("%s enabled (projected .opencode/)", name)
	}
	m = m.setStatusResult(true, "/hero-harness", msg)
	return m, nil
}
