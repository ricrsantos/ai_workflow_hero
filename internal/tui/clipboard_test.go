package tui

import (
	"strings"
	"testing"
)

func TestCopyChatResponseReturnsCmd(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m.transcript = append(m.transcript,
		convMessage{role: convRoleUser, content: "q"},
		convMessage{role: convRoleAgent, content: "answer text", agentName: "orchestration_agent"},
	)

	next, cmd := HandleTestKey(m, "alt+r")
	if cmd == nil {
		t.Fatal("alt+r must return clipboard cmd")
	}
	if next.statusText != "response copied" {
		t.Fatalf("status = %q", next.statusText)
	}
}

func TestCopyChatInputReturnsCmd(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "composer text")

	next, cmd := HandleTestKey(m, "alt+i")
	if cmd == nil {
		t.Fatal("alt+i must return clipboard cmd")
	}
	if next.statusText != "input copied" {
		t.Fatalf("status = %q", next.statusText)
	}
}

func TestCopyShortcutsDoNotSubmit(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "typed")

	next, cmd := HandleTestKey(m, "alt+i")
	if IsConversationStreaming(next) {
		t.Fatal("copy shortcut started streaming")
	}
	if cmd == nil {
		t.Fatal("expected clipboard cmd")
	}
	if ConversationInputForTest(next) != "typed" {
		t.Fatalf("input cleared after copy: %q", ConversationInputForTest(next))
	}
}

func TestCopyEmptyShowsError(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)

	next, cmd := HandleTestKey(m, "alt+r")
	if cmd != nil {
		t.Fatal("empty response copy must not run clipboard cmd")
	}
	if next.statusKind != statusErr || next.statusText != "nothing to copy" {
		t.Fatalf("status = %q kind=%v", next.statusText, next.statusKind)
	}
}

func TestTranscriptPlainTextIncludesThinking(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleUser, content: "q"},
		{role: convRoleThinking, content: "ponder", agentName: "orchestration_agent"},
		{role: convRoleAgent, content: "done", agentName: "orchestration_agent"},
	}
	got := m.transcriptPlainText()
	if !strings.Contains(got, "You") || !strings.Contains(got, "q") {
		t.Fatalf("missing user message: %q", got)
	}
	if !strings.Contains(got, "Thinking: ponder") {
		t.Fatalf("missing thinking: %q", got)
	}
	if !strings.Contains(got, "done") {
		t.Fatalf("missing agent text: %q", got)
	}
}

func TestTranscriptPlainTextCopiesWholeConversation(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleUser, content: "first"},
		{role: convRoleAgent, content: "reply one", agentName: "orchestration_agent"},
		{role: convRoleUser, content: "second"},
		{role: convRoleAgent, content: "reply two", agentName: "orchestration_agent"},
	}
	got := m.transcriptPlainText()
	for _, want := range []string{"You", "first", "reply one", "second", "reply two"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "│") {
		t.Fatalf("plain text must not contain accent bars: %q", got)
	}
}

func TestTranscriptPlainTextOmitsDecoration(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleAgent, content: "line one\nline two", agentName: "orchestration_agent"},
	}
	got := m.transcriptPlainText()
	if strings.Contains(got, "│") {
		t.Fatalf("plain text must not contain accent bars: %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Fatalf("plain text must not contain padding spaces: %q", got)
	}
}
