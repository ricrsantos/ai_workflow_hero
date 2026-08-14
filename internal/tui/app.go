package tui

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

type screen int

const (
	screenConversation screen = iota
	screenStatus
	screenApprovals
	screenArtifacts
	screenCosts
	screenEvents
	screenPalette
	screenOutput
)

type model struct {
	svc    *cycle.Service
	width  int
	height int
	screen screen

	status    cycle.StatusView
	metrics   cycle.MetricsView
	events    cycle.EventsView
	artifacts cycle.ArtifactsView
	approvals cycle.ApprovalsView

	contentOffset int // scroll for Status/Approvals/Artifacts/Costs/Events

	// Fixed footer status bar (running / result / error).
	statusKind    statusKind
	statusLabel   string
	statusText    string
	statusStarted time.Time
	actionBusy    bool

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
	conversationStage string
	harnessSessionID  string
	transcript        []convMessage
	input             string
	inputCursor       int // rune offset into input
	streaming         bool
	streamInterrupted bool
	convError         string
	agentMsgIndex     int
	thinkingMsgIndex  int
	convStreamCh      chan tea.Msg
	chatInputFocused  bool

	// OpenCode-style chat panes: scroll offsets + wait animation.
	inputScrollOffset int
	respScrollOffset  int
	respFollowBottom  bool // auto-stick response to latest lines while streaming
	waitAnimFrame     int

	// Chat OpenCode-style controls.
	chatMode             string // harness.ModeBuild | harness.ModePlan
	chatModelSlug        string
	availableModels      []string
	pickingModel         bool
	runtimeCommandName   string // hero runtime slash body name (e.g. "new") for Chat output normalization
	runtimeModelSlug     string // explicit harness model for active runtime slash (e.g. /hero-start orchestrator)
	runtimeAgentName     string // harness agent name for active runtime slash (e.g. orchestration_agent)
	orchestrationLive    bool   // /hero-start session: follow-ups resume orchestrator model + session
	awaitingRejectReason bool   // Chat is collecting rejection feedback before Runtime Execute

	slashOverlayIndex     int  // selected row in Chat `/` autocomplete
	slashOverlayDismissed bool // Esc or insert closed the overlay until the token changes

	liveAgents []liveAgent // currently executing parent + Task subagents (Chat box)
}

type refreshDataMsg struct {
	status    cycle.StatusView
	metrics   cycle.MetricsView
	events    cycle.EventsView
	artifacts cycle.ArtifactsView
	approvals cycle.ApprovalsView
	err       error
}

type actionResultMsg struct {
	err     error
	success string
	title   string // optional panel title for multiline output
}

func newModel(svc *cycle.Service) model {
	m := model{
		svc:              svc,
		screen:           screenConversation,
		prevScreen:       screenConversation,
		chatMode:         harness.ModeBuild,
		agentMsgIndex:    -1,
		thinkingMsgIndex: -1,
		respFollowBottom: true,
		chatInputFocused: true,
	}
	if svc != nil {
		m.chatModelSlug = install.HarnessModelSlugForProject(svc.ProjectDir, "cursor")
	}
	m = m.reloadPaletteItems()
	return m.syncConversationContext()
}

func newModelWithChat(svc *cycle.Service, models []string, modelSlug, modelWarn string) model {
	m := newModel(svc)
	m.availableModels = append([]string(nil), models...)
	if strings.TrimSpace(modelSlug) != "" {
		m.chatModelSlug = strings.TrimSpace(modelSlug)
	}
	if modelWarn != "" {
		m = m.setStatusResult(false, "model", modelWarn)
	}
	return m
}

