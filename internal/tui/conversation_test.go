package tui

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type streamingHarness struct {
	deltas        []string
	events        []harness.StreamDelta
	sessionID     string
	cancelCalled  bool
	lastPrompt    string
	lastSessionID string
	lastModel     string
	lastMode      string
	lastStageName string
}

func (h *streamingHarness) Name() string { return "streaming" }
func (h *streamingHarness) IsAvailable(context.Context) error { return nil }
func (h *streamingHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return &harness.Session{ID: "new"}, nil
}
func (h *streamingHarness) ResumeSession(context.Context, string) error { return nil }
func (h *streamingHarness) Execute(_ context.Context, req harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	h.lastPrompt = req.Prompt
	h.lastSessionID = req.SessionID
	h.lastModel = req.Model
	h.lastMode = req.Mode
	h.lastStageName = req.StageName
	out := ""
	if len(h.events) > 0 {
		for _, ev := range h.events {
			if ev.Kind == harness.StreamKindText || ev.Kind == "" {
				out += ev.Text
			}
			if req.OnStreamDelta != nil {
				req.OnStreamDelta(ev)
			}
		}
	} else {
		for _, d := range h.deltas {
			out += d
			if req.OnStreamDelta != nil {
				req.OnStreamDelta(harness.StreamDelta{Kind: harness.StreamKindText, Text: d})
			}
		}
	}
	if h.sessionID != "" {
		return &harness.ExecutionResult{
			SessionID:  h.sessionID,
			Output:     out,
			StreamDone: true,
		}, nil
	}
	return &harness.ExecutionResult{Output: out, StreamDone: true}, nil
}
func (h *streamingHarness) Cancel(_ context.Context, sessionID string) error {
	h.cancelCalled = true
	if sessionID == "" {
		return nil
	}
	return nil
}
func (h *streamingHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{State: harness.StatusIdle}, nil
}
func (h *streamingHarness) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{Dispatched: true}, nil
}

func newConversationTestService(t *testing.T) (*cycle.Service, *streamingHarness) {
	t.Helper()
	svc := newTestServiceWithRunningResearch(t)
	h := &streamingHarness{deltas: []string{"Hello", " world"}, sessionID: "sess-abc123"}
	svc.Harness = h
	return svc, h
}

func newTestServiceWithRunningResearch(t *testing.T) *cycle.Service {
	t.Helper()
	dir := t.TempDir()
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
	return svc
}

func TestConversationScreenNavigation(t *testing.T) {
	m := NewTestModel(nil)
	next, _ := HandleTestKey(m, "ctrl+6")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen = %v", CurrentScreen(next))
	}
}

func TestConversationStreamingSubmit(t *testing.T) {
	svc, h := newConversationTestService(t)
	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "What is Hero?")

	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after submit")
	}
	if cmd == nil {
		t.Fatal("expected wait cmd")
	}

	// Drain stream messages until execute completes.
	for IsConversationStreaming(next) {
		msg := cmd()
		next2, nextCmd := next.Update(msg)
		next = next2.(model)
		cmd = nextCmd
		if cmd == nil && IsConversationStreaming(next) {
			t.Fatal("streaming stalled without follow-up cmd")
		}
	}

	if h.lastPrompt != "What is Hero?" {
		t.Fatalf("prompt = %q", h.lastPrompt)
	}
	if got := ConversationTranscriptForTest(next); got != "Hello world" {
		t.Fatalf("transcript = %q", got)
	}
	if HarnessSessionIDForTest(next) != "sess-abc123" {
		t.Fatalf("session = %q", HarnessSessionIDForTest(next))
	}
	stored, err := svc.StageHarnessSessionID("research")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "sess-abc123" {
		t.Fatalf("stored session = %q", stored)
	}
	view := ViewForTest(next)
	if !contains(view, "Hello world") {
		t.Fatalf("view missing transcript: %q", view)
	}
}

