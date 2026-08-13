package cursor_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestAdapterSatisfiesHarnessInterface(t *testing.T) {
	var _ harness.HarnessAdapter = (*cursoradapter.Adapter)(nil)
}

func TestAdapterSupportsChatWithCursorDir(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := cursoradapter.NewAdapter(dir)
	if !adapter.SupportsChat() {
		t.Fatal("expected SupportsChat true when hero-start command exists")
	}
	if adapter.Name() != "cursor" {
		t.Fatalf("name=%q want cursor", adapter.Name())
	}
}

func TestDispatchFallbackWhenAgentCLIMissing(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		CycleID:    1,
		StageName:  "research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dispatched {
		t.Fatalf("expected fallback, got %+v", res)
	}
	if res.Message == "" {
		t.Fatal("expected fallback message")
	}
}

func TestDispatchFallbackWhenChatAssetsMissing(t *testing.T) {
	dir := t.TempDir()
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) {
		return "/usr/bin/cursor-agent", nil
	}
	adapter.VerifyAgent = func(context.Context, string) error { return nil }

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{ProjectDir: dir, StageName: "qa"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dispatched {
		t.Fatalf("expected fallback without chat assets, got %+v", res)
	}
}

func TestDispatchCustomCommandFallbackMessage(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		Prompt:     "# Opsx propose\n\nDo the thing.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dispatched {
		t.Fatalf("expected fallback, got %+v", res)
	}
	want := "Dispatch unavailable; run the same command in Cursor chat."
	if res.Message != want {
		t.Fatalf("message=%q want %q", res.Message, want)
	}
}

