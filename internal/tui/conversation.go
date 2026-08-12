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
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

type convRole string

const (
	convRoleUser     convRole = "user"
	convRoleAgent    convRole = "agent"
	convRoleThinking convRole = "thinking"
	convRoleTool     convRole = "tool"
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
	interrupted bool
}

type streamDeltaMsg struct {
	kind  harness.StreamKind
	delta string
}

type executeDoneMsg struct {
	result *harness.ExecutionResult
	err    error
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
	if m.svc != nil {
		return cursoradapter.NewAdapter(m.svc.ProjectDir)
	}
	return nil
}

func (m model) syncConversationContext() model {
	if m.svc == nil {
		return m
	}
	stage, sessionID, err := m.svc.ConversationContext()
	if err != nil {
		slog.Debug("tui conversation context unavailable", "error", err)
		m.conversationStage = ""
		// Keep harnessSessionID for freechat resume within this TUI session.
		return m
	}
	m.conversationStage = stage
	m.harnessSessionID = sessionID
	return m
}

func (m model) conversationHarnessTool() string {
	if m.svc != nil && m.svc.Harness != nil {
		if name := strings.TrimSpace(m.svc.Harness.Name()); name != "" {
			return name
		}
	}
	return "cursor"
}

func (m model) conversationModelSlug() string {
	if strings.TrimSpace(m.chatModelSlug) != "" {
		return m.chatModelSlug
	}
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	return install.HarnessModelSlugForProject(projectDir, m.conversationHarnessTool())
}

func (m model) conversationModelLabel() string {
	if slug := m.conversationModelSlug(); slug != "" {
		return slug
	}
	return "not set"
}

func (m model) enterConversation() (model, tea.Cmd) {
	m.screen = screenConversation
	m.chatInputFocused = true
	m = m.syncConversationContext()
	m = m.clampInputCursor()
	return m, nil
}

// beginHeroRuntimeConversation opens Chat and executes a Hero runtime command markdown
// (same body as Cursor slash expansion) with the default harness model.
func (m model) beginHeroRuntimeConversation(cmdName string) (model, tea.Cmd) {
	if m.svc == nil {
		m, _ = m.enterConversation()
		m.convError = "cycle service unavailable"
		return m, nil
	}
	label := "/hero-" + cmdName
	path := filepath.Join(m.svc.ProjectDir, cursoradapter.CommandsDir, "hero-"+cmdName+".md")
	prompt, err := cursoradapter.ReadCommandPrompt(path)
	if err != nil {
		slog.Error("tui hero runtime command read failed", "path", path, "error", err)
		m, _ = m.enterConversation()
		m.convError = fmt.Errorf("read command %s: %w", label, err).Error()
		return m, nil
	}
	m, _ = m.enterConversation()
	m.conversationStage = ""
	m.harnessSessionID = ""
	m.runtimeCommandName = cmdName
	m = m.beginConversationExecute(label, tuiRuntimeCommandPrompt(cmdName, prompt))
	return m, tea.Batch(waitConvMsg(m.convStreamCh), convWaitTickCmd())
}

func (m model) beginConversationExecute(userLabel, executePrompt string) model {
	m.streamInterrupted = false
	m.convError = ""
	m.streaming = true
	m.waitAnimFrame = 0
	m.respFollowBottom = true
	m.respScrollOffset = 0
	m.chatInputFocused = false
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: userLabel})
	m.transcript = append(m.transcript, convMessage{role: convRoleAgent, content: ""})
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
			return m, tea.Quit
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
	switch s {
	case "ctrl+q", "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5", "ctrl+6",
		"alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6",
		"/", "ctrl+r", "f5":
		return m.handleKey(msg)
	}

	switch s {
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
		m = m.deleteRuneBeforeCursor()
		m = m.ensureInputCaretVisible()
		return m, nil
	case "delete":
		m = m.deleteRuneAtCursor()
		m = m.ensureInputCaretVisible()
		return m, nil
	default:
		if len(msg.Runes) == 0 || msg.Alt {
			return m, nil
		}
		m = m.insertRunesAtCursor(msg.Runes)
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
		return m, nil
	}
	var cmd tea.Cmd
	m, cmd, ok := m.ensureDefaultModel("chat")
	if !ok {
		return m, cmd
	}
	m.runtimeCommandName = ""
	m = m.syncConversationContext()
	m.input = ""
	m.inputCursor = 0
	m.inputScrollOffset = 0
	m = m.beginConversationExecute(text, text)
	return m, tea.Batch(waitConvMsg(m.convStreamCh), convWaitTickCmd())
}

