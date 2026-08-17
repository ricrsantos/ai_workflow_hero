package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// Exported screen aliases for tests.
const (
	ScreenStatus       = screenStatus
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
	m.height = 40
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
	return next
}

// BeginHeroRuntimeConversationForTest runs a Hero runtime slash via Chat streaming.
func BeginHeroRuntimeConversationForTest(m model, cmdName string) (model, tea.Cmd) {
	return m.beginHeroRuntimeConversation(cmdName, "", heroRuntimeOpts{})
}

// EscalatedStageForTest exposes escalated stage detection.
func EscalatedStageForTest(st cycle.StatusView) string {
	return escalatedStage(st)
}

// BeginHeroCancelExecuteForTest runs cancel Runtime Execute with optional reason.
func BeginHeroCancelExecuteForTest(m model, reason string) (model, tea.Cmd) {
	return m.beginHeroCancelExecute(reason)
}

// BeginHeroContinueExecuteForTest runs continue Runtime Execute with extra iterations.
func BeginHeroContinueExecuteForTest(m model, extra int) (model, tea.Cmd) {
	return m.beginHeroContinueExecute(extra)
}

// BeginHeroResumeExecuteForTest runs resume Runtime Execute with optional cycle number.
func BeginHeroResumeExecuteForTest(m model, cycleN int) (model, tea.Cmd) {
	return m.beginHeroResumeExecute(cycleN)
}

// AwaitingRejectReasonForTest reports whether Chat is collecting rejection feedback.
func AwaitingRejectReasonForTest(m model) bool {
	return m.awaitingRejectReason
}

// BeginHeroRejectExecuteForTest runs reject Runtime Execute with the given reason.
func BeginHeroRejectExecuteForTest(m model, reason string) (model, tea.Cmd) {
	return m.beginHeroRejectExecute(reason)
}

