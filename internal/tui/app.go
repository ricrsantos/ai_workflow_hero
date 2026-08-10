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
)

type screen int

const (
	screenStatus screen = iota
	screenApprovals
	screenArtifacts
	screenCosts
	screenEvents
	screenConversation
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
	streaming         bool
	streamInterrupted bool
	convError         string
	agentMsgIndex     int
	convStreamCh      chan tea.Msg
	cursorBlinkOn     bool
}

type refreshDataMsg struct {
	status    cycle.StatusView
	metrics   cycle.MetricsView
	events    cycle.EventsView
	artifacts cycle.ArtifactsView
	err       error
}

type actionResultMsg struct {
	err     error
	success string
	title   string // optional panel title for multiline output
}

func newModel(svc *cycle.Service) model {
	m := model{
		svc:        svc,
		screen:     screenStatus,
		prevScreen: screenStatus,
	}
	return m.reloadPaletteItems()
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

	case streamDeltaMsg, executeDoneMsg, streamCancelDoneMsg:
		if m.screen == screenConversation {
			return m.handleConversationMsg(msg)
		}
		return m, nil

	case blinkCursorMsg:
		if m.screen != screenConversation || m.streaming {
			return m, nil
		}
		m.cursorBlinkOn = !m.cursorBlinkOn
		return m, blinkCursorCmd()

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
		return "Loading Hero TUI..."
	}
	return m.renderFrame()
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "/":
		m.prevScreen = m.screen
		m.screen = screenPalette
		m.paletteFilter = ""
		m.paletteIndex = 0
		m.paletteOffset = 0
		m = m.reloadPaletteItems()
		return m, nil
	case "ctrl+r", "f5":
		return m, m.refreshCmd()
	case "1":
		m.screen = screenStatus
		return m, nil
	case "2":
		m.screen = screenApprovals
		return m, nil
	case "3":
		m.screen = screenArtifacts
		return m, nil
	case "4":
		m.screen = screenCosts
		return m, nil
	case "5":
		m.screen = screenEvents
		return m, nil
	case "6":
		return m.enterConversation()
	case "a":
		if m.screen == screenApprovals {
			return m.beginAction("/hero-approve", m.approveCmd())
		}
	case "r":
		if m.screen == screenApprovals {
			return m.beginAction("/hero-reject", m.rejectCmd())
		}
	case "d":
		return m.beginAction("dispatch", m.dispatchCmd())
	case "f":
		if m.screen == screenApprovals {
			return m.beginAction("/hero-finish", m.finishCmd())
		}
	case "c":
		if m.screen == screenApprovals {
			return m.beginAction("/hero-cancel", m.cancelCmd())
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
		m.screen = item.screen
		return m, nil
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
		m.screen = screenApprovals
		return m.beginAction("/hero-approve", m.approveCmd())
	case actionReject:
		m.screen = screenApprovals
		return m.beginAction("/hero-reject", m.rejectCmd())
	case actionNew:
		return m.beginAction("/hero-new", m.newCycleCmd())
	case actionStart:
		return m.beginAction("/hero-start", m.startCmd())
	case actionSync:
		return m.beginAction("/hero-sync", m.heroAssetCmd("sync"))
	case actionStatus:
		m.screen = screenStatus
		return m.beginAction("/hero-status", m.statusCmd())
	case actionContinue:
		return m.beginAction("/hero-continue", m.continueCmd())
	case actionBack:
		return m.beginAction("/hero-back", m.heroAssetCmd("back"))
	case actionCancel:
		return m.beginAction("/hero-cancel", m.cancelCmd())
	case actionFinish:
		return m.beginAction("/hero-finish", m.finishCmd())
	case actionArchive:
		return m.beginAction("/hero-archive", m.archiveCmd())
	case actionResume:
		return m.beginAction("/hero-resume", m.resumeCmd())
	case actionCycles:
		return m.beginAction("/hero-cycles", m.cyclesCmd())
	case actionTodos:
		return m.beginAction("/hero-todos", m.todosCmd())
	case actionHelp:
		return m.beginAction("/hero-help", m.helpCmd())
	case actionImportCommand:
		return m.beginAction(item.commandLabel, m.importCommandCmd(item))
	}
	return m, nil
}

