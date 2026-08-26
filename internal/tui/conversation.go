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
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
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

// Visible content rows inside chat panes (excluding status row on composer).
const (
	chatInputVisibleLines  = 3
	chatTranscriptMinLines = 2
)

var waitAnimFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type convMessage struct {
	role        convRole
	content     string
	agentName   string
	modelSlug   string
	harnessID   string // agent/session harness for speaker labels (may differ from freechat)
	callID      string
	interrupted bool
	failed      bool

	// responseLines caches the expensive wrapping/styling work for a transcript
	// message. The cache is invalidated whenever the message content/state changes.
	responseLines        []string
	responseLinesWidth   int
	responseLinesRuntime string
	responseLinesValid   bool
}

type streamDeltaMsg struct {
	executeID string
	delta     harness.StreamDelta
}

// conversationBatchMsg keeps a burst of harness events inside one Bubble Tea
// Update/View cycle. This prevents high-volume /hero-start streams from making
// the renderer rebuild the transcript once per character-sized delta.
type conversationBatchMsg struct {
	messages []tea.Msg
}

type harnessPermissionRequestMsg struct {
	req    harness.PermissionRequest
	respCh chan harness.PermissionResponse
}

type executeDoneMsg struct {
	executeID string
	result    *harness.ExecutionResult
	err       error
	harnessID string // harness used for this execute (session binding)
}

// executePairMsg updates Chat labels to the resolved execute pair (including fallback)
// before Adapter.Execute starts streaming.
type executePairMsg struct {
	executeID string
	harnessID string
	model     string
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

func (m model) showHarnessWait() bool {
	if !m.streaming {
		return false
	}
	if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
		msg := m.transcript[m.agentMsgIndex]
		if msg.failed || msg.interrupted {
			return false
		}
	}
	return true
}

func (m model) showChatWait() bool {
	if m.heroStartBootstrapping || m.heroStartPreparing {
		return true
	}
	return m.showHarnessWait()
}

func (m model) chatWaitLine(rowW int) string {
	frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
	text := " Waiting for harness…"
	if m.heroStartBootstrapping || m.heroStartPreparing {
		text = " Preparing /hero-start…"
	}
	pending := chatInText.Render(frame) + chatInMuted.Render(text)
	return chatThinBarRow(chatBarMuted, pending, rowW)
}

func (m model) waitingForHarnessLine(rowW int) string {
	return m.chatWaitLine(rowW)
}

