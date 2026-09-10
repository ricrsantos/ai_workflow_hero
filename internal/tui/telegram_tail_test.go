package tui

import (
	"strings"
	"testing"
)

func TestParseTelegramTail(t *testing.T) {
	cases := []struct {
		text    string
		n       int
		matched bool
		valid   bool
	}{
		{"/tail", 10, true, true},
		{" /TAIL ", 10, true, true},
		{"/tail 5", 5, true, true},
		{"/tail 100", 100, true, true},
		{"/tail 1", 1, true, true},
		{"/tail 0", 0, true, false},
		{"/tail 101", 0, true, false},
		{"/tail -3", 0, true, false},
		{"/tail abc", 0, true, false},
		{"/tail 5 6", 0, true, false},
		{"/tailx", 0, false, false},
		{"/status", 0, false, false},
		{"tail", 0, false, false},
		{"", 0, false, false},
	}
	for _, tc := range cases {
		n, matched, valid := parseTelegramTail(tc.text)
		if matched != tc.matched || valid != tc.valid || (tc.valid && n != tc.n) {
			t.Errorf("parseTelegramTail(%q)=n:%d matched:%v valid:%v want n:%d matched:%v valid:%v",
				tc.text, n, matched, valid, tc.n, tc.matched, tc.valid)
		}
	}
}

func TestTelegramTailTextUsesLastAgentMessage(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleUser, content: "hi"},
		{role: convRoleAgent, content: "first\nsecond\nthird\nfourth"},
		{role: convRoleUser, content: "again"},
		{role: convRoleAgent, content: "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl"},
	}

	if got := m.telegramTailText(10); got != "c\nd\ne\nf\ng\nh\ni\nj\nk\nl" {
		t.Fatalf("tail(10)=%q", got)
	}
	if got := m.telegramTailText(3); got != "j\nk\nl" {
		t.Fatalf("tail(3)=%q", got)
	}
}

func TestTelegramTailTextReturnsWholeWhenShorter(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleAgent, content: "one\ntwo"},
	}
	if got := m.telegramTailText(10); got != "one\ntwo" {
		t.Fatalf("tail(10)=%q", got)
	}
}

func TestTelegramTailTextIgnoresTrailingNewline(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleAgent, content: "one\ntwo\n"},
	}
	if got := m.telegramTailText(1); got != "two" {
		t.Fatalf("tail(1)=%q", got)
	}
}

func TestTelegramTailTextSkipsEmptyAgentMessages(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleAgent, content: ""},
		{role: convRoleAgent, content: "real\nanswer"},
	}
	if got := m.telegramTailText(1); got != "answer" {
		t.Fatalf("tail(1)=%q", got)
	}
}

func TestTelegramTailTextNoAgentResponse(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleUser, content: "hi"},
		{role: convRoleThinking, content: "pondering"},
	}
	if got := m.telegramTailText(10); got != "No agent response available." {
		t.Fatalf("tail(10)=%q", got)
	}
}

func TestTelegramTailInbound(t *testing.T) {
	outbound := []string{}
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}
	m.transcript = []convMessage{
		{role: convRoleUser, content: "hi"},
		{role: convRoleAgent, content: "line 1\nline 2\nline 3"},
	}

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/tail 2", isCommand: true, address: "proj"})
	if cmd != nil {
		_ = cmd()
	}
	if next.streaming {
		t.Fatal("/tail must not start a harness turn")
	}
	if len(outbound) != 1 || outbound[0] != "line 2\nline 3" {
		t.Fatalf("/tail outbound=%v", outbound)
	}

	outbound = nil
	_, cmd = m.handleTelegramInbound(telegramInboundMsg{text: "/tail 999", isCommand: true, address: "proj"})
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "Usage: /tail") {
		t.Fatalf("/tail invalid outbound=%v", outbound)
	}
}
