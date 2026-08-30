package tui

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	codexadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type screen int

const (
	screenConversation screen = iota
	screenConfig
	screenSettings
	screenStatus
	screenArtifacts
	screenCosts
	screenEvents
	screenPalette
	screenOutput
)

type model struct {
	svc          *cycle.Service
	freeChatMode bool // hero chat: Chat-only, no cycle chrome
	width        int
	height       int
	screen       screen
	shellFocus   shellFocus
	navCursor    int

	status    cycle.StatusView
	metrics   cycle.MetricsView
	events    cycle.EventsView
	artifacts cycle.ArtifactsView
	approvals cycle.ApprovalsView
	config    configScreen
	settings  settingsScreen

	contentOffset int // scroll for Status/Artifacts/Costs/Events

	// Fixed footer status bar (running / result / error).
	statusKind  statusKind
	statusLabel string
	statusText  string
	actionBusy  bool

	// Shared TUI counters. The session timer is cycle-backed or free-chat
	// in-memory; AI wk covers the currently executing demand and AI rp tracks
	// the elapsed time since the last harness response displayed in Chat.
	sessionTimer     sessionTimerState
	aiTimer          aiTimerState
	aiResponseTimer  aiTimerState
	timerLoopStarted bool
	timerGeneration  uint64

	paletteFilter string
	paletteIndex  int
	paletteOffset int // first visible row in the scrollable command list
	paletteItems  []paletteItem
	prevScreen    screen

	// Scrollable command result panel (e.g. /hero-cycles).
	outputTitle  string
	outputRaw    string
	outputLines  []string
	outputOffset int
	outputErr    bool

	// Conversation screen (design D4 / UI-C03-001 §3).
	conversationStage       string
	harnessSessionID        string
	harnessSessionHarnessID string // harness that owns harnessSessionID (session binding)
	transcript              []convMessage
	input                   string
	inputCursor             int // rune offset into input
	inputVerticalColumn     int // preferred visual column while moving up/down
	inputVerticalColumnSet  bool
	streaming               bool
	streamInterrupted       bool
	convError               string
	agentMsgIndex           int
	thinkingMsgIndex        int
	convStreamCh            chan tea.Msg
	chatInputFocused        bool

	// Chat panes: composer scroll + linear transcript scroll/follow.
	inputScrollOffset      int
	transcriptScrollOffset int
	transcriptFollowBottom bool // auto-stick transcript to latest lines while streaming
	waitAnimFrame          int

	// Chat OpenCode-style controls.
	chatMode                 string // harness.ModeBuild | harness.ModePlan
	chatModelSlug            string
	chatHarnessID            string
	modelOptions             []harnessmgr.ModelOption
	availableModels          []string
	pickingModel             bool
	pickingHarness           bool
	pickingHarnessReset      bool
	harnessResetAwaitingOpen bool // loading harness list before reset picker is interactive
	heroStartPreparing       bool // syncing opencode agents before /hero-start orchestration
	heroStartBootstrapping   bool // validating/syncing /hero-start before harness execution
	heroStartCancel          context.CancelFunc
	heroStartRequestID       uint64
	modelPickerHarness       string          // non-empty = /hero-model step 2 (models for this harness)
	harnessDraft             map[string]bool // checkbox state while /hero-harness is open
	harnessPermissionDraft   map[string]map[harness.PermissionProfile]bool
	runtimeCommandName       string // hero runtime slash body name (e.g. "new") for Chat output normalization
	runtimeModelSlug         string // YAML orch/discover slug or /hero-model default for the active runtime slash
	runtimeHarnessID         string // YAML orch/discover harness (or resolved execute pair); preferred over freechat for labels
	runtimeAgentName         string // harness agent name for active runtime slash (e.g. orchestration_agent)
	orchestrationLive        bool   // /hero-start session: follow-ups resume orchestrator model + session
	researchLive             bool   // TUI Research: free-text follow-ups resume discover_agent
	orchestrationSessionID   string // saved orchestrator harness session while Research is live
	researchSessionID        string // discover_agent harness session
	awaitingRejectReason     bool   // Chat is collecting rejection feedback before Runtime Execute
	executeSeq               int    // monotonic id for tagged concurrent Executes
	executes                 map[string]convExecute
	stageHandoffLive         bool
	stageHandoffStage        string
	stageHandoffOutputs      []string
	stageHandoffDoneKey      string // "stage:iteration" already TUI-executed this session

	// C5 model properties (ADR-042).
	propsSvc             *modelprops.Service
	freechatProps        map[string]string // selected freechat property values (status line + execution)
	freechatSnapshot     modelprops.Snapshot
	workflowProps        map[string]string // YAML-derived projection during workflow/runtime commands
	pickingProps         bool              // /hero-model step 3 (property picker open)
	propsDraftHarness    string
	propsDraftModel      string
	propsDraft           map[string]string // in-memory property draft (fs/th/ef)
	propsEdited          map[string]bool   // rows edited this draft session (Enter commit guard)
	propsSnapshot        modelprops.Snapshot
	propsValueList       bool   // secondary multi-value list open (th/ef)
	propsValueKey        string // key whose secondary list is open
	propsValueIndex      int    // cursor inside the secondary value list
	propsRefreshBusy     bool   // background refresh in flight (never blocks the picker)
	propsAwaitingRefresh bool   // waiting for refresh before applying model selection
	propsPendingSelect   *pendingModelSelect
	propsWarningText     string // yellow C5 warning (missing catalog / stale / invalidated)

	slashOverlayIndex     int  // selected row in Chat `/` autocomplete
	slashOverlayDismissed bool // Esc or insert closed the overlay until the token changes

	liveAgents []liveAgent // currently executing parent + Task subagents (Chat box)

	contextUsedTokens      int64
	contextUsageGeneration int64
	contextWindows         contextWindowCatalog

	// Inline confirmation for destructive actions while an agent is streaming.
	confirmPending bool
	confirmMsg     string
	confirmAction  paletteAction
	confirmActionN int // optional numeric arg (e.g. /hero-continue N)

	// Shown once after /hero-new has successfully created its active cycle.
	// It is transient TUI state: reopening Hero must not show it again.
	cycleWelcomeDialog bool
	cycleWelcomeFocus  int // 0 = Go to Config, 1 = Close

	// Harness-native permission prompt (OpenCode permission.asked, etc.).
	harnessPermissionPending bool
	harnessPermissionMsg     string
	harnessPermissionReq     harness.PermissionRequest
	harnessPermissionRespCh  chan harness.PermissionResponse

	// Harness-native question prompt (OpenCode question.asked).
	harnessQuestionPending bool
	harnessQuestionMsg     string
	harnessQuestionReq     harness.QuestionRequest
	harnessQuestionRespCh  chan harness.QuestionResponse
	harnessQuestionIndex   int
	harnessQuestionAnswers [][]string

	// Harness watchdog (v2.3): runtime health during TUI Execute only (warn-only).
	harnessWatchdog       harness.Watchdog
	harnessHealthStatus   harness.HealthStatus
	harnessHealthInFlight bool
	lastExecutePrompt     string

	testMode bool // NewTestModel: omit long-lived execute timers (health probe).
}

