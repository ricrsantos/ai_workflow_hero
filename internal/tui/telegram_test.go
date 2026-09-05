package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
)

func TestTelegramOriginLabel(t *testing.T) {
	if got, ok := telegramOriginLabel(convMessage{role: convRoleUser, origin: "telegram:ai_workflow_2"}); !ok || got != "← [Telegram · ai_workflow_2]" {
		t.Fatalf("user label=%q ok=%v", got, ok)
	}
	if got, ok := telegramOriginLabel(convMessage{role: convRoleAgent, origin: "telegram:ai_workflow_2"}); !ok || got != "→ [Telegram · ai_workflow_2]" {
		t.Fatalf("agent label=%q ok=%v", got, ok)
	}
	if _, ok := telegramOriginLabel(convMessage{role: convRoleUser}); ok {
		t.Fatal("local user message must not carry a Telegram label")
	}
	if _, ok := telegramOriginLabel(convMessage{role: convRoleAgent, origin: "telegram:"}); !ok {
		t.Fatal("agent with empty address should still be labelled")
	}
}

func TestFormatTelegramEventFiltersToLifecycleOnly(t *testing.T) {
	cases := []struct {
		kind    conversation.EventKind
		nonZero bool
	}{
		{conversation.EventCycleStarted, true},
		{conversation.EventCycleFinished, true},
		{conversation.EventStageStarted, true},
		{conversation.EventStageFinished, true},
		{conversation.EventApprovalRequired, true},
		{conversation.EventError, true},
		{conversation.EventFinalResult, true},
	}
	for _, c := range cases {
		got := formatTelegramEvent(conversation.Event{Kind: c.kind, CycleID: 1, StageName: "qa", Message: "x"})
		if (got != "") != c.nonZero {
			t.Errorf("kind %s: got=%q nonZero=%v", c.kind, got, c.nonZero)
		}
	}
	// Unknown kinds (e.g. stream/tool) produce no outbound text.
	if got := formatTelegramEvent(conversation.Event{Kind: "tool_call", Message: "noise"}); got != "" {
		t.Errorf("tool event must not notify, got %q", got)
	}
}

func TestProjectAbbrevNormalization(t *testing.T) {
	if got := projectAbbrev("/home/u/AI Workflow Hero!"); got != "aiworkflowhero" {
		t.Fatalf("abbrev=%q", got)
	}
	if got := normalizeTelegramAbbrev(""); got != "proj" {
		t.Fatalf("empty abbrev default=%q", got)
	}
	if got := normalizeTelegramAbbrev("My-Proj_2"); got != "my-proj_2" {
		t.Fatalf("abbrev=%q", got)
	}
}

func TestSettingsRows_NotInstalledShowsGuidance(t *testing.T) {
	m := model{}
	rows := m.settingsRows()
	if len(rows) != len(verbosityOptions)+1 {
		t.Fatalf("rows=%d", len(rows))
	}
	last := rows[len(rows)-1]
	if last.kind != rowTelegramNotInstalled {
		t.Fatalf("expected not-installed guidance row, got %d", last.kind)
	}
	if !strings.Contains(last.desc, "hero plugin install telegram") {
		t.Fatalf("guidance missing install command: %q", last.desc)
	}
}

func TestSettingsRows_InstalledShowsControls(t *testing.T) {
	m := model{telegram: &telegramState{installed: true, pluginVersion: "2.9.2", protocolVersion: 1, connected: true, address: "ai_workflow_2"}}
	rows := m.settingsRows()
	kinds := map[settingsRowKind]bool{}
	for _, r := range rows {
		kinds[r.kind] = true
	}
	if !kinds[rowTelegramInfo] || !kinds[rowTelegramDaemon] || !kinds[rowTelegramState] || !kinds[rowTelegramAbbrev] || !kinds[rowTelegramAction] {
		t.Fatalf("missing installed telegram rows: %+v", kinds)
	}
	// Configured state shows no numeric chat id or token anywhere.
	for _, r := range rows {
		for _, secret := range []string{"token", "chat_id"} {
			if strings.Contains(strings.ToLower(r.desc), secret) {
				t.Fatalf("row %q leaks secret word %q", r.desc, secret)
			}
		}
	}
}
