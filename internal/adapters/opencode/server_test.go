package opencode

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// freeTCPPort returns a port that is free at call time. Tests use it instead of
// the well-known opencode default 4096 so a live developer serve cannot be
// matched by the registry-URL reaping fallback.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestIsManagedOpenCodeServeInvalidPID(t *testing.T) {
	if IsManagedOpenCodeServe(0) {
		t.Fatal("expected false for pid 0")
	}
	if IsManagedOpenCodeServe(-1) {
		t.Fatal("expected false for negative pid")
	}
	if IsManagedOpenCodeServe(999999999) {
		t.Fatal("expected false for nonexistent pid")
	}
}

func TestReapOrphanServersClearsRegistry(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	port := freeTCPPort(t)
	if _, err := st.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:     adapterName,
		PID:         999999999,
		Port:        port,
		URL:         fmt.Sprintf("http://127.0.0.1:%d", port),
		ProjectPath: dir,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	if err := ReapOrphanServers(context.Background(), dir, st); err != nil {
		t.Fatal(err)
	}
	entries, err := st.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty registry, got %v", entries)
	}
}

func TestPruneStaleServeRegistryRemovesDeadPID(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	port := freeTCPPort(t)
	if _, err := st.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:     adapterName,
		PID:         999999999,
		Port:        port,
		URL:         fmt.Sprintf("http://127.0.0.1:%d", port),
		ProjectPath: dir,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	if err := PruneStaleServeRegistry(context.Background(), dir, st); err != nil {
		t.Fatal(err)
	}
	entries, err := st.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected pruned registry, got %v", entries)
	}
}

func TestStopServeStateClearsRegistry(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	a := NewAdapter(dir, st)
	a.mu.Lock()
	a.servePID = 999999999
	a.baseURL = "http://127.0.0.1:1"
	a.mu.Unlock()

	if _, err := st.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:     adapterName,
		PID:         999999999,
		Port:        1,
		URL:         "http://127.0.0.1:1",
		ProjectPath: dir,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	if err := a.StopServe(context.Background()); err != nil {
		t.Fatal(err)
	}
	entries, err := st.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected cleared registry, got %v", entries)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.baseURL != "" || a.servePID != 0 {
		t.Fatalf("expected cleared adapter state, baseURL=%q pid=%d", a.baseURL, a.servePID)
	}
}

func TestRegisterServePersistsProjectPath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	registerServe(st, dir, 4242, 8080, "http://127.0.0.1:8080")
	entries, err := st.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries=%v", entries)
	}
	if entries[0].ProjectPath != dir {
		t.Fatalf("project_path=%q want %q", entries[0].ProjectPath, dir)
	}
}

func TestPathWithinProject(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "pkg", "x")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	tests := []struct {
		name    string
		cwd     string
		project string
		want    bool
	}{
		{"same directory", root, root, true},
		{"nested directory", nested, root, true},
		{"different project", other, root, false},
		{"empty cwd", "", root, false},
		{"empty project", root, "", false},
		{"prefix is not a child", root + "x", root, false},
	}
	for _, tt := range tests {
		if got := pathWithinProject(tt.cwd, tt.project); got != tt.want {
			t.Fatalf("%s: pathWithinProject(%q, %q) = %v, want %v", tt.name, tt.cwd, tt.project, got, tt.want)
		}
	}
}
