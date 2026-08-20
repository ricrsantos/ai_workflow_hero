package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestStopOpenCodeServe_Injectable(t *testing.T) {
	called := false
	prev := stopOpenCodeServeFn
	stopOpenCodeServeFn = func(ctx context.Context, projectDir string, st *store.Store, reg harnessmgr.Registry) error {
		called = true
		if projectDir != "/tmp/proj" {
			t.Fatalf("projectDir=%q", projectDir)
		}
		return nil
	}
	t.Cleanup(func() { stopOpenCodeServeFn = prev })

	if err := stopOpenCodeServe(context.Background(), "/tmp/proj", nil, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected stopOpenCodeServeFn to run")
	}
}

func TestStopOpenCodeServe_DefaultNoPanic(t *testing.T) {
	prev := stopOpenCodeServeFn
	stopOpenCodeServeFn = nil
	t.Cleanup(func() { stopOpenCodeServeFn = prev })

	if err := stopOpenCodeServe(context.Background(), "", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestStopOpenCodeServeFnError(t *testing.T) {
	prev := stopOpenCodeServeFn
	stopOpenCodeServeFn = func(context.Context, string, *store.Store, harnessmgr.Registry) error {
		return errors.New("stop failed")
	}
	t.Cleanup(func() { stopOpenCodeServeFn = prev })

	if err := stopOpenCodeServe(context.Background(), "/p", nil, nil); err == nil {
		t.Fatal("expected error")
	}
}
