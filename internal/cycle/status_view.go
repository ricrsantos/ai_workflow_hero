package cycle

import (
	"encoding/json"
	"errors"
	"strings"

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

func (s *Service) enrichStatusView(view *StatusView, c store.Cycle, stages []store.Stage) error {
	if s == nil || s.Store == nil || view == nil {
		return errors.New("cycle service unavailable")
	}
	findings, err := s.Store.ListFindingsByCycle(c.ID)
	if err != nil {
		return err
	}
	view.Findings = buildStatusFindings(findings)
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
		if st.Status == store.StageEscalated {
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
