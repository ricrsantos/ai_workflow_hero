package tui

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

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
	screenPalette
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

	flash    string
	flashErr bool

	paletteFilter string
	paletteIndex  int
	paletteItems  []paletteItem
	prevScreen    screen
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
		return m, nil

	case refreshDataMsg:
		if msg.err != nil {
			m.flash = msg.err.Error()
			m.flashErr = true
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
		if msg.err != nil {
			m.flash = msg.err.Error()
			m.flashErr = true
			slog.Error("tui action failed", "error", msg.err)
		} else if msg.success != "" {
			m.flash = msg.success
			m.flashErr = false
			slog.Info("tui action ok", "message", msg.success)
		}
		return m, m.refreshCmd()

	case tea.KeyMsg:
		if m.screen == screenPalette {
			return m.handlePaletteKey(msg)
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
		m = m.reloadPaletteItems()
		return m, nil
	case "ctrl+r", "f5":
		m.flash = ""
		return m, m.refreshCmd()
	case "1":
		m.screen = screenStatus
		m.flash = ""
		return m, nil
	case "2":
		m.screen = screenApprovals
		m.flash = ""
		return m, nil
	case "3":
		m.screen = screenArtifacts
		m.flash = ""
		return m, nil
	case "4":
		m.screen = screenCosts
		m.flash = ""
		return m, nil
	case "5":
		m.screen = screenEvents
		m.flash = ""
		return m, nil
	case "a":
		if m.screen == screenApprovals {
			return m, m.approveCmd()
		}
	case "r":
		if m.screen == screenApprovals {
			return m, m.rejectCmd()
		}
	case "d":
		return m, m.dispatchCmd()
	case "f":
		if m.screen == screenApprovals {
			return m, m.finishCmd()
		}
	case "c":
		if m.screen == screenApprovals {
			return m, m.cancelCmd()
		}
	}
	return m, nil
}

func (m model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = m.prevScreen
		m.paletteFilter = ""
		m.paletteIndex = 0
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "up", "ctrl+p":
		if m.paletteIndex > 0 {
			m.paletteIndex--
		}
		return m, nil
	case "down", "ctrl+n":
		items := m.filteredPaletteItems()
		if m.paletteIndex < len(items)-1 {
			m.paletteIndex++
		}
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
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			m.paletteFilter += string(msg.Runes)
			m.paletteIndex = 0
		}
		return m, nil
	}
}

func (m model) runPaletteAction(item paletteItem) (model, tea.Cmd) {
	switch item.action {
	case actionGoScreen:
		m.screen = item.screen
		m.paletteFilter = ""
		m.paletteIndex = 0
		return m, nil
	case actionApprove:
		m.screen = screenApprovals
		return m, m.approveCmd()
	case actionReject:
		m.screen = screenApprovals
		return m, m.rejectCmd()
	case actionCancel:
		return m, m.cancelCmd()
	case actionFinish:
		return m, m.finishCmd()
	case actionDispatch:
		return m, m.dispatchCmd()
	case actionArchive:
		return m, m.archiveCmd()
	case actionResume:
		return m, m.resumeCmd()
	case actionHelp:
		return m, m.helpCmd()
	case actionImportCommand:
		m.flash = fmt.Sprintf("→ Running %s via markdown expansion…", item.commandLabel)
		m.flashErr = false
		return m, m.importCommandCmd(item)
	case actionRefresh:
		m.flash = ""
		return m, m.refreshCmd()
	case actionQuit:
		return m, tea.Quit
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
		adapter := svc.Harness
		if adapter == nil {
			adapter = cursoradapter.NewAdapter(svc.ProjectDir)
		}
		res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
			ProjectDir: svc.ProjectDir,
			Prompt:     prompt,
		})
		if err != nil {
			slog.Error("tui import command dispatch failed", "command", label, "error", err)
			return actionResultMsg{
				err: fmt.Errorf("dispatch failed for %s; run the same command in Cursor chat", label),
			}
		}
		if !res.Dispatched {
			msg := res.Message
			if msg == "" {
				msg = fmt.Sprintf("dispatch unavailable for %s", label)
			}
			slog.Info("tui import command dispatch unavailable", "command", label, "message", msg)
			return actionResultMsg{
				err: fmt.Errorf("%s; run %s in Cursor chat", msg, label),
			}
		}
		slog.Info("tui import command dispatched", "command", label)
		success := res.Message
		if success == "" {
			success = fmt.Sprintf("%s dispatched.", label)
		}
		return actionResultMsg{success: success}
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
