package tui

import (
	"strings"
	"testing"
	"time"
)

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

func TestPaletteSyncClosesAndSetsRunning(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceInDir(t, dir)
	svc.Harness = &recordingHarness{}

	m := NewTestModel(svc)
	m = OpenPalette(m)
	m = SetPaletteFilter(m, "hero-sync")
	// Select /hero-sync if filtered list has it.
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
	if StatusKindForTest(next) != "running" {
		t.Fatalf("expected running, got %s", StatusKindForTest(next))
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
	if formatElapsed(0) != "0s" {
		t.Fatal(formatElapsed(0))
	}
	if formatElapsed(5*time.Second) != "5s" {
		t.Fatal(formatElapsed(5 * time.Second))
	}
	if formatElapsed(65*time.Second) != "1m05s" {
		t.Fatal(formatElapsed(65 * time.Second))
	}
}

type errString string

func (e errString) Error() string { return string(e) }
