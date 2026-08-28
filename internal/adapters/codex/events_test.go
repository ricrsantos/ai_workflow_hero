package codex

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestAgentMessageDeltaPreservesSpaces(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	for _, delta := range []string{"Hello ", " ", "world"} {
		payload, _ := json.Marshal(map[string]any{
			"threadId": "thr", "itemId": "m1", "delta": delta,
		})
		out := a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, nil, st)
		if out.err != nil {
			t.Fatal(out.err)
		}
	}
	if text != "Hello  world" {
		t.Fatalf("text=%q", text)
	}
}

func TestAgentMessageCompletedDoesNotDuplicate(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	for _, delta := range []string{"Hello ", "Codex"} {
		payload, _ := json.Marshal(map[string]any{
			"threadId": "thr", "itemId": "m1", "delta": delta,
		})
		_ = a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, nil, st)
	}
	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"item": map[string]any{
			"type": "agentMessage", "id": "m1", "text": "Hello Codex",
		},
	})
	_ = a.handleNotification(context.Background(), "item/completed", completed, "thr", req, nil, st)
	if text != "Hello Codex" {
		t.Fatalf("duplicated text=%q", text)
	}
}

func TestAgentMessageCompletedEmitsSuffixOnly(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	payload, _ := json.Marshal(map[string]any{
		"threadId": "thr", "itemId": "m1", "delta": "Hello",
	})
	_ = a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, nil, st)
	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"item": map[string]any{
			"type": "agentMessage", "id": "m1", "text": "Hello\n\nworld",
		},
	})
	_ = a.handleNotification(context.Background(), "item/completed", completed, "thr", req, nil, st)
	if text != "Hello\n\nworld" {
		t.Fatalf("text=%q", text)
	}
}

func TestAgentMessageCompletedRepairsGapInLiveDeltas(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var streamed string
	var buf strings.Builder
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				streamed += d.Text
			}
		},
	}
	partial := "A suíte completa já validou os pacotes; falta a bateria de TUI, que é parte aando."
	full := "A suíte completa já validou os pacotes; falta a bateria de TUI, que é parte aguardando."
	payload, _ := json.Marshal(map[string]any{
		"threadId": "thr", "itemId": "m1", "delta": partial,
	})
	_ = a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, &buf, st)
	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"item": map[string]any{
			"type": "agentMessage", "id": "m1", "text": full,
		},
	})
	_ = a.handleNotification(context.Background(), "item/completed", completed, "thr", req, &buf, st)

	if streamed != partial {
		t.Fatalf("live text=%q want unchanged partial stream", streamed)
	}
	if got := st.output(buf.String()); got != full {
		t.Fatalf("final output=%q want repaired %q", got, full)
	}
}

func TestReasoningLiveDeltaSuppressedCompletedEmits(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var thinking string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindThinking {
				thinking += d.Text
			}
		},
	}
	delta, _ := json.Marshal(map[string]any{
		"threadId": "thr", "delta": "Theuserisasking",
	})
	_ = a.handleNotification(context.Background(), "item/reasoning/summaryTextDelta", delta, "thr", req, nil, st)
	if thinking != "" {
		t.Fatalf("live reasoning must not emit: %q", thinking)
	}
	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"item": map[string]any{
			"type": "reasoning", "id": "r1", "summary": "The user is asking in Portuguese.",
		},
	})
	_ = a.handleNotification(context.Background(), "item/completed", completed, "thr", req, nil, st)
	if thinking != "The user is asking in Portuguese." {
		t.Fatalf("thinking=%q", thinking)
	}
}

func TestDebugOnlyActivities(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var activities []string
	req := harness.ExecuteRequest{
		Debug: false,
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindActivity {
				activities = append(activities, d.Text)
			}
		},
	}

	cases := []struct {
		method  string
		payload map[string]any
	}{
		{"account/rateLimits/updated", map[string]any{"threadId": "thr"}},
		{"thread/tokenUsage/updated", map[string]any{
			"threadId": "thr",
			"usage":    map[string]any{"inputTokens": 0, "outputTokens": 0},
		}},
		{"item/started", map[string]any{
			"threadId": "thr",
			"item":     map[string]any{"type": "userMessage", "id": "u1"},
		}},
		{"item/started", map[string]any{
			"threadId": "thr",
			"item":     map[string]any{"type": "agentMessage", "id": "a1"},
		}},
	}
	for _, tc := range cases {
		raw, _ := json.Marshal(tc.payload)
		_ = a.handleNotification(context.Background(), tc.method, raw, "thr", req, nil, st)
	}
	if len(activities) != 0 {
		t.Fatalf("expected suppression without debug, got %v", activities)
	}

	req.Debug = true
	for _, tc := range cases {
		raw, _ := json.Marshal(tc.payload)
		_ = a.handleNotification(context.Background(), tc.method, raw, "thr", req, nil, st)
	}
	if len(activities) < 4 {
		t.Fatalf("expected activities in debug, got %v", activities)
	}
	joined := strings.Join(activities, "|")
	for _, want := range []string{"rate limits updated", "tokens in=", "userMessage", "agent message"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
}

