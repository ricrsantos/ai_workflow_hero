package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type harnessQuestionRequestMsg struct {
	req    harness.QuestionRequest
	respCh chan harness.QuestionResponse
}

func formatHarnessQuestion(req harness.QuestionRequest, index int) string {
	if len(req.Questions) == 0 {
		return "Harness question: type answer in composer, Alt+Enter confirms, Esc rejects."
	}
	if index < 0 || index >= len(req.Questions) {
		index = 0
	}
	q := req.Questions[index]
	var b strings.Builder
	if len(req.Questions) > 1 {
		b.WriteString("Harness question (")
		b.WriteString(strconv.Itoa(index + 1))
		b.WriteString("/")
		b.WriteString(strconv.Itoa(len(req.Questions)))
	} else {
		b.WriteString("Harness question")
	}
	if h := strings.TrimSpace(q.Header); h != "" {
		b.WriteString(": ")
		b.WriteString(h)
	}
	b.WriteByte('\n')
	if text := strings.TrimSpace(q.Question); text != "" && text != strings.TrimSpace(q.Header) {
		b.WriteString(text)
		b.WriteByte('\n')
	}
	for i, opt := range q.Options {
		b.WriteString("  ")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(") ")
		b.WriteString(opt.Label)
		if d := strings.TrimSpace(opt.Description); d != "" && d != opt.Label {
			b.WriteString(" — ")
			b.WriteString(d)
		}
		b.WriteByte('\n')
	}
	if q.Multiple {
		b.WriteString("Multiple: type numbers separated by commas (e.g. 1,3) or text, then Alt+Enter. Esc rejects.")
	} else {
		b.WriteString("Type option number or text, then Alt+Enter. Esc rejects.")
	}
	return strings.TrimSpace(b.String())
}

func parseQuestionAnswer(text string, q harness.QuestionItem) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if q.Multiple {
		var labels []string
		for _, part := range strings.Split(text, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if label := matchQuestionOption(part, q.Options); label != "" {
				labels = append(labels, label)
			} else {
				labels = append(labels, part)
			}
		}
		return labels
	}
	if n, err := strconv.Atoi(text); err == nil && n >= 1 && n <= len(q.Options) {
		return []string{q.Options[n-1].Label}
	}
	if label := matchQuestionOption(text, q.Options); label != "" {
		return []string{label}
	}
	return []string{text}
}

func matchQuestionOption(text string, options []harness.QuestionOption) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if n, err := strconv.Atoi(text); err == nil && n >= 1 && n <= len(options) {
		return options[n-1].Label
	}
	lower := strings.ToLower(text)
	for _, opt := range options {
		if strings.EqualFold(strings.TrimSpace(opt.Label), text) {
			return opt.Label
		}
		if d := strings.TrimSpace(opt.Description); d != "" && strings.EqualFold(d, text) {
			return opt.Label
		}
		if strings.Contains(strings.ToLower(opt.Label), lower) {
			return opt.Label
		}
	}
	return ""
}

func (m model) clearHarnessQuestionState() model {
	m.harnessQuestionPending = false
	m.harnessQuestionMsg = ""
	m.harnessQuestionReq = harness.QuestionRequest{}
	m.harnessQuestionRespCh = nil
	m.harnessQuestionIndex = 0
	m.harnessQuestionAnswers = nil
	return m
}

func (m model) clearHarnessQuestion() model {
	if m.harnessQuestionPending && m.harnessQuestionRespCh != nil {
		m.harnessQuestionRespCh <- harness.QuestionResponse{Rejected: true, Reason: "cancelled"}
	}
	return m.clearHarnessQuestionState()
}

func (m model) finishHarnessQuestionAnswers() model {
	if m.harnessQuestionRespCh != nil {
		m.harnessQuestionRespCh <- harness.QuestionResponse{Answers: m.harnessQuestionAnswers}
	}
	m.chatInputFocused = true
	return m.clearHarnessQuestionState()
}

