package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// formatLifecycleEventSummary renders ID-first lifecycle rows (UI-C15-001 §12).
func formatLifecycleEventSummary(e store.Event) string {
	switch e.Type {
	case store.EventFindingCreated:
		return formatFindingCreatedEvent(e.PayloadJSON)
	case store.EventFindingDone:
		return formatFindingDoneEvent(e.PayloadJSON)
	case store.EventFindingReopened:
		return formatFindingReopenedEvent(e.PayloadJSON)
	case store.EventFindingDeferred:
		return formatFindingDeferredEvent(e.PayloadJSON)
	case store.EventFindingDeferredRecurrence:
		return formatFindingDeferredRecurrenceEvent(e.PayloadJSON)
	case store.EventTodoAdopted:
		return formatTodoAdoptedEvent(e.PayloadJSON)
	case store.EventTodoReleased:
		return formatTodoReleasedEvent(e.PayloadJSON)
	case store.EventTodoCompletedManual:
		return formatTodoCompletedManualEvent(e.PayloadJSON)
	case store.EventCycleCompletedDeferredTodos:
		return formatCycleCompletedDeferredEvent(e.PayloadJSON)
	default:
		payload := e.PayloadJSON
		if len(payload) > 48 {
			payload = payload[:45] + "..."
		}
		return payload
	}
}

func formatFindingCreatedEvent(payloadJSON string) string {
	var p struct {
		FindingID   string `json:"finding_id"`
		SourceStage string `json:"source_stage"`
		Owner       string `json:"owner"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return payloadJSON
	}
	return fmt.Sprintf("%s · %s → %s",
		strings.TrimSpace(p.FindingID),
		statusSourceStageLabel(p.SourceStage),
		findingOwnerLabel(p.Owner),
	)
}

func formatFindingDoneEvent(payloadJSON string) string {
	var p struct {
		FindingID   string `json:"finding_id"`
		SourceStage string `json:"source_stage"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return payloadJSON
	}
	stage := statusSourceStageLabel(p.SourceStage)
	if stage == "" {
		stage = "Implementation"
	}
	return fmt.Sprintf("%s · %s", strings.TrimSpace(p.FindingID), stage)
}

func formatFindingReopenedEvent(payloadJSON string) string {
	var p struct {
		FindingID   string `json:"finding_id"`
		SourceStage string `json:"source_stage"`
		Round       int    `json:"round"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return payloadJSON
	}
	return fmt.Sprintf("%s · %s · round %d",
		strings.TrimSpace(p.FindingID),
		statusSourceStageLabel(p.SourceStage),
		p.Round,
	)
}

func formatFindingDeferredEvent(payloadJSON string) string {
	var p struct {
		FindingID   string `json:"finding_id"`
		SourceStage string `json:"source_stage"`
		CycleNumber int    `json:"cycle_number"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return payloadJSON
	}
	from := statusSourceStageLabel(p.SourceStage)
	if p.CycleNumber > 0 && from != "" {
		return fmt.Sprintf("%s · from C%d/%s", strings.TrimSpace(p.FindingID), p.CycleNumber, from)
	}
	return strings.TrimSpace(p.FindingID)
}

func formatFindingDeferredRecurrenceEvent(payloadJSON string) string {
	var p struct {
		FindingID   string `json:"finding_id"`
		SourceStage string `json:"source_stage"`
		Round       int    `json:"round"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return payloadJSON
	}
	return fmt.Sprintf("%s · %s · round %d recurrence",
		strings.TrimSpace(p.FindingID),
		statusSourceStageLabel(p.SourceStage),
		p.Round,
	)
}

func formatTodoAdoptedEvent(payloadJSON string) string {
	var p struct {
		TodoID      string `json:"todo_id"`
		CycleNumber int    `json:"cycle_number"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return payloadJSON
	}
	if p.CycleNumber > 0 {
		return fmt.Sprintf("%s · C%d", strings.TrimSpace(p.TodoID), p.CycleNumber)
	}
	return strings.TrimSpace(p.TodoID)
}

func formatTodoReleasedEvent(payloadJSON string) string {
	return formatTodoAdoptedEvent(payloadJSON)
}

func formatTodoCompletedManualEvent(payloadJSON string) string {
	var p struct {
		TodoID    string `json:"todo_id"`
		FindingID string `json:"finding_id"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return payloadJSON
	}
	id := strings.TrimSpace(p.FindingID)
	if id == "" {
		id = strings.TrimSpace(p.TodoID)
	}
	return fmt.Sprintf("%s · outside cycle", id)
}

func formatCycleCompletedDeferredEvent(payloadJSON string) string {
	var p struct {
		Disposition string   `json:"disposition"`
		TodoIDs     []string `json:"todo_ids"`
		Count       int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return payloadJSON
	}
	if len(p.TodoIDs) > 0 {
		return fmt.Sprintf("%s · %s", p.Disposition, strings.Join(p.TodoIDs, ", "))
	}
	if p.Count > 0 {
		return fmt.Sprintf("%s · %d deferred", p.Disposition, p.Count)
	}
	return strings.TrimSpace(p.Disposition)
}

func statusSourceStageLabel(sourceStage string) string {
	switch strings.TrimSpace(strings.ToLower(sourceStage)) {
	case "qa":
		return "QA"
	case "qa_end_to_end":
		return "QA End-to-End"
	case "browser_ui_validation":
		return "Browser UI"
	default:
		return cycle.DisplayStageName(sourceStage)
	}
}
