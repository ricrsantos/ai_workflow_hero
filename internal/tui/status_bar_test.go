package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCompleteBusyExecuteStatusClearsResumeBusy(t *testing.T) {
	m := NewTestModel(nil)
	m = m.setStatusRunning("/hero-resume")
	m = m.completeBusyExecuteStatus(true, busyExecuteCompletedText(m.statusLabel))
	if ActionBusyForTest(m) {
		t.Fatal("expected actionBusy cleared")
	}
	if StatusKindForTest(m) != "ok" {
		t.Fatalf("kind=%s", StatusKindForTest(m))
	}
	if StatusTextForTest(m) != "turn completed" {
		t.Fatalf("text=%q", StatusTextForTest(m))
	}
}

func TestCompleteBusyExecuteStatusNoopWhenIdle(t *testing.T) {
	m := NewTestModel(nil)
	m = m.setStatusResult(true, "/hero-sync", "done")
	next := m.completeBusyExecuteStatus(true, "turn completed")
	if StatusTextForTest(next) != "done" {
		t.Fatalf("must not overwrite idle result: %q", StatusTextForTest(next))
	}
}

func TestBusyExecuteCompletedText(t *testing.T) {
	if got := busyExecuteCompletedText("/hero-start"); got != "orchestration turn completed" {
		t.Fatalf("start text=%q", got)
	}
	if got := busyExecuteCompletedText("/hero-resume"); got != "turn completed" {
		t.Fatalf("resume text=%q", got)
	}
}

func TestStatusBarReadyAndRunning(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	view := ViewForTest(m)
	if !strings.Contains(view, "ready") {
		t.Fatalf("expected idle ready: %q", view)
	}

	m = m.setStatusRunning("/hero-sync")
	view = ViewForTest(m)
	if StatusKindForTest(m) != "running" || !ActionBusyForTest(m) {
		t.Fatalf("kind=%s busy=%v", StatusKindForTest(m), ActionBusyForTest(m))
	}
	if !strings.Contains(view, "/hero-sync") || !strings.Contains(view, "running") {
		t.Fatalf("expected running bar: %q", view)
	}
}

func TestStatusBarWrapsLongError(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 40)
	m = SetHeight(m, 24)
	long := "Dispatch unavailable; authenticate with `cursor agent login`; run /hero-sync in Cursor chat"
	next, _ := ApplyActionResultForTest(m, ActionResultForTest{
		title: "/hero-sync",
		err:   errString(long),
	})
	view := ViewForTest(next)
	if !strings.Contains(view, "✗") && !strings.Contains(view, "/hero-sync") {
		t.Fatalf("expected error status: %q", view)
	}
	// Must not dump a single unbroken super-long line into the frame without wrap markers.
	for _, line := range strings.Split(view, "\n") {
		// Allow ANSI; strip roughly by rune length of visible content being huge is ok if wrapped.
		if len([]rune(line)) > 120 && strings.Contains(line, "Dispatch unavailable") {
			t.Fatalf("unwrapped long status line: %q", line)
		}
	}
}

func TestPaletteSyncOpensConversation(t *testing.T) {
	dir := t.TempDir()
	setupHeroApproveRuntimeFiles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "commands", "hero-sync.md"), []byte("# /hero-sync\n\nSYNC_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestServiceInDir(t, dir)
	h := &streamingHarness{deltas: []string{"syncing"}}
	svc.Harness = h

	m := NewTestModel(svc)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = OpenPalette(m)
	m = SetPaletteFilter(m, "hero-sync")
	items := FilteredPalette(m)
	found := false
	for _, it := range items {
		if it.Label == "/hero-sync" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("filter missed /hero-sync: %+v", items)
	}
	next, cmd := RunPaletteItemForTest(m, "/hero-sync")
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if CurrentScreen(next) == ScreenPalette {
		t.Fatal("palette should close")
	}
	if CurrentScreen(next) != ScreenConversation {
		t.Fatalf("screen=%v want conversation", CurrentScreen(next))
	}
	if !IsConversationStreaming(next) {
		t.Fatal("expected streaming after /hero-sync")
	}
	next = drainConversationStream(t, next, cmd)
	if !strings.Contains(h.lastPrompt, "SYNC_MARKER") {
		t.Fatalf("missing sync command: %q", h.lastPrompt)
	}
	// Sync runs inline as a freechat turn: no orchestration agent body or identity.
	if strings.Contains(h.lastPrompt, "ORCH_SYNC") {
		t.Fatalf("sync prompt must not include orchestration agent: %q", h.lastPrompt)
	}
	if h.lastAgentName != "" {
		t.Fatalf("agent=%q want freechat (empty)", h.lastAgentName)
	}
}

func TestBusyGuardBlocksSecondAction(t *testing.T) {
	svc := newTestService(t)
	m := NewTestModel(svc)
	m = m.setStatusRunning("/hero-sync")
	next, cmd := RunPaletteItemForTest(m, "/hero-status")
	if cmd != nil {
		t.Fatal("expected no cmd while busy")
	}
	if !strings.Contains(ViewForTest(next), "busy") {
		t.Fatalf("expected busy message: %q", ViewForTest(next))
	}
}