func (m model) conversationExecuteCmds() tea.Cmd {
	return tea.Batch(waitConvBatchMsg(m.convStreamCh), convWaitTickCmd(), harnessHealthProbeCmd())
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
	if h := strings.TrimSpace(m.runtimeHarnessID); h != "" {
		return h
	}
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

// applyAgentRuntimePair sets runtimeHarnessID + runtimeModelSlug from workflow-config
// agents.<agentName> (or leaves harness empty so freechat chatHarnessID is used).
func (m model) applyAgentRuntimePair(agentName, modelSlugOverride string) model {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	pair, _, err := workflowconfig.AgentPairFor(projectDir, agentName)
	if err == nil {
		if h := strings.TrimSpace(strings.ToLower(pair.Harness)); h != "" {
			m.runtimeHarnessID = h
		}
		if override := strings.TrimSpace(modelSlugOverride); override != "" {
			m.runtimeModelSlug = override
		} else if s := strings.TrimSpace(pair.Slug); s != "" {
			m.runtimeModelSlug = s
		}
		return m
	}
	if override := strings.TrimSpace(modelSlugOverride); override != "" {
		m.runtimeModelSlug = override
	} else if slug := m.defaultHarnessModelSlug(); slug != "" {
		m.runtimeModelSlug = slug
	}
	return m
}

func (m model) clearRuntimePair() model {
	m.runtimeModelSlug = ""
	m.runtimeHarnessID = ""
	return m
}

// agentHarnessForName returns the YAML harness for a Hero agent, else the active
// runtime/freechat harness used for Chat labels.
func (m model) agentHarnessForName(name string) string {
	name = strings.TrimSpace(name)
	if name != "" && m.svc != nil {
		if pair, _, err := workflowconfig.AgentPairFor(m.svc.ProjectDir, name); err == nil {
			if h := strings.TrimSpace(strings.ToLower(pair.Harness)); h != "" {
				return h
			}
		}
	}
	return m.conversationHarnessTool()
}

func (m model) harnessForMessage(msg convMessage) string {
	if h := strings.TrimSpace(msg.harnessID); h != "" {
		return h
	}
	if name := strings.TrimSpace(msg.agentName); name != "" {
		return m.agentHarnessForName(name)
	}
	return m.conversationHarnessTool()
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
// Codex thread ids never resume as Cursor/OpenCode (and vice versa).
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
		if harnessID != "" {
			m.harnessSessionHarnessID = harnessID
		}
		m.conversationStage = stageResearch
		if m.svc != nil {
			if err := m.svc.SetStageHarnessSessionID(stageResearch, sessionID); err != nil {
				slog.Error("tui persist harness session failed", "error", err)
			}
			if harnessID != "" {
				if err := m.svc.SetStageHarnessID(stageResearch, harnessID); err != nil {
					slog.Debug("tui persist research harness id failed", "error", err)
				}
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

// responseSpeakerHeader is the live origin label: [QA - composer-2.5 · opencode], [HARN - grok-4.6 · cursor].
func (m model) responseSpeakerHeader() string {
	name := strings.TrimSpace(m.runtimeAgentName)
	model := m.conversationModelSlug()
	harnessID := m.conversationHarnessTool()
	if n := len(m.liveAgents); n > 0 {
		a := m.liveAgents[n-1]
		name = a.Name
		if slug := strings.TrimSpace(a.Model); slug != "" {
			model = slug
		}
		if h := strings.TrimSpace(a.Harness); h != "" {
			harnessID = h
		}
		return formatAgentHeader(name, model, harnessID)
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
		if h := strings.TrimSpace(msg.harnessID); h != "" {
			harnessID = h
		} else if name != "" {
			harnessID = m.agentHarnessForName(name)
		}
		break
	}
	return formatAgentHeader(name, model, harnessID)
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
	m = m.clearRuntimePair()
	m.runtimeAgentName = ""
	m.liveAgents = nil
	m.executes = nil
	m.stageHandoffLive = false
	m.stageHandoffStage = ""
	m.stageHandoffOutputs = nil
	m.stageHandoffDoneKey = ""
	m.convError = ""
	m.streamInterrupted = false
	m.agentMsgIndex = -1
	m.thinkingMsgIndex = -1
	m.transcriptScrollOffset = 0
	m.transcriptFollowBottom = true
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
	RejectReason         string
	ContinueExtra        int // 0 = default 1
	CancelReason         string
	ResumeCycleNumber    int // 0 = latest non-archived
	preloadedCommandBody string
	preloadedAgentBody   string
	preloadedPrompt      bool
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
	cmdBody := opts.preloadedCommandBody
	if !opts.preloadedPrompt {
		cmdPath := filepath.Join(m.svc.ProjectDir, cursoradapter.CommandsDir, "hero-"+cmdName+".md")
		var err error
		cmdBody, err = cursoradapter.ReadCommandPrompt(cmdPath)
		if err != nil {
			slog.Error("tui hero runtime command read failed", "path", cmdPath, "error", err)
			m, _ = m.enterConversation()
			m.convError = fmt.Errorf("read command %s: %w", label, err).Error()
			return m, nil
		}
	}
	m, _ = m.enterConversation()
	m.conversationStage = ""
	m.harnessSessionID = ""
	m.harnessSessionHarnessID = ""
	m.runtimeCommandName = cmdName
	m.runtimeModelSlug = strings.TrimSpace(modelSlug)
	m.runtimeHarnessID = ""
	m.runtimeAgentName = ""
	m.orchestrationLive = cmdName == "start" || cmdName == "approve"
	if cmdName == "start" {
		m.researchLive = false
		m.orchestrationSessionID = ""
		m.researchSessionID = ""
	}

	var executePrompt string
	if usesOrchestratorRuntime(cmdName) {
		var composite string
		var err error
		if opts.preloadedPrompt {
			composite = joinRuntimePromptBodies(opts.preloadedAgentBody, cmdBody)
		} else {
			composite, err = orchestratorRuntimePrompt(m.svc.ProjectDir, cmdBody)
		}
		if err != nil {
			slog.Error("tui orchestration runtime prompt failed", "cmd", cmdName, "error", err)
			m.convError = err.Error()
			return m, nil
		}
		executePrompt = tuiRuntimeCommandPrompt(cmdName, composite, opts)
		m = m.withRuntimeAgent(agentOrchestration)
		m = m.applyAgentRuntimePair(agentOrchestration, modelSlug)
	} else {
		executePrompt = tuiRuntimeCommandPrompt(cmdName, cmdBody, opts)
	}

	m = m.beginConversationExecute(label, executePrompt)
	return m, m.conversationExecuteCmds()
}

func orchestratorRuntimePrompt(projectDir, cmdBody string) (string, error) {
	agentPath := filepath.Join(projectDir, cursoradapter.AgentsDir, "orchestration_agent.md")
	agentBody, err := cursoradapter.ReadAgentPrompt(agentPath)
	if err != nil {
		return "", fmt.Errorf("read orchestration_agent: %w", err)
	}
	return joinRuntimePromptBodies(agentBody, cmdBody), nil
}

func joinRuntimePromptBodies(agentBody, cmdBody string) string {
	composite := strings.TrimSpace(agentBody)
	if body := strings.TrimSpace(cmdBody); body != "" {
		if composite != "" {
			composite += "\n\n---\n\n"
		}
		composite += body
	}
	return composite
}

func (m model) beginConversationExecute(userLabel, executePrompt string) model {
	m.executes = nil
	m.convStreamCh = nil
	return m.startTaggedExecute(userLabel, executePrompt, true)
}

func (m model) appendConversationExecute(userLabel, executePrompt string) model {
	return m.startTaggedExecute(userLabel, executePrompt, false)
}

func (m model) nextExecuteID() (model, string) {
	m.executeSeq++
	return m, fmt.Sprintf("ex-%d", m.executeSeq)
}

func (m model) startTaggedExecute(userLabel, executePrompt string, reset bool) model {
	m.streamInterrupted = false
	m.convError = ""
	m.streaming = true
	m.waitAnimFrame = 0
	m.transcriptFollowBottom = true
	m.chatInputFocused = false
	m = m.resetHarnessWatchdog(executePrompt)
	parentName := strings.TrimSpace(m.runtimeAgentName)
	parentModel := m.conversationModelSlug()
	parentHarness := m.conversationHarnessTool()
	m, executeID := m.nextExecuteID()
	if reset || m.executes == nil {
		m.executes = make(map[string]convExecute)
	}
	if reset {
		m.liveAgents = nil
	}
	m.liveAgents = append(m.liveAgents, liveAgent{
		CallID:  executeID,
		Name:    parentName,
		Label:   agentShortLabel(parentName),
		Model:   parentModel,
		Harness: parentHarness,
	})
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: userLabel})
	m.transcript = append(m.transcript, convMessage{
		role:      convRoleAgent,
		content:   "",
		agentName: parentName,
		modelSlug: parentModel,
		harnessID: parentHarness,
	})
	m.agentMsgIndex = len(m.transcript) - 1
	m.thinkingMsgIndex = -1
	if m.executes == nil {
		m.executes = make(map[string]convExecute)
	}
	m.executes[executeID] = convExecute{
		ID:            executeID,
		AgentName:     parentName,
		HarnessID:     parentHarness,
		AgentMsgIndex: m.agentMsgIndex,
	}

	if m.convStreamCh == nil {
		ch := make(chan tea.Msg, 512)
		m.convStreamCh = ch
	}
	m.startConversationExecute(executeID, executePrompt, m.convStreamCh)
	return m.maybeFollowTranscriptBottom()
}

func (m model) handleConversationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	if m.heroStartBootstrapping || m.heroStartPreparing {
		switch s {
		case "ctrl+c":
			return m.cancelHeroStartPreparation()
		case "alt+q":
			return m.showConfirm(actionQuit, 0, "Preparing /hero-start. Quit? [y/N]")
		case "ctrl+1", "alt+1", "ctrl+2", "alt+2", "ctrl+3", "alt+3",
			"ctrl+4", "alt+4", "ctrl+5", "alt+5", "alt+n":
			return m.handleKey(msg)
		case "up", "ctrl+p":
			m = m.scrollTranscript(-1)
			return m, nil
		case "down", "ctrl+n":
			m = m.scrollTranscript(1)
			return m, nil
		case "pgup":
			m = m.scrollTranscript(-m.transcriptVisibleLines(m.contentAreaHeight()))
			return m, nil
		case "pgdown":
			m = m.scrollTranscript(m.transcriptVisibleLines(m.contentAreaHeight()))
			return m, nil
		default:
			// The preflight owns the composer until it completes or is cancelled.
			return m, nil
		}
	}

	if m.streaming && !m.harnessQuestionPending {
		switch s {
		case "ctrl+c":
			return m, m.cancelStreamCmd()
		case "alt+q":
			return m.showConfirm(actionQuit, 0, "Agent is running. Quit? [y/N]")
		case "ctrl+1", "alt+1", "ctrl+2", "alt+2", "ctrl+3", "alt+3",
			"ctrl+4", "alt+4", "ctrl+5", "alt+5", "alt+n":
			// Allow screen navigation while streaming; the goroutine keeps running.
			return m.handleKey(msg)
		case "alt+r":
			return m.copyChatResponse()
		case "alt+i":
			return m.copyChatInput()
		case "up", "ctrl+p":
			m = m.scrollTranscript(-1)
			return m, nil
		case "down", "ctrl+n":
			m = m.scrollTranscript(1)
			return m, nil
		case "pgup":
			m = m.scrollTranscript(-m.transcriptVisibleLines(m.contentAreaHeight()))
			return m, nil
		case "pgdown":
			m = m.scrollTranscript(m.transcriptVisibleLines(m.contentAreaHeight()))
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
	case "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5",
		"alt+q", "alt+n", "alt+1", "alt+2", "alt+3", "alt+4", "alt+5",
		"ctrl+r", "f5":
		return m.handleKey(msg)
	case "alt+r":
		return m.copyChatResponse()
	case "alt+i":
		return m.copyChatInput()
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
		m = m.scrollTranscript(-1)
		return m, nil
	case "down", "ctrl+n":
		maxIn := m.maxInputScroll()
		if m.inputScrollOffset < maxIn {
			m.inputScrollOffset++
			return m, nil
		}
		m = m.scrollTranscript(1)
		return m, nil
	case "pgup":
		m = m.scrollTranscript(-m.transcriptVisibleLines(m.contentAreaHeight()))
		return m, nil
	case "pgdown":
		m = m.scrollTranscript(m.transcriptVisibleLines(m.contentAreaHeight()))
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
		if strings.TrimSpace(m.runtimeHarnessID) == "" && m.workflowAgentActive() {
			m = m.applyAgentRuntimePair(m.runtimeAgentName, m.runtimeModelSlug)
		}
		if m.runtimeAgentName == "" {
			m.runtimeAgentName = agentOrchestration
			m = m.applyAgentRuntimePair(agentOrchestration, m.runtimeModelSlug)
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
	m = m.beginConversationExecute(text, controlSlashFollowUpPrompt(text))
	return m, m.conversationExecuteCmds()
}

func controlSlashFollowUpPrompt(text string) string {
	if strings.EqualFold(strings.TrimSpace(text), "/hero-approve") {
		return tuiApproveFollowUpPrompt()
	}
	return text
}

func (m model) dispatchExactHeroSlash(text string) (model, tea.Cmd, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if m.freeChatMode {
		switch lower {
		case slashModel, "/hero-model":
			m = m.clearChatInput()
			next, cmd := m.openModelPicker()
			return next, cmd, true
		case slashHarness, "/hero-harness":
			m = m.clearChatInput()
			next, cmd := m.openHarnessPicker()
			return next, cmd, true
		case "/new-chat":
			m = m.clearChatInput()
			next, cmd := m.beginNewChat()
			return next, cmd, true
		case "/harness-reset":
			m = m.clearChatInput()
			next, cmd := m.beginHarnessResetPicker()
			return next, cmd, true
		default:
			if strings.HasPrefix(lower, "/hero") {
				m = m.clearChatInput()
				m = m.setStatusResult(false, lower, "not available in free chat")
				return m, nil, true
			}
			return m, nil, false
		}
	}
	switch lower {
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
	case slashModel, "/hero-model":
		m = m.clearChatInput()
		next, cmd := m.openModelPicker()
		return next, cmd, true
	case slashHarness, "/hero-harness":
		m = m.clearChatInput()
		next, cmd := m.openHarnessPicker()
		return next, cmd, true
	case slashRefresh:
		m = m.clearChatInput()
		return m, m.refreshCmd(), true
	case "/hero-config-update":
		m = m.clearChatInput()
		next, cmd := m.beginHeroConfigUpdate()
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
	case "/harness-reset":
		m = m.clearChatInput()
		next, cmd := m.beginHarnessResetPicker()
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

func (m model) startConversationExecute(executeID, prompt string, ch chan<- tea.Msg) {
	svc := m.svc
	stageName := m.conversationStage
	agentName := m.runtimeAgentName
	projectDir := m.executeDir()
	mode := m.chatMode
	if mode == "" {
		mode = harness.ModeBuild
	}
	go func() {
		ctx := context.Background()
		resolved, err := m.resolveExecuteResolution(ctx)
		if err != nil {
			ch <- executeDoneMsg{executeID: executeID, err: err}
			return
		}
		pair := resolved.pair
		if pair.Adapter == nil {
			ch <- executeDoneMsg{executeID: executeID, err: fmt.Errorf("harness adapter unavailable")}
			return
		}
		ch <- executePairMsg{executeID: executeID, harnessID: pair.HarnessID, model: pair.Model}
		sessionID := m.harnessSessionIDForPair(stageName, pair.HarnessID)
		if resolved.warning != "" {
			ch <- streamDeltaMsg{executeID: executeID, delta: harness.StreamDelta{Kind: harness.StreamKindText, Text: resolved.warning + "\n\n"}}
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
				deliverStreamDelta(ch, executeID, delta)
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
			OnQuestionRequest: func(ctx context.Context, qreq harness.QuestionRequest) (harness.QuestionResponse, error) {
				respCh := make(chan harness.QuestionResponse, 1)
				select {
				case ch <- harnessQuestionRequestMsg{req: qreq, respCh: respCh}:
				case <-ctx.Done():
					return harness.QuestionResponse{Rejected: true}, ctx.Err()
				}
				select {
				case resp := <-respCh:
					return resp, nil
				case <-ctx.Done():
					return harness.QuestionResponse{Rejected: true}, ctx.Err()
				}
			},
		}
		req = harness.NormalizeExecuteRequest(req)
		res, err := pair.Adapter.Execute(ctx, req)
		ch <- executeDoneMsg{executeID: executeID, result: res, err: err, harnessID: pair.HarnessID}
	}()
}

// streamDeltaMustDeliver is true for transcript-critical events. Dropping them
// truncates the live transcript mid-sentence under TUI backpressure.
func streamDeltaMustDeliver(kind harness.StreamKind) bool {
	switch kind {
	case harness.StreamKindText, harness.StreamKindThinking, harness.StreamKindWarning, harness.StreamKindSession:
		return true
	default:
		return false
	}
}

// deliverStreamDelta sends a harness delta to the conversation channel.
// Text/thinking/warning/session block until accepted; tool/activity may drop
// after a short wait so high-volume progress cannot stall the harness forever.
func deliverStreamDelta(ch chan<- tea.Msg, executeID string, delta harness.StreamDelta) {
	msg := streamDeltaMsg{executeID: executeID, delta: delta}
	if streamDeltaMustDeliver(delta.Kind) {
		ch <- msg
		return
	}
	select {
	case ch <- msg:
	default:
		select {
		case ch <- msg:
		case <-time.After(2 * time.Second):
			slog.Warn("tui stream delta dropped under backpressure", "kind", delta.Kind)
		}
	}
}

func waitConvMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

const (
	conversationBatchWindow = 25 * time.Millisecond
	conversationBatchMax    = 64
)

// waitConvBatchMsg coalesces short bursts of stream deltas into one Update.
// Harnesses commonly emit many small deltas while the agent is thinking; a
// 25ms window keeps the terminal responsive without making output feel delayed.
func waitConvBatchMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		first := <-ch
		if first == nil {
			return nil
		}
		messages := []tea.Msg{first}
		if isImmediateConversationMessage(first) {
			return conversationBatchMsg{messages: messages}
		}

		timer := time.NewTimer(conversationBatchWindow)
		defer timer.Stop()
		for len(messages) < conversationBatchMax {
			select {
			case msg := <-ch:
				if msg != nil {
					messages = append(messages, msg)
				}
				if isImmediateConversationMessage(msg) {
					return conversationBatchMsg{messages: messages}
				}
			case <-timer.C:
				return conversationBatchMsg{messages: messages}
			}
		}
		return conversationBatchMsg{messages: messages}
	}
}

func isImmediateConversationMessage(msg tea.Msg) bool {
	switch msg.(type) {
	case executeDoneMsg, streamCancelDoneMsg, harnessPermissionRequestMsg, harnessQuestionRequestMsg, executePairMsg:
		return true
	default:
		return false
	}
}

func (m model) cancelStreamCmd() tea.Cmd {
	ch := m.convStreamCh
	executes := make([]convExecute, 0, len(m.executes))
	for _, ex := range m.executes {
		executes = append(executes, ex)
	}
	fallbackAdapter := m.harnessAdapter()
	fallbackSession := m.harnessSessionID
	svc := m.svc
	return func() tea.Msg {
		var err error
		if len(executes) == 0 {
			if fallbackAdapter != nil {
				err = fallbackAdapter.Cancel(context.Background(), fallbackSession)
			}
		}
		for _, ex := range executes {
			adapter := fallbackAdapter
			if svc != nil && svc.Harness == nil && svc.Registry != nil && strings.TrimSpace(ex.HarnessID) != "" {
				if a, aerr := svc.Registry.Adapter(ex.HarnessID); aerr == nil && a != nil {
					adapter = a
				}
			}
			if adapter == nil {
				continue
			}
			sid := strings.TrimSpace(ex.SessionID)
			if sid == "" {
				sid = fallbackSession
			}
			if cerr := adapter.Cancel(context.Background(), sid); cerr != nil {
				slog.Error("tui stream cancel failed", "execute", ex.ID, "error", cerr)
				err = cerr
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
	case conversationBatchMsg:
		for _, item := range msg.messages {
			if item == nil {
				continue
			}
			updated, _ := m.handleConversationMsg(item)
			next, ok := updated.(model)
			if !ok {
				return updated, nil
			}
			m = next
			if !m.streaming {
				break
			}
		}
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvBatchMsg(m.convStreamCh)
		}
		return m, nil

	case streamDeltaMsg:
		m = m.bindExecuteView(msg.executeID)
		if sid := strings.TrimSpace(msg.delta.SessionID); sid != "" {
			if ex, ok := m.executes[msg.executeID]; ok {
				ex.SessionID = sid
				m.executes[msg.executeID] = ex
			}
			hid := ""
			if ex, ok := m.executes[msg.executeID]; ok {
				hid = ex.HarnessID
			}
			m = m.persistHarnessSession(sid, hid)
		}
		prevActivity := m.harnessWatchdog.LastActivityAt()
		m.harnessWatchdog.RecordDelta(msg.delta, time.Now())
		if m.harnessWatchdog.LastActivityAt().After(prevActivity) {
			m = m.clearHarnessHealthWarnings()
		}
		m = m.appendStreamDelta(msg.delta)
		m = m.maybeFollowTranscriptBottom()
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvBatchMsg(m.convStreamCh)
		}
		return m, nil

	case executePairMsg:
		m = m.bindExecuteView(msg.executeID)
		m = m.applyExecutePair(msg.harnessID, msg.model)
		if ex, ok := m.executes[msg.executeID]; ok && msg.executeID != "" {
			if msg.harnessID != "" {
				ex.HarnessID = msg.harnessID
			}
			m.executes[msg.executeID] = ex
		}
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvBatchMsg(m.convStreamCh)
		}
		return m, nil

	case harnessPermissionRequestMsg:
		m.harnessPermissionPending = true
		m.harnessPermissionReq = msg.req
		m.harnessPermissionRespCh = msg.respCh
		m.harnessPermissionMsg = formatHarnessPermission(msg.req)
		m.insertBeforeAgent(convMessage{role: convRoleWarning, content: m.harnessPermissionMsg})
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvBatchMsg(m.convStreamCh)
		}
		return m, nil

	case harnessQuestionRequestMsg:
		m.harnessQuestionPending = true
		m.harnessQuestionReq = msg.req
		m.harnessQuestionRespCh = msg.respCh
		m.harnessQuestionIndex = 0
		m.harnessQuestionAnswers = nil
		m.harnessQuestionMsg = formatHarnessQuestion(msg.req, 0)
		m.insertBeforeAgent(convMessage{role: convRoleWarning, content: m.harnessQuestionMsg})
		m.chatInputFocused = true
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvBatchMsg(m.convStreamCh)
		}
		return m, nil

	case executeDoneMsg:
		m = m.bindExecuteView(msg.executeID)
		stageForMetrics := strings.TrimSpace(m.conversationStage)
		agentForMetrics := strings.TrimSpace(m.runtimeAgentName)
		modelForMetrics := m.conversationModelSlug()
		if ex, ok := m.executes[msg.executeID]; ok {
			if strings.TrimSpace(ex.AgentName) != "" {
				agentForMetrics = ex.AgentName
			}
			if msg.result != nil && strings.TrimSpace(msg.result.SessionID) != "" {
				ex.SessionID = msg.result.SessionID
				m.executes[msg.executeID] = ex
			}
		}
		if m.stageHandoffLive && msg.result != nil {
			if out := strings.TrimSpace(msg.result.Output); out != "" {
				label := agentForMetrics
				if label == "" {
					label = "agent"
				}
				m.stageHandoffOutputs = append(m.stageHandoffOutputs, label+":\n"+out)
			}
		}
		if msg.executeID != "" {
			delete(m.executes, msg.executeID)
			m = m.removeLiveAgent(msg.executeID)
		}
		siblingsRemain := len(m.executes) > 0
		if !siblingsRemain {
			m.streaming = false
			m.convStreamCh = nil
			m.chatInputFocused = true
			m.liveAgents = nil
			m.confirmPending = false
			m.confirmMsg = ""
			m.harnessHealthInFlight = false
			m.harnessHealthStatus = harness.HealthHealthy
			m = m.clearHarnessPermission()
			m = m.clearHarnessQuestion()
			if !m.orchestrationLive {
				m = m.clearRuntimePair()
				m.runtimeAgentName = ""
				m.workflowProps = nil
			}
		}
		if msg.err != nil {
			errText := msg.err.Error()
			m.convError = errText
			label := "execute"
			if m.actionBusy && strings.TrimSpace(m.statusLabel) != "" {
				label = m.statusLabel
			}
			if !siblingsRemain {
				m = m.setStatusResult(false, label, firstStatusLine(errText))
			}
			if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
				existing := strings.TrimSpace(m.transcript[m.agentMsgIndex].content)
				if existing == "" {
					m.transcript[m.agentMsgIndex].content = "✗ " + errText
				} else {
					m.transcript[m.agentMsgIndex].content = existing + "\n✗ " + errText
				}
				m.transcript[m.agentMsgIndex].failed = true
				m.invalidateResponseCache(m.agentMsgIndex)
			}
			slog.Error("tui conversation execute failed", "error", msg.err)
			if siblingsRemain && m.convStreamCh != nil {
				return m, waitConvBatchMsg(m.convStreamCh)
			}
			return m, nil
		}
		if msg.result != nil {
			usage := harness.ResolveUsage(msg.result.Usage, m.lastExecutePrompt, msg.result.Output)
			m.contextUsedTokens = usage.InputTokens + usage.OutputTokens
			if m.svc != nil && stageForMetrics != "" {
				if err := m.svc.AccumulateStageHarnessMetrics(
					stageForMetrics, agentForMetrics, modelForMetrics, usage, msg.result.Duration,
				); err != nil {
					slog.Debug("tui accumulate stage metrics failed", "error", err)
				}
			}
			if msg.result.SessionID != "" && !m.stageHandoffLive {
				m = m.persistHarnessSession(msg.result.SessionID, msg.harnessID)
			}
			if msg.result.Output != "" {
				m = m.reconcileParentAgentOutput(msg.result.Output)
			}
		}
		if agentResponseEmpty(m, msg) {
			m = m.warnEmptyAgentResponse()
		}
		if m.runtimeCommandName == "new" && msg.err == nil && m.svc != nil && !siblingsRemain {
			if _, err := m.svc.PrepareCycle(); err != nil {
				m.convError = err.Error()
				slog.Error("tui prepare cycle after hero-new failed", "error", err)
			}
		}
		m = m.maybeFollowTranscriptBottom()
		slog.Info("tui conversation execute complete", "stage", m.conversationStage, "remaining", len(m.executes))
		if siblingsRemain {
			if m.convStreamCh != nil {
				return m, waitConvBatchMsg(m.convStreamCh)
			}
			return m, nil
		}
		next, handoffCmd := m.maybeHandoffAfterExecute()
		if handoffCmd != nil {
			return next, tea.Batch(next.refreshCmd(), handoffCmd, convWaitTickCmd())
		}
		next = next.completeBusyExecuteStatus(true, busyExecuteCompletedText(next.statusLabel))
		return next, next.refreshCmd()

	case streamCancelDoneMsg:
		m.streaming = false
		m.streamInterrupted = true
		m.convStreamCh = nil
		m.chatInputFocused = true
		m.liveAgents = nil
		m.executes = nil
		m.stageHandoffLive = false
		m.stageHandoffDoneKey = ""
		m.confirmPending = false
		m.confirmMsg = ""
		m = m.clearHarnessPermission()
		m = m.clearHarnessQuestion()
		if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
			m.transcript[m.agentMsgIndex].interrupted = true
			m.invalidateResponseCache(m.agentMsgIndex)
		}
		m = m.completeBusyExecuteStatus(false, "cancelled")
		slog.Info("tui conversation interrupted")
		return m, nil
	}
	return m, nil
}

