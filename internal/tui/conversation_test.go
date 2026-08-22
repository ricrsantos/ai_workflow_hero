package tui

import (
	"context"
	"errors"
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
	sessionIDs    []string
	executeCount  int
	sessionID     string
	cancelCalled  bool
	lastPrompt    string
	lastSessionID string
	lastModel     string
	lastMode      string
	lastStageName string
	lastAgentName string
	lastProps     map[string]string
	err           error
	release       chan struct{}
}

func (h *streamingHarness) Name() string                      { return "streaming" }
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
	h.lastProps = harness.CloneProperties(req.Properties)
	h.executeCount++
	resultSession := h.sessionID
	if n := len(h.sessionIDs); n > 0 {
		idx := h.executeCount - 1
		if idx >= n {
			idx = n - 1
		}
		resultSession = h.sessionIDs[idx]
	}
	if h.err != nil {
		return nil, h.err
	}
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
	if h.release != nil {
		<-h.release
	}
	if resultSession != "" {
		return &harness.ExecutionResult{
			SessionID:  resultSession,
			Output:     out,
			StreamDone: true,
		}, nil
	}
	return &harness.ExecutionResult{Output: out, StreamDone: true}, nil
}
func (h *streamingHarness) Cancel(_ context.Context, sessionID string) error {
	h.cancelCalled = true
	if h.release != nil {
		select {
		case <-h.release:
		default:
			close(h.release)
		}
	}
	if sessionID == "" {
		return nil
	}
	return nil
}
func (h *streamingHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{State: harness.StatusIdle}, nil
}
func (h *streamingHarness) CheckHealth(context.Context, string) (harness.HarnessHealth, error) {
	return harness.HarnessHealth{ProcessAlive: true, ServerAlive: true, SessionAlive: true}, nil
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

func withDefaultChatModel(m model) model {
	m = SetChatModelSlugForTest(m, "composer-2.5")
	return SetChatHarnessIDForTest(m, "cursor")
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
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
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
	next, _ := HandleTestKey(m, "ctrl+1")
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
			case tea.BatchMsg:
				if got := runConversationCmd(func() tea.Msg { return inner }); got != nil {
					switch got.(type) {
					case conversationBatchMsg, streamDeltaMsg, executeDoneMsg, streamCancelDoneMsg:
						return got
					default:
						if found == nil {
							found = got
						}
					}
				}
			case convWaitTickMsg, statusTickMsg, harnessHealthProbeMsg:
				continue
			case conversationBatchMsg, streamDeltaMsg, executeDoneMsg, streamCancelDoneMsg:
				return inner
			case refreshDataMsg:
				if found == nil {
					found = inner
				}
				continue
			default:
				if found == nil {
					found = inner
				}
			}
		}
		return found
	case convWaitTickMsg, statusTickMsg, harnessHealthProbeMsg:
		return nil
	default:
		return msg
	}
}

func TestWaitConvBatchGroupsDeltasAndPrioritizesCompletion(t *testing.T) {
	ch := make(chan tea.Msg, 3)
	ch <- streamDeltaMsg{delta: harness.StreamDelta{Text: "one"}}
	ch <- streamDeltaMsg{delta: harness.StreamDelta{Text: "two"}}
	ch <- executeDoneMsg{}

	msg := waitConvBatchMsg(ch)()
	batch, ok := msg.(conversationBatchMsg)
	if !ok {
		t.Fatalf("message type = %T, want conversationBatchMsg", msg)
	}
	if len(batch.messages) != 3 {
		t.Fatalf("batched messages = %d, want 3", len(batch.messages))
	}
	if _, ok := batch.messages[len(batch.messages)-1].(executeDoneMsg); !ok {
		t.Fatalf("last batched message = %T, want executeDoneMsg", batch.messages[len(batch.messages)-1])
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

func TestConversationCancelDuringStreamWithoutSessionID(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	h.deltas = []string{"partial"}
	h.release = make(chan struct{})

	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "wait")
	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming")
	}
	msg := runConversationCmd(cmd)
	next2, _ := next.Update(msg)
	next = next2.(model)
	if HarnessSessionIDForTest(next) != "" {
		t.Fatalf("session should still be empty, got %q", HarnessSessionIDForTest(next))
	}
	next, cancelCmd := CancelConversationStreamForTest(next)
	if cancelCmd != nil {
		cancelMsg := cancelCmd()
		next3, _ := next.Update(cancelMsg)
		next = next3.(model)
	}
	if IsConversationStreaming(next) {
		t.Fatal("expected streaming stopped")
	}
	if !h.cancelCalled {
		t.Fatal("expected harness Cancel even without session id")
	}
}

