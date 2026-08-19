package tui

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	herodebug "github.com/ricrsantos/ai_workflow_hero/internal/common/debug"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

type convRole string

const (
	convRoleUser     convRole = "user"
	convRoleAgent    convRole = "agent"
	convRoleThinking convRole = "thinking"
	convRoleTool     convRole = "tool"
	convRoleWarning  convRole = "warning"
	convRoleActivity convRole = "activity"
)

// Visible content rows inside the OpenCode-style chat panes (excluding status row).
const (
	chatInputVisibleLines = 2
	chatResponseMinLines  = 2
	chatResponseMaxLines  = 24
)

var waitAnimFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type convMessage struct {
	role        convRole
	content     string
	agentName   string
	modelSlug   string
	callID      string
	interrupted bool
	failed      bool
}

type streamDeltaMsg struct {
	delta harness.StreamDelta
}

type harnessPermissionRequestMsg struct {
	req    harness.PermissionRequest
	respCh chan harness.PermissionResponse
}

type executeDoneMsg struct {
	result    *harness.ExecutionResult
	err       error
	harnessID string // harness used for this execute (session binding)
}

type streamCancelDoneMsg struct {
	err error
}

type convWaitTickMsg struct{}

func convWaitTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return convWaitTickMsg{}
	})
}

func (m model) harnessAdapter() harness.HarnessAdapter {
	if m.svc != nil && m.svc.Harness != nil {
		return m.svc.Harness
	}
	id := m.conversationHarnessTool()
	if m.svc != nil && m.svc.Registry != nil {
		if a, err := m.svc.Registry.Adapter(id); err == nil {
			return a
		}
	}
	if m.svc != nil {
		return cursoradapter.NewAdapter(m.svc.ProjectDir)
	}
	return nil
}

func (m model) conversationHarnessTool() string {
	if h := strings.TrimSpace(m.chatHarnessID); h != "" {
		return h
	}
	if m.svc != nil && strings.TrimSpace(m.conversationStage) != "" {
		if h, err := m.svc.StageHarnessID(m.conversationStage); err == nil && strings.TrimSpace(h) != "" {
			return h
		}
	}
	if m.svc != nil {
		if hero, err := install.LoadHeroJSON(m.svc.ProjectDir); err == nil {
			h, _ := install.GetFreechatDefault(hero)
			if strings.TrimSpace(h) != "" {
				return h
			}
		}
	}
	return "cursor"
}

func (m model) syncConversationContext() model {
	if m.svc == nil {
		return m
	}
	stage, sessionID, err := m.svc.ConversationContext()
	if err != nil {
		slog.Debug("tui conversation context unavailable", "error", err)
		m.conversationStage = ""
		// Keep harnessSessionID for freechat / orchestrator resume within this TUI session.
		return m
	}
	m.conversationStage = stage
	live := strings.TrimSpace(m.harnessSessionID)
	stored := strings.TrimSpace(sessionID)
	if live != "" {
		if m.researchLive {
			// Discover and orchestrator sessions are stored separately during Research.
			return m
		}
		// The TUI orchestrator session spans stages. Never replace a live id with
		// an empty (or different) per-stage SQLite value — that dropped session
		// from the Chat header and forced follow-ups into a fresh agent with no context.
		if stage != "" && live != stored {
			if err := m.svc.SetStageHarnessSessionID(stage, live); err != nil {
				slog.Debug("tui copy harness session to stage failed", "error", err)
			}
		}
		return m
	}
	m.harnessSessionID = stored
	if m.svc != nil && stage != "" {
		if h, err := m.svc.StageHarnessID(stage); err == nil {
			m.harnessSessionHarnessID = strings.TrimSpace(h)
		}
	}
	return m
}

// harnessSessionIDForPair returns sessionID only when it belongs to pairHarness (PRD §4.11).
func (m model) harnessSessionIDForPair(stageName, pairHarness string) string {
	sid := strings.TrimSpace(m.harnessSessionID)
	pairHarness = strings.TrimSpace(strings.ToLower(pairHarness))
	if sid == "" || pairHarness == "" {
		return sid
	}
	if m.svc != nil {
		stage := strings.TrimSpace(stageName)
		if stage != "" {
			if h, err := m.svc.StageHarnessID(stage); err == nil {
				h = strings.TrimSpace(strings.ToLower(h))
				if h != "" && h != pairHarness {
					return ""
				}
			}
		}
	}
	if h := strings.TrimSpace(strings.ToLower(m.harnessSessionHarnessID)); h != "" && h != pairHarness {
		return ""
	}
	return sid
}

// persistHarnessSession stores the harness session on the in-memory model and on
// the active SQLite stage (even when /hero-start cleared conversationStage).
func (m model) persistHarnessSession(sessionID, harnessID string) model {
	sessionID = strings.TrimSpace(sessionID)
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if harnessID == "" {
		harnessID = strings.TrimSpace(strings.ToLower(m.conversationHarnessTool()))
	}
	if sessionID == "" {
		return m
	}
	agent := strings.TrimSpace(m.runtimeAgentName)
	if agent == agentDiscover {
		m.researchSessionID = sessionID
		m.harnessSessionID = sessionID
		m.conversationStage = stageResearch
		if m.svc != nil {
			if err := m.svc.SetStageHarnessSessionID(stageResearch, sessionID); err != nil {
				slog.Error("tui persist harness session failed", "error", err)
			}
		}
		return m
	}
	if m.orchestrationLive {
		m.orchestrationSessionID = sessionID
	}
	if m.researchLive {
		// Orchestrator result while Research is the live Chat session — keep DISC id.
		return m
	}
	m.harnessSessionID = sessionID
	m.harnessSessionHarnessID = harnessID
	if m.svc == nil {
		return m
	}
	stage := strings.TrimSpace(m.conversationStage)
	if stage == "" && m.orchestrationLive {
		if s, err := m.svc.ActiveRunStage(); err == nil {
			stage = s
			m.conversationStage = s
		}
	}
	if stage != "" {
		if err := m.svc.SetStageHarnessSessionID(stage, sessionID); err != nil {
			slog.Error("tui persist harness session failed", "error", err)
		}
	}
	return m
}

func (m model) defaultHarnessModelSlug() string {
	return strings.TrimSpace(m.chatModelSlug)
}

func (m model) conversationModelSlug() string {
	if slug := strings.TrimSpace(m.runtimeModelSlug); slug != "" {
		return slug
	}
	return m.defaultHarnessModelSlug()
}

func (m model) runtimeExecuteModelSlug() string {
	return m.conversationModelSlug()
}

func (m model) conversationModelLabel() string {
	if slug := m.conversationModelSlug(); slug != "" {
		return slug
	}
	return "not set"
}

// responseSpeakerHeader is the green-pane origin label: [QA - composer-2.5], [HARN - grok-4.6].
func (m model) responseSpeakerHeader() string {
	name := strings.TrimSpace(m.runtimeAgentName)
	model := m.conversationModelSlug()
	if n := len(m.liveAgents); n > 0 {
		a := m.liveAgents[n-1]
		name = a.Name
		if slug := strings.TrimSpace(a.Model); slug != "" {
			model = slug
		}
		return formatAgentHeader(name, model, m.conversationHarnessTool())
	}
	turn := m.latestAgentTurn()
	for i := len(turn) - 1; i >= 0; i-- {
		msg := turn[i]
		if strings.TrimSpace(msg.agentName) == "" && strings.TrimSpace(msg.modelSlug) == "" {
			continue
		}
		if n := strings.TrimSpace(msg.agentName); n != "" {
			name = n
		}
		if slug := strings.TrimSpace(msg.modelSlug); slug != "" {
			model = slug
		}
		break
	}
	return formatAgentHeader(name, model, m.conversationHarnessTool())
}

