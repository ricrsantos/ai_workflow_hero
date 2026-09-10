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
	m = m.applyContextOccupancy(occupancyKeyFreechat, harness.Usage{ContextTokens: 12}, "", "")
	cycleKey := cycleOccupancyKey("research", "discover_agent")
	m = m.applyContextOccupancy(cycleKey, harness.Usage{ContextTokens: 100}, "", "")
	if m.contextUsedTokens != 100 {
		t.Fatalf("display=%d want cycle 100", m.contextUsedTokens)
	}
	m = m.showOccupancyKey(occupancyKeyFreechat)
	if m.contextUsedTokens != 12 {
		t.Fatalf("freechat display=%d want 12", m.contextUsedTokens)
	}
}

func TestApplyContextOccupancyIgnoresBilledAggregate(t *testing.T) {
	m := NewTestModel(nil)
	m.transcript = []convMessage{
		{role: convRoleUser, content: "abcd", occupancyKey: occupancyKeyFreechat},      // 4 runes → 1
		{role: convRoleAgent, content: "abcdefgh", occupancyKey: occupancyKeyFreechat}, // 8 runes → 2
	}
	m = m.applyContextOccupancy(occupancyKeyFreechat, harness.Usage{InputTokens: 80000, OutputTokens: 1000, CacheReadTokens: 70000}, "ignored", "ignored")
	if m.contextUsedTokens != 3 {
		t.Fatalf("used=%d want transcript 3, not billed aggregate", m.contextUsedTokens)
	}
}