type refreshDataMsg struct {
	status       cycle.StatusView
	sessionCycle *store.Cycle
	metrics      cycle.MetricsView
	events       cycle.EventsView
	artifacts    cycle.ArtifactsView
	approvals    cycle.ApprovalsView
	err          error
}

type actionResultMsg struct {
	err     error
	success string
	title   string // optional panel title for multiline output
}

func newModel(svc *cycle.Service) model {
	m := model{
		svc:                    svc,
		screen:                 screenConversation,
		prevScreen:             screenConversation,
		chatMode:               harness.ModeBuild,
		agentMsgIndex:          -1,
		thinkingMsgIndex:       -1,
		transcriptFollowBottom: true,
		chatInputFocused:       true,
		sessionTimer:           sessionTimerState{suppressed: true},
	}
	projectDir := ""
	if svc != nil {
		projectDir = svc.ProjectDir
		if hero, err := install.LoadHeroJSON(projectDir); err == nil {
			h, slug := install.GetFreechatDefault(hero)
			m.chatHarnessID = h
			m.chatModelSlug = slug
			m.settings.verbosity = install.NormalizeChatVerbosity(hero.ChatVerbosity)
		}
	}
	if m.settings.verbosity == "" {
		m.settings.verbosity = install.ChatVerbosityDebug
	}
	m.contextWindows = loadContextWindowCatalog(projectDir)
	m = m.reloadPaletteItems()
	m = m.initModelProps(projectDir)
	return m.syncConversationContext()
}

func newModelWithChat(svc *cycle.Service, models []harnessmgr.ModelOption, modelSlug, harnessID, modelWarn string) model {
	m := newModel(svc)
	m.modelOptions = append([]harnessmgr.ModelOption(nil), models...)
	m.availableModels = flattenModelOptions(models)
	if strings.TrimSpace(harnessID) != "" {
		m.chatHarnessID = strings.TrimSpace(harnessID)
	}
	if strings.TrimSpace(modelSlug) != "" {
		m.chatModelSlug = strings.TrimSpace(modelSlug)
	}
	// Boot may provide a legacy C4 pair through harnesses.* while
	// freechat_default.model is still empty.  Re-run the local C5 projection
	// after applying the boot pair so persisted model_properties are restored.
	m = m.loadFreechatProps()
	if modelWarn != "" {
		m = m.setStatusResult(false, "model", modelWarn)
	}
	return m
}

func flattenModelOptions(opts []harnessmgr.ModelOption) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, o.Model)
	}
	return out
}

func (m model) reloadPaletteItems() model {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	items := buildPaletteItems(projectDir)
	if m.hasActiveCycle() {
		for i, item := range items {
			if item.label == "Go to - Status" {
				items = append(items, paletteItem{})
				copy(items[i+1:], items[i:])
				items[i] = paletteItem{label: "Go to - Config", hint: "cycle configuration", action: actionGoScreen, screen: screenConfig}
				break
			}
		}
	}
	if m.freeChatMode {
		// Free chat: no project slash discovery; only non-/hero chat commands.
		items = filterFreeChatPaletteItems(defaultHeroPaletteItems())
	}
	m.paletteItems = items
	return m
}

// executeDir is the harness Execute workspace (cwd in free chat, else project).
func (m model) executeDir() string {
	if m.svc == nil {
		return ""
	}
	return m.svc.ExecuteDir()
}