func TestConversationResumeSession(t *testing.T) {
	svc, h := newConversationTestService(t)
	if err := svc.SetStageHarnessSessionID("research", "existing-session"); err != nil {
		t.Fatal(err)
	}
	h.deltas = []string{"resume"}
	h.sessionID = "existing-session"

	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	if HarnessSessionIDForTest(m) != "existing-session" {
		t.Fatalf("loaded session = %q", HarnessSessionIDForTest(m))
	}
	m = SetConversationInput(m, "continue")
	next, cmd := SubmitConversationForTest(m)
	for IsConversationStreaming(next) {
		msg := cmd()
		next2, nextCmd := next.Update(msg)
		next = next2.(model)
		cmd = nextCmd
		if cmd == nil && IsConversationStreaming(next) {
			t.Fatal("streaming stalled")
		}
	}
	if h.lastSessionID != "existing-session" {
		t.Fatalf("resume session = %q", h.lastSessionID)
	}
}

func TestConversationCancelDuringStream(t *testing.T) {
	svc, h := newConversationTestService(t)
	h.deltas = []string{"partial"}

	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "wait")
	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming")
	}
	// Apply first delta.
	msg := cmd()
	next2, nextCmd := next.Update(msg)
	next = next2.(model)
	next.harnessSessionID = "sess-cancel"
	next, cancelCmd := CancelConversationStreamForTest(next)
	if cancelCmd != nil {
		cancelMsg := cancelCmd()
		next3, _ := next.Update(cancelMsg)
		next = next3.(model)
	}
	if nextCmd != nil {
		// ignore further stream
	}
	if IsConversationStreaming(next) {
		t.Fatal("expected streaming stopped")
	}
	if !h.cancelCalled {
		t.Fatal("expected harness Cancel")
	}
	view := ViewForTest(next)
	if !contains(view, "Interrupted") && !next.streamInterrupted {
		t.Fatalf("expected interrupted state: %q", view)
	}
}

func TestConversationViewWhileStreaming(t *testing.T) {
	svc, _ := newConversationTestService(t)
	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hi")
	next, _ := SubmitConversationForTest(m)
	view := ViewForTest(next)
	if !strings.Contains(view, "Agent responding") {
		t.Fatalf("view: %q", view)
	}
}

func TestConversationThinkingAndToolActivity(t *testing.T) {
	svc, h := newConversationTestService(t)
	h.deltas = nil
	h.events = []harness.StreamDelta{
		{Kind: harness.StreamKindThinking, Text: "Let me inspect the parser."},
		{Kind: harness.StreamKindTool, Text: "Read parse.go"},
		{Kind: harness.StreamKindText, Text: "Looks good."},
	}
	h.sessionID = "sess-think"

	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "check stream")
	next, cmd := SubmitConversationForTest(m)
	for IsConversationStreaming(next) {
		msg := cmd()
		next2, nextCmd := next.Update(msg)
		next = next2.(model)
		cmd = nextCmd
		if cmd == nil && IsConversationStreaming(next) {
			t.Fatal("streaming stalled")
		}
	}
	if got := ConversationTranscriptForTest(next); got != "Looks good." {
		t.Fatalf("agent transcript = %q", got)
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "Thinking:") || !strings.Contains(view, "Let me inspect the parser.") {
		t.Fatalf("view missing thinking: %q", view)
	}
	if !strings.Contains(view, "Read parse.go") {
		t.Fatalf("view missing tool activity: %q", view)
	}
	if !strings.Contains(view, "Looks good.") {
		t.Fatalf("view missing agent answer: %q", view)
	}
}

func TestConversationEmptyStageShowsInput(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hello")
	view := ViewForTest(m)
	if !strings.Contains(view, "Free chat") {
		t.Fatalf("expected freechat header: %q", view)
	}
	if !strings.Contains(view, "hello") {
		t.Fatalf("expected visible input: %q", view)
	}
	if !strings.Contains(view, "composer-2.5") {
		t.Fatalf("expected default model in input status: %q", view)
	}
	if !strings.Contains(view, "Build") {
		t.Fatalf("expected Build mode label: %q", view)
	}
	if !strings.Contains(view, "tab mode") {
		t.Fatalf("expected tab hint: %q", view)
	}
}

