package harness_test

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestEstimateUsageCharsDiv4(t *testing.T) {
	// 8 runes → 2 tokens; 10 runes → 3 tokens (round)
	got := harness.EstimateUsage("abcdefgh", "abcdefghij")
	if got.InputTokens != 2 || got.OutputTokens != 3 {
		t.Fatalf("usage=%+v want in=2 out=3", got)
	}
	if got.ContextTokens != 5 || got.Occupancy() != 5 {
		t.Fatalf("occupancy=%d context=%d want 5", got.Occupancy(), got.ContextTokens)
	}
}

func TestUsageOccupancyPrefersContextTokens(t *testing.T) {
	u := harness.Usage{InputTokens: 10, OutputTokens: 5, CacheReadTokens: 100, ContextTokens: 40}
	if u.Occupancy() != 40 {
		t.Fatalf("occupancy=%d want 40", u.Occupancy())
	}
}

func TestUsageOccupancyDoesNotReconstructFromBilled(t *testing.T) {
	u := harness.Usage{InputTokens: 10, OutputTokens: 5, CacheReadTokens: 100, CacheWriteTokens: 20}
	if u.Occupancy() != 0 {
		t.Fatalf("occupancy=%d want 0; billed aggregate is not window fill", u.Occupancy())
	}
	if u.CallOccupancy() != 135 {
		t.Fatalf("call occupancy=%d want 135 exclusive cache", u.CallOccupancy())
	}
}

func TestCallOccupancyInclusiveCacheDoesNotDoubleCount(t *testing.T) {
	u := harness.Usage{InputTokens: 100, OutputTokens: 4, CacheReadTokens: 80, CacheWriteTokens: 0}
	if u.PromptTokens() != 100 {
		t.Fatalf("prompt=%d want 100 (input already includes cache)", u.PromptTokens())
	}
	if u.CallOccupancy() != 104 {
		t.Fatalf("call occupancy=%d want 104", u.CallOccupancy())
	}
}

func TestResolveUsagePrefersHarnessWithoutInventingOccupancy(t *testing.T) {
	got := harness.ResolveUsage(
		harness.Usage{InputTokens: 100, OutputTokens: 50},
		"ignored",
		"ignored",
	)
	if got.InputTokens != 100 || got.OutputTokens != 50 {
		t.Fatalf("usage=%+v", got)
	}
	if got.ContextTokens != 0 || got.Occupancy() != 0 {
		t.Fatalf("billed usage must not invent occupancy: %+v", got)
	}
}

func TestResolveUsageFallsBackWhenZero(t *testing.T) {
	got := harness.ResolveUsage(harness.Usage{}, "abcd", "abcdefgh")
	if got.InputTokens != 1 || got.OutputTokens != 2 {
		t.Fatalf("usage=%+v want in=1 out=2", got)
	}
	if got.ContextTokens != 3 {
		t.Fatalf("estimate occupancy=%d want 3", got.ContextTokens)
	}
}