func (m model) Init() tea.Cmd {
	slog.Debug("tui init")
	return m.refreshCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.shellFocus == shellFocusNavbar && !m.sidebarVisible() {
			m.shellFocus = shellFocusContent
			m.chatInputFocused = m.screen == screenConversation
		}
		if m.screen == screenPalette {
			m = m.ensurePaletteVisible()
		}
		if m.screen == screenOutput {
			m = m.rebuildOutputLines()
		}
		if m.screen == screenConversation {
			m = m.scrollTranscript(0) // clamp to new transcript viewport
			m = m.ensureInputCaretVisible()
		}
		m = m.clampContentOffset()
		return m, nil

	case refreshDataMsg:
		if msg.err != nil {
			m = m.setStatusResult(false, "refresh", msg.err.Error())
			slog.Error("tui refresh failed", "error", msg.err)
			return m, nil
		}
		hadCycle := m.hasActiveCycle()
		m.status = msg.status
		m.metrics = msg.metrics
		m.events = msg.events
		m.artifacts = msg.artifacts
		m.approvals = msg.approvals
		if hadCycle && !m.hasActiveCycle() {
			m = m.syncActiveCycleChrome()
		} else if !hadCycle && m.hasActiveCycle() {
			m = m.reloadPaletteItems()
		}
		var timerCmd tea.Cmd
		m, timerCmd = m.syncSessionTimer(msg.sessionCycle, time.Now())
		m = m.clampContentOffset()
		slog.Debug("tui data refreshed", "cycle", m.status.CycleNumber)
		return m, timerCmd

	case configLoadedMsg, configSavedMsg, configRetryMsg:
		return m.handleConfigMsg(msg)

	case settingsSavedMsg:
		return m.handleSettingsMsg(msg)

	case actionResultMsg:
		m.actionBusy = false
		label := msg.title
		if label == "" {
			label = m.statusLabel
		}
		if msg.err != nil {
			text := msg.err.Error()
			slog.Error("tui action failed", "error", msg.err)
			if label == "" {
				label = "Error"
			}
			if statusResultOpensPanel(text, m.width) {
				m = m.setStatusResult(false, label, firstStatusLine(text))
				m = m.openOutput(label, text, true)
				return m, m.refreshCmd()
			}
			m = m.setStatusResult(false, label, text)
		} else if msg.success != "" {
			slog.Info("tui action ok", "message", msg.success)
			if label == "" {
				label = "Output"
			}
			if statusResultOpensPanel(msg.success, m.width) {
				m = m.setStatusResult(true, label, firstStatusLine(msg.success))
				m = m.openOutput(label, msg.success, false)
				return m, m.refreshCmd()
			}
			m = m.setStatusResult(true, label, msg.success)
		}
		return m, m.refreshCmd()

	case timerTickMsg:
		return m.handleTimerTick(msg)

	case convWaitTickMsg:
		if m.streaming {
			m.waitAnimFrame++
			m = m.maybeFollowTranscriptBottom()
			return m, convWaitTickCmd()
		}
		if m.propsAwaitingRefresh || m.harnessResetAwaitingOpen || m.heroStartPreparing || m.heroStartBootstrapping {
			m.waitAnimFrame++
			if m.heroStartPreparing || m.heroStartBootstrapping {
				m = m.maybeFollowTranscriptBottom()
			}
			return m, convWaitTickCmd()
		}
		return m, nil

	case harnessResetOpenMsg:
		return m.handleHarnessResetOpenMsg(msg)

	case harnessHealthProbeMsg:
		return m.handleHarnessHealthProbe()

	case harnessHealthResultMsg:
		return m.handleHarnessHealthResult(msg)

	case heroStartPrepareDoneMsg:
		return m.handleHeroStartPrepareDone(msg)

	case heroStartBootstrapDoneMsg:
		return m.handleHeroStartBootstrapDone(msg)

	case confirmResumeMsg:
		return m.dispatchConfirmedAction(msg.action, msg.actionN)

	case listModelsMsg:
		return m.handleListModelsMsg(msg)

	case modelRefreshDoneMsg:
		m.propsRefreshBusy = false
		for _, summary := range msg.summaries {
			if summary.Err != nil {
				slog.Debug("tui model props refresh failed", "harness", summary.HarnessID, "error", summary.Err)
			}
		}
		if m.propsPendingSelect != nil {
			pending := *m.propsPendingSelect
			m.propsPendingSelect = nil
			m.propsAwaitingRefresh = false
			return m.applyModelSelection(pending.modelSlug, pending.harnessID, true, msg.summaries)
		}
		slog.Debug("tui model props refresh done", "harnesses", len(msg.summaries))
		return m, nil

	case conversationBatchMsg, streamDeltaMsg, executeDoneMsg, streamCancelDoneMsg, harnessPermissionRequestMsg, harnessQuestionRequestMsg, executePairMsg:
		// Always process stream messages so the goroutine is never orphaned when
		// the user navigates away from the Chat screen while streaming.
		return m.handleConversationMsg(msg)

	case tea.KeyMsg:
		if m.cycleWelcomeDialog {
			return m.handleCycleWelcomeKey(msg)
		}
		if m.harnessPermissionPending {
			return m.handleHarnessPermissionKey(msg)
		}
		if m.confirmPending {
			return m.handleConfirmKey(msg)
		}
		// C5: warnings clear on the next user action (UI-C05-001 §5). The action
		// itself may set a new warning later in this same processing pass.
		m = m.clearPropsWarning()
		if key.Matches(msg, shellFocusKey) {
			return m.toggleShellFocus()
		}
		if m.shellFocus == shellFocusNavbar {
			return m.handleNavbarKey(msg)
		}
		if m.harnessQuestionPending {
			return m.handleHarnessQuestionKey(msg)
		}
		if m.screen == screenOutput {
			return m.handleOutputKey(msg)
		}
		if m.screen == screenPalette {
			return m.handlePaletteKey(msg)
		}
		if m.screen == screenConversation {
			return m.handleConversationKey(msg)
		}
		if m.screen == screenConfig {
			return m.handleConfigKey(msg)
		}
		if m.screen == screenSettings {
			return m.handleSettingsKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading AI Hero..."
	}
	return m.renderFrame()
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if index, ok := navShortcutIndex(msg); ok {
		return m.navigateNavShortcut(index)
	}
	if isLegacyEventsShortcut(msg) {
		if m.freeChatMode {
			return m, nil
		}
		return m.goListScreen(screenEvents)
	}

	switch msg.String() {
	case "alt+q":
		if m.streaming || m.heroStartBootstrapping || m.heroStartPreparing {
			return m.showConfirm(actionQuit, 0, "Agent is running. Quit? [y/N]")
		}
		return m, tea.Quit
	case "/":
		m.chatInputFocused = false
		m.prevScreen = m.screen
		m.screen = screenPalette
		m.paletteFilter = ""
		m.paletteIndex = 0
		m.paletteOffset = 0
		m.pickingModel = false
		m.pickingHarness = false
		m.pickingHarnessReset = false
		m.harnessResetAwaitingOpen = false
		m.modelPickerHarness = ""
		m.harnessDraft = nil
		m.harnessPermissionDraft = nil
		m = m.reloadPaletteItems()
		return m, nil
	case "alt+r", "f5":
		if m.freeChatMode {
			return m, nil
		}
		return m, m.refreshCmd()
	case "up":
		if m.screenHasContentScroll() {
			return m.scrollContent(-1), nil
		}
	case "down":
		if m.screenHasContentScroll() {
			return m.scrollContent(1), nil
		}
	case "pgup":
		if m.screenHasContentScroll() {
			return m.scrollContent(-m.frameContentHeight()), nil
		}
	case "pgdown":
		if m.screenHasContentScroll() {
			return m.scrollContent(m.frameContentHeight()), nil
		}
	case "home":
		if m.screenHasContentScroll() {
			m.contentOffset = 0
			return m, nil
		}
	case "end":
		if m.screenHasContentScroll() {
			m.contentOffset = m.maxContentOffset()
			return m, nil
		}
	}
	return m, nil
}

func (m model) navigateNavShortcut(index int) (model, tea.Cmd) {
	target, ok := m.navScreenAt(index)
	if !ok {
		return m, nil
	}
	m.shellFocus = shellFocusContent
	return m.navigateToScreen(target)
}