func TestDispatchCustomCommandPassesPromptToPusher(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt := "# Custom\n\nExpanded markdown body."
	var gotPrompt string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/usr/bin/cursor-agent", nil }
	adapter.VerifyAgent = func(context.Context, string) error { return nil }
	adapter.Pusher = func(_ context.Context, _ string, req harness.DispatchRequest) (harness.DispatchResult, error) {
		gotPrompt = req.Prompt
		return harness.DispatchResult{Dispatched: true, Message: "ok"}, nil
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		Prompt:     prompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Dispatched {
		t.Fatalf("expected dispatched, got %+v", res)
	}
	if gotPrompt != prompt {
		t.Fatalf("pusher prompt=%q want %q", gotPrompt, prompt)
	}
}

func TestDispatchSuccessWhenPusherAvailable(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/usr/bin/cursor-agent", nil }
	adapter.VerifyAgent = func(context.Context, string) error { return nil }
	adapter.Pusher = func(_ context.Context, _ string, req harness.DispatchRequest) (harness.DispatchResult, error) {
		return harness.DispatchResult{
			Dispatched: true,
			Message:    "dispatched stage " + req.StageName,
		}, nil
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		StageName:  "research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Dispatched {
		t.Fatalf("expected dispatched=true, got %+v", res)
	}
}

// fakeRunner is an injectable CommandRunner for fixture-based tests.
type fakeRunner struct {
	t        *testing.T
	handlers []fakeCall
}

type fakeCall struct {
	matchArgs func(args []string) bool
	result    cursoradapter.RunResult
	err       error
	capture   *[]string
}

func (f *fakeRunner) match(args []string) (cursoradapter.RunResult, error) {
	for i, h := range f.handlers {
		if h.matchArgs == nil || h.matchArgs(args) {
			if h.capture != nil {
				cp := append([]string(nil), args...)
				*h.capture = cp
			}
			// consume one-shot handlers when matchArgs set
			if h.matchArgs != nil {
				f.handlers = append(f.handlers[:i], f.handlers[i+1:]...)
			}
			return h.result, h.err
		}
	}
	f.t.Fatalf("unexpected Run args: %v", args)
	return cursoradapter.RunResult{}, errors.New("unexpected")
}

func (f *fakeRunner) Run(_ context.Context, _ string, _ string, args []string) (cursoradapter.RunResult, error) {
	return f.match(args)
}

// RunStreaming writes stdout to dest line-by-line so ParseStreamJSON can emit live deltas.
func (f *fakeRunner) RunStreaming(_ context.Context, _ string, _ string, args []string, stdoutDest io.Writer) (cursoradapter.RunResult, error) {
	res, err := f.match(args)
	if stdoutDest != nil && len(res.Stdout) > 0 {
		for _, line := range bytes.SplitAfter(res.Stdout, []byte("\n")) {
			if len(line) == 0 {
				continue
			}
			if _, werr := stdoutDest.Write(line); werr != nil {
				return res, werr
			}
		}
	}
	return res, err
}

func withCursorAssets(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolveAgentCLIPrefersCursorAgent(t *testing.T) {
	spec, err := cursoradapter.ResolveAgentCLI(func(file string) (string, error) {
		if file == cursoradapter.AgentCLI {
			return "/bin/cursor-agent", nil
		}
		return "", errors.New("nope")
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Path != "/bin/cursor-agent" || len(spec.Base) != 0 {
		t.Fatalf("spec=%+v", spec)
	}
}

func TestResolveAgentCLIFallsBackToCursorAgentSubcommand(t *testing.T) {
	spec, err := cursoradapter.ResolveAgentCLI(func(file string) (string, error) {
		if file == cursoradapter.CursorCLI {
			return "/bin/cursor", nil
		}
		return "", errors.New("nope")
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Path != "/bin/cursor" || len(spec.Base) != 1 || spec.Base[0] != "agent" {
		t.Fatalf("spec=%+v", spec)
	}
}

func TestIsAvailableMissingCLI(t *testing.T) {
	adapter := cursoradapter.NewAdapter(t.TempDir())
	adapter.LookPath = func(string) (string, error) { return "", errors.New("missing") }
	err := adapter.IsAvailable(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("err=%v", err)
	}
}

func TestIsAvailableAuthFailure(t *testing.T) {
	adapter := cursoradapter.NewAdapter(t.TempDir())
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{
		{
			matchArgs: func(args []string) bool { return len(args) > 0 && args[0] == "--version" },
			result:    cursoradapter.RunResult{Stdout: []byte("2026.1.0\n")},
		},
		{
			matchArgs: func(args []string) bool { return len(args) > 0 && args[0] == "status" },
			result:    cursoradapter.RunResult{Stderr: []byte("Error: not logged in\n")},
			err:       errors.New("exit 1"),
		},
	}}
	err := adapter.IsAvailable(context.Background())
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), cursoradapter.LoginHint) {
		t.Fatalf("err=%v", err)
	}
}

func TestExecuteJSONFixture(t *testing.T) {
	dir := withCursorAssets(t)
	fixture := `{"type":"result","subtype":"success","is_error":false,"duration_ms":42,"result":"Planning done.","session_id":"sess-abc","usage":{"inputTokens":10,"outputTokens":3}}`
	var gotArgs []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool {
			return containsArg(args, "--print") && containsArg(args, "json")
		},
		result:  cursoradapter.RunResult{Stdout: []byte(fixture)},
		capture: &gotArgs,
	}}}

	res, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		ProjectDir: dir,
		Prompt:     "plan this",
		StageName:  "planning",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "sess-abc" || res.Output != "Planning done." {
		t.Fatalf("result=%+v", res)
	}
	if res.Usage.InputTokens != 10 || res.Usage.OutputTokens != 3 {
		t.Fatalf("usage=%+v", res.Usage)
	}
	if !containsArg(gotArgs, "--print") || !containsArg(gotArgs, "--output-format") {
		t.Fatalf("args=%v", gotArgs)
	}
	if !containsArg(gotArgs, "--trust") {
		t.Fatalf("expected --trust in args=%v", gotArgs)
	}
	if !containsArg(gotArgs, "--force") {
		t.Fatalf("expected --force in args=%v", gotArgs)
	}
	if !containsArg(gotArgs, "--workspace") || !containsArg(gotArgs, dir) {
		t.Fatalf("expected --workspace %s in args=%v", dir, gotArgs)
	}
	if !containsArg(gotArgs, "plan this") {
		t.Fatalf("prompt missing in args=%v", gotArgs)
	}
}

func TestExecutePassesModelFlag(t *testing.T) {
	dir := withCursorAssets(t)
	fixture := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"ok","session_id":"s-model"}`
	var gotArgs []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return true },
		result:    cursoradapter.RunResult{Stdout: []byte(fixture)},
		capture:   &gotArgs,
	}}}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "hi",
		Model:  "composer-2.5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsArg(gotArgs, "--model") || !containsArg(gotArgs, "composer-2.5") {
		t.Fatalf("expected --model composer-2.5, args=%v", gotArgs)
	}
}

func TestExecutePassesModePlanFlag(t *testing.T) {
	dir := withCursorAssets(t)
	fixture := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"ok","session_id":"s-mode"}`
	var gotArgs []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return true },
		result:    cursoradapter.RunResult{Stdout: []byte(fixture)},
		capture:   &gotArgs,
	}}}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "plan this",
		Model:  "composer-2.5",
		Mode:   harness.ModePlan,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsArg(gotArgs, "--model") || !containsArg(gotArgs, "composer-2.5") {
		t.Fatalf("expected --model, args=%v", gotArgs)
	}
	if !containsArg(gotArgs, "--mode") || !containsArg(gotArgs, "plan") {
		t.Fatalf("expected --mode plan, args=%v", gotArgs)
	}
}

