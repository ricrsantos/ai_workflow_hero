package opencode

import (
	"context"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// SupportsLiveStreamAttach implements harness.LiveStreamAttacher.
func (a *Adapter) SupportsLiveStreamAttach() bool {
	return true
}

// AttachLiveStream implements harness.LiveStreamAttacher.
func (a *Adapter) AttachLiveStream(ctx context.Context, req harness.LiveStreamAttachRequest) (*harness.ExecutionResult, error) {
	sessionID := strings.TrimSpace(req.NativeSessionID)
	if sessionID == "" {
		return nil, harness.NewExactResumeUnavailable("", nil)
	}
	projectDir := strings.TrimSpace(req.ProjectDir)
	if err := a.ensureServe(ctx); err != nil {
		return nil, err
	}
	if err := a.resumeSession(ctx, sessionID, projectDir); err != nil {
		return nil, harness.NewExactResumeUnavailable(sessionID, err)
	}

	execReq := harness.ExecuteRequest{
		ProjectDir:    projectDir,
		SessionID:     sessionID,
		Stream:        true,
		OnStreamDelta: req.OnStreamDelta,
	}
	if execReq.OnStreamDelta != nil {
		execReq.OnStreamDelta(harness.SessionDelta(harness.SessionStateRunning, "", "session.bound", sessionID))
	}

	start := time.Now()
	events, err := a.subscribeEvents(ctx, projectDir)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	state := newStreamState()
	state.assetStore = newOpenCodeAssetStore(sessionID, projectDir, a.log())
	state.assetNormalizer = NewOpenCodeAssetNormalizer(OpenCodeAssetNormalizerOptions{
		SessionID:    sessionID,
		WorkspaceDir: projectDir,
	})
	if err := a.readExecuteSSE(ctx, sessionID, projectDir, execReq, state, &buf, events); err != nil {
		return nil, err
	}
	out := buf.String()
	return &harness.ExecutionResult{
		SessionID:  sessionID,
		Output:     out,
		Summary:    truncate(out, 200),
		Usage:      state.usage,
		Duration:   time.Since(start),
		StreamDone: true,
		Assets:     state.assetNormalizer.FinalAssets(),
	}, nil
}