func TestConversationCancelDuringStream(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	h.deltas = []string{"partial"}
	h.release = make(chan struct{})

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
	if !strings.Contains(view, "[HARN]") {
		t.Fatalf("expected HARN speaker label: %q", view)
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
		t.Fatalf("view missing wait placeholder: %q", view)
	}
	if !strings.Contains(view, "[HARN - composer-2.5 · cursor]") {
		t.Fatalf("view missing response speaker label: %q", view)
	}
	foundSpinner := false
	for _, frame := range waitAnimFrames {
		if strings.Contains(view, frame+" Waiting") {
			t.Fatalf("wait spinner must not sit beside Waiting for harness: %q", view)
		}
		if strings.Contains(view, frame) {
			foundSpinner = true
		}
	}
	if !foundSpinner {
		t.Fatalf("wait spinner must sit on the speaker status line: %q", view)
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

func TestHistoryBoxScrollsLongPrompt(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 40)
	m = EnterConversationForTest(m)
	longPrompt := strings.Repeat("word ", 200)
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: longPrompt})

	if m.maxHistoryScroll() == 0 {
		t.Fatal("long prompt should require history scroll")
	}
	hist := m.renderConversationHistory()
	if strings.Contains(hist, longPrompt) {
		t.Fatal("history box must not render full unwrapped prompt")
	}
	if !strings.Contains(hist, "You") {
		t.Fatalf("history box missing You label: %q", hist)
	}
}

func TestHistoryScrollChainBeforeResponse(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 40)
	m = EnterConversationForTest(m)
	m.transcript = append(m.transcript, convMessage{role: convRoleUser, content: strings.Repeat("line ", 120)})
	m.inputScrollOffset = 0
	m.historyScrollOffset = 2
	m.respScrollOffset = 1

	next, _ := HandleTestKey(m, "up")
	if next.historyScrollOffset != 1 {
		t.Fatalf("history offset = %d want 1", next.historyScrollOffset)
	}
	if next.respScrollOffset != 1 {
		t.Fatalf("response offset changed before history exhausted: %d", next.respScrollOffset)
	}

	next, _ = HandleTestKey(next, "up")
	if next.historyScrollOffset != 0 {
		t.Fatalf("history offset = %d want 0", next.historyScrollOffset)
	}
	if next.respScrollOffset != 1 {
		t.Fatalf("response offset should stay until history at top: %d", next.respScrollOffset)
	}

	next, _ = HandleTestKey(next, "up")
	if next.respScrollOffset != 0 {
		t.Fatalf("response offset = %d want 0", next.respScrollOffset)
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
	next, _ := HandleTestKey(m, "ctrl+2")
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
	next, _ := HandleTestKey(m, "ctrl+2")
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

	m := withDefaultChatModel(NewTestModel(svc))
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

	m := withDefaultChatModel(NewTestModel(svc))
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
		t.Fatalf("model=%q want YAML orchestration_agent slug", h.lastModel)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if h.lastSessionID != "" {
		t.Fatalf("expected fresh session, got resume %q", h.lastSessionID)
	}
	if HarnessSessionIDForTest(next) != "start-cycle-sess" {
		t.Fatalf("session=%q", HarnessSessionIDForTest(next))
	}
	stored, err := svc.StageHarnessSessionID("research")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "start-cycle-sess" {
		t.Fatalf("stored session=%q", stored)
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "session:") {
		t.Fatalf("header missing session id: %q", view)
	}

	h.deltas = []string{"continuing"}
	next = SetConversationInput(next, "continue grilling")
	next, cmd = SubmitConversationForTest(next)
	next = drainConversationStream(t, next, cmd)
	if h.lastSessionID != "start-cycle-sess" {
		t.Fatalf("follow-up resume session=%q", h.lastSessionID)
	}
	if h.lastModel != "gpt-5.3-codex-medium" {
		t.Fatalf("follow-up model=%q want YAML orchestration_agent slug", h.lastModel)
	}
}

