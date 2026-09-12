package cycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/ricrsantos/ai_workflow_hero/internal/engine"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/todos"
)

// ReportValidationError re-exports engine report validation failures for CLI/TUI callers.
type ReportValidationError = engine.ReportValidationError

// FailedCloseHandoffResult is returned after a successful atomic failed validation close.
type FailedCloseHandoffResult = engine.FailedCloseHandoffResult

// ImplementationAssignmentInput exposes the task-* and find-* union for scheduling.
type ImplementationAssignmentInput struct {
	TasksPath string
	Linked    bool
	Ready     bool
	ByAgent   map[string][]string
}

// DeferredCompletionDetails is the cycle disposition payload for defer-all closure.
type DeferredCompletionDetails = engine.DeferredCompletionDetails

// ValidationDecodeContext returns decode context for validation stage-agent reports.
func (s *Service) ValidationDecodeContext() (reports.DecodeContext, error) {
	var empty reports.DecodeContext
	if s == nil || s.Engine == nil || s.Store == nil {
		return empty, errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return empty, err
	}
	return s.Engine.ValidationDecodeContext(c.ID)
}

// CloseStageFailedWithFindings validates reportJSON and atomically persists findings,
// closes the source stage as failed, and loops back to Implementation (ADR-084).
// Callers must not wrap this in store.InTx.
func (s *Service) CloseStageFailedWithFindings(stageName string, reportJSON []byte, metricsJSON string) (FailedCloseHandoffResult, error) {
	var empty FailedCloseHandoffResult
	if s == nil || s.Engine == nil || s.Store == nil {
		return empty, errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return empty, err
	}
	metrics, err := engine.ParseMetricsJSON(metricsJSON)
	if err != nil {
		return empty, err
	}
	slog.Info("atomic failed validation close requested", "cycle", c.Number, "stage", stageName)
	out, err := s.Engine.CloseStageFailedWithFindings(c.ID, stageName, reportJSON, metrics)
	if err != nil {
		var rve *engine.ReportValidationError
		if errors.As(err, &rve) {
			return empty, err
		}
		slog.Error("atomic failed validation close failed", "cycle", c.Number, "stage", stageName, "error", err)
		return empty, err
	}
	return out, nil
}

// ListActionableFindings returns open/reopened findings for the active cycle.
func (s *Service) ListActionableFindings(owner string) ([]store.Finding, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return nil, err
	}
	return s.Store.ListActionableFindings(c.ID, owner)
}

// ActiveImplementationAgents returns agents enabled for Implementation on the active cycle.
func (s *Service) ActiveImplementationAgents() ([]string, error) {
	if s == nil || s.Engine == nil {
		return nil, errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return nil, err
	}
	return s.Engine.ActiveImplementationAgents(c.ID)
}

// ImplementationAssignmentInput builds the ordered task-*/find-* union per agent.
func (s *Service) BuildImplementationAssignmentInput(activeAgents []string) (ImplementationAssignmentInput, error) {
	var out ImplementationAssignmentInput
	if s == nil || s.Store == nil {
		return out, errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return out, err
	}
	path, raw, linked, ready := s.linkedOpenSpecTasks(&c)
	out.TasksPath = path
	out.Linked = linked
	out.Ready = ready
	findings, err := s.Store.ListActionableFindings(c.ID, "")
	if err != nil {
		return out, err
	}
	if !linked || !ready {
		return out, nil
	}
	checklist := implementationChecklist{Linked: linked, Ready: ready, Raw: raw, Path: path}
	plan := partitionImplementationTasks(raw, activeAgents)
	if !plan.Valid {
		if len(plan.Errors) > 0 {
			return out, fmt.Errorf("implementation task plan invalid: %s", strings.Join(plan.Errors, "; "))
		}
		return out, fmt.Errorf("implementation task plan invalid")
	}
	byAgent, mergeErrors := mergeImplementationAssignment(plan, findings, activeAgents)
	if len(mergeErrors) > 0 {
		return out, fmt.Errorf("implementation assignment union invalid: %s", strings.Join(mergeErrors, "; "))
	}
	out.ByAgent = make(map[string][]string, len(byAgent))
	for agent, blocks := range byAgent {
		out.ByAgent[agent] = implementationAssignmentIDs(blocks)
	}
	_ = checklist // reserved for future prompt helpers
	return out, nil
}

