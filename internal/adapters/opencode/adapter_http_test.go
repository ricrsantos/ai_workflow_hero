package opencode

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type stubRunner struct {
	started int32
}

func (s *stubRunner) Start(ctx context.Context, dir, name string, args ...string) (ProcessHandle, error) {
	atomic.AddInt32(&s.started, 1)
	return stubHandle{pid: 4242}, nil
}

type stubHandle struct {
	pid int
}

func (h stubHandle) PID() int    { return h.pid }
func (h stubHandle) Wait() error { return nil }
func (h stubHandle) Kill() error { return nil }

func TestEnsureServeNeverAttachesToForeignPort(t *testing.T) {
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("adapter must not call a foreign serve URL")
	}))
	defer foreign.Close()

	own := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer own.Close()

	runner := &stubRunner{}
	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = runner
	a.HTTP = own.Client()
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) {
		return own.URL, 1, nil
	}

	if err := a.ensureServe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&runner.started) != 1 {
		t.Fatalf("started=%d want 1", runner.started)
	}
	a.mu.Lock()
	got := a.baseURL
	a.mu.Unlock()
	if got != own.URL {
		t.Fatalf("baseURL=%q want own server %q", got, own.URL)
	}
}

func TestCreateSessionAndExecuteHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-1"})
		case "/session/sess-1":
			w.WriteHeader(http.StatusOK)
		case "/session/sess-1/message":
			_ = json.NewEncoder(w).Encode(messageResponse{Parts: []part{{Type: "text", Text: "hello"}}})
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

	sess, err := a.CreateSession(context.Background(), harness.SessionRequest{StageName: "research"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID != "sess-1" {
		t.Fatalf("id=%q", sess.ID)
	}
	if err := a.ResumeSession(context.Background(), "sess-1"); err != nil {
		t.Fatal(err)
	}
	res, err := a.Execute(context.Background(), harness.ExecuteRequest{
		SessionID: "sess-1",
		Prompt:    "hi",
		Model:     "anthropic/claude-sonnet-4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "hello" {
		t.Fatalf("output=%q", res.Output)
	}
}

func TestExecuteStreamAndCancelHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-2"})
		case "/session/sess-2":
			w.WriteHeader(http.StatusOK)
		case "/session/sess-2/prompt_async":
			w.WriteHeader(http.StatusAccepted)
		case "/event":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"sess-2\",\"part\":{\"id\":\"prt-user\",\"type\":\"text\",\"text\":\"user question\",\"messageID\":\"msg-user\"}}}\n\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"message.updated\",\"properties\":{\"sessionID\":\"sess-2\",\"info\":{\"id\":\"msg-asst\",\"role\":\"assistant\"}}}\n\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"sess-2\",\"part\":{\"id\":\"prt-1\",\"type\":\"text\",\"text\":\"chunk\",\"messageID\":\"msg-asst\"}}}\n\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"session.idle\",\"properties\":{\"sessionID\":\"sess-2\"}}\n\n")
		case "/session/sess-2/abort":
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

	var deltas []string
	var boundSession string
	res, err := a.Execute(context.Background(), harness.ExecuteRequest{
		SessionID: "sess-2",
		Prompt:    "stream",
		Stream:    true,
		OnStreamDelta: func(d harness.StreamDelta) {
			deltas = append(deltas, d.Text)
			if d.Kind == harness.StreamKindSession && d.HarnessType == "session.bound" {
				boundSession = d.SessionID
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "chunk") {
		t.Fatalf("output=%q deltas=%v", res.Output, deltas)
	}
	if strings.Contains(res.Output, "user question") {
		t.Fatalf("user prompt leaked into output: %q", res.Output)
	}
	if boundSession != "sess-2" {
		t.Fatalf("session.bound=%q want sess-2", boundSession)
	}
	if err := a.Cancel(context.Background(), "sess-2"); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteStreamSSEReconnect(t *testing.T) {
	var eventHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-reconnect"})
		case "/session/sess-reconnect":
			w.WriteHeader(http.StatusOK)
		case "/session/sess-reconnect/prompt_async":
			w.WriteHeader(http.StatusAccepted)
		case "/event":
			atomic.AddInt32(&eventHits, 1)
			w.Header().Set("Content-Type", "text/event-stream")
			if atomic.LoadInt32(&eventHits) == 1 {
				_, _ = io.WriteString(w, "data: {\"type\":\"message.updated\",\"properties\":{\"sessionID\":\"sess-reconnect\",\"info\":{\"id\":\"msg-asst\",\"role\":\"assistant\"}}}\n\n")
				_, _ = io.WriteString(w, "data: {\"type\":\"message.part.updated\",\"properties\":{\"sessionID\":\"sess-reconnect\",\"part\":{\"id\":\"prt-1\",\"type\":\"text\",\"text\":\"partial\",\"messageID\":\"msg-asst\"}}}\n\n")
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				return
			}
			_, _ = io.WriteString(w, "data: {\"type\":\"session.idle\",\"properties\":{\"sessionID\":\"sess-reconnect\"}}\n\n")
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
		SessionID:     "sess-reconnect",
		Prompt:        "stream",
		Stream:        true,
		OnStreamDelta: func(harness.StreamDelta) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "partial") {
		t.Fatalf("output=%q", res.Output)
	}
	if atomic.LoadInt32(&eventHits) < 2 {
		t.Fatalf("eventHits=%d want >=2", eventHits)
	}
}

func TestListModelsHTTP_ObjectShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config/providers" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{
  "providers": [
    {
      "id": "anthropic",
      "models": {
        "claude-sonnet-4": { "name": "Claude Sonnet 4" },
        "claude-opus-4": { "name": "Claude Opus 4" }
      }
    },
    {
      "id": "xai",
      "models": {
        "grok-4": { "name": "Grok 4" }
      }
    }
  ],
  "default": { "anthropic": "claude-sonnet-4" }
}`))
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &stubRunner{}
	a.HTTP = srv.Client()
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	models, err := a.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"anthropic/claude-sonnet-4": true,
		"anthropic/claude-opus-4":   true,
		"xai/grok-4":                true,
	}
	if len(models) != len(want) {
		t.Fatalf("models=%v", models)
	}
	for _, m := range models {
		if !want[m] {
			t.Fatalf("unexpected model %q in %v", m, models)
		}
	}
}

func TestListModelsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config/providers" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(providersResponse{Providers: []providerEntry{
			{ID: "anthropic", Models: map[string]modelMeta{"claude-sonnet-4": {}}},
		}})
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &stubRunner{}
	a.HTTP = srv.Client()
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	models, err := a.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0] != "anthropic/claude-sonnet-4" {
		t.Fatalf("models=%v", models)
	}
}

func TestEnsureServeRecoversDeadURL(t *testing.T) {
	own := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer own.Close()

	runner := &stubRunner{}
	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = runner
	a.HTTP = own.Client()
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) {
		return own.URL, 1, nil
	}
	a.mu.Lock()
	a.baseURL = "http://127.0.0.1:1"
	a.mu.Unlock()

	if err := a.ensureServe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&runner.started) != 1 {
		t.Fatalf("started=%d want 1", runner.started)
	}
	a.mu.Lock()
	got := a.baseURL
	a.mu.Unlock()
	if got != own.URL {
		t.Fatalf("baseURL=%q want %q", got, own.URL)
	}
}

func TestEnsureServeStartsOncePerAdapter(t *testing.T) {
	own := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer own.Close()

	runner := &stubRunner{}
	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = runner
	a.HTTP = own.Client()
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) {
		return own.URL, 1, nil
	}

	if err := a.ensureServe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.ensureServe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&runner.started) != 1 {
		t.Fatalf("started=%d want 1", runner.started)
	}
}