func TestConversationSyncPreservesLiveSession(t *testing.T) {
	m, _, svc := newConversationTestModel(t)
	m = EnterConversationForTest(m)
	m.harnessSessionID = "live-orch-sess"

	m = EnterConversationForTest(m)
	if HarnessSessionIDForTest(m) != "live-orch-sess" {
		t.Fatalf("sync wiped live session: %q", HarnessSessionIDForTest(m))
	}
	stored, err := svc.StageHarnessSessionID("research")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "live-orch-sess" {
		t.Fatalf("live session not copied to stage: %q", stored)
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
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
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
	if cmd == nil {
		t.Fatal("expected async preflight cmd when no active cycle")
	}
	msg := RunCmdForTest(cmd)
	next, cmd = HandleTestMsg(next, msg)
	if cmd != nil {
		t.Fatal("expected preflight to finish without execute cmd when no active cycle")
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
	if cmd == nil {
		t.Fatal("expected async preflight when orchestrator model is missing")
	}
	msg := RunCmdForTest(cmd)
	next, cmd = HandleTestMsg(next, msg)
	if cmd != nil {
		t.Fatal("expected preflight to finish without execute when model is missing")
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
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
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

	m := withDefaultChatModel(NewTestModel(svc))
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
		t.Fatalf("model=%q want YAML orchestration_agent slug", h.lastModel)
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

	m := withDefaultChatModel(NewTestModel(svc))
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
		t.Fatalf("model=%q want YAML orchestration_agent slug", h.lastModel)
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

	m := withDefaultChatModel(NewTestModel(svc))
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

	m := withDefaultChatModel(NewTestModel(svc))
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
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
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
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
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

	m := withDefaultChatModel(NewTestModel(svc))
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

	m := withDefaultChatModel(NewTestModel(svc))
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

	m := withDefaultChatModel(NewTestModel(svc))
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

	m := withDefaultChatModel(NewTestModel(svc))
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

	m := withDefaultChatModel(NewTestModel(svc))
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

func TestHeroSyncRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-sync.md"), []byte("# /hero-sync\n\nSYNC_RUNTIME"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH_SYNC"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceInstalledNoCycle(t, dir)
	h := &streamingHarness{deltas: []string{"sync"}}
	svc.Harness = h

	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	next, cmd := RunPaletteItemForTest(OpenPalette(m), "/hero-sync")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v", CurrentScreen(next))
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "SYNC_RUNTIME") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q", h.lastAgentName)
	}
	if h.lastModel != "composer-2.5" {
		t.Fatalf("model=%q want composer-2.5 (not YAML orchestrator)", h.lastModel)
	}
	if h.lastSessionID != "" {
		t.Fatalf("expected fresh session, got %q", h.lastSessionID)
	}
}

func TestHeroSyncRequiresModel(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceInstalledNoCycle(t, dir)
	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(OpenPalette(m), "/hero-sync")
	if cmd != nil {
		t.Fatal("expected no async cmd without model")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("status=%s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "/hero-model") {
		t.Fatalf("missing model hint: %q", StatusTextForTest(next))
	}
}

func TestHeroStatusRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-status.md"), []byte("# /hero-status\n\nSTATUS_RUNTIME"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH_STATUS"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceInDir(t, dir)
	h := &streamingHarness{deltas: []string{"status"}}
	svc.Harness = h

	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	next, cmd := RunPaletteItemForTest(OpenPalette(m), "/hero-status")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v", CurrentScreen(next))
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "STATUS_RUNTIME") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, "hero status") {
		t.Fatalf("missing status preamble: %q", h.lastPrompt)
	}
	if h.lastModel != "composer-2.5" {
		t.Fatalf("model=%q want composer-2.5", h.lastModel)
	}
}

func TestHeroArchiveRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-archive.md"), []byte("# /hero-archive\n\nARCHIVE_RUNTIME"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH_ARCHIVE"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	h := &streamingHarness{deltas: []string{"archiving"}}
	svc.Harness = h

	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(OpenPalette(m), "/hero-archive")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v", CurrentScreen(next))
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "ARCHIVE_RUNTIME") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, "hero cycle archive") {
		t.Fatalf("missing archive preamble: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, "end2end_qa_agent") {
		t.Fatalf("archive must forbid stage-agent dispatch: %q", h.lastPrompt)
	}
	if h.lastModel != "gpt-5.3-codex-medium" {
		t.Fatalf("model=%q want YAML orchestration_agent slug, not YAML stage agent", h.lastModel)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if h.lastSessionID != "" {
		t.Fatalf("archive must start a fresh session, got resume %q", h.lastSessionID)
	}
}

func TestHeroArchiveUsesDefaultModelNotLiveStageSession(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-archive.md"), []byte("# /hero-archive\n\nARCHIVE_FRESH"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH_ARCHIVE"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	h := &streamingHarness{deltas: []string{"archiving"}}
	svc.Harness = h

	m := withDefaultChatModel(NewTestModel(svc))
	m = SetOrchestrationLiveForTest(m, true)
	m = SetHarnessSessionIDForTest(m, "e2e-stage-session")
	next, cmd := RunPaletteItemForTest(OpenPalette(m), "/hero-archive")
	next = drainConversationStream(t, next, cmd)
	if h.lastSessionID != "" {
		t.Fatalf("must not resume QA E2E/stage session %q", h.lastSessionID)
	}
	if h.lastModel != "gpt-5.3-codex-medium" {
		t.Fatalf("model=%q want YAML orchestration_agent slug", h.lastModel)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if !strings.Contains(h.lastPrompt, "ARCHIVE_FRESH") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
}

func TestHeroArchiveRequiresActiveCycle(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	svc := newTestServiceInstalledNoCycle(t, dir)
	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(OpenPalette(m), "/hero-archive")
	if cmd != nil {
		t.Fatal("expected no async cmd")
	}
	if StatusKindForTest(next) != "err" {
		t.Fatalf("status=%s", StatusKindForTest(next))
	}
	if !strings.Contains(StatusTextForTest(next), "/hero-new") {
		t.Fatalf("missing hint: %q", StatusTextForTest(next))
	}
}

func TestHeroResumeRuntimeConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-resume.md"), []byte("# /hero-resume\n\nRESUME_RUNTIME"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH_RESUME"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceInstalledNoCycle(t, dir)
	h := &streamingHarness{deltas: []string{"resuming"}}
	svc.Harness = h

	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	next, cmd := RunPaletteItemForTest(OpenPalette(m), "/hero-resume")
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v", CurrentScreen(next))
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "RESUME_RUNTIME") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q", h.lastAgentName)
	}
}

