package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
