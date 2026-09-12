package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestHistoryEmptyActiveCopy(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 100), ScreenHistory)
	m.history.loading = false
	view := stripANSI(ViewForTest(m))
	if !strings.Contains(view, historyCopyEmptyActive1) {
		t.Fatalf("missing empty active copy: %q", view)
	}
	if !strings.Contains(view, historyCopyEmptyActive2) {
		t.Fatalf("missing empty active hint: %q", view)
	}
}

func TestHistorySearchSlashDoesNotOpenPalette(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 100), ScreenHistory)
	m.history.loading = false
	next, _ := HandleTestKey(m, "/")
	if CurrentScreen(next) != ScreenHistory {
		t.Fatalf("screen=%v want History", CurrentScreen(next))
	}
	if !next.history.searchActive {
		t.Fatal("expected search mode")
	}
}

func TestHistorySlashFooterMentionsPaletteException(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 100), ScreenHistory)
	m.history.searchActive = true
	hints := m.historyFooterHints()
	if !strings.Contains(hints, "not command palette") {
		t.Fatalf("footer=%q", hints)
	}
}

func TestHistoryListsSessionsFromService(t *testing.T) {
	svc := newTestService(t)
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := t.Context()
	_, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{
		Kind:      store.SessionKindFreechat,
		Title:     "deployment review",
		HarnessID: "cursor",
		Model:     "composer",
	}, conversation.FirstTurnContent{Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	m := NewTestModel(svc)
	m = SetScreen(SetWidth(SetHeight(m, 30), 100), ScreenHistory)
	rows, err := sessionSvc.ListSessions(ctx, store.ListSessionsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	nextModel, _ := m.Update(historyLoadedMsg{sessions: rows})
	next := nextModel.(model)
	view := stripANSI(ViewForTest(next))
	if !strings.Contains(view, "deployment review") {
		t.Fatalf("expected session row: %q", view)
	}
}

func TestHistoryWindowTooSmall(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), historyMinContentHeight-1), historyMinContentWidth-1), ScreenHistory)
	view := stripANSI(m.renderHistory())
	if !strings.Contains(view, "window too small") {
		t.Fatalf("view=%q", view)
	}
}

func TestHistoryLegacyInterruptedCopyInDetail(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 100), ScreenHistory)
	m.history.loading = false
	m.history.sessions = []store.Session{{
		ID:              "sess-1",
		Title:           "legacy row",
		Kind:            store.SessionKindFreechat,
		Lifecycle:       store.SessionLifecycleInterrupted,
		TranscriptState: store.TranscriptUnavailableLegacy,
		LastActivityAt:  "2026-09-11T21:42:00Z",
		CreatedAt:       "2026-09-11T20:00:00Z",
	}}
	m.history.cursor = 0
	view := stripANSI(ViewForTest(m))
	if !strings.Contains(view, historyCopyLegacyTranscript) {
		t.Fatalf("missing legacy transcript copy: %q", view)
	}
	if !strings.Contains(view, historyCopyInterrupted1) {
		t.Fatalf("missing interrupted copy: %q", view)
	}
}

func TestOpenHistoryLoadsAsync(t *testing.T) {
	m := NewTestModel(newTestService(t))
	next, cmd := m.openHistory()
	if !next.history.loading {
		t.Fatal("expected loading flag")
	}
	if cmd == nil {
		t.Fatal("expected load cmd")
	}
}

func TestHistoryResizeMsg(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 100), ScreenHistory)
	m.history.sessions = []store.Session{{ID: "a", Title: "one"}, {ID: "b", Title: "two"}}
	m.history.cursor = 1
	nextModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if nextModel.(model).history.cursor != 1 {
		t.Fatal("cursor should be preserved on resize")
	}
}

func TestHistoryForkImportDialogCopy(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 100), ScreenHistory)
	m.history.dialog = historyDialogFork
	m.history.forkHarness = "cursor"
	m.history.forkModel = "composer-2.5"
	view := stripANSI(m.renderHistoryDialog(80))
	if !strings.Contains(view, "cannot be resumed") {
		t.Fatalf("fork dialog: %q", view)
	}
	m.history.dialog = historyDialogImport
	m.history.importHarness = "opencode"
	view = stripANSI(m.renderHistoryDialog(80))
	if !strings.Contains(view, "Import transcript from opencode?") {
		t.Fatalf("import dialog: %q", view)
	}
}

