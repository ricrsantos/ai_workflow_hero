package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	codexadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

type harnessResetOpenMsg struct {
	items []paletteItem
	err   string
}

func (m model) beginHarnessResetPicker() (model, tea.Cmd) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	if projectDir == "" {
		m = m.setStatusResult(false, "/harness-reset", "project unavailable")
		return m, nil
	}

	m = m.openPaletteOverlay()
	m.harnessResetAwaitingOpen = true
	m.pickingHarnessReset = false
	m.pickingHarness = false
	m.pickingModel = false
	m.modelPickerHarness = ""
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	m.paletteItems = nil
	m.waitAnimFrame = 0

	return m, tea.Batch(convWaitTickCmd(), m.loadHarnessResetPickerCmd(projectDir))
}

func (m model) loadHarnessResetPickerCmd(projectDir string) tea.Cmd {
	var registry harnessmgr.Registry
	if m.svc != nil {
		registry = m.svc.Registry
	}
	return func() tea.Msg {
		hero, err := install.LoadHeroJSON(projectDir)
		if err != nil {
			return harnessResetOpenMsg{err: err.Error()}
		}
		enabled := install.ListEnabledHarnesses(hero)
		if len(enabled) == 0 {
			return harnessResetOpenMsg{err: "No harnesses are enabled in this project."}
		}
		items := make([]paletteItem, 0, len(enabled))
		for _, id := range enabled {
			items = append(items, paletteItem{
				label:     harnessDisplayName(id),
				hint:      "(" + harnessAvailabilityLabel(registry, id) + ")",
				action:    actionSelectHarnessReset,
				harnessID: id,
			})
		}
		return harnessResetOpenMsg{items: items}
	}
}

func (m model) handleHarnessResetOpenMsg(msg harnessResetOpenMsg) (model, tea.Cmd) {
	m.harnessResetAwaitingOpen = false
	if msg.err != "" {
		m = m.closePalette()
		m = m.setStatusResult(false, "/harness-reset", msg.err)
		return m, nil
	}
	m.paletteItems = msg.items
	m.pickingHarnessReset = true
	m.paletteIndex = 0
	m.paletteOffset = 0
	m = m.ensurePaletteVisible()
	return m, nil
}

func harnessAvailabilityLabel(registry harnessmgr.Registry, id string) string {
	if registry != nil {
		if a, err := registry.Adapter(id); err == nil {
			if err := a.IsAvailable(context.Background()); err != nil {
				return "unavailable"
			}
		}
	}
	return "available"
}

func (m model) applyHarnessReset(harnessID string) (model, tea.Cmd) {
	const label = "/harness-reset"
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	name := harnessDisplayName(harnessID)

	m = m.closePalette()

	if !m.harnessEnabled(harnessID) {
		m = m.setStatusResult(false, label, fmt.Sprintf("%s is not enabled in this project.", name))
		return m, nil
	}

	ctx := context.Background()
	var batch []tea.Cmd

	switch harnessID {
	case "opencode":
		if !m.openCodeManagedByHero(ctx) {
			m = m.setStatusWarning(label, "OpenCode has not been started by Hero yet.")
			return m, nil
		}
		if m.streaming && m.chatHarnessID == "opencode" {
			batch = append(batch, m.cancelStreamCmd())
		}
		if err := m.stopOpenCodeServeHarness(ctx); err != nil {
			m = m.setStatusResult(false, label, err.Error())
			return m, nil
		}
		m = m.clearHarnessBindingIfMatch("opencode")
		m = m.setStatusResult(true, label, "OpenCode serve stopped. It will restart on the next request.")
		return m, tea.Batch(batch...)
	case "codex":
		if !m.codexManagedByHero() {
			m = m.setStatusWarning(label, "Codex has not been started by Hero yet.")
			return m, nil
		}
		if m.streaming && m.chatHarnessID == "codex" {
			batch = append(batch, m.cancelStreamCmd())
		}
		if err := m.stopCodexAppServerHarness(ctx); err != nil {
			m = m.setStatusResult(false, label, err.Error())
			return m, nil
		}
		m = m.clearHarnessBindingIfMatch("codex")
		m = m.setStatusResult(true, label, "Codex app-server stopped. It will restart on the next request.")
		return m, tea.Batch(batch...)
	case "cursor":
		cancelled := false
		if m.streaming && (m.chatHarnessID == "cursor" || m.chatHarnessID == "") {
			batch = append(batch, m.cancelStreamCmd())
			cancelled = true
		}
		if a := m.registryCursorAdapter(); a != nil {
			if a.HasInFlight() {
				_ = a.Cancel(ctx, "")
				cancelled = true
			}
		}
		if !cancelled {
			m = m.setStatusWarning(label, "No Cursor agent process is running.")
			return m, nil
		}
		m = m.clearHarnessBindingIfMatch("cursor")
		m = m.setStatusResult(true, label, "Cursor agent process cancelled.")
		return m, tea.Batch(batch...)
	default:
		m = m.setStatusResult(false, label, "Unsupported harness.")
		return m, nil
	}
}

