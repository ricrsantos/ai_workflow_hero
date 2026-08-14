package cycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// ApprovalEntry is one requested/approved/rejected/escalated/continued event.
type ApprovalEntry struct {
	Stage  string `json:"stage"`
	Event  string `json:"event"`
	TS     string `json:"ts"`
	Detail string `json:"detail,omitempty"`
}

// ApprovalsView is the Approvals TUI/CLI listing for the active cycle.
type ApprovalsView struct {
	CycleNumber int             `json:"cycleNumber"`
	Title       string          `json:"title"`
	Pending     string          `json:"pending,omitempty"`
	Entries     []ApprovalEntry `json:"entries"`
}

var approvalEventTypes = []string{
	store.EventPendingApproval,
	store.EventApproved,
	store.EventRejected,
	store.EventEscalated,
	store.EventContinued,
}

// Approvals returns pending stage plus chronological approval history.
func (s *Service) Approvals() (ApprovalsView, error) {
	if s == nil || s.Store == nil {
		return ApprovalsView{}, nil
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		if errors.Is(err, store.ErrNoActiveCycle) {
			return ApprovalsView{}, nil
		}
		return ApprovalsView{}, err
	}
	stages, err := s.Store.ListStages(c.ID)
	if err != nil {
		return ApprovalsView{}, err
	}
	events, err := s.Store.ListEventsByTypes(c.ID, approvalEventTypes)
	if err != nil {
		return ApprovalsView{}, err
	}

	view := ApprovalsView{
		CycleNumber: c.Number,
		Title:       c.Title,
		Pending:     pendingStageName(stages),
	}
	for _, e := range events {
		entry := ApprovalEntry{
			Stage:  displayStageName(eventStageName(e.PayloadJSON)),
			Event:  approvalEventLabel(e.Type),
			TS:     e.TS,
			Detail: approvalEventDetail(e.Type, e.PayloadJSON),
		}
		if entry.Stage == "" {
			entry.Stage = "—"
		}
		view.Entries = append(view.Entries, entry)
	}
	return view, nil
}

func pendingStageName(stages []store.Stage) string {
	for _, st := range stages {
		if st.Status == store.StagePendingApproval {
			return displayStageName(st.Name)
		}
	}
	return ""
}

func approvalEventLabel(eventType string) string {
	switch eventType {
	case store.EventPendingApproval:
		return "requested"
	case store.EventApproved:
		return "approved"
	case store.EventRejected:
		return "rejected"
	case store.EventEscalated:
		return "escalated"
	case store.EventContinued:
		return "continued"
	default:
		return eventType
	}
}

type approvalPayload struct {
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
	Extra  int    `json:"extra"`
}

func eventStageName(payloadJSON string) string {
	var p approvalPayload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return ""
	}
	return strings.TrimSpace(p.Stage)
}

func approvalEventDetail(eventType, payloadJSON string) string {
	var p approvalPayload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return ""
	}
	switch eventType {
	case store.EventRejected, store.EventEscalated:
		return strings.TrimSpace(p.Reason)
	case store.EventContinued:
		if p.Extra > 0 {
			return fmt.Sprintf("+%d", p.Extra)
		}
	}
	return ""
}
