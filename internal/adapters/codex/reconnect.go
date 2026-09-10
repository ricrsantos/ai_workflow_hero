package codex

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	connectionReconnectAttempts = 5
	connectionReconnectDelay    = 500 * time.Millisecond
	interruptedTurnContinueLimit = 2

	interruptedTurnContinuePrompt = "The Codex app-server process restarted and interrupted your in-progress turn. Continue the current task from the last completed step. Do not repeat work that is already done. Retry only an interrupted tool if it is still required, then finish."

	interruptedTurnContinueWarning = "WARNING: Codex app-server restarted during this run. Hero asked the agent to continue from the last completed step."
)

func sleepOrDone(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *Adapter) setReconnecting(v bool) {
	if a == nil {
		return
	}
	a.reconnecting.Store(v)
}

func (a *Adapter) isReconnecting() bool {
	return a != nil && a.reconnecting.Load()
}

// recoverAppServerAfterDisconnect clears a dead RPC child, starts a fresh
// app-server, and resumes the thread so the next turn/start keeps the session.
func (a *Adapter) recoverAppServerAfterDisconnect(ctx context.Context, sessionID string, req harness.ExecuteRequest) error {
	a.setReconnecting(true)
	defer a.setReconnecting(false)

	_ = a.clearDeadAppServer(context.Background())
	if waitErr := sleepOrDone(ctx, connectionReconnectDelay); waitErr != nil {
		return waitErr
	}
	if err := a.ensureAppServer(ctx); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	if err := a.ResumeSession(ctx, sessionID); err != nil {
		a.mu.Lock()
		_, loaded := a.sessions[sessionID]
		a.mu.Unlock()
		if loaded {
			a.log().Debug("codex thread resume skipped after reconnect", "thread_id", sessionID, "error", err)
			return nil
		}
		return fmt.Errorf("codex thread resume after reconnect: %w", err)
	}
	if req.OnStreamDelta != nil {
		req.OnStreamDelta(harness.ConnectionReconnectedDelta(sessionID))
	}
	a.log().Info("codex app-server reconnected", "thread_id", sessionID)
	return nil
}