func (m model) refreshCmd() tea.Cmd {
	svc := m.svc
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
		if mErr != nil && eErr != nil && aErr != nil && st.CycleNumber == 0 {
			return refreshDataMsg{err: mErr}
		}
		return refreshDataMsg{
			status: st, metrics: metrics, events: events, artifacts: artifacts,
		}
	}
}

func (m model) approveCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		if err := svc.Approve("", ""); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{success: "Stage approved."}
	}
}

func (m model) rejectCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		if err := svc.Reject(""); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{success: "Stage rejected."}
	}
}

func (m model) cancelCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		if err := svc.Cancel(""); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{success: "Cycle cancelled."}
	}
}

func (m model) finishCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		if err := svc.Finish(""); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{success: "Cycle finished."}
	}
}

func (m model) newCycleCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		res, err := svc.NewCycle("", "")
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{
			success: fmt.Sprintf("Created cycle C%d — %s (%d stages).",
				res.Cycle.Number, res.Cycle.Title, len(res.Stages)),
		}
	}
}

func (m model) startCmd() tea.Cmd {
	return m.dispatchCmd()
}

func (m model) statusCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		st, err := svc.Status()
		if err != nil {
			return actionResultMsg{err: err}
		}
		if st.CycleNumber == 0 {
			return actionResultMsg{success: "No active cycle. Run /hero-new to start."}
		}
		return actionResultMsg{
			success: fmt.Sprintf("Cycle C%d — %s (%s)", st.CycleNumber, st.Title, st.Status),
		}
	}
}

func (m model) continueCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		if err := svc.Continue(1); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{success: "Granted +1 extra iteration(s)."}
	}
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

func (m model) heroAssetCmd(name string) tea.Cmd {
	svc := m.svc
	label := "/hero-" + name
	return func() tea.Msg {
		path := filepath.Join(svc.ProjectDir, cursoradapter.CommandsDir, "hero-"+name+".md")
		prompt, err := cursoradapter.ReadCommandPrompt(path)
		if err != nil {
			slog.Error("tui hero command read failed", "path", path, "error", err)
			return actionResultMsg{title: label, err: fmt.Errorf("read command %s: %w", label, err)}
		}
		return dispatchPromptMsg(svc, label, prompt)
	}
}

func dispatchPromptMsg(svc *cycle.Service, label, prompt string) tea.Msg {
	adapter := svc.Harness
	if adapter == nil {
		adapter = cursoradapter.NewAdapter(svc.ProjectDir)
	}
	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: svc.ProjectDir,
		Prompt:     prompt,
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

func (m model) dispatchCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		res, err := svc.Run("")
		if err != nil {
			return actionResultMsg{err: err}
		}
		msg := res.Message
		if msg == "" {
			msg = fmt.Sprintf("Dispatch for stage %q.", res.Stage)
		}
		if !res.Dispatched {
			return actionResultMsg{success: "Dispatch: " + msg}
		}
		return actionResultMsg{success: msg}
	}
}

func (m model) archiveCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		res, err := svc.Archive()
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{
			success: fmt.Sprintf("Cycle C%d archived to %s.", res.CycleNumber, res.ArchiveDir),
		}
	}
}

func (m model) resumeCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		if err := svc.Resume(0); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{success: "Cycle resumed."}
	}
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
	return func() tea.Msg {
		prompt, err := cursoradapter.ReadCommandPrompt(path)
		if err != nil {
			slog.Error("tui import command read failed", "path", path, "error", err)
			return actionResultMsg{err: fmt.Errorf("read command %s: %w", label, err)}
		}
		return dispatchPromptMsg(svc, label, prompt)
	}
}

// parseDigitKeys handles multi-char test keys like "ctrl+p".
func parseTestKey(s string) tea.KeyMsg {
	switch strings.ToLower(s) {
	case "/":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
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
	case "q":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	default:
		if len(s) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(s[0])}}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}