func (m model) enterConversation() (model, tea.Cmd) {
	m.screen = screenConversation
	m.chatInputFocused = true
	m = m.syncConversationContext()
	m = m.clampInputCursor()
	return m, nil
}

func (m model) resetChatSession() model {
	m.transcript = nil
	m.harnessSessionID = ""
	m.harnessSessionHarnessID = ""
	m.conversationStage = ""
	m.orchestrationLive = false
	m.researchLive = false
	m.orchestrationSessionID = ""
	m.researchSessionID = ""
	m.awaitingRejectReason = false
	m.runtimeCommandName = ""
	m.runtimeModelSlug = ""
	m.runtimeAgentName = ""
	m.liveAgents = nil
	m.convError = ""
	m.streamInterrupted = false
	m.agentMsgIndex = -1
	m.thinkingMsgIndex = -1
	m.respScrollOffset = 0
	m.respFollowBottom = true
	m.waitAnimFrame = 0
	m.contextUsedTokens = 0
	m = m.clearChatInput()
	if m.svc != nil {
		stage, _, err := m.svc.ConversationContext()
		if err == nil && stage != "" {
			if err := m.svc.SetStageHarnessSessionID(stage, ""); err != nil {
				slog.Debug("tui clear harness session failed", "error", err)
			}
		}
	}
	m = m.syncConversationContext()
	return m
}

// heroRuntimeOpts carries command-specific context for Runtime Execute preambles.
type heroRuntimeOpts struct {
	RejectReason      string
	ContinueExtra     int // 0 = default 1
	CancelReason      string
	ResumeCycleNumber int // 0 = latest non-archived
}

func usesOrchestratorRuntime(cmdName string) bool {
	switch cmdName {
	case "start", "approve", "reject", "cancel", "finish", "continue", "back",
		"sync", "status", "archive", "resume":
		return true
	default:
		return false
	}
}

// beginHeroRuntimeConversation opens Chat and executes a Hero runtime command markdown
// (same body as Cursor slash expansion). modelSlug is optional; when empty, uses the
// conversation model (YAML orchestrator, discover agent, or /hero-model default).
func (m model) beginHeroRuntimeConversation(cmdName, modelSlug string, opts heroRuntimeOpts) (model, tea.Cmd) {
	if m.svc == nil {
		m, _ = m.enterConversation()
		m.convError = "cycle service unavailable"
		return m, nil
	}
	label := "/hero-" + cmdName
	if cmdName == "reject" && strings.TrimSpace(opts.RejectReason) != "" {
		label = "/hero-reject: " + strings.TrimSpace(opts.RejectReason)
	}
	if cmdName == "continue" {
		extra := opts.ContinueExtra
		if extra <= 0 {
			extra = 1
		}
		if extra != 1 {
			label = fmt.Sprintf("/hero-continue %d", extra)
		}
	}
	if cmdName == "cancel" && strings.TrimSpace(opts.CancelReason) != "" {
		label = "/hero-cancel: " + strings.TrimSpace(opts.CancelReason)
	}
	if cmdName == "resume" && opts.ResumeCycleNumber > 0 {
		label = fmt.Sprintf("/hero-resume %d", opts.ResumeCycleNumber)
	}
	cmdPath := filepath.Join(m.svc.ProjectDir, cursoradapter.CommandsDir, "hero-"+cmdName+".md")
	cmdBody, err := cursoradapter.ReadCommandPrompt(cmdPath)
	if err != nil {
		slog.Error("tui hero runtime command read failed", "path", cmdPath, "error", err)
		m, _ = m.enterConversation()
		m.convError = fmt.Errorf("read command %s: %w", label, err).Error()
		return m, nil
	}
	m, _ = m.enterConversation()
	m.conversationStage = ""
	m.harnessSessionID = ""
	m.harnessSessionHarnessID = ""
	m.runtimeCommandName = cmdName
	m.runtimeModelSlug = strings.TrimSpace(modelSlug)
	m.runtimeAgentName = ""
	m.orchestrationLive = cmdName == "start"
	if cmdName == "start" {
		m.researchLive = false
		m.orchestrationSessionID = ""
		m.researchSessionID = ""
	}

	var executePrompt string
	if usesOrchestratorRuntime(cmdName) {
		composite, err := orchestratorRuntimePrompt(m.svc.ProjectDir, cmdBody)
		if err != nil {
			slog.Error("tui orchestration runtime prompt failed", "cmd", cmdName, "error", err)
			m.convError = err.Error()
			return m, nil
		}
		executePrompt = tuiRuntimeCommandPrompt(cmdName, composite, opts)
		m = m.withRuntimeAgent(agentOrchestration)
	} else {
		executePrompt = tuiRuntimeCommandPrompt(cmdName, cmdBody, opts)
	}

	m = m.beginConversationExecute(label, executePrompt)
	return m, tea.Batch(waitConvMsg(m.convStreamCh), convWaitTickCmd())
}

func orchestratorRuntimePrompt(projectDir, cmdBody string) (string, error) {
	agentPath := filepath.Join(projectDir, cursoradapter.AgentsDir, "orchestration_agent.md")
	agentBody, err := cursoradapter.ReadAgentPrompt(agentPath)
	if err != nil {
		return "", fmt.Errorf("read orchestration_agent: %w", err)
	}
	composite := strings.TrimSpace(agentBody)
	if body := strings.TrimSpace(cmdBody); body != "" {
		if composite != "" {
			composite += "\n\n---\n\n"
		}
		composite += body
	}
	return composite, nil
}

func (m model) beginConversationExecute(userLabel, executePrompt string) model {
	m.streamInterrupted = false
	m.convError = ""
	m.streaming = true
	m.waitAnimFrame = 0
	m.respFollowBottom = true
	m.respScrollOffset = 0
	m.chatInputFocused = false
	parentName := strings.TrimSpace(m.runtimeAgentName)
	parentModel := m.conversationModelSlug()
	m.liveAgents = []liveAgent{{
		Name:  parentName,
		Label: agentShortLabel(parentName),
		Model: parentModel,
	}}
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: userLabel})
	m.transcript = append(m.transcript, convMessage{
		role:      convRoleAgent,
		content:   "",
		agentName: parentName,
		modelSlug: parentModel,
	})
	m.agentMsgIndex = len(m.transcript) - 1
	m.thinkingMsgIndex = -1

	ch := make(chan tea.Msg, 32)
	m.convStreamCh = ch
	m.startConversationExecute(executePrompt, ch)
	return m
}

