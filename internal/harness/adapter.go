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
	// Model is the harness CLI model slug (e.g. composer-2.5). Empty leaves harness default.
	Model string
	// Mode is the agent mode: "build" (default/agent) or "plan". Empty means build.
	Mode string
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
	// Mode is the agent mode: "build" (default/agent) or "plan". Empty means build.
	Mode string
	// Properties carries normalized model-property values keyed by C5 keys
	// (fs, th, ef). Adapters own the native mapping; the map is copied at the
	// request boundary so adapters cannot mutate TUI state (ADR-038/041).
	Properties map[string]string
	// Debug enables verbose harness event output in the TUI (hero --debug).
	Debug bool
	// OnStreamDelta receives live stream events when Stream is true (optional).
	OnStreamDelta func(delta StreamDelta)
	// OnPermissionRequest blocks until the user approves or denies a harness
	// permission prompt (OpenCode permission.asked, etc.). When nil, adapters
	// emit a warning and fail explicitly instead of hanging silently.
	OnPermissionRequest func(ctx context.Context, req PermissionRequest) (PermissionResponse, error)
	// OnQuestionRequest blocks until the user answers or rejects a harness
	// question prompt (OpenCode question.asked, etc.). When nil, adapters emit
	// a warning and fail explicitly instead of hanging silently.
	OnQuestionRequest func(ctx context.Context, req QuestionRequest) (QuestionResponse, error)
}

// NormalizeExecuteRequest returns a request safe to hand to an adapter.  The
// properties map is copied and reduced to the normalized C5 transport keys so
// an adapter cannot mutate the caller's selection map or receive the display
// sentinel "na".  Keep this helper at the shared boundary rather than making
// each caller know how provider adapters protect request state.
func NormalizeExecuteRequest(req ExecuteRequest) ExecuteRequest {
	req.Properties = NormalizeProperties(req.Properties)
	return req
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

// ModelLister is implemented by harness adapters that can enumerate available models.
type ModelLister interface {
	ListModels(ctx context.Context) ([]string, error)
}

// Chat mode constants for ExecuteRequest.Mode.
const (
	ModeBuild = "build"
	ModePlan  = "plan"
)