func (m model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.harnessResetAwaitingOpen {
		switch msg.String() {
		case "esc":
			m.harnessResetAwaitingOpen = false
			m = m.closePalette()
			return m, nil
		case "alt+q":
			return m, tea.Quit
		default:
			return m, nil
		}
	}
	if m.pickingProps {
		return m.handlePropertyPickerKey(msg)
	}
	if isNavKey(msg) || msg.String() == "alt+q" || msg.String() == "alt+r" || msg.String() == "f5" {
		// Leave palette chrome before global navigation / refresh / quit.
		m.pickingModel = false
		m.pickingHarness = false
		m.pickingHarnessReset = false
		m.harnessResetAwaitingOpen = false
		m.modelPickerHarness = ""
		m.harnessDraft = nil
		m.harnessPermissionDraft = nil
		m.paletteFilter = ""
		m.paletteIndex = 0
		m.paletteOffset = 0
		return m.handleKey(msg)
	}
	switch msg.String() {
	case "esc":
		if m.propsAwaitingRefresh {
			m = m.clearPropsPendingSelect()
			m = m.closePalette()
			m = m.setStatusResult(true, "/model", "Selection cancelled — no changes saved.")
			return m, nil
		}
		if m.pickingModel && m.modelPickerHarness != "" {
			return m.openModelPicker()
		}
		m = m.closePalette()
		return m, nil
	case "up":
		if m.pickingHarness {
			return m.moveHarnessPickerSelection(-1), nil
		}
		if m.paletteIndex > 0 {
			m.paletteIndex--
		}
		m = m.ensurePaletteVisible()
		return m, nil
	case "down":
		if m.pickingHarness {
			return m.moveHarnessPickerSelection(1), nil
		}
		items := m.filteredPaletteItems()
		if m.paletteIndex < len(items)-1 {
			m.paletteIndex++
		}
		m = m.ensurePaletteVisible()
		return m, nil
	case "pgup":
		m.paletteIndex -= m.paletteListHeight()
		if m.paletteIndex < 0 {
			m.paletteIndex = 0
		}
		m = m.ensurePaletteVisible()
		return m, nil
	case "pgdown":
		items := m.filteredPaletteItems()
		m.paletteIndex += m.paletteListHeight()
		if m.paletteIndex >= len(items) {
			m.paletteIndex = len(items) - 1
		}
		if m.paletteIndex < 0 {
			m.paletteIndex = 0
		}
		m = m.ensurePaletteVisible()
		return m, nil
	case " ", "space":
		if m.pickingHarness {
			return m.toggleHarnessPickerDraft()
		}
		if len(msg.Runes) > 0 && !msg.Alt {
			m.paletteFilter += string(msg.Runes)
			m.paletteIndex = 0
			m.paletteOffset = 0
		}
		return m, nil
	case "enter":
		if m.propsAwaitingRefresh {
			return m, nil
		}
		if m.pickingHarness {
			return m.applyHarnessDraft()
		}
		items := m.filteredPaletteItems()
		if len(items) == 0 {
			return m, nil
		}
		if m.paletteIndex >= len(items) {
			m.paletteIndex = len(items) - 1
		}
		return m.runPaletteAction(items[m.paletteIndex])
	case "backspace":
		if len(m.paletteFilter) > 0 {
			m.paletteFilter = m.paletteFilter[:len(m.paletteFilter)-1]
			m.paletteIndex = 0
			m.paletteOffset = 0
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			m.paletteFilter += string(msg.Runes)
			m.paletteIndex = 0
			m.paletteOffset = 0
		}
		return m, nil
	}
}

