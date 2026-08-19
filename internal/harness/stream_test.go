package harness_test

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestWarningDeltaFormat(t *testing.T) {
	d := harness.WarningDelta("opencode", "future.event", "sess-1", `{"foo":1}`)
	if d.Kind != harness.StreamKindWarning {
		t.Fatalf("kind=%q", d.Kind)
	}
	for _, want := range []string{"WARNING Harness event not recognized", "harness: opencode", "event: future.event", "session: sess-1"} {
		if !strings.Contains(d.Text, want) {
			t.Fatalf("text missing %q: %s", want, d.Text)
		}
	}
}
