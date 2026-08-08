package tui

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
	next, _ := HandleTestKey(m, "3")
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
		if item.Label == "/hero:approve" {
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
	want := []string{"/hero:approve", "/hero:reject", "/hero:cancel", "/hero:finish", "/hero:archive", "/hero:resume", "/hero:help"}
	labels := map[string]bool{}
	for _, item := range PaletteItemsForTest(m) {
		labels[item.Label] = true
	}
	for _, label := range want {
		if !labels[label] {
			t.Fatalf("missing palette label %q; labels=%v", label, labels)
		}
	}
	if labels["/hero:run"] {
		t.Fatal("palette must not invent /hero:run")
	}
}

func TestEmptyCycleHintMentionsHeroNew(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	view := ViewForTest(m)
	if !contains(view, "/hero:new") {
		t.Fatalf("empty state missing /hero:new: %q", view)
	}
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
	if _, ok := labels["/hero-approve"]; ok {
		t.Fatal("hero-approve must not appear in imported list")
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
func (h *recordingHarness) SupportsChat() bool {
	return true
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
	m = ReloadPaletteForTest(m)
	next, cmd := RunPaletteItemForTest(m, "/opsx-archive")
	if cmd == nil {
		t.Fatal("expected import cmd")
	}
	if !contains(ViewForTest(next), "markdown expansion") {
		t.Fatalf("expected progress flash: %q", ViewForTest(next))
	}
	msg := cmd()
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
	if !contains(ViewForTest(next2.(model)), "import ok") {
		t.Fatalf("view: %q", ViewForTest(next2.(model)))
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
	m = ReloadPaletteForTest(m)
	_, cmd := RunPaletteItemForTest(m, "/fail-cmd")
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	msg := cmd()
	result := msg.(actionResultMsg)
	if result.err == nil {
		t.Fatal("expected dispatch unavailable error")
	}
	if !contains(result.err.Error(), "Cursor chat") {
		t.Fatalf("err = %v", result.err)
	}
}

type unavailableHarness struct{}

func (unavailableHarness) Name() string        { return "unavailable" }
func (unavailableHarness) SupportsChat() bool  { return true }
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
	svc := newTestService(t)
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m = SetScreen(m, ScreenApprovals)
	next, cmd := HandleTestKey(m, "a")
	if cmd == nil {
		t.Fatal("expected approve cmd")
	}
	msg := cmd()
	result, ok := msg.(actionResultMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	next2, _ := next.Update(result)
	if next2.(model).View() == "" {
		t.Fatal("expected view after approve")
	}
}

func TestDispatchActionWithService(t *testing.T) {
	svc := newTestService(t)
	m := NewTestModel(svc)
	_, cmd := HandleTestKey(m, "d")
	if cmd == nil {
		t.Fatal("expected dispatch cmd")
	}
	msg := cmd()
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
