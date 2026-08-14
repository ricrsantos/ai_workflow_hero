package tui

import (
	"strings"
	"testing"
	"time"

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