func (m model) handleConversationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	if m.streaming {
		switch s {
		case "ctrl+c":
			return m, m.cancelStreamCmd()
		case "ctrl+q":
			return m.showConfirm(actionQuit, 0, "Agent is running. Quit? [y/N]")
		case "ctrl+1", "alt+1", "ctrl+2", "alt+2", "ctrl+3", "alt+3",
			"ctrl+4", "alt+4", "ctrl+5", "alt+5":
			// Allow screen navigation while streaming; the goroutine keeps running.
			return m.handleKey(msg)
		case "up", "ctrl+p":
			m = m.scrollResponse(-1)
			return m, nil
		case "down", "ctrl+n":
			m = m.scrollResponse(1)
			return m, nil
		case "pgup":
			m = m.scrollResponse(-m.responseVisibleLines(m.contentAreaHeight()))
			return m, nil
		case "pgdown":
			m = m.scrollResponse(m.responseVisibleLines(m.contentAreaHeight()))
			return m, nil
		default:
			// Other keys are ignored while waiting for the agent.
			return m, nil
		}
	}

	m.chatInputFocused = true

	// Global shortcuts (modifier+key) work even while typing in chat.
	// `/` is NOT global here — it stays in the composer (Cursor-style overlay).
	switch s {
	case "ctrl+q", "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5",
		"alt+1", "alt+2", "alt+3", "alt+4", "alt+5",
		"ctrl+r", "f5":
		return m.handleKey(msg)
	}

	// Newline keys are handled before the slash overlay so they never select
	// an item. Enter remains send (or overlay insert/execute).
	if isComposerNewlineKey(s) {
		return m.insertComposerNewline(), nil
	}

	if m.chatSlashOverlayActive() {
		switch s {
		case "up", "ctrl+p":
			if m.slashOverlayIndex > 0 {
				m.slashOverlayIndex--
			}
			return m, nil
		case "down", "ctrl+n":
			items := m.filteredChatSlashItems()
			if m.slashOverlayIndex < len(items)-1 {
				m.slashOverlayIndex++
			}
			return m, nil
		case "enter", "tab":
			return m.applyChatSlashSelection()
		case "esc":
			m.slashOverlayDismissed = true
			return m, nil
		}
	}

	switch s {
	case "esc":
		if strings.TrimSpace(m.input) != "" {
			m = m.clearChatInput()
		}
		return m, nil
	case "tab":
		if m.chatMode == harness.ModePlan {
			m.chatMode = harness.ModeBuild
		} else {
			m.chatMode = harness.ModePlan
		}
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		if strings.TrimSpace(m.input) == "" {
			return m, nil
		}
		return m.submitConversation()
	case "up", "ctrl+p":
		if m.inputScrollOffset > 0 {
			m.inputScrollOffset--
			return m, nil
		}
		m = m.scrollResponse(-1)
		return m, nil
	case "down", "ctrl+n":
		maxIn := m.maxInputScroll()
		if m.inputScrollOffset < maxIn {
			m.inputScrollOffset++
			return m, nil
		}
		m = m.scrollResponse(1)
		return m, nil
	case "pgup":
		m = m.scrollResponse(-m.responseVisibleLines(m.contentAreaHeight()))
		return m, nil
	case "pgdown":
		m = m.scrollResponse(m.responseVisibleLines(m.contentAreaHeight()))
		return m, nil
	case "left":
		if m.inputCursor > 0 {
			m.inputCursor--
		}
		m = m.ensureInputCaretVisible()
		return m, nil
	case "right":
		if m.inputCursor < runeLen(m.input) {
			m.inputCursor++
		}
		m = m.ensureInputCaretVisible()
		return m, nil
	case "home":
		m.inputCursor = 0
		m = m.ensureInputCaretVisible()
		return m, nil
	case "end":
		m.inputCursor = runeLen(m.input)
		m = m.ensureInputCaretVisible()
		return m, nil
	case "backspace":
		prev := chatSlashToken(m.input)
		m = m.deleteRuneBeforeCursor()
		m = m.afterChatInputEdit(prev)
		m = m.ensureInputCaretVisible()
		return m, nil
	case "delete":
		prev := chatSlashToken(m.input)
		m = m.deleteRuneAtCursor()
		m = m.afterChatInputEdit(prev)
		m = m.ensureInputCaretVisible()
		return m, nil
	default:
		if len(msg.Runes) == 0 || msg.Alt {
			return m, nil
		}
		prev := chatSlashToken(m.input)
		m = m.insertRunesAtCursor(msg.Runes)
		m = m.afterChatInputEdit(prev)
		m = m.ensureInputCaretVisible()
		return m, nil
	}
}

func runeLen(s string) int {
	return len([]rune(s))
}

func (m model) clampInputCursor() model {
	n := runeLen(m.input)
	if m.inputCursor < 0 {
		m.inputCursor = 0
	}
	if m.inputCursor > n {
		m.inputCursor = n
	}
	return m
}

func (m model) insertRunesAtCursor(rs []rune) model {
	runes := []rune(m.input)
	cur := m.inputCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	out := make([]rune, 0, len(runes)+len(rs))
	out = append(out, runes[:cur]...)
	out = append(out, rs...)
	out = append(out, runes[cur:]...)
	m.input = string(out)
	m.inputCursor = cur + len(rs)
	return m
}

func (m model) deleteRuneBeforeCursor() model {
	runes := []rune(m.input)
	if m.inputCursor <= 0 || len(runes) == 0 {
		return m
	}
	cur := m.inputCursor
	m.input = string(append(runes[:cur-1], runes[cur:]...))
	m.inputCursor = cur - 1
	return m
}

func (m model) deleteRuneAtCursor() model {
	runes := []rune(m.input)
	if m.inputCursor >= len(runes) {
		return m
	}
	cur := m.inputCursor
	m.input = string(append(runes[:cur], runes[cur+1:]...))
	return m
}

func (m model) submitConversation() (model, tea.Cmd) {
	text := strings.TrimSpace(m.input)
	if text == "" {
		if m.awaitingRejectReason {
			m.convError = "Rejection reason is required."
		}
		return m, nil
	}

	if m.awaitingRejectReason {
		m = m.clearChatInput()
		m.awaitingRejectReason = false
		m.convError = ""
		return m.beginHeroRejectExecute(text)
	}

	// Live orchestrator (e.g. waiting for /hero-approve): send the slash as a
	// follow-up. TUI Execute would gate on SQLite PendingApproval and fail
	// while the agent is still asking in Chat.
	if m.chatFollowUpControlSlash(text) {
		return m.submitChatFollowUp(text)
	}

	if reason, ok := parseHeroRejectInline(text); ok {
		m = m.clearChatInput()
		m.convError = ""
		if reason == "" {
			return m.beginHeroRejectPrompt()
		}
		return m.beginHeroRejectExecute(reason)
	}

	if reason, ok := parseHeroCancelInline(text); ok {
		m = m.clearChatInput()
		m.convError = ""
		return m.beginHeroCancelExecute(reason)
	}

	if extra, ok := parseHeroContinueInline(text); ok {
		m = m.clearChatInput()
		m.convError = ""
		return m.beginHeroContinueExecute(extra)
	}

	if cycleN, ok := parseHeroResumeInline(text); ok {
		m = m.clearChatInput()
		m.convError = ""
		return m.beginHeroResumeExecute(cycleN)
	}

	if next, cmd, ok := m.dispatchExactHeroSlash(text); ok {
		return next, cmd
	}

	return m.submitChatFollowUp(text)
}

