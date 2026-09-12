package reports

import (
	"encoding/json"
	"strings"
)

// ImplementationReport is the decoded Implementation stage-agent payload.
type ImplementationReport struct {
	Stage           string
	Agent           string
	Status          string
	TasksCompleted  []string
	TasksRemaining  []string
	TestsPassed     bool
	AcceptanceGates map[string]bool
	Summary         string
	Blocker         string
	NextAction      string
}

var implementationAllowed = map[string]struct{}{
	"stage": {}, "agent": {}, "status": {}, "tasks_completed": {}, "tasks_remaining": {},
	"tests_passed": {}, "acceptance_gates": {}, "summary": {}, "blocker": {}, "next_action": {},
	"files_changed": {}, // informational; ignored by the scheduler
}

const (
	ImplStatusComplete = "complete"
	ImplStatusPartial  = "partial"
	ImplStatusBlocked  = "blocked"
)

// DecodeImplementation validates and decodes an Implementation JSON report.
func DecodeImplementation(data []byte, expectedAgent string, assignment []string, ctx DecodeContext) (*ImplementationReport, *DiagnosticError) {
	root, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	if err := unknownFields(root, implementationAllowed, ""); err != nil {
		return nil, err
	}

	stage, err := requireString(root, "stage")
	if err != nil {
		return nil, err
	}
	if stage != SourceImplementation {
		return nil, diag(CodeInvalidEnum, "stage", stage, "stage must be implementation")
	}

	agent, err := requireString(root, "agent")
	if err != nil {
		return nil, err
	}
	expectedAgent = strings.TrimSpace(expectedAgent)
	if expectedAgent != "" && agent != expectedAgent {
		return nil, diag(CodeInvalidEnum, "agent", agent, "agent must match the assigned implementation agent")
	}

	statusRaw, err := requireString(root, "status")
	if err != nil {
		return nil, err
	}
	status := strings.ToLower(statusRaw)
	switch status {
	case ImplStatusComplete, ImplStatusPartial, ImplStatusBlocked:
	default:
		return nil, diag(CodeInvalidEnum, "status", statusRaw, "status must be complete, partial, or blocked")
	}

	testsPassed, err := requireBool(root, "tests_passed")
	if err != nil {
		return nil, err
	}

	gates, err := decodeAcceptanceGates(root)
	if err != nil {
		return nil, err
	}

	completed, err := decodeAssignmentIDs(root, "tasks_completed")
	if err != nil {
		return nil, err
	}
	remaining, err := decodeAssignmentIDs(root, "tasks_remaining")
	if err != nil {
		return nil, err
	}

	if err := validateAssignmentLists(completed, remaining, assignment); err != nil {
		return nil, err
	}

	summary, err := requireString(root, "summary")
	if err != nil {
		return nil, err
	}

	report := &ImplementationReport{
		Stage:           stage,
		Agent:           agent,
		Status:          status,
		TasksCompleted:  completed,
		TasksRemaining:  remaining,
		TestsPassed:     testsPassed,
		AcceptanceGates: gates,
		Summary:         summary,
	}

	if status == ImplStatusComplete {
		if len(remaining) != 0 {
			return nil, diag(CodeInvalidEnum, "tasks_remaining", "", "complete reports must have an empty tasks_remaining array")
		}
		for key, value := range gates {
			if !value {
				return nil, diag(CodeFalseAcceptanceGate, "acceptance_gates."+key, "false",
					"complete reports require all canonical acceptance gates to be true")
			}
		}
	} else {
		blocker, err := requireString(root, "blocker")
		if err != nil {
			err.Field = "blocker"
			return nil, err
		}
		next, err := requireString(root, "next_action")
		if err != nil {
			err.Field = "next_action"
			return nil, err
		}
		report.Blocker = blocker
		report.NextAction = next
	}

	return report, nil
}

func decodeAcceptanceGates(root object) (map[string]bool, *DiagnosticError) {
	raw, ok := root["acceptance_gates"]
	if !ok {
		return nil, diag(CodeMissingField, "acceptance_gates", "", "field is required")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, diag(CodeInvalidEnum, "acceptance_gates", truncateValue(string(raw)), "acceptance_gates must be an object")
	}
	allowed := map[string]struct{}{
		"completed_tasks_verified": {},
		"task_ownership_respected": {},
		"required_tests_passed":    {},
	}
	if err := unknownFields(fields, allowed, "acceptance_gates."); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(allowed))
	for key := range allowed {
		sub, ok := fields[key]
		if !ok {
			return nil, diag(CodeMissingField, "acceptance_gates."+key, "", "canonical gate is required")
		}
		if string(sub) == "null" {
			return nil, diag(CodeMissingField, "acceptance_gates."+key, "null", "canonical gate is required")
		}
		var value bool
		if err := json.Unmarshal(sub, &value); err != nil {
			return nil, diag(CodeInvalidEnum, "acceptance_gates."+key, truncateValue(string(sub)), "gate must be a boolean")
		}
		out[key] = value
	}
	return out, nil
}

