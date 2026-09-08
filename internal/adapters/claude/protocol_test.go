package claude

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type recordedLauncher struct {
	responses map[string]ProbeResult
	err       error
	invoked   []Invocation
}

func (l *recordedLauncher) Run(_ context.Context, invocation Invocation) (ProbeResult, error) {
	l.invoked = append(l.invoked, invocation.Clone())
	if l.err != nil {
		return ProbeResult{}, l.err
	}
	return l.responses[strings.Join(invocation.Args, " ")], nil
}

const supportedHelp = `
  --output-format <format>      text, json, stream-json
  --verbose                     verbose output
  --include-partial-messages    include partial messages
  --model <model>               select model
  --resume <session>            resume a session
  --permission-prompt-tool <tool> handle permission prompts
  --mcp-config <file>            load MCP config
  --strict-mcp-config            use only supplied MCP config
`

func TestProtocolGateVerifyRecordsDeterministicProbes(t *testing.T) {
	launcher := &recordedLauncher{responses: map[string]ProbeResult{
		"--version": {Stdout: "Claude Code 2.1.261\n"},
		"--help":    {Stdout: supportedHelp},
	}}

	if err := (ProtocolGate{Launcher: launcher, Logger: testLogger()}).Verify(context.Background()); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	want := []Invocation{{Args: []string{"--version"}}, {Args: []string{"--help"}}}
	if !reflect.DeepEqual(launcher.invoked, want) {
		t.Fatalf("invocations = %#v, want %#v", launcher.invoked, want)
	}
}

func TestValidateLiveAskTransportFlagsFailsClosedWithoutStrictMCP(t *testing.T) {
	err := ValidateLiveAskTransportFlags("--permission-prompt-tool --mcp-config")
	if err == nil || !strings.Contains(err.Error(), "--strict-mcp-config") {
		t.Fatalf("err=%v", err)
	}
}

func TestProtocolGateVerifyRejectsUnsupportedVersion(t *testing.T) {
	launcher := &recordedLauncher{responses: map[string]ProbeResult{
		"--version": {Stdout: "2.1.260"},
	}}

	err := (ProtocolGate{Launcher: launcher, Logger: testLogger()}).Verify(context.Background())
	if err == nil || !strings.Contains(err.Error(), "require 2.1.261") {
		t.Fatalf("Verify() error = %v, want minimum-version guidance", err)
	}
}

func TestStreamInvocationCapturesExecutionContract(t *testing.T) {
	got := StreamInvocation("/project", "sonnet")
	if !got.StartNewProcessGroup {
		t.Fatal("StartNewProcessGroup = false, want true")
	}
	if got.WorkingDir != "/project" {
		t.Fatalf("WorkingDir = %q", got.WorkingDir)
	}
	want := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--model", "sonnet"}
	if !reflect.DeepEqual(got.Args, want) {
		t.Fatalf("Args = %q, want %q", got.Args, want)
	}
}

func TestRecordedLauncherCapturesExecutionEnvironmentAndProcessGroup(t *testing.T) {
	launcher := &recordedLauncher{responses: map[string]ProbeResult{}}
	invocation := StreamInvocation("/project", "sonnet")
	invocation.Environment = []string{"CLAUDE_CONFIG_DIR=/temporary/config"}
	if _, err := launcher.Run(context.Background(), invocation); err != nil {
		t.Fatal(err)
	}
	if len(launcher.invoked) != 1 || !reflect.DeepEqual(launcher.invoked[0], invocation) {
		t.Fatalf("recorded invocation = %#v, want %#v", launcher.invoked, invocation)
	}
}

func TestDecodeNDJSONFixtureLineByLine(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "stream.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	events, err := DecodeNDJSON(file)
	if err != nil {
		t.Fatalf("DecodeNDJSON() error = %v", err)
	}
	wantTypes := []string{"system", "system", "assistant", "assistant", "assistant", "user", "system", "system", "system", "system", "system", "system", "result", "result", "system", "mystery"}
	if len(events) != len(wantTypes) {
		t.Fatalf("events = %d, want %d", len(events), len(wantTypes))
	}
	for i, want := range wantTypes {
		if events[i].Line != i+1 || events[i].Type != want || !reflect.DeepEqual([]byte(events[i].Raw), []byte(strings.TrimSpace(string(events[i].Raw)))) {
			t.Fatalf("event[%d] = %+v, want line=%d type=%q", i, events[i], i+1, want)
		}
	}
}

func TestDecodeNDJSONFixtureRejectsMalformedLine(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "malformed.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	_, err = DecodeNDJSON(file)
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("DecodeNDJSON() error = %v, want line-specific error", err)
	}
}

func TestProtocolGateVerifyAskTransport(t *testing.T) {
	launcher := &recordedLauncher{responses: map[string]ProbeResult{"--help": {Stdout: supportedHelp}}}
	request, err := os.ReadFile(filepath.Join("testdata", "permission-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	decision, err := os.ReadFile(filepath.Join("testdata", "permission-allow.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := (ProtocolGate{Launcher: launcher, Logger: testLogger()}).VerifyAskTransport(context.Background(), request, decision); err != nil {
		t.Fatalf("VerifyAskTransport() error = %v", err)
	}
}

func TestProtocolGateVerifyAskTransportFailsClosed(t *testing.T) {
	launcher := &recordedLauncher{responses: map[string]ProbeResult{"--help": {Stdout: "--output-format stream-json"}}}
	err := (ProtocolGate{Launcher: launcher, Logger: testLogger()}).VerifyAskTransport(context.Background(), []byte(`{}`), []byte(`{}`))
	if !errors.Is(err, ErrAskUnsupported) {
		t.Fatalf("VerifyAskTransport() error = %v, want ErrAskUnsupported", err)
	}
}

func TestPermissionFixtureRejectsTokenReplayAndInvalidTermination(t *testing.T) {
	request, err := os.ReadFile(filepath.Join("testdata", "permission-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePermissionToolRequest(request); err != nil {
		t.Fatalf("ValidatePermissionToolRequest() error = %v", err)
	}
	token := NewOneTimeToken("fixture-one-time-token")
	if err := token.Consume("fixture-one-time-token"); err != nil {
		t.Fatalf("Consume() first use error = %v", err)
	}
	if err := token.Consume("fixture-one-time-token"); !errors.Is(err, ErrPermissionTokenInvalid) {
		t.Fatalf("Consume() replay error = %v, want ErrPermissionTokenInvalid", err)
	}
	if err := ValidatePermissionToolRequest([]byte(`{"jsonrpc":"2.0","id":"same","method":"tools/call","params":{"name":"approval_prompt","arguments":{"token":"","tool_name":"Bash","input":{}}}}`)); err == nil {
		t.Fatal("empty replay token was accepted")
	}
	termination, err := os.ReadFile(filepath.Join("testdata", "permission-termination.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePermissionToolRequest(termination); err == nil {
		t.Fatal("termination notification was accepted as a permission request")
	}
}

func TestPermissionDecisionFixtures(t *testing.T) {
	for _, name := range []string{"permission-allow.json", "permission-deny.json"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidatePermissionToolDecision(raw); err != nil {
				t.Fatalf("ValidatePermissionToolDecision() error = %v", err)
			}
		})
	}
}
