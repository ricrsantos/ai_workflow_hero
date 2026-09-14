package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestConversationEnterInsertsNewlineWithoutSubmit(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hello")
	next, cmd := HandleTestKey(m, "enter")
	if cmd != nil {
		t.Fatal("enter must not submit, got cmd")
	}
	if IsConversationStreaming(next) {
		t.Fatal("enter started a stream")
	}
	if ConversationInputForTest(next) != "hello\n" {
		t.Fatalf("input=%q want hello\\n", ConversationInputForTest(next))
	}
	if InputCursorForTest(next) != runeLen("hello\n") {
		t.Fatalf("cursor=%d", InputCursorForTest(next))
	}
}

func TestConversationAltEnterSubmits(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hello")
	next, cmd := HandleTestKey(m, "alt+enter")
	if !IsConversationStreaming(next) {
		t.Fatal("alt+enter should submit")
	}
	if ConversationInputForTest(next) != "" {
		t.Fatalf("composer should clear after send, input=%q", ConversationInputForTest(next))
	}
	next = drainConversationStream(t, next, cmd)
	if h.lastPrompt != "hello" {
		t.Fatalf("prompt=%q", h.lastPrompt)
	}
}

func TestConversationMultilineSubmitPreservesNewlines(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "one")
	next, cmd := HandleTestKey(m, "enter")
	if cmd != nil {
		t.Fatal("enter must not submit")
	}
	next, _ = HandleTestKeyMsg(next, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("two")})
	next, cmd = HandleTestKey(next, "alt+enter")
	if !IsConversationStreaming(next) {
		t.Fatal("alt+enter should submit multiline")
	}
	next = drainConversationStream(t, next, cmd)
	if h.lastPrompt != "one\ntwo" {
		t.Fatalf("prompt=%q want one\\ntwo", h.lastPrompt)
	}
}

func TestConversationEnterInsertsNewlineForUnknownSlash(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "/not-a-command")
	next, _ := HandleTestKey(m, "enter")
	if ConversationInputForTest(next) != "/not-a-command\n" {
		t.Fatalf("input=%q", ConversationInputForTest(next))
	}
}

func TestConversationNewlineHint(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	if !strings.Contains(view, "enter newline") {
		t.Fatalf("missing newline hint: %q", view)
	}
	if strings.Contains(view, "ctrl+enter") || strings.Contains(view, "ctrl+j") || strings.Contains(view, "shift+enter") {
		t.Fatalf("hint must advertise only enter/alt+enter: %q", view)
	}
	if !strings.Contains(view, "alt+enter send") {
		t.Fatalf("missing send hint: %q", view)
	}
	if !strings.Contains(view, "ctrl+c interrupt") {
		t.Fatalf("missing interrupt hint: %q", view)
	}
	if !strings.Contains(view, "esc navbar") {
		t.Fatalf("missing navbar hint: %q", view)
	}
}

func TestConversationVerticalArrowsMoveComposerCaret(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "first\nx\nthird")

	next, _ := HandleTestKey(m, "up")
	if got, want := InputCursorForTest(next), runeLen("first\nx"); got != want {
		t.Fatalf("first up cursor=%d want %d", got, want)
	}
	next, _ = HandleTestKey(next, "up")
	if got, want := InputCursorForTest(next), runeLen("first"); got != want {
		t.Fatalf("second up cursor=%d want %d", got, want)
	}

	// Keep the original column when a shorter line is crossed, so moving
	// back down returns to the same position on the longer line.
	next, _ = HandleTestKey(next, "down")
	if got, want := InputCursorForTest(next), runeLen("first\nx"); got != want {
		t.Fatalf("first down cursor=%d want %d", got, want)
	}
	next, _ = HandleTestKey(next, "down")
	if got, want := InputCursorForTest(next), runeLen("first\nx\nthird"); got != want {
		t.Fatalf("second down cursor=%d want %d", got, want)
	}
}

func TestConversationVerticalArrowsFollowSoftWraps(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetWidth(m, 30)
	width := m.chatContentWidth()
	m = SetConversationInput(m, strings.Repeat("a", width+4))

	next, _ := HandleTestKey(m, "up")
	if got, want := InputCursorForTest(next), 4; got != want {
		t.Fatalf("up across soft wrap cursor=%d want %d", got, want)
	}
	next, _ = HandleTestKey(next, "down")
	if got, want := InputCursorForTest(next), width+4; got != want {
		t.Fatalf("down across soft wrap cursor=%d want %d", got, want)
	}
}

func TestInputVisualLinesWrapWholeWordsAndHardWrapLongWords(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  []string
	}{
		{name: "whole words", input: "one two three", width: 9, want: []string{"one two ", "three"}},
		{name: "long word", input: "supercalifragilistic", width: 5, want: []string{"super", "calif", "ragil", "istic"}},
		{name: "explicit newline", input: "one two\nthree four", width: 20, want: []string{"one two", "three four"}},
		{name: "wide runes", input: "ab界 cd", width: 4, want: []string{"ab界", " cd"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runes := []rune(tt.input)
			lines := inputVisualLines(tt.input, tt.width)
			got := make([]string, 0, len(lines))
			for _, line := range lines {
				got = append(got, string(runes[line.start:line.end]))
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Fatalf("lines=%q want %q", got, tt.want)
			}
		})
	}
}

func TestConversationHomeEndUseCurrentVisualLine(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetWidth(m, 30)
	width := m.chatContentWidth()
	input := strings.Repeat("a", width) + " second"
	m = SetConversationInput(m, input)

	m, _ = HandleTestKey(m, "ctrl+home")
	m, _ = HandleTestKey(m, "right")
	m, _ = HandleTestKey(m, "end")
	if got, want := InputCursorForTest(m), width; got != want {
		t.Fatalf("end cursor=%d want first visual-line end %d", got, want)
	}
	lines := inputVisualLines(input, width)
	line, _ := inputCursorVisualPositionWithAffinity(lines, m.inputCursor, m.inputCursorPreviousLine)
	if line != 0 {
		t.Fatalf("end rendered on visual line %d want 0", line)
	}

	m, _ = HandleTestKey(m, "ctrl+end")
	m, _ = HandleTestKey(m, "home")
	if got, want := InputCursorForTest(m), width; got != want {
		t.Fatalf("home cursor=%d want visual-line start %d", got, want)
	}
	m, _ = HandleTestKey(m, "end")
	if got, want := InputCursorForTest(m), runeLen(input); got != want {
		t.Fatalf("end cursor=%d want visual-line end %d", got, want)
	}
	m, _ = HandleTestKey(m, "ctrl+home")
	if got := InputCursorForTest(m); got != 0 {
		t.Fatalf("ctrl+home cursor=%d want input start", got)
	}
	m, _ = HandleTestKey(m, "ctrl+end")
	if got, want := InputCursorForTest(m), runeLen(input); got != want {
		t.Fatalf("ctrl+end cursor=%d want input end %d", got, want)
	}
}

func TestConversationCaretOverlaysCurrentCharacter(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "abcd")
	m.inputCursor = 1

	line := m.inputLinesWithCaret(20)[0]
	if got := lipgloss.Width(line); got != len("abcd") {
		t.Fatalf("rendered width=%d want %d; caret must not add a cell", got, len("abcd"))
	}
	if plain := stripANSI(line); plain != "abcd" {
		t.Fatalf("rendered text=%q want original text", plain)
	}
}