func (m model) rejectHarnessQuestion() model {
	if m.harnessQuestionRespCh != nil {
		m.harnessQuestionRespCh <- harness.QuestionResponse{Rejected: true, Reason: "rejected"}
	}
	m.insertBeforeAgent(convMessage{role: convRoleWarning, content: "Harness question rejected."})
	m = m.clearChatInput()
	return m.clearHarnessQuestionState()
}

func (m model) submitHarnessQuestionAnswer() (model, tea.Cmd) {
	if !m.harnessQuestionPending || len(m.harnessQuestionReq.Questions) == 0 {
		return m, nil
	}
	text := strings.TrimSpace(m.input)
	if text == "" {
		return m, nil
	}
	idx := m.harnessQuestionIndex
	if idx < 0 || idx >= len(m.harnessQuestionReq.Questions) {
		idx = 0
	}
	answers := parseQuestionAnswer(text, m.harnessQuestionReq.Questions[idx])
	if len(answers) == 0 {
		m = m.setStatusWarning("question", "Invalid answer — try an option number or text.")
		return m, nil
	}
	m.harnessQuestionAnswers = append(m.harnessQuestionAnswers, answers)
	m.insertBeforeAgent(convMessage{role: convRoleWarning, content: "Answer: " + strings.Join(answers, ", ")})
	m = m.clearChatInput()
	m = m.setStatusRunning("question")

	next := idx + 1
	if next < len(m.harnessQuestionReq.Questions) {
		m.harnessQuestionIndex = next
		m.harnessQuestionMsg = formatHarnessQuestion(m.harnessQuestionReq, next)
		m.insertBeforeAgent(convMessage{role: convRoleWarning, content: m.harnessQuestionMsg})
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvBatchMsg(m.convStreamCh)
		}
		return m, nil
	}
	m = m.finishHarnessQuestionAnswers()
	if m.streaming && m.convStreamCh != nil {
		return m, waitConvBatchMsg(m.convStreamCh)
	}
	return m, nil
}

func (m model) handleHarnessQuestionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.harnessQuestionPending {
		return m.handleConversationKey(msg)
	}
	return m.handleHarnessQuestionComposer(msg)
}

func (m model) handleHarnessQuestionComposer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	m.chatInputFocused = true

	switch s {
	case "ctrl+c":
		return m, m.cancelStreamCmd()
	case "alt+q":
		return m.showConfirm(actionQuit, 0, "Harness question pending. Quit? [y/N]")
	case "ctrl+1", "alt+1", "ctrl+2", "alt+2", "ctrl+3", "alt+3",
		"ctrl+4", "alt+4", "ctrl+5", "alt+5", "alt+n":
		return m.handleKey(msg)
	case "alt+r":
		return m.copyChatResponse()
	case "alt+i":
		return m.copyChatInput()
	case "esc":
		m = m.rejectHarnessQuestion()
		if m.streaming && m.convStreamCh != nil {
			return m, waitConvBatchMsg(m.convStreamCh)
		}
		return m, nil
	case "enter":
		return m.insertComposerNewline(), nil
	case "alt+enter":
		return m.submitHarnessQuestionAnswer()
	case "up", "ctrl+p":
		if next, moved := m.moveInputCursorVertical(-1); moved {
			return next, nil
		}
		m = m.scrollTranscript(-1)
		return m, nil
	case "down", "ctrl+n":
		if next, moved := m.moveInputCursorVertical(1); moved {
			return next, nil
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
		m.inputVerticalColumnSet = false
		m = m.ensureInputCaretVisible()
		return m, nil
	case "right":
		if m.inputCursor < runeLen(m.input) {
			m.inputCursor++
		}
		m.inputVerticalColumnSet = false
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
	case "ctrl+u":
		m = m.clearChatInput()
		return m, nil
	case "home":
		m.inputCursor = 0
		m.inputVerticalColumnSet = false
		m = m.ensureInputCaretVisible()
		return m, nil
	case "end":
		m.inputCursor = runeLen(m.input)
		m.inputVerticalColumnSet = false
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
