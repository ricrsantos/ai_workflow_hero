package codex

import (
	"context"
	"errors"
	"testing"
)

func TestCheckHealthNotRunning(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	health, err := a.CheckHealth(context.Background(), "")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if health.ProcessAlive || health.ServerAlive {
		t.Fatalf("expected not running, got %+v", health)
	}
	if health.Details != "codex app-server not running" {
		t.Fatalf("details=%q", health.Details)
	}
}

func TestCheckHealthDoesNotSpawn(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "/mock/codex", nil }
	a.Runner = failStartHealthRunner{}
	health, err := a.CheckHealth(context.Background(), "thread-1")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if health.ProcessAlive {
		t.Fatal("CheckHealth must not start app-server")
	}
}

type failStartHealthRunner struct{}

func (failStartHealthRunner) Start(context.Context, string, string, ...string) (StdioHandle, error) {
	return nil, errors.New("CheckHealth must not spawn codex app-server")
}
