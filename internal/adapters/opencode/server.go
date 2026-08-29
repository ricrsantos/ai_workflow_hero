package opencode

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const (
	serveTerminateTimeout = 5 * time.Second
	serveTerminatePoll    = 100 * time.Millisecond
	serveWatchdogInterval = 60 * time.Second
)

// IsManagedOpenCodeServe reports whether pid is alive and its command line looks
// like an opencode serve child Hero may have started.
func IsManagedOpenCodeServe(pid int) bool {
	if pid <= 0 || processZombie(pid) || !processAlive(pid) {
		return false
	}
	cmdline, err := processCommandLine(pid)
	if err != nil {
		return false
	}
	lower := strings.ToLower(cmdline)
	return strings.Contains(lower, "opencode") && strings.Contains(lower, "serve")
}

// TerminateManagedProcess sends SIGTERM, waits, then SIGKILL if needed.
// It only terminates processes that pass IsManagedOpenCodeServe.
func TerminateManagedProcess(ctx context.Context, pid int) error {
	if pid <= 0 {
		return nil
	}
	if !IsManagedOpenCodeServe(pid) {
		slog.Warn("skip terminate: pid is not a managed opencode serve", "pid", pid)
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

	deadline := time.NewTimer(serveTerminateTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(serveTerminatePoll)
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

func stopProcessHandle(ctx context.Context, handle ProcessHandle) error {
	if handle == nil {
		return nil
	}
	if err := handle.Kill(); err != nil {
		// process may already be gone
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- handle.Wait() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-waitCh:
		return nil
	case <-time.After(serveTerminateTimeout):
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

// KillProcess terminates a managed opencode serve by PID (cross-platform entry).
// Deprecated: prefer TerminateManagedProcess for graceful shutdown.
func KillProcess(pid int) {
	terminateAndReap(context.Background(), pid)
}

// ReapOrphanServers stops registry-recorded serve processes left from a prior
// Hero run and clears stale rows before normal startup continues.
func ReapOrphanServers(ctx context.Context, projectDir string, st *store.Store) error {
	if st == nil {
		return nil
	}
	entries, err := st.ListServeRegistry()
	if err != nil {
		return err
	}
	projectDir = strings.TrimSpace(projectDir)
	for _, e := range entries {
		if e.Harness != adapterName {
			continue
		}
		if projectDir != "" && strings.TrimSpace(e.ProjectPath) != "" && e.ProjectPath != projectDir {
			continue
		}
		reapEntry(ctx, e)
		_ = st.DeleteServeRegistry(e.ID)
	}
	return nil
}

func reapEntry(ctx context.Context, e store.ServeRegistryEntry) {
	if e.PID > 0 && processZombie(e.PID) {
		slog.Debug("reap zombie opencode serve registry row", "pid", e.PID, "port", e.Port)
		return
	}
	if e.PID > 0 && IsManagedOpenCodeServe(e.PID) {
		slog.Info("reaping orphan opencode serve", "pid", e.PID, "port", e.Port, "url", e.URL)
		terminateAndReap(ctx, e.PID)
		return
	}
	if strings.TrimSpace(e.URL) == "" || !serveURLAlive(ctx, e.URL, http.DefaultClient) {
		if e.PID > 0 && !processAlive(e.PID) {
			slog.Debug("reap stale opencode serve registry row", "pid", e.PID, "port", e.Port)
		}
		return
	}
	for _, pid := range listOpenCodeServePIDs() {
		if e.Port > 0 && !processListenPort(pid, e.Port) {
			continue
		}
		slog.Info("reaping orphan opencode serve by registry url", "pid", pid, "port", e.Port, "url", e.URL)
		terminateAndReap(ctx, pid)
		return
	}
	if e.PID > 0 && processAlive(e.PID) {
		slog.Warn("registry pid is not opencode serve; skipping kill", "pid", e.PID, "port", e.Port)
	}
}

// stopRegistryProcesses terminates all opencode serve rows in the registry.
func stopRegistryProcesses(ctx context.Context, st *store.Store, skipPID int) {
	if st == nil {
		return
	}
	entries, err := st.ListServeRegistry()
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Harness != adapterName || e.PID <= 0 {
			continue
		}
		if e.PID == skipPID {
			continue
		}
		terminateAndReap(ctx, e.PID)
	}
}

func registerServe(st *store.Store, projectDir string, pid, port int, url string) {
	if st == nil {
		return
	}
	if _, err := st.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:     adapterName,
		PID:         pid,
		Port:        port,
		URL:         url,
		ProjectPath: projectDir,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		slog.Warn("register opencode serve failed", "error", err, "pid", pid, "port", port)
	}
}

// PruneStaleServeRegistry removes registry rows whose PID is gone or no longer
// looks like opencode serve. It never terminates live processes (watchdog-safe).
func PruneStaleServeRegistry(ctx context.Context, projectDir string, st *store.Store) error {
	if st == nil {
		return nil
	}
	entries, err := st.ListServeRegistry()
	if err != nil {
		return err
	}
	projectDir = strings.TrimSpace(projectDir)
	for _, e := range entries {
		if e.Harness != adapterName {
			continue
		}
		if projectDir != "" && strings.TrimSpace(e.ProjectPath) != "" && e.ProjectPath != projectDir {
			continue
		}
		if e.PID <= 0 || !processAlive(e.PID) {
			_ = st.DeleteServeRegistry(e.ID)
			continue
		}
		if !IsManagedOpenCodeServe(e.PID) {
			slog.Warn("prune serve registry: pid is not opencode serve", "pid", e.PID)
			_ = st.DeleteServeRegistry(e.ID)
		}
	}
	return nil
}

// RunServeWatchdog periodically prunes inconsistent registry rows while fn runs.
// It is optional hardening for long TUI sessions (v2.4 design).
func RunServeWatchdog(ctx context.Context, projectDir string, st *store.Store, fn func() error) error {
	if st == nil {
		return fn()
	}
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(serveWatchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				if err := PruneStaleServeRegistry(watchCtx, projectDir, st); err != nil {
					slog.Warn("serve watchdog prune failed", "error", err)
				}
			}
		}
	}()

	err := fn()
	cancel()
	wg.Wait()
	return err
}

// ServeURLAlive reports whether an opencode serve base URL responds.
func ServeURLAlive(ctx context.Context, baseURL string) bool {
	return serveURLAlive(ctx, baseURL, http.DefaultClient)
}

func serveURLAlive(ctx context.Context, baseURL string, client HTTPDoer) bool {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false
	}
	if client == nil {
		client = http.DefaultClient
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/config/providers", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

// startServeProcess spawns opencode serve and returns the resolved base URL.
func (a *Adapter) startServeProcess(ctx context.Context) (baseURL string, port int, pid int, err error) {
	return a.startServeProcessWithProfile(ctx, harness.PermissionProfileAsk)
}

func (a *Adapter) startServeProcessWithProfile(ctx context.Context, profile harness.PermissionProfile) (baseURL string, port int, pid int, err error) {
	cli, err := a.cliPath()
	if err != nil {
		return "", 0, 0, err
	}
	args := []string{"serve", "--port", "0", "--hostname", "127.0.0.1"}
	env := []string{"OPENCODE_CONFIG_CONTENT=" + permissionConfigContent(profile)}
	var handle ProcessHandle
	if runner, ok := a.Runner.(EnvironmentProcessRunner); ok {
		handle, err = runner.StartWithEnv(ctx, a.ProjectDir, cli, env, args...)
	} else {
		handle, err = a.Runner.Start(ctx, a.ProjectDir, cli, args...)
	}
	if err != nil {
		return "", 0, 0, fmt.Errorf("start opencode serve: %w", err)
	}
	resolve := a.ResolveServeURL
	if resolve == nil {
		resolve = defaultServeURLResolver
	}
	url, port, err := resolve(handle)
	if err != nil {
		_ = stopProcessHandle(context.Background(), handle)
		return "", 0, 0, fmt.Errorf("resolve opencode serve url: %w", err)
	}
	pid = handle.PID()
	a.mu.Lock()
	a.baseURL = url
	a.servePID = pid
	a.servePort = port
	a.serveHandle = handle
	a.serveProfile = harness.NormalizePermissionProfile(profile)
	a.mu.Unlock()
	return url, port, pid, nil
}

// permissionConfigContent is a process-scoped OpenCode override. It takes
// precedence over opencode.json, so changing a Hero profile never rewrites a
// user's project configuration.
func permissionConfigContent(profile harness.PermissionProfile) string {
	if harness.NormalizePermissionProfile(profile) == harness.PermissionProfileAutoProject {
		return `{"permission":{"*":"allow","bash":"ask","external_directory":"ask","webfetch":"ask","websearch":"ask","mcp_*":"ask"}}`
	}
	return `{"permission":"ask"}`
}

// stopServeState clears in-memory serve state and terminates managed processes.
func (a *Adapter) stopServeState(ctx context.Context) error {
	a.mu.Lock()
	handle := a.serveHandle
	pid := a.servePID
	a.serveHandle = nil
	a.baseURL = ""
	a.servePID = 0
	a.servePort = 0
	a.serveProfile = harness.PermissionProfileAsk
	a.mu.Unlock()

	if handle != nil {
		_ = stopProcessHandle(ctx, handle)
	} else if pid > 0 {
		terminateAndReap(ctx, pid)
	}
	stopRegistryProcesses(ctx, a.Store, 0)
	if a.Store != nil {
		_ = a.Store.ClearServeRegistry()
	}
	return nil
}
