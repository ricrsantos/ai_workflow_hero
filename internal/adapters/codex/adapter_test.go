package codex_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestAdapter_Name(t *testing.T) {
	a := codex.NewAdapter(t.TempDir(), nil)
	if a.Name() != "codex" {
		t.Fatalf("name=%q", a.Name())
	}
}

func TestAdapter_IsAvailableWithoutCLI(t *testing.T) {
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	if err := a.IsAvailable(context.Background()); err == nil {
		t.Fatal("expected error when CLI missing")
	}
}

func TestAdapter_ImplementsContract(t *testing.T) {
	var _ harness.HarnessAdapter = (*codex.Adapter)(nil)
	var _ harness.ModelLister = (*codex.Adapter)(nil)
}

// mockStdioPeer speaks a minimal Codex app-server JSON-RPC over pipes.
type mockStdioPeer struct {
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter

	mu       sync.Mutex
	authNil  bool
	models   []string
	turnProp map[string]any
	onTurn   func(params map[string]any)
	injectApproval bool
}

func newMockPeer() *mockStdioPeer {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	return &mockStdioPeer{
		stdinR:  stdinR,
		stdinW:  stdinW,
		stdoutR: stdoutR,
		stdoutW: stdoutW,
		models:  []string{"gpt-5.4", "gpt-5.3-codex"},
	}
}

func (m *mockStdioPeer) Start(_ context.Context, _, _ string, args ...string) (codex.StdioHandle, error) {
	if len(args) == 0 || args[0] != "app-server" {
		return nil, errors.New("expected app-server")
	}
	go m.serve()
	return &pipeHandle{peer: m, pid: 4242}, nil
}

type pipeHandle struct {
	peer *mockStdioPeer
	pid  int
}

func (h *pipeHandle) PID() int                 { return h.pid }
func (h *pipeHandle) Stdin() codex.WriteCloser  { return h.peer.stdinW }
func (h *pipeHandle) Stdout() codex.ReadCloser  { return h.peer.stdoutR }
func (h *pipeHandle) Wait() error               { return nil }
func (h *pipeHandle) Kill() error {
	_ = h.peer.stdinR.Close()
	_ = h.peer.stdoutW.Close()
	return nil
}

func (m *mockStdioPeer) write(v any) {
	b, _ := json.Marshal(v)
	m.mu.Lock()
	defer m.mu.Unlock()
	_, _ = m.stdoutW.Write(append(b, '\n'))
}

