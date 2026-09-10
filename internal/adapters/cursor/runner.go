package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/userpath"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// AgentCLI is the preferred Cursor Agent CLI binary name searched on PATH.
const AgentCLI = "cursor-agent"

// CursorCLI is the Cursor IDE CLI that may expose an `agent` subcommand.
const CursorCLI = "cursor"

// LoginHint is the remediation text for authentication failures.
const LoginHint = "cursor agent login"

// TrustHint explains workspace trust for non-interactive Hero harness runs.
// Hero already passes --trust on Execute; this is for rare cases where the CLI
// still blocks (e.g. path mismatch). Bare `cursor agent --trust` is not valid.
const TrustHint = "trust this folder in Cursor IDE (Hero already passes --trust on each run)"

// CommandSpec describes a resolved Cursor Agent CLI invocation.
type CommandSpec struct {
	Path string   // absolute or PATH-resolved binary
	Base []string // prefix args (e.g. ["agent"] when using `cursor agent`)
}

// RunResult is the captured stdout/stderr of a CLI invocation.
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// CommandRunner runs an external process. Injectable for unit tests (design D3).
type CommandRunner interface {
	Run(ctx context.Context, dir string, path string, args []string) (RunResult, error)
}

// StreamingCommandRunner streams stdout to stdoutDest while the process runs
// so callers can parse NDJSON deltas live. Stdout is still captured in RunResult.
type StreamingCommandRunner interface {
	CommandRunner
	RunStreaming(ctx context.Context, dir, path string, args []string, stdoutDest io.Writer) (RunResult, error)
}

// ExecCommandRunner runs real processes via os/exec.
type ExecCommandRunner struct{}

// Run implements CommandRunner.
func (ExecCommandRunner) Run(ctx context.Context, dir string, path string, args []string) (RunResult, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = userpath.AugmentPATH(os.Environ(), userpath.ExtraBinDirs()...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := RunResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
		}
		return res, err
	}
	return res, nil
}

// RunStreaming implements StreamingCommandRunner: copies stdout to stdoutDest as it arrives.
func (ExecCommandRunner) RunStreaming(ctx context.Context, dir, path string, args []string, stdoutDest io.Writer) (RunResult, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = userpath.AugmentPATH(os.Environ(), userpath.ExtraBinDirs()...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return RunResult{ExitCode: -1}, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return RunResult{Stderr: stderr.Bytes(), ExitCode: -1}, err
	}

	var captured bytes.Buffer
	writers := []io.Writer{&captured}
	if stdoutDest != nil {
		writers = append(writers, stdoutDest)
	}
	_, copyErr := io.Copy(io.MultiWriter(writers...), stdoutPipe)
	waitErr := cmd.Wait()

	res := RunResult{Stdout: captured.Bytes(), Stderr: stderr.Bytes()}
	if waitErr != nil {
		if ee, ok := waitErr.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
		}
		return res, waitErr
	}
	if copyErr != nil {
		return res, copyErr
	}
	return res, nil
}

// LookPathFunc locates an executable on PATH (defaults to exec.LookPath).
type LookPathFunc func(file string) (string, error)

// ResolveAgentCLI finds cursor-agent, then falls back to `cursor agent` (design D3).
func ResolveAgentCLI(lookPath LookPathFunc) (CommandSpec, error) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if path, err := lookPath(AgentCLI); err == nil && path != "" {
		return CommandSpec{Path: path}, nil
	}
	if path, err := lookPath(CursorCLI); err == nil && path != "" {
		return CommandSpec{Path: path, Base: []string{"agent"}}, nil
	}
	return CommandSpec{}, fmt.Errorf("cursor agent CLI not found on PATH (tried %s and %s %s); harness unavailable", AgentCLI, CursorCLI, "agent")
}

// BuildArgs prepends Base (e.g. "agent") to CLI flags/args.
func (c CommandSpec) BuildArgs(extra ...string) []string {
	out := make([]string, 0, len(c.Base)+len(extra))
	out = append(out, c.Base...)
	out = append(out, extra...)
	return out
}

// AuthError reports that the Cursor Agent CLI requires login.
type AuthError struct {
	Detail string
}

func (e *AuthError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("Cursor Agent CLI authentication required; run `%s`", LoginHint)
	}
	return fmt.Sprintf("Cursor Agent CLI authentication required (%s); run `%s`", e.Detail, LoginHint)
}