func (m model) beginAction(label string, cmd tea.Cmd) (model, tea.Cmd) {
	if m.actionBusy {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	m = m.setStatusRunning(label)
	return m, cmd
}

func (m model) runPaletteAction(item paletteItem) (model, tea.Cmd) {
	switch item.action {
	case actionGoScreen:
		m = m.closePalette()
		if item.screen == screenConversation {
			return m.enterConversation()
		}
		return m.goListScreen(item.screen)
	case actionSelectModel:
		if item.modelSlug != "" && item.harnessID != "" {
			return m.selectChatModelPair(item.modelSlug, item.harnessID)
		}
		modelPart, harnessPart := splitModelPairLabel(item.label)
		return m.selectChatModelPair(modelPart, harnessPart)
	case actionPickModelHarness:
		return m.pickModelHarness(item.harnessID)
	case actionToggleHarness:
		return m.toggleHarnessPickerDraft()
	case actionToggleHarnessPermission:
		return m.toggleHarnessPickerDraft()
	case actionApplyHarness:
		return m.applyHarnessDraft()
	case actionHarnessReset:
		return m.beginHarnessResetPicker()
	case actionSelectHarnessReset:
		return m.applyHarnessReset(item.harnessID)
	case actionQuit:
		if m.heroStartBootstrapping || m.heroStartPreparing {
			m, _ = m.cancelHeroStartPreparation()
		}
		return m, tea.Quit
	case actionRefresh:
		m = m.closePalette()
		return m, m.refreshCmd()
	}

	if m.actionBusy {
		m = m.closePalette()
		m = m.setStatusBusyBlocked()
		return m, nil
	}

	// Destructive actions while streaming require a confirmation dialog instead
	// of silently blocking them, so the user understands what will happen.
	if m.streaming && isDestructiveAction(item.action) {
		m = m.closePalette()
		return m.showConfirm(item.action, 0, confirmMsgForAction(item.action))
	}

	m = m.closePalette()

	switch item.action {
	case actionApprove:
		return m.beginHeroApprove()
	case actionReject:
		return m.beginHeroReject()
	case actionNew:
		return m.beginHeroNew()
	case actionStart:
		return m.beginHeroStart()
	case actionSync:
		return m.beginHeroSync()
	case actionStatus:
		return m.beginHeroStatus()
	case actionContinue:
		return m.beginHeroContinue(1)
	case actionBack:
		return m.beginHeroBack()
	case actionCancel:
		return m.beginHeroCancel()
	case actionFinish:
		return m.beginHeroFinish()
	case actionArchive:
		return m.beginHeroArchive()
	case actionResume:
		return m.beginHeroResume(0)
	case actionCycles:
		return m.beginAction("/hero-cycles", m.cyclesCmd())
	case actionTodos:
		return m.beginAction("/hero-todos", m.todosCmd())
	case actionModel:
		return m.openModelPicker()
	case actionConfigUpdate:
		return m.beginHeroConfigUpdate()
	case actionHarness:
		return m.openHarnessPicker()
	case actionHelp:
		return m.beginAction("/hero-help", m.helpCmd())
	case actionImportCommand:
		m, cmd, ok := m.ensureDefaultModel(item.commandLabel)
		if !ok {
			return m, cmd
		}
		return m.beginAction(item.commandLabel, m.importCommandCmd(item))
	case actionNewChat:
		return m.beginNewChat()
	}
	return m, nil
}

func (m model) goListScreen(s screen) (model, tea.Cmd) {
	if m.freeChatMode && s != screenConversation {
		return m, nil
	}
	m.chatInputFocused = false
	if m.screen != s {
		m.contentOffset = 0
	}
	m.screen = s
	return m, m.refreshCmd()
}

func (m model) hasActiveCycle() bool {
	return !m.freeChatMode && m.status.CycleNumber > 0
}

// syncActiveCycleChrome reconciles palette and navigation when no cycle is active
// (e.g. after /hero-archive). Reuses the same rules as TUI boot without a cycle.
func (m model) syncActiveCycleChrome() model {
	m = m.reloadPaletteItems()
	if !m.hasActiveCycle() {
		if m.screen == screenConfig {
			m.config = configScreen{}
			m, _ = m.enterConversation()
			m.contentOffset = 0
		}
		items := m.visibleNavScreens()
		if m.navCursor >= len(items) {
			if len(items) == 0 {
				m.navCursor = 0
			} else {
				m.navCursor = len(items) - 1
			}
		}
	}
	return m
}

func (m model) refreshCmd() tea.Cmd {
	if m.freeChatMode {
		return nil
	}
	svc := m.svc
	if svc == nil {
		return nil
	}
	return func() tea.Msg {
		st, err := svc.Status()
		if err != nil {
			return refreshDataMsg{err: err}
		}
		sessionCycle, sErr := svc.SessionCycle()
		if sErr != nil {
			slog.Debug("tui session cycle refresh failed", "error", sErr)
		}
		metrics, mErr := svc.Metrics()
		if mErr != nil {
			// Metrics may fail when no active cycle; keep empty view.
			metrics = cycle.MetricsView{}
		}
		events, eErr := svc.Events("", 50)
		if eErr != nil {
			events = cycle.EventsView{}
		}
		artifacts, aErr := svc.Artifacts()
		if aErr != nil {
			artifacts = cycle.ArtifactsView{}
		}
		approvals, apErr := svc.Approvals()
		if apErr != nil {
			approvals = cycle.ApprovalsView{}
		}
		if mErr != nil && eErr != nil && aErr != nil && apErr != nil && st.CycleNumber == 0 {
			return refreshDataMsg{err: mErr}
		}
		return refreshDataMsg{
			status: st, sessionCycle: sessionCycle, metrics: metrics, events: events, artifacts: artifacts, approvals: approvals,
		}
	}
}

func (m model) validateOrchestratorPreconditions() (errMsg string) {
	if m.svc == nil {
		return "cycle service unavailable"
	}
	st, err := m.svc.Status()
	if err != nil || st.CycleNumber == 0 {
		return noActiveCycleForStartMessage()
	}
	return ""
}

func (m model) beginNewChat() (model, tea.Cmd) {
	if m.streaming {
		m, _ = m.enterConversation()
		m = m.setStatusResult(false, "/new-chat", newChatBlockedMessage())
		return m, nil
	}
	if m.actionBusy {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	m, _ = m.enterConversation()
	m = m.resetChatSession()
	// /new-chat starts a free-chat session when no cycle is active, so it owns
	// the explicit Session reset in that case. While a cycle is active, the
	// cycle timer must continue through a conversation reset. Other resets
	// (for example /hero-start preflight) must not erase a completed cycle's
	// frozen duration before archive.
	if !m.hasActiveCycle() {
		m = m.resetSessionTimer()
	}
	m = m.setStatusResult(true, "/new-chat", "New chat started with default model.")
	return m, nil
}

func newChatBlockedMessage() string {
	return "Wait for the agent to finish or press esc to interrupt before starting a new chat."
}

func (m model) beginHeroNew() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	m, cmd, ok := m.ensureDefaultModel("/hero-new")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("new", "", heroRuntimeOpts{})
}

func (m model) beginHeroStart() (model, tea.Cmd) {
	if m.streaming || m.heroStartPreparing || m.heroStartBootstrapping {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if m.svc == nil {
		m = m.setStatusResult(false, "/hero-start", "cycle service unavailable")
		return m, nil
	}

	// All filesystem and SQLite bootstrap work runs as a tea.Cmd. The Bubble
	// Tea Update loop must remain available for repaint, navigation, and cancel
	// while /hero-start validates and synchronizes the cycle.
	m, _ = m.enterConversation()
	m = m.resetChatSession()
	m.chatInputFocused = false
	m.heroStartBootstrapping = true
	m.waitAnimFrame = 0
	m.transcriptFollowBottom = true
	m = m.maybeFollowTranscriptBottom()
	m.actionBusy = true
	m.statusKind = statusRunning
	m.statusLabel = "/hero-start"
	m.statusText = "preparing…"
	m.heroStartRequestID++
	requestID := m.heroStartRequestID
	ctx, cancel := context.WithCancel(context.Background())
	m.heroStartCancel = cancel
	return m, tea.Batch(convWaitTickCmd(), m.heroStartBootstrapCmd(ctx, requestID))
}

type heroStartBootstrapDoneMsg struct {
	requestID    uint64
	slug         string
	needsPrepare bool
	commandBody  string
	agentBody    string
	err          error
}

func (m model) heroStartBootstrapCmd(ctx context.Context, requestID uint64) tea.Cmd {
	svc := m.svc
	projectDir := ""
	if svc != nil {
		projectDir = svc.ProjectDir
	}
	return func() tea.Msg {
		if svc == nil {
			return heroStartBootstrapDoneMsg{requestID: requestID, err: fmt.Errorf("cycle service unavailable")}
		}
		st, err := svc.Status()
		if err != nil {
			return heroStartBootstrapDoneMsg{requestID: requestID, err: err}
		}
		if st.CycleNumber == 0 {
			return heroStartBootstrapDoneMsg{requestID: requestID, err: fmt.Errorf("%s", noActiveCycleForStartMessage())}
		}
		if err := ctx.Err(); err != nil {
			return heroStartBootstrapDoneMsg{requestID: requestID, err: err}
		}

		slug, _ := m.orchestratorModelSlug()
		if strings.TrimSpace(slug) == "" {
			return heroStartBootstrapDoneMsg{
				requestID: requestID,
				err:       fmt.Errorf("%s", defaultModelRequiredMessage("/hero-start")),
			}
		}
		if err := svc.SyncCycleConfig(); err != nil {
			return heroStartBootstrapDoneMsg{requestID: requestID, err: err}
		}
		if err := ctx.Err(); err != nil {
			return heroStartBootstrapDoneMsg{requestID: requestID, err: err}
		}

		commandBody, err := cursoradapter.ReadCommandPrompt(filepath.Join(projectDir, cursoradapter.CommandsDir, "hero-start.md"))
		if err != nil {
			return heroStartBootstrapDoneMsg{requestID: requestID, err: fmt.Errorf("read command /hero-start: %w", err)}
		}
		agentBody, err := cursoradapter.ReadAgentPrompt(filepath.Join(projectDir, cursoradapter.AgentsDir, "orchestration_agent.md"))
		if err != nil {
			return heroStartBootstrapDoneMsg{requestID: requestID, err: fmt.Errorf("read orchestration_agent: %w", err)}
		}

		return heroStartBootstrapDoneMsg{
			requestID:    requestID,
			slug:         slug,
			needsPrepare: m.heroStartNeedsPrepare(),
			commandBody:  commandBody,
			agentBody:    agentBody,
		}
	}
}

func (m model) handleHeroStartBootstrapDone(msg heroStartBootstrapDoneMsg) (model, tea.Cmd) {
	if !m.heroStartBootstrapping || msg.requestID != m.heroStartRequestID {
		return m, nil
	}
	m.heroStartBootstrapping = false
	m.heroStartCancel = nil
	if msg.err != nil {
		m.actionBusy = false
		m = m.setStatusResult(false, "/hero-start", msg.err.Error())
		m.convError = msg.err.Error()
		m.chatInputFocused = true
		return m, nil
	}
	if msg.needsPrepare {
		return m.beginHeroStartPrepare(msg.requestID, msg.slug, msg.commandBody, msg.agentBody)
	}
	return m.beginHeroRuntimeConversation("start", msg.slug, heroRuntimeOpts{
		preloadedCommandBody: msg.commandBody,
		preloadedAgentBody:   msg.agentBody,
		preloadedPrompt:      true,
	})
}

// heroStartNeedsPrepare is true when OpenCode and/or Codex Prepare-on-start must run.
func (m model) heroStartNeedsPrepare() bool {
	return m.heroStartNeedsOpenCodePrepare() || m.heroStartNeedsCodexPrepare()
}

func (m model) heroStartNeedsOpenCodePrepare() bool {
	if m.svc == nil {
		return false
	}
	projectDir := m.svc.ProjectDir
	cfg, _, err := workflowconfig.LoadCurrent(projectDir)
	if err != nil {
		return false
	}
	if len(opencodeadapter.AgentsUsingHarness(cfg, "opencode")) == 0 {
		return false
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return false
	}
	for _, id := range install.ListEnabledHarnesses(hero) {
		if strings.EqualFold(id, "opencode") {
			return true
		}
	}
	return false
}

func (m model) heroStartNeedsCodexPrepare() bool {
	if m.svc == nil {
		return false
	}
	projectDir := m.svc.ProjectDir
	cfg, _, err := workflowconfig.LoadCurrent(projectDir)
	if err != nil {
		return false
	}
	if len(codexadapter.AgentsUsingHarness(cfg, "codex")) == 0 {
		return false
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return false
	}
	for _, id := range install.ListEnabledHarnesses(hero) {
		if strings.EqualFold(id, "codex") {
			return true
		}
	}
	return false
}

func (m model) beginHeroSync() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-sync")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("sync", slug, heroRuntimeOpts{})
}

func (m model) beginHeroStatus() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-status")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("status", slug, heroRuntimeOpts{})
}

