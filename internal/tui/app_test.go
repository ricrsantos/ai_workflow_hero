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
		"/hero-new", "/hero-start", "/hero-sync", "/hero-status",
		"/hero-approve", "/hero-reject", "/hero-continue", "/hero-back",
		"/hero-cancel", "/hero-finish", "/hero-archive", "/hero-resume",
		"/hero-cycles", "/hero-todos", "/hero-model", "/hero-help",
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
	m = SetWidth(m, 80)
	view := ViewForTest(m)
	if !contains(view, "/hero-new") {
		t.Fatalf("empty state missing /hero-new: %q", view)
	}
}

func TestHeroModelPaletteHint(t *testing.T) {
	m := NewTestModel(nil)
	for _, item := range PaletteItemsForTest(m) {
		if item.Label == "/hero-model" {
			if item.Hint != "select default model" {
				t.Fatalf("hint=%q want select default model", item.Hint)
			}
			return
		}
	}
	t.Fatal("missing /hero-model in palette")
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

func (h *recordingHarness) Name() string { return "recording" }
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
func (unavailableHarness) ResumeSession(context.Context, string) error { return errors.New("unavailable") }
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
	m := NewTestModel(svc)
	m = SetScreen(m, ScreenApprovals)
	next, cmd := HandleTestKey(m, "a")
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
	m := NewTestModel(svc)
	m = SetScreen(m, ScreenApprovals)
	next, cmd := HandleTestKey(m, "r")
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

func TestDispatchActionWithService(t *testing.T) {
	m := NewTestModel(newTestService(t))
	m = SetChatModelSlugForTest(m, "composer-2.5")
	_, cmd := HandleTestKey(m, "d")
	if cmd == nil {
		t.Fatal("expected dispatch cmd")
	}
	msg := RunCmdForTest(cmd)
	result, ok := msg.(actionResultMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	if result.err != nil {
		t.Fatal(result.err)
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
	if strings.Contains(view, "▸  Go to - Status") || strings.Contains(view, "▸ Go to - Status") {
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
  "harnesses": {"cursor": {"model": "composer-2.5", "enable_fast_model": false}}
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
	m = SetAvailableModelsForTest(m, []string{"composer-2.5", "cursor-grok-4.5-high"})
	m = SetChatModelSlugForTest(m, "composer-2.5")
	next, _ := RunPaletteItemForTest(m, "/hero-model")
	if !PickingModelForTest(next) {
		t.Fatal("expected model picker open")
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "Models") || !strings.Contains(view, "cursor-grok-4.5-high") {
		t.Fatalf("picker view=%q", view)
	}
	next = SetPaletteFilter(next, "grok")
	items := FilteredPalette(next)
	if len(items) != 1 || items[0].Label != "cursor-grok-4.5-high" {
		t.Fatalf("filtered=%v", items)
	}
	next = SetPaletteIndexForTest(next, 0)
	next, _ = HandleTestKey(next, "enter")
	if PickingModelForTest(next) {
		t.Fatal("picker should close after select")
	}
	data, err := os.ReadFile(filepath.Join(cfgDir, "hero.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"model": "cursor-grok-4.5-high"`) {
		t.Fatalf("hero.json not updated: %s", data)
	}
	if StatusKindForTest(next) != "ok" {
		t.Fatalf("status=%s", StatusKindForTest(next))
	}
}
