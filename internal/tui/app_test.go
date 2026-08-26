package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
)

func TestCanLaunch_RefusesNO_COLOR(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := CanLaunch(os.Stdout); got == nil {
		t.Fatal("expected refusal with NO_COLOR")
	}
}

func TestCanLaunch_RefusesNonTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	var buf bytes.Buffer
	if got := CanLaunch(&buf); got == nil {
		t.Fatal("expected refusal for non-TTY writer")
	}
}

func TestNavigateScreens(t *testing.T) {
	m := NewTestModel(nil)
	m = SetScreen(m, ScreenStatus)
	if CurrentScreen(m) != ScreenStatus {
		t.Fatalf("screen = %v", CurrentScreen(m))
	}
	next, _ := HandleTestKey(m, "ctrl+3")
	if CurrentScreen(next) != ScreenArtifacts {
		t.Fatalf("artifacts screen = %v", CurrentScreen(next))
	}
}

func TestBootOpensChat(t *testing.T) {
	m := NewTestModel(nil)
	if CurrentScreen(m) != ScreenConversation {
		t.Fatalf("boot screen = %v want conversation", CurrentScreen(m))
	}
	if !ChatInputFocusedForTest(m) {
		t.Fatal("expected chat input focused at boot")
	}
	view := ViewForTest(SetWidth(SetHeight(m, 24), 100))
	if !strings.Contains(view, "Chat") {
		t.Fatalf("expected Chat nav item in view: %q", view)
	}
}

func TestSlashOpensCommands(t *testing.T) {
	m := NewTestModel(nil)
	m = SetScreen(m, ScreenStatus)
	next, _ := HandleTestKey(m, "/")
	if CurrentScreen(next) != ScreenPalette {
		t.Fatalf("expected commands screen after /, got %v", CurrentScreen(next))
	}
}

func TestPaletteFilter(t *testing.T) {
	m := NewTestModel(nil)
	m = OpenPalette(m)
	m = SetPaletteFilter(m, "approve")
	items := FilteredPalette(m)
	if len(items) == 0 {
		t.Fatal("expected approve palette matches")
	}
	found := false
	for _, item := range items {
		if item.Label == "/hero-approve" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("items: %+v", items)
	}
}

func TestHeroPaletteSlashLabels(t *testing.T) {
	m := NewTestModel(nil)
	want := []string{
		"/new-chat",
		"/hero-new", "/hero-start", "/hero-sync", "/hero-status",
		"/hero-approve", "/hero-reject", "/hero-continue", "/hero-back",
		"/hero-cancel", "/hero-finish", "/hero-archive", "/hero-resume",
		"/hero-cycles", "/hero-todos", "/model", "/hero-config-update", "/harness", "/harness-reset", "/hero-help",
	}
	labels := map[string]bool{}
	for _, item := range PaletteItemsForTest(m) {
		labels[item.Label] = true
	}
	for _, label := range want {
		if !labels[label] {
			t.Fatalf("missing palette label %q; labels=%v", label, labels)
		}
	}
	if labels["/hero-run"] {
		t.Fatal("palette must not invent /hero-run")
	}
}

func TestEmptyCycleHintMentionsHeroNew(t *testing.T) {
	m := NewTestModel(nil)
	m = SetScreen(m, ScreenStatus)
	m = SetWidth(m, 80)
	view := ViewForTest(m)
	if !contains(view, "/hero-new") {
		t.Fatalf("empty state missing /hero-new: %q", view)
	}
}

func TestHeroModelPaletteHint(t *testing.T) {
	m := NewTestModel(nil)
	for _, item := range PaletteItemsForTest(m) {
		if item.Label == "/model" {
			if item.Hint != "select default model" {
				t.Fatalf("hint=%q want select default model", item.Hint)
			}
			return
		}
	}
	t.Fatal("missing /model in palette")
}

