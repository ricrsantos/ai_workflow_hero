package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestStopCodexAppServer_Injectable(t *testing.T) {
	called := false
	prev := stopCodexAppServerFn
	stopCodexAppServerFn = func(ctx context.Context, projectDir string, st *store.Store, reg harnessmgr.Registry) error {
		called = true
		if projectDir != "/tmp/proj" {
			t.Fatalf("projectDir=%q", projectDir)
		}
		return nil
	}
	t.Cleanup(func() { stopCodexAppServerFn = prev })

	if err := stopCodexAppServer(context.Background(), "/tmp/proj", nil, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected stopCodexAppServerFn to run")
	}
}

func TestStopCodexAppServer_DefaultNoPanic(t *testing.T) {
	prev := stopCodexAppServerFn
	stopCodexAppServerFn = nil
	t.Cleanup(func() { stopCodexAppServerFn = prev })

	if err := stopCodexAppServer(context.Background(), "", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestStopCodexAppServerFnError(t *testing.T) {
	prev := stopCodexAppServerFn
	stopCodexAppServerFn = func(context.Context, string, *store.Store, harnessmgr.Registry) error {
		return errors.New("stop failed")
	}
	t.Cleanup(func() { stopCodexAppServerFn = prev })

	if err := stopCodexAppServer(context.Background(), "/p", nil, nil); err == nil {
		t.Fatal("expected error")
	}
}
