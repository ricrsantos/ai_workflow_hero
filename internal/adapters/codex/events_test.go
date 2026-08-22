package codex

import (
	"context"
	"encoding/json"
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
