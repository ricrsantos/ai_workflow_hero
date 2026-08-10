package cursor_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func (f *fakeRunner) Run(_ context.Context, _ string, _ string, args []string) (cursoradapter.RunResult, error) {
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

	var deltas []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool { return containsArg(args, "stream-json") },
		result:    cursoradapter.RunResult{Stdout: stream.Bytes()},
	}}}

	res, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "hi",
		Stream: true,
		OnStreamDelta: func(d string) {
			deltas = append(deltas, d)
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