func TestConversationTabTogglesMode(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	if ChatModeForTest(m) != harness.ModeBuild {
		t.Fatalf("mode=%q", ChatModeForTest(m))
	}
	next, _ := HandleTestKey(m, "tab")
	if ChatModeForTest(next) != harness.ModePlan {
		t.Fatalf("after tab mode=%q", ChatModeForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "Plan") {
		t.Fatalf("expected Plan label: %q", view)
	}
	next2, _ := HandleTestKey(next, "tab")
	if ChatModeForTest(next2) != harness.ModeBuild {
		t.Fatalf("toggle back mode=%q", ChatModeForTest(next2))
	}
}

func TestConversationSubmitPassesModePlan(t *testing.T) {
	svc, h := newConversationTestService(t)
	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetChatModeForTest(m, harness.ModePlan)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = SetConversationInput(m, "plan please")
	next, cmd := SubmitConversationForTest(m)
	for IsConversationStreaming(next) {
		msg := cmd()
		next2, nextCmd := next.Update(msg)
		next = next2.(model)
		cmd = nextCmd
		if cmd == nil && IsConversationStreaming(next) {
			t.Fatal("streaming stalled")
		}
	}
	if h.lastMode != harness.ModePlan {
		t.Fatalf("mode=%q", h.lastMode)
	}
	if h.lastModel != "composer-2.5" {
		t.Fatalf("model=%q", h.lastModel)
	}
}

func TestConversationInputGuidanceWhenEmpty(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	if strings.Contains(view, "type here") {
		t.Fatalf("placeholder must be removed: %q", view)
	}
	if !strings.Contains(view, "enter send") {
		t.Fatalf("expected input hints: %q", view)
	}
	if !strings.Contains(view, "Build") {
		t.Fatalf("expected Build mode: %q", view)
	}
	if !strings.Contains(view, "tab mode") {
		t.Fatalf("expected tab hint: %q", view)
	}
	if !ChatInputFocusedForTest(m) {
		t.Fatal("expected chat input focused on conversation screen")
	}
}

func TestConversationArrowKeysMoveCursor(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "abcd")
	if InputCursorForTest(m) != 4 {
		t.Fatalf("cursor=%d", InputCursorForTest(m))
	}
	next, _ := HandleTestKey(m, "left")
	if InputCursorForTest(next) != 3 {
		t.Fatalf("after left cursor=%d", InputCursorForTest(next))
	}
	next, _ = HandleTestKey(next, "left")
	next, _ = HandleTestKey(next, "left")
	if InputCursorForTest(next) != 1 {
		t.Fatalf("cursor=%d", InputCursorForTest(next))
	}
	// Insert in the middle.
	next, _ = HandleTestKeyMsg(next, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if ConversationInputForTest(next) != "aXbcd" {
		t.Fatalf("input=%q", ConversationInputForTest(next))
	}
	if InputCursorForTest(next) != 2 {
		t.Fatalf("cursor after insert=%d", InputCursorForTest(next))
	}
	next, _ = HandleTestKey(next, "home")
	if InputCursorForTest(next) != 0 {
		t.Fatalf("home cursor=%d", InputCursorForTest(next))
	}
	next, _ = HandleTestKey(next, "end")
	if InputCursorForTest(next) != 5 {
		t.Fatalf("end cursor=%d", InputCursorForTest(next))
	}
}

func TestConversationFocusCaretNoBlinkPipe(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	if strings.Contains(view, "type here") {
		t.Fatalf("unexpected placeholder: %q", view)
	}
	// Leave chat → hollow caret when returning via view of unfocused model.
	next, _ := HandleTestKey(m, "ctrl+1")
	if ChatInputFocusedForTest(next) {
		t.Fatal("expected focus lost on Status")
	}
	next = EnterConversationForTest(next)
	if !ChatInputFocusedForTest(next) {
		t.Fatal("expected focus restored")
	}
}

func TestConversationScreenNavFromEmptyInput(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next, _ := HandleTestKey(m, "ctrl+1")
	if CurrentScreen(next) != ScreenStatus {
		t.Fatalf("screen = %v, want Status", CurrentScreen(next))
	}
	m = EnterConversationForTest(m)
	next, _ = HandleTestKey(m, "ctrl+5")
	if CurrentScreen(next) != ScreenEvents {
		t.Fatalf("screen = %v, want Events", CurrentScreen(next))
	}
}

func TestConversationDigitsTypeWhenInputNonEmpty(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "opt ")
	next, _ := HandleTestKey(m, "1")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen = %v, want Conversation", CurrentScreen(next))
	}
	if ConversationInputForTest(next) != "opt 1" {
		t.Fatalf("input = %q", ConversationInputForTest(next))
	}
}