func (m model) reloadPaletteItems() model {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	m.paletteItems = buildPaletteItems(projectDir)
	return m
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
		if m.screen == screenPalette {
			m = m.ensurePaletteVisible()
		}
		if m.screen == screenOutput {
			m = m.rebuildOutputLines()
		}
		if m.screen == screenConversation {
			m = m.scrollResponse(0) // clamp to new response viewport
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
		m.status = msg.status
		m.metrics = msg.metrics
		m.events = msg.events
		m.artifacts = msg.artifacts
		m.approvals = msg.approvals
		m = m.clampContentOffset()
		slog.Debug("tui data refreshed", "cycle", m.status.CycleNumber)
		return m, nil

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

	case statusTickMsg:
		if m.statusKind != statusRunning {
			return m, nil
		}
		return m, statusTickCmd()

	case convWaitTickMsg:
		if !m.streaming {
			return m, nil
		}
		m.waitAnimFrame++
		return m, convWaitTickCmd()

	case streamDeltaMsg, executeDoneMsg, streamCancelDoneMsg:
		if m.screen == screenConversation {
			return m.handleConversationMsg(msg)
		}
		return m, nil

	case tea.KeyMsg:
		if m.screen == screenOutput {
			return m.handleOutputKey(msg)
		}
		if m.screen == screenPalette {
			return m.handlePaletteKey(msg)
		}
		if m.screen == screenConversation {
			return m.handleConversationKey(msg)
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
	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		return m, tea.Quit
	case "/":
		m.chatInputFocused = false
		m.prevScreen = m.screen
		m.screen = screenPalette
		m.paletteFilter = ""
		m.paletteIndex = 0
		m.paletteOffset = 0
		m.pickingModel = false
		m = m.reloadPaletteItems()
		return m, nil
	case "ctrl+r", "f5":
		return m, m.refreshCmd()
	case "ctrl+1", "alt+1":
		return m.enterConversation()
	case "ctrl+2", "alt+2":
		return m.goListScreen(screenStatus)
	case "ctrl+3", "alt+3":
		return m.goListScreen(screenApprovals)
	case "ctrl+4", "alt+4":
		return m.goListScreen(screenArtifacts)
	case "ctrl+5", "alt+5":
		return m.goListScreen(screenCosts)
	case "ctrl+6", "alt+6":
		return m.goListScreen(screenEvents)
	case "up", "ctrl+p":
		if m.screenHasContentScroll() {
			return m.scrollContent(-1), nil
		}
	case "down", "ctrl+n":
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
	case "a":
		if m.screen == screenApprovals {
			return m.beginHeroApprove()
		}
	case "r":
		if m.screen == screenApprovals {
			return m.beginHeroReject()
		}
	case "f":
		if m.screen == screenApprovals {
			return m.beginHeroFinish()
		}
	case "c":
		if m.screen == screenApprovals {
			return m.beginHeroCancel()
		}
	}
	return m, nil
}

func (m model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m = m.closePalette()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+q", "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5", "ctrl+6",
		"alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6",
		"ctrl+r", "f5":
		// Leave palette chrome before global navigation / refresh / quit.
		m.pickingModel = false
		m.paletteFilter = ""
		m.paletteIndex = 0
		m.paletteOffset = 0
		return m.handleKey(msg)
	case "up", "ctrl+p":
		if m.paletteIndex > 0 {
			m.paletteIndex--
		}
		m = m.ensurePaletteVisible()
		return m, nil
	case "down", "ctrl+n":
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
	case "enter":
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
	return m, tea.Batch(cmd, statusTickCmd())
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
		return m.selectChatModel(item.label)
	case actionQuit:
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
	case actionHelp:
		return m.beginAction("/hero-help", m.helpCmd())
	case actionImportCommand:
		m, cmd, ok := m.ensureDefaultModel(item.commandLabel)
		if !ok {
			return m, cmd
		}
		return m.beginAction(item.commandLabel, m.importCommandCmd(item))
	}
	return m, nil
}

func (m model) goListScreen(s screen) (model, tea.Cmd) {
	m.chatInputFocused = false
	if m.screen != s {
		m.contentOffset = 0
	}
	m.screen = s
	return m, m.refreshCmd()
}

func (m model) refreshCmd() tea.Cmd {
	svc := m.svc
	if svc == nil {
		return nil
	}
	return func() tea.Msg {
		st, err := svc.Status()
		if err != nil {
			return refreshDataMsg{err: err}
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
			status: st, metrics: metrics, events: events, artifacts: artifacts, approvals: approvals,
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
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	if m.svc == nil {
		m = m.setStatusResult(false, "/hero-start", "cycle service unavailable")
		return m, nil
	}
	st, err := m.svc.Status()
	if err != nil || st.CycleNumber == 0 {
		m = m.setStatusResult(false, "/hero-start", noActiveCycleForStartMessage())
		return m, nil
	}
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-start")
	if !ok {
		return m, cmd
	}
	if err := m.svc.SyncCycleConfig(); err != nil {
		m = m.setStatusResult(false, "/hero-start", err.Error())
		return m, nil
	}
	return m.beginHeroRuntimeConversation("start", slug, heroRuntimeOpts{})
}

func (m model) beginHeroSync() (model, tea.Cmd) {
	if m.streaming {
		m = m.setStatusBusyBlocked()
		return m, nil
	}
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-sync")
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-status")
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-archive")
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-resume")
	if !ok {
		if cycleN > 0 {
			m, _ = m.enterConversation()
			m.convError = defaultModelRequiredMessage("/hero-resume")
			return m, nil
		}
		return m, cmd
	}
	return m.beginHeroRuntimeConversation("resume", slug, heroRuntimeOpts{ResumeCycleNumber: cycleN})
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
	m, cmd, _, ok := m.defaultExecuteModel("/hero-reject")
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
		content: "Enter rejection feedback below, then press Enter.",
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-reject")
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-cancel")
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-finish")
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-continue")
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-back")
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
	m, cmd, slug, ok := m.defaultExecuteModel("/hero-approve")
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

func dispatchPromptMsg(svc *cycle.Service, label, prompt, modelSlug, mode string) tea.Msg {
	adapter := svc.Harness
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
		return dispatchPromptMsg(svc, label, prompt, modelSlug, mode)
	}
}

// parseDigitKeys handles multi-char test keys like "ctrl+p".
func parseTestKey(s string) tea.KeyMsg {
	switch strings.ToLower(s) {
	case "/":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	case "ctrl+q":
		return tea.KeyMsg{Type: tea.KeyCtrlQ}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
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
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	case "ctrl+1", "alt+1":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true}
	case "ctrl+2", "alt+2":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true}
	case "ctrl+3", "alt+3":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true}
	case "ctrl+4", "alt+4":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}, Alt: true}
	case "ctrl+5", "alt+5":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}, Alt: true}
	case "ctrl+6", "alt+6":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}, Alt: true}
	default:
		if len(s) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(s[0])}}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}
