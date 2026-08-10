package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

// Exported screen aliases for tests.
const (
	ScreenStatus       = screenStatus
	ScreenApprovals    = screenApprovals
	ScreenArtifacts    = screenArtifacts
	ScreenCosts        = screenCosts
	ScreenEvents       = screenEvents
	ScreenConversation = screenConversation
	ScreenPalette      = screenPalette
	ScreenOutput       = screenOutput
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
	m.paletteOffset = 0
	return m
}

// PaletteOffsetForTest returns the palette scroll offset.
func PaletteOffsetForTest(m model) int {
	return m.paletteOffset
}

// SetPaletteIndexForTest sets the selected palette index.
func SetPaletteIndexForTest(m model, index int) model {
	m.paletteIndex = index
	return m.ensurePaletteVisible()
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

// EnterConversationForTest switches to the conversation screen.
func EnterConversationForTest(m model) model {
	next, _ := m.enterConversation()
	next.cursorBlinkOn = true
	return next
}

// SetConversationInput sets the conversation input buffer.
func SetConversationInput(m model, input string) model {
	m.input = input
	return m
}

// ConversationInputForTest returns the conversation input buffer.
func ConversationInputForTest(m model) string {
	return m.input
}

// ConversationErrorForTest returns the conversation error banner text.
func ConversationErrorForTest(m model) string {
	return m.convError
}

// ConversationTranscriptForTest returns agent transcript text for tests.
func ConversationTranscriptForTest(m model) string {
	var parts []string
	for _, msg := range m.transcript {
		if msg.role == convRoleAgent {
			parts = append(parts, msg.content)
		}
	}
	return strings.Join(parts, "")
}

// IsConversationStreaming reports whether conversation is streaming.
func IsConversationStreaming(m model) bool {
	return m.streaming
}

// HarnessSessionIDForTest returns the in-memory harness session id.
func HarnessSessionIDForTest(m model) string {
	return m.harnessSessionID
}

// SubmitConversationForTest submits the current input (test helper).
func SubmitConversationForTest(m model) (model, tea.Cmd) {
	return m.submitConversation()
}

// ApplyActionResultForTest applies an action result message.
func ApplyActionResultForTest(m model, msg actionResultMsg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

// RunCmdForTest executes a tea.Cmd, expanding BatchMsg into nested cmds, and
// returns the first non-tick business message (preferring actionResultMsg).
func RunCmdForTest(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	var found tea.Msg
	var walk func(tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		msg := c()
		switch m := msg.(type) {
		case tea.BatchMsg:
			for _, nested := range m {
				walk(nested)
			}
		case statusTickMsg:
			// ignore ticker
		default:
			if found == nil {
				found = msg
			}
			if _, ok := msg.(actionResultMsg); ok {
				found = msg
			}
		}
	}
	walk(cmd)
	return found
}

// StatusKindForTest returns the footer status kind name.
func StatusKindForTest(m model) string {
	switch m.statusKind {
	case statusRunning:
		return "running"
	case statusOK:
		return "ok"
	case statusErr:
		return "err"
	default:
		return "idle"
	}
}

// ActionBusyForTest reports whether a palette/dispatch action is in flight.
func ActionBusyForTest(m model) bool {
	return m.actionBusy
}

// OutputOffsetForTest returns the output panel scroll offset.
func OutputOffsetForTest(m model) int {
	return m.outputOffset
}

// PrevScreenForTest returns the screen restored on esc from overlays.
func PrevScreenForTest(m model) screen {
	return m.prevScreen
}

// SetHeight sets terminal height for tests.
func SetHeight(m model, h int) model {
	m.height = h
	return m
}

// CancelConversationStreamForTest interrupts an active stream.
func CancelConversationStreamForTest(m model) (model, tea.Cmd) {
	next, cmd := m.handleConversationKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	return next.(model), cmd
}