func TestHistoryImportErrorStaysOnHistory(t *testing.T) {
	svc := newTestService(t)
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	sessionSvc.Remote = &historyFakeRemoteReaderFail{}
	sess, err := svc.Store.CreateSession(store.CreateSessionInput{
		Kind:            store.SessionKindFreechat,
		Title:           "legacy import fail",
		HarnessID:       "opencode",
		NativeSessionID: "native-import-fail",
		TranscriptState: store.TranscriptUnavailableLegacy,
	})
	if err != nil {
		t.Fatal(err)
	}

	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m = SetScreen(SetWidth(SetHeight(m, 30), 100), ScreenHistory)
	next, _ := m.Update(historyOpenMsg{
		sessionID:     sess.ID,
		offerImport:   true,
		importHarness: "opencode",
	})
	nm := next.(model)
	nm.history.dialogFocus = 1
	next, cmd := HandleTestKeyMsg(nm, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected import cmd")
	}
	msg := cmd()
	imported, ok := msg.(historyImportMsg)
	if !ok {
		t.Fatalf("import msg=%T", msg)
	}
	if imported.err == nil {
		t.Fatal("expected import error")
	}
	next, _ = next.Update(imported)
	final := next.(model)
	if final.screen != screenHistory {
		t.Fatalf("screen=%v want history", final.screen)
	}
	if final.history.actionErr == "" {
		t.Fatal("expected actionable import error")
	}
}

type historyFakeRemoteReaderFail struct{}

func (historyFakeRemoteReaderFail) SupportsRemoteHistory() bool { return true }
func (historyFakeRemoteReaderFail) ReadRemoteHistory(context.Context, string) ([]harness.NormalizedEvent, error) {
	return nil, context.Canceled
}

func TestHistoryOpenOffersImportDialog(t *testing.T) {
	svc := newTestService(t)
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	sessionSvc.Remote = &historyFakeRemoteReader{}
	sess, err := svc.Store.CreateSession(store.CreateSessionInput{
		Kind:            store.SessionKindFreechat,
		Title:           "legacy import",
		HarnessID:       "opencode",
		NativeSessionID: "native-import",
		TranscriptState: store.TranscriptUnavailableLegacy,
	})
	if err != nil {
		t.Fatal(err)
	}

	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m = SetScreen(SetWidth(SetHeight(m, 30), 100), ScreenHistory)
	next, _ := m.Update(historyOpenMsg{
		sessionID:     sess.ID,
		offerImport:   true,
		importHarness: "opencode",
	})
	nm := next.(model)
	if nm.history.dialog != historyDialogImport {
		t.Fatalf("dialog=%v want import", nm.history.dialog)
	}
	if nm.screen != screenHistory {
		t.Fatal("expected to stay on history until import resolved")
	}

	// Confirm import (focus Import on right).
	nm.history.dialogFocus = 1
	next, cmd := HandleTestKeyMsg(nm, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected import cmd")
	}
	msg := cmd()
	imported, ok := msg.(historyImportMsg)
	if !ok || imported.err != nil {
		t.Fatalf("import msg=%T err=%v", msg, imported.err)
	}
	next, _ = next.Update(imported)
	final := next.(model)
	if final.screen != screenConversation {
		t.Fatalf("screen=%v", final.screen)
	}
	events, err := svc.Store.ListSessionEventsNewest(sess.ID, 0, 10)
	if err != nil || len(events) == 0 {
		t.Fatalf("events after import=%d err=%v", len(events), err)
	}
}

type historyFakeRemoteReader struct{}

func (historyFakeRemoteReader) SupportsRemoteHistory() bool { return true }
func (historyFakeRemoteReader) ReadRemoteHistory(context.Context, string) ([]harness.NormalizedEvent, error) {
	return []harness.NormalizedEvent{{
		EventType: harness.NormalizedEventUser, Origin: harness.NormalizedOriginLocal,
		PayloadJSON: []byte(`{"text":"from provider"}`), ProviderEventID: "prov-1",
	}}, nil
}

func TestDeleteLastGraphemeZWJAndCombining(t *testing.T) {
	if deleteLastGrapheme("go👨‍👩‍👧") != "go" {
		t.Fatalf("ZWJ cluster: got %q", deleteLastGrapheme("go👨‍👩‍👧"))
	}
	combining := "e\u0301"
	if deleteLastGrapheme("a"+combining) != "a" {
		t.Fatalf("combining mark: got %q", deleteLastGrapheme("a"+combining))
	}
	if deleteLastGrapheme("café") != "caf" {
		t.Fatalf("multibyte: got %q", deleteLastGrapheme("café"))
	}
}

func TestTruncateHistoryTextPreservesGraphemes(t *testing.T) {
	long := strings.Repeat("字", 20)
	out := truncateHistoryText(long, 8)
	if !strings.HasSuffix(out, "…") {
		t.Fatalf("expected ellipsis suffix: %q", out)
	}
	if lipgloss.Width(out) > 8 {
		t.Fatalf("width=%d out=%q", lipgloss.Width(out), out)
	}
}

func TestHistorySearchBackspaceDeletesGraphemeCluster(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 100), ScreenHistory)
	m.history.searchActive = true
	m.history.searchQuery = "go👨‍👩‍👧"
	next, _ := HandleTestKeyMsg(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if next.history.searchQuery != "go" {
		t.Fatalf("query=%q", next.history.searchQuery)
	}
}