func (m model) startConversationExecute(prompt string, ch chan<- tea.Msg) {
	svc := m.svc
	adapter := m.harnessAdapter()
	stageName := m.conversationStage
	sessionID := m.harnessSessionID
	modelSlug := m.conversationModelSlug()
	projectDir := ""
	if svc != nil {
		projectDir = svc.ProjectDir
	}
	mode := m.chatMode
	if mode == "" {
		mode = harness.ModeBuild
	}
	go func() {
		if adapter == nil {
			ch <- executeDoneMsg{err: fmt.Errorf("harness adapter unavailable")}
			return
		}
		ctx := context.Background()
		req := harness.ExecuteRequest{
			ProjectDir: projectDir,
			Prompt:     prompt,
			SessionID:  sessionID,
			Stream:     true,
			StageName:  stageName,
			Model:      modelSlug,
			Mode:       mode,
			OnStreamDelta: func(delta harness.StreamDelta) {
				ch <- streamDeltaMsg{kind: delta.Kind, delta: delta.Text}
			},
		}
		res, err := adapter.Execute(ctx, req)
		ch <- executeDoneMsg{result: res, err: err}
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
		if adapter != nil && sessionID != "" {
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
		m = m.appendStreamDelta(msg.kind, msg.delta)
		m = m.maybeFollowResponseBottom()
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvMsg(m.convStreamCh)
		}
		return m, nil

	case executeDoneMsg:
		m.streaming = false
		m.convStreamCh = nil
		m.chatInputFocused = true
		if msg.err != nil {
			m.convError = msg.err.Error()
			slog.Error("tui conversation execute failed", "error", msg.err)
			return m, nil
		}
		if msg.result != nil {
			if msg.result.SessionID != "" {
				m.harnessSessionID = msg.result.SessionID
				if m.svc != nil && m.conversationStage != "" {
					if err := m.svc.SetStageHarnessSessionID(m.conversationStage, msg.result.SessionID); err != nil {
						slog.Error("tui persist harness session failed", "error", err)
					}
				}
			}
			// Prefer canonical result text over accumulated deltas (partial vs final).
			if msg.result.Output != "" && m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
				m.transcript[m.agentMsgIndex].content = msg.result.Output
			}
		}
		m = m.maybeFollowResponseBottom()
		slog.Info("tui conversation execute complete", "stage", m.conversationStage)
		return m, m.refreshCmd()

	case streamCancelDoneMsg:
		m.streaming = false
		m.streamInterrupted = true
		m.convStreamCh = nil
		m.chatInputFocused = true
		if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
			m.transcript[m.agentMsgIndex].interrupted = true
		}
		slog.Info("tui conversation interrupted")
		return m, nil
	}
	return m, nil
}

