package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestEnsureServeRegistersWithLiveStore(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	oc := NewAdapter(dir, st)
	oc.LookPath = func(string) (string, error) { return "opencode", nil }
	oc.Runner = &stubRunner{}
	oc.HTTP = srv.Client()
	oc.ResolveServeURL = func(ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	if err := oc.ensureServe(context.Background()); err != nil {
		t.Fatal(err)
	}

	entries, err := st.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("registry entries=%v want 1", entries)
	}
	if entries[0].PID != 4242 {
		t.Fatalf("pid=%d want 4242", entries[0].PID)
	}
	if entries[0].ProjectPath != dir {
		t.Fatalf("project_path=%q", entries[0].ProjectPath)
	}

	if err := oc.StopServe(context.Background()); err != nil {
		t.Fatal(err)
	}
	entries, err = st.ListServeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected cleared registry after stop, got %v", entries)
	}
}