func (m model) beginHeroArchive() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if errMsg := m.validateOrchestratorPreconditions(); errMsg != "" {
		m = m.setStatusResult(false, "/hero-archive", errMsg)
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-archive")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("archive", slug, heroRuntimeOpts{})
}

func (m model) beginHeroResume(cycleN int) (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	return m.beginHeroResumeExecute(cycleN)
}

func (m model) beginHeroResumeExecute(cycleN int) (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if cycleN < 0 {
		m, _ = m.enterConversation()
		m.convError = "Cycle number must be a positive integer (e.g. /hero-resume 4)."
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-resume")
	if !ok {
		if cycleN > 0 {
			m, _ = m.enterConversation()
			m.convError = defaultModelRequiredMessage("/hero-resume")
			return m, nil
		}
		return m, cmd
	}
	m = m.setStatusRunning("/hero-resume")
	next, execCmd := m.beginHeroRuntimeConversation("resume", slug, heroRuntimeOpts{ResumeCycleNumber: cycleN})
	return next, execCmd
}

func (m model) validateHeroRejectPreconditions() (errMsg string) {
	if m.svc == nil {
		return "cycle service unavailable"
	}
	st, err := m.svc.Status()
	if err != nil || st.CycleNumber == 0 {
		return noActiveCycleForStartMessage()
	}
	if pendingApprovalStage(st) == "" {
		return noPendingApprovalMessage()
	}
	return ""
}

func (m model) beginHeroReject() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if errMsg := m.validateHeroRejectPreconditions(); errMsg != "" {
		m = m.setStatusResult(false, "/hero-reject", errMsg)
		return m, nil
	}
	m, cmd, _, ok := m.orchestratorExecuteModel("/hero-reject")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRejectPrompt()
}

