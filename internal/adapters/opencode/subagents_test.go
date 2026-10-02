package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// Recorded from `opencode serve` /event SSE: the parent session runs two task
// subagents in parallel. Task parts arrive as message.part.updated (pending →
// running → completed); each subagent must start once and complete once.
func TestOpenCodeSubagentLifecycleFromRecordedSSE(t *testing.T) {
	f, err := os.Open("testdata/subagents_sse_serve.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var events []map[string]any
	parent := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<22)
	for sc.Scan() {
		var evt map[string]any
		if err := json.Unmarshal(sc.Bytes(), &evt); err != nil {
			t.Fatal(err)
		}
		if props, _ := evt["properties"].(map[string]any); parent == "" && evt["type"] == "session.created" {
			info, _ := props["info"].(map[string]any)
			parent, _ = info["id"].(string)
		}
		events = append(events, evt)
	}
	if parent == "" {
		t.Fatal("fixture has no parent session")
	}
	a := &Adapter{ProjectDir: t.TempDir()}
	state := newStreamState()
	var got []harness.StreamDelta
	req := harness.ExecuteRequest{OnStreamDelta: func(d harness.StreamDelta) {
		if d.Subagent {
			got = append(got, d)
		}
	}}
	for _, evt := range events {
		a.processSSEEvent(context.Background(), evt, parent, state, req, nil)
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