func (m model) submitChatFollowUp(text string) (model, tea.Cmd) {
	if m.chatFollowUpControlSlash(text) && m.researchLive {
		var cmd tea.Cmd
		var slug string
		var ok bool
		m, cmd, slug, ok = m.orchestratorExecuteModel("chat")
		if !ok {
			return m, cmd
		}
		m.runtimeModelSlug = slug
		m = m.prepareOrchestratorFollowUp()
	} else if m.researchLive {
		m = m.prepareDiscoverFollowUp()
	} else if m.orchestrationLive || m.workflowAgentActive() {
		if strings.TrimSpace(m.runtimeModelSlug) == "" {
			// Runtime command paths normally set runtimeModelSlug before the
			// first turn.  An injected single-harness service is also used by
			// the C4 conversation tests and deliberately has no registry-side
			// YAML model resolution; keep its selected chat slug intact.
			if m.svc != nil && m.svc.Harness != nil {
				m.runtimeModelSlug = m.defaultHarnessModelSlug()
			}
			if strings.TrimSpace(m.runtimeModelSlug) == "" {
				var cmd tea.Cmd
				var slug string
				var ok bool
				m, cmd, slug, ok = m.orchestratorExecuteModel("chat")
				if !ok {
					return m, cmd
				}
				m.runtimeModelSlug = slug
			}
		}
		if m.runtimeAgentName == "" {
			m.runtimeAgentName = agentOrchestration
		}
	} else {
		var cmd tea.Cmd
		var ok bool
		m, cmd, ok = m.ensureDefaultModel("chat")
		if !ok {
			return m, cmd
		}
	}
	m.runtimeCommandName = ""
	m = m.syncConversationContext()
	m = m.clearChatInput()
	m = m.beginConversationExecute(text, text)
	return m, tea.Batch(waitConvMsg(m.convStreamCh), convWaitTickCmd())
}

func (m model) dispatchExactHeroSlash(text string) (model, tea.Cmd, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "/hero-approve":
		m = m.clearChatInput()
		next, cmd := m.beginHeroApprove()
		return next, cmd, true
	case "/hero-finish":
		m = m.clearChatInput()
		next, cmd := m.beginHeroFinish()
		return next, cmd, true
	case "/hero-new":
		m = m.clearChatInput()
		next, cmd := m.beginHeroNew()
		return next, cmd, true
	case "/hero-start":
		m = m.clearChatInput()
		next, cmd := m.beginHeroStart()
		return next, cmd, true
	case "/hero-sync":
		m = m.clearChatInput()
		next, cmd := m.beginHeroSync()
		return next, cmd, true
	case "/hero-status":
		m = m.clearChatInput()
		next, cmd := m.beginHeroStatus()
		return next, cmd, true
	case "/hero-archive":
		m = m.clearChatInput()
		next, cmd := m.beginHeroArchive()
		return next, cmd, true
	case "/hero-back":
		m = m.clearChatInput()
		next, cmd := m.beginHeroBack()
		return next, cmd, true
	case "/hero-model":
		m = m.clearChatInput()
		next, cmd := m.openModelPicker()
		return next, cmd, true
	case "/hero-cycles":
		m = m.clearChatInput()
		next, cmd := m.beginAction("/hero-cycles", m.cyclesCmd())
		return next, cmd, true
	case "/hero-todos":
		m = m.clearChatInput()
		next, cmd := m.beginAction("/hero-todos", m.todosCmd())
		return next, cmd, true
	case "/hero-help":
		m = m.clearChatInput()
		next, cmd := m.beginAction("/hero-help", m.helpCmd())
		return next, cmd, true
	case "/new-chat":
		m = m.clearChatInput()
		next, cmd := m.beginNewChat()
		return next, cmd, true
	default:
		return m, nil, false
	}
}

// parseHeroRejectInline returns (reason, true) when text is /hero-reject with optional reason.
func parseHeroRejectInline(text string) (string, bool) {
	lower := strings.ToLower(text)
	if lower == "/hero-reject" {
		return "", true
	}
	const prefix = "/hero-reject "
	if strings.HasPrefix(lower, prefix) {
		return strings.TrimSpace(text[len(prefix):]), true
	}
	return "", false
}

// parseHeroCancelInline returns (reason, true) when text is /hero-cancel with optional reason.
func parseHeroCancelInline(text string) (string, bool) {
	lower := strings.ToLower(text)
	if lower == "/hero-cancel" {
		return "", true
	}
	const prefix = "/hero-cancel "
	if strings.HasPrefix(lower, prefix) {
		return strings.TrimSpace(text[len(prefix):]), true
	}
	return "", false
}

// parseHeroContinueInline returns (extra, true) when text is /hero-continue with optional N.
func parseHeroContinueInline(text string) (int, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "/hero-continue" {
		return 1, true
	}
	const prefix = "/hero-continue "
	if !strings.HasPrefix(lower, prefix) {
		return 0, false
	}
	arg := strings.TrimSpace(text[len(prefix):])
	if arg == "" {
		return 1, true
	}
	var extra int
	if _, err := fmt.Sscanf(arg, "%d", &extra); err != nil || extra <= 0 {
		return 0, true // matched command but invalid N — caller shows error
	}
	return extra, true
}

// parseHeroResumeInline returns (cycleNumber, true) when text is /hero-resume with optional N.
// cycleNumber 0 means resume the latest non-archived cycle.
func parseHeroResumeInline(text string) (int, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "/hero-resume" {
		return 0, true
	}
	const prefix = "/hero-resume "
	if !strings.HasPrefix(lower, prefix) {
		return 0, false
	}
	arg := strings.TrimSpace(text[len(prefix):])
	if arg == "" {
		return 0, true
	}
	var n int
	if _, err := fmt.Sscanf(arg, "%d", &n); err != nil || n <= 0 {
		return -1, true // matched command but invalid N — caller shows error
	}
	return n, true
}

func (m model) startConversationExecute(prompt string, ch chan<- tea.Msg) {
	svc := m.svc
	stageName := m.conversationStage
	agentName := m.runtimeAgentName
	projectDir := ""
	if svc != nil {
		projectDir = svc.ProjectDir
	}
	mode := m.chatMode
	if mode == "" {
		mode = harness.ModeBuild
	}
	go func() {
		ctx := context.Background()
		resolved, err := m.resolveExecuteResolution(ctx)
		if err != nil {
			ch <- executeDoneMsg{err: err}
			return
		}
		pair := resolved.pair
		if pair.Adapter == nil {
			ch <- executeDoneMsg{err: fmt.Errorf("harness adapter unavailable")}
			return
		}
		sessionID := m.harnessSessionIDForPair(stageName, pair.HarnessID)
		if resolved.warning != "" {
			ch <- streamDeltaMsg{delta: harness.StreamDelta{Kind: harness.StreamKindText, Text: resolved.warning + "\n\n"}}
		}
		if svc != nil && strings.TrimSpace(stageName) != "" {
			if err := svc.SetStageHarnessID(stageName, pair.HarnessID); err != nil {
				slog.Debug("tui persist stage harness id failed", "error", err)
			}
		}
		req := harness.ExecuteRequest{
			ProjectDir: projectDir,
			Prompt:     prompt,
			SessionID:  sessionID,
			Stream:     true,
			Debug:      herodebug.Enabled,
			StageName:  stageName,
			AgentName:  agentName,
			Model:      pair.Model,
			Mode:       mode,
			// C5: attach the normalized property projection (freechat for
			// Chat//hero-new, YAML-derived for workflow commands; ADR-041/042).
			Properties: resolved.props,
			OnStreamDelta: func(delta harness.StreamDelta) {
				ch <- streamDeltaMsg{delta: delta}
			},
			OnPermissionRequest: func(ctx context.Context, perm harness.PermissionRequest) (harness.PermissionResponse, error) {
				respCh := make(chan harness.PermissionResponse, 1)
				select {
				case ch <- harnessPermissionRequestMsg{req: perm, respCh: respCh}:
				case <-ctx.Done():
					return harness.PermissionResponse{}, ctx.Err()
				}
				select {
				case resp := <-respCh:
					return resp, nil
				case <-ctx.Done():
					return harness.PermissionResponse{}, ctx.Err()
				}
			},
		}
		req = harness.NormalizeExecuteRequest(req)
		res, err := pair.Adapter.Execute(ctx, req)
		ch <- executeDoneMsg{result: res, err: err, harnessID: pair.HarnessID}
	}()
}

func waitConvMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m model) cancelStreamCmd() tea.Cmd {
	adapter := m.harnessAdapter()
	sessionID := m.harnessSessionID
	ch := m.convStreamCh
	return func() tea.Msg {
		var err error
		if adapter != nil {
			// Session ID is empty until Execute returns (e.g. /hero-start). Still
			// cancel the in-flight process via the adapter's pending track key.
			err = adapter.Cancel(context.Background(), sessionID)
			if err != nil {
				slog.Error("tui stream cancel failed", "error", err)
			}
		}
		if ch != nil {
			ch <- streamCancelDoneMsg{err: err}
		}
		return streamCancelDoneMsg{err: err}
	}
}

func (m model) handleConversationMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case streamDeltaMsg:
		m = m.appendStreamDelta(msg.delta)
		m = m.maybeFollowResponseBottom()
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvMsg(m.convStreamCh)
		}
		return m, nil

	case harnessPermissionRequestMsg:
		m.harnessPermissionPending = true
		m.harnessPermissionReq = msg.req
		m.harnessPermissionRespCh = msg.respCh
		m.harnessPermissionMsg = formatHarnessPermission(msg.req)
		m.insertBeforeAgent(convMessage{role: convRoleWarning, content: m.harnessPermissionMsg})
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvMsg(m.convStreamCh)
		}
		return m, nil

	case executeDoneMsg:
		m.streaming = false
		m.convStreamCh = nil
		m.chatInputFocused = true
		m.liveAgents = nil
		m.confirmPending = false
		m.confirmMsg = ""
		m = m.clearHarnessPermission()
		if !m.orchestrationLive {
			m.runtimeModelSlug = ""
			m.runtimeAgentName = ""
			m.workflowProps = nil
		}
		if msg.err != nil {
			errText := msg.err.Error()
			m.convError = errText
			m = m.setStatusResult(false, "execute", firstStatusLine(errText))
			if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
				existing := strings.TrimSpace(m.transcript[m.agentMsgIndex].content)
				if existing == "" {
					m.transcript[m.agentMsgIndex].content = "✗ " + errText
				} else {
					m.transcript[m.agentMsgIndex].content = existing + "\n✗ " + errText
				}
				m.transcript[m.agentMsgIndex].failed = true
			}
			slog.Error("tui conversation execute failed", "error", msg.err)
			return m, nil
		}
		if msg.result != nil {
			m.contextUsedTokens = msg.result.Usage.InputTokens + msg.result.Usage.OutputTokens
			if msg.result.SessionID != "" {
				m = m.persistHarnessSession(msg.result.SessionID, msg.harnessID)
			}
			// Prefer canonical result text when it is longer than streamed deltas.
			if msg.result.Output != "" && m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) && !m.transcriptHasSubagent() {
				streamed := m.transcript[m.agentMsgIndex].content
				if len(msg.result.Output) >= len(streamed) {
					m.transcript[m.agentMsgIndex].content = msg.result.Output
				}
			}
		}
		if m.runtimeCommandName == "new" && msg.err == nil && m.svc != nil {
			if _, err := m.svc.PrepareCycle(); err != nil {
				m.convError = err.Error()
				slog.Error("tui prepare cycle after hero-new failed", "error", err)
			}
		}
		m = m.maybeFollowResponseBottom()
		slog.Info("tui conversation execute complete", "stage", m.conversationStage)
		next, handoffCmd := m.maybeHandoffAfterExecute()
		if handoffCmd != nil {
			return next, tea.Batch(next.refreshCmd(), handoffCmd, convWaitTickCmd())
		}
		return next, next.refreshCmd()

	case streamCancelDoneMsg:
		m.streaming = false
		m.streamInterrupted = true
		m.convStreamCh = nil
		m.chatInputFocused = true
		m.liveAgents = nil
		m.confirmPending = false
		m.confirmMsg = ""
		m = m.clearHarnessPermission()
		if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
			m.transcript[m.agentMsgIndex].interrupted = true
		}
		slog.Info("tui conversation interrupted")
		return m, nil
	}
	return m, nil
}

func (m model) appendStreamDelta(d harness.StreamDelta) model {
	if d.Phase == harness.StreamPhaseStarted {
		m = m.addLiveAgent(d)
	}
	if d.Phase == harness.StreamPhaseCompleted {
		m = m.removeLiveAgent(d.CallID)
		if d.Kind == harness.StreamKindTool {
			return m
		}
	}
	if d.Phase == harness.StreamPhaseStarted && d.Kind == harness.StreamKindTool {
		return m
	}
	switch d.Kind {
	case harness.StreamKindPermission:
		return m
	case harness.StreamKindSession:
		if d.Metadata != nil && d.Metadata["state"] == harness.SessionStateFailed {
			m.convError = d.Text
			m = m.setStatusResult(false, "harness", firstStatusLine(d.Text))
		}
		return m
	case harness.StreamKindWarning:
		slog.Warn("harness stream warning", "harness_type", d.HarnessType, "text", d.Text)
		m.insertBeforeAgent(convMessage{role: convRoleWarning, content: d.Text})
		return m
	case harness.StreamKindActivity:
		if strings.TrimSpace(d.Text) == "" {
			return m
		}
		m.insertBeforeAgent(convMessage{role: convRoleActivity, content: d.Text})
		return m
	}
	if d.Text == "" {
		return m
	}
	if d.CallID == "" {
		d.AgentName = strings.TrimSpace(d.AgentName)
		if d.AgentName == "" {
			d.AgentName = strings.TrimSpace(m.runtimeAgentName)
		}
		if strings.TrimSpace(d.Model) == "" {
			d.Model = m.conversationModelSlug()
		}
	}
	switch d.Kind {
	case harness.StreamKindThinking:
		if d.CallID != "" {
			m.appendAttributed(convRoleThinking, d)
			return m
		}
		if m.thinkingMsgIndex >= 0 && m.thinkingMsgIndex < len(m.transcript) &&
			m.transcript[m.thinkingMsgIndex].role == convRoleThinking &&
			m.thinkingMsgIndex == m.agentMsgIndex-1 {
			m.transcript[m.thinkingMsgIndex].content += d.Text
			return m
		}
		m.thinkingMsgIndex = m.insertBeforeAgent(convMessage{
			role:      convRoleThinking,
			content:   d.Text,
			agentName: d.AgentName,
			modelSlug: d.Model,
		})
		return m
	case harness.StreamKindTool:
		m.insertBeforeAgent(convMessage{
			role:      convRoleTool,
			content:   d.Text,
			agentName: d.AgentName,
			modelSlug: d.Model,
			callID:    d.CallID,
		})
		return m
	default:
		if d.CallID != "" {
			m.appendAttributed(convRoleAgent, d)
			return m
		}
		m.appendAgentDelta(d.Text)
		if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
			if m.transcript[m.agentMsgIndex].agentName == "" {
				m.transcript[m.agentMsgIndex].agentName = d.AgentName
			}
			if m.transcript[m.agentMsgIndex].modelSlug == "" {
				m.transcript[m.agentMsgIndex].modelSlug = d.Model
			}
		}
		return m
	}
}

func formatHarnessPermission(req harness.PermissionRequest) string {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Harness permission"
	}
	desc := strings.TrimSpace(req.Description)
	if desc != "" {
		return fmt.Sprintf("Harness permission: %s — %s. Allow? [y/N]", title, desc)
	}
	return fmt.Sprintf("Harness permission: %s. Allow? [y/N]", title)
}

