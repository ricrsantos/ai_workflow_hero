package tui

import (
	"context"
	"fmt"
	"log/slog"
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
	convRoleUser  convRole = "user"
	convRoleAgent convRole = "agent"
)

type convMessage struct {
	role        convRole
	content     string
	interrupted bool
}

type streamDeltaMsg struct {
	delta string
}

type executeDoneMsg struct {
	result *harness.ExecutionResult
	err    error
}

type streamCancelDoneMsg struct {
	err error
}

type blinkCursorMsg struct{}

func blinkCursorCmd() tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(time.Time) tea.Msg {
		return blinkCursorMsg{}
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
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	return install.HarnessModelSlugForProject(projectDir, m.conversationHarnessTool())
}

func (m model) enterConversation() (model, tea.Cmd) {
	m.screen = screenConversation
	m.cursorBlinkOn = true
	m = m.syncConversationContext()
	return m, blinkCursorCmd()
}

func (m model) handleConversationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	if m.streaming {
		switch s {
		case "ctrl+c":
			return m, m.cancelStreamCmd()
		default:
			// Esc and other keys are ignored while waiting for the agent.
			return m, nil
		}
	}

	// Footer promises 1-6 / q / slash when the input buffer is empty.
	if m.input == "" {
		switch s {
		case "1", "2", "3", "4", "5", "6", "q", "/", "ctrl+r", "f5", "ctrl+c":
			return m.handleKey(msg)
		}
	}

	switch s {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.input = ""
		m.cursorBlinkOn = true
		return m, blinkCursorCmd()
	case "enter":
		if strings.TrimSpace(m.input) == "" {
			return m, nil
		}
		return m.submitConversation()
	case "backspace":
		if len(m.input) == 0 {
			return m, nil
		}
		m.input = m.input[:len(m.input)-1]
		m.cursorBlinkOn = true
		return m, blinkCursorCmd()
	default:
		if len(msg.Runes) == 0 || msg.Alt {
			return m, nil
		}
		m.input += string(msg.Runes)
		m.cursorBlinkOn = true
		return m, blinkCursorCmd()
	}
}

func (m model) submitConversation() (model, tea.Cmd) {
	text := strings.TrimSpace(m.input)
	if text == "" {
		return m, nil
	}
	m = m.syncConversationContext()
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: text})
	m.input = ""
	m.streamInterrupted = false
	m.convError = ""
	m.streaming = true
	m.transcript = append(m.transcript, convMessage{role: convRoleAgent, content: ""})
	m.agentMsgIndex = len(m.transcript) - 1

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
			OnStreamDelta: func(delta string) {
				ch <- streamDeltaMsg{delta: delta}
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
		m.appendAgentDelta(msg.delta)
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvMsg(m.convStreamCh)
		}
		return m, nil

	case executeDoneMsg:
		m.streaming = false
		m.convStreamCh = nil
		m.cursorBlinkOn = true
		if msg.err != nil {
			m.convError = msg.err.Error()
			slog.Error("tui conversation execute failed", "error", msg.err)
			return m, blinkCursorCmd()
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
			if msg.result.Output != "" && m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
				if m.transcript[m.agentMsgIndex].content == "" {
					m.transcript[m.agentMsgIndex].content = msg.result.Output
				}
			}
		}
		slog.Info("tui conversation execute complete", "stage", m.conversationStage)
		return m, blinkCursorCmd()

	case streamCancelDoneMsg:
		m.streaming = false
		m.streamInterrupted = true
		m.convStreamCh = nil
		m.cursorBlinkOn = true
		if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
			m.transcript[m.agentMsgIndex].interrupted = true
		}
		slog.Info("tui conversation interrupted")
		return m, blinkCursorCmd()
	}
	return m, nil
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
	modelSlug := m.conversationModelSlug()
	tool := m.conversationHarnessTool()
	if stage == "" {
		header := fmt.Sprintf("Free chat · harness %s · model %s", tool, modelSlug)
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
		header := fmt.Sprintf("Cycle C%d — %s · iter %s · model %s", m.status.CycleNumber, stage, iter, modelSlug)
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
			if msg.role == convRoleAgent {
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
	sepW := m.width
	if sepW <= 0 {
		sepW = 72
	}
	if sepW > 72 {
		sepW = 72
	}
	if sepW < 20 {
		sepW = 20
	}
	b.WriteString(mutedStyle.Render(strings.Repeat("─", sepW)))
	b.WriteByte('\n')

	if m.streaming {
		b.WriteString(mutedStyle.Render("Message (waiting for agent…)"))
		b.WriteByte('\n')
		b.WriteString(promptStyle.Render("> "))
		b.WriteString(mutedStyle.Render(m.input))
		return b.String()
	}

	b.WriteString(promptStyle.Render("Message"))
	b.WriteString(mutedStyle.Render("  ·  Enter send · Esc clear · 1-5 leave"))
	b.WriteByte('\n')
	b.WriteString(promptStyle.Render("> "))
	b.WriteString(m.input)
	b.WriteString(m.renderInputCaret())
	if m.input == "" {
		b.WriteString(mutedStyle.Render(" type here"))
	}
	return b.String()
}

func (m model) renderInputCaret() string {
	if m.streaming {
		return ""
	}
	// Blue block when "on"; thin pipe when "off" so a caret is always visible
	// even if blink ticks are delayed or the terminal ignores reverse video.
	if m.cursorBlinkOn {
		return caretStyle.Render(" ")
	}
	return promptStyle.Render("|")
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