func (m model) appendStreamDelta(kind harness.StreamKind, delta string) model {
	if delta == "" {
		return m
	}
	switch kind {
	case harness.StreamKindThinking:
		if m.thinkingMsgIndex >= 0 && m.thinkingMsgIndex < len(m.transcript) &&
			m.transcript[m.thinkingMsgIndex].role == convRoleThinking &&
			m.thinkingMsgIndex == m.agentMsgIndex-1 {
			m.transcript[m.thinkingMsgIndex].content += delta
			return m
		}
		m.thinkingMsgIndex = m.insertBeforeAgent(convMessage{role: convRoleThinking, content: delta})
		return m
	case harness.StreamKindTool:
		m.insertBeforeAgent(convMessage{role: convRoleTool, content: delta})
		return m
	default:
		m.appendAgentDelta(delta)
		return m
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
	stage := m.conversationStage
	tool := m.conversationHarnessTool()
	if stage == "" {
		b.WriteString(headerStyle.Render(fmt.Sprintf("Chat · harness %s", tool)))
		if m.harnessSessionID != "" {
			b.WriteString(mutedStyle.Render(" │ "))
			b.WriteString(mutedStyle.Render("session: " + truncateSessionID(m.harnessSessionID)))
		}
		b.WriteByte('\n')
	} else {
		iter := ""
		for _, st := range m.status.Stages {
			if strings.EqualFold(st.Name, stage) {
				iter = st.Iteration
				break
			}
		}
		b.WriteString(headerStyle.Render(fmt.Sprintf("Cycle C%d — %s · iter %s", m.status.CycleNumber, stage, iter)))
		if m.harnessSessionID != "" {
			b.WriteString(mutedStyle.Render(" │ "))
			b.WriteString(mutedStyle.Render("session: " + truncateSessionID(m.harnessSessionID)))
		}
		b.WriteByte('\n')
	}

	b.WriteString(m.renderConversationHistory())
	b.WriteByte('\n')
	b.WriteString(m.renderConversationResponse(responseLines))

	if m.convError != "" {
		b.WriteByte('\n')
		b.WriteString(errorStyle.Render("✗ " + m.convError))
		b.WriteByte('\n')
		if strings.Contains(strings.ToLower(m.convError), "auth") ||
			strings.Contains(strings.ToLower(m.convError), "login") {
			b.WriteString(mutedStyle.Render("→ Check harness login: cursor agent login"))
			b.WriteByte('\n')
		}
		if strings.Contains(strings.ToLower(m.convError), "workspace trust") ||
			strings.Contains(strings.ToLower(m.convError), "trust required") {
			b.WriteString(mutedStyle.Render("→ Trust this folder in Cursor, or run: cursor agent --trust"))
			b.WriteByte('\n')
		}
	}

	b.WriteString(m.renderConversationInput())
	return strings.TrimRight(b.String(), "\n")
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

	statusContent := chatInAgent.Render("Agent") +
		chatInMuted.Render(" · ") +
		chatInModel.Render(m.conversationModelLabel()) +
		chatInMuted.Render(" · ") +
		chatInMuted.Render(m.conversationHarnessTool())
	if visible > 0 && len(lines) > visible {
		statusContent += chatInMuted.Render(fmt.Sprintf(" · %d–%d/%d", offset+1, minInt(offset+visible, len(lines)), len(lines)))
	}
	rows = append(rows, chatAccentRow(accent, statusContent, innerW))

	var b strings.Builder
	b.WriteString(chatBoxStyle.Width(m.chatBoxWidth()).Render(strings.Join(rows, "\n")))
	b.WriteByte('\n')
	if m.streaming {
		b.WriteString(mutedStyle.Render("↑↓ scroll · ctrl+c interrupt"))
	} else {
		b.WriteString(mutedStyle.Render("↑↓ scroll response"))
	}
	b.WriteByte('\n')
	return b.String()
}

func (m model) responseContentLines(contentW int) []string {
	turn := m.latestAgentTurn()
	if len(turn) == 0 {
		if m.streaming {
			frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
			return []string{chatInText.Render(frame + " Waiting for harness…")}
		}
		return []string{chatInMuted.Render("Agent response will appear here.")}
	}

	var out []string
	for _, msg := range turn {
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
		case convRoleAgent:
			text := msg.content
			if msg.interrupted && text == "" {
				text = "Interrupted"
			} else if msg.interrupted && text != "" {
				text += "\n[Interrupted]"
			}
			text = formatChatAgentText(m.runtimeCommandName, text)
			style := chatInOK
			if msg.interrupted {
				style = chatInWarn
			}
			if text == "" && m.streaming {
				frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
				out = append(out, chatInText.Render(frame+" Waiting for harness…"))
				continue
			}
			for _, line := range splitOutputLines(text, contentW) {
				out = append(out, style.Render(line))
			}
		}
	}
	if len(out) == 0 && m.streaming {
		frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
		return []string{chatInText.Render(frame + " Waiting for harness…")}
	}
	return out
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
		b.WriteString(mutedStyle.Render("tab mode · / commands · enter send · ←→ move · ↑↓ scroll"))
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
