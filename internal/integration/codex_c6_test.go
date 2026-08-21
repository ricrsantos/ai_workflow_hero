package integration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/tui"
	"github.com/ricrsantos/ai_workflow_hero/internal/upgrade"
)

// TestIntegration_Upgrade24EnableCodexMockExecuteOrphanReap covers C6 §9.1:
// 2.4.x upgrade fixture → enable Codex via harness path → mock stdio Execute
// streams deltas → Codex orphan reap clears registry.
func TestIntegration_Upgrade24EnableCodexMockExecuteOrphanReap(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "2.4.1")

	// Rewrite hero.json as a 2.4.x Cursor+OpenCode project (no codex key).
	v24 := install.HeroJSON{
		CLI:    install.CLIInfo{Version: "2.4.1", InstalledAt: "2026-08-20T00:00:00Z", Tools: []string{"cursor", "opencode"}},
		Assets: install.AssetsInfo{Version: "2.4.1", InstalledAt: "2026-08-20T00:00:00Z"},
		Harnesses: map[string]install.HarnessConfig{
			"cursor":   {Enabled: true, Model: "composer-2.5"},
			"opencode": {Enabled: true, Model: "opencode-go/deepseek-v4-flash"},
		},
	}
	data, err := json.MarshalIndent(v24, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	heroPath := filepath.Join(dir, cursoradapter.HeroJSONPath)
	if err := os.WriteFile(heroPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(dir, ".codex"))

	var sb strings.Builder
	if _, err := upgrade.Run(upgrade.Options{
		ProjectDir: dir,
		Version:    "2.5.0",
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("upgrade 2.4 → 2.5: %v", err)
	}

	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.CLI.Version != "2.5.0" {
		t.Fatalf("cli.version=%q", hero.CLI.Version)
	}
	if install.IsHarnessEnabled(hero, "codex") {
		t.Fatal("upgrade must leave Codex disabled")
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex")); !os.IsNotExist(err) {
		t.Fatal("upgrade must not auto-provision .codex/")
	}

	// /hero-harness enable path (same helper the TUI picker calls).
	if err := install.EnableHarnessWithProjection(dir, "codex", assets.FS); err != nil {
		t.Fatalf("enable Codex: %v", err)
	}
	hero, err = install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.IsHarnessEnabled(hero, "codex") {
		t.Fatal("codex should be enabled after harness path")
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "agents")); err != nil {
		t.Fatalf(".codex/ not projected: %v", err)
	}

	st, err := store.OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	peer := newIntegrationCodexPeer()
	a := codex.NewAdapter(dir, st)
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
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Output, "Hello") || !strings.Contains(res.Output, "Codex") {
		t.Fatalf("output=%q", res.Output)
	}
	mu.Lock()
	var sawText, sawThink bool
	for _, d := range deltas {
		switch d.Kind {
		case harness.StreamKindText:
			sawText = true
		case harness.StreamKindThinking:
			sawThink = true
		}
	}
	mu.Unlock()
	if !sawText || !sawThink {
		t.Fatalf("expected text+thinking deltas, got %+v", deltas)
	}

	// Orphan reap fixture: dead registry PID must be cleared (never kill foreign).
	if _, err := st.InsertCodexServeRegistry(dir, 999999); err != nil {
		t.Fatal(err)
	}
	if err := tui.ReapCodexOrphansForTest(context.Background(), dir, st); err != nil {
		t.Fatalf("reap: %v", err)
	}
	entries, err := st.ListCodexServeRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty Codex registry after orphan reap, got %v", entries)
	}
}

// integrationCodexPeer is a minimal mock app-server for §9.1 integration.
type integrationCodexPeer struct {
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
	mu      sync.Mutex
}

func newIntegrationCodexPeer() *integrationCodexPeer {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	return &integrationCodexPeer{
		stdinR: stdinR, stdinW: stdinW,
		stdoutR: stdoutR, stdoutW: stdoutW,
	}
}

func (m *integrationCodexPeer) Start(_ context.Context, _, _ string, args ...string) (codex.StdioHandle, error) {
	if len(args) == 0 || args[0] != "app-server" {
		return nil, io.ErrUnexpectedEOF
	}
	go m.serve()
	return &integrationPipeHandle{peer: m, pid: 4242}, nil
}

type integrationPipeHandle struct {
	peer *integrationCodexPeer
	pid  int
}

func (h *integrationPipeHandle) PID() int                { return h.pid }
func (h *integrationPipeHandle) Stdin() codex.WriteCloser { return h.peer.stdinW }
func (h *integrationPipeHandle) Stdout() codex.ReadCloser { return h.peer.stdoutR }
func (h *integrationPipeHandle) Wait() error              { return nil }
func (h *integrationPipeHandle) Kill() error {
	_ = h.peer.stdinR.Close()
	_ = h.peer.stdoutW.Close()
	return nil
}

func (m *integrationCodexPeer) write(v any) {
	b, _ := json.Marshal(v)
	m.mu.Lock()
	defer m.mu.Unlock()
	_, _ = m.stdoutW.Write(append(b, '\n'))
}

func (m *integrationCodexPeer) serve() {
	sc := bufio.NewScanner(m.stdinR)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var msg map[string]any
		if json.Unmarshal([]byte(line), &msg) != nil {
			continue
		}
		method, _ := msg["method"].(string)
		id := msg["id"]
		switch method {
		case "initialize":
			m.write(map[string]any{"id": id, "result": map[string]any{"userAgent": "codex-mock"}})
		case "initialized":
		case "account/read":
			m.write(map[string]any{"id": id, "result": map[string]any{
				"account":            map[string]any{"type": "chatgpt", "email": "u@example.com"},
				"requiresOpenaiAuth": true,
			}})
		case "thread/start":
			m.write(map[string]any{"id": id, "result": map[string]any{
				"thread": map[string]any{"id": "thr_int_1"},
			}})
		case "turn/start":
			m.write(map[string]any{"id": id, "result": map[string]any{
				"turn": map[string]any{"id": "turn_1", "status": "inProgress", "items": []any{}, "error": nil},
			}})
			m.write(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{
				"threadId": "thr_int_1", "itemId": "item_1", "delta": "Hello ",
			}})
			m.write(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{
				"threadId": "thr_int_1", "itemId": "item_1", "delta": "Codex",
			}})
			m.write(map[string]any{"method": "item/reasoning/summaryTextDelta", "params": map[string]any{
				"threadId": "thr_int_1", "delta": "thinking…",
			}})
			m.write(map[string]any{"method": "item/completed", "params": map[string]any{
				"threadId": "thr_int_1",
				"item": map[string]any{
					"type": "reasoning", "id": "rsn_1", "summary": "thinking…",
				},
			}})
			m.write(map[string]any{"method": "turn/completed", "params": map[string]any{
				"turn": map[string]any{"id": "turn_1", "status": "completed", "error": nil},
			}})
		default:
			if id != nil {
				m.write(map[string]any{"id": id, "error": map[string]any{
					"code": -32601, "message": "method not found: " + method,
				}})
			}
		}
	}
}
