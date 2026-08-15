package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConversationAltEnterInsertsNewlineWithoutSubmit(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hello")
	next, cmd := HandleTestKey(m, "alt+enter")
	if cmd != nil {
		t.Fatal("alt+enter must not submit, got cmd")
	}
	if IsConversationStreaming(next) {
		t.Fatal("alt+enter started a stream")
	}
	if ConversationInputForTest(next) != "hello\n" {
		t.Fatalf("input=%q want hello\\n", ConversationInputForTest(next))
	}
	if InputCursorForTest(next) != runeLen("hello\n") {
		t.Fatalf("cursor=%d", InputCursorForTest(next))
	}
}

func TestConversationEnterStillSubmits(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hello")
	next, cmd := HandleTestKey(m, "enter")
	if !IsConversationStreaming(next) {
		t.Fatal("enter should submit")
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
	next, cmd := HandleTestKey(m, "alt+enter")
	if cmd != nil {
		t.Fatal("alt+enter must not submit")
	}
	next, _ = HandleTestKeyMsg(next, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("two")})
	next, cmd = HandleTestKey(next, "enter")
	if !IsConversationStreaming(next) {
		t.Fatal("enter should submit multiline")
	}
	next = drainConversationStream(t, next, cmd)
	if h.lastPrompt != "one\ntwo" {
		t.Fatalf("prompt=%q want one\\ntwo", h.lastPrompt)
	}
}

func TestConversationNewlineHidesSlashOverlay(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next, _ := HandleTestKey(m, "/")
	if !ChatSlashOverlayActiveForTest(next) {
		t.Fatal("expected overlay after /")
	}
	next, _ = HandleTestKey(next, "alt+enter")
	if ChatSlashOverlayActiveForTest(next) {
		t.Fatal("newline should hide slash overlay")
	}
	if ConversationInputForTest(next) != "/\n" {
		t.Fatalf("input=%q", ConversationInputForTest(next))
	}
}

func TestConversationNewlineHint(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	if !strings.Contains(view, "alt+enter newline") {
		t.Fatalf("missing newline hint: %q", view)
	}
	if strings.Contains(view, "ctrl+enter") || strings.Contains(view, "ctrl+j") || strings.Contains(view, "shift+enter") {
		t.Fatalf("hint must advertise only alt+enter: %q", view)
	}
	if !strings.Contains(view, "enter send") {
		t.Fatalf("missing send hint: %q", view)
	}
}
