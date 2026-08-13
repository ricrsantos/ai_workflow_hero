package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	lastAgentName string
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
	h.lastAgentName = req.AgentName
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

func newConversationTestModel(t *testing.T) (model, *streamingHarness, *cycle.Service) {
	t.Helper()
	svc, h := newConversationTestService(t)
	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	return m, h, svc
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
agents:
  orchestration_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
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

func drainConversationStream(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	next := m
	for IsConversationStreaming(next) {
		msg := runConversationCmd(cmd)
		if msg == nil {
			t.Fatal("streaming stalled without message")
		}
		next2, nextCmd := next.Update(msg)
		next = next2.(model)
		cmd = nextCmd
		if cmd == nil && IsConversationStreaming(next) {
			t.Fatal("streaming stalled without follow-up cmd")
		}
	}
	return next
}

// runConversationCmd executes a tea.Cmd, expanding BatchMsg and skipping wait ticks.
func runConversationCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch m := msg.(type) {
	case tea.BatchMsg:
		var found tea.Msg
		for _, nested := range m {
			if nested == nil {
				continue
			}
			inner := nested()
			switch inner.(type) {
			case convWaitTickMsg, statusTickMsg:
				continue
			case streamDeltaMsg, executeDoneMsg, streamCancelDoneMsg:
				return inner
			default:
				if found == nil {
					found = inner
				}
			}
		}
		return found
	case convWaitTickMsg, statusTickMsg:
		return nil
	default:
		return msg
	}
}

func TestConversationStreamingSubmit(t *testing.T) {
	m, h, svc := newConversationTestModel(t)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "What is Hero?")

	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after submit")
	}
	if cmd == nil {
		t.Fatal("expected wait cmd")
	}

	next = drainConversationStream(t, next, cmd)

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
	m, h, svc := newConversationTestModel(t)
	if err := svc.SetStageHarnessSessionID("research", "existing-session"); err != nil {
		t.Fatal(err)
	}
	h.deltas = []string{"resume"}
	h.sessionID = "existing-session"

	m = EnterConversationForTest(m)
	if HarnessSessionIDForTest(m) != "existing-session" {
		t.Fatalf("loaded session = %q", HarnessSessionIDForTest(m))
	}
	m = SetConversationInput(m, "continue")
	next, cmd := SubmitConversationForTest(m)
	next = drainConversationStream(t, next, cmd)
	if h.lastSessionID != "existing-session" {
		t.Fatalf("resume session = %q", h.lastSessionID)
	}
}

func TestConversationCancelDuringStream(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	h.deltas = []string{"partial"}

	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "wait")
	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming")
	}
	// Apply first delta.
	msg := runConversationCmd(cmd)
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

func TestConversationResponsePaneLayout(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	if !strings.Contains(view, "Agent response will appear here") {
		t.Fatalf("expected empty response pane: %q", view)
	}
	if !strings.Contains(view, "Agent") {
		t.Fatalf("expected Agent status label: %q", view)
	}
	if !strings.Contains(view, "↑↓ scroll") {
		t.Fatalf("expected scroll hint: %q", view)
	}
	if !strings.Contains(view, "ctrl+q quit") && !strings.Contains(view, "alt+1-6") {
		t.Fatalf("expected footer menu visible: %q", view)
	}
	// Etapa hint moved to status bar under ready (not in the chat header).
	if !strings.Contains(view, "ready") || !strings.Contains(view, "No active etapa") {
		t.Fatalf("expected ready + etapa hint in status: %q", view)
	}
	headerEnd := strings.Index(view, "Submit a message")
	if headerEnd < 0 {
		headerEnd = strings.Index(view, "Agent response")
	}
	if headerEnd > 0 && strings.Contains(view[:headerEnd], "No active etapa") {
		t.Fatalf("etapa hint must not sit in chat header: %q", view[:headerEnd])
	}
}

