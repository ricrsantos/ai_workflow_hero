package engine

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const deferredSkipSummary = "cycle completed with deferred ToDos"

// CloseImplementationWhenAssignmentEmpty completes a running Implementation stage
// when both OpenSpec and findings authorities are empty. Callers must verify
// workload before invoking (PRD-C15-001 §7.3).
func (e *Engine) CloseImplementationWhenAssignmentEmpty(cycleID int64, summary string) error {
	st, err := e.Store.GetStage(cycleID, "implementation")
	if err != nil {
		return err
	}
	if st.Status != store.StageRunning {
		return fmt.Errorf("implementation is %s, expected Running for empty-assignment close", st.Status)
	}
	if strings.TrimSpace(summary) != "" {
		st.Summary = summary
	}
	e.Logger.Info("closing implementation with empty assignment",
		"cycle_id", cycleID, "summary", strings.TrimSpace(summary))
	return e.completeAndAdvance(cycleID, st)
}

// DeferredCompletionDetails is persisted on the cycle when every blocker was deferred.
type DeferredCompletionDetails struct {
	TodoIDs      []string `json:"todo_ids"`
	Count        int      `json:"count"`
	OriginStages []string `json:"origin_stages,omitempty"`
	Summary      string   `json:"summary"`
}

// CompleteCycleWithDeferredTodosDisposition marks the active cycle completed with
// completed_with_deferred_todos, skips enabled downstream validation stages, and
// does not resolve adopted ToDos (ADR-088, ADR-089).
func (e *Engine) CompleteCycleWithDeferredTodosDisposition(cycleID int64, details DeferredCompletionDetails) error {
	if len(details.TodoIDs) == 0 {
		return fmt.Errorf("deferred completion requires at least one todo id")
	}
	details.Count = len(details.TodoIDs)
	if strings.TrimSpace(details.Summary) == "" {
		details.Summary = deferredSkipSummary
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("encode deferred completion details: %w", err)
	}

	c, err := e.Store.GetCycle(cycleID)
	if err != nil {
		return err
	}
	if c.Status != store.CycleStatusActive {
		return fmt.Errorf("cycle is %s, expected active", c.Status)
	}

	err = e.Store.InTx(func(tx *sql.Tx) error {
		stages, err := e.Store.ListStagesTx(tx, cycleID)
		if err != nil {
			return err
		}
		implOrder := stageSortOrder(stages, "implementation")
		now := e.now()
		for i := range stages {
			st := stages[i]
			switch st.Status {
			case store.StageCompleted, store.StageSkipped, store.StageFailed:
				continue
			}
			if st.SortOrder < implOrder {
				continue
			}
			if st.Name == "implementation" {
				st.Status = store.StageCompleted
				st.CompletedAt = now
				if strings.TrimSpace(st.Summary) == "" {
					st.Summary = details.Summary
				}
			} else {
				st.Status = store.StageSkipped
				st.Summary = deferredSkipSummary
				st.CompletedAt = now
			}
			if err := e.Store.UpdateStageTx(tx, st); err != nil {
				return err
			}
		}

		if err := setCycleCompletionDispositionTx(tx, cycleID, store.CompletionDispositionDeferredTodos, string(payload)); err != nil {
			return err
		}
		completedAt := now
		if _, err := tx.Exec(`UPDATE cycles SET status = ?, completed_at = ? WHERE id = ?`,
			store.CycleStatusCompleted, completedAt, cycleID); err != nil {
			return fmt.Errorf("complete cycle with deferred todos: %w", err)
		}

		eventPayload, err := json.Marshal(map[string]any{
			"disposition": store.CompletionDispositionDeferredTodos,
			"todo_ids":    details.TodoIDs,
			"count":       details.Count,
			"summary":     details.Summary,
		})
		if err != nil {
			return err
		}
		if _, err := e.Store.AppendEventTx(tx, store.Event{
			CycleID:     cycleID,
			Type:        store.EventCycleCompletedDeferredTodos,
			PayloadJSON: string(eventPayload),
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		e.Logger.Error("deferred-work cycle completion failed", "cycle_id", cycleID, "error", err)
		return err
	}
	e.Logger.Info("cycle completed with deferred todos disposition",
		"cycle_id", cycleID, "todo_count", details.Count)
	eventID, listErr := e.Store.ListEvents(cycleID, store.EventCycleCompletedDeferredTodos, 1)
	if listErr == nil && len(eventID) > 0 {
		e.publish(eventID[0].ID, conversation.EventCycleFinished, cycleID, c.Title, "", details.Summary)
	}
	return nil
}

func stageSortOrder(stages []store.Stage, name string) int {
	for _, st := range stages {
		if st.Name == name {
			return st.SortOrder
		}
	}
	return -1
}

func setCycleCompletionDispositionTx(tx *sql.Tx, cycleID int64, disposition, dispositionJSON string) error {
	res, err := tx.Exec(`
UPDATE cycles SET completion_disposition = ?, completion_disposition_json = ? WHERE id = ?`,
		disposition, dispositionJSON, cycleID)
	if err != nil {
		return fmt.Errorf("update completion disposition: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// ActiveImplementationAgents returns implementation agent names enabled in the cycle config snapshot.
func (e *Engine) ActiveImplementationAgents(cycleID int64) ([]string, error) {
	c, err := e.Store.GetCycle(cycleID)
	if err != nil {
		return nil, err
	}
	_, agents := activeOwnersFromConfigSnapshot(c.ConfigSnapshotJSON)
	return agents, nil
}
