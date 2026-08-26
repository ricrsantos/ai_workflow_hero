package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func typeChat(t *testing.T, m model, s string) model {
	t.Helper()
	next := m
	for _, r := range s {
		var cmd interface{}
		next, cmd = HandleTestKey(next, string(r))
		_ = cmd
		if CurrentScreen(next) != ScreenConversation {
			t.Fatalf("typing %q left chat, screen=%v after %q", s, CurrentScreen(next), string(r))
		}
	}
	return next
}

func TestChatSlashDoesNotOpenPalette(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next, _ := HandleTestKey(m, "/")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if ConversationInputForTest(next) != "/" {
		t.Fatalf("input=%q want /", ConversationInputForTest(next))
	}
	if !ChatSlashOverlayActiveForTest(next) {
		t.Fatal("expected slash overlay after /")
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "/new-chat") {
		t.Fatalf("overlay missing /new-chat: %q", view)
	}
	if !strings.Contains(view, "/model") {
		t.Fatalf("overlay missing /model: %q", view)
	}
}

func TestSlashOnStatusStillOpensPalette(t *testing.T) {
	m := NewTestModel(nil)
	m = SetScreen(m, ScreenStatus)
	next, _ := HandleTestKey(m, "/")
	if CurrentScreen(next) != ScreenPalette {
		t.Fatalf("expected palette from Status, got %v", CurrentScreen(next))
	}
}

func TestChatSlashOverlayInsertThenSend(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next := typeChat(t, m, "/hero-approve")
	if ConversationInputForTest(next) != "/hero-approve" {
		t.Fatalf("input=%q", ConversationInputForTest(next))
	}
	if !ChatSlashOverlayActiveForTest(next) {
		t.Fatal("overlay should stay open until insert")
	}
	items := FilteredChatSlashForTest(next)
	found := false
	for _, item := range items {
		if item.Label == "/hero-approve" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("filtered items missing /hero-approve: %+v", items)
	}

	next, _ = HandleTestKey(next, "enter")
	if ConversationInputForTest(next) != "/hero-approve" {
		t.Fatalf("after insert input=%q", ConversationInputForTest(next))
	}
	if ChatSlashOverlayActiveForTest(next) {
		t.Fatal("overlay should close after insert")
	}
	if !SlashOverlayDismissedForTest(next) {
		t.Fatal("expected overlay dismissed after insert")
	}
	if CurrentScreen(next) != ScreenConversation {
		t.Fatal("should stay on chat")
	}
}

func TestChatSlashTabInsertsControlNotModeToggle(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next := typeChat(t, m, "/hero-approve")
	if ChatModeForTest(next) != harness.ModeBuild {
		t.Fatalf("mode=%q", ChatModeForTest(next))
	}
	next, _ = HandleTestKey(next, "tab")
	if ChatModeForTest(next) != harness.ModeBuild {
		t.Fatalf("tab on overlay should not toggle mode, got %q", ChatModeForTest(next))
	}
	if ConversationInputForTest(next) != "/hero-approve" {
		t.Fatalf("tab should insert control slash, input=%q", ConversationInputForTest(next))
	}
	if CurrentScreen(next) != ScreenConversation {
		t.Fatal("should stay on chat")
	}
}

func TestChatSlashTabOnGoToDoesNotToggleMode(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next := typeChat(t, m, "/go")
	if ChatModeForTest(next) != harness.ModeBuild {
		t.Fatalf("mode=%q", ChatModeForTest(next))
	}
	next, _ = HandleTestKey(next, "tab")
	if ChatModeForTest(next) != harness.ModeBuild {
		t.Fatalf("tab on overlay should not toggle mode, got %q", ChatModeForTest(next))
	}
	if ConversationInputForTest(next) != "" {
		t.Fatalf("tab on Go to should execute, not insert, input=%q", ConversationInputForTest(next))
	}
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("Go to - Chat should stay on conversation, screen=%v", CurrentScreen(next))
	}
}

func TestChatSlashEnterGoToStatusNavigates(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next, _ := HandleTestKey(m, "/")
	items := FilteredChatSlashForTest(next)
	idx := -1
	for i, item := range items {
		if item.Label == "Go to - Status" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("overlay missing Go to - Status: %+v", items)
	}
	for i := 0; i < idx; i++ {
		var cmd interface{}
		next, cmd = HandleTestKey(next, "down")
		_ = cmd
	}
	next, _ = HandleTestKey(next, "enter")
	if CurrentScreen(next) != ScreenStatus {
		t.Fatalf("screen=%v want status", CurrentScreen(next))
	}
	if ConversationInputForTest(next) != "" {
		t.Fatalf("composer should be empty after Go to, input=%q", ConversationInputForTest(next))
	}
}

func TestChatSlashEnterHeroNewExecutesNotInserts(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next := typeChat(t, m, "/hero-new")
	if !ChatSlashOverlayActiveForTest(next) {
		t.Fatal("overlay should be open for /hero-new")
	}
	next, _ = HandleTestKey(next, "enter")
	if ConversationInputForTest(next) != "" {
		t.Fatalf("composer should clear on execute, not insert /hero-new, input=%q", ConversationInputForTest(next))
	}
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !strings.Contains(StatusTextForTest(next), "/model") {
		t.Fatalf("expected palette-style execute (model required), status=%q", StatusTextForTest(next))
	}
}

func TestChatSlashEscDismissesOverlay(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next, _ := HandleTestKey(m, "/")
	next, _ = HandleTestKey(next, "esc")
	if ChatSlashOverlayActiveForTest(next) {
		t.Fatal("esc should dismiss overlay")
	}
	if ConversationInputForTest(next) != "/" {
		t.Fatalf("esc should keep typed slash, input=%q", ConversationInputForTest(next))
	}
}

func TestChatHeroApproveFollowUpWithoutPendingApproval(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	m = EnterConversationForTest(m)
	m = SetOrchestrationLiveForTest(m, true)
	m = SetHarnessSessionIDForTest(m, "orch-sess")
	m = SetConversationInput(m, "/hero-approve")
	m.slashOverlayDismissed = true

	next, cmd := SubmitConversationForTest(m)
	if StatusKindForTest(next) == "err" && strings.Contains(StatusTextForTest(next), "pending approval") {
		t.Fatalf("live follow-up must not gate on SQLite PendingApproval: %q", StatusTextForTest(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected follow-up streaming")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "hero approve --metrics-json") {
		t.Fatalf("prompt=%q want /hero-approve follow-up (not hero-approve.md)", h.lastPrompt)
	}
	if h.lastSessionID != "orch-sess" {
		t.Fatalf("session=%q want orch-sess", h.lastSessionID)
	}
	if strings.Contains(h.lastPrompt, "hero-approve.md") || strings.Contains(h.lastPrompt, "You are running /hero-approve") {
		t.Fatalf("should not TUI-Execute approve prompt: %q", h.lastPrompt)
	}
}

func TestChatHeroApproveWithoutSessionUsesExecuteGate(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	m := withDefaultChatModel(NewTestModel(svc))
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "/hero-approve")
	m.slashOverlayDismissed = true

	next, cmd := SubmitConversationForTest(m)
	if cmd != nil && IsConversationStreaming(next) {
		t.Fatal("expected no execute when no pending approval and no live session")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s (%q)", StatusKindForTest(next), StatusTextForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "pending approval") {
		t.Fatalf("status=%q", StatusTextForTest(next))
	}
}