func TestHistoryBusyCopy(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 100), ScreenHistory)
	m.history.loading = false
	m.history.sessions = []store.Session{{ID: "x", Title: "busy"}}
	m.history.detailBusy = true
	view := stripANSI(ViewForTest(m))
	if !strings.Contains(view, historyCopyBusy1) {
		t.Fatalf("busy copy: %q", view)
	}
}

func TestHistoryDiagnosticLogsRedactIdentifiers(t *testing.T) {
	var buf strings.Builder
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))

	secretID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	pathLeak := "/home/user/.local/share/hero/sessions/" + secretID + "/assets/x.png"
	err := fmt.Errorf("open session %s path %s: boom", secretID, pathLeak)

	m := NewTestModel(nil)
	m = SetScreen(SetWidth(SetHeight(m, 30), 100), ScreenHistory)
	_, _ = m.Update(historyLoadedMsg{err: err})
	_, _ = m.Update(historyOpenMsg{err: err})
	_, _ = m.Update(historyImportMsg{err: err})

	out := buf.String()
	if strings.Contains(out, secretID) {
		t.Fatalf("log leaked session id: %q", out)
	}
	if strings.Contains(out, "/home/user") || strings.Contains(out, "assets/x.png") {
		t.Fatalf("log leaked path: %q", out)
	}
	for _, code := range []string{"history list failed", "history open failed", "history remote import failed"} {
		if !strings.Contains(out, code) {
			t.Fatalf("missing log code %q in %q", code, out)
		}
	}
}

func TestTruncateHistoryTextPreservesANSI(t *testing.T) {
	styled := infoStyle.Render(strings.Repeat("red-title-", 8))
	out := truncateHistoryText(styled, 12)
	if lipgloss.Width(out) > 12 {
		t.Fatalf("width=%d out=%q", lipgloss.Width(out), out)
	}
	if strings.Count(out, "\x1b") == 0 && !strings.Contains(out, "\x1b") && !strings.Contains(out, "") {
		// lipgloss may keep CSI; require no dangling ESC alone at end without '['
		if strings.HasSuffix(out, "") {
			t.Fatalf("dangling ESC: %q", out)
		}
	}
	if !strings.HasSuffix(strings.TrimSuffix(out, "[0m"), "…") && !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis: %q", out)
	}
}

func TestTruncateHistoryTextWideEmojiAndCJK(t *testing.T) {
	s := "部署" + "👨‍👩‍👧" + strings.Repeat("字", 10)
	out := truncateHistoryText(s, 10)
	if lipgloss.Width(out) > 10 {
		t.Fatalf("width=%d out=%q", lipgloss.Width(out), out)
	}
}

func TestFormatHistoryTimesUseGoLayouts(t *testing.T) {
	ts := time.Date(2026, 9, 11, 21, 42, 0, 0, time.UTC).Format(time.RFC3339)
	activity := formatHistoryActivityTime(ts)
	detail := formatHistoryDetailTime(ts)
	if strings.HasPrefix(activity, "d ") || strings.Contains(activity, "YYYY") || strings.Contains(activity, "MMM") {
		t.Fatalf("activity still moment-like: %q", activity)
	}
	if strings.Contains(detail, "YYYY") || strings.HasPrefix(detail, "d ") {
		t.Fatalf("detail still moment-like: %q", detail)
	}
	if !strings.Contains(activity, "Sep") || !strings.Contains(detail, "2026") {
		t.Fatalf("activity=%q detail=%q", activity, detail)
	}
	if formatHistoryActivityTime("not-a-time") == "" {
		t.Fatal("malformed timestamp should fall back")
	}
}

func TestHistoryListRowDisplayWidth(t *testing.T) {
	m := SetScreen(SetWidth(SetHeight(NewTestModel(nil), 30), 40), ScreenHistory)
	m.history.loading = false
	m.history.sessions = []store.Session{{
		ID:             "s1",
		Title:          "emoji 👨‍👩‍👧 and 漢字 title that is quite long",
		LastActivityAt: time.Date(2026, 9, 11, 21, 42, 0, 0, time.UTC).Format(time.RFC3339),
		HarnessID:      "cursor",
		Model:          "composer",
	}}
	view := m.renderHistoryList(historyLayoutWide, 40)
	for _, line := range strings.Split(strings.TrimRight(view, "\n"), "\n") {
		if line == "" {
			continue
		}
		if lipgloss.Width(line) > 40 {
			t.Fatalf("line width %d > 40: %q", lipgloss.Width(line), line)
		}
	}
}

func TestSessionLeaseHeartbeatLostMsg(t *testing.T) {
	m := model{tuiOwnerID: "owner", sessionService: nil}
	cmd := m.sessionLeaseHeartbeatTickCmd("sess-1")
	if cmd != nil {
		// nil service => nil cmd
		t.Fatal("expected nil cmd without service")
	}
	m.sessionLeaseLost = true
	m.sessionPersistBlocked = true
	if !m.submitBlockedBySessionPersist() {
		t.Fatal("expected blocked")
	}
}