func TestConversationBareDigitDoesNotNavigateWhenEmpty(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next, _ := HandleTestKey(m, "1")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen = %v, want Conversation (bare 1 types)", CurrentScreen(next))
	}
	if ConversationInputForTest(next) != "1" {
		t.Fatalf("input = %q", ConversationInputForTest(next))
	}
}

func TestConversationBareQTypesNotQuit(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	next, cmd := HandleTestKey(m, "q")
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("bare q must not quit")
		}
	}
	if ConversationInputForTest(next) != "q" {
		t.Fatalf("input = %q", ConversationInputForTest(next))
	}
}

func TestConversationSubmitWithoutStage(t *testing.T) {
	dir := t.TempDir()
	heroDir := dir + "/.workflow-hero/config"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hero := []byte(`{
  "cli": {"version": "1.0.0", "installedAt": "2026-01-01T00:00:00Z", "tools": ["cursor"]},
  "assets": {"version": "1.0.0", "installedAt": "2026-01-01T00:00:00Z"},
  "harnesses": {"cursor": {"model": "composer-2.5", "enable_fast_model": false}}
}
`)
	if err := os.WriteFile(heroDir+"/hero.json", hero, 0o644); err != nil {
		t.Fatal(err)
	}
	// Minimal install so OpenService works? We need a Service without active cycle.
	// OpenService requires hero installed. Use cycle.OpenService with hero.json only may fail.
	// Prefer NewTestModel with a service that has Harness but no running stage.
	svc := newTestServiceInstalledNoCycle(t, dir)
	h := &streamingHarness{deltas: []string{"pong"}, sessionID: "free-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "ping")
	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming for freechat")
	}
	for IsConversationStreaming(next) {
		msg := cmd()
		next2, nextCmd := next.Update(msg)
		next = next2.(model)
		cmd = nextCmd
		if cmd == nil && IsConversationStreaming(next) {
			t.Fatal("streaming stalled")
		}
	}
	if h.lastPrompt != "ping" {
		t.Fatalf("prompt=%q", h.lastPrompt)
	}
	if h.lastStageName != "" {
		t.Fatalf("stage=%q want empty", h.lastStageName)
	}
	if h.lastModel != "composer-2.5" {
		t.Fatalf("model=%q", h.lastModel)
	}
	if HarnessSessionIDForTest(next) != "free-sess" {
		t.Fatalf("session=%q", HarnessSessionIDForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "Free chat") || !strings.Contains(view, "pong") {
		t.Fatalf("view=%q", view)
	}
}

func newTestServiceInstalledNoCycle(t *testing.T, dir string) *cycle.Service {
	t.Helper()
	if err := os.MkdirAll(dir+"/.workflow-hero/cycles/current", 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: Freechat
objective: test
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`)
	if err := os.WriteFile(dir+"/.workflow-hero/cycles/current/workflow-config.yml", cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}