func (m model) beginHeroRejectPrompt() (model, tea.Cmd) {
	m, _ = m.enterConversation()
	m.awaitingRejectReason = true
	m.convError = ""
	m.chatInputFocused = true
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: "/hero-reject"})
	m.transcript = append(m.transcript, convMessage{
		role:    convRoleAgent,
		content: "Enter rejection feedback below, then press Alt+Enter.",
	})
	return m, nil
}

func (m model) beginHeroRejectExecute(reason string) (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		m, _ = m.enterConversation()
		m.awaitingRejectReason = true
		m.convError = "Rejection reason is required."
		return m, nil
	}
	if errMsg := m.validateHeroRejectPreconditions(); errMsg != "" {
		m, _ = m.enterConversation()
		m.convError = errMsg
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-reject")
	if !ok {
		m, _ = m.enterConversation()
		m.convError = defaultModelRequiredMessage("/hero-reject")
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("reject", slug, heroRuntimeOpts{RejectReason: reason})
}

func (m model) beginHeroCancel() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if errMsg := m.validateOrchestratorPreconditions(); errMsg != "" {
		m = m.setStatusResult(false, "/hero-cancel", errMsg)
		return m, nil
	}
	return m.beginHeroCancelExecute("")
}

func (m model) beginHeroCancelExecute(reason string) (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if errMsg := m.validateOrchestratorPreconditions(); errMsg != "" {
		m, _ = m.enterConversation()
		m.convError = errMsg
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-cancel")
	if !ok {
		m, _ = m.enterConversation()
		m.convError = defaultModelRequiredMessage("/hero-cancel")
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("cancel", slug, heroRuntimeOpts{CancelReason: strings.TrimSpace(reason)})
}

func (m model) beginHeroFinish() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if errMsg := m.validateOrchestratorPreconditions(); errMsg != "" {
		m = m.setStatusResult(false, "/hero-finish", errMsg)
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-finish")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("finish", slug, heroRuntimeOpts{})
}

func (m model) beginHeroContinue(extra int) (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	return m.beginHeroContinueExecute(extra)
}

func (m model) beginHeroContinueExecute(extra int) (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if extra <= 0 {
		m, _ = m.enterConversation()
		m.convError = "Extra iterations must be a positive number (e.g. /hero-continue 2)."
		return m, nil
	}
	if m.svc == nil {
		m = m.setStatusResult(false, "/hero-continue", "cycle service unavailable")
		return m, nil
	}
	st, err := m.svc.Status()
	if err != nil || st.CycleNumber == 0 {
		m = m.setStatusResult(false, "/hero-continue", noActiveCycleForStartMessage())
		return m, nil
	}
	if escalatedStage(st) == "" {
		m = m.setStatusResult(false, "/hero-continue", noEscalatedStageMessage())
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-continue")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("continue", slug, heroRuntimeOpts{ContinueExtra: extra})
}

func (m model) beginHeroBack() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if m.svc == nil {
		m = m.setStatusResult(false, "/hero-back", "cycle service unavailable")
		return m, nil
	}
	st, err := m.svc.Status()
	if err != nil || st.CycleNumber == 0 {
		m = m.setStatusResult(false, "/hero-back", noActiveCycleForStartMessage())
		return m, nil
	}
	if !strings.EqualFold(pendingApprovalStage(st), "judge") {
		m = m.setStatusResult(false, "/hero-back", noJudgePendingApprovalMessage())
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-back")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("back", slug, heroRuntimeOpts{})
}

func noActiveCycleForStartMessage() string {
	return "No active cycle. Run /hero-new to start."
}

func noPendingApprovalMessage() string {
	return "No stage pending approval."
}

func noEscalatedStageMessage() string {
	return "No escalated stage. Run /hero-continue only when a stage is Escalated."
}

func noJudgePendingApprovalMessage() string {
	return "No Judge stage pending approval for /hero-back."
}

func (m model) beginHeroApprove() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if m.svc == nil {
		m = m.setStatusResult(false, "/hero-approve", "cycle service unavailable")
		return m, nil
	}
	st, err := m.svc.Status()
	if err != nil || st.CycleNumber == 0 {
		m = m.setStatusResult(false, "/hero-approve", noActiveCycleForStartMessage())
		return m, nil
	}
	if pendingApprovalStage(st) == "" {
		m = m.setStatusResult(false, "/hero-approve", noPendingApprovalMessage())
		return m, nil
	}
	m, cmd, slug, ok := m.orchestratorExecuteModel("/hero-approve")
	if !ok {
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("approve", slug, heroRuntimeOpts{})
}

func (m model) cyclesCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		text, err := formatCyclesList(svc)
		if err != nil {
			return actionResultMsg{err: err, title: "/hero-cycles"}
		}
		return actionResultMsg{success: text, title: "/hero-cycles"}
	}
}

func (m model) todosCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		text, err := formatTodosList(svc.ProjectDir)
		if err != nil {
			return actionResultMsg{err: err, title: "/hero-todos"}
		}
		return actionResultMsg{success: text, title: "/hero-todos"}
	}
}

func dispatchPromptMsg(svc *cycle.Service, label, prompt, modelSlug, mode, harnessID string) tea.Msg {
	adapter := svc.Harness
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	// Prefer an explicitly injected adapter (tests / hero run). Otherwise route by
	// active chat harness via the multi-harness registry (Codex/OpenCode/Cursor).
	if adapter == nil && svc != nil && svc.Registry != nil && harnessID != "" {
		if a, err := svc.Registry.Adapter(harnessID); err == nil {
			adapter = a
		}
	}
	if adapter == nil {
		adapter = cursoradapter.NewAdapter(svc.ProjectDir)
	}
	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: svc.ProjectDir,
		Prompt:     prompt,
		Model:      modelSlug,
		Mode:       mode,
	})
	if err != nil {
		slog.Error("tui command dispatch failed", "command", label, "error", err)
		return actionResultMsg{
			title: label,
			err:   fmt.Errorf("dispatch failed for %s; run the same command in Cursor chat", label),
		}
	}
	if !res.Dispatched {
		msg := res.Message
		if msg == "" {
			msg = fmt.Sprintf("dispatch unavailable for %s", label)
		}
		slog.Info("tui command dispatch unavailable", "command", label, "message", msg)
		return actionResultMsg{
			title: label,
			err:   fmt.Errorf("%s; run %s in Cursor chat", msg, label),
		}
	}
	slog.Info("tui command dispatched", "command", label)
	success := res.Message
	if success == "" {
		success = fmt.Sprintf("%s dispatched.", label)
	}
	return actionResultMsg{title: label, success: success}
}

