package tui

import (
	"context"
	"log/slog"

	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// stopOpenCodeServeFn is injectable for tests (normal shutdown ADR-035).
var stopOpenCodeServeFn func(context.Context, string, *store.Store, harnessmgr.Registry) error

func init() {
	stopOpenCodeServeFn = defaultStopOpenCodeServe
}

func defaultStopOpenCodeServe(ctx context.Context, projectDir string, st *store.Store, reg harnessmgr.Registry) error {
	if projectDir == "" {
		return nil
	}
	slog.Info("stopping opencode serve")

	if reg != nil {
		if a, err := reg.Adapter("opencode"); err == nil {
			if stopper, ok := a.(interface{ StopServe(context.Context) error }); ok {
				if err := stopper.StopServe(ctx); err != nil {
					slog.Warn("stop opencode via registry adapter failed", "error", err)
				}
			}
		}
	}

	adapter := opencodeadapter.NewAdapter(projectDir, st)
	if err := adapter.StopServe(ctx); err != nil {
		return err
	}
	return opencodeadapter.ReapOrphanServers(ctx, projectDir, st)
}

func stopOpenCodeServe(ctx context.Context, projectDir string, st *store.Store, reg harnessmgr.Registry) error {
	if stopOpenCodeServeFn == nil {
		return defaultStopOpenCodeServe(ctx, projectDir, st, reg)
	}
	return stopOpenCodeServeFn(ctx, projectDir, st, reg)
}
