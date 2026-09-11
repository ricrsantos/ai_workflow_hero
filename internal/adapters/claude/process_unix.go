//go:build linux || darwin

package claude

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type commandProbeLauncher struct{ path string }

func (l commandProbeLauncher) Run(ctx context.Context, invocation Invocation) (ProbeResult, error) {
	cmd := exec.CommandContext(ctx, l.path, invocation.Args...)
	cmd.Dir = invocation.WorkingDir
	if len(invocation.Environment) > 0 {
		cmd.Env = append(os.Environ(), invocation.Environment...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := ProbeResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if exit, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exit.ExitCode()
		return result, nil
	}
	return result, err
}

type execProcessLauncher struct{}

func (execProcessLauncher) Start(_ context.Context, path string, invocation Invocation) (Process, error) {
	cmd := exec.Command(path, invocation.Args...)
	cmd.Dir = invocation.WorkingDir
	if len(invocation.Environment) > 0 {
		cmd.Env = append(os.Environ(), invocation.Environment...)
	}
	if invocation.StartNewProcessGroup {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("Claude stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("Claude stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("Claude stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	return &execProcess{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

type execProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (p *execProcess) Stdout() io.Reader     { return p.stdout }
func (p *execProcess) Stderr() io.Reader     { return p.stderr }
func (p *execProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *execProcess) Wait() error           { return p.cmd.Wait() }
func (p *execProcess) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}
func (p *execProcess) Interrupt() error { return signalProcessGroup(p.PID(), syscall.SIGINT) }
func (p *execProcess) Kill() error      { return signalProcessGroup(p.PID(), syscall.SIGKILL) }

func signalProcessGroup(pid int, signal syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("Claude process is not running")
	}
	err := syscall.Kill(-pid, signal)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}

func readRedacted(r io.Reader) string {
	if r == nil {
		return ""
	}
	data, _ := io.ReadAll(io.LimitReader(r, 4096))
	text := strings.TrimSpace(string(data))
	if len(text) > 400 {
		text = text[:400]
	}
	return text
}

func classifyStartError(err error) error { return fmt.Errorf("start Claude Code: %w", err) }
func classifyExecutionError(waitErr error, stderr string) error {
	lower := strings.ToLower(stderr)
	if strings.Contains(lower, "not logged in") || strings.Contains(lower, "authentication") || strings.Contains(lower, "login") {
		return fmt.Errorf("Claude Code is not authenticated; run `claude login` in a terminal, then retry")
	}
	if strings.Contains(lower, "effort") {
		return fmt.Errorf("Claude Code rejected the selected effort property")
	}
	if stderr != "" {
		return fmt.Errorf("Claude Code exited unsuccessfully (diagnostic redacted): %w", waitErr)
	}
	return fmt.Errorf("Claude Code exited unsuccessfully: %w", waitErr)
}
