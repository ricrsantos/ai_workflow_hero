package opencode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	sseReconnectAttempts = 12
	sseReconnectDelay    = 500 * time.Millisecond
)

var (
	sseIdleGrace         = harness.OpenCodeStallTimeout
	sseIdleProbeInterval = harness.HealthProbeInterval
)

var errStreamComplete = errors.New("opencode stream complete")

func isStreamDisconnect(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "eof") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connect: connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "use of closed network connection")
}

func (a *Adapter) readExecuteSSE(
	ctx context.Context,
	sessionID, projectDir string,
	req harness.ExecuteRequest,
	state *streamState,
	buf *strings.Builder,
	initialEvents io.ReadCloser,
) error {
	emitMalformed := func(payload string) {
		if req.OnStreamDelta != nil {
			req.OnStreamDelta(harness.WarningDelta(adapterName, "sse.malformed", sessionID, payload))
		}
	}
	handler := func(evt map[string]any) error {
		out := a.processSSEEvent(ctx, evt, sessionID, state, req, buf)
		if out.err != nil {
			return out.err
		}
		if out.done {
			return errStreamComplete
		}
		return nil
	}

	for attempt := 0; attempt < sseReconnectAttempts; attempt++ {
		var events io.ReadCloser
		var err error
		if attempt == 0 && initialEvents != nil {
			events = initialEvents
			initialEvents = nil
		} else {
			events, err = a.subscribeEvents(ctx, projectDir)
			if err != nil {
				if isServeConnectionError(err) || (attempt > 0 && isStreamDisconnect(err)) {
					if waitErr := sleepOrDone(ctx, sseReconnectDelay); waitErr != nil {
						return waitErr
					}
					continue
				}
				return err
			}
		}

		watchCtx, stopWatch := context.WithCancel(ctx)
		var sessionGone, sessionIdle atomic.Bool
		go a.watchSSESession(watchCtx, sessionID, projectDir, events, &sessionGone, &sessionIdle)

		readErr := readSSEEvents(ctx, events, handler, emitMalformed)
		stopWatch()
		events.Close()

		if sessionGone.Load() {
			if a.tryRecoverCompletedSession(ctx, sessionID, projectDir, req, state, buf) {
				return nil
			}
			return fmt.Errorf("opencode session %q not found", sessionID)
		}
		if sessionIdle.Load() {
			_ = a.tryRecoverCompletedSession(ctx, sessionID, projectDir, req, state, buf)
			return nil
		}

		if errors.Is(readErr, errStreamComplete) {
			return nil
		}
		if readErr != nil && !isStreamDisconnect(readErr) {
			return readErr
		}

		// Fast completion race: SSE connected after session.idle; recover from message list.
		if buf.Len() == 0 && a.tryRecoverCompletedSession(ctx, sessionID, projectDir, req, state, buf) {
			return nil
		}

		// Stream ended before session.idle/message.done — reconnect while ctx is live.
		if attempt+1 >= sseReconnectAttempts {
			if a.tryRecoverCompletedSession(ctx, sessionID, projectDir, req, state, buf) {
				return nil
			}
			if readErr != nil {
				return readErr
			}
			return fmt.Errorf("opencode event stream ended before session completed")
		}
		a.log().Warn("opencode event stream disconnected, reconnecting",
			"error", readErr, "attempt", attempt+1, "sessionID", sessionID)
		if waitErr := sleepOrDone(ctx, sseReconnectDelay); waitErr != nil {
			return waitErr
		}
	}
	return fmt.Errorf("opencode event stream ended before session completed")
}

func (a *Adapter) watchSSESession(ctx context.Context, sessionID, projectDir string, events io.Closer, gone, idle *atomic.Bool) {
	grace := sseIdleGrace
	if grace <= 0 {
		grace = harness.OpenCodeStallTimeout
	}
	interval := sseIdleProbeInterval
	if interval <= 0 {
		interval = harness.HealthProbeInterval
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(grace):
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if a.closeSSEIfSessionSettled(ctx, sessionID, projectDir, events, gone, idle) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Adapter) closeSSEIfSessionSettled(ctx context.Context, sessionID, projectDir string, events io.Closer, gone, idle *atomic.Bool) bool {
	switch a.probeSessionTurn(ctx, sessionID, projectDir) {
	case sessionTurnIdle:
		a.log().Info("opencode session idle while SSE still open; closing event stream", "sessionID", sessionID)
		if idle != nil {
			idle.Store(true)
		}
		_ = events.Close()
		return true
	case sessionTurnGone:
		a.log().Warn("opencode session gone while SSE still open", "sessionID", sessionID)
		if gone != nil {
			gone.Store(true)
		}
		_ = events.Close()
		return true
	default:
		return false
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		d = sseReconnectDelay
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
