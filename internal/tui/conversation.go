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
		return m
	}
	m.conversationStage = stage
	m.harnessSessionID = sessionID
	return m
}

func (m model) enterConversation() model {
	m.screen = screenConversation
	m.flash = ""
	m = m.syncConversationContext()
	return m
}

func (m model) handleConversationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.streaming {
			return m, m.cancelStreamCmd()
		}
		return m, tea.Quit
	case "esc":
		if m.streaming {
			return m, nil
		}
		m.input = ""
		return m, nil
	case "enter":
		if m.streaming || strings.TrimSpace(m.input) == "" {
			return m, nil
		}
		return m.submitConversation()
	case "backspace":
		if m.streaming || len(m.input) == 0 {
			return m, nil
		}
		m.input = m.input[:len(m.input)-1]
		return m, nil
	default:
		if m.streaming || len(msg.Runes) == 0 || msg.Alt {
			return m, nil
		}
		m.input += string(msg.Runes)
		return m, nil
	}
}

func (m model) submitConversation() (model, tea.Cmd) {
	text := strings.TrimSpace(m.input)
	if text == "" {
		return m, nil
	}
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: text})
	m.input = ""
	m.streaming = true
	m.streamInterrupted = false
	m.convError = ""
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
			if msg.result.Output != "" && m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
				if m.transcript[m.agentMsgIndex].content == "" {
					m.transcript[m.agentMsgIndex].content = msg.result.Output
				}
			}
		}
		slog.Info("tui conversation execute complete", "stage", m.conversationStage)
		return m, nil

	case streamCancelDoneMsg:
		m.streaming = false
		m.streamInterrupted = true
		m.convStreamCh = nil
		if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
			m.transcript[m.agentMsgIndex].interrupted = true
		}
		slog.Info("tui conversation interrupted")
		return m, nil
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
	if stage == "" {
		b.WriteString(mutedStyle.Render("No active etapa for conversation. Start a stage with /hero-start or hero stage start."))
		return b.String()
	}

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
		b.WriteString(mutedStyle.Render("→ Check harness login: cursor agent login"))
	}

	b.WriteByte('\n')
	inputLine := "> " + m.input
	if m.streaming {
		b.WriteString(mutedStyle.Render(inputLine + " (waiting…)"))
	} else {
		b.WriteString(inputLine)
		if m.input == "" {
			b.WriteString(mutedStyle.Render(" type message · enter submit"))
		}
	}
	return b.String()
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
