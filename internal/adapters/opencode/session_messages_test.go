package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestExecuteStreamRecoversFromSessionMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(opencodeSession{ID: "sess-recover"})
		case "/session/sess-recover/prompt_async":
			w.WriteHeader(http.StatusAccepted)
		case "/event":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"type\":\"server.connected\",\"properties\":{}}\n\n"))
		case "/session/sess-recover/message":
			_ = json.NewEncoder(w).Encode([]sessionMessage{
				{
					Info: map[string]any{"role": "assistant", "id": "msg-asst"},
					Parts: []part{{Type: "text", Text: "recovered"}},
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

	res, err := a.Execute(context.Background(), harness.ExecuteRequest{
		SessionID:     "sess-recover",
		Prompt:        "fast",
		Stream:        true,
		OnStreamDelta: func(harness.StreamDelta) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "recovered") {
		t.Fatalf("output=%q", res.Output)
	}
}
