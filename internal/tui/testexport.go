package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

// Exported screen aliases for tests.
const (
	ScreenStatus    = screenStatus
	ScreenApprovals = screenApprovals
	ScreenArtifacts = screenArtifacts
	ScreenCosts     = screenCosts
	ScreenEvents    = screenEvents
	ScreenPalette   = screenPalette
)

// PaletteItemView is a test-facing palette item.
type PaletteItemView struct {
	Label string
	Hint  string
}

// ActionResultForTest exposes action results to tests.
type ActionResultForTest = actionResultMsg

// NewTestModel builds a model for unit tests.
func NewTestModel(svc *cycle.Service) model {
	m := newModel(svc)
	m.width = 100
	return m
}

// SetScreen sets the active screen.
func SetScreen(m model, s screen) model {
	m.screen = s
	return m
}

// CurrentScreen returns the active screen.
func CurrentScreen(m model) screen {
	return m.screen
}

// OpenPalette opens the command palette.
func OpenPalette(m model) model {
	m.prevScreen = m.screen
	m.screen = screenPalette
	m.paletteFilter = ""
	m.paletteIndex = 0
	return m
}

// SetPaletteFilter applies a palette filter.
func SetPaletteFilter(m model, filter string) model {
	m.paletteFilter = filter
	return m
}

// FilteredPalette returns filtered palette items for tests.
func FilteredPalette(m model) []PaletteItemView {
	items := m.filteredPaletteItems()
	out := make([]PaletteItemView, len(items))
	for i, item := range items {
		out[i] = PaletteItemView{Label: item.label, Hint: item.hint}
	}
	return out
}

// PaletteItemsForTest returns all palette items (unfiltered).
func PaletteItemsForTest(m model) []PaletteItemView {
	out := make([]PaletteItemView, len(m.paletteItems))
	for i, item := range m.paletteItems {
		out[i] = PaletteItemView{Label: item.label, Hint: item.hint}
	}
	return out
}

// ReloadPaletteForTest rebuilds palette items from the service project dir.
func ReloadPaletteForTest(m model) model {
	return m.reloadPaletteItems()
}

// BuildPaletteForTest builds palette items for a project and optional user home.
func BuildPaletteForTest(projectDir, userHome string) []PaletteItemView {
	items := buildPaletteItemsWithHome(projectDir, userHome)
	out := make([]PaletteItemView, len(items))
	for i, item := range items {
		out[i] = PaletteItemView{Label: item.label, Hint: item.hint}
	}
	return out
}

// RunPaletteItemForTest executes a palette item by label (unfiltered list).
func RunPaletteItemForTest(m model, label string) (model, tea.Cmd) {
	for _, item := range m.paletteItems {
		if item.label == label {
			return m.runPaletteAction(item)
		}
	}
	return m, nil
}

// ViewForTest renders the model view.
func ViewForTest(m model) string {
	return m.View()
}

// HandleTestKey applies a key string to the model.
func HandleTestKey(m model, key string) (model, tea.Cmd) {
	next, cmd := m.Update(parseTestKey(key))
	return next.(model), cmd
}

// HandleTestKeyMsg applies a key message to the model.
func HandleTestKeyMsg(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

// SetWidth sets terminal width for tests.
func SetWidth(m model, w int) model {
	m.width = w
	return m
}

// RefreshDataForTest builds a refresh message.
func RefreshDataForTest(st cycle.StatusView) refreshDataMsg {
	return refreshDataMsg{status: st}
}

// PendingApprovalForTest exposes pending stage detection.
func PendingApprovalForTest(st cycle.StatusView) string {
	return pendingApprovalStage(st)
}
