package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
