package tui

// isComposerNewlineKey is true for the ordinary Chat composer newline binding.
// Recognized slash commands are handled before this check so Enter executes
// them; Alt+Enter submits prompts and does not conflict with numbered screen
// jumps. Do not enable the Kitty keyboard protocol on Bubble Tea v1 (ordinary
// keys become unknown CSI and the composer freezes).
func isComposerNewlineKey(s string) bool {
	return s == "enter"
}

func (m model) insertComposerNewline() model {
	prev := chatSlashToken(m.input)
	m = m.insertRunesAtCursor([]rune{'\n'})
	m = m.afterChatInputEdit(prev)
	return m.ensureInputCaretVisible()
}

type inputVisualLine struct {
	start int
	end   int
}

// inputVisualLines mirrors the composer wrapping rules. Offsets are rune
// offsets, matching model.inputCursor and the fixed-width renderer.
func inputVisualLines(input string, width int) []inputVisualLine {
	if width < 1 {
		width = 1
	}
	runes := []rune(input)
	lines := make([]inputVisualLine, 0, len(runes)/width+1)
	start, column := 0, 0
	for i, r := range runes {
		if r == '\n' {
			lines = append(lines, inputVisualLine{start: start, end: i})
			start = i + 1
			column = 0
			continue
		}
		if column >= width {
			lines = append(lines, inputVisualLine{start: start, end: i})
			start = i
			column = 0
		}
		column++
	}
	lines = append(lines, inputVisualLine{start: start, end: len(runes)})
	return lines
}

func inputCursorVisualPosition(lines []inputVisualLine, cursor int) (int, int) {
	if len(lines) == 0 {
		return 0, 0
	}
	if cursor < lines[0].start {
		return 0, 0
	}
	for i, line := range lines {
		if cursor < line.start {
			continue
		}
		if cursor < line.end {
			return i, cursor - line.start
		}
		if cursor == line.end {
			// At a soft-wrap boundary the caret belongs to the next visual
			// row, so a subsequent character is inserted on that row.
			if i+1 < len(lines) && lines[i+1].start == cursor {
				return i + 1, 0
			}
			return i, cursor - line.start
		}
	}
	last := lines[len(lines)-1]
	return len(lines) - 1, last.end - last.start
}

func (m model) moveInputCursorVertical(direction int) (model, bool) {
	if direction != -1 && direction != 1 {
		return m, false
	}
	m = m.clampInputCursor()
	lines := inputVisualLines(m.input, m.chatContentWidth())
	lineIndex, column := inputCursorVisualPosition(lines, m.inputCursor)
	targetIndex := lineIndex + direction
	if targetIndex < 0 || targetIndex >= len(lines) {
		return m, false
	}
	if !m.inputVerticalColumnSet {
		m.inputVerticalColumn = column
		m.inputVerticalColumnSet = true
	}
	target := lines[targetIndex]
	targetColumn := m.inputVerticalColumn
	maxColumn := target.end - target.start
	if targetColumn > maxColumn {
		targetColumn = maxColumn
	}
	m.inputCursor = target.start + targetColumn
	return m.ensureInputCaretVisible(), true
}