// ImplementationWorkloadEmpty reports whether both assignment authorities are empty.
func (s *Service) ImplementationWorkloadEmpty() (bool, error) {
	if s == nil || s.Store == nil {
		return false, errors.New("cycle service unavailable")
	}
	agents, err := s.ActiveImplementationAgents()
	if err != nil {
		return false, err
	}
	input, err := s.BuildImplementationAssignmentInput(agents)
	if err != nil {
		return false, err
	}
	for _, ids := range input.ByAgent {
		if len(ids) > 0 {
			return false, nil
		}
	}
	return true, nil
}

// ValidateImplementationReportUnion checks tasks_completed/tasks_remaining against assignment IDs.
func ValidateImplementationReportUnion(completed, remaining, assignment []string) error {
	if derr := reports.ValidateAssignmentUnion(completed, remaining, assignment); derr != nil {
		return fmt.Errorf("%s", derr.Error())
	}
	return nil
}

// CloseImplementationWhenAssignmentEmpty completes Implementation without an empty wave (PRD §7.3).
func (s *Service) CloseImplementationWhenAssignmentEmpty(summary string) error {
	if s == nil || s.Engine == nil {
		return errors.New("cycle service unavailable")
	}
	empty, err := s.ImplementationWorkloadEmpty()
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("implementation assignment is not empty")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	slog.Info("closing implementation with empty workload", "cycle", c.Number)
	return s.Engine.CloseImplementationWhenAssignmentEmpty(c.ID, summary)
}

// CompleteCycleWithDeferredTodos closes the cycle with completed_with_deferred_todos (PRD §8.3).
func (s *Service) CompleteCycleWithDeferredTodos(details DeferredCompletionDetails) error {
	if s == nil || s.Engine == nil {
		return errors.New("cycle service unavailable")
	}
	empty, err := s.ImplementationWorkloadEmpty()
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("cannot complete with deferred todos while blockers remain")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	slog.Info("completing cycle with deferred todos disposition", "cycle", c.Number, "todo_count", len(details.TodoIDs))
	return s.Engine.CompleteCycleWithDeferredTodosDisposition(c.ID, details)
}

// --- ToDo projection hooks (ADR-087) ---

// DeferFindingTodo persists a finding-derived pending ToDo with idempotency.
func (s *Service) DeferFindingTodo(idempotencyKey string, p store.CreateFindingTodoParams) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	return s.Store.DeferFindingTodoIdempotent(idempotencyKey, c.ID, p)
}

// CompletePendingTodo resolves a pending ToDo with projection idempotency.
func (s *Service) CompletePendingTodo(p store.CompleteTodoIdempotentParams) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	if p.CycleID == 0 {
		c, err := s.Store.GetActiveCycle()
		if err == nil {
			p.CycleID = c.ID
		} else if !errors.Is(err, store.ErrNoActiveCycle) {
			return err
		}
	}
	return s.Store.CompleteTodoIdempotent(p)
}

// ListPendingTodos returns pending project ToDos for Research adoption (ADR-089).
func (s *Service) ListPendingTodos() ([]store.Todo, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("cycle service unavailable")
	}
	return s.Store.ListPendingTodos()
}

// AdoptTodosForResearch adopts one or more pending ToDos into the active cycle.
func (s *Service) AdoptTodosForResearch(todoIDs []string, note string) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	for _, id := range dedupeStrings(todoIDs) {
		if err := s.AdoptPendingTodoForResearch(id, note); err != nil {
			return err
		}
	}
	return nil
}