func (m *mockStdioPeer) serve() {
	sc := bufio.NewScanner(m.stdinR)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		method, _ := msg["method"].(string)
		id := msg["id"]
		params, _ := msg["params"].(map[string]any)
		switch method {
		case "initialize":
			m.write(map[string]any{"id": id, "result": map[string]any{"userAgent": "codex-mock"}})
		case "initialized":
			// notification ack — nothing
		case "account/read":
			if m.authNil {
				m.write(map[string]any{"id": id, "result": map[string]any{
					"account": nil, "requiresOpenaiAuth": true,
				}})
			} else {
				m.write(map[string]any{"id": id, "result": map[string]any{
					"account": map[string]any{"type": "chatgpt", "email": "u@example.com"},
					"requiresOpenaiAuth": true,
				}})
			}
		case "thread/start":
			m.write(map[string]any{"id": id, "result": map[string]any{
				"thread": map[string]any{"id": "thr_test_1"},
			}})
			m.write(map[string]any{"method": "thread/started", "params": map[string]any{
				"thread": map[string]any{"id": "thr_test_1"},
			}})
		case "thread/resume":
			m.write(map[string]any{"id": id, "result": map[string]any{
				"thread": map[string]any{"id": paramsString(params, "threadId")},
			}})
		case "model/list":
			var data []map[string]any
			for _, id := range m.models {
				data = append(data, map[string]any{"id": id, "model": id})
			}
			m.write(map[string]any{"id": id, "result": map[string]any{
				"data": data, "nextCursor": nil,
			}})
		case "turn/start":
			m.mu.Lock()
			m.turnProp = params
			onTurn := m.onTurn
			m.mu.Unlock()
			if onTurn != nil {
				onTurn(params)
			}
			m.write(map[string]any{"id": id, "result": map[string]any{
				"turn": map[string]any{"id": "turn_1", "status": "inProgress", "items": []any{}, "error": nil},
			}})
			m.write(map[string]any{"method": "turn/started", "params": map[string]any{
				"turn": map[string]any{"id": "turn_1", "status": "inProgress"},
			}})
			m.write(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{
				"threadId": "thr_test_1", "itemId": "item_1", "delta": "Hello ",
			}})
			m.write(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{
				"threadId": "thr_test_1", "itemId": "item_1", "delta": "Codex",
			}})
			m.write(map[string]any{"method": "item/reasoning/summaryTextDelta", "params": map[string]any{
				"threadId": "thr_test_1", "delta": "thinking…",
			}})
			m.write(map[string]any{"method": "item/completed", "params": map[string]any{
				"threadId": "thr_test_1",
				"item": map[string]any{
					"type": "reasoning", "id": "rsn_1", "summary": "thinking…",
				},
			}})
			m.write(map[string]any{"method": "item/completed", "params": map[string]any{
				"threadId": "thr_test_1",
				"item": map[string]any{
					"type": "agentMessage", "id": "item_1", "text": "Hello Codex",
				},
			}})
			m.write(map[string]any{"method": "thread/tokenUsage/updated", "params": map[string]any{
				"threadId": "thr_test_1",
				"usage": map[string]any{
					"inputTokens":  10,
					"outputTokens": 5,
				},
			}})
			m.write(map[string]any{"method": "unknown/event.from.future", "params": map[string]any{
				"threadId": "thr_test_1", "foo": "bar",
			}})
			if m.injectApproval {
				m.write(map[string]any{
					"id":     99001,
					"method": "item/commandExecution/requestApproval",
					"params": map[string]any{
						"threadId": "thr_test_1",
						"turnId":   "turn_1",
						"itemId":   "cmd_1",
						"command":  "rm -rf /tmp/x",
						"reason":   "destructive",
					},
				})
				// Wait for client approval response before completing the turn.
				for sc.Scan() {
					line := strings.TrimSpace(sc.Text())
					var resp map[string]any
					if json.Unmarshal([]byte(line), &resp) != nil {
						continue
					}
					if resp["id"] != nil {
						break
					}
				}
			}
			m.write(map[string]any{"method": "turn/completed", "params": map[string]any{
				"turn": map[string]any{"id": "turn_1", "status": "completed", "error": nil},
			}})
		case "turn/interrupt":
			m.write(map[string]any{"id": id, "result": map[string]any{}})
		default:
			if id != nil {
				m.write(map[string]any{"id": id, "error": map[string]any{
					"code": -32601, "message": "method not found: " + method,
				}})
			}
		}
	}
}

func paramsString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func TestExecute_MockStdioStreamsAndUsage(t *testing.T) {
	peer := newMockPeer()
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "/mock/codex", nil }
	a.Runner = peer

	var deltas []harness.StreamDelta
	var mu sync.Mutex
	res, err := a.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "hi",
		Model:  "gpt-5.4",
		Stream: true,
		OnStreamDelta: func(d harness.StreamDelta) {
			mu.Lock()
			deltas = append(deltas, d)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "thr_test_1" {
		t.Fatalf("session=%q", res.SessionID)
	}
	if !strings.Contains(res.Output, "Hello") || !strings.Contains(res.Output, "Codex") {
		t.Fatalf("output=%q", res.Output)
	}
	if res.Usage.InputTokens != 10 || res.Usage.OutputTokens != 5 {
		t.Fatalf("usage=%+v", res.Usage)
	}

	mu.Lock()
	defer mu.Unlock()
	var sawText, sawThink, sawWarn, sawUsageWarn bool
	var textJoined string
	for _, d := range deltas {
		switch d.Kind {
		case harness.StreamKindText:
			sawText = true
			textJoined += d.Text
		case harness.StreamKindThinking:
			sawThink = true
		case harness.StreamKindWarning:
			if strings.Contains(d.Text, "unknown/event") || strings.Contains(d.HarnessType, "unknown") {
				sawWarn = true
			}
			if strings.Contains(d.Text, "USD") {
				sawUsageWarn = true
			}
		}
	}
	if !sawText || !sawThink {
		t.Fatalf("deltas missing kinds text=%v think=%v: %+v", sawText, sawThink, deltas)
	}
	if sawWarn {
		t.Fatal("unrecognized events must not warn without Debug")
	}
	if textJoined != "Hello Codex" {
		t.Fatalf("agent text duplicated or spaced wrong: %q", textJoined)
	}
	if !sawUsageWarn {
		t.Fatal("expected USD-missing usage warning")
	}
}

