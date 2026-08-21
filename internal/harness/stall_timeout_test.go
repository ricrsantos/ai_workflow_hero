package harness_test

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestStallTimeoutForHarness_Codex(t *testing.T) {
	if got := harness.StallTimeoutForHarness("codex"); got != harness.CodexStallTimeout {
		t.Fatalf("codex stall = %v, want %v", got, harness.CodexStallTimeout)
	}
	if harness.CodexStallTimeout != harness.OpenCodeStallTimeout {
		t.Fatalf("codex stall should match opencode analog (%v vs %v)", harness.CodexStallTimeout, harness.OpenCodeStallTimeout)
	}
	if got := harness.StallTimeoutForHarness("cursor"); got != harness.CursorStallTimeout {
		t.Fatalf("cursor stall = %v", got)
	}
}