func (m model) bindExecuteView(executeID string) model {
	executeID = strings.TrimSpace(executeID)
	if executeID == "" {
		return m
	}
	ex, ok := m.executes[executeID]
	if !ok {
		return m
	}
	m.agentMsgIndex = ex.AgentMsgIndex
	if strings.TrimSpace(ex.AgentName) != "" {
		m.runtimeAgentName = ex.AgentName
	}
	if strings.TrimSpace(ex.HarnessID) != "" {
		m.runtimeHarnessID = ex.HarnessID
	}
	return m
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
		text := strings.TrimSpace(d.Text)
		if text == "" {
			name := strings.TrimSpace(d.AgentName)
			if name == "" {
				name = "task"
			}
			text = "Task " + name
		}
		m.insertBeforeAgent(convMessage{
			role:      convRoleTool,
			content:   text,
			agentName: d.AgentName,
			modelSlug: d.Model,
			harnessID: m.agentHarnessForName(d.AgentName),
			callID:    d.CallID,
		})
		return m
	}
	switch d.Kind {
	case harness.StreamKindPermission, harness.StreamKindQuestion:
		return m
	case harness.StreamKindSession:
		if d.Metadata != nil && d.Metadata["state"] == harness.SessionStateFailed {
			m.convError = d.Text
			m = m.setStatusResult(false, "harness", firstStatusLine(d.Text))
		}
		return m
	case harness.StreamKindWarning:
		slog.Warn("harness stream warning", "harness_type", d.HarnessType, "text", d.Text)
		// UI-C06-001 §5 / D11: yellow status-area warning (not raw JSON in assistant text).
		m = m.setStatusWarning("harness", firstStatusLine(d.Text))
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
			m.invalidateResponseCache(m.thinkingMsgIndex)
			return m
		}
		m.thinkingMsgIndex = m.insertBeforeAgent(convMessage{
			role:      convRoleThinking,
			content:   d.Text,
			agentName: d.AgentName,
			modelSlug: d.Model,
			harnessID: m.agentHarnessForName(d.AgentName),
		})
		return m
	case harness.StreamKindTool:
		m.insertBeforeAgent(convMessage{
			role:      convRoleTool,
			content:   d.Text,
			agentName: d.AgentName,
			modelSlug: d.Model,
			harnessID: m.agentHarnessForName(d.AgentName),
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
		return m, waitConvBatchMsg(m.convStreamCh)
	}
	return m, nil
}

