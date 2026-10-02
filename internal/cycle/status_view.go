package cycle

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// StatusFindingsBlock is the additive findings section for status JSON (ADR-090).
type StatusFindingsBlock struct {
	Counts StatusFindingCounts `json:"counts"`
	Items  []StatusFindingRow  `json:"items"`
}

// StatusFindingCounts aggregates finding rows by status for the current cycle.
type StatusFindingCounts struct {
	Open         int `json:"open"`
	Reopened     int `json:"reopened"`
	Done         int `json:"done"`
	DeferredTodo int `json:"deferred_todo"`
}

// StatusFindingRow is one finding row in status JSON.
type StatusFindingRow struct {
	ID          string `json:"id"`
	SourceStage string `json:"sourceStage"`
	Owner       string `json:"owner"`
	Status      string `json:"status"`
	Round       int    `json:"round"`
	Issue       string `json:"issue"`
}

// StatusLoopBackRow is one loop-back history entry in status JSON.
type StatusLoopBackRow struct {
	From       string   `json:"from"`
	To         string   `json:"to"`
	Round      int      `json:"round"`
	FindingIDs []string `json:"findingIds"`
	OccurredAt string   `json:"occurredAt"`
}

// StatusTodosBlock is the additive ToDo summary for status JSON.
type StatusTodosBlock struct {
	Pending           int `json:"pending"`
	Adopted           int `json:"adopted"`
	DeferredFromCycle int `json:"deferredFromCycle"`
}

// StatusBlockedStage contains the additive operational pause and the safe
// corrective actions needed before the scheduler can continue that stage.
type StatusBlockedStage struct {
	Name              string               `json:"name"`
	Blockers          []StatusStageBlocker `json:"blockers"`
	RemainingBudgetMS int64                `json:"remainingBudgetMs,omitempty"`
}

// StatusStageBlocker is the non-secret projection of one active blocker.
type StatusStageBlocker struct {
	ID                  string   `json:"id"`
	Reason              string   `json:"reason"`
	AffectedCoverageIDs []string `json:"affectedCoverageIds,omitempty"`
	AffectedProfileIDs  []string `json:"affectedProfileIds,omitempty"`
	Uncertainty         string   `json:"uncertainty"`
	NextAction          string   `json:"nextAction"`
}

// StatusStageBudget exposes cumulative active time without execution payloads.
type StatusStageBudget struct {
	Name               string `json:"name"`
	State              string `json:"state"`
	ConsumedMS         int64  `json:"consumedMs"`
	RemainingMS        int64  `json:"remainingMs"`
	PauseReason        string `json:"pauseReason,omitempty"`
	InterruptionReason string `json:"interruptionReason,omitempty"`
}

func (s *Service) enrichStatusView(view *StatusView, c store.Cycle, stages []store.Stage) error {
	if s == nil || s.Store == nil || view == nil {
		return errors.New("cycle service unavailable")
	}
	findings, err := s.Store.ListFindingsByCycle(c.ID)
	if err != nil {
		return err
	}
	view.Findings = buildStatusFindings(findings)
	now := time.Now()
	if s.Engine != nil && s.Engine.Now != nil {
		now = s.Engine.Now()
	}
	blocked, err := buildStatusBlockedStages(s.Store, c.ID, stages, now)
	if err != nil {
		return err
	}
	view.BlockedStages = blocked
	view.StageBudgets, err = buildStatusStageBudgets(s.Store, c.ID, stages, now)
	if err != nil {
		return err
	}
	view.LoopBacks = buildStatusLoopBacks(s.Store, c.ID)
	todos, err := buildStatusTodos(s.Store, c.ID)
	if err != nil {
		return err
	}
	view.Todos = todos
	view.CompletionDisposition = statusCompletionDisposition(s.Store, c.ID)
	view.AvailableActions = statusAvailableActions(c, stages, view.Findings.Counts)
	return nil
}

