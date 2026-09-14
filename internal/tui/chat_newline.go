package tui

import (
	"unicode"

	"github.com/rivo/uniseg"
)

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
	start        int
	end          int
	displayWidth int
}

// inputVisualLines is the source of truth for composer rendering and cursor
// navigation. Offsets remain rune offsets, while wrapping uses terminal cells.
func inputVisualLines(input string, width int) []inputVisualLine {
	if width < 1 {
		width = 1
	}
	runes := []rune(input)
	lines := make([]inputVisualLine, 0, len(runes)/width+1)
	logicalStart := 0
	for i, r := range runes {
		if r != '\n' {
			continue
		}
		lines = append(lines, wrapInputLogicalLine(runes, logicalStart, i, width)...)
		logicalStart = i + 1
	}
	lines = append(lines, wrapInputLogicalLine(runes, logicalStart, len(runes), width)...)
	return lines
}

func wrapInputLogicalLine(runes []rune, start, end, width int) []inputVisualLine {
	if start == end {
		return []inputVisualLine{{start: start, end: end}}
	}

	lines := make([]inputVisualLine, 0, (end-start)/width+1)
	for start < end {
		cellWidth := 0
		lastWordStart := -1
		i := start
		for i < end {
			runeWidth := uniseg.StringWidth(string(runes[i]))
			if cellWidth+runeWidth > width {
				break
			}
			cellWidth += runeWidth
			i++
			if unicode.IsSpace(runes[i-1]) {
				lastWordStart = i
			}
		}

		if i == end {
			lines = append(lines, inputVisualLine{start: start, end: end, displayWidth: cellWidth})
			break
		}

		cut := i
		if lastWordStart > start {
			cut = lastWordStart
		}
		if cut == start {
			// A zero-width cluster or a rune wider than the available region
			// must still make progress.
			cut++
		}
		lines = append(lines, inputVisualLine{
			start:        start,
			end:          cut,
			displayWidth: inputDisplayWidth(runes[start:cut]),
		})
		start = cut
	}
	return lines
}

func inputDisplayWidth(runes []rune) int {
	return uniseg.StringWidth(string(runes))
}

func inputCursorVisualPosition(lines []inputVisualLine, cursor int) (int, int) {
	return inputCursorVisualPositionWithAffinity(lines, cursor, false)
}

func inputCursorVisualPositionWithAffinity(lines []inputVisualLine, cursor int, previousLine bool) (int, int) {
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
			if previousLine {
				return i, cursor - line.start
			}
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

func inputCursorDisplayColumn(runes []rune, line inputVisualLine, cursor int) int {
	if cursor <= line.start {
		return 0
	}
	if cursor > line.end {
		cursor = line.end
	}
	return inputDisplayWidth(runes[line.start:cursor])
}

func inputCursorAtDisplayColumn(runes []rune, line inputVisualLine, column int) int {
	if column <= 0 {
		return line.start
	}
	currentWidth := 0
	for i := line.start; i < line.end; i++ {
		nextWidth := currentWidth + uniseg.StringWidth(string(runes[i]))
		if nextWidth > column {
			return i
		}
		currentWidth = nextWidth
	}
	return line.end
}

func (m model) moveInputCursorVertical(direction int) (model, bool) {
	if direction != -1 && direction != 1 {
		return m, false
	}
	m = m.clampInputCursor()
	runes := []rune(m.input)
	lines := inputVisualLines(m.input, m.chatContentWidth())
	lineIndex, _ := inputCursorVisualPositionWithAffinity(lines, m.inputCursor, m.inputCursorPreviousLine)
	column := inputCursorDisplayColumn(runes, lines[lineIndex], m.inputCursor)
	targetIndex := lineIndex + direction
	if targetIndex < 0 || targetIndex >= len(lines) {
		return m, false
	}
	if !m.inputVerticalColumnSet {
		m.inputVerticalColumn = column
		m.inputVerticalColumnSet = true
	}
	target := lines[targetIndex]
	targetColumn := minInt(m.inputVerticalColumn, target.displayWidth)
	m.inputCursor = inputCursorAtDisplayColumn(runes, target, targetColumn)
	m.inputCursorPreviousLine = m.inputCursor == target.end && targetIndex+1 < len(lines) && lines[targetIndex+1].start == target.end
	return m.ensureInputCaretVisible(), true
}

func (m model) moveInputCursorToVisualLineBoundary(end bool) model {
	m = m.clampInputCursor()
	lines := inputVisualLines(m.input, m.chatContentWidth())
	lineIndex, _ := inputCursorVisualPositionWithAffinity(lines, m.inputCursor, m.inputCursorPreviousLine)
	if end {
		m.inputCursor = lines[lineIndex].end
		m.inputCursorPreviousLine = lineIndex+1 < len(lines) && lines[lineIndex+1].start == m.inputCursor
	} else {
		m.inputCursor = lines[lineIndex].start
		m.inputCursorPreviousLine = false
	}
	m.inputVerticalColumnSet = false
	return m.ensureInputCaretVisible()
}
