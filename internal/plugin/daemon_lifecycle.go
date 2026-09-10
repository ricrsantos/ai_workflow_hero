package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
)

// StopTelegramDaemon requests a graceful stop from the daemon recorded in the
// private pid file. A registered TUI reconnects and starts the newly installed
// daemon. Missing or already-finished daemons are treated as a no-op.
func StopTelegramDaemon() error {
	pidPath, err := telegram.DaemonPIDPath()
	if err != nil {
		return fmt.Errorf("resolve Telegram daemon pid file: %w", err)
	}
	pluginDir, err := telegram.PluginDir(telegram.PluginName)
	if err != nil {
		return fmt.Errorf("resolve Telegram plugin directory: %w", err)
	}
	daemonPath := filepath.Join(pluginDir, telegram.DaemonBinaryName)

	var firstErr error
	stopped := false
	if data, readErr := os.ReadFile(pidPath); readErr == nil {
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr != nil || pid <= 1 {
			firstErr = fmt.Errorf("invalid Telegram daemon pid file %q", strings.TrimSpace(string(data)))
		} else if signaled, signalErr := signalDaemon(pid, daemonPath); signalErr != nil {
			firstErr = signalErr
		} else {
			stopped = signaled
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		firstErr = fmt.Errorf("read Telegram daemon pid file: %w", readErr)
	}

	// Older daemon builds did not create a pid file. Linux can still recover
	// that transition safely by matching the executable inode exposed by /proc;
	// other supported platforms simply wait for their normal daemon lifecycle.
	if runtime.GOOS == "linux" && !stopped {
		if err := stopLinuxDaemonProcesses(daemonPath); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func signalDaemon(pid int, daemonPath string) (bool, error) {
	if runtime.GOOS == "linux" {
		exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
		if err != nil {
			return false, nil
		}
		if !sameExecutable(exe, daemonPath) {
			return false, nil
		}
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, fmt.Errorf("find Telegram daemon process %d: %w", pid, err)
	}
	if err := process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return false, fmt.Errorf("stop Telegram daemon %d: %w", pid, err)
	}
	return true, nil
}

func stopLinuxDaemonProcesses(daemonPath string) error {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return fmt.Errorf("read process table: %w", err)
	}
	var firstErr error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil || !sameExecutable(exe, daemonPath) {
			continue
		}
		if _, err := signalDaemon(pid, daemonPath); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func sameExecutable(exe, expected string) bool {
	return exe == expected || exe == expected+" (deleted)"
}