func TestImportedCommandsInPalette(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	projCmds := filepath.Join(dir, cursoradapter.CommandsDir)
	if err := os.MkdirAll(projCmds, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projCmds, "hero-approve.md"), []byte("hero"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projCmds, "opsx-propose.md"), []byte("---\ndescription: x\n---\npropose body"), 0o644); err != nil {
		t.Fatal(err)
	}
	userCmds := filepath.Join(home, cursoradapter.CommandsDir)
	if err := os.MkdirAll(userCmds, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userCmds, "my-tool.md"), []byte("tool body"), 0o644); err != nil {
		t.Fatal(err)
	}

	items := BuildPaletteForTest(dir, home)
	labels := map[string]string{}
	for _, item := range items {
		labels[item.Label] = item.Hint
	}
	if labels["/opsx-propose"] != "harness command · project" {
		t.Fatalf("opsx-propose hint: %q", labels["/opsx-propose"])
	}
	if labels["/my-tool"] != "harness command · user (~/.cursor/commands)" {
		t.Fatalf("my-tool hint: %q", labels["/my-tool"])
	}
	if labels["/hero-approve"] == "harness command · project" {
		t.Fatal("hero-approve must not appear as an imported harness command")
	}

	svc := newTestServiceInDir(t, dir)
	m := NewTestModel(svc)
	m = ReloadPaletteForTest(m)
	m = SetPaletteFilter(m, "opsx")
	filtered := FilteredPalette(m)
	if len(filtered) != 1 || filtered[0].Label != "/opsx-propose" {
		t.Fatalf("filter opsx: %+v", filtered)
	}
}

type recordingHarness struct {
	lastPrompt string
	dispatched bool
}

func (h *recordingHarness) Name() string                      { return "recording" }
func (h *recordingHarness) IsAvailable(context.Context) error { return nil }
func (h *recordingHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return &harness.Session{ID: "rec"}, nil
}
func (h *recordingHarness) ResumeSession(context.Context, string) error { return nil }
func (h *recordingHarness) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return &harness.ExecutionResult{Output: "ok", StreamDone: true}, nil
}
func (h *recordingHarness) Cancel(context.Context, string) error { return nil }
func (h *recordingHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{State: harness.StatusIdle}, nil
}
func (h *recordingHarness) Dispatch(_ context.Context, req harness.DispatchRequest) (harness.DispatchResult, error) {
	h.lastPrompt = req.Prompt
	h.dispatched = true
	return harness.DispatchResult{Dispatched: true, Message: "import ok"}, nil
}

