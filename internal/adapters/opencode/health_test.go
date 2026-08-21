package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckHealthGlobalEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"healthy": true, "version": "1.2.3"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.HTTP = srv.Client()
	a.Runner = &failStartRunner{}
	a.mu.Lock()
	a.baseURL = srv.URL
	a.servePID = 0
	a.mu.Unlock()

	health, err := a.CheckHealth(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !health.ServerAlive {
		t.Fatalf("ServerAlive=false details=%q", health.Details)
	}
	if health.Details == "" {
		t.Fatal("expected details")
	}
}

func TestCheckHealthFallsBackWhenGlobalMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config/providers":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.HTTP = srv.Client()
	a.Runner = &failStartRunner{}
	a.mu.Lock()
	a.baseURL = srv.URL
	a.mu.Unlock()

	health, err := a.CheckHealth(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !health.ServerAlive {
		t.Fatalf("ServerAlive=false via fallback, details=%q", health.Details)
	}
}

func TestCheckHealthSessionReadOnlyNoSpawn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"healthy": true})
		case "/session/sess-ok":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.HTTP = srv.Client()
	a.Runner = &failStartRunner{}
	a.mu.Lock()
	a.baseURL = srv.URL
	a.mu.Unlock()

	health, err := a.CheckHealth(context.Background(), "sess-ok")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !health.SessionAlive {
		t.Fatalf("SessionAlive=false details=%q", health.Details)
	}
}

func TestCheckHealthSession404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"healthy": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.HTTP = srv.Client()
	a.Runner = &failStartRunner{}
	a.mu.Lock()
	a.baseURL = srv.URL
	a.mu.Unlock()

	health, err := a.CheckHealth(context.Background(), "missing")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if health.SessionAlive {
		t.Fatal("expected SessionAlive=false on 404")
	}
}

func TestCheckHealthSessionInconclusiveKeepsAlive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"healthy": true})
		case "/session/sess-busy":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.HTTP = srv.Client()
	a.Runner = &failStartRunner{}
	a.mu.Lock()
	a.baseURL = srv.URL
	a.mu.Unlock()

	health, err := a.CheckHealth(context.Background(), "sess-busy")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !health.SessionAlive {
		t.Fatalf("500 must not flip SessionAlive; details=%q", health.Details)
	}
	if !strings.Contains(health.Details, "inconclusive") {
		t.Fatalf("expected inconclusive detail, got %q", health.Details)
	}
}

func TestCheckHealthSessionTimeoutKeepsAlive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"healthy": true})
		case "/session/slow":
			time.Sleep(3 * time.Second)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.HTTP = srv.Client()
	a.Runner = &failStartRunner{}
	a.mu.Lock()
	a.baseURL = srv.URL
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	health, err := a.CheckHealth(ctx, "slow")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !health.SessionAlive {
		t.Fatalf("timeout must not flip SessionAlive; details=%q", health.Details)
	}
}

func TestResumeSessionNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/session/sess-err" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.HTTP = srv.Client()
	a.Runner = &failStartRunner{}
	a.mu.Lock()
	a.baseURL = srv.URL
	a.mu.Unlock()

	err := a.ResumeSession(context.Background(), "sess-err")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error=%v", err)
	}
}

// failStartRunner fails if CheckHealth accidentally tries to spawn serve.
type failStartRunner struct{}

func (failStartRunner) Start(context.Context, string, string, ...string) (ProcessHandle, error) {
	return nil, errors.New("CheckHealth must not spawn opencode serve")
}