func (m model) helpCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		path := filepath.Join(svc.ProjectDir, cursoradapter.WorkflowHelpPath)
		return actionResultMsg{success: fmt.Sprintf("See %s for Hero workflow help.", path)}
	}
}

func (m model) importCommandCmd(item paletteItem) tea.Cmd {
	svc := m.svc
	label := item.commandLabel
	path := item.commandPath
	modelSlug := m.conversationModelSlug()
	harnessID := m.conversationHarnessTool()
	mode := m.chatMode
	if mode == "" {
		mode = harness.ModeBuild
	}
	return func() tea.Msg {
		prompt, err := cursoradapter.ReadCommandPrompt(path)
		if err != nil {
			slog.Error("tui import command read failed", "path", path, "error", err)
			return actionResultMsg{err: fmt.Errorf("read command %s: %w", label, err)}
		}
		return dispatchPromptMsg(svc, label, prompt, modelSlug, mode, harnessID)
	}
}

// isDestructiveAction reports whether the palette action, when executed while
// the agent is streaming, would cancel or replace the current agent session.
func isDestructiveAction(a paletteAction) bool {
	switch a {
	case actionNew, actionStart, actionCancel, actionFinish, actionArchive, actionBack, actionQuit:
		return true
	}
	return false
}

func confirmMsgForAction(a paletteAction) string {
	switch a {
	case actionNew:
		return "Agent is running. /hero-new will interrupt it. Continue? [y/N]"
	case actionStart:
		return "Agent is running. /hero-start will interrupt it. Continue? [y/N]"
	case actionCancel:
		return "Agent is running. /hero-cancel will interrupt it. Continue? [y/N]"
	case actionFinish:
		return "Agent is running. /hero-finish will interrupt it. Continue? [y/N]"
	case actionArchive:
		return "Agent is running. /hero-archive will interrupt it. Continue? [y/N]"
	case actionBack:
		return "Agent is running. /hero-back will interrupt it. Continue? [y/N]"
	case actionQuit:
		return "Agent is running. Quit? [y/N]"
	}
	return "Agent is running. This will interrupt it. Continue? [y/N]"
}

// showConfirm sets the pending confirmation state and navigates to Chat so the
// footer prompt is visible.
func (m model) showConfirm(action paletteAction, actionN int, msg string) (model, tea.Cmd) {
	m.confirmPending = true
	m.confirmAction = action
	m.confirmActionN = actionN
	m.confirmMsg = msg
	if m.screen != screenConversation {
		m, _ = m.enterConversation()
	}
	return m, nil
}

// handleConfirmKey processes a key press while a confirmation dialog is active.
// Only y/Y confirms; any other key (including n, N, esc) denies.
func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		action := m.confirmAction
		actionN := m.confirmActionN
		m.confirmPending = false
		m.confirmMsg = ""

		if action == actionQuit {
			if m.heroStartBootstrapping || m.heroStartPreparing {
				m, _ = m.cancelHeroStartPreparation()
				return m, tea.Quit
			}
			// Cancel the stream first, then quit.
			return m, tea.Batch(m.cancelStreamCmd(), tea.Quit)
		}

		// Cancel the running stream, then dispatch the confirmed action once
		// the stream finishes (streaming will be false by then, so beginHero*
		// guards will not block).  We use cancelStreamCmd here; the actual
		// action is launched immediately — beginHero* checks streaming==false
		// after cancel completes because streamCancelDoneMsg clears streaming.
		// Since both run in the same goroutine-safe Update cycle, we run the
		// cancel first and schedule the action as a follow-up via a command.
		cancelCmd := m.cancelStreamCmd()
		pendingAction := action
		pendingN := actionN
		followUpCmd := func() tea.Msg {
			return confirmResumeMsg{action: pendingAction, actionN: pendingN}
		}
		return m, tea.Batch(cancelCmd, followUpCmd)

	default:
		// n, N, esc, or any other key — cancel the confirmation.
		m.confirmPending = false
		m.confirmMsg = ""
		return m, nil
	}
}

// confirmResumeMsg is dispatched after the user confirms a destructive action
// and a stream cancel has been requested. The Update loop dispatches the action
// once this message arrives (streaming may still be true at this point if the
// cancel hasn't completed, so we retry via the beginHero* which will handle it
// when the cancel finishes — but in practice the cancel goroutine is fast).
type confirmResumeMsg struct {
	action  paletteAction
	actionN int
}

// dispatchConfirmedAction executes the action that was confirmed while streaming.
// By the time this runs the cancel has been requested; streaming guards in
// beginHero* functions will block if still true (the stream finishes within
// milliseconds) — that is acceptable since the user already confirmed intent.
func (m model) dispatchConfirmedAction(action paletteAction, actionN int) (tea.Model, tea.Cmd) {
	switch action {
	case actionQuit:
		return m, tea.Quit
	case actionNew:
		return m.beginHeroNew()
	case actionStart:
		return m.beginHeroStart()
	case actionCancel:
		return m.beginHeroCancel()
	case actionFinish:
		return m.beginHeroFinish()
	case actionArchive:
		return m.beginHeroArchive()
	case actionBack:
		return m.beginHeroBack()
	}
	return m, nil
}

// parseTestKey converts human-readable bindings into Bubble Tea key messages.
func parseTestKey(s string) tea.KeyMsg {
	switch strings.ToLower(s) {
	case "/":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	case "alt+q":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}, Alt: true}
	case "alt+n":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true}
	case "alt+m":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}, Alt: true}
	case "alt+r":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}, Alt: true}
	case "alt+s":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true}
	case "alt+u":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}, Alt: true}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "alt+enter":
		return tea.KeyMsg{Type: tea.KeyEnter, Alt: true}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	case "alt+1":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true}
	case "alt+2":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true}
	case "alt+3":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true}
	case "alt+4":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}, Alt: true}
	case "alt+5":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}, Alt: true}
	case "alt+6":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}, Alt: true}
	default:
		if len(s) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(s[0])}}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}