func TestHeroResumeInlineCycleNumber(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-resume.md"), []byte("# /hero-resume\n\nINLINE_RESUME"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceInstalledNoCycle(t, dir)
	h := &streamingHarness{deltas: []string{"resume"}}
	svc.Harness = h

	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "/hero-resume 4")
	next, cmd := SubmitConversationForTest(m)
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v", CurrentScreen(next))
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "INLINE_RESUME") {
		t.Fatalf("missing command: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, "Resume cycle C4") {
		t.Fatalf("missing cycle N in preamble: %q", h.lastPrompt)
	}
}

func TestHeroStartUsesYamlOrchestratorWithoutHeroModel(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-start.md"), []byte("# /hero-start\n\nSTART_YAML"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{deltas: []string{"starting"}}
	svc.Harness = h

	m := NewTestModel(svc)
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	if cmd == nil {
		t.Fatalf("expected execute with YAML orch model; status=%s %q", StatusKindForTest(next), StatusTextForTest(next))
	}
	next = drainConversationStream(t, next, cmd)
	if h.lastModel != "gpt-5.3-codex-medium" {
		t.Fatalf("model=%q want YAML orchestration_agent without /hero-model", h.lastModel)
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "gpt-5.3-codex-medium") {
		t.Fatalf("input box missing YAML orch model: %q", view)
	}
}

func TestHeroSyncPrefersYamlOrchestratorOverDefault(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-sync.md"), []byte("# /hero-sync\n\nSYNC_YAML"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	h := &streamingHarness{deltas: []string{"sync"}}
	svc.Harness = h

	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(OpenPalette(m), "/hero-sync")
	next = drainConversationStream(t, next, cmd)
	if h.lastModel != "gpt-5.3-codex-medium" {
		t.Fatalf("model=%q want YAML orchestration_agent, not /hero-model", h.lastModel)
	}
}

func TestConversationExecuteErrorWrapsInView(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	h.err = errors.New("cursor agent execute failed: exit status 1 (Cannot use this model: gpt-5.3-codex-medium. Available models: auto, gpt-5.3-codex-low, gpt-5.3-codex-high)")
	m = SetWidth(m, 80)
	m = SetHeight(m, 40)
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "hello")
	next, cmd := SubmitConversationForTest(m)
	next = drainConversationStream(t, next, cmd)
	view := ViewForTest(next)
	if !strings.Contains(view, "gpt-5.3-codex-high") {
		t.Fatalf("full error must be visible after wrap: %q", view)
	}
	errorLines := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Cannot use this model") || strings.Contains(line, "gpt-5.3-codex") {
			errorLines++
		}
	}
	if errorLines < 2 {
		t.Fatalf("expected wrapped error across multiple lines, got %d in %q", errorLines, view)
	}
}

func TestTabBarListsChatFirst(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	view := ViewForTest(m)
	chat := strings.Index(view, "Chat")
	status := strings.Index(view, "Status")
	if chat < 0 || status < 0 || chat > status {
		t.Fatalf("expected Chat before Status in tab bar: %q", view)
	}
}

func TestConversationAgentsBoxIdle(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	view := ViewForTest(m)
	if !strings.Contains(view, "agents: 0") {
		t.Fatalf("expected idle agents box: %q", view)
	}
}

func TestConversationHeaderMatchesDisplayStageName(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	m.conversationStage = "qa_end_to_end"
	m.status = cycle.StatusView{
		CycleNumber: 1,
		Stages: []cycle.StatusStage{
			{Name: "Qa End To End", Iteration: "1/3"},
		},
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "iter 1/3") {
		t.Fatalf("header should resolve display stage name to slug iter: %q", view)
	}
	if !strings.Contains(view, "qa_end_to_end") {
		t.Fatalf("header missing stage slug: %q", view)
	}
}