func TestMapTokenUsageUsesLastTurnFromV2Snapshot(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	payload, err := json.Marshal(map[string]any{
		"threadId": "thr",
		"usage": map[string]any{
			"last": map[string]any{
				"inputTokens":  12,
				"outputTokens": 4,
			},
			"total": map[string]any{
				"inputTokens":  120,
				"outputTokens": 40,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out := a.handleNotification(context.Background(), "thread/tokenUsage/updated", payload, "thr", harness.ExecuteRequest{}, nil, st); out.err != nil {
		t.Fatal(out.err)
	}
	a.mu.Lock()
	got := a.usageBySession["thr"]
	a.mu.Unlock()
	if got.InputTokens != 12 || got.OutputTokens != 4 {
		t.Fatalf("usage=%+v want last-turn usage", got)
	}
}

func TestTurnSlotSerializesAndHonorsCancellation(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	if err := a.acquireTurnSlot(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- a.acquireTurnSlot(ctx)
	}()

	select {
	case err := <-result:
		t.Fatalf("second turn acquired before first was released: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued turn error=%v want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued turn did not observe cancellation")
	}
	a.releaseTurnSlot()
}

func TestTextBufWrittenBeforeOnStreamDelta(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var buf strings.Builder
	blocked := make(chan struct{})
	entered := make(chan struct{})
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind != harness.StreamKindText {
				return
			}
			close(entered)
			<-blocked
		},
	}
	payload, _ := json.Marshal(map[string]any{
		"threadId": "thr", "itemId": "m1", "delta": "Hello",
	})
	done := make(chan struct{})
	go func() {
		_ = a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, &buf, st)
		close(done)
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("OnStreamDelta did not start")
	}
	if buf.String() != "Hello" {
		t.Fatalf("textBuf=%q want Hello before OnStreamDelta returns", buf.String())
	}
	close(blocked)
	<-done
}

func TestTurnCompletedLastAgentMessageRepairsTruncation(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var text string
	var buf strings.Builder
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	// Partial live deltas only.
	payload, _ := json.Marshal(map[string]any{
		"threadId": "thr", "itemId": "m1", "delta": "Hello ",
	})
	_ = a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, &buf, st)

	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"turn": map[string]any{
			"status":           "completed",
			"lastAgentMessage": "Hello world from summary",
		},
	})
	out := a.handleNotification(context.Background(), "turn/completed", completed, "thr", req, &buf, st)
	if !out.done {
		t.Fatal("expected turn done")
	}
	if text != "Hello world from summary" {
		t.Fatalf("text=%q want repaired full message", text)
	}
	if buf.String() != "Hello world from summary" {
		t.Fatalf("buf=%q", buf.String())
	}
}

func TestTurnCompletedSingleItemRepairsGapInLiveDeltas(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var buf strings.Builder
	req := harness.ExecuteRequest{OnStreamDelta: func(harness.StreamDelta) {}}
	partial := "A bateria de TUI está rodando sem falhas parciais."
	full := "A bateria de TUI está aguardando e rodando sem falhas parciais."
	payload, _ := json.Marshal(map[string]any{
		"threadId": "thr", "itemId": "m1", "delta": partial,
	})
	_ = a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, &buf, st)
	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"turn": map[string]any{
			"status": "completed", "lastAgentMessage": full,
		},
	})
	_ = a.handleNotification(context.Background(), "turn/completed", completed, "thr", req, &buf, st)

	if got := st.output(buf.String()); got != full {
		t.Fatalf("final output=%q want repaired %q", got, full)
	}
}

func TestAgentMessageItemsSeparatedByNewline(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	for _, item := range []struct {
		id, delta string
	}{
		{"m1", "Starting Research discovery. I'll read the docs."},
		{"m2", "→ I'm using the grilling protocol."},
		{"m3", "→ Primeira pergunta: confirmamos o escopo?"},
	} {
		payload, _ := json.Marshal(map[string]any{
			"threadId": "thr", "itemId": item.id, "delta": item.delta,
		})
		if out := a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, nil, st); out.err != nil {
			t.Fatal(out.err)
		}
	}
	want := "Starting Research discovery. I'll read the docs.\n→ I'm using the grilling protocol.\n→ Primeira pergunta: confirmamos o escopo?"
	if text != want {
		t.Fatalf("text=%q want %q", text, want)
	}
}