func buildStatusStageBudgets(st *store.Store, cycleID int64, stages []store.Stage, now time.Time) ([]StatusStageBudget, error) {
	var budgets []StatusStageBudget
	for _, stage := range stages {
		budget, err := st.GetStageBudget(cycleID, stage.Name)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		remaining := budget.RemainingAt(now)
		budgets = append(budgets, StatusStageBudget{
			Name: displayStageName(stage.Name), State: string(budget.State),
			ConsumedMS: (budget.Limit - remaining).Milliseconds(), RemainingMS: remaining.Milliseconds(),
			PauseReason: budget.PauseReason, InterruptionReason: budget.InterruptionReason,
		})
	}
	slog.Debug("stage budget status projected", "cycle_id", cycleID, "count", len(budgets))
	return budgets, nil
}

func buildStatusFindings(findings []store.Finding) *StatusFindingsBlock {
	block := &StatusFindingsBlock{Items: make([]StatusFindingRow, 0, len(findings))}
	for _, f := range findings {
		switch f.Status {
		case store.FindingStatusOpen:
			block.Counts.Open++
		case store.FindingStatusReopened:
			block.Counts.Reopened++
		case store.FindingStatusDone:
			block.Counts.Done++
		case store.FindingStatusDeferredTodo:
			block.Counts.DeferredTodo++
		}
		block.Items = append(block.Items, StatusFindingRow{
			ID:          f.ID,
			SourceStage: f.SourceStage,
			Owner:       f.Owner,
			Status:      f.Status,
			Round:       f.Round,
			Issue:       strings.TrimSpace(f.Issue),
		})
	}
	return block
}

func buildStatusLoopBacks(st *store.Store, cycleID int64) []StatusLoopBackRow {
	events, err := st.ListEvents(cycleID, store.EventLoopBack, 0)
	if err != nil || len(events) == 0 {
		return nil
	}
	out := make([]StatusLoopBackRow, 0, len(events))
	roundByRoute := make(map[string]int)
	for _, ev := range events {
		row, ok := parseLoopBackEvent(ev)
		if !ok {
			continue
		}
		route := row.From + "->" + row.To
		roundByRoute[route]++
		row.Round = roundByRoute[route]
		out = append(out, row)
	}
	return out
}

func parseLoopBackEvent(ev store.Event) (StatusLoopBackRow, bool) {
	var payload struct {
		From       string   `json:"from"`
		To         string   `json:"to"`
		FindingIDs []string `json:"finding_ids"`
	}
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		return StatusLoopBackRow{}, false
	}
	from := strings.TrimSpace(payload.From)
	to := strings.TrimSpace(payload.To)
	if to == "" {
		to = "implementation"
	}
	if from == "" {
		return StatusLoopBackRow{}, false
	}
	ids := payload.FindingIDs
	if ids == nil {
		ids = []string{}
	}
	return StatusLoopBackRow{
		From:       from,
		To:         to,
		FindingIDs: ids,
		OccurredAt: strings.TrimSpace(ev.TS),
	}, true
}

func buildStatusTodos(st *store.Store, cycleID int64) (*StatusTodosBlock, error) {
	pending, err := st.CountTodosByStatus(store.TodoStatusPending)
	if err != nil {
		return nil, err
	}
	adopted, err := st.CountTodosByStatus(store.TodoStatusAdopted)
	if err != nil {
		return nil, err
	}
	deferred, err := st.ListFindingsByStatus(cycleID, store.FindingStatusDeferredTodo)
	if err != nil {
		return nil, err
	}
	return &StatusTodosBlock{
		Pending:           pending,
		Adopted:           adopted,
		DeferredFromCycle: len(deferred),
	}, nil
}

func statusCompletionDisposition(st *store.Store, cycleID int64) *string {
	disp, _, err := st.GetCycleCompletionDisposition(cycleID)
	if err != nil || strings.TrimSpace(disp) == "" {
		return nil
	}
	return &disp
}

func statusAvailableActions(c store.Cycle, stages []store.Stage, counts StatusFindingCounts) []string {
	if c.Status != store.CycleStatusActive {
		return nil
	}
	escalated := false
	for _, st := range stages {
		if st.Status == store.StageEscalated || st.Status == store.StageBlocked {
			escalated = true
			break
		}
	}
	if !escalated {
		return nil
	}
	actions := []string{"hero-continue"}
	if counts.Open+counts.Reopened > 0 {
		actions = append(actions, "hero-add-todo")
	}
	actions = append(actions, "hero-cancel", "hero-finish")
	return actions
}