func TestExecuteOmitsModeForBuild(t *testing.T) {
	dir := withCursorAssets(t)
	fixture := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"ok","session_id":"s-build"}`
	var gotArgs []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return true },
		result:    cursoradapter.RunResult{Stdout: []byte(fixture)},
		capture:   &gotArgs,
	}}}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "build this",
		Mode:   harness.ModeBuild,
	})
	if err != nil {
		t.Fatal(err)
	}
	if containsArg(gotArgs, "--mode") {
		t.Fatalf("build mode must omit --mode, args=%v", gotArgs)
	}
}

func TestExecuteResumeFlag(t *testing.T) {
	dir := withCursorAssets(t)
	fixture := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"ok","session_id":"sess-1"}`
	var gotArgs []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return true },
		result:    cursoradapter.RunResult{Stdout: []byte(fixture)},
		capture:   &gotArgs,
	}}}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt:    "continue",
		SessionID: "sess-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsArg(gotArgs, "--resume=sess-1") {
		t.Fatalf("expected resume flag, args=%v", gotArgs)
	}
}

func TestExecuteStreamJSONFixture(t *testing.T) {
	dir := withCursorAssets(t)
	var stream bytes.Buffer
	stream.WriteString(`{"type":"system","subtype":"init","session_id":"s1"}` + "\n")
	stream.WriteString(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello "}]},"session_id":"s1"}` + "\n")
	stream.WriteString(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"world"}]},"session_id":"s1"}` + "\n")
	stream.WriteString(`{"type":"result","subtype":"success","is_error":false,"duration_ms":9,"result":"Hello world","session_id":"s1"}` + "\n")

	var gotArgs []string
	var deltas []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return containsArg(args, "stream-json") },
		result:    cursoradapter.RunResult{Stdout: stream.Bytes()},
		capture:   &gotArgs,
	}}}

	res, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "hi",
		Stream: true,
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText || d.Kind == "" {
				deltas = append(deltas, d.Text)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "Hello world" || res.SessionID != "s1" || !res.StreamDone {
		t.Fatalf("result=%+v", res)
	}
	if strings.Join(deltas, "") != "Hello world" {
		t.Fatalf("deltas=%v", deltas)
	}
	if !containsArg(gotArgs, "--stream-partial-output") {
		t.Fatalf("expected --stream-partial-output, args=%v", gotArgs)
	}
}

func TestExecuteStreamDeltasArriveBeforeProcessEnds(t *testing.T) {
	dir := withCursorAssets(t)
	line1 := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"live"}]},"session_id":"s2"}` + "\n"
	line2 := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"live","session_id":"s2"}` + "\n"

	var processDone atomic.Bool
	var sawLiveDelta atomic.Bool
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &slowStreamRunner{
		t: t,
		stdout: []byte(line1 + line2),
		betweenLines: 20 * time.Millisecond,
		onFinished: func() { processDone.Store(true) },
	}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "hi",
		Stream: true,
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Text == "live" && !processDone.Load() {
				sawLiveDelta.Store(true)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawLiveDelta.Load() {
		t.Fatal("expected OnStreamDelta before process finished writing")
	}
}

// slowStreamRunner writes NDJSON lines with a delay so live parse can observe mid-run deltas.
type slowStreamRunner struct {
	t            *testing.T
	stdout       []byte
	betweenLines time.Duration
	onFinished   func()
}

func (s *slowStreamRunner) Run(context.Context, string, string, []string) (cursoradapter.RunResult, error) {
	s.t.Fatal("Run should not be called when RunStreaming is available")
	return cursoradapter.RunResult{}, errors.New("unexpected Run")
}

func (s *slowStreamRunner) RunStreaming(_ context.Context, _ string, _ string, _ []string, stdoutDest io.Writer) (cursoradapter.RunResult, error) {
	for i, line := range bytes.SplitAfter(s.stdout, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		if i > 0 && s.betweenLines > 0 {
			time.Sleep(s.betweenLines)
		}
		if stdoutDest != nil {
			if _, err := stdoutDest.Write(line); err != nil {
				return cursoradapter.RunResult{Stdout: s.stdout}, err
			}
		}
	}
	if s.onFinished != nil {
		s.onFinished()
	}
	return cursoradapter.RunResult{Stdout: s.stdout}, nil
}

func TestDispatchDefaultPusherUsesExecute(t *testing.T) {
	dir := withCursorAssets(t)
	fixture := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"stage output","session_id":"s9"}`
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.VerifyAgent = func(context.Context, string) error { return nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return containsArg(args, "--print") },
		result:    cursoradapter.RunResult{Stdout: []byte(fixture)},
	}}}
	// Pusher left nil → default Execute path.

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		StageName:  "research",
		Prompt:     "do research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Dispatched {
		t.Fatalf("expected dispatched via default pusher, got %+v", res)
	}
	if !strings.Contains(res.Message, "stage output") {
		t.Fatalf("message=%q", res.Message)
	}
}

