package tui

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
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
