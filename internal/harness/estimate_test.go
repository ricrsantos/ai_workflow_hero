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

func TestUsageOccupancyReconstructsFromCache(t *testing.T) {
	u := harness.Usage{InputTokens: 10, OutputTokens: 5, CacheReadTokens: 100, CacheWriteTokens: 20}
	if u.Occupancy() != 135 {
		t.Fatalf("occupancy=%d want 135", u.Occupancy())
	}
}

func TestResolveUsagePrefersHarness(t *testing.T) {
	got := harness.ResolveUsage(
		harness.Usage{InputTokens: 100, OutputTokens: 50},
		"ignored",
		"ignored",
	)
	if got.InputTokens != 100 || got.OutputTokens != 50 {
		t.Fatalf("usage=%+v", got)
	}
}

func TestResolveUsageFallsBackWhenZero(t *testing.T) {
	got := harness.ResolveUsage(harness.Usage{}, "abcd", "abcdefgh")
	if got.InputTokens != 1 || got.OutputTokens != 2 {
		t.Fatalf("usage=%+v want in=1 out=2", got)
	}
}