func TestConversationSessionInlineWithHeader(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m.harnessSessionID = "37e8feb2abcdcc44"
	view := ViewForTest(m)
	if !strings.Contains(view, "Chat · harness") {
		t.Fatalf("missing freechat header: %q", view)
	}
	if !strings.Contains(view, "│") || !strings.Contains(view, "session:") {
		t.Fatalf("expected session inline with pipe: %q", view)
	}
}

func TestConversationViewWhileStreaming(t *testing.T) {
	m, _, _ := newConversationTestModel(t)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hi")
	next, _ := SubmitConversationForTest(m)
	view := ViewForTest(next)
	if !strings.Contains(view, "Waiting for harness") {
		t.Fatalf("view missing wait animation: %q", view)
	}
	if !strings.Contains(view, "Agent") {
		t.Fatalf("view missing response pane: %q", view)
	}
	// Spinner must not prefix the Agent status label — only content area animates.
	if strings.Contains(view, " Agent ·") || strings.Contains(view, "Agent ·") {
		// "Agent ·" is expected; braille+Agent is not.
	}
	for _, frame := range waitAnimFrames {
		if strings.Contains(view, frame+" Agent") {
			t.Fatalf("wait spinner must not sit beside Agent label: %q", view)
		}
	}
}

func TestChatAccentRowIsSingleLine(t *testing.T) {
	long := strings.Repeat("abcdefghij", 20)
	styled := chatInOK.Render(long)
	row := chatAccentRow(chatAccentResponse, styled, 42)
	if strings.Count(row, "\n") != 0 {
		t.Fatalf("accent row must be a single visual line, got newlines: %q", row)
	}
	if lipgloss.Width(row) != 42 {
		t.Fatalf("row width=%d want 42", lipgloss.Width(row))
	}
	row2 := chatAccentRow(chatAccentResponse, "short", 42)
	if strings.Count(row2, "\n") != 0 {
		t.Fatalf("short accent row wrapped: %q", row2)
	}
	empty := chatAccentRow(chatAccentResponse, "", 42)
	if lipgloss.Width(empty) != 42 {
		t.Fatalf("empty row width=%d want 42", lipgloss.Width(empty))
	}
}

func TestResponseVisibleLinesScalesWithHeight(t *testing.T) {
	m := NewTestModel(nil)
	m = SetHeight(m, 20)
	m = SetWidth(m, 80)
	contentH := m.contentAreaHeight()
	if got := m.responseVisibleLines(contentH); got < chatResponseMinLines {
		t.Fatalf("short terminal: got %d", got)
	}
	m = SetHeight(m, 50)
	tall := m.responseVisibleLines(m.contentAreaHeight())
	m = SetHeight(m, 30)
	short := m.responseVisibleLines(m.contentAreaHeight())
	if tall <= short {
		t.Fatalf("taller terminal should grow response pane: tall=%d short=%d", tall, short)
	}
}

func TestConversationFrameFillsHeight(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 40)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	got := countContentLines(view)
	if got != 40 {
		t.Fatalf("frame lines=%d want 40\n%s", got, view)
	}
}

func TestStatusBarUsesTwoLines(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	if m.statusBarLineCount() != 2 {
		t.Fatalf("status lines = %d", m.statusBarLineCount())
	}
}

func TestConversationThinkingAndToolActivity(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	h.deltas = nil
	h.events = []harness.StreamDelta{
		{Kind: harness.StreamKindThinking, Text: "Let me inspect the parser."},
		{Kind: harness.StreamKindTool, Text: "Read parse.go"},
		{Kind: harness.StreamKindText, Text: "Looks good."},
	}
	h.sessionID = "sess-think"

	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "check stream")
	next, cmd := SubmitConversationForTest(m)
	next = drainConversationStream(t, next, cmd)
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
	if !strings.Contains(view, "Chat") {
		t.Fatalf("expected chat header: %q", view)
	}
	if !strings.Contains(view, "hello") {
		t.Fatalf("expected visible input: %q", view)
	}
	if !strings.Contains(view, "not set") {
		t.Fatalf("expected unset model in input status: %q", view)
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
	next = drainConversationStream(t, next, cmd)
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
	next = drainConversationStream(t, next, cmd)
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
	if !strings.Contains(view, "Chat") || !strings.Contains(view, "pong") {
		t.Fatalf("view=%q", view)
	}
}