func decodeAssignmentIDs(root object, name string) ([]string, *DiagnosticError) {
	values, err := requireStringArray(root, name)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(values))
	seen := map[string]int{}
	for i, raw := range values {
		id, derr := normalizeAssignmentID(raw)
		if derr != nil {
			derr.Field = name + "[" + itoa(i) + "]"
			return nil, derr
		}
		if prev, ok := seen[id]; ok {
			return nil, diag(CodeDuplicateID, name+"["+itoa(i)+"]", id,
				"duplicate of "+name+"["+itoa(prev)+"]")
		}
		seen[id] = i
		out = append(out, id)
	}
	return out, nil
}

func normalizeAssignmentID(raw string) (string, *DiagnosticError) {
	id := strings.TrimSpace(raw)
	if strings.HasPrefix(id, "[") && strings.HasSuffix(id, "]") {
		id = strings.TrimSuffix(strings.TrimPrefix(id, "["), "]")
		id = strings.TrimSpace(id)
	}
	if strings.HasPrefix(id, "task-") {
		if strings.TrimSpace(strings.TrimPrefix(id, "task-")) == "" {
			return "", diag(CodeInvalidEnum, "", raw, "task ID is invalid")
		}
		return id, nil
	}
	if strings.HasPrefix(id, "find-") {
		if strings.TrimSpace(strings.TrimPrefix(id, "find-")) == "" {
			return "", diag(CodeInvalidEnum, "", raw, "finding ID is invalid")
		}
		return id, nil
	}
	return "", diag(CodeInvalidEnum, "", raw, "assignment ID must be a task-* or find-* ID")
}

// ValidateAssignmentUnion checks disjoint completed/remaining sets against the exact wave assignment.
func ValidateAssignmentUnion(completed, remaining, assignment []string) *DiagnosticError {
	return validateAssignmentLists(completed, remaining, assignment)
}

// NormalizeAssignmentID accepts task-* and find-* assignment IDs.
func NormalizeAssignmentID(raw string) (string, *DiagnosticError) {
	return normalizeAssignmentID(raw)
}

func validateAssignmentLists(completed, remaining []string, assignment []string) *DiagnosticError {
	assignSet := make(map[string]struct{}, len(assignment))
	for _, id := range assignment {
		assignSet[id] = struct{}{}
	}

	if len(assignSet) == 0 {
		if len(completed) > 0 || len(remaining) > 0 {
			return diag(CodeNonemptyEmptyAssignment, "tasks_completed", strings.Join(completed, ","),
				"verification wave assigned no IDs; completed and remaining must be empty")
		}
		return nil
	}

	seen := map[string]string{}
	for i, id := range completed {
		if prev, ok := seen[id]; ok {
			if prev != "tasks_completed" {
				return diag(CodeOverlappingArrays, "tasks_completed["+itoa(i)+"]", id,
					"ID also appears in "+prev)
			}
			return diag(CodeDuplicateID, "tasks_completed["+itoa(i)+"]", id, "duplicate ID in tasks_completed")
		}
		seen[id] = "tasks_completed"
		if _, ok := assignSet[id]; !ok {
			return diag(CodeUnassignedID, "tasks_completed["+itoa(i)+"]", id, "ID was not in the wave assignment")
		}
	}
	for i, id := range remaining {
		if prev, ok := seen[id]; ok {
			return diag(CodeOverlappingArrays, "tasks_remaining["+itoa(i)+"]", id,
				"ID also appears in "+prev)
		}
		seen[id] = "tasks_remaining"
		if _, ok := assignSet[id]; !ok {
			return diag(CodeUnassignedID, "tasks_remaining["+itoa(i)+"]", id, "ID was not in the wave assignment")
		}
	}

	reported := make(map[string]struct{}, len(completed)+len(remaining))
	for _, id := range completed {
		reported[id] = struct{}{}
	}
	for _, id := range remaining {
		reported[id] = struct{}{}
	}
	for id := range assignSet {
		if _, ok := reported[id]; !ok {
			return diag(CodeAssignmentUnionMismatch, "tasks_completed", id,
				"assigned ID missing from completed and remaining")
		}
	}
	return nil
}
