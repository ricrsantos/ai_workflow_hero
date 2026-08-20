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
	if pid <= 0 || !processAlive(pid) {
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
			return ctx.Err()
		case <-deadline.C:
			if processAlive(pid) {
				_ = proc.Kill()
			}
			return nil
		case <-ticker.C:
			if !processAlive(pid) {
				return nil
			}
		}
	}
}

// KillProcess terminates a managed opencode serve by PID (cross-platform entry).
// Deprecated: prefer TerminateManagedProcess for graceful shutdown.
func KillProcess(pid int) {
	_ = TerminateManagedProcess(context.Background(), pid)
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
	if e.PID <= 0 {
		return
	}
	if !processAlive(e.PID) {
		slog.Debug("reap stale opencode serve registry row", "pid", e.PID, "port", e.Port)
		return
	}
	if IsManagedOpenCodeServe(e.PID) {
		slog.Info("reaping orphan opencode serve", "pid", e.PID, "port", e.Port, "url", e.URL)
		if err := TerminateManagedProcess(ctx, e.PID); err != nil {
			slog.Warn("reap orphan terminate failed", "pid", e.PID, "error", err)
		}
		return
	}
	slog.Warn("registry pid is not opencode serve; skipping kill", "pid", e.PID, "port", e.Port)
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
		_ = TerminateManagedProcess(ctx, e.PID)
	}
}

func registerServe(st *store.Store, projectDir string, pid, port int, url string) {
	if st == nil {
		return
	}
	_, _ = st.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:     adapterName,
		PID:         pid,
		Port:        port,
		URL:         url,
		ProjectPath: projectDir,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	})
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
	cli, err := a.cliPath()
	if err != nil {
		return "", 0, 0, err
	}
	handle, err := a.Runner.Start(ctx, a.ProjectDir, cli, "serve", "--port", "0", "--hostname", "127.0.0.1")
	if err != nil {
		return "", 0, 0, fmt.Errorf("start opencode serve: %w", err)
	}
	resolve := a.ResolveServeURL
	if resolve == nil {
		resolve = defaultServeURLResolver
	}
	url, port, err := resolve(handle)
	if err != nil {
		_ = handle.Kill()
		return "", 0, 0, fmt.Errorf("resolve opencode serve url: %w", err)
	}
	return url, port, handle.PID(), nil
}

// stopServeState clears in-memory serve state and terminates managed processes.
func (a *Adapter) stopServeState(ctx context.Context) error {
	a.mu.Lock()
	pid := a.servePID
	a.baseURL = ""
	a.servePID = 0
	a.servePort = 0
	a.mu.Unlock()

	if pid > 0 {
		_ = TerminateManagedProcess(ctx, pid)
	}
	stopRegistryProcesses(ctx, a.Store, pid)
	if a.Store != nil {
		_ = a.Store.ClearServeRegistry()
	}
	return nil
}
