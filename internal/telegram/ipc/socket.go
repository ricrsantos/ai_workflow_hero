package ipc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

// Listen opens an OS-user-private unix socket at path. The parent directory is
// created 0700 and the socket is chmod'd 0600 so only the effective UID can
// connect (ADR-060). Existing sockets are removed first.
func Listen(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("ipc: mkdir run dir: %w", err)
	}
	// Remove any stale socket from a previous daemon run.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("ipc: remove stale socket: %w", err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("ipc: listen: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("ipc: chmod socket: %w", err)
	}
	return ln, nil
}

// Dial connects to the daemon socket at path.
func Dial(path string) (net.Conn, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("ipc: dial: %w", err)
	}
	return conn, nil
}

// CurrentUID returns the effective user id of this process, or -1 when unknown.
// The daemon compares a client's declared UID against this value and rejects
// mismatches (ADR-060).
func CurrentUID() int {
	if runtime.GOOS == "windows" {
		return -1
	}
	return os.Geteuid()
}
