package cursor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// AgentCLI is the preferred Cursor Agent CLI binary name searched on PATH.
const AgentCLI = "cursor-agent"

// CursorCLI is the Cursor IDE CLI that may expose an `agent` subcommand.
const CursorCLI = "cursor"

// LoginHint is the remediation text for authentication failures.
const LoginHint = "cursor agent login"

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

// ExecCommandRunner runs real processes via os/exec.
type ExecCommandRunner struct{}

// Run implements CommandRunner.
func (ExecCommandRunner) Run(ctx context.Context, dir string, path string, args []string) (RunResult, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	if dir != "" {
		cmd.Dir = dir
	}
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

// IsAuthFailure reports whether stderr/stdout indicates a login requirement.
func IsAuthFailure(stdout, stderr string) bool {
	combined := strings.ToLower(stdout + "\n" + stderr)
	needles := []string{
		"not logged in",
		"not authenticated",
		"authentication required",
		"please log in",
		"please login",
		"run `cursor agent login`",
		"cursor agent login",
		"unauthorized",
		"auth required",
	}
	for _, n := range needles {
		if strings.Contains(combined, n) {
			return true
		}
	}
	return false
}
