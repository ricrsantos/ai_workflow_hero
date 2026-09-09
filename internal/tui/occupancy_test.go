package tui

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestOccupancyKeyForFreechatAndCycle(t *testing.T) {
	if occupancyKeyFor(convExecute{Freechat: true}, true) != occupancyKeyFreechat {
		t.Fatal("freechat execute")
	}
	got := occupancyKeyFor(convExecute{StageName: "qa", AgentName: "qa_agent"}, true)
	want := cycleOccupancyKey("qa", "qa_agent")
	if got != want {
		t.Fatalf("cycle key=%q want %q", got, want)
	}
	if occupancyKeyFor(convExecute{}, false) != occupancyKeyFreechat {
		t.Fatal("untagged execute is freechat occupancy")
	}
}

func TestApplyContextOccupancyAssignsPerKey(t *testing.T) {
	m := NewTestModel(nil)
	m = m.applyContextOccupancy(occupancyKeyFreechat, harness.Usage{InputTokens: 10, OutputTokens: 2}, "", "")
	cycleKey := cycleOccupancyKey("research", "discover_agent")
	m = m.applyContextOccupancy(cycleKey, harness.Usage{InputTokens: 80, OutputTokens: 20}, "", "")
	if m.contextUsedTokens != 100 {
		t.Fatalf("display=%d want cycle 100", m.contextUsedTokens)
	}
	m = m.showOccupancyKey(occupancyKeyFreechat)
	if m.contextUsedTokens != 12 {
		t.Fatalf("freechat display=%d want 12", m.contextUsedTokens)
	}
}
