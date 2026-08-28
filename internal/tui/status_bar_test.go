package tui

import (
	"os"
	"path/filepath"
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
	if err := os.WriteFile(filepath.Join(dir, ".cursor", "agents", "orchestration_agent.md"), []byte("---\nname: orchestration_agent\n---\n\nORCH_SYNC"), 0o644); err != nil {
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
	if !strings.Contains(h.lastPrompt, "ORCH_SYNC") {
		t.Fatalf("missing orchestration agent: %q", h.lastPrompt)
	}
	if h.lastAgentName != "orchestration_agent" {
		t.Fatalf("agent=%q", h.lastAgentName)
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