func TestImportCommandDispatch(t *testing.T) {
	dir := t.TempDir()
	projCmds := filepath.Join(dir, cursoradapter.CommandsDir)
	if err := os.MkdirAll(projCmds, 0o755); err != nil {
		t.Fatal(err)
	}
	cmdPath := filepath.Join(projCmds, "opsx-archive.md")
	if err := os.WriteFile(cmdPath, []byte("---\na: 1\n---\narchive prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceInDir(t, dir)
	rec := &recordingHarness{}
	svc.Harness = rec

	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = ReloadPaletteForTest(m)
	next, cmd := RunPaletteItemForTest(m, "/opsx-archive")
	if cmd == nil {
		t.Fatal("expected import cmd")
	}
	if CurrentScreen(next) == ScreenPalette {
		t.Fatal("palette should close after selecting command")
	}
	if StatusKindForTest(next) != "running" {
		t.Fatalf("expected running status, got %s view=%q", StatusKindForTest(next), ViewForTest(next))
	}
	if !contains(ViewForTest(next), "/opsx-archive") || !contains(ViewForTest(next), "running") {
		t.Fatalf("expected status bar running: %q", ViewForTest(next))
	}
	msg := RunCmdForTest(cmd)
	result, ok := msg.(actionResultMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	if rec.lastPrompt != "archive prompt" {
		t.Fatalf("prompt = %q", rec.lastPrompt)
	}
	next2, _ := next.Update(result)
	view := ViewForTest(next2.(model))
	if !contains(view, "import ok") {
		t.Fatalf("view: %q", view)
	}
}

func TestImportCommandDispatchUnavailable(t *testing.T) {
	dir := t.TempDir()
	projCmds := filepath.Join(dir, cursoradapter.CommandsDir)
	if err := os.MkdirAll(projCmds, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projCmds, "fail-cmd.md"), []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceInDir(t, dir)
	svc.Harness = unavailableHarness{}

	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = ReloadPaletteForTest(m)
	_, cmd := RunPaletteItemForTest(m, "/fail-cmd")
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	msg := RunCmdForTest(cmd)
	result := msg.(actionResultMsg)
	if result.err == nil {
		t.Fatal("expected dispatch unavailable error")
	}
	if !contains(result.err.Error(), "Cursor chat") {
		t.Fatalf("err = %v", result.err)
	}
}

type unavailableHarness struct{}

func (unavailableHarness) Name() string { return "unavailable" }
func (unavailableHarness) IsAvailable(context.Context) error {
	return errors.New("unavailable")
}
func (unavailableHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return nil, errors.New("unavailable")
}
func (unavailableHarness) ResumeSession(context.Context, string) error {
	return errors.New("unavailable")
}
func (unavailableHarness) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return nil, errors.New("unavailable")
}
func (unavailableHarness) Cancel(context.Context, string) error { return nil }
func (unavailableHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{State: harness.StatusFailed}, nil
}
func (unavailableHarness) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{Dispatched: false, Message: "adapter offline"}, nil
}

func TestRefreshDataMsgUpdatesModel(t *testing.T) {
	m := NewTestModel(nil)
	m = SetScreen(m, ScreenStatus)
	m = SetWidth(m, 80)
	msg := RefreshDataForTest(cycle.StatusView{
		CycleNumber: 1,
		Title:       "Feature",
		Stages:      []cycle.StatusStage{{Name: "Research", Status: "Running"}},
	})
	next, _ := m.Update(msg)
	nextM := next.(model)
	view := nextM.View()
	if view == "" {
		t.Fatal("expected rendered view")
	}
	if !contains(view, "Feature") {
		t.Fatalf("view missing title: %q", view)
	}
}

func TestPendingApprovalDetection(t *testing.T) {
	st := cycle.StatusView{
		Stages: []cycle.StatusStage{
			{Name: "QA", Status: "PendingApproval", HumanApproval: "Pending"},
		},
	}
	if got := PendingApprovalForTest(st); got != "QA" {
		t.Fatalf("pending = %q", got)
	}
}

func TestApproveActionWithService(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-approve.md"), []byte("# /hero-approve\n\napprove body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nagent body"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	h := &streamingHarness{deltas: []string{"Stage approved."}, sessionID: "approve-key-sess"}
	svc.Harness = h
	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-approve")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after approve key")
	}
	if cmd == nil {
		t.Fatal("expected wait cmd")
	}
	next = drainConversationStream(t, next, cmd)
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if !strings.Contains(h.lastPrompt, "approve body") {
		t.Fatalf("prompt missing command body: %q", h.lastPrompt)
	}
}

func TestRejectActionWithService(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-reject.md"), []byte("# /hero-reject\n\nreject body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nagent body"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	h := &streamingHarness{deltas: []string{"Stage rejected."}, sessionID: "reject-key-sess"}
	svc.Harness = h
	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-reject")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !AwaitingRejectReasonForTest(next) {
		t.Fatal("expected awaiting reject reason after r key")
	}
	if IsConversationStreaming(next) {
		t.Fatal("should not stream before reason is submitted")
	}
	reason := "needs more test coverage"
	next = SetConversationInput(next, reason)
	next, cmd = SubmitConversationForTest(next)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after reject reason submit")
	}
	if cmd == nil {
		t.Fatal("expected wait cmd")
	}
	next = drainConversationStream(t, next, cmd)
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if !strings.Contains(h.lastPrompt, "reject body") {
		t.Fatalf("prompt missing command body: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, reason) {
		t.Fatalf("prompt missing rejection reason: %q", h.lastPrompt)
	}
}

func TestCancelActionWithService(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-cancel.md"), []byte("# /hero-cancel\n\ncancel body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nagent body"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{deltas: []string{"Cycle cancelled."}, sessionID: "cancel-key-sess"}
	svc.Harness = h
	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-cancel")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after cancel key")
	}
	next = drainConversationStream(t, next, cmd)
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if !strings.Contains(h.lastPrompt, "cancel body") {
		t.Fatalf("prompt missing command body: %q", h.lastPrompt)
	}
}

