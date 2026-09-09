package claude

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type fakeTurnLauncher struct {
	mu         sync.Mutex
	path       string
	invocation Invocation
	process    *fakeProcess
	starts     int
	onStart    func(Invocation) error
}

func (l *fakeTurnLauncher) Start(_ context.Context, path string, invocation Invocation) (Process, error) {
	l.mu.Lock()
	l.path, l.invocation, l.starts = path, invocation.Clone(), l.starts+1
	onStart := l.onStart
	l.mu.Unlock()
	if onStart != nil {
		if err := onStart(invocation.Clone()); err != nil {
			return nil, err
		}
	}
	return l.process, nil
}

type fakeProcess struct {
	mu          sync.Mutex
	stdout      io.Reader
	stderr      io.Reader
	waitErr     error
	interrupts  int
	kills       int
	waitGate    chan struct{}
	waitStarted chan struct{}
	releaseOnce sync.Once
}

func (p *fakeProcess) Stdout() io.Reader { return p.stdout }
func (p *fakeProcess) Stderr() io.Reader { return p.stderr }
func (p *fakeProcess) Wait() error {
	if p.waitStarted != nil {
		close(p.waitStarted)
	}
	if p.waitGate != nil {
		<-p.waitGate
	}
	return p.waitErr
}
func (p *fakeProcess) PID() int { return 4242 }
func (p *fakeProcess) Interrupt() error {
	p.mu.Lock()
	p.interrupts++
	p.mu.Unlock()
	p.releaseWait()
	return nil
}
func (p *fakeProcess) Kill() error {
	p.mu.Lock()
	p.kills++
	p.mu.Unlock()
	p.releaseWait()
	return nil
}

func (p *fakeProcess) releaseWait() {
	if p.waitGate != nil {
		p.releaseOnce.Do(func() { close(p.waitGate) })
	}
}

func (p *fakeProcess) interruptCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.interrupts
}

func (p *fakeProcess) killCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.kills
}

type fakeBridgeHandle struct {
	mu         sync.Mutex
	runCalls   int
	closeCalls int
}

func (b *fakeBridgeHandle) Run(context.Context) error {
	b.mu.Lock()
	b.runCalls++
	b.mu.Unlock()
	return nil
}

func (b *fakeBridgeHandle) Close() error {
	b.mu.Lock()
	b.closeCalls++
	b.mu.Unlock()
	return nil
}

