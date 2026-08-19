package opencode_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
)

type prepareStubRunner struct {
	started int32
}

func (s *prepareStubRunner) Start(ctx context.Context, dir, name string, args ...string) (opencode.ProcessHandle, error) {
	atomic.AddInt32(&s.started, 1)
	return prepareStubHandle{pid: 4242}, nil
}

type prepareStubHandle struct {
	pid int
}

func (h prepareStubHandle) PID() int    { return h.pid }
func (h prepareStubHandle) Wait() error { return nil }
func (h prepareStubHandle) Kill() error { return nil }

func TestPrepareHeroStartSyncsResetsAndProbes(t *testing.T) {
	dir := setupOpenCodePrepareProject(t)

	var serveStarts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "sess-probe"})
		case "/session/sess-probe/message":
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			agent, _ := payload["agent"].(string)
			if agent != "backend_agent" {
				t.Errorf("probe agent=%q want backend_agent", agent)
			}
			if _, ok := payload["model"]; ok {
				t.Error("probe must rely on agent definition model, not request model")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"parts": []map[string]string{{"type": "text", "text": "ok"}},
			})
		default:
			if r.URL.Path == "/config/providers" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	runner := &prepareStubRunner{}
	a := opencode.NewAdapter(dir, nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = runner
	a.HTTP = srv.Client()
	a.ResolveServeURL = func(opencode.ProcessHandle) (string, int, error) {
		atomic.AddInt32(&serveStarts, 1)
		return srv.URL, 1, nil
	}

	oldDelay := opencode.ServeResetDelayForTest()
	opencode.SetServeResetDelayForTest(0)
	t.Cleanup(func() { opencode.SetServeResetDelayForTest(oldDelay) })

	if err := opencode.PrepareHeroStartWithAdapter(context.Background(), dir, nil, a); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&serveStarts) < 1 {
		t.Fatal("expected serve restart after reset")
	}

	data, err := os.ReadFile(filepath.Join(dir, ".opencode", "agents", "backend_agent.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "model: opencode-go/deepseek-v4-pro") {
		t.Fatalf("agent file not synced: %s", data)
	}
}

func TestPrepareHeroStartProbeFailure(t *testing.T) {
	dir := setupOpenCodePrepareProject(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session":
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "sess-probe"})
		case "/session/sess-probe/message":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`model not accepted`))
		default:
			if r.URL.Path == "/config/providers" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := opencode.NewAdapter(dir, nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &prepareStubRunner{}
	a.HTTP = srv.Client()
	a.ResolveServeURL = func(opencode.ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	opencode.SetServeResetDelayForTest(0)
	err := opencode.PrepareHeroStartWithAdapter(context.Background(), dir, nil, a)
	if err == nil {
		t.Fatal("expected probe failure")
	}
	if !strings.Contains(err.Error(), "Exit Hero TUI") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrepareHeroStartNoOpenCodeAgents(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), []byte(`title: t
objective: t
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
fallback_model:
  harness: cursor
  model: composer-2.5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := opencode.PrepareHeroStart(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
}

func TestResetServeWaitsDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := opencode.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &prepareStubRunner{}
	a.HTTP = srv.Client()
	a.ResolveServeURL = func(opencode.ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	oldDelay := opencode.ServeResetDelayForTest()
	opencode.SetServeResetDelayForTest(50 * time.Millisecond)
	t.Cleanup(func() { opencode.SetServeResetDelayForTest(oldDelay) })

	start := time.Now()
	if err := a.ResetServe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("reset too fast: %v", elapsed)
	}
}

func setupOpenCodePrepareProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	heroCfgDir := filepath.Join(dir, ".workflow-hero", "config")
	agentsDir := filepath.Join(dir, ".opencode", "agents")
	for _, d := range []string{cfgDir, heroCfgDir, agentsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), []byte(`title: t
objective: t
agents:
  backend_agent:
    harness: opencode
    model: opencode-go/deepseek-v4-pro
    reasoning_effort: max
    enable_fast_model: false
    thinking: na
  qa_agent:
    harness: opencode
    model: opencode/deepseek-v4-flash-free
fallback_model:
  harness: cursor
  model: composer-2.5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(heroCfgDir, "hero.json"), []byte(`{
  "harnesses": {
    "opencode": { "enabled": true, "model": "opencode-go/deepseek-v4-pro" }
  },
  "freechat_default": { "harness": "opencode", "model": "opencode-go/deepseek-v4-pro" }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "backend_agent.md"), []byte(`---
name: backend_agent
description: backend
model: stale/model
---
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "qa_agent.md"), []byte(`---
name: qa_agent
description: qa
model: stale/model
---
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
