package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestEmptyCycleScreenMessage(t *testing.T) {
	if got := emptyCycleScreenMessage("artifacts", 0); got != "No active cycle. Run /hero-new to start." {
		t.Fatalf("got %q", got)
	}
	if got := emptyCycleScreenMessage("metrics", 3); got != "No metrics for cycle C3." {
		t.Fatalf("got %q", got)
	}
}

func TestEmptyArtifactsCostsEventsNoC0(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m.artifacts = cycle.ArtifactsView{}
	m.metrics = cycle.MetricsView{}
	m.events = cycle.EventsView{}

	for _, screen := range []screen{ScreenArtifacts, ScreenCosts, ScreenEvents} {
		m = SetScreen(m, screen)
		view := ViewForTest(m)
		if strings.Contains(view, "C0") {
			t.Fatalf("screen %v must not mention C0: %q", screen, view)
		}
		if !strings.Contains(view, "No active cycle") {
			t.Fatalf("screen %v expected no-active-cycle message: %q", screen, view)
		}
	}
}

func TestChatFooterUsesRealBindingsAndIncludesAllHints(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = EnterConversationForTest(m)

	want := "tab mode · / commands · enter newline · alt+enter send · alt+r/i copy · ↑↓ scroll · alt+q quit"
	for _, state := range []struct {
		screen    screen
		streaming bool
		overlay   bool
	}{
		{screen: screenConversation},
		{screen: screenConversation, streaming: true},
		{screen: screenConversation, overlay: true},
		{screen: screenPalette},
		{screen: screenOutput},
		{screen: screenStatus},
	} {
		m.screen = state.screen
		m.streaming = state.streaming
		m.slashOverlayDismissed = !state.overlay
		if got := m.footerHints(); got != want {
			t.Fatalf("footer for screen=%v streaming=%t overlay=%t = %q, want %q", state.screen, state.streaming, state.overlay, got, want)
		}
	}
}

func TestFooterWrapsWithoutClippingAndReservesRows(t *testing.T) {
	const width, height = 40, 24
	m := NewTestModel(nil)
	m = SetWidth(m, width)
	m = SetHeight(m, height)
	m = EnterConversationForTest(m)

	footerLines := m.footerHintLines()
	if len(footerLines) < 2 {
		t.Fatalf("expected narrow footer to wrap, got %v", footerLines)
	}
	for _, line := range footerLines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("footer line width=%d want <= %d: %q", got, width, line)
		}
	}

	viewLines := strings.Split(stripANSI(ViewForTest(m)), "\n")
	if got := len(viewLines); got != height {
		t.Fatalf("frame lines=%d want %d", got, height)
	}
	footerStart := len(viewLines) - len(footerLines)
	for i, want := range footerLines {
		if got := viewLines[footerStart+i]; got != want {
			t.Fatalf("footer line %d=%q want %q\n%s", i, got, want, strings.Join(viewLines, "\n"))
		}
	}
}

func TestFooterRemainsVisibleWhenContentAreaIsShort(t *testing.T) {
	const width, height = 80, 10
	m := SetHeight(SetWidth(NewTestModel(nil), width), height)
	m = EnterConversationForTest(m)

	viewLines := strings.Split(stripANSI(ViewForTest(m)), "\n")
	if got := len(viewLines); got != height {
		t.Fatalf("frame lines=%d want %d\n%s", got, height, strings.Join(viewLines, "\n"))
	}
	footerLines := m.footerHintLines()
	if len(viewLines) < len(footerLines) {
		t.Fatalf("frame has fewer lines than footer: %d < %d", len(viewLines), len(footerLines))
	}
	start := len(viewLines) - len(footerLines)
	for i, want := range footerLines {
		if got := viewLines[start+i]; got != want {
			t.Fatalf("footer line %d=%q want %q\n%s", i, got, want, strings.Join(viewLines, "\n"))
		}
	}
	if got := strings.Join(viewLines[start:], " · "); got != fixedFooterHints {
		t.Fatalf("footer=%q want %q", got, fixedFooterHints)
	}
}