func TestFinishActionWithService(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-finish.md"), []byte("# /hero-finish\n\nfinish body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nagent body"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{deltas: []string{"Cycle finished."}, sessionID: "finish-key-sess"}
	svc.Harness = h
	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-finish")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after finish key")
	}
	next = drainConversationStream(t, next, cmd)
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if !strings.Contains(h.lastPrompt, "finish body") {
		t.Fatalf("prompt missing command body: %q", h.lastPrompt)
	}
}

func TestDispatchKeyRemoved(t *testing.T) {
	m := NewTestModel(newTestService(t))
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = SetScreen(m, ScreenStatus)
	next, cmd := HandleTestKey(m, "d")
	if cmd != nil {
		t.Fatal("d must not start harness dispatch")
	}
	if CurrentScreen(next) != ScreenStatus {
		t.Fatalf("screen=%v want status", CurrentScreen(next))
	}
}

func TestPaletteHasGoToChat(t *testing.T) {
	m := NewTestModel(nil)
	found := false
	for _, item := range PaletteItemsForTest(m) {
		if item.Label == "Go to - Chat" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing Go to - Chat in palette")
	}
	next, cmd := RunPaletteItemForTest(m, "Go to - Chat")
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
}

func TestPaletteEnterSelectsGoScreen(t *testing.T) {
	m := NewTestModel(nil)
	m = OpenPalette(m)
	next, cmd := HandleTestKeyMsg(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("unexpected cmd: %v", cmd)
	}
	if CurrentScreen(next) == ScreenPalette {
		t.Fatal("expected to leave palette on enter with default selection")
	}
}

func TestPaletteScrollKeepsSelectionVisible(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m.height = 16
	m = OpenPalette(m)
	m = ReloadPaletteForTest(m)

	items := FilteredPalette(m)
	if len(items) < 8 {
		t.Fatalf("need enough palette items for scroll test, got %d", len(items))
	}

	// Move selection far down; offset must follow so first items leave the window.
	next := m
	for i := 0; i < 10; i++ {
		var cmd tea.Cmd
		next, cmd = HandleTestKey(next, "down")
		_ = cmd
	}
	if PaletteOffsetForTest(next) == 0 {
		t.Fatal("expected paletteOffset > 0 after moving down")
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "more above") {
		t.Fatalf("expected scroll-above hint: %q", view)
	}
	if !strings.Contains(view, "of "+itoa(len(items))) && !strings.Contains(view, "of") {
		t.Fatalf("expected range caption: %q", view)
	}
	// Top item should no longer be the selected line when scrolled.
	if strings.Contains(view, "▸  Go to - Chat") || strings.Contains(view, "▸ Go to - Chat") {
		t.Fatalf("scrolled view should not keep first item selected: %q", view)
	}
}

func TestPaletteViewportHidesOverflow(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m.height = 14
	m = OpenPalette(m)
	m = ReloadPaletteForTest(m)
	view := ViewForTest(m)
	if !strings.Contains(view, "more below") {
		t.Fatalf("expected more-below hint on short terminal: %q", view)
	}
	// Quit is usually last — should be outside the first page.
	if strings.Contains(view, "Quit — exit TUI") {
		t.Fatalf("last items should be scrolled out of initial viewport: %q", view)
	}
}

// ---------------------------------------------------------------------------
// Navigation while streaming
// ---------------------------------------------------------------------------

func TestNavigationAllowedWhileStreaming(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetStreamingForTest(m, true)

	for _, key := range []string{"alt+2", "alt+3", "alt+4", "alt+5"} {
		next, _ := HandleTestKey(m, key)
		if CurrentScreen(next) == ScreenConversation {
			t.Errorf("key %q: expected to leave Chat while streaming, got ScreenConversation", key)
		}
		if !IsConversationStreaming(next) {
			t.Errorf("key %q: streaming must remain true after navigation", key)
		}
	}
}

