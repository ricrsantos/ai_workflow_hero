package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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

func (m model) enterConversation() (model, tea.Cmd) {
	m.screen = screenConversation
	m.chatInputFocused = true
	m = m.syncConversationContext()
	m = m.clampInputCursor()
	return m, nil
}

func (m model) handleConversationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	if m.streaming {
		switch s {
		case "ctrl+c":
			return m, m.cancelStreamCmd()
		case "ctrl+q":
			return m, tea.Quit
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
	case "left":
		if m.inputCursor > 0 {
			m.inputCursor--
		}
		return m, nil
	case "right":
		if m.inputCursor < runeLen(m.input) {
			m.inputCursor++
		}
		return m, nil
	case "home":
		m.inputCursor = 0
		return m, nil
	case "end":
		m.inputCursor = runeLen(m.input)
		return m, nil
	case "backspace":
		m = m.deleteRuneBeforeCursor()
		return m, nil
	case "delete":
		m = m.deleteRuneAtCursor()
		return m, nil
	default:
		if len(msg.Runes) == 0 || msg.Alt {
			return m, nil
		}
		m = m.insertRunesAtCursor(msg.Runes)
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
	m = m.syncConversationContext()
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: text})
	m.input = ""
	m.inputCursor = 0
	m.streamInterrupted = false
	m.convError = ""
	m.streaming = true
	m.chatInputFocused = false
	m.transcript = append(m.transcript, convMessage{role: convRoleAgent, content: ""})
	m.agentMsgIndex = len(m.transcript) - 1
	m.thinkingMsgIndex = -1

	ch := make(chan tea.Msg, 32)
	m.convStreamCh = ch
	m.startConversationExecute(text, ch)
	return m, waitConvMsg(ch)
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
		slog.Info("tui conversation execute complete", "stage", m.conversationStage)
		return m, nil

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

func (m model) appendAgentDelta(delta string) {
	if m.agentMsgIndex < 0 || m.agentMsgIndex >= len(m.transcript) {
		m.transcript = append(m.transcript, convMessage{role: convRoleAgent, content: delta})
		m.agentMsgIndex = len(m.transcript) - 1
		return
	}
	m.transcript[m.agentMsgIndex].content += delta
}

func (m model) renderConversation() string {
	var b strings.Builder
	stage := m.conversationStage
	tool := m.conversationHarnessTool()
	if stage == "" {
		header := fmt.Sprintf("Free chat · harness %s", tool)
		b.WriteString(headerStyle.Render(header))
		b.WriteByte('\n')
		if m.harnessSessionID != "" {
			b.WriteString(mutedStyle.Render("session: " + truncateSessionID(m.harnessSessionID)))
			b.WriteByte('\n')
		}
		b.WriteString(mutedStyle.Render("No active etapa — chatting with harness defaults. /hero-new then /hero-start for a cycle."))
		b.WriteByte('\n')
		b.WriteByte('\n')
	} else {
		iter := ""
		for _, st := range m.status.Stages {
			if strings.EqualFold(st.Name, stage) {
				iter = st.Iteration
				break
			}
		}
		header := fmt.Sprintf("Cycle C%d — %s · iter %s", m.status.CycleNumber, stage, iter)
		b.WriteString(headerStyle.Render(header))
		b.WriteByte('\n')
		if m.harnessSessionID != "" {
			b.WriteString(mutedStyle.Render("session: " + truncateSessionID(m.harnessSessionID)))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	if len(m.transcript) == 0 {
		b.WriteString(mutedStyle.Render("Submit a message to start an interação."))
		b.WriteByte('\n')
	} else {
		for _, msg := range m.transcript {
			prefix := "You: "
			style := infoStyle
			switch msg.role {
			case convRoleThinking:
				prefix = "Thinking: "
				style = thinkingStyle
			case convRoleTool:
				prefix = "→ "
				style = mutedStyle
			case convRoleAgent:
				prefix = "Agent: "
				style = lipglossStyleForAgent(msg.interrupted)
			}
			line := prefix + msg.content
			if msg.interrupted && msg.content == "" {
				line = prefix + "Interrupted"
			}
			b.WriteString(style.Render(wrapWidth(line, m.width)))
			b.WriteByte('\n')
		}
	}

	if m.streaming {
		b.WriteByte('\n')
		b.WriteString(infoStyle.Render("→ Agent responding…"))
		b.WriteByte('\n')
	}
	if m.convError != "" {
		b.WriteByte('\n')
		b.WriteString(errorStyle.Render("✗ " + m.convError))
		b.WriteByte('\n')
		if strings.Contains(strings.ToLower(m.convError), "auth") ||
			strings.Contains(strings.ToLower(m.convError), "login") {
			b.WriteString(mutedStyle.Render("→ Check harness login: cursor agent login"))
			b.WriteByte('\n')
		}
	}

	b.WriteByte('\n')
	b.WriteString(m.renderConversationInput())
	return b.String()
}

func (m model) renderConversationInput() string {
	var b strings.Builder
	boxW := m.width
	if boxW <= 0 {
		boxW = 72
	}
	if boxW > 88 {
		boxW = 88
	}
	if boxW < 28 {
		boxW = 28
	}
	// Border (2) + right padding (1); left is flush for the accent bar.
	innerW := boxW - 3
	if innerW < 16 {
		innerW = 16
	}
	contentW := innerW - 1 // minus accent column
	if contentW < 12 {
		contentW = 12
	}

	mode := m.chatMode
	if mode == "" {
		mode = harness.ModeBuild
	}
	accent := chatAccentBuild
	modeStyle := chatModeBuildStyle
	modeLabel := "Build"
	if mode == harness.ModePlan {
		accent = chatAccentPlan
		modeStyle = chatModePlanStyle
		modeLabel = "Plan"
	}

	var textContent string
	if m.streaming {
		textContent = chatMutedInBox.Render(m.input)
	} else {
		textContent = m.renderInputWithCaret()
	}

	statusContent := modeStyle.Render(modeLabel) +
		chatMutedInBox.Render(" · ") +
		chatModelStyle.Render(m.conversationModelSlug()) +
		chatMutedInBox.Render(" · ") +
		chatMutedInBox.Render(m.conversationHarnessTool())

	// Accent bar on every row so it runs full height, flush to the left border.
	body := strings.Join([]string{
		chatAccentRow(accent, textContent, contentW),
		chatAccentRow(accent, "", contentW),
		chatAccentRow(accent, statusContent, contentW),
	}, "\n")
	b.WriteString(chatBoxStyle.Render(body))
	b.WriteByte('\n')
	if !m.streaming {
		b.WriteString(mutedStyle.Render("tab mode · / commands · enter send · ←→ move"))
	} else {
		b.WriteString(mutedStyle.Render("ctrl+c interrupt"))
	}
	return b.String()
}

// chatAccentRow paints a 1-cell mode-colored bar + padded content on chatBg.
func chatAccentRow(accent lipgloss.Style, content string, contentW int) string {
	bar := accent.Width(1).Render(" ")
	rest := chatLineStyle.PaddingLeft(1).Width(contentW).Render(content)
	return bar + rest
}

func (m model) renderInputWithCaret() string {
	runes := []rune(m.input)
	cur := m.inputCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	before := chatTextStyle.Render(string(runes[:cur]))
	after := chatTextStyle.Render(string(runes[cur:]))
	return before + m.renderInputCaret() + after
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
