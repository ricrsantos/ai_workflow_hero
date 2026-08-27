package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// TestExecuteUsesRequestProjectDir verifies hero chat: adapter ProjectDir may
// differ from ExecuteRequest.ProjectDir (WorkDir). Session-scoped API calls
// must carry the request directory query.
func TestExecuteUsesRequestProjectDir(t *testing.T) {
	workDir := t.TempDir()
	homeDir := t.TempDir()
	encoded := url.QueryEscape(workDir)

	var mu sync.Mutex
	paths := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths[r.URL.Path+"?"+r.URL.RawQuery]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAdapter(homeDir, nil)
	a.mu.Lock()
	a.baseURL = srv.URL
	a.mu.Unlock()

	ctx := context.Background()
	if _, err := a.postProject(ctx, workDir, "/session", []byte("{}")); err != nil {
		t.Fatalf("postProject session: %v", err)
	}
	if _, err := a.postProject(ctx, workDir, "/session/sess-1/prompt_async", []byte("{}")); err != nil {
		t.Fatalf("postProject prompt_async: %v", err)
	}
	if _, err := a.getProject(ctx, workDir, "/event"); err != nil {
		t.Fatalf("getProject event: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if paths["/session?directory="+encoded] != 1 {
		t.Fatalf("POST /session directory: got paths=%v", paths)
	}
	if paths["/session/sess-1/prompt_async?directory="+encoded] != 1 {
		t.Fatalf("POST prompt_async directory: got paths=%v", paths)
	}
	if paths["/event?directory="+encoded] != 1 {
		t.Fatalf("GET /event directory: got paths=%v", paths)
	}
	for k := range paths {
		if strings.Contains(k, url.QueryEscape(homeDir)) {
			t.Fatalf("adapter home dir leaked into request: %q", k)
		}
	}
}