func TestNavigateBackToChatWhileStreaming(t *testing.T) {
	m := NewTestModel(nil)
	m = SetScreen(m, ScreenStatus)
	m = SetStreamingForTest(m, true)

	// alt+1 goes back to chat
	next, _ := HandleTestKey(m, "alt+1")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("alt+1 from Status while streaming: screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("streaming must remain true after returning to Chat")
	}
}

// ---------------------------------------------------------------------------
// Stream messages processed regardless of current screen
// ---------------------------------------------------------------------------

func TestStreamDeltaProcessedOffChatScreen(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetStreamingForTest(m, true)
	// Navigate away to Status
	m, _ = HandleTestKey(m, "alt+2")
	if CurrentScreen(m) != ScreenStatus {
		t.Fatal("expected Status screen")
	}

	// Inject a stream delta — it must be processed even off Chat.
	next, _ := m.Update(StreamDeltaMsgForTest("hello world"))
	nextM := next.(model)
	transcript := ConversationTranscriptForTest(nextM)
	if !strings.Contains(transcript, "hello world") {
		t.Fatalf("stream delta not processed off-Chat: transcript=%q", transcript)
	}
}

func TestExecuteDoneProcessedOffChatScreen(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetStreamingForTest(m, true)
	// Navigate away
	m, _ = HandleTestKey(m, "alt+3")

	// Inject executeDoneMsg — streaming must clear.
	next, _ := m.Update(ExecuteDoneMsgForTest(nil))
	nextM := next.(model)
	if IsConversationStreaming(nextM) {
		t.Fatal("streaming must clear when executeDoneMsg arrives off-Chat screen")
	}
}

// ---------------------------------------------------------------------------
// Confirmation dialog
// ---------------------------------------------------------------------------

func TestDestructivePaletteActionWhileStreamingShowsConfirm(t *testing.T) {
	m := NewTestModel(nil)
	m = SetStreamingForTest(m, true)
	m = OpenPalette(m)

	next, _ := RunPaletteItemForTest(m, "/hero-cancel")
	if !ConfirmPendingForTest(next) {
		t.Fatal("expected confirmPending after /hero-cancel while streaming")
	}
	if !strings.Contains(ConfirmMsgForTest(next), "y/N") {
		t.Fatalf("confirm msg missing [y/N]: %q", ConfirmMsgForTest(next))
	}
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("expected Chat screen for confirm prompt, got %v", CurrentScreen(next))
	}
	// Confirm dialog must be visible in the rendered view.
	view := ViewForTest(SetWidth(SetHeight(next, 24), 80))
	if !strings.Contains(view, "y/N") {
		t.Fatalf("confirm prompt not in view: %q", view)
	}
}

func TestConfirmDialogNDenies(t *testing.T) {
	m := NewTestModel(nil)
	m = SetStreamingForTest(m, true)
	m = OpenPalette(m)
	next, _ := RunPaletteItemForTest(m, "/hero-cancel")
	if !ConfirmPendingForTest(next) {
		t.Fatal("expected confirmPending")
	}

	// Press n — confirmation must be dismissed, streaming still running.
	denied, _ := HandleTestKey(next, "n")
	if ConfirmPendingForTest(denied) {
		t.Fatal("confirmPending must clear after n")
	}
	if !IsConversationStreaming(denied) {
		t.Fatal("streaming must still be true after denial")
	}
}

func TestConfirmDialogEscDenies(t *testing.T) {
	m := NewTestModel(nil)
	m = SetStreamingForTest(m, true)
	m = OpenPalette(m)
	next, _ := RunPaletteItemForTest(m, "/hero-new")
	if !ConfirmPendingForTest(next) {
		t.Fatal("expected confirmPending")
	}

	denied, _ := HandleTestKey(next, "esc")
	if ConfirmPendingForTest(denied) {
		t.Fatal("confirmPending must clear after esc")
	}
	if !IsConversationStreaming(denied) {
		t.Fatal("streaming must still be true after esc denial")
	}
}