// IsAuthFailure reports whether CLI stderr (and non-JSON stdout noise) indicates
// a login requirement. NDJSON stream payloads are ignored so tool results or
// assistant text that mention `cursor agent login` cannot false-positive.
func IsAuthFailure(stdout, stderr string) bool {
	return containsAuthNeedle(stderr) || containsAuthNeedle(nonJSONOutput(stdout))
}

func containsAuthNeedle(s string) bool {
	lower := strings.ToLower(s)
	needles := []string{
		"not logged in",
		"not authenticated",
		"authentication required",
		"please log in",
		"please login",
		"auth required",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

func nonJSONOutput(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isJSONLine(trimmed) {
			continue
		}
		b.WriteString(trimmed)
		b.WriteByte('\n')
	}
	return b.String()
}

func isJSONLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "{")
}

// authFailureDetail is the user-visible AuthError.Detail: stderr first, then
// non-JSON stdout. NDJSON init/result lines are never interpolated.
func authFailureDetail(stderr, stdout string) string {
	if d := firstNonJSONLine(stderr); d != "" {
		return d
	}
	return firstNonJSONLine(stdout)
}

func firstNonJSONLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isJSONLine(line) {
			continue
		}
		return line
	}
	return ""
}

// ndjsonSessionID returns the first session_id in Cursor stream-json stdout.
func ndjsonSessionID(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !isJSONLine(line) {
			continue
		}
		var ev struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		if ev.SessionID != "" {
			return ev.SessionID
		}
	}
	return ""
}

// TrustError reports that the Cursor Agent CLI requires workspace trust.
type TrustError struct {
	Detail string
}

func (e *TrustError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("cursor agent workspace trust required; %s", TrustHint)
	}
	return fmt.Sprintf("cursor agent workspace trust required (%s); %s", e.Detail, TrustHint)
}

// IsTrustFailure reports whether CLI stderr (and non-JSON stdout noise) indicates
// workspace trust is required. NDJSON stream payloads are ignored so assistant
// or tool text that mentions "workspace trust" cannot false-positive (same
// hardening as IsAuthFailure / C9).
//
// Real Cursor trust blocks emit plain text such as "⚠ Workspace Trust Required"
// (often with exit 0), so callers must not require processFailed.
func IsTrustFailure(stdout, stderr string) bool {
	return containsTrustNeedle(stderr) || containsTrustNeedle(nonJSONOutput(stdout))
}

func containsTrustNeedle(s string) bool {
	lower := strings.ToLower(s)
	needles := []string{
		"workspace trust required",
		"workspace trust",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

// trustFailureDetail is the user-visible TrustError.Detail: stderr first, then
// non-JSON stdout. NDJSON init/result lines are never interpolated.
func trustFailureDetail(stderr, stdout string) string {
	return authFailureDetail(stderr, stdout)
}

// IsRetriableFailure reports transient Cursor Agent CLI failures that callers should retry
// (e.g. RetriableError: [resource_exhausted]).
func IsRetriableFailure(stdout, stderr string, err error) bool {
	var b strings.Builder
	b.WriteString(stdout)
	b.WriteByte('\n')
	b.WriteString(stderr)
	if err != nil {
		b.WriteByte('\n')
		b.WriteString(err.Error())
	}
	s := strings.ToLower(b.String())
	if strings.Contains(s, "resource_exhausted") {
		return true
	}
	// Match RetriableError but not NonRetriableError (the latter contains the former).
	stripped := strings.ReplaceAll(s, "nonretriableerror", "")
	return strings.Contains(stripped, "retriableerror")
}

// IsTransportFailure reports process/stdio disconnects that should retry Execute
// with the same SessionID (distinct from API RetriableError).
func IsTransportFailure(stdout, stderr string, err error) bool {
	if harness.IsConnectionClosed(err) {
		return true
	}
	var b strings.Builder
	b.WriteString(stdout)
	b.WriteByte('\n')
	b.WriteString(stderr)
	if err != nil {
		b.WriteByte('\n')
		b.WriteString(err.Error())
	}
	s := strings.ToLower(b.String())
	return strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "connection closed") ||
		strings.Contains(s, "signal: killed") ||
		strings.Contains(s, "signal: terminated")
}
