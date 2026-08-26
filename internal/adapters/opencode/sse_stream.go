package opencode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	sseReconnectAttempts = 12
	sseReconnectDelay    = 500 * time.Millisecond
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

		readErr := readSSEEvents(ctx, events, handler, emitMalformed)
		events.Close()

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