func newTestAdapter(process *fakeProcess) (*Adapter, *fakeTurnLauncher) {
	launcher := &fakeTurnLauncher{process: process}
	return &Adapter{
		ProjectDir: "/project",
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		LookPath:   func(string) (string, error) { return "/bin/claude", nil },
		ProbeLauncher: &recordedLauncher{responses: map[string]ProbeResult{
			"--version": {Stdout: "Claude Code 2.1.261"},
			"--help":    {Stdout: supportedHelp},
		}},
		ProcessLauncher: launcher,
		Clock:           fixedClock{at: time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)},
		TokenSource:     func() (string, error) { return "token", nil },
		sessions:        map[string]*sessionState{},
		active:          map[string]*runningTurn{},
	}, launcher
}

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func TestAdapterExecuteSupervisesAndNormalizesClaudeTurn(t *testing.T) {
	process := &fakeProcess{
		stdout: strings.NewReader(`{"type":"system","subtype":"init","session_id":"claude-s1","model":"sonnet"}
{"type":"assistant","partial":true,"message":{"content":[{"type":"text","text":"partial "}]}}
{"type":"system","subtype":"retry","attempt":2}
{"type":"result","subtype":"success","session_id":"claude-s1","result":"final answer","usage":{"input_tokens":12,"output_tokens":8}}
`),
		stderr: strings.NewReader(""),
	}
	a, launcher := newTestAdapter(process)
	var deltas []harness.StreamDelta
	result, err := a.Execute(context.Background(), harness.ExecuteRequest{
		ProjectDir:        "/work",
		Prompt:            "implement this once",
		StageName:         "implementation",
		Model:             "sonnet",
		Stream:            true,
		PermissionProfile: harness.PermissionProfileAutoProject,
		Properties:        map[string]string{"ef": "high"},
		OnStreamDelta:     func(delta harness.StreamDelta) { deltas = append(deltas, delta) },
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.SessionID != "claude-s1" || result.Output != "final answer" || result.Usage.InputTokens != 12 || result.Usage.OutputTokens != 8 || result.Usage.ContextTokens != 20 || !result.StreamDone {
		t.Fatalf("result=%+v", result)
	}
	wantArgs := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--model", "sonnet", "--permission-mode", "acceptEdits", "--effort", "high", "implement this once"}
	if got := launcher.invocation.Args; strings.Join(got, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("args=%q want %q", got, wantArgs)
	}
	if launcher.path != "/bin/claude" || launcher.invocation.WorkingDir != "/work" || !launcher.invocation.StartNewProcessGroup {
		t.Fatalf("launch=%q %+v", launcher.path, launcher.invocation)
	}
	var gotText, gotActivity, gotSession bool
	for _, delta := range deltas {
		gotText = gotText || delta.Kind == harness.StreamKindText
		gotActivity = gotActivity || delta.Kind == harness.StreamKindActivity
		gotSession = gotSession || (delta.Kind == harness.StreamKindSession && delta.SessionID == "claude-s1")
	}
	if !gotText || !gotActivity || !gotSession {
		t.Fatalf("deltas=%+v", deltas)
	}
	status, err := a.Status(context.Background(), "claude-s1")
	if err != nil || status.State != harness.StatusCompleted {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestAdapterExecuteResumeAddsOnlyBoundNativeIDAndDoesNotRetryPrompt(t *testing.T) {
	process := &fakeProcess{stdout: strings.NewReader(`{"type":"result","subtype":"error","session_id":"claude-s1","error":"resume unavailable"}` + "\n"), stderr: strings.NewReader(""), waitErr: errors.New("exit status 1")}
	a, launcher := newTestAdapter(process)
	if _, err := a.Execute(context.Background(), harness.ExecuteRequest{Prompt: "do not resend", SessionID: "claude-s1", Model: "sonnet", PermissionProfile: harness.PermissionProfileAutoAll}); err == nil {
		t.Fatal("resume failure must be returned")
	}
	if launcher.starts != 1 {
		t.Fatalf("starts=%d want one", launcher.starts)
	}
	joined := strings.Join(launcher.invocation.Args, " ")
	if !strings.Contains(joined, "--resume claude-s1") || strings.Count(joined, "do not resend") != 1 {
		t.Fatalf("resume argv=%q", launcher.invocation.Args)
	}
}

func TestAdapterRejectsForeignHarnessSessionBeforeLaunchingClaude(t *testing.T) {
	for _, tc := range []struct {
		name, sessionID, harness string
	}{
		{name: "codex", sessionID: "thr_codex_1", harness: "Codex"},
		{name: "opencode", sessionID: "opencode:ses_1", harness: "OpenCode"},
		{name: "cursor", sessionID: "cursor:agent_1", harness: "Cursor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, launcher := newTestAdapter(&fakeProcess{})
			_, err := a.Execute(context.Background(), harness.ExecuteRequest{
				Prompt:            "do not send",
				SessionID:         tc.sessionID,
				Model:             "sonnet",
				PermissionProfile: harness.PermissionProfileAutoAll,
			})
			if err == nil || !strings.Contains(err.Error(), tc.harness+" session") {
				t.Fatalf("err=%v", err)
			}
			if launcher.starts != 0 {
				t.Fatalf("foreign session must not launch Claude: %d", launcher.starts)
			}
		})
	}
}

func TestAdapterAskFailsClosedWithoutDecisionCallback(t *testing.T) {
	process := &fakeProcess{}
	a, launcher := newTestAdapter(process)
	_, err := a.Execute(context.Background(), harness.ExecuteRequest{Prompt: "needs approval", Model: "sonnet", PermissionProfile: harness.PermissionProfileAsk})
	if !errors.Is(err, ErrAskUnsupported) {
		t.Fatalf("err=%v", err)
	}
	if launcher.starts != 0 {
		t.Fatalf("ask must not silently launch permissively: %d", launcher.starts)
	}
}

func TestAdapterAskAddsProvenPermissionPromptTool(t *testing.T) {
	process := &fakeProcess{stdout: strings.NewReader(`{"type":"system","subtype":"init","session_id":"claude-s1","model":"claude-sonnet-4-20250514","effort":"high"}
{"type":"result","subtype":"success","session_id":"claude-s1","result":"done"}` + "\n"), stderr: strings.NewReader("")}
	a, launcher := newTestAdapter(process)
	result, err := a.Execute(context.Background(), harness.ExecuteRequest{
		Prompt:            "needs approval",
		Model:             "sonnet",
		PermissionProfile: harness.PermissionProfileAsk,
		OnPermissionRequest: func(context.Context, harness.PermissionRequest) (harness.PermissionResponse, error) {
			return harness.PermissionResponse{Approved: true}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(launcher.invocation.Args, "--permission-prompt-tool") || !containsString(launcher.invocation.Args, PermissionPromptToolName) {
		t.Fatalf("ask args=%q", launcher.invocation.Args)
	}
	if result.NativeModel != "claude-sonnet-4-20250514" || result.EffectiveProperties["ef"] != "high" {
		t.Fatalf("result runtime metadata=%+v", result)
	}
}

func TestAdapterAskStartsExecutionScopedBridgeAndInjectsToken(t *testing.T) {
	process := &fakeProcess{stdout: strings.NewReader(`{"type":"result","subtype":"success","session_id":"claude-s1","result":"done"}` + "\n"), stderr: strings.NewReader("")}
	a, launcher := newTestAdapter(process)
	handle := &fakeBridgeHandle{}
	var config PermissionBridgeConfig
	a.PermissionBridgeStarter = PermissionBridgeStarterFunc(func(_ context.Context, got PermissionBridgeConfig) (PermissionBridgeHandle, error) {
		config = got
		return handle, nil
	})

	_, err := a.Execute(context.Background(), harness.ExecuteRequest{
		Prompt:            "needs approval",
		Model:             "sonnet",
		PermissionProfile: harness.PermissionProfileAsk,
		OnPermissionRequest: func(context.Context, harness.PermissionRequest) (harness.PermissionResponse, error) {
			return harness.PermissionResponse{Approved: false}, nil
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if config.Token != "token" || config.Request == nil {
		t.Fatalf("bridge config=%+v", config)
	}
	if containsString(launcher.invocation.Environment, PermissionTokenEnv+"=token") {
		t.Fatalf("permission token must not be inherited by Claude: %q", launcher.invocation.Environment)
	}
	handle.mu.Lock()
	closes := handle.closeCalls
	handle.mu.Unlock()
	if closes != 1 {
		t.Fatalf("bridge cleanup calls=%d want 1", closes)
	}
}

func TestAdapterAskGivesTheClaudeChildOnlyThePrivateLiveMCPBridge(t *testing.T) {
	process := &fakeProcess{stdout: strings.NewReader(`{"type":"system","subtype":"init","session_id":"claude-s1"}
{"type":"result","subtype":"success","session_id":"claude-s1","result":"done"}` + "\n"), stderr: strings.NewReader("")}
	a, launcher := newTestAdapter(process)
	handle := &fakeLiveBridgeHandle{config: permissionMCPConfig{Address: "127.0.0.1:43123", Token: "token"}}
	a.PermissionBridgeStarter = PermissionBridgeStarterFunc(func(_ context.Context, got PermissionBridgeConfig) (PermissionBridgeHandle, error) {
		if got.Token != "token" || got.Request == nil {
			t.Fatalf("bridge config=%+v", got)
		}
		return handle, nil
	})
	launcher.onStart = func(invocation Invocation) error {
		configPath := argumentValue(invocation.Args, "--mcp-config")
		if configPath == "" || !containsString(invocation.Args, "--strict-mcp-config") {
			t.Fatalf("Claude did not receive the strict private MCP config: %q", invocation.Args)
		}
		if containsString(invocation.Environment, PermissionTokenEnv+"=token") {
			t.Fatalf("Claude inherited permission token: %q", invocation.Environment)
		}
		info, err := os.Stat(configPath)
		if err != nil {
			return err
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("MCP config mode=%#o", info.Mode().Perm())
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		var config struct {
			MCPServers map[string]struct {
				Type string            `json:"type"`
				Args []string          `json:"args"`
				Env  map[string]string `json:"env"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return err
		}
		server, ok := config.MCPServers["hero_permissions"]
		if !ok || len(config.MCPServers) != 1 || server.Type != "stdio" || !reflect.DeepEqual(server.Args, []string{"internal", "claude-permission-bridge"}) || server.Env[PermissionTokenEnv] != "token" || server.Env[PermissionBridgeAddressEnv] != "127.0.0.1:43123" {
			t.Fatalf("private MCP config=%s", data)
		}
		return nil
	}

	if _, err := a.Execute(context.Background(), harness.ExecuteRequest{
		Prompt:            "needs approval",
		Model:             "sonnet",
		PermissionProfile: harness.PermissionProfileAsk,
		OnPermissionRequest: func(context.Context, harness.PermissionRequest) (harness.PermissionResponse, error) {
			return harness.PermissionResponse{Approved: true}, nil
		},
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if handle.closeCalls != 1 {
		t.Fatalf("bridge close calls=%d", handle.closeCalls)
	}
}

func TestAdapterNormalizesPermissionAndQuestionFramesThroughCallbacks(t *testing.T) {
	process := &fakeProcess{stdout: strings.NewReader(`{"type":"system","subtype":"init","session_id":"claude-s1"}
{"type":"permission","subtype":"request","request_id":"permission-1","tool_name":"Bash"}
{"type":"question","subtype":"request","request_id":"question-1","question":"Proceed?"}
{"type":"result","subtype":"success","session_id":"claude-s1","result":"done"}` + "\n"), stderr: strings.NewReader("")}
	a, _ := newTestAdapter(process)
	var permissions []harness.PermissionRequest
	var questions []harness.QuestionRequest
	result, err := a.Execute(context.Background(), harness.ExecuteRequest{
		Prompt:            "needs answers",
		Model:             "sonnet",
		Stream:            true,
		PermissionProfile: harness.PermissionProfileAutoAll,
		OnPermissionRequest: func(_ context.Context, request harness.PermissionRequest) (harness.PermissionResponse, error) {
			permissions = append(permissions, request)
			return harness.PermissionResponse{Approved: false, Reason: "no"}, nil
		},
		OnQuestionRequest: func(_ context.Context, request harness.QuestionRequest) (harness.QuestionResponse, error) {
			questions = append(questions, request)
			return harness.QuestionResponse{}, nil
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.SessionID != "claude-s1" || len(permissions) != 1 || permissions[0].ID != "permission-1" || permissions[0].SessionID != "claude-s1" || len(questions) != 1 || questions[0].ID != "question-1" || questions[0].SessionID != "claude-s1" {
		t.Fatalf("result=%+v permissions=%+v questions=%+v", result, permissions, questions)
	}
}

func TestAdapterAvailabilityUsesProbeButNeverTurnLauncher(t *testing.T) {
	process := &fakeProcess{}
	a, turns := newTestAdapter(process)
	probe := &recordedLauncher{responses: map[string]ProbeResult{
		"--version": {Stdout: "Claude Code 2.1.261"},
		"--help":    {Stdout: supportedHelp},
	}}
	a.ProbeLauncher = probe
	if err := a.IsAvailable(context.Background()); err != nil {
		t.Fatalf("IsAvailable: %v", err)
	}
	if turns.starts != 0 || len(probe.invoked) != 2 {
		t.Fatalf("turn starts=%d probes=%d", turns.starts, len(probe.invoked))
	}
}

func TestAdapterListModelsUsesLocalCatalogWithoutLaunchingClaude(t *testing.T) {
	process := &fakeProcess{}
	a, turns := newTestAdapter(process)
	models, err := a.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	for _, want := range []string{"sonnet", "opus", "haiku", "fable"} {
		if !containsString(models, want) {
			t.Fatalf("models=%v missing %q", models, want)
		}
	}
	if turns.starts != 0 {
		t.Fatalf("catalog discovery must not launch Claude: %d", turns.starts)
	}
}

func TestAdapterExecuteIncludesCacheInOccupancy(t *testing.T) {
	process := &fakeProcess{
		stdout: strings.NewReader(`{"type":"system","subtype":"init","session_id":"claude-s1","model":"sonnet"}
{"type":"result","subtype":"success","session_id":"claude-s1","result":"ok","usage":{"input_tokens":200,"output_tokens":50,"cache_read_input_tokens":5000,"cache_creation_input_tokens":100}}
`),
		stderr: strings.NewReader(""),
	}
	a, _ := newTestAdapter(process)
	result, err := a.Execute(context.Background(), harness.ExecuteRequest{
		ProjectDir:        "/work",
		Prompt:            "continue",
		Model:             "sonnet",
		Stream:            true,
		PermissionProfile: harness.PermissionProfileAutoAll,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Usage.InputTokens != 200 || result.Usage.OutputTokens != 50 || result.Usage.CacheReadTokens != 5000 || result.Usage.CacheWriteTokens != 100 {
		t.Fatalf("usage=%+v", result.Usage)
	}
	if result.Usage.Occupancy() != 5350 {
		t.Fatalf("occupancy=%d want 5350", result.Usage.Occupancy())
	}
}

func TestAdapterCancelIsIdempotent(t *testing.T) {
	process := &fakeProcess{}
	a, _ := newTestAdapter(process)
	a.setRunning("claude-s1", process)
	if err := a.Cancel(context.Background(), "claude-s1"); err != nil {
		t.Fatal(err)
	}
	if err := a.Cancel(context.Background(), "claude-s1"); err != nil {
		t.Fatal(err)
	}
	if process.interrupts != 1 {
		t.Fatalf("interrupts=%d", process.interrupts)
	}
}

func TestAdapterCancelEscalatesOnlyTheActiveClaudeProcessGroup(t *testing.T) {
	process := &fakeProcess{}
	a, _ := newTestAdapter(process)
	a.CancelGrace = time.Millisecond
	a.setRunning("claude-s1", process)

	if err := a.Cancel(context.Background(), "claude-s1"); err != nil {
		t.Fatal(err)
	}
	if err := a.Cancel(context.Background(), "claude-s1"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for process.killCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if process.interruptCount() != 1 || process.killCount() != 1 {
		t.Fatalf("interrupts=%d kills=%d", process.interruptCount(), process.killCount())
	}
}

func TestAdapterCancelClosesAskBridgeAfterInterruptedTurn(t *testing.T) {
	process := &fakeProcess{
		stdout:      strings.NewReader(`{"type":"system","subtype":"init","session_id":"claude-s1"}` + "\n" + `{"type":"result","subtype":"success","session_id":"claude-s1","result":"done"}` + "\n"),
		stderr:      strings.NewReader(""),
		waitGate:    make(chan struct{}),
		waitStarted: make(chan struct{}),
	}
	a, _ := newTestAdapter(process)
	bridge := &fakeBridgeHandle{}
	a.PermissionBridgeStarter = PermissionBridgeStarterFunc(func(context.Context, PermissionBridgeConfig) (PermissionBridgeHandle, error) {
		return bridge, nil
	})
	done := make(chan error, 1)
	go func() {
		_, err := a.Execute(context.Background(), harness.ExecuteRequest{
			Prompt:            "wait for permission bridge cleanup",
			SessionID:         "claude-s1",
			Model:             "sonnet",
			PermissionProfile: harness.PermissionProfileAsk,
			OnPermissionRequest: func(context.Context, harness.PermissionRequest) (harness.PermissionResponse, error) {
				return harness.PermissionResponse{Approved: false}, nil
			},
		})
		done <- err
	}()
	select {
	case <-process.waitStarted:
	case <-time.After(time.Second):
		t.Fatal("Claude process did not reach Wait")
	}
	if err := a.Cancel(context.Background(), "claude-s1"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("interrupted Claude turn did not return")
	}
	bridge.mu.Lock()
	closes := bridge.closeCalls
	bridge.mu.Unlock()
	if closes != 1 {
		t.Fatalf("bridge close calls=%d want 1", closes)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func argumentValue(arguments []string, flag string) string {
	for i, argument := range arguments {
		if argument == flag && i+1 < len(arguments) {
			return arguments[i+1]
		}
	}
	return ""
}

type fakeLiveBridgeHandle struct {
	config     permissionMCPConfig
	closeCalls int
}

func (b *fakeLiveBridgeHandle) Run(context.Context) error { return nil }
func (b *fakeLiveBridgeHandle) Close() error {
	b.closeCalls++
	return nil
}
func (b *fakeLiveBridgeHandle) permissionMCPConfig() permissionMCPConfig { return b.config }