func TestStatusBarWrapsQuestionUpToSixLines(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 40)
	m = SetHeight(m, 24)
	m.harnessQuestionPending = true
	m.harnessQuestionMsg = "Harness question: Qual é a melhor estratégia para migrar o banco?\nLinha longa que certamente vai quebrar em várias linhas do terminal estreito para testar wrap.\n  1) Opção A — descrição longa que também quebra\n  2) Opção B\nType option number or text, then Alt+Enter. Esc rejects."
	lines := m.statusBarDisplayLines(40)
	if len(lines) < 3 {
		t.Fatalf("expected wrapped question >2 lines, got %d: %q", len(lines), lines)
	}
	if len(lines) > 6 {
		t.Fatalf("expected max 6 lines, got %d", len(lines))
	}
	if got := m.statusBarLineCount(); got != len(lines) {
		t.Fatalf("lineCount=%d display=%d", got, len(lines))
	}
	view := ViewForTest(m)
	for _, line := range lines {
		_ = line
	}
	if view == "" {
		t.Fatal("expected view")
	}
}

func TestStatusBarScrollWithAlt(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 30)
	m = SetHeight(m, 24)
	var b strings.Builder
	for i := 1; i <= 20; i++ {
		b.WriteString("linha longa de teste para quebra automática número ")
		b.WriteString(strings.TrimSpace(strings.Repeat("x", 0)) + strconv.Itoa(i))
		b.WriteByte('\n')
	}
	m.harnessQuestionPending = true
	m.harnessQuestionMsg = b.String()
	total := len(m.statusBarAllLines(30))
	if total <= 6 {
		t.Fatalf("expected overflow fixture, total=%d", total)
	}
	if m.statusBarLineCount() != 6 {
		t.Fatalf("lineCount=%d want 6", m.statusBarLineCount())
	}
	next, _ := HandleTestKey(m, "alt+down")
	if next.statusScrollOffset != 1 {
		t.Fatalf("offset=%d want 1", next.statusScrollOffset)
	}
	// ↑↓ do transcript não pode mover a status bar.
	plain, _ := HandleTestKey(m, "down")
	if plain.statusScrollOffset != 0 {
		t.Fatalf("plain down moved status offset=%d", plain.statusScrollOffset)
	}
	next, _ = HandleTestKey(next, "alt+end")
	if next.statusScrollOffset != total-6 {
		t.Fatalf("end offset=%d want %d", next.statusScrollOffset, total-6)
	}
	next, _ = HandleTestKey(next, "alt+home")
	if next.statusScrollOffset != 0 {
		t.Fatalf("home offset=%d want 0", next.statusScrollOffset)
	}
	// Indicador de scroll sem consumir linha extra.
	lines := next.statusBarDisplayLines(30)
	_ = lines
	m2 := m
	m2.statusScrollOffset = 0
	m2 = m2.clampStatusScroll(30)
	disp := m2.statusBarDisplayLines(30)
	found := false
	for _, line := range disp {
		if strings.Contains(line, "Alt+") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected scroll hint Alt+ in %q", disp)
	}
}

func TestStatusBarReturnsToNormalAfterQuestion(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m.harnessQuestionPending = true
	m.harnessQuestionMsg = "q1\nq2\nq3\nq4\nq5\nq6\nq7\nq8"
	m.statusScrollOffset = 2
	m = m.clearHarnessQuestionState()
	if m.statusBarLineCount() != 2 {
		t.Fatalf("lineCount=%d want 2 after clear", m.statusBarLineCount())
	}
	if m.statusScrollOffset != 0 {
		t.Fatalf("offset=%d want 0 after clear", m.statusScrollOffset)
	}
}

func TestStatusBarCapsByWindowHeight(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 40)
	m = SetHeight(m, 12)
	m.harnessQuestionPending = true
	m.harnessQuestionMsg = "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl"
	if got := m.statusBarLineCount(); got > 6 {
		t.Fatalf("lineCount=%d want <=6", got)
	}
	// Altura mínima ainda preserva o normal de 2 linhas.
	m = SetHeight(m, 8)
	if got := m.statusBarLineCount(); got < 2 {
		t.Fatalf("lineCount=%d want >=2", got)
	}
}

func TestFormatElapsed(t *testing.T) {
	if formatElapsed(0) != "00:00:00" {
		t.Fatal(formatElapsed(0))
	}
	if formatElapsed(5*time.Second) != "00:00:05" {
		t.Fatal(formatElapsed(5 * time.Second))
	}
	if formatElapsed(65*time.Second) != "00:01:05" {
		t.Fatal(formatElapsed(65 * time.Second))
	}
	if formatElapsed(24*time.Hour+time.Second) != "24:00:01" {
		t.Fatal(formatElapsed(24*time.Hour + time.Second))
	}
}

type errString string

func (e errString) Error() string { return string(e) }
