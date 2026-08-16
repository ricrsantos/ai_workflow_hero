package tui

import (
	"context"
	"log/slog"

	opencodeadapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// stopOpenCodeServeFn is injectable for tests (normal shutdown ADR-035).
var stopOpenCodeServeFn = defaultStopOpenCodeServe

func defaultStopOpenCodeServe(ctx context.Context, projectDir string, st *store.Store) error {
	if projectDir == "" {
		return nil
	}
	adapter := opencodeadapter.NewAdapter(projectDir, st)
	slog.Info("stopping opencode serve")
	return adapter.StopServe(ctx)
}

func stopOpenCodeServe(ctx context.Context, projectDir string, st *store.Store) error {
	if stopOpenCodeServeFn == nil {
		return defaultStopOpenCodeServe(ctx, projectDir, st)
	}
	return stopOpenCodeServeFn(ctx, projectDir, st)
}
