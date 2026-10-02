package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// replayAppServerFixture feeds recorded app-server notifications through the
// adapter for one parent thread and returns the subagent lifecycle deltas.
func replayAppServerFixture(t *testing.T, path string) (parent string, subagents []harness.StreamDelta) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	req := harness.ExecuteRequest{OnStreamDelta: func(d harness.StreamDelta) {
		if d.Subagent {
			subagents = append(subagents, d)
		}
	}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<22)
	for sc.Scan() {
		var msg struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(sc.Bytes(), &msg); err != nil {
			t.Fatal(err)
		}
		if parent == "" && msg.Method == "thread/started" {
			var p struct {
				Thread struct {
					ID string `json:"id"`
				} `json:"thread"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			parent = p.Thread.ID
		}
		if parent == "" {
			continue
		}
		a.handleNotification(context.Background(), msg.Method, msg.Params, parent, req, nil, st)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return parent, subagents
}

// Recorded with codex-cli 0.159.2: the parent spawns alpha and beta in
// parallel, waits, and closes them. Each agent must start once and complete
// once, keyed by its own thread id.
func TestCodexSubagentLifecycleFromRecordedAppServer(t *testing.T) {
	parent, got := replayAppServerFixture(t, "testdata/subagents_app_server_0.159.2.jsonl")
	if parent == "" {
		t.Fatal("fixture has no parent thread")
	}
	type edge struct{ name, phase string }
	byCall := map[string][]edge{}
	order := []string{}
	for _, d := range got {
		if _, seen := byCall[d.CallID]; !seen {
			order = append(order, d.CallID)
		}
		byCall[d.CallID] = append(byCall[d.CallID], edge{d.AgentName, d.Phase})
	}
	if len(order) != 2 {
		t.Fatalf("subagents=%d want 2: %+v", len(order), got)
	}
	wantNames := []string{"alpha", "beta"}
	for i, id := range order {
		edges := byCall[id]
		if len(edges) != 2 || edges[0].phase != harness.StreamPhaseStarted || edges[1].phase != harness.StreamPhaseCompleted {
			t.Fatalf("agent %s edges=%+v want started then completed", id, edges)
		}
		if edges[0].name != wantNames[i] {
			t.Fatalf("agent %s name=%q want %q", id, edges[0].name, wantNames[i])
		}
	}
}

func TestCodexCollabSpawnFallbackAndTerminalStates(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	var got []harness.StreamDelta
	req := harness.ExecuteRequest{OnStreamDelta: func(d harness.StreamDelta) {
		if d.Subagent {
			got = append(got, d)
		}
	}}
	send := func(item map[string]any) {
		payload, _ := json.Marshal(map[string]any{"threadId": "thr", "item": item})
		a.handleNotification(context.Background(), "item/completed", payload, "thr", req, nil, newTurnStreamState())
	}
	send(map[string]any{"type": "collabAgentToolCall", "id": "c1", "tool": "spawnAgent", "model": "gpt-6-luna",
		"receiverThreadIds": []any{"child-1"}, "agentsStates": map[string]any{}})
	send(map[string]any{"type": "collabAgentToolCall", "id": "c2", "tool": "wait", "receiverThreadIds": []any{},
		"agentsStates": map[string]any{"child-1": map[string]any{"status": "completed"}}})
	if len(got) != 2 || got[0].Phase != harness.StreamPhaseStarted || got[1].Phase != harness.StreamPhaseCompleted ||
		got[0].CallID != got[1].CallID || got[0].Model != "gpt-6-luna" {
		t.Fatalf("got=%+v", got)
	}
}

func TestCodexSubagentName(t *testing.T) {
	cases := map[string]string{"/root/alpha": "alpha", "/root/qa_agent": "qa_agent", "/root": "task", "": "task"}
	for in, want := range cases {
		if got := codexSubagentName(in); got != want {
			t.Errorf("%q -> %q want %q", in, got, want)
		}
	}
}