func (m model) clearHarnessPermission() model {
	if m.harnessPermissionPending && m.harnessPermissionRespCh != nil {
		m.harnessPermissionRespCh <- harness.PermissionResponse{Approved: false, Reason: "cancelled"}
	}
	m.harnessPermissionPending = false
	m.harnessPermissionMsg = ""
	m.harnessPermissionRespCh = nil
	return m
}

func (m model) replyHarnessPermission(approved bool) model {
	if m.harnessPermissionRespCh != nil {
		m.harnessPermissionRespCh <- harness.PermissionResponse{Approved: approved}
	}
	m.harnessPermissionPending = false
	m.harnessPermissionMsg = ""
	m.harnessPermissionRespCh = nil
	return m
}

func (m model) handleHarnessPermissionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m = m.replyHarnessPermission(true)
	case "n", "N", "esc":
		m = m.replyHarnessPermission(false)
	default:
		return m, nil
	}
	if m.streaming && m.convStreamCh != nil {
		return m, waitConvMsg(m.convStreamCh)
	}
	return m, nil
}

func (m model) addLiveAgent(d harness.StreamDelta) model {
	callID := strings.TrimSpace(d.CallID)
	if callID == "" {
		return m
	}
	for _, a := range m.liveAgents {
		if a.CallID == callID {
			return m
		}
	}
	name := strings.TrimSpace(d.AgentName)
	if !isKnownHeroAgent(name) {
		// Nested generic Tasks (explore, generalPurpose, …) must not chip HARN.
		// HARN is only the parent session when no Hero agent is bound.
		return m
	}
	m.liveAgents = append(m.liveAgents, liveAgent{
		CallID: callID,
		Name:   name,
		Label:  agentShortLabel(name),
		Model:  strings.TrimSpace(d.Model),
	})
	return m
}

func (m model) removeLiveAgent(callID string) model {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return m
	}
	out := m.liveAgents[:0]
	for _, a := range m.liveAgents {
		if a.CallID != callID {
			out = append(out, a)
		}
	}
	m.liveAgents = out
	return m
}

func (m model) transcriptHasSubagent() bool {
	for _, msg := range m.latestAgentTurn() {
		if strings.TrimSpace(msg.callID) != "" {
			return true
		}
	}
	return false
}

func (m *model) appendAttributed(role convRole, d harness.StreamDelta) {
	for i := m.agentMsgIndex - 1; i >= 0; i-- {
		msg := m.transcript[i]
		if msg.role == convRoleUser {
			break
		}
		if msg.role == role && msg.callID == d.CallID {
			m.transcript[i].content += d.Text
			if role == convRoleThinking {
				m.thinkingMsgIndex = i
			}
			return
		}
	}
	idx := m.insertBeforeAgent(convMessage{
		role:      role,
		content:   d.Text,
		agentName: d.AgentName,
		modelSlug: d.Model,
		callID:    d.CallID,
	})
	if role == convRoleThinking {
		m.thinkingMsgIndex = idx
	}
}

// insertBeforeAgent inserts msg just before the agent answer bubble and returns its index.
func (m *model) insertBeforeAgent(msg convMessage) int {
	if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
		idx := m.agentMsgIndex
		m.transcript = append(m.transcript[:idx], append([]convMessage{msg}, m.transcript[idx:]...)...)
		m.agentMsgIndex++
		return idx
	}
	m.transcript = append(m.transcript, msg)
	return len(m.transcript) - 1
}

func (m *model) appendAgentDelta(delta string) {
	if m.agentMsgIndex < 0 || m.agentMsgIndex >= len(m.transcript) {
		m.transcript = append(m.transcript, convMessage{role: convRoleAgent, content: delta})
		m.agentMsgIndex = len(m.transcript) - 1
		return
	}
	m.transcript[m.agentMsgIndex].content += delta
}

func (m model) renderConversation(contentH int) string {
	n := m.responseVisibleLines(contentH)
	s := m.buildConversation(n)
	// Absorb measurement drift so leftover rows go into the response pane.
	for countContentLines(s) < contentH && n < chatResponseMaxLines {
		n++
		s = m.buildConversation(n)
	}
	return s
}

func countContentLines(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func (m model) buildConversation(responseLines int) string {
	if responseLines < 0 {
		responseLines = 0
	}
	var b strings.Builder
	b.WriteString(m.renderConversationHeader())
	b.WriteByte('\n')

	b.WriteString(m.renderConversationHistory())
	b.WriteByte('\n')
	b.WriteString(m.renderConversationResponse(responseLines))

	if m.convError != "" && !m.latestAgentFailed() {
		b.WriteByte('\n')
		b.WriteString(m.renderWrappedConvError())
	}

	b.WriteString(m.renderChatSlashOverlay())
	b.WriteString(m.renderConversationInput())
	return strings.TrimRight(b.String(), "\n")
}

func (m model) renderConversationHeader() string {
	stage := m.conversationStage
	tool := m.conversationHarnessTool()
	var left strings.Builder
	if stage == "" {
		left.WriteString(headerStyle.Render(fmt.Sprintf("Chat · harness %s", tool)))
		if m.harnessSessionID != "" {
			left.WriteString(mutedStyle.Render(" │ "))
			left.WriteString(mutedStyle.Render("session: " + truncateSessionID(m.harnessSessionID)))
		}
	} else {
		iter := ""
		want := stageNameKey(stage)
		for _, st := range m.status.Stages {
			if stageNameKey(st.Name) == want {
				iter = st.Iteration
				break
			}
		}
		left.WriteString(headerStyle.Render(fmt.Sprintf("Cycle C%d — %s · iter %s", m.status.CycleNumber, stage, iter)))
		if m.harnessSessionID != "" {
			left.WriteString(mutedStyle.Render(" │ "))
			left.WriteString(mutedStyle.Render("session: " + truncateSessionID(m.harnessSessionID)))
		}
	}
	right := m.renderAgentsBox()
	gap := 1
	leftW := m.width - agentsBoxWidth - gap
	if m.width <= 0 || leftW < 20 {
		return left.String() + "\n" + right
	}
	leftBlock := lipgloss.NewStyle().Width(leftW).MaxWidth(leftW).Render(left.String())
	return lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, right)
}

// stageNameKey normalizes SQLite slugs (qa_end_to_end) and status display names
// ("Qa End To End") so the Chat header can look up iteration counts.
func stageNameKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func (m model) renderAgentsBox() string {
	n := len(m.liveAgents)
	labels := make([]string, 0, n)
	for _, a := range m.liveAgents {
		labels = append(labels, a.Label)
	}
	innerW := agentsBoxWidth - 2
	if innerW < 8 {
		innerW = 8
	}
	line1 := fmt.Sprintf("agents: %d", n)
	line2 := wrapAgentLabels(labels, innerW)
	body := line1
	if line2 != "" {
		body += "\n" + line2
	} else {
		body += "\n"
	}
	return chatBoxStyle.Width(agentsBoxWidth).Render(body)
}

func (m model) latestAgentFailed() bool {
	for _, msg := range m.latestAgentTurn() {
		if msg.failed {
			return true
		}
	}
	return false
}