func (m model) applyExecutePair(harnessID, model string) model {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	model = strings.TrimSpace(model)
	if harnessID != "" {
		m.runtimeHarnessID = harnessID
	}
	if model != "" {
		m.runtimeModelSlug = model
	}
	if len(m.liveAgents) > 0 {
		updated := false
		for i, a := range m.liveAgents {
			if ex, ok := m.executes[a.CallID]; ok && ex.AgentMsgIndex == m.agentMsgIndex {
				if model != "" {
					a.Model = model
				}
				if harnessID != "" {
					a.Harness = harnessID
				}
				m.liveAgents[i] = a
				updated = true
			}
		}
		if !updated {
			a := m.liveAgents[0]
			if strings.TrimSpace(a.CallID) == "" {
				if model != "" {
					a.Model = model
				}
				if harnessID != "" {
					a.Harness = harnessID
				}
				m.liveAgents[0] = a
			}
		}
	}
	if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
		msg := m.transcript[m.agentMsgIndex]
		if model != "" {
			msg.modelSlug = model
		}
		if harnessID != "" {
			msg.harnessID = harnessID
		}
		m.transcript[m.agentMsgIndex] = msg
		m.invalidateResponseCache(m.agentMsgIndex)
	}
	return m
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
	if name == "" {
		return m
	}
	switch {
	case isKnownHeroAgent(name):
		if strings.TrimSpace(d.Text) != "" && !looksLikeTaskTool(d.Text) {
			return m
		}
	case harness.IsGenericTaskType(name), looksLikeTaskTool(d.Text):
	default:
		return m
	}
	m.liveAgents = append(m.liveAgents, liveAgent{
		CallID:  callID,
		Name:    name,
		Label:   agentShortLabel(name),
		Model:   strings.TrimSpace(d.Model),
		Harness: m.agentHarnessForName(name),
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

// reconcileParentAgentOutput replaces the parent agent message when Output is a
// longer prefix-superset of what the TUI streamed (or the parent slot is empty).
// Subagent rows are left untouched.
func (m model) reconcileParentAgentOutput(output string) model {
	if m.agentMsgIndex < 0 || m.agentMsgIndex >= len(m.transcript) {
		return m
	}
	streamed := m.transcript[m.agentMsgIndex].content
	if !shouldReplaceStreamedAgentText(streamed, output) {
		return m
	}
	m.transcript[m.agentMsgIndex].content = output
	m.invalidateResponseCache(m.agentMsgIndex)
	return m
}

// shouldReplaceStreamedAgentText is true when canonical Output repairs a
// truncated or empty live transcript without clobbering a longer divergent stream.
func shouldReplaceStreamedAgentText(streamed, output string) bool {
	if strings.TrimSpace(output) == "" {
		return false
	}
	if streamed == "" {
		return true
	}
	if streamed == output {
		return false
	}
	if strings.HasPrefix(output, streamed) {
		return true
	}
	// Byte-length fallback: Output is at least as complete as the stream.
	return len(output) >= len(streamed)
}

func (m *model) appendAttributed(role convRole, d harness.StreamDelta) {
	for i := m.agentMsgIndex - 1; i >= 0; i-- {
		msg := m.transcript[i]
		if msg.role == convRoleUser {
			break
		}
		if msg.role == role && msg.callID == d.CallID {
			m.transcript[i].content += d.Text
			m.invalidateResponseCache(i)
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
		harnessID: m.agentHarnessForName(d.AgentName),
		callID:    d.CallID,
	})
	if role == convRoleThinking {
		m.thinkingMsgIndex = idx
	}
}

func looksLikeTaskTool(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	return strings.HasPrefix(t, "task ") || t == "task" || harness.IsTaskToolName(t)
}

// insertBeforeAgent inserts msg just before the agent answer bubble and returns its index.
func (m *model) insertBeforeAgent(msg convMessage) int {
	if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
		idx := m.agentMsgIndex
		m.transcript = append(m.transcript[:idx], append([]convMessage{msg}, m.transcript[idx:]...)...)
		m.shiftExecuteIndexes(idx)
		m.agentMsgIndex++
		return idx
	}
	m.transcript = append(m.transcript, msg)
	return len(m.transcript) - 1
}

func (m *model) shiftExecuteIndexes(insertedAt int) {
	for id, ex := range m.executes {
		if ex.AgentMsgIndex >= insertedAt {
			ex.AgentMsgIndex++
			m.executes[id] = ex
		}
	}
}

func (m *model) appendAgentDelta(delta string) {
	if m.agentMsgIndex < 0 || m.agentMsgIndex >= len(m.transcript) {
		m.transcript = append(m.transcript, convMessage{role: convRoleAgent, content: delta})
		m.agentMsgIndex = len(m.transcript) - 1
		return
	}
	m.transcript[m.agentMsgIndex].content += delta
	m.invalidateResponseCache(m.agentMsgIndex)
}

func (m *model) invalidateResponseCache(index int) {
	if index < 0 || index >= len(m.transcript) {
		return
	}
	m.transcript[index].responseLines = nil
	m.transcript[index].responseLinesValid = false
}

func (m model) renderConversation(contentH int) string {
	n := m.transcriptVisibleLines(contentH)
	s := m.buildConversation(n)
	// The transcript pane absorbs leftover height after header, status hint,
	// input, and error rows. Shrink first so the frame never discards chrome.
	for countContentLines(s) > contentH && n > 0 {
		n--
		s = m.buildConversation(n)
	}
	// Grow into unused rows so the composer stays pinned to the bottom.
	for countContentLines(s) < contentH {
		prev := countContentLines(s)
		n++
		s = m.buildConversation(n)
		if countContentLines(s) <= prev {
			break
		}
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

func (m model) buildConversation(transcriptLines int) string {
	if transcriptLines < 0 {
		transcriptLines = 0
	}
	var b strings.Builder
	if header := m.renderConversationHeader(); header != "" {
		b.WriteString(header)
		b.WriteByte('\n')
	}

	b.WriteString(m.renderConversationTranscript(transcriptLines))

	if m.convError != "" && !m.latestAgentFailed() {
		b.WriteByte('\n')
		b.WriteString(m.renderWrappedConvError())
	}

	b.WriteString(m.renderChatSlashOverlay())
	b.WriteString(m.renderConversationInput())
	return strings.TrimRight(b.String(), "\n")
}

// renderConversationHeader returns narrow-terminal fallback chrome only. Harness
// and cycle context live in the sidebar and composer; the old "Chat · harness"
// row is intentionally omitted.
func (m model) renderConversationHeader() string {
	if m.sidebarVisible() {
		return ""
	}
	return m.renderAgentsBox()
}

func (m model) renderAgentsBox() string {
	innerW := agentsBoxWidth - chatBoxStyle.GetHorizontalFrameSize()
	if innerW < 8 {
		innerW = 8
	}
	body := strings.Join(m.agentsSidebarLines(innerW), "\n")
	return chatBoxStyle.Width(innerW).Render(body)
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

// renderConversationTranscript renders the full session as a linear, borderless
// scrollable chat with a thin │ accent bar per actor (Bonito-inspired).
func (m model) renderConversationTranscript(visibleLines int) string {
	rowW := m.transcriptRowWidth()
	contentW := m.transcriptTextWidth()
	lines := m.transcriptContentLines(contentW)
	visible := visibleLines
	if visible < 0 {
		visible = 0
	}
	offset := m.transcriptScrollOffset
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

	var b strings.Builder
	for i := 0; i < visible; i++ {
		idx := offset + i
		if idx < len(lines) {
			b.WriteString(padDisplayWidth(lines[idx], rowW))
		} else {
			b.WriteString(strings.Repeat(" ", rowW))
		}
		b.WriteByte('\n')
	}
	if hint := m.renderScrollHintLine(); hint != "" {
		b.WriteString(hint)
		b.WriteByte('\n')
	}
	return b.String()
}

func (m model) transcriptRowWidth() int {
	w := m.contentWidth()
	if w <= 0 {
		w = 72
	}
	if w < 28 {
		w = 28
	}
	return w
}

// transcriptTextWidth is columns after the thin bar + gap.
func (m model) transcriptTextWidth() int {
	w := m.transcriptRowWidth() - 2
	if w < 8 {
		w = 8
	}
	return w
}

func (m model) transcriptContentLines(contentW int) []string {
	rowW := m.transcriptRowWidth()
	if len(m.transcript) == 0 {
		if m.streaming || len(m.liveAgents) > 0 {
			header := m.responseSpeakerHeader()
			out := []string{chatThinBarRow(chatBarAgent, chatInAgent.Render(header), rowW)}
			if m.showChatWait() {
				out = append(out, m.chatWaitLine(rowW))
			}
			return out
		}
		if m.showChatWait() {
			return []string{m.chatWaitLine(rowW)}
		}
		return []string{chatThinBarRow(chatBarMuted, chatInMuted.Render("Submit a message to start an interação."), rowW)}
	}

	var out []string
	for i := range m.transcript {
		msg := &m.transcript[i]
		if msg.role == convRoleAgent && strings.TrimSpace(msg.content) == "" && m.streaming && !msg.failed && !msg.interrupted {
			// Keep a pending agent slot visible (header only) while waiting for text.
			if i == m.agentMsgIndex {
				bar, label := m.transcriptMessageChrome(*msg)
				out = append(out, chatThinBarRow(bar, label, rowW))
				if i < len(m.transcript)-1 {
					out = append(out, "")
				}
			}
			continue
		}
		bar, label := m.transcriptMessageChrome(*msg)
		out = append(out, chatThinBarRow(bar, label, rowW))
		body := m.transcriptMessageBody(msg, contentW)
		for _, line := range body {
			out = append(out, chatThinBarRow(bar, line, rowW))
		}
		if i < len(m.transcript)-1 {
			out = append(out, "")
		}
	}
	if m.showChatWait() {
		if len(out) == 0 {
			header := m.responseSpeakerHeader()
			out = []string{chatThinBarRow(chatBarAgent, chatInAgent.Render(header), rowW)}
		}
		out = append(out, m.chatWaitLine(rowW))
	}
	return out
}

func (m model) transcriptMessageChrome(msg convMessage) (lipgloss.Style, string) {
	switch msg.role {
	case convRoleUser:
		return chatBarUser, chatInUser.Render("You")
	case convRoleThinking:
		header := formatAgentHeader(msg.agentName, msg.modelSlug, m.harnessForMessage(msg))
		return chatBarMuted, chatInThink.Render(header)
	case convRoleTool, convRoleActivity:
		header := formatAgentHeader(msg.agentName, msg.modelSlug, m.harnessForMessage(msg))
		return chatBarMuted, chatInMuted.Render(header)
	case convRoleWarning:
		header := formatAgentHeader(msg.agentName, msg.modelSlug, m.harnessForMessage(msg))
		return chatBarWarn, chatInWarn.Render(header)
	default:
		header := formatAgentHeader(msg.agentName, msg.modelSlug, m.harnessForMessage(msg))
		style := chatInAgent
		bar := chatBarAgent
		if msg.failed {
			style = chatInErr
			bar = chatBarErr
		} else if msg.interrupted {
			style = chatInWarn
			bar = chatBarWarn
		}
		return bar, style.Render(header)
	}
}

func (m model) transcriptMessageBody(msg *convMessage, contentW int) []string {
	if msg.role == convRoleUser {
		var out []string
		for _, line := range splitOutputLines(msg.content, contentW) {
			out = append(out, chatInText.Render(line))
		}
		if len(out) == 0 {
			out = append(out, "")
		}
		return out
	}
	return m.cachedResponseLines(msg, contentW)
}

// transcriptVisibleLines sizes the linear transcript so it absorbs leftover height.
func (m model) transcriptVisibleLines(contentH int) int {
	if contentH <= 0 {
		contentH = m.contentAreaHeight()
	}
	base := countContentLines(m.buildConversation(0))
	n := contentH - base
	if n < chatTranscriptMinLines {
		return chatTranscriptMinLines
	}
	return n
}

func (m model) chatBoxWidth() int {
	// lipgloss Width is the inner content width; borders are drawn outside it.
	// Size the inner width so the full bordered box fits in contentWidth().
	outer := m.contentWidth()
	if outer <= 0 {
		outer = 72
	}
	if outer < 28 {
		outer = 28
	}
	inner := outer - chatBoxStyle.GetHorizontalFrameSize()
	if inner < 10 {
		inner = 10
	}
	return inner
}

// chatInnerWidth is the width available inside the rounded border (excludes border cells).
func (m model) chatInnerWidth() int {
	inner := m.chatBoxWidth()
	if inner < 10 {
		inner = 10
	}
	return inner
}

// chatContentWidth is text columns after the accent bar + gap (composer).
func (m model) chatContentWidth() int {
	w := m.chatInnerWidth() - 2
	if w < 8 {
		w = 8
	}
	return w
}

func (m model) chatInputVisibleLines() int {
	if m.frameContentHeight() < 18 {
		return 1
	}
	return chatInputVisibleLines
}

func (m model) cachedResponseLines(msg *convMessage, contentW int) []string {
	if msg.responseLinesValid && msg.responseLinesWidth == contentW && msg.responseLinesRuntime == m.runtimeCommandName {
		return msg.responseLines
	}

	var lines []string
	switch msg.role {
	case convRoleThinking:
		text := formatChatAgentText(m.runtimeCommandName, "Thinking: "+msg.content)
		for _, line := range splitOutputLines(text, contentW) {
			lines = append(lines, chatInThink.Render(line))
		}
	case convRoleTool:
		text := formatChatAgentText(m.runtimeCommandName, "→ "+msg.content)
		for _, line := range splitOutputLines(text, contentW) {
			lines = append(lines, chatInMuted.Render(line))
		}
	case convRoleWarning:
		for _, line := range splitOutputLines(msg.content, contentW) {
			lines = append(lines, chatInWarn.Render(line))
		}
	case convRoleActivity:
		text := formatChatAgentText(m.runtimeCommandName, "· "+msg.content)
		for _, line := range splitOutputLines(text, contentW) {
			lines = append(lines, chatInMuted.Render(line))
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
		if text != "" || !m.streaming {
			for _, line := range splitOutputLines(text, contentW) {
				lines = append(lines, style.Render(line))
			}
		}
	}

	msg.responseLines = lines
	msg.responseLinesWidth = contentW
	msg.responseLinesRuntime = m.runtimeCommandName
	msg.responseLinesValid = true
	return lines
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

func (m model) latestAgentTurnIndices() []int {
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
	indices := make([]int, 0, len(m.transcript)-start)
	for i := start; i < len(m.transcript); i++ {
		if m.transcript[i].role != convRoleUser {
			indices = append(indices, i)
		}
	}
	return indices
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
	visible := m.chatInputVisibleLines()
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

// chatAccentRow paints solid accent + gap + content to exactly innerW cells (composer).
func chatAccentRow(accent lipgloss.Style, content string, innerW int) string {
	if innerW < 3 {
		innerW = 3
	}
	contentW := innerW - 2 // accent + gap
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", "")
	if lipgloss.Width(content) > contentW {
		content = truncateDisplayWidth(content, contentW)
	}
	pad := contentW - lipgloss.Width(content)
	if pad < 0 {
		pad = 0
	}
	bar := accent.Width(1).Render(" ")
	return bar + " " + content + strings.Repeat(" ", pad)
}

// chatThinBarRow paints a thin │ accent + gap + content to exactly rowW cells.
func chatThinBarRow(barStyle lipgloss.Style, content string, rowW int) string {
	if rowW < 3 {
		rowW = 3
	}
	contentW := rowW - 2
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", "")
	if lipgloss.Width(content) > contentW {
		content = truncateDisplayWidth(content, contentW)
	}
	pad := contentW - lipgloss.Width(content)
	if pad < 0 {
		pad = 0
	}
	return barStyle.Render(chatBarGlyph()) + " " + content + strings.Repeat(" ", pad)
}

// padDisplayWidth right-pads a (possibly styled) line to exactly width cells.
func padDisplayWidth(s string, width int) string {
	if width < 1 {
		return ""
	}
	w := lipgloss.Width(s)
	if w > width {
		return truncateDisplayWidth(s, width)
	}
	if w == width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// truncateDisplayWidth clips styled/plain text to maxCells, appending "…" when clipped.
func truncateDisplayWidth(s string, maxCells int) string {
	if maxCells < 1 {
		return ""
	}
	if lipgloss.Width(s) <= maxCells {
		return s
	}
	ellipsis := "…"
	if maxCells == 1 {
		return ellipsis
	}
	budget := maxCells - lipgloss.Width(ellipsis)
	if budget < 1 {
		return ellipsis
	}
	// Prefer lipgloss MaxWidth so ANSI sequences stay intact, then mark the clip.
	clipped := lipgloss.NewStyle().MaxWidth(budget).Render(s)
	return clipped + ellipsis
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
	h := m.frameContentHeight()
	if h < 3 {
		h = 3
	}
	return h
}

func (m model) maxTranscriptScroll() int {
	lines := len(m.transcriptContentLines(m.transcriptTextWidth()))
	maxOff := lines - m.transcriptVisibleLines(m.contentAreaHeight())
	if maxOff < 0 {
		return 0
	}
	return maxOff
}

func (m model) scrollTranscript(delta int) model {
	m.transcriptScrollOffset += delta
	maxOff := m.maxTranscriptScroll()
	if m.transcriptScrollOffset < 0 {
		m.transcriptScrollOffset = 0
	}
	if m.transcriptScrollOffset > maxOff {
		m.transcriptScrollOffset = maxOff
	}
	m.transcriptFollowBottom = m.transcriptScrollOffset >= maxOff
	return m
}

func (m model) maybeFollowTranscriptBottom() model {
	if !m.transcriptFollowBottom {
		return m
	}
	m.transcriptScrollOffset = m.maxTranscriptScroll()
	return m
}

func (m model) copyChatResponse() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.transcriptPlainText())
	if text == "" {
		return m.setStatusResult(false, "copy", "nothing to copy"), nil
	}
	return m.setStatusResult(true, "copy", "response copied"), copyToClipboardCmd(text)
}

func (m model) copyChatInput() (tea.Model, tea.Cmd) {
	text := m.input
	if strings.TrimSpace(text) == "" {
		return m.setStatusResult(false, "copy", "nothing to copy"), nil
	}
	return m.setStatusResult(true, "copy", "input copied"), copyToClipboardCmd(text)
}

// transcriptPlainText renders the whole chat box (You + every agent turn) as
// plain text for clipboard copy. It keeps speaker headers and thinking/tool
// markers but omits the per-line accent bars and right-padding that decorate
// the rendered box.
func (m model) transcriptPlainText() string {
	if len(m.transcript) == 0 {
		return ""
	}
	var parts []string
	prevKey := ""
	prevWasSub := false
	for i := range m.transcript {
		msg := &m.transcript[i]
		if msg.role == convRoleUser {
			content := strings.TrimSpace(msg.content)
			if content == "" {
				continue
			}
			if len(parts) > 0 {
				parts = append(parts, "")
			}
			parts = append(parts, "You", content)
			prevKey = ""
			prevWasSub = false
			continue
		}
		if msg.role == convRoleAgent && strings.TrimSpace(msg.content) == "" && m.streaming && !msg.failed && !msg.interrupted {
			continue
		}
		key := messageAgentKey(*msg)
		isSub := strings.TrimSpace(msg.callID) != ""
		if key != prevKey {
			if prevKey != "" && prevWasSub {
				parts = append(parts, "")
			}
			if isSub {
				parts = append(parts, "")
			}
			header := formatAgentHeader(msg.agentName, msg.modelSlug, m.harnessForMessage(*msg))
			parts = append(parts, header)
			prevKey = key
			prevWasSub = isSub
		}
		switch msg.role {
		case convRoleThinking:
			parts = append(parts, formatChatAgentText(m.runtimeCommandName, "Thinking: "+msg.content))
		case convRoleTool:
			parts = append(parts, formatChatAgentText(m.runtimeCommandName, "→ "+msg.content))
		case convRoleWarning:
			parts = append(parts, msg.content)
		case convRoleActivity:
			parts = append(parts, formatChatAgentText(m.runtimeCommandName, "· "+msg.content))
		case convRoleAgent:
			text := msg.content
			if msg.interrupted && text == "" {
				text = "Interrupted"
			} else if msg.interrupted && text != "" {
				text += "\n[Interrupted]"
			}
			text = formatChatAgentText(m.runtimeCommandName, text)
			if text == "" && m.streaming {
				continue
			}
			parts = append(parts, text)
		}
	}
	if prevWasSub {
		parts = append(parts, "")
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func (m model) maxInputScroll() int {
	lines := len(m.inputContentLines(m.chatContentWidth()))
	maxOff := lines - m.chatInputVisibleLines()
	if maxOff < 0 {
		return 0
	}
	return maxOff
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
	visible := m.chatInputVisibleLines()
	if caretLine >= m.inputScrollOffset+visible {
		m.inputScrollOffset = caretLine - visible + 1
	}
	maxOff := len(lines) - visible
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
