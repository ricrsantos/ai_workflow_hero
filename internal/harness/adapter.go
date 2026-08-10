package harness

import (
	"context"
	"time"
)

// DispatchRequest describes a best-effort push of stage work into a harness.
type DispatchRequest struct {
	ProjectDir string
	CycleID    int64
	StageName  string
	Prompt     string
}

// DispatchResult is returned by HarnessAdapter.Dispatch.
type DispatchResult struct {
	Dispatched bool
	Message    string
}

// SessionRequest starts a harness execution session for a project/stage.
type SessionRequest struct {
	ProjectDir string
	StageName  string
	AgentName  string
}

// Session is a harness-owned execution session (Cursor chat id, etc.).
type Session struct {
	ID         string
	ProjectDir string
	StageName  string
	AgentName  string
	CreatedAt  time.Time
}

// ExecuteRequest is a normalized prompt execution request (design D2).
type ExecuteRequest struct {
	ProjectDir string
	Prompt     string
	SessionID  string // optional; when set, resume via harness --resume
	Stream     bool   // when true, prefer stream-json and invoke OnStreamDelta
	StageName  string
	AgentName  string
	// Model is the harness CLI model slug (e.g. composer-2.5). Empty leaves harness default.
	Model string
	// OnStreamDelta receives assistant text chunks when Stream is true (optional).
	OnStreamDelta func(delta string)
}

// Usage holds optional token counts parsed from harness output.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

// ExecutionResult is the normalized outcome of Execute.
type ExecutionResult struct {
	SessionID  string
	Output     string
	Summary    string
	Usage      Usage
	Duration   time.Duration
	StreamDone bool
}

// ExecutionStatus reports session/execution state.
type ExecutionStatus struct {
	SessionID string
	State     string // idle, running, completed, cancelled, failed
	Message   string
}

// Status state constants.
const (
	StatusIdle      = "idle"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
	StatusFailed    = "failed"
)

// HarnessAdapter abstracts IDE/harness integration (Cursor in V1).
// Full contract per design D2 / ADR-025. Dispatch remains for legacy callers.
type HarnessAdapter interface {
	Name() string
	IsAvailable(ctx context.Context) error
	CreateSession(ctx context.Context, req SessionRequest) (*Session, error)
	ResumeSession(ctx context.Context, sessionID string) error
	Execute(ctx context.Context, req ExecuteRequest) (*ExecutionResult, error)
	Cancel(ctx context.Context, sessionID string) error
	Status(ctx context.Context, sessionID string) (*ExecutionStatus, error)
	Dispatch(ctx context.Context, req DispatchRequest) (DispatchResult, error)
}
