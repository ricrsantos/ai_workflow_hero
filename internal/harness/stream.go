package harness

import (
	"fmt"
	"strings"
)

// StreamKind classifies a live harness stream event for TUI display.
type StreamKind string

const (
	StreamKindText       StreamKind = "text"
	StreamKindThinking   StreamKind = "thinking"
	StreamKindTool       StreamKind = "tool"
	StreamKindWarning    StreamKind = "warning"
	StreamKindPermission StreamKind = "permission"
	StreamKindActivity   StreamKind = "activity"
	StreamKindSession    StreamKind = "session"
)

// StreamPhase marks lifecycle events on StreamDelta.
const (
	StreamPhaseStarted   = "started"
	StreamPhaseCompleted = "completed"
)

// Session state metadata values for StreamKindSession.
const (
	SessionStateIdle    = "idle"
	SessionStateFailed  = "failed"
	SessionStateRunning = "running"
)

// StreamDelta is a live event emitted during Execute when Stream is true.
type StreamDelta struct {
	Kind        StreamKind
	Text        string
	AgentName   string // Hero agent id (qa_agent) or empty for the parent session
	Model       string // kebab model slug when known
	CallID      string // Task call_id when attributed to a subagent
	Phase       string // StreamPhaseStarted / StreamPhaseCompleted, or empty
	HarnessType string // raw harness event type (permission.asked, tool_call, …)
	SessionID   string
	Metadata    map[string]string
}

// PermissionRequest is a harness-native approval prompt (tool/shell access).
type PermissionRequest struct {
	ID          string
	Title       string
	Description string
	HarnessType string
	SessionID   string
}

// PermissionResponse is the user's answer to a harness permission prompt.
type PermissionResponse struct {
	Approved bool
	Reason   string
}

// WarningDelta builds a normalized warning for unrecognized or malformed harness events.
func WarningDelta(harnessName, eventType, sessionID, payload string) StreamDelta {
	text := fmt.Sprintf("WARNING Harness event not recognized\nharness: %s\nevent: %s\nsession: %s\npayload: %s",
		strings.TrimSpace(harnessName),
		strings.TrimSpace(eventType),
		strings.TrimSpace(sessionID),
		truncatePayload(payload, 400),
	)
	return StreamDelta{
		Kind:        StreamKindWarning,
		Text:        text,
		HarnessType: eventType,
		SessionID:   sessionID,
	}
}

// SessionDelta builds a session lifecycle event.
func SessionDelta(state, message, harnessType, sessionID string) StreamDelta {
	return StreamDelta{
		Kind:        StreamKindSession,
		Text:        strings.TrimSpace(message),
		HarnessType: harnessType,
		SessionID:   sessionID,
		Metadata:    map[string]string{"state": state},
	}
}

// ActivityDelta builds an observability event (file edits, todos, LSP, …).
func ActivityDelta(harnessType, summary, sessionID string) StreamDelta {
	return StreamDelta{
		Kind:        StreamKindActivity,
		Text:        strings.TrimSpace(summary),
		HarnessType: harnessType,
		SessionID:   sessionID,
	}
}

func truncatePayload(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
