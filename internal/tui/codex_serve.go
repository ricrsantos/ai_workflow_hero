package tui

import (
	"context"
	"log/slog"

	codexadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// stopCodexAppServerFn is injectable for tests (app-server lifecycle ADR-044).
var stopCodexAppServerFn func(context.Context, string, *store.Store, harnessmgr.Registry) error

func init() {
	stopCodexAppServerFn = defaultStopCodexAppServer
}

func defaultStopCodexAppServer(ctx context.Context, projectDir string, st *store.Store, reg harnessmgr.Registry) error {
	if projectDir == "" {
		return nil
	}
	slog.Info("stopping codex app-server")

	if reg != nil {
		if a, err := reg.Adapter("codex"); err == nil {
			if stopper, ok := a.(interface{ StopAppServer(context.Context) error }); ok {
				if err := stopper.StopAppServer(ctx); err != nil {
					slog.Warn("stop codex via registry adapter failed", "error", err)
				}
			}
		}
	}

	adapter := codexadapter.NewAdapter(projectDir, st)
	if err := adapter.StopAppServer(ctx); err != nil {
		return err
	}
	return codexadapter.ReapOrphanAppServers(ctx, projectDir, st)
}

func stopCodexAppServer(ctx context.Context, projectDir string, st *store.Store, reg harnessmgr.Registry) error {
	if stopCodexAppServerFn == nil {
		return defaultStopCodexAppServer(ctx, projectDir, st, reg)
	}
	return stopCodexAppServerFn(ctx, projectDir, st, reg)
}
