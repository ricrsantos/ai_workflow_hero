package codex

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const (
	appTerminateTimeout = 5 * time.Second
	appTerminatePoll    = 100 * time.Millisecond
	handshakeTimeout    = 30 * time.Second

	// These defaults keep the Codex app-server non-interactive while preserving
	// the workspace-write boundary. They are also sent through the app-server
	// protocol so behavior does not depend solely on project config loading.
	codexApprovalPolicy    = "never"
	codexSandboxMode       = "workspaceWrite"
	codexSandboxConfigMode = "workspace-write"
)

// ExecRunner is the default ProcessRunner using os/exec with stdio pipes.
type ExecRunner struct{}

type execStdioHandle struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (ExecRunner) Start(ctx context.Context, dir, name string, args ...string) (StdioHandle, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if attr := parentDeathSignalAttr(); attr != nil {
		cmd.SysProcAttr = attr
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	// Discard stderr so a full pipe cannot stall the child; debug logs go elsewhere.
	cmd.Stderr = io.Discard
	h := &execStdioHandle{cmd: cmd, stdin: stdin, stdout: stdout}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	return h, nil
}

func (h *execStdioHandle) PID() int           { return h.cmd.Process.Pid }
func (h *execStdioHandle) Stdin() WriteCloser { return h.stdin }
func (h *execStdioHandle) Stdout() ReadCloser { return h.stdout }
func (h *execStdioHandle) Wait() error        { return h.cmd.Wait() }
func (h *execStdioHandle) Kill() error        { return h.cmd.Process.Kill() }

// IsManagedCodexAppServer reports whether pid looks like a Hero-eligible
// `codex app-server` child. Never kills foreign processes that fail this check.
func IsManagedCodexAppServer(pid int) bool {
	if pid <= 0 || processZombie(pid) || !processAlive(pid) {
		return false
	}
	cmdline, err := processCommandLine(pid)
	if err != nil {
		return false
	}
	lower := strings.ToLower(cmdline)
	return strings.Contains(lower, "codex") && strings.Contains(lower, "app-server")
}

// TerminateManagedProcess sends SIGTERM, waits, then SIGKILL if needed.
func TerminateManagedProcess(ctx context.Context, pid int) error {
	if pid <= 0 {
		return nil
	}
	if !IsManagedCodexAppServer(pid) {
		slog.Warn("skip terminate: pid is not a managed codex app-server", "pid", pid)
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		_ = proc.Kill()
		waitProcessExit(pid)
		return nil
	}

	deadline := time.NewTimer(appTerminateTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(appTerminatePoll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = proc.Kill()
			waitProcessExit(pid)
			return ctx.Err()
		case <-deadline.C:
			if processAlive(pid) {
				_ = proc.Kill()
			}
			waitProcessExit(pid)
			return nil
		case <-ticker.C:
			if !processAlive(pid) || processZombie(pid) {
				waitProcessExit(pid)
				return nil
			}
		}
	}
}

func terminateAndReap(ctx context.Context, pid int) {
	if pid <= 0 || processZombie(pid) {
		return
	}
	_ = TerminateManagedProcess(ctx, pid)
	waitProcessExit(pid)
}

func stopProcessHandle(ctx context.Context, handle StdioHandle) error {
	if handle == nil {
		return nil
	}
	pid := handle.PID()
	if pid > 0 && IsManagedCodexAppServer(pid) {
		_ = TerminateManagedProcess(ctx, pid)
	} else {
		_ = handle.Kill()
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- handle.Wait() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-waitCh:
		return nil
	case <-time.After(appTerminateTimeout):
		return nil
	}
}

func waitProcessExit(pid int) {
	if pid <= 0 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_, _ = proc.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// ReapOrphanAppServers stops registry-recorded Codex app-server processes left
// from a prior Hero run. Never attaches to foreign processes (ADR-044).
func ReapOrphanAppServers(ctx context.Context, projectDir string, st *store.Store) error {
	if st == nil {
		return nil
	}
	entries, err := st.ListCodexServeRegistry(projectDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.PID > 0 && IsManagedCodexAppServer(e.PID) {
			slog.Info("reaping orphan codex app-server", "pid", e.PID, "project", e.ProjectPath)
			terminateAndReap(ctx, e.PID)
		} else if e.PID > 0 && processAlive(e.PID) {
			slog.Warn("registry pid is not codex app-server; skipping kill", "pid", e.PID)
		}
		_ = st.DeleteServeRegistry(e.ID)
	}
	return nil
}

func registerAppServer(st *store.Store, projectDir string, pid int) {
	if st == nil || pid <= 0 {
		return
	}
	if _, err := st.InsertCodexServeRegistry(projectDir, pid); err != nil {
		slog.Warn("register codex app-server failed", "error", err, "pid", pid)
	}
}

func clearCodexRegistry(st *store.Store, projectDir string) {
	if st == nil {
		return
	}
	entries, err := st.ListCodexServeRegistry(projectDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = st.DeleteServeRegistry(e.ID)
	}
}

// ensureAppServer lazily starts Hero-managed `codex app-server` and handshakes.
// It never attaches to a foreign process — lost stdio means start a new child.
func (a *Adapter) ensureAppServer(ctx context.Context) error {
	a.mu.Lock()
	if a.rpc != nil {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	a.startMu.Lock()
	defer a.startMu.Unlock()

	a.mu.Lock()
	if a.rpc != nil {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	// Reap orphans from a prior TUI crash, then always spawn our own child.
	_ = ReapOrphanAppServers(ctx, a.ProjectDir, a.Store)

	cli, err := a.cliPath()
	if err != nil {
		return fmt.Errorf("codex CLI not on PATH: %w", err)
	}
	runner := a.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	projectDir := canonicalProjectDir(a.ProjectDir)
	handle, err := runner.Start(ctx, projectDir, cli, appServerArgs(projectDir)...)
	if err != nil {
		a.log().Error("codex app-server start failed", "error", err)
		return fmt.Errorf("%w", &AppServerError{
			Message: "Codex app-server failed to start (incompatible or not installed).",
			Err:     err,
		})
	}

	rpc := newRPCConn(handle.Stdin(), handle.Stdout())
	hsCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	if err := a.handshake(hsCtx, rpc); err != nil {
		a.log().Error("codex app-server handshake failed", "error", err)
		_ = rpc.Close()
		_ = stopProcessHandle(context.Background(), handle)
		return fmt.Errorf("%w", &AppServerError{
			Message: "Codex app-server failed to start (incompatible or not installed).",
			Err:     err,
		})
	}

	pid := handle.PID()
	registerAppServer(a.Store, a.ProjectDir, pid)
	a.mu.Lock()
	a.appHandle = handle
	a.appPID = pid
	a.rpc = rpc
	a.mu.Unlock()
	a.log().Info("codex app-server started", "pid", pid)
	return nil
}

// canonicalProjectDir returns the absolute path Codex should use for both the
// child working directory and the project trust key. Resolving symlinks keeps
// the trust entry aligned with the path Codex discovers from its cwd.
func canonicalProjectDir(projectDir string) string {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return ""
	}

	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return filepath.Clean(projectDir)
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	return abs
}

// appServerArgs starts Codex with explicit project trust and the same
// non-interactive workspace policy as .codex/config.toml. The trust override
// must be supplied at process startup because Codex intentionally ignores
// project-scoped .codex layers until the project is trusted.
func appServerArgs(projectDir string) []string {
	args := []string{"app-server"}
	if projectDir != "" {
		args = append(args, "-c", fmt.Sprintf(
			"projects.%s.trust_level=%s",
			strconv.Quote(projectDir),
			strconv.Quote("trusted"),
		))
	}
	args = append(args,
		"-c", fmt.Sprintf("approval_policy=%s", strconv.Quote(codexApprovalPolicy)),
		"-c", fmt.Sprintf("sandbox_mode=%s", strconv.Quote(codexSandboxConfigMode)),
		"-c", "sandbox_workspace_write.network_access=true",
	)
	return args
}

func (a *Adapter) handshake(ctx context.Context, rpc *rpcConn) error {
	info := a.ClientInfo
	if info == nil {
		info = map[string]string{
			"name":    ClientName,
			"title":   ClientTitle,
			"version": ClientVersion,
		}
	}
	params := map[string]any{
		"clientInfo": info,
	}
	var result map[string]any
	if err := rpc.Call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if err := rpc.Notify("initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized: %w", err)
	}
	return nil
}

func (a *Adapter) stopAppServerState(ctx context.Context) error {
	a.mu.Lock()
	handle := a.appHandle
	pid := a.appPID
	rpc := a.rpc
	a.appHandle = nil
	a.appPID = 0
	a.rpc = nil
	a.sessions = make(map[string]*sessionState)
	a.cancels = make(map[string]context.CancelFunc)
	a.activeTurn = make(map[string]string)
	a.mu.Unlock()

	if rpc != nil {
		_ = rpc.Close()
	}
	if handle != nil {
		_ = stopProcessHandle(ctx, handle)
	} else if pid > 0 {
		terminateAndReap(ctx, pid)
	}
	clearCodexRegistry(a.Store, a.ProjectDir)
	a.log().Info("codex app-server stopped")
	return nil
}

// AppServerError is returned when the child fails to start or handshake.
type AppServerError struct {
	Message string
	Err     error
}

func (e *AppServerError) Error() string {
	// UI-C06-001 §6 — user-facing copy only; unwrap Err for diagnostics.
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = "Codex app-server failed to start (incompatible or not installed)."
	}
	return msg + "\n\n  Suggestion: verify `codex` on PATH (`codex --version`) and retry.\n  Hero does not pin a Codex CLI version."
}

func (e *AppServerError) Unwrap() error { return e.Err }
