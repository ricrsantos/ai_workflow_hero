package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// telegramTailCommand is the Telegram-only command that returns the tail of the
// most recent agent response in the Chat transcript.
const telegramTailCommand = "/tail"

const (
	tailDefaultLines = 10
	tailMaxLines     = 100
)

// parseTelegramTail parses the Telegram /tail command. It accepts "/tail"
// (default 10 lines) or "/tail n" with n in [1, 100]. matched reports whether
// the text is a /tail command at all; valid reports whether it parsed cleanly.
func parseTelegramTail(text string) (n int, matched, valid bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 || !strings.EqualFold(fields[0], telegramTailCommand) {
		return 0, false, false
	}
	switch len(fields) {
	case 1:
		return tailDefaultLines, true, true
	case 2:
		v, err := strconv.Atoi(fields[1])
		if err != nil || v < 1 || v > tailMaxLines {
			return 0, true, false
		}
		return v, true, true
	default:
		return 0, true, false
	}
}

// handleTelegramTail returns the last n lines of the most recent agent response.
func (m model) handleTelegramTail(n int) (model, tea.Cmd) {
	return m, m.telegramOutboundCmd(m.telegramTailText(n))
}

// telegramTailText extracts the last n lines of the most recent non-empty agent
// message in the transcript. A trailing newline is not counted as an empty
// line; responses shorter than n lines are returned whole.
func (m model) telegramTailText(n int) string {
	var content string
	for i := len(m.transcript) - 1; i >= 0; i-- {
		if m.transcript[i].role == convRoleAgent && strings.TrimSpace(m.transcript[i].content) != "" {
			content = m.transcript[i].content
			break
		}
	}
	if content == "" {
		return "No agent response available."
	}
	lines := strings.Split(content, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
