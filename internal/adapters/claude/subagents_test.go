package claude

import (
	"bufio"
	"os"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// Recorded with Claude Code 2.1.287 (stream-json): two general-purpose Agent
// subagents run in parallel. Each must start once with its description as the
// name and complete once; task_updated and task_notification both report the
// end, but only the first closes the lifecycle.
func TestClaudeSubagentLifecycleFromRecordedStream(t *testing.T) {
	f, err := os.Open("testdata/subagents_stream_2.1.287.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	a := newResultAssembler(false)
	var got []harness.StreamDelta
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<22)
	line := 0
	for sc.Scan() {
		line++
		ev, err := decodeRawEvent(line, sc.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		deltas, _ := a.consume(ev)
		for _, d := range deltas {
			if d.Subagent {
				got = append(got, d)
			}
		}
	}
	if len(got) != 4 {
		t.Fatalf("subagent deltas=%d want 4: %+v", len(got), got)
	}
	started := map[string]string{}
	for _, d := range got {
		switch d.Phase {
		case harness.StreamPhaseStarted:
			started[d.CallID] = d.AgentName
		case harness.StreamPhaseCompleted:
			if _, ok := started[d.CallID]; !ok {
				t.Fatalf("completion before start: %+v", d)
			}
		}
	}
	names := map[string]bool{}
	for _, n := range started {
		names[n] = true
	}
	if !names["general-purpose"] || len(started) != 2 {
		t.Fatalf("started=%v", started)
	}
}

func TestClaudeBackgroundShellTaskIsNotSubagent(t *testing.T) {
	a := newResultAssembler(false)
	for i, raw := range []string{
		`{"type":"system","subtype":"task_started","task_id":"b1","description":"npm test","task_type":"local_bash"}`,
		`{"type":"system","subtype":"task_notification","task_id":"b1","status":"completed"}`,
	} {
		ev, err := decodeRawEvent(i+1, []byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		deltas, _ := a.consume(ev)
		for _, d := range deltas {
			if d.Subagent {
				t.Fatalf("background shell flagged as subagent: %+v", d)
			}
		}
	}
}