func TestAgentMessageCompletedNewItemInsertsNewline(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	payload, _ := json.Marshal(map[string]any{
		"threadId": "thr", "itemId": "m1", "delta": "Starting Research.",
	})
	_ = a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, nil, st)
	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"item": map[string]any{
			"type": "agentMessage", "id": "m2", "text": "→ Next status line.",
		},
	})
	_ = a.handleNotification(context.Background(), "item/completed", completed, "thr", req, nil, st)
	want := "Starting Research.\n→ Next status line."
	if text != want {
		t.Fatalf("text=%q want %q", text, want)
	}
}

func TestTurnCompletedLastAgentMessageDoesNotAppendDivergentLastItem(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var text string
	var buf strings.Builder
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	for _, item := range []struct {
		id, delta string
	}{
		{"m1", "Starting Research discovery."},
		{"m2", "→ Primeira pergunta: para este ciclo, confirmamos o escopo?"},
	} {
		payload, _ := json.Marshal(map[string]any{
			"threadId": "thr", "itemId": item.id, "delta": item.delta,
		})
		_ = a.handleNotification(context.Background(), "item/agentMessage/delta", payload, "thr", req, &buf, st)
	}
	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"turn": map[string]any{
			"status":           "completed",
			"lastAgentMessage": "Primeira pergunta: confirmamos o escopo?",
		},
	})
	out := a.handleNotification(context.Background(), "turn/completed", completed, "thr", req, &buf, st)
	if !out.done {
		t.Fatal("expected turn done")
	}
	want := "Starting Research discovery.\n→ Primeira pergunta: para este ciclo, confirmamos o escopo?"
	if text != want {
		t.Fatalf("text=%q want %q", text, want)
	}
	if buf.String() != want {
		t.Fatalf("buf=%q want %q", buf.String(), want)
	}
}

func TestTurnCompletedLastAgentMessageEmitsWhenNoLiveDeltas(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	completed, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"turn": map[string]any{
			"status":           "completed",
			"lastAgentMessage": "Hello from summary only",
		},
	})
	out := a.handleNotification(context.Background(), "turn/completed", completed, "thr", req, nil, st)
	if !out.done {
		t.Fatal("expected turn done")
	}
	if text != "Hello from summary only" {
		t.Fatalf("text=%q", text)
	}
}

func TestNotifyMustDeliver(t *testing.T) {
	if !notifyMustDeliver("item/agentMessage/delta") {
		t.Fatal("agentMessage delta must deliver")
	}
	if !notifyMustDeliver("item/completed") {
		t.Fatal("item/completed must deliver")
	}
	if !notifyMustDeliver("turn/completed") {
		t.Fatal("turn/completed must deliver")
	}
	if !notifyMustDeliver("thread/tokenUsage/updated") {
		t.Fatal("thread/tokenUsage/updated must deliver")
	}
	if notifyMustDeliver("item/commandExecution/outputDelta") {
		t.Fatal("command output is best-effort")
	}
}

func TestCollabToolCallStartedAttributesGenericTask(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var got []harness.StreamDelta
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			got = append(got, d)
		},
	}
	payload, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"item": map[string]any{
			"type":  "collabToolCall",
			"id":    "c-explore",
			"agent": "explore",
			"model": "gpt-5.4",
		},
	})
	if out := a.handleNotification(context.Background(), "item/started", payload, "thr", req, nil, st); out.err != nil {
		t.Fatal(out.err)
	}
	if len(got) != 1 {
		t.Fatalf("deltas=%d want 1: %+v", len(got), got)
	}
	d := got[0]
	if d.Kind != harness.StreamKindTool || d.Phase != harness.StreamPhaseStarted {
		t.Fatalf("kind/phase=%s %s", d.Kind, d.Phase)
	}
	if d.CallID != "c-explore" {
		t.Fatalf("callID=%q", d.CallID)
	}
	if d.AgentName != "explore" {
		t.Fatalf("agent=%q want explore", d.AgentName)
	}
}

func TestCollabToolCallStartedAttributesNamedAgent(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	st := newTurnStreamState()
	var got []harness.StreamDelta
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			got = append(got, d)
		},
	}
	payload, _ := json.Marshal(map[string]any{
		"threadId": "thr",
		"item": map[string]any{
			"type":  "collabToolCall",
			"id":    "c-plan",
			"name":  "Task planning_agent",
			"model": "gpt-5.4",
		},
	})
	if out := a.handleNotification(context.Background(), "item/started", payload, "thr", req, nil, st); out.err != nil {
		t.Fatal(out.err)
	}
	if len(got) != 1 {
		t.Fatalf("deltas=%d want 1", len(got))
	}
	d := got[0]
	if d.AgentName != "planning_agent" || d.CallID != "c-plan" || d.Phase != harness.StreamPhaseStarted {
		t.Fatalf("delta=%+v", d)
	}
}
