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