func TestFormatEventTimeLocal(t *testing.T) {
	ts := "2025-08-13T23:44:32Z"
	want, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatal(err)
	}
	if got := formatEventTimeLocal(ts); got != want.Local().Format("15:04:05") {
		t.Fatalf("got %q want %q", got, want.Local().Format("15:04:05"))
	}
	if got := formatEventTimeLocal("not-a-timestamp"); got != "not-a-timestamp" {
		t.Fatalf("fallback got %q", got)
	}
}

func TestFormatDurationMMSS(t *testing.T) {
	cases := map[int64]string{
		0:       "—",
		120_000: "02:00",
		540_000: "09:00",
		960_000: "16:00",
		96_000:  "01:36",
		900_000: "15:00",
		45_000:  "00:45",
		20_000:  "00:20",
	}
	for ms, want := range cases {
		if got := formatDurationMMSS(ms); got != want {
			t.Fatalf("ms=%d got %q want %q", ms, got, want)
		}
	}
}

func TestRenderCostsFormatted(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 120)
	m = SetHeight(m, 30)
	m.metrics = cycle.MetricsView{
		CycleNumber: 1,
		Title:       "Implementação inicial",
		Rows: []cycle.MetricsRow{
			{Stage: "research", Agent: "discover_agent", Model: "composer-2.5", InputTokens: 4500, OutputTokens: 8750, CostUSD: 0.024, DurationMS: 540_000},
			{Stage: "implementation", Agent: "frontend_agent", Model: "cursor-grok-4.6-high", InputTokens: 25000, OutputTokens: 14200, CostUSD: 0.18, DurationMS: 900_000},
		},
		TotalIn: 29500, TotalOut: 22950, TotalCost: 0.204,
	}
	m = SetScreen(m, ScreenCosts)
	view := ViewForTest(m)

	for _, want := range []string{"Stage", "Agent", "Model", "09:00", "15:00", "$0.0240", "$0.1800", "cursor-grok-4.6-high"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in view:\n%s", want, view)
		}
	}
	if strings.Contains(view, "ms") {
		t.Fatalf("duration must not show ms: %q", view)
	}
}

func TestRenderEventsLocalTime(t *testing.T) {
	ts := "2025-08-13T23:44:32Z"
	want, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(nil)
	m = SetWidth(m, 80)
	m = SetHeight(m, 24)
	m.events = cycle.EventsView{
		CycleNumber: 1,
		Events: []store.Event{
			{TS: ts, Type: "cycle_created", PayloadJSON: `{"number":1}`},
		},
	}
	m = SetScreen(m, ScreenEvents)
	view := ViewForTest(m)
	if !strings.Contains(view, want.Local().Format("15:04:05")) {
		t.Fatalf("expected local time in view:\n%s", view)
	}
	if strings.Contains(view, "23:44:32") && want.Local().Format("15:04:05") != "23:44:32" {
		t.Fatalf("must not show UTC time when local differs:\n%s", view)
	}
}

func TestRenderArtifactsTableAndScroll(t *testing.T) {
	m := NewTestModel(nil)
	m = SetWidth(m, 100)
	m = SetHeight(m, 16)
	arts := make([]store.Artifact, 40)
	for i := range arts {
		arts[i] = store.Artifact{
			Kind:      "prd",
			Label:     fmt.Sprintf("Doc %02d", i),
			Path:      fmt.Sprintf("docs/product/file-%02d.md", i),
			CreatedAt: "2025-08-13T23:44:32Z",
		}
	}
	m.artifacts = cycle.ArtifactsView{CycleNumber: 4, Artifacts: arts}
	m = SetScreen(m, ScreenArtifacts)
	view := ViewForTest(m)
	if !strings.Contains(view, "Artifacts — C4") {
		t.Fatalf("expected header: %q", view)
	}
	if !strings.Contains(view, "file-00.md") {
		t.Fatalf("expected first artifact: %q", view)
	}
	if strings.Contains(view, "file-39.md") {
		t.Fatalf("last artifact should be scrolled out: %q", view)
	}

	scrolled, _ := HandleTestKey(m, "end")
	if ContentOffsetForTest(scrolled) == 0 {
		t.Fatal("expected content offset after end")
	}
	scrolledView := ViewForTest(scrolled)
	if !strings.Contains(scrolledView, "file-39.md") && !strings.Contains(scrolledView, "Doc 39") {
		t.Fatalf("expected later artifact after scroll: %q", scrolledView)
	}
}