func TestDispatchDefaultPusherPassesModelAndMode(t *testing.T) {
	dir := withCursorAssets(t)
	fixture := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"ok","session_id":"s9"}`
	var captured []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.VerifyAgent = func(context.Context, string) error { return nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return containsArg(args, "--print") },
		result:    cursoradapter.RunResult{Stdout: []byte(fixture)},
		capture:   &captured,
	}}}

	_, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		StageName:  "research",
		Prompt:     "do research",
		Model:      "composer-2.5",
		Mode:       harness.ModePlan,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsArg(captured, "--model") || !containsArg(captured, "composer-2.5") {
		t.Fatalf("expected --model composer-2.5 in args: %v", captured)
	}
	if !containsArg(captured, "--mode") || !containsArg(captured, "plan") {
		t.Fatalf("expected --mode plan in args: %v", captured)
	}
}

func TestDispatchDoesNotResumePriorSession(t *testing.T) {
	dir := withCursorAssets(t)
	fixture1 := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"first","session_id":"sess-prior"}`
	fixture2 := `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"second","session_id":"sess-fresh"}`
	var secondArgs []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.VerifyAgent = func(context.Context, string) error { return nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{
		{
			matchArgs: func(args []string) bool { return true },
			result:    cursoradapter.RunResult{Stdout: []byte(fixture1)},
		},
		{
			matchArgs: func(args []string) bool { return true },
			result:    cursoradapter.RunResult{Stdout: []byte(fixture2)},
			capture:   &secondArgs,
		},
	}}

	if _, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt:    "chat",
		SessionID: "sess-prior",
	}); err != nil {
		t.Fatal(err)
	}
	// Arm resume via ResumeSession (legacy path) — Dispatch must still stay fresh.
	if err := adapter.ResumeSession(context.Background(), "sess-prior"); err != nil {
		t.Fatal(err)
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		Prompt:     "full hero-sync prompt body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Dispatched {
		t.Fatalf("expected dispatch: %+v", res)
	}
	for _, a := range secondArgs {
		if strings.HasPrefix(a, "--resume=") {
			t.Fatalf("Dispatch must not resume prior session, args=%v", secondArgs)
		}
	}
}

func TestCancelEmptySessionIDCancelsInFlight(t *testing.T) {
	dir := withCursorAssets(t)
	started := make(chan struct{})
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = blockingStreamRunner{started: started}

	errCh := make(chan error, 1)
	go func() {
		_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
			ProjectDir: dir,
			Prompt:     "start",
			Stream:     true,
		})
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not start")
	}
	if err := adapter.Cancel(context.Background(), ""); err != nil {
		t.Fatalf("Cancel empty id: %v", err)
	}
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected execute cancelled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not return after cancel")
	}
}

type blockingStreamRunner struct {
	started chan struct{}
}

func (b blockingStreamRunner) Run(ctx context.Context, dir, path string, args []string) (cursoradapter.RunResult, error) {
	return b.RunStreaming(ctx, dir, path, args, nil)
}

func (b blockingStreamRunner) RunStreaming(ctx context.Context, _, _ string, _ []string, _ io.Writer) (cursoradapter.RunResult, error) {
	if b.started != nil {
		close(b.started)
	}
	<-ctx.Done()
	return cursoradapter.RunResult{ExitCode: -1}, ctx.Err()
}

func TestCreateResumeCancelStatus(t *testing.T) {
	adapter := cursoradapter.NewAdapter(t.TempDir())
	sess, err := adapter.CreateSession(context.Background(), harness.SessionRequest{StageName: "research"})
	if err != nil || sess == nil {
		t.Fatalf("CreateSession: %v %+v", err, sess)
	}
	if err := adapter.ResumeSession(context.Background(), "sess-x"); err != nil {
		t.Fatal(err)
	}
	st, err := adapter.Status(context.Background(), "sess-x")
	if err != nil || st.State != harness.StatusIdle {
		t.Fatalf("status=%+v err=%v", st, err)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
