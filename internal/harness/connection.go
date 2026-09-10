package harness

import (
	"errors"
	"strings"
)

// ErrConnectionClosed marks a harness transport drop that adapters may retry
// while preserving the in-progress SessionID.
var ErrConnectionClosed = errors.New("connection closed")

// Stream HarnessType values for disconnect / reconnect UX (TUI status + transcript).
const (
	ConnectionClosedHarnessType      = "connection.closed"
	ConnectionReconnectedHarnessType = "connection.reconnected"
)

const (
	connectionClosedWarningText = "connection closed; reconnecting…"
	connectionReconnectedText   = "harness reconnected"
)

// IsConnectionClosed reports whether err is a transport disconnect that may be
// recovered by restarting the harness connection and resuming the session.
func IsConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrConnectionClosed) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection closed") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "connect: connection refused") ||
		strings.Contains(msg, "connection refused")
}

// ConnectionClosedDelta is the user-visible disconnect warning during reconnect.
func ConnectionClosedDelta(sessionID string) StreamDelta {
	return StreamDelta{
		Kind:        StreamKindWarning,
		Text:        "⚠ " + connectionClosedWarningText,
		HarnessType: ConnectionClosedHarnessType,
		SessionID:   sessionID,
	}
}

// ConnectionReconnectedDelta is the user-visible recovery notice.
func ConnectionReconnectedDelta(sessionID string) StreamDelta {
	return StreamDelta{
		Kind:        StreamKindWarning,
		Text:        "✓ " + connectionReconnectedText,
		HarnessType: ConnectionReconnectedHarnessType,
		SessionID:   sessionID,
	}
}

// IsConnectionLifecycleDelta reports disconnect/reconnect stream events.
func IsConnectionLifecycleDelta(d StreamDelta) bool {
	switch strings.TrimSpace(d.HarnessType) {
	case ConnectionClosedHarnessType, ConnectionReconnectedHarnessType:
		return true
	default:
		return false
	}
}