// AdoptPendingTodoForResearch marks pending ToDos adopted by the active cycle (ADR-089).
func (s *Service) AdoptPendingTodoForResearch(todoID, note string) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	id := strings.TrimSpace(todoID)
	if _, err := s.Store.GetTodo(id); errors.Is(err, store.ErrNotFound) {
		promoted, perr := todos.PromoteSelectedLegacyLine(s.Store, s.ProjectDir, id)
		if perr != nil {
			return fmt.Errorf("unknown todo %q", id)
		}
		id = promoted
		slog.Info("promoted legacy pending line for research adoption", "todo_id", id, "cycle", c.Number)
	} else if err != nil {
		return err
	}
	slog.Info("adopting todo for research", "cycle", c.Number, "todo_id", id)
	if err := s.Store.AdoptTodo(id, c.ID, note); err != nil {
		return err
	}
	key := fmt.Sprintf("adopt-c%d-%s", c.Number, id)
	todoJSON, err := json.Marshal([]string{id})
	if err != nil {
		return err
	}
	if _, _, err := s.UpsertTodoProjectionOp(store.UpsertProjectionOpParams{
		CycleID:        c.ID,
		OpKind:         store.ProjectionOpAdopt,
		IdempotencyKey: key,
		Status:         store.ProjectionStatusIntentPersisted,
		TodoIDsJSON:    string(todoJSON),
	}); err != nil {
		return err
	}
	return s.ReconcileTodoProjection(key)
}

// ReleaseAdoptedTodosForCycle releases unresolved adopted ToDos after a non-validating terminal outcome.
func (s *Service) ReleaseAdoptedTodosForCycle(note string) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	slog.Info("releasing adopted todos for cycle", "cycle", c.Number)
	return s.Store.ReleaseAdoptedTodosForCycle(c.ID, note)
}

// ResolveAdoptedTodosForCycle marks adopted ToDos resolved after validated cycle completion.
func (s *Service) ResolveAdoptedTodosForCycle() error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	slog.Info("resolving adopted todos for validated completion", "cycle", c.Number)
	return s.Store.ResolveAdoptedTodosForCycle(c.ID)
}

// UpsertTodoProjectionOp records recoverable current-state.md projection intent.
func (s *Service) UpsertTodoProjectionOp(p store.UpsertProjectionOpParams) (store.TodoProjectionOp, bool, error) {
	if s == nil || s.Store == nil {
		return store.TodoProjectionOp{}, false, errors.New("cycle service unavailable")
	}
	if p.CycleID == 0 {
		c, err := s.Store.GetActiveCycle()
		if err != nil {
			return store.TodoProjectionOp{}, false, err
		}
		p.CycleID = c.ID
	}
	return s.Store.UpsertTodoProjectionOp(p)
}

// UpdateTodoProjectionOpStatus advances projection reconciliation state.
func (s *Service) UpdateTodoProjectionOpStatus(idempotencyKey, status, candidateSHA string) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	return s.Store.UpdateTodoProjectionOpStatus(idempotencyKey, status, candidateSHA)
}

// ReconcileTodoProjection installs and verifies current-state.md for a projection op (ADR-087).
func (s *Service) ReconcileTodoProjection(idempotencyKey string) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	return todos.ReconcileProjection(s.ProjectDir, s.Store, idempotencyKey)
}

// CycleCompletionDisposition returns stored terminal disposition fields for the active cycle.
func (s *Service) CycleCompletionDisposition() (disposition, dispositionJSON string, err error) {
	if s == nil || s.Store == nil {
		return "", "", errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return "", "", err
	}
	return s.Store.GetCycleCompletionDisposition(c.ID)
}

// SetCycleCompletionDisposition stores disposition metadata (used after projection verification).
func (s *Service) SetCycleCompletionDisposition(disposition string, details any) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	var raw string
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("encode disposition details: %w", err)
		}
		raw = string(b)
	}
	slog.Info("setting cycle completion disposition", "cycle", c.Number, "disposition", disposition)
	return s.Store.SetCycleCompletionDisposition(c.ID, disposition, raw)
}

type implementationChecklist struct {
	Linked bool
	Ready  bool
	Path   string
	Raw    string
}

func (s *Service) linkedOpenSpecTasks(c *store.Cycle) (path, raw string, linked, ready bool) {
	if s == nil || c == nil {
		return "", "", false, false
	}
	change := strings.TrimSpace(c.OpenspecChange)
	if change == "" || change == "." || change == ".." || strings.ContainsAny(change, `/\\`) {
		return "", "", false, false
	}
	linked = true
	path = filepath.Join(s.ProjectDir, "openspec", "changes", change, "tasks.md")
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return path, "", linked, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return path, "", linked, false
	}
	return path, string(data), linked, true
}
