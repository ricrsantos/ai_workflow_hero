package store

// Cycle status constants.
const (
	CycleStatusActive    = "active"
	CycleStatusCompleted = "completed"
	CycleStatusCancelled = "cancelled"
	CycleStatusArchived  = "archived"
)

// Stage status constants (ADR-012 / design D4).
const (
	StageWaiting         = "Waiting"
	StageRunning         = "Running"
	StagePendingApproval = "PendingApproval"
	StageCompleted       = "Completed"
	StageEscalated       = "Escalated"
	StageFailed          = "Failed"
	StageSkipped         = "Skipped"
)

// Common event types.
const (
	EventCycleCreated    = "cycle_created"
	EventStageStarted    = "stage_started"
	EventStageCompleted  = "stage_completed"
	EventPendingApproval = "pending_approval"
	EventApproved        = "approved"
	EventRejected        = "rejected"
	EventCancelled       = "cancelled"
	EventFinished        = "finished"
	EventContinued       = "continued"
	EventEscalated       = "escalated"
	EventHarnessInvoked  = "harness_invoked"
	EventLegacyImported  = "legacy_imported"
)

// Cycle is a development cycle row.
type Cycle struct {
	ID                 int64
	Number             int
	Title              string
	Objective          string
	Status             string
	StartedAt          string
	CompletedAt        string
	ConfigSnapshotJSON string
	LockHolder         string
	LockAt             string
	// OpenspecChange is the OpenSpec change directory name (empty when unset).
	OpenspecChange string
}

// Stage is a stage row within a cycle.
type Stage struct {
	ID                   int64
	CycleID              int64
	Name                 string
	Status               string
	Iteration            int
	MaxIterations        int
	ExtraIterations      int
	RequireHumanApproval bool
	TimeoutMinutes       int
	StartedAt            string
	CompletedAt          string
	Summary              string
	SortOrder            int
	// HarnessSessionID is the Cursor CLI session id for --resume within an etapa (schema v3).
	HarnessSessionID string
}

// Event is an append-only operational event.
type Event struct {
	ID          int64
	CycleID     int64
	TS          string
	Type        string
	PayloadJSON string
}

// Metric is a per-stage (and optional agent) cost/token estimate.
type Metric struct {
	ID           int64
	CycleID      int64
	StageName    string
	Model        string
	Agent        string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	DurationMS   int64
}

// Artifact is metadata for a project file linked to a cycle.
type Artifact struct {
	ID        int64
	CycleID   int64
	Path      string
	Kind      string
	Label     string
	CreatedAt string
}

// ConversationEntry is a minimal approval/question record.
type ConversationEntry struct {
	ID      int64
	CycleID int64
	TS      string
	Role    string
	Kind    string
	Body    string
}

// EffectiveMaxIterations returns max_iterations + extra_iterations.
func (s Stage) EffectiveMaxIterations() int {
	return s.MaxIterations + s.ExtraIterations
}