// SetConversationInput sets the conversation input buffer.
func SetConversationInput(m model, input string) model {
	m.input = input
	m.inputCursor = runeLen(input)
	m.chatInputFocused = true
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
		case statusTickMsg, convWaitTickMsg:
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

func ContentOffsetForTest(m model) int {
	return m.contentOffset
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

// ChatModeForTest returns the active chat mode.
func ChatModeForTest(m model) string {
	return m.chatMode
}

// InputCursorForTest returns the input caret rune offset.
func InputCursorForTest(m model) int {
	return m.inputCursor
}

// ChatInputFocusedForTest reports whether the chat input has focus.
func ChatInputFocusedForTest(m model) bool {
	return m.chatInputFocused
}

func ChatSlashOverlayActiveForTest(m model) bool {
	return m.chatSlashOverlayActive()
}

func FilteredChatSlashForTest(m model) []PaletteItemView {
	items := m.filteredChatSlashItems()
	out := make([]PaletteItemView, len(items))
	for i, item := range items {
		out[i] = PaletteItemView{Label: item.label, Hint: item.hint}
	}
	return out
}

func SetOrchestrationLiveForTest(m model, live bool) model {
	m.orchestrationLive = live
	return m
}

func SetHarnessSessionIDForTest(m model, id string) model {
	m.harnessSessionID = id
	return m
}

func SlashOverlayDismissedForTest(m model) bool {
	return m.slashOverlayDismissed
}

// SetChatModeForTest sets the chat mode.
func SetChatModeForTest(m model, mode string) model {
	m.chatMode = mode
	return m
}

// SetChatModelSlugForTest sets the active chat model slug.
func SetChatModelSlugForTest(m model, slug string) model {
	m.chatModelSlug = slug
	return m
}

// ChatModelSlugForTest returns the active chat model slug.
func ChatModelSlugForTest(m model) string {
	return m.chatModelSlug
}

// SetChatHarnessIDForTest sets the active chat harness id.
func SetChatHarnessIDForTest(m model, harnessID string) model {
	m.chatHarnessID = harnessID
	return m
}

// SetAvailableModelsForTest sets Cursor model ids for the picker cache.
func SetAvailableModelsForTest(m model, models []string) model {
	m.availableModels = append([]string(nil), models...)
	m.modelOptions = nil
	for _, slug := range models {
		m.modelOptions = append(m.modelOptions, harnessmgr.ModelOption{Model: slug, Harness: "cursor"})
	}
	return m
}

// SetModelOptionsForTest sets the mixed-harness model catalog.
func SetModelOptionsForTest(m model, opts []harnessmgr.ModelOption) model {
	m.modelOptions = append([]harnessmgr.ModelOption(nil), opts...)
	m.availableModels = flattenModelOptions(m.modelOptions)
	return m
}

// ModelPickerHarnessForTest returns the selected harness in /hero-model step 2.
func ModelPickerHarnessForTest(m model) string {
	return m.modelPickerHarness
}

// HandleTestMsg applies an arbitrary tea.Msg to the model.
func HandleTestMsg(m model, msg tea.Msg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

// StatusTextForTest returns the footer status text.
func StatusTextForTest(m model) string {
	return m.statusText
}

// ReapOpenCodeOrphansForTest runs orphan serve cleanup (integration tests).
func ReapOpenCodeOrphansForTest(ctx context.Context, projectDir string, st *store.Store) error {
	return reapOpenCodeOrphans(ctx, projectDir, st)
}

// StopOpenCodeServeFnForTest returns the injectable stop hook.
func StopOpenCodeServeFnForTest() func(context.Context, string, *store.Store) error {
	return stopOpenCodeServeFn
}

// SetStopOpenCodeServeFnForTest replaces the injectable stop hook for tests.
func SetStopOpenCodeServeFnForTest(fn func(context.Context, string, *store.Store) error) {
	stopOpenCodeServeFn = fn
}

// HarnessSessionIDForPairForTest exposes session binding for tests.
func HarnessSessionIDForPairForTest(m model, stageName, pairHarness string) string {
	return m.harnessSessionIDForPair(stageName, pairHarness)
}

// SetHarnessSessionHarnessIDForTest sets the in-memory session harness id.
func SetHarnessSessionHarnessIDForTest(m model, harnessID string) model {
	m.harnessSessionHarnessID = harnessID
	return m
}
func PickingHarnessForTest(m model) bool {
	return m.pickingHarness
}

// PickingModelForTest reports whether the model picker is open.
func PickingModelForTest(m model) bool {
	return m.pickingModel
}

// LiveAgentsForTest returns the live agent labels currently tracked on Chat.
func LiveAgentsForTest(m model) []liveAgent {
	out := make([]liveAgent, len(m.liveAgents))
	copy(out, m.liveAgents)
	return out
}

// ConfirmPendingForTest reports whether an inline confirmation dialog is active.
func ConfirmPendingForTest(m model) bool {
	return m.confirmPending
}

// ConfirmMsgForTest returns the current confirmation prompt text.
func ConfirmMsgForTest(m model) string {
	return m.confirmMsg
}

// SetStreamingForTest forcibly sets the streaming flag (for unit tests that need
// to simulate a running agent without a real goroutine).
func SetStreamingForTest(m model, streaming bool) model {
	m.streaming = streaming
	return m
}

func ResearchLiveForTest(m model) bool {
	return m.researchLive
}

func RuntimeAgentNameForTest(m model) string {
	return m.runtimeAgentName
}

func OrchestrationSessionIDForTest(m model) string {
	return m.orchestrationSessionID
}

func ResearchSessionIDForTest(m model) string {
	return m.researchSessionID
}

// StreamDeltaMsgForTest wraps a delta into a streamDeltaMsg for Update injection.
func StreamDeltaMsgForTest(text string) tea.Msg {
	return streamDeltaMsg{delta: harness.StreamDelta{Text: text}}
}

// ExecuteDoneMsgForTest builds an executeDoneMsg for test Update injection.
func ExecuteDoneMsgForTest(err error) tea.Msg {
	return executeDoneMsg{err: err}
}

// ExecuteDoneResultForTest builds an executeDoneMsg with a harness result.
func ExecuteDoneResultForTest(res *harness.ExecutionResult, err error) tea.Msg {
	return executeDoneMsg{result: res, err: err}
}
