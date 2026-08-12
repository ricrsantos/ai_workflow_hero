package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const outputFlashLineLimit = 3

// openOutput switches to the scrollable output panel for a command result.
func (m model) openOutput(title, text string, isErr bool) model {
	if title == "" {
		title = "Output"
	}
	m.screen = screenOutput
	m.outputTitle = title
	m.outputErr = isErr
	m.outputOffset = 0
	m.outputRaw = text
	m.paletteFilter = ""
	m.paletteIndex = 0
	m.paletteOffset = 0
	m = m.rebuildOutputLines()
	return m
}

func (m model) rebuildOutputLines() model {
	width := m.width - 6
	if width < 20 {
		width = 20
	}
	m.outputLines = splitOutputLines(m.outputRaw, width)
	return m.ensureOutputVisible()
}

func splitOutputLines(text string, width int) []string {
	if width < 8 {
		width = 8
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return []string{""}
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		if para == "" {
			out = append(out, "")
			continue
		}
		out = append(out, wrapOutputLine(para, width)...)
	}
	return out
}

func wrapOutputLine(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	runes := []rune(strings.TrimRight(s, "\r"))
	if len(runes) == 0 {
		return []string{""}
	}
	var lines []string
	for len(runes) > 0 {
		if len(runes) <= width {
			lines = append(lines, string(runes))
			break
		}
		cut := width
		chunk := string(runes[:cut])
		if sp := strings.LastIndex(chunk, " "); sp > width/4 {
			lines = append(lines, strings.TrimSpace(chunk[:sp]))
			runes = []rune(strings.TrimLeft(string(runes[sp:]), " "))
			continue
		}
		lines = append(lines, chunk)
		runes = runes[cut:]
	}
	return lines
}

func shouldOpenOutputPanel(text string, width int) bool {
	if strings.Contains(text, "\n") {
		return true
	}
	lines := splitOutputLines(text, max(20, width-6))
	return len(lines) > outputFlashLineLimit
}

func (m model) outputListHeight() int {
	chrome := 11 + m.statusBarLineCount() + 2
	h := m.height - chrome
	if h < 4 {
		h = 4
	}
	return h
}

func (m model) ensureOutputVisible() model {
	n := len(m.outputLines)
	if n == 0 {
		m.outputOffset = 0
		return m
	}
	vh := m.outputListHeight()
	maxOff := n - vh
	if maxOff < 0 {
		maxOff = 0
	}
	if m.outputOffset < 0 {
		m.outputOffset = 0
	}
	if m.outputOffset > maxOff {
		m.outputOffset = maxOff
	}
	return m
}

func (m model) handleOutputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = m.prevScreen
		m.outputLines = nil
		m.outputRaw = ""
		m.outputOffset = 0
		m.outputTitle = ""
		m.outputErr = false
		return m, nil
	case "ctrl+c", "ctrl+q":
		return m, tea.Quit
	case "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5", "ctrl+6",
		"alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6",
		"ctrl+r", "f5":
		m.outputLines = nil
		m.outputRaw = ""
		m.outputOffset = 0
		m.outputTitle = ""
		m.outputErr = false
		return m.handleKey(msg)
	case "up", "ctrl+p":
		if m.outputOffset > 0 {
			m.outputOffset--
		}
		return m, nil
	case "down", "ctrl+n":
		m.outputOffset++
		m = m.ensureOutputVisible()
		return m, nil
	case "pgup":
		m.outputOffset -= m.outputListHeight()
		m = m.ensureOutputVisible()
		return m, nil
	case "pgdown":
		m.outputOffset += m.outputListHeight()
		m = m.ensureOutputVisible()
		return m, nil
	case "home":
		m.outputOffset = 0
		return m, nil
	case "end":
		m.outputOffset = len(m.outputLines)
		m = m.ensureOutputVisible()
		return m, nil
	case "/":
		// Keep prevScreen as the screen under this overlay (not Output itself).
		under := m.prevScreen
		m.screen = screenPalette
		m.prevScreen = under
		m.paletteFilter = ""
		m.paletteIndex = 0
		m.paletteOffset = 0
		m = m.reloadPaletteItems()
		return m, nil
	}
	return m, nil
}

func (m model) renderOutput() string {
	var b strings.Builder
	title := m.outputTitle
	if title == "" {
		title = "Output"
	}
	if m.outputErr {
		b.WriteString(errorStyle.Render(title))
	} else {
		b.WriteString(titleStyle.Render(title))
	}
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("↑↓ scroll · PgUp/PgDn · esc close"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	m = m.ensureOutputVisible()
	lines := m.outputLines
	if len(lines) == 0 {
		b.WriteString(mutedStyle.Render("(empty)"))
		return b.String()
	}

	listH := m.outputListHeight()
	start := m.outputOffset
	end := start + listH
	if end > len(lines) {
		end = len(lines)
	}

	var list strings.Builder
	if start > 0 {
		list.WriteString(mutedStyle.Render(fmt.Sprintf("  ▲ %d more above", start)))
		list.WriteByte('\n')
	}
	for i := start; i < end; i++ {
		list.WriteString(styleOutputLine(lines[i], m.outputErr))
		list.WriteByte('\n')
	}
	below := len(lines) - end
	if below > 0 {
		list.WriteString(mutedStyle.Render(fmt.Sprintf("  ▼ %d more below", below)))
		list.WriteByte('\n')
	}

	rangeLabel := fmt.Sprintf(" %d–%d of %d ", start+1, end, len(lines))
	panelWidth := m.width - 2
	if panelWidth < 24 {
		panelWidth = 24
	}
	border := colorBlue
	if m.outputErr {
		border = colorRed
	}
	panel := outputPanelStyle.
		BorderForeground(border).
		Width(panelWidth).
		Render(strings.TrimRight(list.String(), "\n"))
	b.WriteString(mutedStyle.Render(rangeLabel))
	b.WriteByte('\n')
	b.WriteString(panel)
	return b.String()
}

// styleOutputLine applies Hero semantic colors to a single result line.
func styleOutputLine(line string, isErr bool) string {
	trim := strings.TrimSpace(line)
	if trim == "" {
		return line
	}
	if isErr {
		return errorStyle.Render(line)
	}
	switch {
	case strings.HasPrefix(trim, "→"):
		return infoStyle.Render(line)
	case strings.HasPrefix(trim, "✓"):
		return successStyle.Render(line)
	case strings.HasPrefix(trim, "⚠"):
		return warnStyle.Render(line)
	case strings.HasPrefix(trim, "✗"):
		return errorStyle.Render(line)
	case strings.HasPrefix(trim, "•"):
		return infoStyle.Render(line)
	case strings.HasPrefix(trim, "Total:"):
		return outputTotalStyle.Render(line)
	case strings.HasPrefix(trim, "C") && strings.Contains(trim, "—"):
		return outputCycleStyle.Render(line)
	case strings.HasPrefix(line, "  ") && !strings.HasPrefix(trim, "Total:"):
		return mutedStyle.Render(line)
	default:
		return outputBodyStyle.Render(line)
	}
}