func TestConfirmClearsWhenStreamFinishes(t *testing.T) {
	m := NewTestModel(nil)
	m = SetStreamingForTest(m, true)
	m = OpenPalette(m)
	next, _ := RunPaletteItemForTest(m, "/hero-cancel")
	if !ConfirmPendingForTest(next) {
		t.Fatal("expected confirmPending")
	}

	// Stream finishes naturally before the user answers.
	afterDone, _ := next.Update(ExecuteDoneMsgForTest(nil))
	afterM := afterDone.(model)
	if ConfirmPendingForTest(afterM) {
		t.Fatal("confirmPending must auto-clear when stream finishes")
	}
}

func TestNonDestructivePaletteActionNotConfirmedWhileStreaming(t *testing.T) {
	m := NewTestModel(nil)
	m = SetStreamingForTest(m, true)
	m = OpenPalette(m)

	// /hero-approve is non-destructive while streaming (needs stream to end first).
	next, _ := RunPaletteItemForTest(m, "/hero-approve")
	if ConfirmPendingForTest(next) {
		t.Fatal("/hero-approve must not trigger confirm dialog while streaming")
	}
}

func TestAltQWhileStreamingShowsConfirm(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetStreamingForTest(m, true)

	next, cmd := HandleTestKey(m, "alt+q")
	// cmd should NOT be tea.Quit — that would exit without asking.
	if cmd != nil {
		// Run the cmd to see if it's Quit.
		msg := RunCmdForTest(cmd)
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Fatal("alt+q while streaming must not quit immediately — should show confirm dialog")
		}
	}
	if !ConfirmPendingForTest(next) {
		t.Fatal("alt+q while streaming must set confirmPending")
	}
	if !strings.Contains(ConfirmMsgForTest(next), "Quit") {
		t.Fatalf("confirm msg should mention Quit: %q", ConfirmMsgForTest(next))
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func newTestService(t *testing.T) *cycle.Service {
	t.Helper()
	return newTestServiceInDir(t, t.TempDir())
}

func newTestServiceInDir(t *testing.T, dir string) *cycle.Service {
	t.Helper()
	heroDir := dir + "/.workflow-hero/cycles/current"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: TUI Test
objective: test
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: false
  qa:
    enabled: true
    max_iterations: 1
    require_human_approval: true
`)
	if err := os.WriteFile(heroDir+"/workflow-config.yml", cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/.workflow-hero/config", 0o755); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "done", "", false); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestHeroModelPickerSelectsAndPersists(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "cli": {"version": "1.0.0", "installedAt": "2026-01-01T00:00:00Z", "tools": ["cursor"]},
  "assets": {"version": "1.0.0", "installedAt": "2026-01-01T00:00:00Z"},
  "harnesses": {
    "cursor": {"enabled": true, "model": "", "enable_fast_model": false},
    "opencode": {"enabled": true}
  }
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "cycles", "current"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: Model Picker
objective: test
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`)
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m := NewTestModel(svc)
	if ChatModelSlugForTest(m) != "" {
		t.Fatalf("must not auto-select a default model, got %q", ChatModelSlugForTest(m))
	}
	m = SetAvailableModelsForTest(m, []string{"composer-2.5", "auto"})
	next := OpenHeroModelForTest(m)
	if !PickingModelForTest(next) {
		t.Fatal("expected model picker open")
	}
	if ModelPickerHarnessForTest(next) != "" {
		t.Fatal("expected harness submenu first")
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "/model · select harness") || !strings.Contains(view, "Cursor") || !strings.Contains(view, "OpenCode") {
		t.Fatalf("harness submenu view=%q", view)
	}
	if strings.Contains(view, "auto") && strings.Contains(view, "/model · select harness") {
		t.Fatalf("models must not appear before harness select: %q", view)
	}
	next = SetPaletteIndexForTest(next, 0)
	next, _ = HandleTestKey(next, "enter")
	if ModelPickerHarnessForTest(next) != "cursor" {
		t.Fatalf("harness=%q", ModelPickerHarnessForTest(next))
	}
	view = ViewForTest(next)
	if !strings.Contains(view, "/model · Cursor") || !strings.Contains(view, "auto") {
		t.Fatalf("model list view=%q", view)
	}
	next = SetPaletteFilter(next, "auto")
	items := FilteredPalette(next)
	if len(items) != 1 || items[0].Label != "auto" {
		t.Fatalf("filtered=%v", items)
	}
	next = SetPaletteIndexForTest(next, 0)
	next, _ = HandleTestKey(next, "enter")
	if PickingModelForTest(next) {
		t.Fatal("picker should close after select")
	}
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation after model select", CurrentScreen(next))
	}
	data, err := os.ReadFile(filepath.Join(cfgDir, "hero.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"model": "auto"`) {
		t.Fatalf("hero.json not updated: %s", data)
	}
	// C5: `auto` is in the Cursor catalog (with `na` properties), so the pair
	// commits cleanly without a missing-catalog warning.
	if StatusKindForTest(next) != "ok" {
		t.Fatalf("status=%s", StatusKindForTest(next))
	}
}