func TestConversationAgentsBoxAddsHeroTaskNotGenericHARN(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	m.runtimeAgentName = "orchestration_agent"
	m.liveAgents = []liveAgent{{Name: "orchestration_agent", Label: "ORCH", Model: "gpt-5.3-codex-medium"}}
	m.streaming = true

	next, _ := m.Update(streamDeltaMsg{delta: harness.StreamDelta{
		Kind:      harness.StreamKindTool,
		AgentName: "planning_agent",
		Model:     "gpt-5.3-codex-medium",
		CallID:    "t-plan",
		Phase:     harness.StreamPhaseStarted,
	}})
	got := LiveAgentsForTest(next.(model))
	if len(got) != 2 {
		t.Fatalf("live agents=%+v want ORCH+PLAN", got)
	}
	labels := got[0].Label + " " + got[1].Label
	if !strings.Contains(labels, "ORCH") || !strings.Contains(labels, "PLAN") {
		t.Fatalf("labels=%q want ORCH PLAN", labels)
	}

	next, _ = next.Update(streamDeltaMsg{delta: harness.StreamDelta{
		Kind:      harness.StreamKindTool,
		AgentName: "explore",
		Model:     "composer-2.5",
		CallID:    "t-explore",
		Phase:     harness.StreamPhaseStarted,
	}})
	got = LiveAgentsForTest(next.(model))
	if len(got) != 2 {
		t.Fatalf("nested explore must not chip HARN: %+v", got)
	}
	view := ViewForTest(next.(model))
	if strings.Contains(view, "HARN") {
		t.Fatalf("agents box must not show HARN for nested generic Task: %q", view)
	}
}

func TestConversationFreechatParentShowsHARN(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	m.liveAgents = []liveAgent{{Name: "", Label: agentShortLabel(""), Model: "composer-2.5"}}
	view := ViewForTest(m)
	if !strings.Contains(view, "HARN") {
		t.Fatalf("freechat parent should show HARN: %q", view)
	}
}

func TestConversationAgentsBoxLiveLabels(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	m.liveAgents = []liveAgent{
		{Name: "orchestration_agent", Label: "ORCH"},
		{Name: "backend_agent", Label: "BACK"},
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "agents: 2") {
		t.Fatalf("expected agents: 2: %q", view)
	}
	if !strings.Contains(view, "ORCH") || !strings.Contains(view, "BACK") {
		t.Fatalf("expected ORCH | BACK labels: %q", view)
	}
}

func TestConversationResponseSpeakerFollowsLiveAgent(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	m = SetChatModelSlugForTest(m, "grok-4.6")
	m = SetChatHarnessIDForTest(m, "cursor")
	m.liveAgents = []liveAgent{
		{Name: "orchestration_agent", Label: "ORCH", Model: "grok-4.6"},
		{Name: "qa_agent", Label: "QA", Model: "composer-2.5", CallID: "t1"},
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "[QA - composer-2.5 · cursor]") {
		t.Fatalf("expected QA speaker on response status: %q", view)
	}
	if strings.Contains(view, "[Agent") {
		t.Fatalf("must not print fixed Agent label: %q", view)
	}
}

// UI-C06-001 §5 / design D11: Codex speaker + input status harness id.
func TestConversationCodexSpeakerAndInputStatus(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 28)
	m = EnterConversationForTest(m)
	m = SetChatModelSlugForTest(m, "gpt-5.4")
	m = SetChatHarnessIDForTest(m, "codex")
	m.runtimeAgentName = "orchestration_agent"
	m.streaming = true
	m.liveAgents = []liveAgent{
		{Name: "orchestration_agent", Label: "ORCH", Model: "gpt-5.4"},
	}
	view := ViewForTest(m)
	if !strings.Contains(view, "[ORCH - gpt-5.4 · codex]") {
		t.Fatalf("expected Codex orch speaker: %q", view)
	}
	if !strings.Contains(view, "Build") || !strings.Contains(view, "gpt-5.4") || !strings.Contains(view, "codex") {
		t.Fatalf("expected input status Build · gpt-5.4 · codex: %q", view)
	}
}

// UI-C06-001 §5: unknown app-server event → yellow status warning, not raw JSON dump.
func TestConversationUnknownCodexEventWarnsInStatus(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m = EnterConversationForTest(m)
	m.streaming = true
	m.agentMsgIndex = 0
	m.transcript = []convMessage{{role: convRoleAgent, content: ""}}

	warnText := `WARNING: unrecognized Codex app-server event "totally/unknown"`
	next, _ := m.Update(streamDeltaMsg{delta: harness.StreamDelta{
		Kind:        harness.StreamKindWarning,
		Text:        warnText,
		HarnessType: "totally/unknown",
	}})
	nm := next.(model)
	if StatusKindForTest(nm) != "warn" {
		t.Fatalf("status kind=%q want warn", StatusKindForTest(nm))
	}
	if !strings.Contains(StatusTextForTest(nm), "unrecognized Codex") {
		t.Fatalf("status text=%q", StatusTextForTest(nm))
	}
	view := ViewForTest(nm)
	if strings.Contains(view, `"x":1`) || strings.Contains(view, "raw-json") {
		t.Fatalf("must not dump raw JSON in view: %q", view)
	}
	foundWarn := false
	for _, msg := range nm.transcript {
		if msg.role == convRoleWarning {
			foundWarn = true
			if strings.Contains(msg.content, "{") {
				t.Fatalf("warning transcript must not include JSON dump: %q", msg.content)
			}
		}
	}
	if !foundWarn {
		t.Fatal("expected warning message in transcript")
	}
}

