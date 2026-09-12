package cycle

import (
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
)

// StageAgentAuditBody is the JSON envelope stored in a conversation body for
// a stage-agent assignment or result. Body is kept as the exact string passed
// by the caller (including whitespace), while stage, agent, and wave make the
// handoff auditable without requiring a schema migration.
//
// Assignments persist ordered TaskIDs, which may mix task-* and find-* IDs.
// Results always retain the raw agent output in Body. When the scheduler has
// validated a report, TasksCompleted and TasksRemaining store the normalized
// assignment item IDs and ResultValidated is true. Raw-only results set
// ResultValidated to false so consumers can distinguish unparsed output.
type StageAgentAuditBody struct {
	Stage string `json:"stage"`
	Agent string `json:"agent"`
	Wave  int    `json:"wave"`
	// TaskIDs is populated for assignment records. The order is the dispatch
	// order, which makes the exact assignment auditable without parsing Body.
	TaskIDs []string `json:"task_ids,omitempty"`
	// TasksCompleted and TasksRemaining are populated only for validated result
	// records. Historical field names refer to assignment item IDs (task-* or
	// find-*), not OpenSpec checkboxes alone.
	TasksCompleted  []string `json:"tasks_completed,omitempty"`
	TasksRemaining  []string `json:"tasks_remaining,omitempty"`
	ResultValidated *bool    `json:"result_validated,omitempty"`
	Body            string   `json:"body"`
}

func normalizeStageAgentTaskIDs(kind string, taskIDs []string) ([]string, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(taskIDs))
	seen := make(map[string]struct{}, len(taskIDs))
	for i, raw := range taskIDs {
		id, err := normalizeStageAgentAssignmentID(kind, i, raw)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("record %s: duplicate task id %q", kind, id)
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func normalizeStageAgentValidatedResultIDs(kind string, completed, remaining []string) ([]string, []string, error) {
	normCompleted, err := normalizeStageAgentReportIDList(kind, "tasks_completed", completed)
	if err != nil {
		return nil, nil, err
	}
	normRemaining, err := normalizeStageAgentReportIDList(kind, "tasks_remaining", remaining)
	if err != nil {
		return nil, nil, err
	}
	if derr := reports.ValidateAssignmentUnion(normCompleted, normRemaining, unionAssignmentIDs(normCompleted, normRemaining)); derr != nil {
		return nil, nil, fmt.Errorf("record %s: %s", kind, derr.Error())
	}
	return normCompleted, normRemaining, nil
}

func normalizeStageAgentAssignmentID(kind string, index int, raw string) (string, error) {
	id, derr := reports.NormalizeAssignmentID(raw)
	if derr != nil {
		return "", fmt.Errorf("record %s: assignment id at index %d: %s", kind, index, derr.Error())
	}
	return id, nil
}

func normalizeStageAgentReportIDList(kind, field string, values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for i, raw := range values {
		id, derr := reports.NormalizeAssignmentID(raw)
		if derr != nil {
			return nil, fmt.Errorf("record %s: %s at index %d: %s", kind, field, i, derr.Error())
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("record %s: duplicate %s id %q", kind, field, id)
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func unionAssignmentIDs(completed, remaining []string) []string {
	seen := make(map[string]struct{}, len(completed)+len(remaining))
	union := make([]string, 0, len(completed)+len(remaining))
	for _, id := range completed {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		union = append(union, id)
	}
	for _, id := range remaining {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		union = append(union, id)
	}
	return union
}

func stageAgentAuditEnvelope(kind string, stage, agent string, wave int, body string, taskIDs, completed, remaining []string, resultValidated *bool) (StageAgentAuditBody, error) {
	stage = strings.TrimSpace(stage)
	agent = strings.TrimSpace(agent)

	normalizedTaskIDs, err := normalizeStageAgentTaskIDs(kind, taskIDs)
	if err != nil {
		return StageAgentAuditBody{}, err
	}
	var normalizedCompleted, normalizedRemaining []string
	if resultValidated != nil && *resultValidated {
		normalizedCompleted, normalizedRemaining, err = normalizeStageAgentValidatedResultIDs(kind, completed, remaining)
		if err != nil {
			return StageAgentAuditBody{}, err
		}
	} else if len(completed) > 0 || len(remaining) > 0 {
		return StageAgentAuditBody{}, fmt.Errorf("record %s: validated result IDs require result_validated=true", kind)
	}

	switch kind {
	case ConversationKindStageAgentAssignment:
		if resultValidated != nil {
			return StageAgentAuditBody{}, fmt.Errorf("record %s: result_validated is not allowed on assignments", kind)
		}
		if len(normalizedCompleted) > 0 || len(normalizedRemaining) > 0 {
			return StageAgentAuditBody{}, fmt.Errorf("record %s: tasks_completed and tasks_remaining are not allowed on assignments", kind)
		}
	case ConversationKindStageAgentResult:
		if len(normalizedTaskIDs) > 0 {
			return StageAgentAuditBody{}, fmt.Errorf("record %s: task_ids are not allowed on results", kind)
		}
	default:
		return StageAgentAuditBody{}, fmt.Errorf("record %s: unknown conversation kind", kind)
	}

	return StageAgentAuditBody{
		Stage:           stage,
		Agent:           agent,
		Wave:            wave,
		TaskIDs:         normalizedTaskIDs,
		TasksCompleted:  normalizedCompleted,
		TasksRemaining:  normalizedRemaining,
		ResultValidated: resultValidated,
		Body:            body,
	}, nil
}
