package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// StageBudget returns the durable ledger for a timed stage.
func (e *Engine) StageBudget(cycleID int64, stageName string) (store.StageBudget, error) {
	return e.Store.GetStageBudget(cycleID, stageName)
}

// BeginStageBudget starts or resumes the existing cumulative budget for a
// stage. A retry never creates a fresh allowance.
func (e *Engine) BeginStageBudget(cycleID int64, stageName string) (store.StageBudget, error) {
	st, err := e.Store.GetStage(cycleID, stageName)
	if err != nil {
		return store.StageBudget{}, err
	}
	if st.TimeoutMinutes <= 0 {
		return store.StageBudget{}, nil
	}
	b, err := e.Store.BeginStageBudget(cycleID, stageName, time.Duration(st.TimeoutMinutes)*time.Minute, e.Now())
	if err != nil {
		return b, err
	}
	e.recordStageBudgetEvent(b, "start")
	return b, nil
}

// CheckpointStageBudget stores active elapsed time at now.
func (e *Engine) CheckpointStageBudget(cycleID int64, stageName string, generation int64) (store.StageBudget, error) {
	b, err := e.Store.CheckpointStageBudget(cycleID, stageName, generation, e.Now())
	if err != nil {
		return store.StageBudget{}, err
	}
	if b.State == store.StageBudgetExpired {
		e.recordStageBudgetEvent(b, "expired")
	}
	return b, nil
}

// AcceptStageBudget is the serialized TUI completion boundary. Unlimited
// stages have no ledger and accept generation zero.
func (e *Engine) AcceptStageBudget(cycleID int64, stageName string, generation int64) (bool, store.StageBudget, error) {
	b, err := e.Store.GetStageBudget(cycleID, stageName)
	if errors.Is(err, store.ErrNotFound) {
		return generation == 0, store.StageBudget{}, nil
	}
	if err != nil {
		return false, store.StageBudget{}, err
	}
	accepted, b, err := e.Store.AcceptStageBudget(cycleID, stageName, generation, e.Now())
	if err != nil {
		return false, store.StageBudget{}, err
	}
	if b.State == store.StageBudgetExpired {
		e.recordStageBudgetEvent(b, "expired")
	}
	return accepted, b, nil
}

// PauseStageBudget stops accrual while no stage-owned work can progress.
func (e *Engine) PauseStageBudget(cycleID int64, stageName string, generation int64, reason string) (store.StageBudget, error) {
	b, err := e.Store.PauseStageBudget(cycleID, stageName, generation, store.StageBudgetWaiting, reason, e.Now())
	if errors.Is(err, store.ErrNotFound) {
		return store.StageBudget{}, nil
	}
	if err != nil {
		return b, err
	}
	e.recordStageBudgetEvent(b, "pause")
	return b, nil
}

// ResumeStageBudget resumes time after a human-only wait. rotateGeneration is
// false for that same execution and true for a newly dispatched attempt.
func (e *Engine) ResumeStageBudget(cycleID int64, stageName string, generation int64, rotateGeneration bool) (store.StageBudget, error) {
	b, err := e.Store.ResumeStageBudget(cycleID, stageName, generation, e.Now(), rotateGeneration)
	if errors.Is(err, store.ErrNotFound) {
		return store.StageBudget{}, nil
	}
	if err != nil {
		return b, err
	}
	e.recordStageBudgetEvent(b, "resume")
	return b, nil
}

// InterruptStageBudget records controlled termination and revokes outstanding
// acceptance authority before the TUI asks a harness to stop.
func (e *Engine) InterruptStageBudget(cycleID int64, stageName string, generation int64, reason string) (store.StageBudget, error) {
	b, err := e.Store.InterruptStageBudget(cycleID, stageName, generation, reason, e.Now())
	if errors.Is(err, store.ErrNotFound) {
		return store.StageBudget{}, nil
	}
	if err != nil {
		return b, err
	}
	e.recordStageBudgetEvent(b, "interrupt")
	return b, nil
}

// ExpireStageBudget checkpoints through the exact deadline and revokes the
// generation before returning control to the TUI cancellation path.
func (e *Engine) ExpireStageBudget(cycleID int64, stageName string, generation int64) (store.StageBudget, bool, error) {
	b, expired, err := e.Store.ExpireStageBudget(cycleID, stageName, generation, e.Now())
	if errors.Is(err, store.ErrNotFound) {
		return store.StageBudget{}, false, nil
	}
	if err != nil {
		return store.StageBudget{}, false, err
	}
	if expired {
		e.recordStageBudgetEvent(b, "expired")
	}
	return b, expired, nil
}

