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
	// PermissionProfile is the persisted project-scoped approval preset. Each
	// adapter maps this normalized value to its native permission mechanism.
	PermissionProfile PermissionProfile
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
	req.PermissionProfile = NormalizePermissionProfile(req.PermissionProfile)
	return req
}

// Usage holds optional token counts for one harness Execute turn.
//
// InputTokens/OutputTokens/cache fields are billed consumption for this
// Execute (cycle Costs accumulate those). They may sum every model call in a
// tool loop and must not be treated as window fill.
//
// ContextTokens is window occupancy after the last model call (prompt
// including cache plus that call's output). Adapters set it only from a
// per-call snapshot, never from a billed aggregate. It is never summed
// across Executes.
type Usage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ContextTokens    int64
}

// HasBilledCounts reports whether the harness provided billed token fields.
func (u Usage) HasBilledCounts() bool {
	return u.InputTokens > 0 || u.OutputTokens > 0 || u.CacheReadTokens > 0 || u.CacheWriteTokens > 0
}

// HasCounts reports whether any token field was provided by the harness.
func (u Usage) HasCounts() bool {
	return u.HasBilledCounts() || u.ContextTokens > 0
}

// PromptTokens is the prompt-side size of a single model-call snapshot.
// When InputTokens already includes cache (OpenAI-style), cache fields are
// not added again. Otherwise cache read/write are added (Anthropic-style).
func (u Usage) PromptTokens() int64 {
	in := u.InputTokens
	if in < 0 {
		in = 0
	}
	cache := u.CacheReadTokens + u.CacheWriteTokens
	if cache < 0 {
		cache = 0
	}
	if cache > 0 && in >= cache {
		return in
	}
	return in + cache
}

// CallOccupancy is window fill for a single model-call snapshot (prompt plus
// that call's output). Do not call this on billed aggregates.
func (u Usage) CallOccupancy() int64 {
	n := u.PromptTokens() + u.OutputTokens
	if n < 0 {
		return 0
	}
	return n
}

// Occupancy is the context-window fill after this turn. Only an adapter-set
// ContextTokens counts; billed input/cache/output are never reconstructed
// into occupancy.
func (u Usage) Occupancy() int64 {
	if u.ContextTokens > 0 {
		return u.ContextTokens
	}
	return 0
}

// WithCallOccupancy fills ContextTokens from this snapshot's CallOccupancy
// when unset. Use only when the Usage represents one model call.
func (u Usage) WithCallOccupancy() Usage {
	if u.ContextTokens <= 0 {
		u.ContextTokens = u.CallOccupancy()
	}
	return u
}

// ExecutionResult is the normalized outcome of Execute.
type ExecutionResult struct {
	SessionID  string
	Output     string
	Summary    string
	Usage      Usage
	Duration   time.Duration
	StreamDone bool
	// NativeModel is the harness-reported model that actually served this
	// execution. It remains distinct from ExecuteRequest.Model because account
	// policy may resolve an alias differently at runtime.
	NativeModel string
	// EffectiveProperties contains optional runtime-authoritative native
	// properties. Not every harness reports these in its stream.
	EffectiveProperties map[string]string
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