func TestExecute_Unauthenticated(t *testing.T) {
	peer := newMockPeer()
	peer.authNil = true
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "/mock/codex", nil }
	a.Runner = peer

	_, err := a.Execute(context.Background(), harness.ExecuteRequest{Prompt: "hi"})
	if err == nil {
		t.Fatal("expected auth error")
	}
	var auth *codex.AuthError
	if !errors.As(err, &auth) {
		t.Fatalf("want AuthError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "codex login") {
		t.Fatalf("missing login guidance: %v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "api key") {
		t.Fatalf("must not mention API key: %v", err)
	}
}

func TestListModels_NativeIDs(t *testing.T) {
	peer := newMockPeer()
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "/mock/codex", nil }
	a.Runner = peer

	models, err := a.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "gpt-5.4" {
		t.Fatalf("models=%v", models)
	}
}

func TestExecute_PropertyMapping(t *testing.T) {
	peer := newMockPeer()
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "/mock/codex", nil }
	a.Runner = peer

	_, err := a.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "hi",
		Model:  "gpt-5.4",
		Properties: map[string]string{
			harness.PropertyEffort: "high",
			harness.PropertyThink:  "max",
			harness.PropertyFast:   "true", // unsupported → omitted
		},
		Stream: true,
		OnStreamDelta: func(harness.StreamDelta) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.turnProp["effort"] != "high" {
		t.Fatalf("effort=%v", peer.turnProp["effort"])
	}
	if peer.turnProp["summary"] != "detailed" {
		t.Fatalf("summary=%v", peer.turnProp["summary"])
	}
	if _, ok := peer.turnProp["fast"]; ok {
		t.Fatal("fs must not appear in native payload")
	}
}

func TestPermission_AllowPrompt(t *testing.T) {
	peer := newMockPeer()
	peer.injectApproval = true
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "/mock/codex", nil }
	a.Runner = peer

	var sawPerm bool
	asked := make(chan struct{}, 1)
	_, err := a.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "hi",
		Stream: true,
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindPermission && strings.Contains(d.Text, "Allow?") {
				sawPerm = true
			}
		},
		OnPermissionRequest: func(_ context.Context, pr harness.PermissionRequest) (harness.PermissionResponse, error) {
			select {
			case asked <- struct{}{}:
			default:
			}
			if pr.HarnessType != "item/commandExecution/requestApproval" {
				t.Errorf("harness type=%q", pr.HarnessType)
			}
			return harness.PermissionResponse{Approved: true}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-asked:
	case <-time.After(2 * time.Second):
		t.Fatal("permission callback not invoked")
	}
	if !sawPerm {
		t.Fatal("expected Allow? stream delta")
	}
}

func TestEnsureAppServer_NeverAttachesForeign(t *testing.T) {
	// Starting twice reuses the Hero-managed child; Runner is only called once.
	peer := newMockPeer()
	starts := 0
	runner := &countingRunner{inner: peer, count: &starts}
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "/mock/codex", nil }
	a.Runner = runner

	if _, err := a.ListModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ListModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	if starts != 1 {
		t.Fatalf("expected single Hero-managed start, got %d", starts)
	}
}

type countingRunner struct {
	inner codex.ProcessRunner
	count *int
}

func (c *countingRunner) Start(ctx context.Context, dir, name string, args ...string) (codex.StdioHandle, error) {
	*c.count++
	return c.inner.Start(ctx, dir, name, args...)
}

func TestAppServerHandshakeFailure(t *testing.T) {
	a := codex.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "/mock/codex", nil }
	a.Runner = &brokenRunner{}

	err := a.IsAvailable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected handshake failure")
	}
	var ase *codex.AppServerError
	if !errors.As(err, &ase) {
		t.Fatalf("want AppServerError, got %T: %v", err, err)
	}
}

type brokenRunner struct{}

func (brokenRunner) Start(_ context.Context, _, _ string, _ ...string) (codex.StdioHandle, error) {
	r, w := io.Pipe()
	_ = w.Close() // stdout EOF immediately → handshake fails
	return &brokenHandle{stdout: r, stdin: discardWC{}}, nil
}

type brokenHandle struct {
	stdout *io.PipeReader
	stdin  discardWC
}

func (h *brokenHandle) PID() int                { return 1 }
func (h *brokenHandle) Stdin() codex.WriteCloser { return h.stdin }
func (h *brokenHandle) Stdout() codex.ReadCloser { return h.stdout }
func (h *brokenHandle) Wait() error              { return nil }
func (h *brokenHandle) Kill() error              { return nil }

type discardWC struct{}

func (discardWC) Write(p []byte) (int, error) { return len(p), nil }
func (discardWC) Close() error                { return nil }