func TestConversationSubagentTranscriptLabels(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	h.deltas = nil
	h.events = []harness.StreamDelta{
		{Kind: harness.StreamKindText, Text: "Launching QA\n"},
		{Kind: harness.StreamKindTool, Text: "Task qa_agent", AgentName: "qa_agent", Model: "composer-2.5", CallID: "t1", Phase: harness.StreamPhaseStarted},
		{Kind: harness.StreamKindText, Text: "Running unit tests\n", AgentName: "qa_agent", Model: "composer-2.5", CallID: "t1"},
		{Kind: harness.StreamKindTool, Text: "Task qa_agent (completed)", AgentName: "qa_agent", Model: "composer-2.5", CallID: "t1", Phase: harness.StreamPhaseCompleted},
		{Kind: harness.StreamKindText, Text: "QA failed\n"},
	}
	h.sessionID = "sess-sub"

	m = SetWidth(m, 80)
	m = SetHeight(m, 40)
	m = EnterConversationForTest(m)
	m.runtimeAgentName = "orchestration_agent"
	m = SetConversationInput(m, "start qa")
	next, cmd := SubmitConversationForTest(m)
	next = drainConversationStream(t, next, cmd)
	if len(LiveAgentsForTest(next)) != 0 {
		t.Fatalf("live agents after stream: %+v", LiveAgentsForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "[ORCH - composer-2.5 · cursor]") {
		t.Fatalf("missing orchestrator label: %q", view)
	}
	if !strings.Contains(view, "[QA - composer-2.5 · cursor]") {
		t.Fatalf("missing QA label: %q", view)
	}
	if !strings.Contains(view, "Launching QA") || !strings.Contains(view, "Running unit tests") || !strings.Contains(view, "QA failed") {
		t.Fatalf("missing transcript text: %q", view)
	}
	if strings.Contains(view, "Task qa_agent (completed)") {
		t.Fatalf("task lifecycle should not render as tool line: %q", view)
	}
}

func TestNewChatClearsSession(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m.harnessSessionID = "old-session"
	m.orchestrationLive = true
	m.contextUsedTokens = 180000
	m.transcript = []convMessage{{role: convRoleUser, content: "hello"}}
	next, cmd := RunPaletteItemForTest(m, "/new-chat")
	if cmd != nil {
		t.Fatal("new-chat should not spawn async cmd")
	}
	if len(ConversationTranscriptForTest(next)) != 0 {
		t.Fatal("expected empty transcript after new-chat")
	}
	if HarnessSessionIDForTest(next) != "" {
		t.Fatalf("session=%q want empty", HarnessSessionIDForTest(next))
	}
	if next.orchestrationLive {
		t.Fatal("orchestrationLive should be false")
	}
	if next.contextUsedTokens != 0 {
		t.Fatalf("context used=%d want 0", next.contextUsedTokens)
	}
	if StatusTextForTest(next) != "New chat started with default model." {
		t.Fatalf("status=%q", StatusTextForTest(next))
	}
}

func TestNewChatBlockedWhileStreaming(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetStreamingForTest(m, true)
	next, cmd := RunPaletteItemForTest(m, "/new-chat")
	if cmd != nil {
		t.Fatal("blocked new-chat should not spawn cmd")
	}
	if !strings.Contains(StatusTextForTest(next), "ctrl+c") {
		t.Fatalf("status=%q", StatusTextForTest(next))
	}
}

func setupDiscoverRuntimeFiles(t *testing.T, dir string) {
	t.Helper()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	agentDir := filepath.Join(dir, ".cursor", "agents")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# /hero-start\n\nHERO_START_RUNTIME_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCHESTRATION_AGENT_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "discover_agent.md"), []byte("---\nname: discover_agent\n---\n\nDISCOVER_AGENT_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDiscoverAgentYAML(t *testing.T, dir string) {
	t.Helper()
	heroDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	cfg := []byte(`title: TUI Test
objective: test
agents:
  orchestration_agent:
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
  discover_agent:
    harness: cursor
    model: claude-sonnet-4.6
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`)
	if err := os.WriteFile(filepath.Join(heroDir, "workflow-config.yml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHeroStartHandsOffToDiscoverAgent(t *testing.T) {
	dir := t.TempDir()
	setupDiscoverRuntimeFiles(t, dir)
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeDiscoverAgentYAML(t, dir)
	h := &streamingHarness{
		deltas:     []string{"ok"},
		sessionIDs: []string{"orch-sess", "disc-sess"},
	}
	svc.Harness = h

	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next = drainConversationStream(t, next, cmd)

	if h.lastAgentName != "discover_agent" {
		t.Fatalf("agent=%q want discover_agent", h.lastAgentName)
	}
	if !strings.Contains(h.lastPrompt, "DISCOVER_AGENT_MARKER") {
		t.Fatalf("prompt missing discover body: %q", h.lastPrompt)
	}
	if !strings.Contains(h.lastPrompt, "hero stage close") {
		t.Fatalf("prompt missing research close override: %q", h.lastPrompt)
	}
	if h.lastModel != "claude-sonnet-4.6-medium" {
		t.Fatalf("model=%q want claude-sonnet-4.6-medium", h.lastModel)
	}
	if h.lastSessionID != "" {
		t.Fatalf("discover execute should be a fresh session, got resume %q", h.lastSessionID)
	}
	if !ResearchLiveForTest(next) {
		t.Fatal("expected researchLive")
	}
	if RuntimeAgentNameForTest(next) != "discover_agent" {
		t.Fatalf("runtime agent=%q", RuntimeAgentNameForTest(next))
	}
	if OrchestrationSessionIDForTest(next) != "orch-sess" {
		t.Fatalf("orch session=%q", OrchestrationSessionIDForTest(next))
	}
	if ResearchSessionIDForTest(next) != "disc-sess" {
		t.Fatalf("disc session=%q", ResearchSessionIDForTest(next))
	}
	if HarnessSessionIDForTest(next) != "disc-sess" {
		t.Fatalf("live session=%q", HarnessSessionIDForTest(next))
	}
	stored, err := svc.StageHarnessSessionID("research")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "disc-sess" {
		t.Fatalf("stored research session=%q", stored)
	}

	h.deltas = []string{"grilling"}
	next = SetConversationInput(next, "continue grilling")
	next, cmd = SubmitConversationForTest(next)
	next = drainConversationStream(t, next, cmd)
	if h.lastAgentName != "discover_agent" {
		t.Fatalf("follow-up agent=%q", h.lastAgentName)
	}
	if h.lastSessionID != "disc-sess" {
		t.Fatalf("follow-up resume=%q want disc-sess", h.lastSessionID)
	}
	if h.lastModel != "claude-sonnet-4.6-medium" {
		t.Fatalf("follow-up model=%q", h.lastModel)
	}
}

func TestResearchControlSlashGoesToOrchestrator(t *testing.T) {
	dir := t.TempDir()
	setupDiscoverRuntimeFiles(t, dir)
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeDiscoverAgentYAML(t, dir)
	h := &streamingHarness{
		deltas:     []string{"ok"},
		sessionIDs: []string{"orch-sess", "disc-sess", "orch-sess"},
	}
	svc.Harness = h

	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next = drainConversationStream(t, next, cmd)
	if !ResearchLiveForTest(next) {
		t.Fatal("expected researchLive")
	}

	next = SetConversationInput(next, "/hero-approve")
	next.slashOverlayDismissed = true
	next, cmd = SubmitConversationForTest(next)
	next = drainConversationStream(t, next, cmd)
	if h.lastPrompt != "/hero-approve" {
		t.Fatalf("prompt=%q want /hero-approve follow-up", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent", h.lastAgentName)
	}
	if h.lastSessionID != "orch-sess" {
		t.Fatalf("control slash session=%q want orch-sess", h.lastSessionID)
	}
}

func TestDiscoverCloseResumesOrchestrator(t *testing.T) {
	dir := t.TempDir()
	setupDiscoverRuntimeFiles(t, dir)
	svc := newTestServiceWithRunningResearchInDir(t, dir)
	writeDiscoverAgentYAML(t, dir)
	h := &streamingHarness{
		deltas:     []string{"ok"},
		sessionIDs: []string{"orch-sess", "disc-sess", "disc-sess", "orch-sess"},
	}
	svc.Harness = h

	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next = drainConversationStream(t, next, cmd)

	if err := svc.CloseStage("research", "done", `{"agent":"discover_agent","input_tokens":1,"output_tokens":1}`, false); err != nil {
		t.Fatal(err)
	}

	next = SetConversationInput(next, "research finished")
	next, cmd = SubmitConversationForTest(next)
	next = drainConversationStream(t, next, cmd)

	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q want orchestration_agent after research close", h.lastAgentName)
	}
	if !strings.Contains(h.lastPrompt, "Research closed") {
		t.Fatalf("prompt missing continue-after-research: %q", h.lastPrompt)
	}
	if h.lastSessionID != "orch-sess" {
		t.Fatalf("resume session=%q want orch-sess", h.lastSessionID)
	}
	if ResearchLiveForTest(next) {
		t.Fatal("researchLive should be false after close")
	}
	if RuntimeAgentNameForTest(next) != "orchestration_agent" {
		t.Fatalf("runtime agent=%q", RuntimeAgentNameForTest(next))
	}
}

func TestConversationContextBarShownWhenCatalogHasWindow(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m.contextUsedTokens = 180000
	view := stripANSI(ViewForTest(m))
	if !strings.Contains(view, "↑↓ scroll") {
		t.Fatalf("missing scroll hint: %q", view)
	}
	if !strings.Contains(view, "180k/200k") {
		t.Fatalf("expected context bar label: %q", view)
	}
	if !strings.Contains(view, contextBarFillChar) {
		t.Fatalf("expected filled context bar: %q", view)
	}
}

func TestConversationContextBarHiddenWithoutWindow(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetChatModelSlugForTest(m, "not-a-catalog-model")
	m.contextUsedTokens = 180000
	view := stripANSI(ViewForTest(m))
	if strings.Contains(view, contextBarFillChar) || strings.Contains(view, contextBarEmptyChar) {
		t.Fatalf("bar should be omitted without context_window: %q", view)
	}
	if strings.Contains(view, "/200k") || strings.Contains(view, "180k/") {
		t.Fatalf("label should be omitted without context_window: %q", view)
	}
}

func TestExecuteDoneUpdatesContextUsedTokens(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m.streaming = true
	next, _ := m.Update(ExecuteDoneResultForTest(&harness.ExecutionResult{
		SessionID: "sess-1",
		Output:    "done",
		Usage:     harness.Usage{InputTokens: 100000, OutputTokens: 80000},
	}, nil))
	got := next.(model)
	if got.contextUsedTokens != 180000 {
		t.Fatalf("used=%d want 180000", got.contextUsedTokens)
	}
	view := stripANSI(ViewForTest(got))
	if !strings.Contains(view, "180k/200k") {
		t.Fatalf("view missing updated bar: %q", view)
	}
}

func TestHarnessSessionIDForPair_BlocksStageHarnessMismatch(t *testing.T) {
	svc := newTestServiceWithRunningResearch(t)
	if err := svc.SetStageHarnessID("research", "cursor"); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m = SetHarnessSessionIDForTest(m, "cursor-sess-abc")
	m = SetHarnessSessionHarnessIDForTest(m, "cursor")
	m.conversationStage = "research"

	got := HarnessSessionIDForPairForTest(m, "research", "opencode")
	if got != "" {
		t.Fatalf("session=%q want empty when stage harness differs", got)
	}
}

func TestHarnessSessionIDForPair_BlocksCodexThreadAsCursorOrOpenCode(t *testing.T) {
	svc := newTestServiceWithRunningResearch(t)
	if err := svc.SetStageHarnessID("research", "codex"); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m = SetHarnessSessionIDForTest(m, "thread-codex-xyz")
	m = SetHarnessSessionHarnessIDForTest(m, "codex")
	m.conversationStage = "research"

	for _, foreign := range []string{"cursor", "opencode"} {
		got := HarnessSessionIDForPairForTest(m, "research", foreign)
		if got != "" {
			t.Fatalf("codex thread must not resume as %s; got %q", foreign, got)
		}
	}
	if got := HarnessSessionIDForPairForTest(m, "research", "codex"); got != "thread-codex-xyz" {
		t.Fatalf("same-harness resume got %q", got)
	}
}

func TestHarnessSessionIDForPair_BlocksInMemoryHarnessMismatch(t *testing.T) {
	m := NewTestModel(nil)
	m = SetHarnessSessionIDForTest(m, "cursor-sess-abc")
	m = SetHarnessSessionHarnessIDForTest(m, "cursor")

	got := HarnessSessionIDForPairForTest(m, "", "opencode")
	if got != "" {
		t.Fatalf("session=%q want empty when in-memory harness differs", got)
	}
}

func TestHarnessSessionIDForPair_BlocksInMemoryCodexAsCursor(t *testing.T) {
	m := NewTestModel(nil)
	m = SetHarnessSessionIDForTest(m, "thread-codex-1")
	m = SetHarnessSessionHarnessIDForTest(m, "codex")

	got := HarnessSessionIDForPairForTest(m, "", "cursor")
	if got != "" {
		t.Fatalf("session=%q want empty when in-memory codex vs cursor", got)
	}
}

func TestHarnessSessionIDForPair_AllowsSameHarness(t *testing.T) {
	m := NewTestModel(nil)
	m = SetHarnessSessionIDForTest(m, "cursor-sess-abc")
	m = SetHarnessSessionHarnessIDForTest(m, "cursor")

	got := HarnessSessionIDForPairForTest(m, "", "cursor")
	if got != "cursor-sess-abc" {
		t.Fatalf("session=%q want preserved for same harness", got)
	}
}

func TestEmptyAgentResponseWarning(t *testing.T) {
	svc := newTestServiceWithRunningResearch(t)
	h := &streamingHarness{deltas: nil, events: nil}
	svc.Harness = h
	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = EnterConversationForTest(m)
	m = SetConversationInput(m, "ping")
	next, cmd := SubmitConversationForTest(m)
	next = drainConversationStream(t, next, cmd)
	view := next.View()
	if !strings.Contains(view, "empty response") {
		t.Fatalf("expected empty response warning in view, got:\n%s", view)
	}
}
