package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestExecuteStreamCompletesWhenSessionIdleWhileSSEOpen(t *testing.T) {
	prevGrace, prevProbe := sseIdleGrace, sseIdleProbeInterval
	sseIdleGrace = 20 * time.Millisecond
	sseIdleProbeInterval = 20 * time.Millisecond
	t.Cleanup(func() {
		sseIdleGrace = prevGrace
		sseIdleProbeInterval = prevProbe
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-idle-open"})
		case "/session/sess-idle-open":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": map[string]any{"type": "idle"}})
		case "/session/sess-idle-open/prompt_async":
			w.WriteHeader(http.StatusAccepted)
		case "/event":
			w.Header().Set("Content-Type", "text/event-stream")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			_, _ = w.Write([]byte("data: {\"type\":\"server.heartbeat\",\"properties\":{}}\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
		case "/session/sess-idle-open/message":
			_ = json.NewEncoder(w).Encode([]sessionMessage{
				{
					Info:  map[string]any{"role": "assistant", "id": "msg-asst"},
					Parts: []part{{Type: "text", Text: "idle-recovered"}},
				},
			})
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := a.Execute(ctx, harness.ExecuteRequest{
		SessionID:     "sess-idle-open",
		Prompt:        "go",
		Stream:        true,
		OnStreamDelta: func(harness.StreamDelta) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "idle-recovered") {
		t.Fatalf("output=%q", res.Output)
	}
}

func TestExecuteResumeMissingSessionStartsNew(t *testing.T) {
	var warned atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/session/dead":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-fresh"})
		case r.URL.Path == "/session/sess-fresh/message":
			_ = json.NewEncoder(w).Encode(messageResponse{Parts: []part{{Type: "text", Text: "ok"}}})
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

	res, err := a.Execute(context.Background(), harness.ExecuteRequest{
		SessionID: "dead",
		Prompt:    "hi",
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindWarning && strings.Contains(d.Text, "dead") {
				warned.Store(true)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "sess-fresh" {
		t.Fatalf("session=%q", res.SessionID)
	}
	if !warned.Load() {
		t.Fatal("expected resume warning")
	}
}

func TestCancelEmptySessionCancelsInFlight(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-cancel"})
		case "/session/sess-cancel":
			w.WriteHeader(http.StatusOK)
		case "/session/sess-cancel/prompt_async":
			w.WriteHeader(http.StatusAccepted)
		case "/event":
			close(started)
			w.Header().Set("Content-Type", "text/event-stream")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
		case "/session/sess-cancel/abort":
			w.WriteHeader(http.StatusOK)
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

	errCh := make(chan error, 1)
	go func() {
		_, err := a.Execute(context.Background(), harness.ExecuteRequest{
			Prompt:        "hang",
			Stream:        true,
			OnStreamDelta: func(harness.StreamDelta) {},
		})
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not start SSE")
	}
	if err := a.Cancel(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancelled execute error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not return after Cancel(\"\")")
	}
}

func TestExecuteStreamKeepsWaitingWhileSessionBusy(t *testing.T) {
	prevGrace, prevProbe := sseIdleGrace, sseIdleProbeInterval
	sseIdleGrace = 20 * time.Millisecond
	sseIdleProbeInterval = 20 * time.Millisecond
	t.Cleanup(func() {
		sseIdleGrace = prevGrace
		sseIdleProbeInterval = prevProbe
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-busy-open"})
		case "/session/sess-busy-open":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": map[string]any{"type": "busy"}})
		case "/session/sess-busy-open/prompt_async":
			w.WriteHeader(http.StatusAccepted)
		case "/event":
			w.Header().Set("Content-Type", "text/event-stream")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
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

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := a.Execute(ctx, harness.ExecuteRequest{
		SessionID:     "sess-busy-open",
		Prompt:        "go",
		Stream:        true,
		OnStreamDelta: func(harness.StreamDelta) {},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v want deadline exceeded (busy session must not complete Execute)", err)
	}
}

func TestExecuteStreamErrorsWhenSessionGoneWhileSSEOpen(t *testing.T) {
	prevGrace, prevProbe := sseIdleGrace, sseIdleProbeInterval
	sseIdleGrace = 20 * time.Millisecond
	sseIdleProbeInterval = 20 * time.Millisecond
	t.Cleanup(func() {
		sseIdleGrace = prevGrace
		sseIdleProbeInterval = prevProbe
	})

	var sessionGets int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-gone-open"})
		case "/session/sess-gone-open":
			if atomic.AddInt32(&sessionGets, 1) == 1 {
				w.WriteHeader(http.StatusOK)
				return
			}
			http.NotFound(w, r)
		case "/session/sess-gone-open/prompt_async":
			w.WriteHeader(http.StatusAccepted)
		case "/event":
			w.Header().Set("Content-Type", "text/event-stream")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := a.Execute(ctx, harness.ExecuteRequest{
		SessionID:     "sess-gone-open",
		Prompt:        "go",
		Stream:        true,
		OnStreamDelta: func(harness.StreamDelta) {},
	})
	if err == nil || !strings.Contains(err.Error(), "sess-gone-open") {
		t.Fatalf("err=%v want session not found", err)
	}
}
