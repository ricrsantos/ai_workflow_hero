package cursor_test

import (
	"os"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// Recorded with cursor-agent (model auto): a getMcpTools lookup for "Task"
// precedes two parallel taskToolCall subagents. Only the two Task calls are
// subagents, each started once and completed once.
func TestCursorSubagentLifecycleFromRecordedStream(t *testing.T) {
	f, err := os.Open("testdata/subagents_stream_auto.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var got []harness.StreamDelta
	if _, err := cursor.ParseStreamJSON(f, func(d harness.StreamDelta) {
		if d.Subagent {
			got = append(got, d)
		}
	}); err != nil {
		t.Fatal(err)
	}
	phases := map[string][]string{}
	names := map[string]string{}
	for _, d := range got {
		phases[d.CallID] = append(phases[d.CallID], d.Phase)
		if d.Phase == harness.StreamPhaseStarted {
			names[d.CallID] = d.AgentName
		}
	}
	if len(phases) != 2 {
		t.Fatalf("subagents=%d want 2: %+v", len(phases), got)
	}
	for id, p := range phases {
		if len(p) != 2 || p[0] != harness.StreamPhaseStarted || p[1] != harness.StreamPhaseCompleted {
			t.Fatalf("agent %s phases=%v", id, p)
		}
		if names[id] != "alpha check" && names[id] != "beta check" {
			t.Fatalf("agent %s name=%q", id, names[id])
		}
	}
}