func TestHeroModelPickerFromSlashMenuReturnsToChat(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {
    "cursor": {"enabled": true, "model": "", "enable_fast_model": false},
    "opencode": {"enabled": true}
  }
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "cycles", "current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml"), []byte("title: t\nobjective: t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m := NewTestModel(svc)
	m = SetAvailableModelsForTest(m, []string{"composer-2.5", "auto"})
	m = OpenPalette(m)
	if CurrentScreen(m) != ScreenPalette {
		t.Fatalf("screen=%v", CurrentScreen(m))
	}
	next := OpenHeroModelForTest(m)
	next = SetPaletteIndexForTest(next, 0)
	next, _ = HandleTestKey(next, "enter") // cursor harness
	next = SetPaletteFilter(next, "auto")
	next = SetPaletteIndexForTest(next, 0)
	next, _ = HandleTestKey(next, "enter") // model without catalog metadata
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want chat after selecting model from slash menu", CurrentScreen(next))
	}
	if PickingModelForTest(next) {
		t.Fatal("model picker should be closed")
	}
}

func TestHeroModelPickerSkipsHarnessWhenOnlyOneEnabled(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": false}}
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "cycles", "current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml"), []byte("title: t\nobjective: t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m := NewTestModel(svc)
	m = SetAvailableModelsForTest(m, []string{"composer-2.5"})
	next := OpenHeroModelForTest(m)
	if ModelPickerHarnessForTest(next) != "cursor" {
		t.Fatalf("harness=%q want cursor (skip submenu)", ModelPickerHarnessForTest(next))
	}
	view := ViewForTest(next)
	if strings.Contains(view, "OpenCode") {
		t.Fatalf("disabled harness should not appear: %q", view)
	}
}

func TestHeroModelPickerListsOpenCodeModels(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "harnesses": {"cursor": {"enabled": true}, "opencode": {"enabled": true}}
}
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), hero, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "cycles", "current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml"), []byte("title: t\nobjective: t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	m := NewTestModel(svc)
	m = SetModelOptionsForTest(m, []harnessmgr.ModelOption{
		{Model: "composer-2.5", Harness: "cursor"},
		{Model: "anthropic/claude-sonnet-4", Harness: "opencode"},
		{Model: "xai/grok-4", Harness: "opencode"},
	})
	next := OpenHeroModelForTest(m)
	items := FilteredPalette(next)
	idx := -1
	for i, item := range items {
		if item.Label == "OpenCode" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("items=%v", items)
	}
	next = SetPaletteIndexForTest(next, idx)
	next, _ = HandleTestKey(next, "enter")
	if ModelPickerHarnessForTest(next) != "opencode" {
		t.Fatalf("harness=%q", ModelPickerHarnessForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "/model · OpenCode") {
		t.Fatalf("title=%q", view)
	}
	if !strings.Contains(view, "anthropic/claude-sonnet-4") || !strings.Contains(view, "xai/grok-4") {
		t.Fatalf("missing OpenCode models: %q", view)
	}
	if strings.Contains(view, "composer-2.5") {
		t.Fatalf("cursor models leaked into OpenCode list: %q", view)
	}
}