func (m model) renderWrappedConvError() string {
	var b strings.Builder
	width := m.chatBoxWidth()
	for _, line := range splitOutputLines("✗ "+m.convError, width) {
		b.WriteString(errorStyle.Render(line))
		b.WriteByte('\n')
	}
	lower := strings.ToLower(m.convError)
	if strings.Contains(lower, "auth") || strings.Contains(lower, "login") {
		b.WriteString(mutedStyle.Render("→ Check harness login: cursor agent login"))
		b.WriteByte('\n')
	}
	if strings.Contains(lower, "workspace trust") || strings.Contains(lower, "trust required") {
		b.WriteString(mutedStyle.Render("→ Trust this folder in Cursor, or run: cursor agent --trust"))
		b.WriteByte('\n')
	}
	return b.String()
}

// renderConversationHistory shows prior user turns (compact) above the response pane.
func (m model) renderConversationHistory() string {
	users := make([]convMessage, 0)
	for _, msg := range m.transcript {
		if msg.role == convRoleUser {
			users = append(users, msg)
		}
	}
	if len(users) == 0 {
		return mutedStyle.Render("Submit a message to start an interação.")
	}
	// Keep history minimal so the response pane absorbs vertical growth.
	maxShown := 1
	start := 0
	if len(users) > maxShown {
		start = len(users) - maxShown
	}
	var b strings.Builder
	if start > 0 {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("… %d earlier", start)))
		b.WriteByte('\n')
	}
	wrapW := m.chatBoxWidth()
	for _, msg := range users[start:] {
		line := "You: " + msg.content
		b.WriteString(infoStyle.Render(wrapWidth(line, wrapW)))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m model) historyLineCount() int {
	users := 0
	for _, msg := range m.transcript {
		if msg.role == convRoleUser {
			users++
		}
	}
	if users == 0 {
		return 1 // empty-state hint
	}
	n := 1
	if users > 1 {
		n++ // "… N earlier"
	}
	return n
}

func (m model) chatBoxWidth() int {
	boxW := m.width
	if boxW <= 0 {
		boxW = 72
	}
	if boxW < 28 {
		boxW = 28
	}
	return boxW
}

// chatInnerWidth is the width available inside the rounded border (excludes border cells).
func (m model) chatInnerWidth() int {
	inner := m.chatBoxWidth() - 2
	if inner < 10 {
		inner = 10
	}
	return inner
}

// chatContentWidth is text columns after the accent bar + gap.
func (m model) chatContentWidth() int {
	w := m.chatInnerWidth() - 2
	if w < 8 {
		w = 8
	}
	return w
}

// responseVisibleLines sizes the agent pane so it alone absorbs leftover terminal height.
// Measures a 0-row probe so growth goes into the response box, not gaps between panes.
func (m model) responseVisibleLines(contentH int) int {
	if contentH <= 0 {
		contentH = m.contentAreaHeight()
	}
	base := countContentLines(m.buildConversation(0))
	n := contentH - base
	if n < chatResponseMinLines {
		return chatResponseMinLines
	}
	if n > chatResponseMaxLines {
		return chatResponseMaxLines
	}
	return n
}

func (m model) renderConversationResponse(responseLines int) string {
	contentW := m.chatContentWidth()
	innerW := m.chatInnerWidth()
	accent := chatAccentResponse

	lines := m.responseContentLines(contentW)
	visible := responseLines
	if visible < 0 {
		visible = 0
	}
	offset := m.respScrollOffset
	maxOff := len(lines) - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOff {
		offset = maxOff
	}

	rows := make([]string, 0, visible+1)
	for i := 0; i < visible; i++ {
		idx := offset + i
		var cell string
		if idx < len(lines) {
			cell = lines[idx]
		}
		rows = append(rows, chatAccentRow(accent, cell, innerW))
	}

	statusContent := ""
	if m.streaming {
		frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
		statusContent = chatInText.Render(frame) + chatInMuted.Render(" ")
	}
	statusContent += chatInAgent.Render(m.responseSpeakerHeader())
	if visible > 0 && len(lines) > visible {
		statusContent += chatInMuted.Render(fmt.Sprintf(" · %d–%d/%d", offset+1, minInt(offset+visible, len(lines)), len(lines)))
	}
	rows = append(rows, chatAccentRow(accent, statusContent, innerW))

	var b strings.Builder
	b.WriteString(chatBoxStyle.Width(m.chatBoxWidth()).Render(strings.Join(rows, "\n")))
	b.WriteByte('\n')
	b.WriteString(m.renderScrollHintLine())
	b.WriteByte('\n')
	return b.String()
}

func (m model) responseContentLines(contentW int) []string {
	turn := m.latestAgentTurn()
	if len(turn) == 0 {
		if m.streaming {
			return []string{chatInMuted.Render("Waiting for harness…")}
		}
		return []string{chatInMuted.Render("Agent response will appear here.")}
	}

	var out []string
	prevKey := ""
	prevWasSub := false
	for _, msg := range turn {
		if msg.role == convRoleAgent && strings.TrimSpace(msg.content) == "" && m.streaming && !msg.failed && !msg.interrupted {
			continue
		}
		key := messageAgentKey(msg)
		isSub := strings.TrimSpace(msg.callID) != ""
		if key != prevKey {
			if prevKey != "" && prevWasSub {
				out = append(out, "")
			}
			if isSub {
				out = append(out, "")
			}
			header := formatAgentHeader(msg.agentName, msg.modelSlug, m.conversationHarnessTool())
			out = append(out, chatInAgent.Render(header))
			prevKey = key
			prevWasSub = isSub
		}
		switch msg.role {
		case convRoleThinking:
			text := formatChatAgentText(m.runtimeCommandName, "Thinking: "+msg.content)
			for _, line := range splitOutputLines(text, contentW) {
				out = append(out, chatInThink.Render(line))
			}
		case convRoleTool:
			text := formatChatAgentText(m.runtimeCommandName, "→ "+msg.content)
			for _, line := range splitOutputLines(text, contentW) {
				out = append(out, chatInMuted.Render(line))
			}
		case convRoleWarning:
			for _, line := range splitOutputLines(msg.content, contentW) {
				out = append(out, chatInWarn.Render(line))
			}
		case convRoleActivity:
			text := formatChatAgentText(m.runtimeCommandName, "· "+msg.content)
			for _, line := range splitOutputLines(text, contentW) {
				out = append(out, chatInMuted.Render(line))
			}
		case convRoleAgent:
			text := msg.content
			if msg.interrupted && text == "" {
				text = "Interrupted"
			} else if msg.interrupted && text != "" {
				text += "\n[Interrupted]"
			}
			text = formatChatAgentText(m.runtimeCommandName, text)
			style := chatInOK
			if msg.failed {
				style = chatInErr
			} else if msg.interrupted {
				style = chatInWarn
			}
			if text == "" && m.streaming {
				continue
			}
			for _, line := range splitOutputLines(text, contentW) {
				out = append(out, style.Render(line))
			}
		}
	}
	if prevWasSub {
		out = append(out, "")
	}
	if len(out) == 0 && m.streaming {
		return []string{chatInMuted.Render("Waiting for harness…")}
	}
	return out
}

func messageAgentKey(msg convMessage) string {
	if id := strings.TrimSpace(msg.callID); id != "" {
		return "sub:" + id
	}
	return "parent"
}

// latestAgentTurn returns thinking/tool/agent messages after the last user message.
func (m model) latestAgentTurn() []convMessage {
	if len(m.transcript) == 0 {
		return nil
	}
	lastUser := -1
	for i := len(m.transcript) - 1; i >= 0; i-- {
		if m.transcript[i].role == convRoleUser {
			lastUser = i
			break
		}
	}
	start := lastUser + 1
	if start < 0 {
		start = 0
	}
	var turn []convMessage
	for _, msg := range m.transcript[start:] {
		if msg.role == convRoleUser {
			continue
		}
		turn = append(turn, msg)
	}
	return turn
}