// IncreaseStageBudget adds explicit positive time while preserving consumed
// time and returns the stage to a waiting state for its normal continue flow.
func (e *Engine) IncreaseStageBudget(cycleID int64, stageName string, additional time.Duration) (store.StageBudget, error) {
	if additional <= 0 {
		return store.StageBudget{}, fmt.Errorf("budget increase must be positive")
	}
	b, err := e.Store.GetStageBudget(cycleID, stageName)
	if err != nil {
		return store.StageBudget{}, err
	}
	now := e.Now()
	if b.State == store.StageBudgetActive && b.RemainingAt(now) <= 0 {
		var expired bool
		b, expired, err = e.Store.ExpireStageBudget(cycleID, stageName, b.Generation, now)
		if err != nil {
			return store.StageBudget{}, err
		}
		if expired {
			e.recordStageBudgetEvent(b, "expired")
		}
	}
	if b.RemainingAt(now) > 0 {
		return store.StageBudget{}, fmt.Errorf("stage budget still has time remaining")
	}
	b, err = e.Store.IncreaseStageBudget(cycleID, stageName, additional, now)
	if err != nil {
		return store.StageBudget{}, err
	}
	e.recordStageBudgetEvent(b, "increase")
	return b, nil
}

// RecoverActiveStageBudgets marks work interrupted at its last durable
// checkpoint, excludes offline time, and revokes the pre-restart generation.
func (e *Engine) RecoverActiveStageBudgets() ([]store.StageBudget, error) {
	budgets, err := e.Store.RecoverActiveStageBudgets(e.Now())
	if err != nil {
		return nil, err
	}
	for _, b := range budgets {
		e.recordStageBudgetEvent(b, "restart_interrupted")
	}
	return budgets, nil
}

func (e *Engine) recordStageBudgetEvent(b store.StageBudget, transition string) {
	if e == nil || e.Store == nil || b.CycleID <= 0 || b.StageName == "" {
		return
	}
	payload, err := json.Marshal(struct {
		StageName          string `json:"stage"`
		Transition         string `json:"transition"`
		State              string `json:"state"`
		LimitMS            int64  `json:"limit_ms"`
		ConsumedMS         int64  `json:"consumed_ms"`
		Generation         int64  `json:"generation"`
		PauseReason        string `json:"pause_reason,omitempty"`
		InterruptionReason string `json:"interruption_reason,omitempty"`
	}{
		StageName: b.StageName, Transition: transition, State: string(b.State),
		LimitMS: int64(b.Limit / time.Millisecond), ConsumedMS: int64(b.Consumed / time.Millisecond),
		Generation: b.Generation, PauseReason: b.PauseReason, InterruptionReason: b.InterruptionReason,
	})
	if err != nil {
		e.Logger.Error("stage budget event encode failed", "cycle_id", b.CycleID, "stage", b.StageName, "error", err)
		return
	}
	if _, err := e.Store.AppendEvent(store.Event{CycleID: b.CycleID, Type: store.EventStageBudget, PayloadJSON: string(payload)}); err != nil {
		e.Logger.Error("stage budget event persist failed", "cycle_id", b.CycleID, "stage", b.StageName, "transition", transition, "error", err)
	}
}

func (e *Engine) acceptAndPauseStageBudget(cycleID int64, stageName string, failed bool) error {
	b, err := e.Store.GetStageBudget(cycleID, stageName)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load stage budget before close: %w", err)
	}
	if b.State != store.StageBudgetActive {
		return fmt.Errorf("stage %s budget is %s; result cannot close the stage", stageName, b.State)
	}
	now := e.Now()
	accepted, b, err := e.Store.AcceptStageBudget(cycleID, stageName, b.Generation, now)
	if err != nil {
		return fmt.Errorf("accept stage result: %w", err)
	}
	if !accepted {
		if b.State == store.StageBudgetExpired {
			e.recordStageBudgetEvent(b, "expired")
			return store.ErrStageBudgetExpired
		}
		return store.ErrStageBudgetGeneration
	}
	state := store.StageBudgetWaiting
	reason := "stage_closed"
	if failed {
		state = store.StageBudgetBlocked
		reason = "stage_failed"
	}
	b, err = e.Store.PauseStageBudget(cycleID, stageName, b.Generation, state, reason, now)
	if err != nil {
		return fmt.Errorf("pause stage budget after close: %w", err)
	}
	e.recordStageBudgetEvent(b, "terminate")
	return nil
}