func TestHeroNewRequiresDefaultModel(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceInstalledNoCycle(t, dir)
	m := NewTestModel(svc)
	m = SetAvailableModelsForTest(m, []string{"composer-2.5", "cursor-grok-4.5-high"})
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-new")
	if cmd != nil {
		t.Fatal("expected no async cmd when default model is missing")
	}
	if PickingModelForTest(next) {
		t.Fatal("model picker should not open; show status error instead")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "/hero-model") || !strings.Contains(view, "/hero-new again") {
		t.Fatalf("missing default-model hint: %q", view)
	}
}

func TestConversationSubmitRequiresDefaultModel(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	h := &streamingHarness{deltas: []string{"hi"}}
	svc.Harness = h
	m := NewTestModel(svc)
	m = SetAvailableModelsForTest(m, []string{"composer-2.5"})
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hello")
	next, cmd := SubmitConversationForTest(m)
	if cmd != nil {
		t.Fatal("expected no stream cmd before model selection")
	}
	if PickingModelForTest(next) {
		t.Fatal("model picker should not open; show status error instead")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if h.lastPrompt != "" {
		t.Fatalf("execute should not run without model, prompt=%q", h.lastPrompt)
	}
}

func TestHeroNewRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := "HERO_NEW_RUNTIME_MARKER"
	body := "# /hero-new — Start a New Development Cycle\n\n" + marker
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-new.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceInstalledNoCycle(t, dir)
	h := &streamingHarness{deltas: []string{"preparing cycle"}, sessionID: "new-cycle-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	next, cmd := BeginHeroRuntimeConversationForTest(m, "new")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after /hero-new")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, marker) {
		t.Fatalf("prompt missing hero-new body: %q", h.lastPrompt)
	}
	if h.lastModel != "composer-2.5" {
		t.Fatalf("model=%q want composer-2.5", h.lastModel)
	}
	if h.lastSessionID != "" {
		t.Fatalf("expected fresh session, got resume %q", h.lastSessionID)
	}
	if HarnessSessionIDForTest(next) != "new-cycle-sess" {
		t.Fatalf("session=%q", HarnessSessionIDForTest(next))
	}
	st, err := svc.Status()
	if err != nil || st.CycleNumber == 0 {
		t.Fatalf("expected active cycle after hero-new, status=%+v err=%v", st, err)
	}
	if st.Title != "" {
		t.Fatalf("title should be empty until hero-start, got %q", st.Title)
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "/hero-new") {
		t.Fatalf("missing user label in view: %q", view)
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

func TestHeroStartRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	agentDir := filepath.Join(dir, ".cursor", "agents")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmdMarker := "HERO_START_RUNTIME_MARKER"
	agentMarker := "ORCHESTRATION_AGENT_MARKER"
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# /hero-start\n\n"+cmdMarker), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\n"+agentMarker), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{deltas: []string{"starting cycle"}, sessionID: "start-cycle-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after /hero-start")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, cmdMarker) {
		t.Fatalf("prompt missing hero-start body: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, agentMarker) {
		t.Fatalf("prompt missing orchestration_agent body: %q", h.lastPrompt)
	}
	if h.lastModel != "gpt-5.3-codex-medium" {
		t.Fatalf("model=%q want gpt-5.3-codex-medium", h.lastModel)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if h.lastSessionID != "" {
		t.Fatalf("expected fresh session, got resume %q", h.lastSessionID)
	}
}

func newTestServiceWithRunningResearchInDir(t *testing.T, dir string) *cycle.Service {
	t.Helper()
	heroDir := dir + "/.workflow-hero/cycles/current"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: TUI Test
objective: test
agents:
  orchestration_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
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

func TestHeroStartRequiresActiveCycle(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceInstalledNoCycle(t, dir)
	m := NewTestModel(svc)
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	if cmd != nil {
		t.Fatal("expected no async cmd when no active cycle")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "/hero-new") {
		t.Fatalf("missing /hero-new hint: %q", StatusTextForTest(next))
	}
}

func TestHeroStartRequiresOrchestratorModel(t *testing.T) {
	dir := t.TempDir()
	heroDir := dir + "/.workflow-hero/cycles/current"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: Empty models
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
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	h := &streamingHarness{}
	svc.Harness = h

	m := NewTestModel(svc)
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	if cmd != nil {
		t.Fatal("expected no execute when orchestrator model missing")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if h.lastPrompt != "" {
		t.Fatalf("execute should not run, prompt=%q", h.lastPrompt)
	}
}

func setupHeroApproveRuntimeFiles(t *testing.T, dir string) {
	t.Helper()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	agentDir := filepath.Join(dir, ".cursor", "agents")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func newTestServiceWithPendingApprovalInDir(t *testing.T, dir string) *cycle.Service {
	t.Helper()
	heroDir := dir + "/.workflow-hero/cycles/current"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: TUI Test
objective: test
agents:
  orchestration_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
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
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestHeroApproveRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	cmdMarker := "HERO_APPROVE_RUNTIME_MARKER"
	agentMarker := "ORCHESTRATION_AGENT_APPROVE_MARKER"
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-approve.md"), []byte("# /hero-approve\n\n"+cmdMarker), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\n"+agentMarker), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	h := &streamingHarness{deltas: []string{"approving stage"}, sessionID: "approve-cycle-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(m, "/hero-approve")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after /hero-approve")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, cmdMarker) {
		t.Fatalf("prompt missing hero-approve body: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, agentMarker) {
		t.Fatalf("prompt missing orchestration_agent body: %q", h.lastPrompt)
	}
	if h.lastModel != "gpt-5.3-codex-medium" {
		t.Fatalf("model=%q want gpt-5.3-codex-medium", h.lastModel)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if h.lastSessionID != "" {
		t.Fatalf("expected fresh session, got resume %q", h.lastSessionID)
	}
}

func TestHeroApproveRequiresActiveCycle(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceInstalledNoCycle(t, dir)
	m := NewTestModel(svc)
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-approve")
	if cmd != nil {
		t.Fatal("expected no async cmd when no active cycle")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "/hero-new") {
		t.Fatalf("missing /hero-new hint: %q", StatusTextForTest(next))
	}
}

func TestHeroApproveRequiresPendingApproval(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{}
	svc.Harness = h

	m := NewTestModel(svc)
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-approve")
	if cmd != nil {
		t.Fatal("expected no execute when no pending approval")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "pending approval") {
		t.Fatalf("missing pending approval hint: %q", StatusTextForTest(next))
	}
	if h.lastPrompt != "" {
		t.Fatalf("execute should not run, prompt=%q", h.lastPrompt)
	}
}

func TestHeroApproveRequiresOrchestratorModel(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	heroDir := dir + "/.workflow-hero/cycles/current"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: Empty models
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
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	h := &streamingHarness{}
	svc.Harness = h

	m := NewTestModel(svc)
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-approve")
	if cmd != nil {
		t.Fatal("expected no execute when orchestrator model missing")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if h.lastPrompt != "" {
		t.Fatalf("execute should not run, prompt=%q", h.lastPrompt)
	}
}

func TestHeroRejectRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	cmdMarker := "HERO_REJECT_RUNTIME_MARKER"
	agentMarker := "ORCHESTRATION_AGENT_REJECT_MARKER"
	reason := "tests are failing"
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-reject.md"), []byte("# /hero-reject\n\n"+cmdMarker), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\n"+agentMarker), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	h := &streamingHarness{deltas: []string{"rejecting stage"}, sessionID: "reject-cycle-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	next, cmd := BeginHeroRejectExecuteForTest(m, reason)
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after /hero-reject execute")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, cmdMarker) {
		t.Fatalf("prompt missing hero-reject body: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, agentMarker) {
		t.Fatalf("prompt missing orchestration_agent body: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, reason) {
		t.Fatalf("prompt missing rejection reason: %q", h.lastPrompt)
	}
	if h.lastModel != "gpt-5.3-codex-medium" {
		t.Fatalf("model=%q want gpt-5.3-codex-medium", h.lastModel)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if h.lastSessionID != "" {
		t.Fatalf("expected fresh session, got resume %q", h.lastSessionID)
	}
}

func TestHeroRejectRequiresActiveCycle(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceInstalledNoCycle(t, dir)
	m := NewTestModel(svc)
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-reject")
	if cmd != nil {
		t.Fatal("expected no async cmd when no active cycle")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "/hero-new") {
		t.Fatalf("missing /hero-new hint: %q", StatusTextForTest(next))
	}
}

func TestHeroRejectRequiresPendingApproval(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{}
	svc.Harness = h

	m := NewTestModel(svc)
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-reject")
	if cmd != nil {
		t.Fatal("expected no execute when no pending approval")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "pending approval") {
		t.Fatalf("missing pending approval hint: %q", StatusTextForTest(next))
	}
	if h.lastPrompt != "" {
		t.Fatalf("execute should not run, prompt=%q", h.lastPrompt)
	}
}

func TestHeroRejectRequiresOrchestratorModel(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	heroDir := dir + "/.workflow-hero/cycles/current"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: Empty models
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
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	h := &streamingHarness{}
	svc.Harness = h

	m := NewTestModel(svc)
	m = OpenPalette(m)
	next, cmd := RunPaletteItemForTest(m, "/hero-reject")
	if cmd != nil {
		t.Fatal("expected no execute when orchestrator model missing")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("expected error status, got %s", StatusKindForTest(next))
	}
	if h.lastPrompt != "" {
		t.Fatalf("execute should not run, prompt=%q", h.lastPrompt)
	}
}

func TestHeroRejectRequiresReason(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	h := &streamingHarness{}
	svc.Harness = h

	m := NewTestModel(svc)
	next, _ := RunPaletteItemForTest(m, "/hero-reject")
	if !AwaitingRejectReasonForTest(next) {
		t.Fatal("expected awaiting reject reason after palette reject")
	}
	next = SetConversationInput(next, "")
	next, cmd := SubmitConversationForTest(next)
	if cmd != nil {
		t.Fatal("expected no execute without reason")
	}
	if !AwaitingRejectReasonForTest(next) {
		t.Fatal("expected still awaiting reject reason")
	}
	if ConversationErrorForTest(next) == "" {
		t.Fatal("expected rejection reason required error")
	}
	if h.lastPrompt != "" {
		t.Fatalf("execute should not run, prompt=%q", h.lastPrompt)
	}
}

func TestHeroRejectInlineReason(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	reason := "fix tests"
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-reject.md"), []byte("# /hero-reject\n\nREJECT_INLINE"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nAGENT_INLINE"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	h := &streamingHarness{deltas: []string{"ok"}, sessionID: "inline-reject"}
	svc.Harness = h

	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "/hero-reject "+reason)
	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming for inline /hero-reject")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, reason) {
		t.Fatalf("prompt missing inline reason: %q", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
}

func newTestServiceWithEscalatedStageInDir(t *testing.T, dir string) *cycle.Service {
	t.Helper()
	heroDir := dir + "/.workflow-hero/cycles/current"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: TUI Test
objective: test
agents:
  orchestration_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
stages:
  research:
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
	if err := svc.CloseStage("research", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reject("needs rework"); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("research"); err == nil {
		t.Fatal("expected escalation error on exhausted iteration budget")
	}
	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(EscalatedStageForTest(st), "research") {
		t.Fatalf("escalated=%q want research", EscalatedStageForTest(st))
	}
	return svc
}

func newTestServiceWithJudgePendingApprovalInDir(t *testing.T, dir string) *cycle.Service {
	t.Helper()
	heroDir := dir + "/.workflow-hero/cycles/current"
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte(`title: TUI Test
objective: test
agents:
  orchestration_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: false
  planning:
    enabled: false
  implementation:
    enabled: false
  qa:
    enabled: false
  judge:
    enabled: true
    max_iterations: 1
    require_human_approval: true
  browser_ui_validation:
    enabled: false
  qa_end_to_end:
    enabled: false
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
	if err := svc.StartStage("judge"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("judge", "ambiguous SDD", "", false); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(PendingApprovalForTest(st), "judge") {
		t.Fatalf("pending=%q want judge", PendingApprovalForTest(st))
	}
	return svc
}

func TestHeroCancelRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	reason := "scope changed"
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-cancel.md"), []byte("# /hero-cancel\n\nCANCEL_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nCANCEL_AGENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{deltas: []string{"cancelled"}, sessionID: "cancel-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	next, cmd := BeginHeroCancelExecuteForTest(m, reason)
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "CANCEL_MARKER") {
		t.Fatalf("missing command body: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, "CANCEL_AGENT") {
		t.Fatalf("missing agent body: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, reason) {
		t.Fatalf("missing cancel reason: %q", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q", h.lastAgentName)
	}
}

func TestHeroCancelRequiresActiveCycle(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceInstalledNoCycle(t, dir)
	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(m, "/hero-cancel")
	if cmd != nil {
		t.Fatal("expected no async cmd")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("status=%s", StatusKindForTest(next))
	}
}

func TestHeroFinishRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-finish.md"), []byte("# /hero-finish\n\nFINISH_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nFINISH_AGENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{deltas: []string{"finished"}, sessionID: "finish-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(m, "/hero-finish")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v", CurrentScreen(next))
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "FINISH_MARKER") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q", h.lastAgentName)
	}
}

func TestHeroContinueRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-continue.md"), []byte("# /hero-continue\n\nCONTINUE_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nCONTINUE_AGENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithEscalatedStageInDir(t, dir)
	h := &streamingHarness{deltas: []string{"continued"}, sessionID: "continue-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	next, cmd := BeginHeroContinueExecuteForTest(m, 2)
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v", CurrentScreen(next))
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "CONTINUE_MARKER") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, "hero continue --extra 2") {
		t.Fatalf("missing extra 2: %q", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q", h.lastAgentName)
	}
}

func TestHeroContinueRequiresEscalatedStage(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(m, "/hero-continue")
	if cmd != nil {
		t.Fatal("expected no async cmd")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("status=%s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "Escalated") {
		t.Fatalf("missing escalated hint: %q", StatusTextForTest(next))
	}
}

func TestHeroContinueInlineExtra(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-continue.md"), []byte("# /hero-continue\n\nINLINE_CONTINUE"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nAGENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithEscalatedStageInDir(t, dir)
	h := &streamingHarness{deltas: []string{"ok"}, sessionID: "inline-continue"}
	svc.Harness = h

	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "/hero-continue 3")
	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "hero continue --extra 3") {
		t.Fatalf("prompt=%q", h.lastPrompt)
	}
}

func TestHeroBackRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-back.md"), []byte("# /hero-back\n\nBACK_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nBACK_AGENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithJudgePendingApprovalInDir(t, dir)
	h := &streamingHarness{deltas: []string{"reopening planning"}, sessionID: "back-sess"}
	svc.Harness = h

	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(m, "/hero-back")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v", CurrentScreen(next))
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "BACK_MARKER") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q", h.lastAgentName)
	}
}

func TestHeroBackRequiresJudgePendingApproval(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(m, "/hero-back")
	if cmd != nil {
		t.Fatal("expected no async cmd")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("status=%s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "Judge") {
		t.Fatalf("missing judge hint: %q", StatusTextForTest(next))
	}
}
