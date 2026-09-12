package harness

import (
	"context"
	"errors"
	"time"
)

// ErrExactResumeUnavailable marks a failed resume of a caller-supplied native
// session ID. Adapters MUST NOT start a replacement native session in that case.
var ErrExactResumeUnavailable = errors.New("exact harness resume unavailable")

// ExactResumeError describes a rejected exact resume attempt.
type ExactResumeError struct {
	NativeSessionID string
	Cause           error
}

func (e *ExactResumeError) Error() string {
	return ErrExactResumeUnavailable.Error()
}

func (e *ExactResumeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *ExactResumeError) Is(target error) bool {
	return target == ErrExactResumeUnavailable
}

// NewExactResumeUnavailable wraps cause as an exact-resume failure.
func NewExactResumeUnavailable(nativeSessionID string, cause error) error {
	return &ExactResumeError{NativeSessionID: nativeSessionID, Cause: cause}
}

// Normalized event types align with durable session_events.event_type (design D1).
const (
	NormalizedEventUser       = "user"
	NormalizedEventAssistant  = "assistant"
	NormalizedEventThinking   = "thinking"
	NormalizedEventTool       = "tool"
	NormalizedEventWarning    = "warning"
	NormalizedEventPermission = "permission"
	NormalizedEventQuestion   = "question"
	NormalizedEventAttachment = "attachment"
	NormalizedEventAsset      = "asset"
	NormalizedEventInterrupt  = "interruption"
	NormalizedEventNote       = "note"
)

// NormalizedEventOrigin values align with session_events.origin.
const (
	NormalizedOriginLocal    = "local"
	NormalizedOriginTelegram = "telegram"
)

// NormalizedEvent is a provider-agnostic transcript event suitable for import
// into session_events after user confirmation (ADR-097).
type NormalizedEvent struct {
	EventType       string
	Origin          string
	OriginAddress   string
	PayloadJSON     []byte
	ProviderEventID string
	CreatedAt       time.Time
}

// RemoteHistoryReader is an optional adapter capability for confirmed remote import.
type RemoteHistoryReader interface {
	SupportsRemoteHistory() bool
	ReadRemoteHistory(ctx context.Context, nativeSessionID string) ([]NormalizedEvent, error)
}

// NativeSessionDeleter is an optional adapter capability for best-effort provider purge.
type NativeSessionDeleter interface {
	SupportsNativeDelete() bool
	DeleteNativeSession(ctx context.Context, nativeSessionID string) error
}

// LiveStreamAttachRequest observes an in-progress native harness turn without
// issuing a new prompt. ProjectDir is harness-specific (OpenCode directory).
type LiveStreamAttachRequest struct {
	NativeSessionID string
	ProjectDir      string
	OnStreamDelta   func(StreamDelta)
}

// LiveStreamAttacher is an optional adapter capability for reconnecting to a
// live provider stream after TUI restart. Cursor/Codex/Claude do not implement
// this contract; OpenCode may attach via session resume and SSE.
type LiveStreamAttacher interface {
	SupportsLiveStreamAttach() bool
	AttachLiveStream(ctx context.Context, req LiveStreamAttachRequest) (*ExecutionResult, error)
}
