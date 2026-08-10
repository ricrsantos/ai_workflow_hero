package tui

import (
	"strings"
	"testing"
)

func TestSplitOutputLinesWraps(t *testing.T) {
	lines := splitOutputLines("abcdefghijklmnop\nxy", 8)
	if len(lines) < 3 {
		t.Fatalf("lines=%v", lines)
	}
	if lines[0] != "abcdefgh" || lines[1] != "ijklmnop" {
		t.Fatalf("wrap=%v", lines)
	}
	if lines[2] != "xy" {
		t.Fatalf("newline para=%v", lines)
	}
}

func TestShouldOpenOutputPanel(t *testing.T) {
	if shouldOpenOutputPanel("ok", 80) {
		t.Fatal("short one-liner should stay flash")
	}
	if !shouldOpenOutputPanel("a\nb\nc\nd", 80) {
		t.Fatal("multiline should open panel")
	}
	long := strings.Repeat("x", 200)
	if !shouldOpenOutputPanel(long, 40) {
		t.Fatal("wrapped long line should open panel")
	}
}

func TestOutputPanelScrollAndEsc(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 14)
	m = SetScreen(m, ScreenStatus)
	m = OpenPalette(m)
	if PrevScreenForTest(m) != ScreenStatus {
		t.Fatalf("prev=%v", PrevScreenForTest(m))
	}

	var body strings.Builder
	for i := 0; i < 40; i++ {
		body.WriteString("cycle line ")
		body.WriteByte(byte('A' + (i % 26)))
		body.WriteByte('\n')
	}
	next, _ := ApplyActionResultForTest(m, ActionResultForTest{
		success: body.String(),
		title:   "/hero-cycles",
	})
	if CurrentScreen(next) != ScreenOutput {
		t.Fatalf("screen=%v want Output", CurrentScreen(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "/hero-cycles") {
		t.Fatalf("missing title: %q", view)
	}
	if !strings.Contains(view, "more below") {
		t.Fatalf("expected scroll hint: %q", view)
	}
	if strings.Contains(view, "cycle line Z") && strings.Contains(view, "cycle line A") {
		// Z is near end; with short height A visible and Z should be below
	}
	if strings.Contains(view, "cycle line Z") {
		t.Fatalf("last lines should be scrolled out initially: %q", view)
	}

	scrolled := next
	for i := 0; i < 8; i++ {
		scrolled, _ = HandleTestKey(scrolled, "down")
	}
	if OutputOffsetForTest(scrolled) == 0 {
		t.Fatal("expected outputOffset > 0 after down")
	}
	if !strings.Contains(ViewForTest(scrolled), "more above") {
		t.Fatalf("expected more-above: %q", ViewForTest(scrolled))
	}

	closed, _ := HandleTestKey(scrolled, "esc")
	if CurrentScreen(closed) != ScreenStatus {
		t.Fatalf("esc should restore Status, got %v", CurrentScreen(closed))
	}
}

func TestStyleOutputLineUsesSemanticColors(t *testing.T) {
	if infoStyle.Render("x") == "x" {
		t.Skip("lipgloss colors disabled (NO_COLOR / non-TTY)")
	}
	cycle := styleOutputLine("C3 — title [active]", false)
	if !strings.Contains(cycle, "C3") || cycle == "C3 — title [active]" {
		t.Fatalf("cycle header should be styled, got %q", cycle)
	}
	arrow := styleOutputLine("→ Cycles (1 total)", false)
	if arrow == "→ Cycles (1 total)" {
		t.Fatal("progress line should be styled")
	}
	errLine := styleOutputLine("boom", true)
	if errLine == "boom" {
		t.Fatal("error body should be styled")
	}
}

func TestShortActionResultStaysFlash(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	next, _ := ApplyActionResultForTest(m, ActionResultForTest{success: "Stage approved."})
	if CurrentScreen(next) == ScreenOutput {
		t.Fatal("short success must not open output panel")
	}
	if !strings.Contains(ViewForTest(next), "Stage approved.") {
		t.Fatalf("expected flash: %q", ViewForTest(next))
	}
}
