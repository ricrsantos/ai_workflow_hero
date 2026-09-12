package cycle

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/engine"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/todos"
)

// AddFindingTodosResult summarizes deterministic add-todo outcomes.
type AddFindingTodosResult struct {
	TodoIDs         []string
	PartialDeferral bool
	CycleCompleted  bool
}

var errNotEscalated = errors.New("no escalated stage; add-todo is available only when a stage loop is Escalated")

// AddFindingTodos defers open/reopened findings to pending ToDos during Escalated triage.
func (s *Service) AddFindingTodos(findingIDs []string, idempotencyPrefix string) (AddFindingTodosResult, error) {
	var empty AddFindingTodosResult
	if s == nil || s.Store == nil {
		return empty, errors.New("cycle service unavailable")
	}
	if len(findingIDs) == 0 {
		return empty, fmt.Errorf("at least one finding id is required")
	}
	if _, err := s.requireEscalatedStage(); err != nil {
		return empty, err
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return empty, err
	}
	prefix := strings.TrimSpace(idempotencyPrefix)
	if prefix == "" {
		prefix = fmt.Sprintf("defer-c%d-", c.Number)
	}

	unique := dedupeStrings(findingIDs)
	findings := make([]store.Finding, 0, len(unique))
	for _, id := range unique {
		f, err := s.Store.GetFinding(c.ID, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return empty, fmt.Errorf("unknown finding %q", id)
			}
			return empty, err
		}
		switch f.Status {
		case store.FindingStatusOpen, store.FindingStatusReopened, store.FindingStatusDeferredTodo:
		default:
			return empty, fmt.Errorf("finding %q is %s, expected open or reopened", id, f.Status)
		}
		findings = append(findings, f)
	}

	var todoIDs []string
	for _, f := range findings {
		key := prefix + f.ID
		p := store.CreateFindingTodoParams{
			ID:                 f.ID,
			OriginCycleID:      c.ID,
			OriginSourceStage:  f.SourceStage,
			Summary:            f.Issue,
			AcceptanceCriteria: f.AcceptanceCriteria,
		}
		if err := s.DeferFindingTodo(key, p); err != nil {
			return empty, err
		}
		if err := s.ReconcileTodoProjection(key); err != nil {
			return empty, err
		}
		todoIDs = append(todoIDs, f.ID)
	}
	sort.Strings(todoIDs)

	emptyWorkload, err := s.ImplementationWorkloadEmpty()
	if err != nil {
		return empty, err
	}
	out := AddFindingTodosResult{TodoIDs: todoIDs}
	if emptyWorkload {
		actionable, err := s.Store.ListActionableFindings(c.ID, "")
		if err != nil {
			return empty, err
		}
		if len(actionable) == 0 {
			originStages := uniqueOriginStages(findings)
			details := engine.DeferredCompletionDetails{
				TodoIDs:      todoIDs,
				OriginStages: originStages,
				Summary:      "cycle completed with deferred ToDos",
			}
			slog.Info("defer-all blockers cleared; completing cycle with deferred todos", "cycle", c.Number, "todo_count", len(todoIDs))
			if err := s.CompleteCycleWithDeferredTodos(details); err != nil {
				return empty, err
			}
			out.CycleCompleted = true
		} else {
			out.PartialDeferral = true
			slog.Info("partial finding deferral with remaining actionable findings", "cycle", c.Number, "todo_count", len(todoIDs), "remaining_findings", len(actionable))
		}
	} else {
		out.PartialDeferral = true
		slog.Info("partial finding deferral", "cycle", c.Number, "todo_count", len(todoIDs))
	}
	return out, nil
}

// CompleteManualTodos resolves pending ToDos with a required safe resolution note.
// An active cycle is optional: pending ToDos may be completed between cycles.
func (s *Service) CompleteManualTodos(todoIDs []string, note, idempotencyPrefix string) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("resolution note is required")
	}
	if redact.HasToken(note) {
		return fmt.Errorf("resolution note contains a forbidden token pattern")
	}
	if len(todoIDs) == 0 {
		return fmt.Errorf("at least one todo id is required")
	}
	var (
		cycleID  int64
		cycleNum int
		hasCycle bool
	)
	if c, err := s.Store.GetActiveCycle(); err == nil {
		cycleID = c.ID
		cycleNum = c.Number
		hasCycle = true
	} else if !errors.Is(err, store.ErrNoActiveCycle) {
		return err
	}
	prefix := strings.TrimSpace(idempotencyPrefix)
	if prefix == "" {
		if hasCycle {
			prefix = fmt.Sprintf("complete-c%d-", cycleNum)
		} else {
			prefix = "complete-nocycle-"
		}
	}
	completed := 0
	for _, raw := range dedupeStrings(todoIDs) {
		id := raw
		t, err := s.Store.GetTodo(id)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				return err
			}
			promoted, perr := todos.PromoteSelectedLegacyLine(s.Store, s.ProjectDir, id)
			if perr != nil {
				return fmt.Errorf("unknown todo %q", id)
			}
			id = promoted
			t, err = s.Store.GetTodo(id)
			if err != nil {
				return err
			}
			slog.Info("promoted legacy pending line for manual complete", "todo_id", id)
		}
		if t.Status == store.TodoStatusAdopted {
			if hasCycle && t.AdoptedCycleID == cycleID {
				return fmt.Errorf("todo %q is adopted by the active cycle", id)
			}
			return fmt.Errorf("todo %q is adopted and cannot be completed manually", id)
		}
		if t.Status != store.TodoStatusPending && t.Status != store.TodoStatusResolved {
			return fmt.Errorf("todo %q is %s; only pending ToDos can be completed manually", id, t.Status)
		}
		key := prefix + id
		if err := s.CompletePendingTodo(store.CompleteTodoIdempotentParams{
			TodoID:         id,
			ResolutionNote: note,
			IdempotencyKey: key,
			CycleID:        cycleID,
		}); err != nil {
			return err
		}
		if err := s.ReconcileTodoProjection(key); err != nil {
			return err
		}
		completed++
	}
	if hasCycle {
		slog.Info("manual todo completion", "cycle", cycleNum, "count", completed)
	} else {
		slog.Info("manual todo completion", "count", completed)
	}
	return nil
}

func (s *Service) requireEscalatedStage() (string, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return "", err
	}
	stages, err := s.Store.ListStages(c.ID)
	if err != nil {
		return "", err
	}
	for _, st := range stages {
		if st.Status == store.StageEscalated {
			return st.Name, nil
		}
	}
	return "", errNotEscalated
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func uniqueOriginStages(findings []store.Finding) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, f := range findings {
		st := strings.TrimSpace(f.SourceStage)
		if st == "" {
			continue
		}
		if _, ok := seen[st]; ok {
			continue
		}
		seen[st] = struct{}{}
		out = append(out, st)
	}
	sort.Strings(out)
	return out
}
