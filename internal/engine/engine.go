package engine

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// Engine applies deterministic cycle/stage transitions against a Store.
type Engine struct {
	Store  *store.Store
	Logger *slog.Logger
	Now    func() time.Time
}

// New creates an Engine bound to s.
func New(s *store.Store) *Engine {
	return &Engine{
		Store:  s,
		Logger: slog.Default(),
		Now:    time.Now,
	}
}

func (e *Engine) now() string {
	return e.Now().UTC().Format(time.RFC3339)
}

// MetricInput is optional metrics carried by approve/finish payloads.
type MetricInput struct {
	StageName    string  `json:"stage_name"`
	Model        string  `json:"model"`
	Agent        string  `json:"agent"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	DurationMS   int64   `json:"duration_ms"`
}

// StageCloseInput is the payload when closing a running stage.
type StageCloseInput struct {
	Summary string
	Metrics []MetricInput
	Failed  bool
}

// LockHolder identifies the session acquiring the cycle lock.
type LockHolder string

// AcquireLock locks the active cycle for holder. Returns store.ErrBusy if held.
func (e *Engine) AcquireLock(holder string) (store.Cycle, error) {
	c, err := e.Store.GetActiveCycle()
	if err != nil {
		return store.Cycle{}, err
	}
	if c.LockHolder != "" && c.LockHolder != holder {
		return c, store.ErrBusy
	}
	if err := e.Store.SetCycleLock(c.ID, holder, e.now()); err != nil {
		return c, err
	}
	c.LockHolder = holder
	c.LockAt = e.now()
	return c, nil
}

// ReleaseLock clears the lock if held by holder (or any if holder is empty and force).
func (e *Engine) ReleaseLock(cycleID int64, holder string) error {
	c, err := e.Store.GetCycle(cycleID)
	if err != nil {
		return err
	}
	if c.LockHolder != "" && holder != "" && c.LockHolder != holder {
		return store.ErrBusy
	}
	return e.Store.SetCycleLock(cycleID, "", "")
}

// withLock runs fn while holding the active cycle lock.
func (e *Engine) withLock(holder string, fn func(store.Cycle) error) error {
	if holder == "" {
		holder = fmt.Sprintf("hero-%d", e.Now().UnixNano())
	}
	c, err := e.AcquireLock(holder)
	if err != nil {
		return err
	}
	defer func() { _ = e.ReleaseLock(c.ID, holder) }()
	return fn(c)
}

// StartStage marks a waiting stage as Running and increments iteration.
// Timeout is checked between iterations (PRD §5.4): if wall-clock elapsed since
// the stage's first start exceeds TimeoutMinutes, the stage is Escalated.
func (e *Engine) StartStage(cycleID int64, stageName string) error {
	st, err := e.Store.GetStage(cycleID, stageName)
	if err != nil {
		return err
	}
	if st.Status != store.StageWaiting && st.Status != store.StageEscalated {
		return fmt.Errorf("stage %s is %s, cannot start", stageName, st.Status)
	}
	if st.Status == store.StageEscalated {
		return fmt.Errorf("stage %s is Escalated; run continue/cancel/finish first", stageName)
	}
	if reason, ok := e.budgetExhausted(st); ok {
		if err := e.escalateStage(cycleID, st, reason); err != nil {
			return err
		}
		if reason == "timeout" {
			return fmt.Errorf("stage %s timeout exhausted", stageName)
		}
		return fmt.Errorf("stage %s iteration budget exhausted", stageName)
	}
	st.Iteration++
	st.Status = store.StageRunning
	// Preserve first-start wall clock across iterations for timeout accounting.
	if st.StartedAt == "" {
		st.StartedAt = e.now()
	}
	if err := e.Store.UpdateStage(st); err != nil {
		return err
	}
	_, err = e.Store.AppendEvent(store.Event{
		CycleID: cycleID, Type: store.EventStageStarted,
		PayloadJSON: fmt.Sprintf(`{"stage":%q,"iteration":%d}`, stageName, st.Iteration),
	})
	e.Logger.Info("stage started", "cycle_id", cycleID, "stage", stageName, "iteration", st.Iteration)
	return err
}

// CloseStage completes a running stage into PendingApproval or Completed/Failed.
func (e *Engine) CloseStage(cycleID int64, stageName string, in StageCloseInput) error {
	st, err := e.Store.GetStage(cycleID, stageName)
	if err != nil {
		return err
	}
	if st.Status != store.StageRunning {
		return fmt.Errorf("stage %s is %s, expected Running", stageName, st.Status)
	}
	if err := e.persistMetrics(cycleID, stageName, in.Metrics); err != nil {
		return err
	}
	st.Summary = in.Summary
	if in.Failed {
		st.Status = store.StageFailed
		st.CompletedAt = e.now()
		if err := e.Store.UpdateStage(st); err != nil {
			return err
		}
		_, err = e.Store.AppendEvent(store.Event{
			CycleID: cycleID, Type: store.EventStageCompleted,
			PayloadJSON: fmt.Sprintf(`{"stage":%q,"status":"Failed"}`, stageName),
		})
		return err
	}
	if st.RequireHumanApproval {
		st.Status = store.StagePendingApproval
		if err := e.Store.UpdateStage(st); err != nil {
			return err
		}
		_, err = e.Store.AppendEvent(store.Event{
			CycleID: cycleID, Type: store.EventPendingApproval,
			PayloadJSON: fmt.Sprintf(`{"stage":%q}`, stageName),
		})
		return err
	}
	return e.completeAndAdvance(cycleID, st)
}

// Approve approves a pending-approval stage and advances.
func (e *Engine) Approve(holder string, summary string, metrics []MetricInput) error {
	return e.withLock(holder, func(c store.Cycle) error {
		st, err := e.findPending(c.ID)
		if err != nil {
			return err
		}
		if err := e.persistMetrics(c.ID, st.Name, metrics); err != nil {
			return err
		}
		if summary != "" {
			st.Summary = summary
		}
		if _, err := e.Store.AppendEvent(store.Event{
			CycleID: c.ID, Type: store.EventApproved,
			PayloadJSON: fmt.Sprintf(`{"stage":%q}`, st.Name),
		}); err != nil {
			return err
		}
		_, _ = e.Store.AddConversation(store.ConversationEntry{
			CycleID: c.ID, Role: "user", Kind: "approval", Body: "approved: " + st.Name,
		})
		return e.completeAndAdvance(c.ID, st)
	})
}

// Reject marks a pending stage Failed (or returns it to Waiting for rework).
func (e *Engine) Reject(holder string, reason string) error {
	return e.withLock(holder, func(c store.Cycle) error {
		st, err := e.findPending(c.ID)
		if err != nil {
			return err
		}
		st.Status = store.StageWaiting
		st.Summary = reason
		if err := e.Store.UpdateStage(st); err != nil {
			return err
		}
		_, err = e.Store.AppendEvent(store.Event{
			CycleID: c.ID, Type: store.EventRejected,
			PayloadJSON: fmt.Sprintf(`{"stage":%q,"reason":%q}`, st.Name, reason),
		})
		return err
	})
}

// Cancel cancels the active cycle.
func (e *Engine) Cancel(holder string, reason string) error {
	return e.withLock(holder, func(c store.Cycle) error {
		if err := e.Store.UpdateCycleStatus(c.ID, store.CycleStatusCancelled, e.now()); err != nil {
			return err
		}
		_, err := e.Store.AppendEvent(store.Event{
			CycleID: c.ID, Type: store.EventCancelled,
			PayloadJSON: fmt.Sprintf(`{"reason":%q}`, reason),
		})
		e.Logger.Info("cycle cancelled", "cycle_id", c.ID)
		return err
	})
}

// Finish marks the cycle completed (optionally closing pending approval).
func (e *Engine) Finish(holder string, metrics []MetricInput) error {
	return e.withLock(holder, func(c store.Cycle) error {
		if st, err := e.findPending(c.ID); err == nil {
			if err := e.persistMetrics(c.ID, st.Name, metrics); err != nil {
				return err
			}
			st.Status = store.StageCompleted
			st.CompletedAt = e.now()
			if err := e.Store.UpdateStage(st); err != nil {
				return err
			}
		}
		if err := e.Store.UpdateCycleStatus(c.ID, store.CycleStatusCompleted, e.now()); err != nil {
			return err
		}
		_, err := e.Store.AppendEvent(store.Event{
			CycleID: c.ID, Type: store.EventFinished, PayloadJSON: `{}`,
		})
		return err
	})
}

// Continue grants extra iterations to an escalated stage and sets it Waiting.
// Clears StartedAt so the timeout clock restarts on the next StartStage.
func (e *Engine) Continue(holder string, extra int) error {
	if extra <= 0 {
		extra = 1
	}
	return e.withLock(holder, func(c store.Cycle) error {
		stages, err := e.Store.ListStages(c.ID)
		if err != nil {
			return err
		}
		var st *store.Stage
		for i := range stages {
			if stages[i].Status == store.StageEscalated {
				st = &stages[i]
				break
			}
		}
		if st == nil {
			return fmt.Errorf("no escalated stage to continue")
		}
		st.ExtraIterations += extra
		st.Status = store.StageWaiting
		st.StartedAt = "" // restart timeout clock after human continue
		if err := e.Store.UpdateStage(*st); err != nil {
			return err
		}
		_, err = e.Store.AppendEvent(store.Event{
			CycleID: c.ID, Type: store.EventContinued,
			PayloadJSON: fmt.Sprintf(`{"stage":%q,"extra":%d}`, st.Name, extra),
		})
		return err
	})
}

// EscalateIfExhausted marks a running/waiting stage Escalated when iteration
// or timeout budget is spent (checked between iterations / at checkpoints).
func (e *Engine) EscalateIfExhausted(cycleID int64, stageName string) error {
	st, err := e.Store.GetStage(cycleID, stageName)
	if err != nil {
		return err
	}
	if st.Status == store.StageCompleted || st.Status == store.StageSkipped || st.Status == store.StageEscalated {
		return nil
	}
	reason, ok := e.budgetExhausted(st)
	if !ok {
		return nil
	}
	return e.escalateStage(cycleID, st, reason)
}

// budgetExhausted reports whether iteration or timeout budget is spent.
// Timeout uses wall-clock elapsed since stage StartedAt (first StartStage).
func (e *Engine) budgetExhausted(st store.Stage) (reason string, exhausted bool) {
	if st.Iteration >= st.EffectiveMaxIterations() {
		return "iteration_budget", true
	}
	if st.TimeoutMinutes > 0 && st.StartedAt != "" {
		started, err := time.Parse(time.RFC3339, st.StartedAt)
		if err != nil {
			e.Logger.Error("parse stage started_at", "stage", st.Name, "started_at", st.StartedAt, "err", err)
			return "", false
		}
		elapsed := e.Now().UTC().Sub(started.UTC())
		if elapsed >= time.Duration(st.TimeoutMinutes)*time.Minute {
			e.Logger.Info("stage timeout exceeded",
				"stage", st.Name, "timeout_minutes", st.TimeoutMinutes, "elapsed", elapsed.String())
			return "timeout", true
		}
	}
	return "", false
}

func (e *Engine) escalateStage(cycleID int64, st store.Stage, reason string) error {
	st.Status = store.StageEscalated
	if err := e.Store.UpdateStage(st); err != nil {
		return err
	}
	_, err := e.Store.AppendEvent(store.Event{
		CycleID: cycleID, Type: store.EventEscalated,
		PayloadJSON: fmt.Sprintf(`{"stage":%q,"reason":%q}`, st.Name, reason),
	})
	e.Logger.Info("stage escalated", "cycle_id", cycleID, "stage", st.Name, "reason", reason)
	return err
}

func (e *Engine) findPending(cycleID int64) (store.Stage, error) {
	stages, err := e.Store.ListStages(cycleID)
	if err != nil {
		return store.Stage{}, err
	}
	for _, st := range stages {
		if st.Status == store.StagePendingApproval {
			return st, nil
		}
	}
	return store.Stage{}, fmt.Errorf("no stage pending approval")
}

func (e *Engine) completeAndAdvance(cycleID int64, st store.Stage) error {
	st.Status = store.StageCompleted
	st.CompletedAt = e.now()
	if err := e.Store.UpdateStage(st); err != nil {
		return err
	}
	if _, err := e.Store.AppendEvent(store.Event{
		CycleID: cycleID, Type: store.EventStageCompleted,
		PayloadJSON: fmt.Sprintf(`{"stage":%q,"status":"Completed"}`, st.Name),
	}); err != nil {
		return err
	}
	return e.advanceToNext(cycleID, st.SortOrder)
}

func (e *Engine) advanceToNext(cycleID int64, afterSort int) error {
	stages, err := e.Store.ListStages(cycleID)
	if err != nil {
		return err
	}
	for _, st := range stages {
		if st.SortOrder <= afterSort {
			continue
		}
		if st.Status == store.StageSkipped {
			continue
		}
		if st.Status == store.StageWaiting {
			// Leave next stage Waiting; Runtime/start will call StartStage.
			e.Logger.Info("next stage ready", "cycle_id", cycleID, "stage", st.Name)
			return nil
		}
	}
	// No further stages — cycle may be completed by finish.
	e.Logger.Info("no further stages", "cycle_id", cycleID)
	return nil
}

func (e *Engine) persistMetrics(cycleID int64, defaultStage string, metrics []MetricInput) error {
	for _, m := range metrics {
		stage := m.StageName
		if stage == "" {
			stage = defaultStage
		}
		if err := e.Store.UpsertMetric(store.Metric{
			CycleID:      cycleID,
			StageName:    stage,
			Model:        m.Model,
			Agent:        m.Agent,
			InputTokens:  m.InputTokens,
			OutputTokens: m.OutputTokens,
			CostUSD:      m.CostUSD,
			DurationMS:   m.DurationMS,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ParseMetricsJSON parses a --metrics-json payload into MetricInput slice.
func ParseMetricsJSON(raw string) ([]MetricInput, error) {
	raw = trimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var multi []MetricInput
	if err := json.Unmarshal([]byte(raw), &multi); err == nil {
		return multi, nil
	}
	var single MetricInput
	if err := json.Unmarshal([]byte(raw), &single); err != nil {
		return nil, fmt.Errorf("parse metrics-json: %w", err)
	}
	return []MetricInput{single}, nil
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// --- workflow-config import -------------------------------------------------

// WorkflowConfig is the subset of workflow-config.yml needed for cycle creation.
type WorkflowConfig struct {
	Title     string                     `yaml:"title"`
	Objective string                     `yaml:"objective"`
	Stages    map[string]StageConfigYAML `yaml:"stages"`
}

// StageConfigYAML is one stage block from workflow-config.yml.
type StageConfigYAML struct {
	Enabled              bool `yaml:"enabled"`
	MaxIterations        int  `yaml:"max_iterations"`
	TimeoutMinutes       int  `yaml:"timeout_minutes"`
	RequireHumanApproval bool `yaml:"require_human_approval"`
}

// Canonical stage order for 1.0 (matches Runtime).
var canonicalStageOrder = []string{
	"research",
	"planning",
	"implementation",
	"qa",
	"judge",
	"browser_ui_validation",
	"qa_end_to_end",
}

// LoadWorkflowConfig reads and parses workflow-config.yml.
func LoadWorkflowConfig(path string) (WorkflowConfig, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkflowConfig{}, nil, err
	}
	var cfg WorkflowConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return WorkflowConfig{}, nil, fmt.Errorf("parse workflow-config: %w", err)
	}
	return cfg, data, nil
}

// NewCycleOptions configures cycle creation from workflow-config.
type NewCycleOptions struct {
	ProjectDir  string
	ConfigPath  string // optional; default current/workflow-config.yml
	Title       string // override when DeferMeta is false
	Objective   string // override when DeferMeta is false
	CycleNumber int    // 0 = auto
	// DeferMeta leaves title and objective empty in SQLite (filled later via SyncCycleConfigFromWorkflow).
	DeferMeta bool
}

// NewCycleResult is returned by CreateCycleFromConfig.
type NewCycleResult struct {
	Cycle  store.Cycle
	Stages []store.Stage
}

// CreateCycleFromConfig imports workflow-config.yml into a new active cycle snapshot.
func (e *Engine) CreateCycleFromConfig(opts NewCycleOptions) (NewCycleResult, error) {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = resolveWorkflowConfigPath(opts.ProjectDir)
	}
	cfg, raw, err := LoadWorkflowConfig(configPath)
	if err != nil {
		return NewCycleResult{}, fmt.Errorf("workflow-config.yml: %w", err)
	}
	var title, objective string
	if opts.DeferMeta {
		title = ""
		objective = ""
	} else {
		title = opts.Title
		if title == "" {
			title = cfg.Title
		}
		objective = opts.Objective
		if objective == "" {
			objective = cfg.Objective
		}
	}
	num := opts.CycleNumber
	if num <= 0 {
		num, err = e.Store.NextCycleNumber()
		if err != nil {
			return NewCycleResult{}, err
		}
	}

	// Archive/deactivate previous active cycles.
	if prev, err := e.Store.GetActiveCycle(); err == nil {
		_ = e.Store.UpdateCycleStatus(prev.ID, store.CycleStatusArchived, e.now())
	}

	id, err := e.Store.CreateCycle(store.Cycle{
		Number:             num,
		Title:              title,
		Objective:          objective,
		Status:             store.CycleStatusActive,
		StartedAt:          e.now(),
		ConfigSnapshotJSON: string(raw),
	})
	if err != nil {
		return NewCycleResult{}, err
	}

	stages := buildStagesFromConfig(id, cfg)
	if err := e.Store.CreateStages(stages); err != nil {
		return NewCycleResult{}, err
	}
	if _, err := e.Store.AppendEvent(store.Event{
		CycleID: id, Type: store.EventCycleCreated,
		PayloadJSON: fmt.Sprintf(`{"number":%d,"title":%q}`, num, title),
	}); err != nil {
		return NewCycleResult{}, err
	}

	c, err := e.Store.GetCycle(id)
	if err != nil {
		return NewCycleResult{}, err
	}
	listed, err := e.Store.ListStages(id)
	if err != nil {
		return NewCycleResult{}, err
	}
	e.Logger.Info("cycle created", "cycle_id", id, "number", num, "stages", len(listed))
	return NewCycleResult{Cycle: c, Stages: listed}, nil
}

// SyncCycleConfigFromWorkflow reads workflow-config.yml and updates the active cycle's
// title, objective, config snapshot, and still-open stage budgets (used by /hero-start
// before stage orchestration). Completed/failed stages are left unchanged.
func (e *Engine) SyncCycleConfigFromWorkflow(projectDir string) error {
	configPath := resolveWorkflowConfigPath(projectDir)
	cfg, raw, err := LoadWorkflowConfig(configPath)
	if err != nil {
		return fmt.Errorf("workflow-config.yml: %w", err)
	}
	c, err := e.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	if err := e.Store.UpdateCycleMeta(c.ID, cfg.Title, cfg.Objective, string(raw)); err != nil {
		return err
	}
	if err := e.syncStagesFromWorkflow(c.ID, cfg); err != nil {
		return err
	}
	e.Logger.Info("cycle config synced", "cycle", c.Number, "title", cfg.Title)
	return nil
}

func (e *Engine) syncStagesFromWorkflow(cycleID int64, cfg WorkflowConfig) error {
	existing, err := e.Store.ListStages(cycleID)
	if err != nil {
		return err
	}
	byName := make(map[string]store.Stage, len(existing))
	for _, st := range existing {
		byName[st.Name] = st
	}
	for _, name := range canonicalStageOrder {
		sc, ok := cfg.Stages[name]
		if !ok {
			continue
		}
		st, ok := byName[name]
		if !ok {
			continue
		}
		if st.Status == store.StageCompleted || st.Status == store.StageFailed {
			continue
		}
		maxIter := sc.MaxIterations
		if maxIter <= 0 {
			maxIter = 1
		}
		st.MaxIterations = maxIter
		st.TimeoutMinutes = sc.TimeoutMinutes
		st.RequireHumanApproval = sc.RequireHumanApproval
		if st.Status == store.StageWaiting || st.Status == store.StageSkipped {
			if sc.Enabled {
				st.Status = store.StageWaiting
			} else {
				st.Status = store.StageSkipped
			}
		}
		if err := e.Store.UpdateStage(st); err != nil {
			return err
		}
	}
	return nil
}

func resolveWorkflowConfigPath(projectDir string) string {
	configPath := filepath.Join(projectDir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	if _, err := os.Stat(configPath); err != nil {
		alt := filepath.Join(projectDir, ".workflow-hero", "templates", "workflow-config.yml")
		if _, err2 := os.Stat(alt); err2 == nil {
			return alt
		}
	}
	return configPath
}

func buildStagesFromConfig(cycleID int64, cfg WorkflowConfig) []store.Stage {
	var out []store.Stage
	order := 0
	for _, name := range canonicalStageOrder {
		sc, ok := cfg.Stages[name]
		if !ok {
			continue
		}
		status := store.StageWaiting
		maxIter := sc.MaxIterations
		if maxIter <= 0 {
			maxIter = 1
		}
		if !sc.Enabled {
			status = store.StageSkipped
		}
		out = append(out, store.Stage{
			CycleID:              cycleID,
			Name:                 name,
			Status:               status,
			MaxIterations:        maxIter,
			TimeoutMinutes:       sc.TimeoutMinutes,
			RequireHumanApproval: sc.RequireHumanApproval,
			SortOrder:            order,
		})
		order++
	}
	return out
}