func (m model) openCodeManagedByHero(ctx context.Context) bool {
	if a := m.registryOpenCodeAdapter(); a != nil {
		return a.HasManagedServe(ctx)
	}
	return false
}

func (m model) codexManagedByHero() bool {
	if a := m.registryCodexAdapter(); a != nil {
		return a.HasManagedAppServer()
	}
	return false
}

func (m model) stopOpenCodeServeHarness(ctx context.Context) error {
	if a := m.registryOpenCodeAdapter(); a != nil {
		return a.StopServe(ctx)
	}
	projectDir := ""
	var st *store.Store
	var reg harnessmgr.Registry
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
		st = m.svc.Store
		reg = m.svc.Registry
	}
	return stopOpenCodeServe(ctx, projectDir, st, reg)
}

func (m model) stopCodexAppServerHarness(ctx context.Context) error {
	if a := m.registryCodexAdapter(); a != nil {
		return a.StopAppServer(ctx)
	}
	projectDir := ""
	var st *store.Store
	var reg harnessmgr.Registry
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
		st = m.svc.Store
		reg = m.svc.Registry
	}
	return stopCodexAppServer(ctx, projectDir, st, reg)
}

func (m model) registryOpenCodeAdapter() *opencodeadapter.Adapter {
	if m.svc == nil || m.svc.Registry == nil {
		return nil
	}
	a, err := m.svc.Registry.Adapter("opencode")
	if err != nil {
		return nil
	}
	oc, ok := a.(*opencodeadapter.Adapter)
	if !ok {
		return nil
	}
	return oc
}

func (m model) registryCodexAdapter() *codexadapter.Adapter {
	if m.svc == nil || m.svc.Registry == nil {
		return nil
	}
	a, err := m.svc.Registry.Adapter("codex")
	if err != nil {
		return nil
	}
	cx, ok := a.(*codexadapter.Adapter)
	if !ok {
		return nil
	}
	return cx
}

func (m model) registryCursorAdapter() *cursoradapter.Adapter {
	if m.svc == nil || m.svc.Registry == nil {
		return nil
	}
	a, err := m.svc.Registry.Adapter("cursor")
	if err != nil {
		return nil
	}
	c, ok := a.(*cursoradapter.Adapter)
	if !ok {
		return nil
	}
	return c
}

func (m model) clearHarnessBindingIfMatch(harnessID string) model {
	if strings.TrimSpace(strings.ToLower(m.harnessSessionHarnessID)) == harnessID ||
		strings.TrimSpace(strings.ToLower(m.chatHarnessID)) == harnessID {
		m.harnessSessionID = ""
		m.harnessSessionHarnessID = ""
	}
	return m
}