func buildStatusBlockedStages(st *store.Store, cycleID int64, stages []store.Stage, now time.Time) ([]StatusBlockedStage, error) {
	var blocked []StatusBlockedStage
	for _, stage := range stages {
		if stage.Status != store.StageBlocked {
			continue
		}
		rows, err := st.ListActiveStageBlockers(cycleID, stage.Name)
		if err != nil {
			return nil, err
		}
		item := StatusBlockedStage{Name: displayStageName(stage.Name), Blockers: make([]StatusStageBlocker, 0, len(rows))}
		for _, row := range rows {
			reason := safeBlockedStatusReason(row.Reason)
			uncertainty, nextAction := safeBlockedStatusDetails(reason)
			item.Blockers = append(item.Blockers, StatusStageBlocker{
				ID: row.ID, Reason: reason,
				AffectedCoverageIDs: append([]string(nil), row.AffectedCoverageIDs...),
				AffectedProfileIDs:  append([]string(nil), row.AffectedProfileIDs...),
				Uncertainty:         uncertainty, NextAction: nextAction,
			})
		}
		budget, err := st.GetStageBudget(cycleID, stage.Name)
		if err == nil {
			remaining := budget.RemainingAt(now)
			if remaining > 0 {
				item.RemainingBudgetMS = remaining.Milliseconds()
			}
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		blocked = append(blocked, item)
	}
	return blocked, nil
}

// Blocker status data is sent to the TUI and addressed Telegram chats. The
// report's reason is reduced to a known value code and free-text diagnostics
// are replaced with deterministic actions, so a malformed harness/provider
// response cannot turn this status projection into a credential echo surface.
func safeBlockedStatusReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "service_unavailable", "browser_permission_unavailable", "selected_account_unavailable",
		"protected_access_unverified", "fixtures_unavailable", "method_unavailable",
		"tool_unavailable", "tool_missing", "tool_version", "tool_version_incompatible",
		"unsupported_authentication", "invalid_recipe", "test_access_disabled",
		"preparation_attempt_timed_out", "stage_budget_exhausted", "prerequisite_attempt_limit",
		"preparation_interrupted", "preparation_capability_unavailable", "prerequisite_check_failed",
		"credentials_unavailable", "account_unusable", "invalid_account", "login_unverified",
		"role_unverified", "context_unavailable", "execution_unavailable", "session_cleanup",
		"artifact_suppression":
		return strings.TrimSpace(reason)
	default:
		return "prerequisite_check_failed"
	}
}

func safeBlockedStatusDetails(reason string) (uncertainty, nextAction string) {
	switch reason {
	case "tool_unavailable", "tool_missing", "tool_version", "tool_version_incompatible", "method_unavailable":
		return "the selected browser method is unavailable or below its required version", "Configure the selected browser method using the project setup instructions; Hero does not install tools."
	case "credentials_unavailable", "selected_account_unavailable", "account_unusable", "invalid_account", "test_access_disabled":
		return "the selected test account could not be verified", "Enable Config → Test users and configure the selected account locally in the project-root .env.hero; credential values are not shown."
	case "unsupported_authentication":
		return "the login requires an unsupported interactive authentication step", "Use an approved test account with a supported login flow, then run /hero-continue."
	case "browser_permission_unavailable":
		return "the required browser permission is unavailable", "Grant the permission in the stage TUI session, then run /hero-continue."
	case "fixtures_unavailable":
		return "the planned local fixtures are unavailable", "Provide the declared local fixtures from Planning, then run /hero-continue."
	case "stage_budget_exhausted", "preparation_attempt_timed_out":
		return "the active execution budget or bounded preparation limit was reached", "Explicitly increase an expired stage budget before /hero-continue."
	case "preparation_interrupted":
		return "preparation was interrupted before validation", "Review the current prerequisites, then run /hero-continue."
	default:
		return "a required browser prerequisite could not be verified", "Review the browser plan, test users and selected-method configuration, then run /hero-continue."
	}
}
