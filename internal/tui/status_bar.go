package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type statusKind int

const (
	statusIdle statusKind = iota
	statusRunning
	statusOK
	statusErr
)

// Fixed chrome for the footer status area.
const statusBarMaxLines = 2

type statusTickMsg struct{}

func (m model) closePalette() model {
	wasPicking := m.pickingModel
	m.screen = m.prevScreen
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	if wasPicking {
		m.pickingModel = false
		m = m.reloadPaletteItems()
	}
	if m.screen == screenConversation {
		m.chatInputFocused = true
	}
	return m
}

func (m model) setStatusRunning(label string) model {
	m.statusKind = statusRunning
	m.statusLabel = label
	m.statusText = "running…"
	m.statusStarted = time.Now()
	m.actionBusy = true
	return m
}

func (m model) setStatusResult(ok bool, label, text string) model {
	m.actionBusy = false
	if label != "" {
		m.statusLabel = label
	}
	m.statusText = strings.TrimSpace(text)
	if ok {
		m.statusKind = statusOK
	} else {
		m.statusKind = statusErr
	}
	return m
}

func (m model) setStatusBusyBlocked() model {
	busy := m.statusLabel
	if busy == "" {
		busy = "previous action"
	}
	m.statusKind = statusErr
	m.statusText = fmt.Sprintf("busy — wait for %s to finish", busy)
	return m
}

func statusTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return statusTickMsg{}
	})
}

func (m model) statusBarLineCount() int {
	return statusBarMaxLines
}

func (m model) renderStatusBar() string {
	width := m.width
	if width < 20 {
		width = 20
	}
	lines := m.statusBarDisplayLines(width)
	for len(lines) < statusBarMaxLines {
		lines = append(lines, "")
	}
	if len(lines) > statusBarMaxLines {
		lines = lines[:statusBarMaxLines]
	}

	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}

func (m model) statusBarDisplayLines(width int) []string {
	switch m.statusKind {
	case statusRunning:
		elapsed := time.Since(m.statusStarted).Truncate(time.Second)
		label := m.statusLabel
		if label == "" {
			label = "action"
		}
		head := fmt.Sprintf("● %s  running  %s", label, formatElapsed(elapsed))
		return []string{infoStyle.Render(head)}
	case statusOK:
		return wrapStatusMessage("✓", m.statusLabel, m.statusText, successStyle, width)
	case statusErr:
		return wrapStatusMessage("✗", m.statusLabel, m.statusText, errorStyle, width)
	default:
		lines := []string{mutedStyle.Render("ready")}
		if hint := m.conversationStatusHint(); hint != "" {
			lines = append(lines, mutedStyle.Render(hint))
		}
		return lines
	}
}

// conversationStatusHint is the second ready-line on the Chat screen (frees header space).
func (m model) conversationStatusHint() string {
	if m.screen != screenConversation {
		return ""
	}
	if m.conversationStage == "" {
		return "No active etapa — chatting with harness defaults. /hero-new then /hero-start for a cycle."
	}
	return fmt.Sprintf("Etapa: %s", m.conversationStage)
}

func wrapStatusMessage(icon, label, text string, style lipgloss.Style, width int) []string {
	prefix := icon + " "
	if label != "" {
		prefix += label + " — "
	}
	body := text
	if body == "" {
		body = "(no message)"
	}
	raw := prefix + body
	wrapped := splitOutputLines(raw, width)
	if len(wrapped) > statusBarMaxLines {
		wrapped = wrapped[:statusBarMaxLines]
		last := wrapped[statusBarMaxLines-1]
		runes := []rune(last)
		if len(runes) > 3 {
			wrapped[statusBarMaxLines-1] = string(runes[:len(runes)-3]) + "..."
		}
	}
	out := make([]string, len(wrapped))
	for i, line := range wrapped {
		out[i] = style.Render(line)
	}
	return out
}

func formatElapsed(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	mins := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", mins, s)
}

func statusResultOpensPanel(text string, width int) bool {
	return shouldOpenOutputPanel(text, width)
}

func firstStatusLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return strings.TrimSpace(text[:i])
	}
	return text
}
