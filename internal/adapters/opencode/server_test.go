package opencode

import (
	"context"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

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

	if _, err := st.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:     adapterName,
		PID:         999999999,
		Port:        4096,
		URL:         "http://127.0.0.1:4096",
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

	if _, err := st.InsertServeRegistry(store.ServeRegistryEntry{
		Harness:     adapterName,
		PID:         999999999,
		Port:        4096,
		URL:         "http://127.0.0.1:4096",
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
