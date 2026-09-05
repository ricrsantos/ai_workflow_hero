package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type eventFailClient struct {
	inner  HTTPDoer
	failAt int32
	n      int32
}

func (c *eventFailClient) Do(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/event" {
		n := atomic.AddInt32(&c.n, 1)
		if c.failAt > 0 && n == c.failAt {
			return nil, errors.New(`Get "http://127.0.0.1:4096/event": dial tcp 127.0.0.1:4096: connect: connection refused`)
		}
	}
	return c.inner.Do(req)
}

func TestInspectAssistantTurn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		msg         storedMessage
		complete    bool
		runningTool bool
		hasText     bool
	}{
		{
			name: "completed text",
			msg: storedMessage{
				Info:  map[string]any{"role": "assistant", "time": map[string]any{"created": 1.0, "completed": 2.0}},
				Parts: []map[string]any{{"type": "text", "text": "done"}},
			},
			complete: true,
			hasText:  true,
		},
		{
			name: "running bash",
			msg: storedMessage{
				Info: map[string]any{"role": "assistant", "time": map[string]any{"created": 1.0}},
				Parts: []map[string]any{
					{"type": "step-start"},
					{"type": "tool", "tool": "bash", "state": map[string]any{"status": "running", "input": map[string]any{"command": "go test ./..."}}},
				},
			},
			runningTool: true,
		},
		{
			name: "incomplete no tool",
			msg: storedMessage{
				Info:  map[string]any{"role": "assistant", "time": map[string]any{"created": 1.0}},
				Parts: []map[string]any{{"type": "reasoning"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inspectAssistantTurn(tc.msg)
			if got.complete != tc.complete || got.runningTool != tc.runningTool || got.hasText != tc.hasText {
				t.Fatalf("got complete=%v running=%v hasText=%v want complete=%v running=%v hasText=%v",
					got.complete, got.runningTool, got.hasText, tc.complete, tc.runningTool, tc.hasText)
			}
			if tc.hasText && !got.finishedWithText() {
				t.Fatal("expected finishedWithText")
			}
			if tc.runningTool && got.finishedWithText() {
				t.Fatal("running tool must not look finished")
			}
		})
	}
}

func TestExecuteStreamContinuesAfterServeRestart(t *testing.T) {
	var mu sync.Mutex
	var prompts []string
	var aborts int32
	var eventHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-restart"})
		case "/session/sess-restart":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "sess-restart"})
		case "/session/sess-restart/prompt_async":
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			prompts = append(prompts, string(body))
			mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
		case "/session/sess-restart/abort":
			atomic.AddInt32(&aborts, 1)
			w.WriteHeader(http.StatusOK)
		case "/event":
			n := atomic.AddInt32(&eventHits, 1)
			w.Header().Set("Content-Type", "text/event-stream")
			if n == 1 {
				_, _ = w.Write([]byte("data: {\"type\":\"server.connected\",\"properties\":{}}\n\n"))
				return
			}
			_, _ = w.Write([]byte("data: {\"type\":\"message.updated\",\"properties\":{\"sessionID\":\"sess-restart\",\"info\":{\"id\":\"msg-2\",\"role\":\"assistant\"}}}\n\n"))
			_, _ = w.Write([]byte("data: {\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"sess-restart\",\"part\":{\"id\":\"prt-1\",\"type\":\"text\",\"text\":\"continued-ok\",\"messageID\":\"msg-2\"}}}\n\n"))
			_, _ = w.Write([]byte("data: {\"type\":\"session.idle\",\"properties\":{\"sessionID\":\"sess-restart\"}}\n\n"))
		case "/session/sess-restart/message":
			_ = json.NewEncoder(w).Encode([]storedMessage{{
				Info: map[string]any{"role": "assistant", "id": "msg-stuck", "time": map[string]any{"created": 1.0}},
				Parts: []map[string]any{
					{"type": "tool", "tool": "bash", "state": map[string]any{"status": "running"}},
				},
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &stubRunner{}
	a.HTTP = &eventFailClient{inner: srv.Client(), failAt: 2}
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	var warned atomic.Bool
	res, err := a.Execute(context.Background(), harness.ExecuteRequest{
		SessionID: "sess-restart",
		Prompt:    "implement telegram",
		AgentName: "generic_agent",
		Stream:    true,
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindWarning && strings.Contains(d.Text, "continue") {
				warned.Store(true)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "continued-ok") {
		t.Fatalf("output=%q", res.Output)
	}
	if atomic.LoadInt32(&aborts) < 1 {
		t.Fatal("expected abort of the interrupted turn")
	}
	mu.Lock()
	got := append([]string(nil), prompts...)
	mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("prompt_async count=%d want 2 bodies=%v", len(got), got)
	}
	if !strings.Contains(got[0], "implement telegram") {
		t.Fatalf("first prompt=%s", got[0])
	}
	if !strings.Contains(got[1], interruptedTurnContinuePrompt) {
		t.Fatalf("continue prompt=%s", got[1])
	}
	if !strings.Contains(got[1], "generic_agent") {
		t.Fatalf("continue payload dropped agent: %s", got[1])
	}
	if !warned.Load() {
		t.Fatal("expected continue warning on the stream")
	}
}

func TestExecuteStreamRecoversCompletedTurnAfterServeRestart(t *testing.T) {
	var prompts int32
	var aborts int32
	var eventHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-done"})
		case "/session/sess-done":
			w.WriteHeader(http.StatusOK)
		case "/session/sess-done/prompt_async":
			atomic.AddInt32(&prompts, 1)
			w.WriteHeader(http.StatusAccepted)
		case "/session/sess-done/abort":
			atomic.AddInt32(&aborts, 1)
			w.WriteHeader(http.StatusOK)
		case "/event":
			n := atomic.AddInt32(&eventHits, 1)
			w.Header().Set("Content-Type", "text/event-stream")
			if n == 1 {
				_, _ = w.Write([]byte("data: {\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"sess-done\",\"part\":{\"id\":\"prt-0\",\"type\":\"text\",\"text\":\"partial\",\"messageID\":\"msg-1\"}}}\n\n"))
				return
			}
			<-r.Context().Done()
		case "/session/sess-done/message":
			_ = json.NewEncoder(w).Encode([]storedMessage{{
				Info:  map[string]any{"role": "assistant", "time": map[string]any{"created": 1.0, "completed": 2.0}},
				Parts: []map[string]any{{"type": "text", "text": "already-done"}},
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &stubRunner{}
	a.HTTP = &eventFailClient{inner: srv.Client(), failAt: 2}
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	res, err := a.Execute(context.Background(), harness.ExecuteRequest{
		SessionID:     "sess-done",
		Prompt:        "go",
		Stream:        true,
		OnStreamDelta: func(harness.StreamDelta) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "already-done") {
		t.Fatalf("output=%q", res.Output)
	}
	if atomic.LoadInt32(&prompts) != 1 {
		t.Fatalf("prompt_async=%d want 1", prompts)
	}
	if atomic.LoadInt32(&aborts) != 0 {
		t.Fatalf("abort=%d want 0", aborts)
	}
}

func TestExecuteStreamDoesNotContinueOnSSEBlip(t *testing.T) {
	prevGrace, prevProbe := sseIdleGrace, sseIdleProbeInterval
	sseIdleGrace = 20 * time.Millisecond
	sseIdleProbeInterval = 20 * time.Millisecond
	t.Cleanup(func() {
		sseIdleGrace = prevGrace
		sseIdleProbeInterval = prevProbe
	})

	var prompts int32
	var aborts int32
	var eventHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-blip"})
		case "/session/sess-blip":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": map[string]any{"type": "busy"}})
		case "/session/sess-blip/prompt_async":
			atomic.AddInt32(&prompts, 1)
			w.WriteHeader(http.StatusAccepted)
		case "/session/sess-blip/abort":
			atomic.AddInt32(&aborts, 1)
			w.WriteHeader(http.StatusOK)
		case "/event":
			n := atomic.AddInt32(&eventHits, 1)
			w.Header().Set("Content-Type", "text/event-stream")
			if n == 1 {
				_, _ = w.Write([]byte("data: {\"type\":\"server.heartbeat\",\"properties\":{}}\n\n"))
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
		case "/session/sess-blip/message":
			_ = json.NewEncoder(w).Encode([]storedMessage{{
				Info: map[string]any{"role": "assistant", "time": map[string]any{"created": 1.0}},
				Parts: []map[string]any{
					{"type": "tool", "tool": "bash", "state": map[string]any{"status": "running"}},
				},
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &stubRunner{}
	a.HTTP = srv.Client()
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_, err := a.Execute(ctx, harness.ExecuteRequest{
		SessionID:     "sess-blip",
		Prompt:        "go",
		Stream:        true,
		OnStreamDelta: func(harness.StreamDelta) {},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v want deadline exceeded", err)
	}
	if atomic.LoadInt32(&prompts) != 1 {
		t.Fatalf("prompt_async=%d want 1 (SSE blip must not re-prompt)", prompts)
	}
	if atomic.LoadInt32(&aborts) != 0 {
		t.Fatalf("abort=%d want 0", aborts)
	}
}