func (m model) renderConversationInput() string {
	var b strings.Builder
	contentW := m.chatContentWidth()
	innerW := m.chatInnerWidth()

	mode := m.chatMode
	if mode == "" {
		mode = harness.ModeBuild
	}
	accent := chatAccentBuild
	modeStyle := chatInBuild
	modeLabel := "Build"
	if mode == harness.ModePlan {
		accent = chatAccentPlan
		modeStyle = chatInPlan
		modeLabel = "Plan"
	}

	lines := m.inputContentLines(contentW)
	visible := chatInputVisibleLines
	offset := m.inputScrollOffset
	maxOff := len(lines) - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOff {
		offset = maxOff
	}

	rows := make([]string, 0, visible+1)
	for i := 0; i < visible; i++ {
		idx := offset + i
		var cell string
		if idx < len(lines) {
			cell = lines[idx]
		}
		rows = append(rows, chatAccentRow(accent, cell, innerW))
	}

	statusContent := modeStyle.Render(modeLabel) +
		chatInMuted.Render(" · ") +
		chatInModel.Render(m.conversationModelLabel()) +
		chatInMuted.Render(" · ") +
		chatInMuted.Render(m.conversationHarnessTool())
	if len(lines) > visible {
		statusContent += chatInMuted.Render(fmt.Sprintf(" · %d–%d/%d", offset+1, minInt(offset+visible, len(lines)), len(lines)))
	}
	rows = append(rows, chatAccentRow(accent, statusContent, innerW))

	b.WriteString(chatBoxStyle.Width(m.chatBoxWidth()).Render(strings.Join(rows, "\n")))
	b.WriteByte('\n')
	if !m.streaming {
		if m.chatSlashOverlayActive() {
			b.WriteString(mutedStyle.Render("enter insert · tab insert · esc close · ↑↓"))
		} else {
			b.WriteString(mutedStyle.Render("tab mode · / commands · enter send · alt+enter newline · ←→ move · ↑↓ scroll"))
		}
	} else {
		b.WriteString(mutedStyle.Render("ctrl+c interrupt"))
	}
	return b.String()
}

func (m model) inputContentLines(contentW int) []string {
	if m.streaming {
		text := m.input
		if text == "" {
			return []string{""}
		}
		var out []string
		for _, line := range wrapOutputLine(text, contentW) {
			out = append(out, chatInMuted.Render(line))
		}
		return out
	}
	return m.inputLinesWithCaret(contentW)
}

func (m model) inputLinesWithCaret(contentW int) []string {
	runes := []rune(m.input)
	cur := m.inputCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}

	before := string(runes[:cur])
	after := string(runes[cur:])
	plain := before + "\x00" + after
	if plain == "\x00" {
		return []string{m.renderInputCaret()}
	}

	var lines []string
	var current strings.Builder
	lineLen := 0
	flush := func() {
		raw := current.String()
		current.Reset()
		lineLen = 0
		if !strings.Contains(raw, "\x00") {
			lines = append(lines, chatInText.Render(raw))
			return
		}
		parts := strings.SplitN(raw, "\x00", 2)
		styled := chatInText.Render(parts[0]) + m.renderInputCaret()
		if len(parts) > 1 {
			styled += chatInText.Render(parts[1])
		}
		lines = append(lines, styled)
	}
	for _, r := range plain {
		if r == '\n' {
			flush()
			continue
		}
		current.WriteRune(r)
		lineLen++
		if lineLen >= contentW {
			flush()
		}
	}
	if current.Len() > 0 || len(lines) == 0 {
		flush()
	}
	if len(lines) == 0 {
		return []string{m.renderInputCaret()}
	}
	return lines
}

// chatAccentRow paints accent + gap + content to exactly innerW cells (no nested Background).
func chatAccentRow(accent lipgloss.Style, content string, innerW int) string {
	if innerW < 3 {
		innerW = 3
	}
	contentW := innerW - 2 // accent + gap
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", "")
	if lipgloss.Width(content) > contentW {
		content = lipgloss.NewStyle().MaxWidth(contentW).Render(content)
	}
	pad := contentW - lipgloss.Width(content)
	if pad < 0 {
		pad = 0
	}
	bar := accent.Width(1).Render(" ")
	return bar + " " + content + strings.Repeat(" ", pad)
}

func (m model) renderInputCaret() string {
	if m.streaming {
		return ""
	}
	if m.chatInputFocused {
		return caretFilledStyle.Render(" ")
	}
	return caretHollowStyle.Render("▮")
}

func (m model) contentAreaHeight() int {
	h := m.height - (5 + statusBarMaxLines)
	if h < 3 {
		h = 3
	}
	return h
}

func (m model) maxInputScroll() int {
	lines := len(m.inputContentLines(m.chatContentWidth()))
	maxOff := lines - chatInputVisibleLines
	if maxOff < 0 {
		return 0
	}
	return maxOff
}

func (m model) maxResponseScroll() int {
	lines := len(m.responseContentLines(m.chatContentWidth()))
	maxOff := lines - m.responseVisibleLines(m.contentAreaHeight())
	if maxOff < 0 {
		return 0
	}
	return maxOff
}

func (m model) scrollResponse(delta int) model {
	m.respScrollOffset += delta
	maxOff := m.maxResponseScroll()
	if m.respScrollOffset < 0 {
		m.respScrollOffset = 0
	}
	if m.respScrollOffset > maxOff {
		m.respScrollOffset = maxOff
	}
	m.respFollowBottom = m.respScrollOffset >= maxOff
	return m
}

func (m model) maybeFollowResponseBottom() model {
	if !m.respFollowBottom {
		return m
	}
	m.respScrollOffset = m.maxResponseScroll()
	return m
}

func (m model) ensureInputCaretVisible() model {
	contentW := m.chatContentWidth()
	lines := m.inputLinesWithCaret(contentW)
	// Approximate caret line: count wraps up to cursor.
	caretLine := 0
	runes := []rune(m.input)
	cur := m.inputCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	lineLen := 0
	for i := 0; i < cur; i++ {
		if runes[i] == '\n' {
			caretLine++
			lineLen = 0
			continue
		}
		lineLen++
		if lineLen >= contentW {
			caretLine++
			lineLen = 0
		}
	}
	if caretLine < m.inputScrollOffset {
		m.inputScrollOffset = caretLine
	}
	if caretLine >= m.inputScrollOffset+chatInputVisibleLines {
		m.inputScrollOffset = caretLine - chatInputVisibleLines + 1
	}
	maxOff := len(lines) - chatInputVisibleLines
	if maxOff < 0 {
		maxOff = 0
	}
	if m.inputScrollOffset < 0 {
		m.inputScrollOffset = 0
	}
	if m.inputScrollOffset > maxOff {
		m.inputScrollOffset = maxOff
	}
	return m
}

func truncateSessionID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…" + id[len(id)-4:]
}

func lipglossStyleForAgent(interrupted bool) lipgloss.Style {
	if interrupted {
		return warnStyle
	}
	return successStyle
}

func wrapWidth(s string, width int) string {
	if width <= 0 {
		width = 80
	}
	var out strings.Builder
	lineLen := 0
	for _, r := range s {
		if r == '\n' {
			out.WriteByte('\n')
			lineLen = 0
			continue
		}
		out.WriteRune(r)
		lineLen++
		if lineLen >= width {
			out.WriteByte('\n')
			lineLen = 0
		}
	}
	return out.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
