package reports

import (
	"fmt"
	"strings"
)

// Code is a stable diagnostic identifier for invalid reports.
type Code string

const (
	CodeInvalidJSON             Code = "invalid_json"
	CodeUnknownField            Code = "unknown_field"
	CodeMissingField            Code = "missing_field"
	CodeInvalidEnum             Code = "invalid_enum"
	CodeInvalidOwner            Code = "invalid_owner"
	CodeUnknownReopenID         Code = "unknown_reopen_id"
	CodeDuplicateID             Code = "duplicate_id"
	CodeOverlappingArrays       Code = "overlapping_arrays"
	CodeAssignmentUnionMismatch Code = "assignment_union_mismatch"
	CodeUnassignedID            Code = "unassigned_id"
	CodeFalseAcceptanceGate     Code = "false_acceptance_gate"
	CodeNonemptyEmptyAssignment Code = "nonempty_empty_assignment"
	CodeNoActionableFinding     Code = "no_actionable_finding"
)

// DiagnosticError is a field-specific validation failure returned by decoders.
type DiagnosticError struct {
	Code  Code
	Field string
	Value string
	Rule  string
}

func (e *DiagnosticError) Error() string {
	if e == nil {
		return ""
	}
	field := strings.TrimSpace(e.Field)
	if field == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Rule)
	}
	if strings.TrimSpace(e.Value) == "" {
		return fmt.Sprintf("%s: %s — %s", e.Code, field, e.Rule)
	}
	return fmt.Sprintf("%s: %s — %s (value: %s)", e.Code, field, e.Rule, e.Value)
}

// Diagnostics collects multiple errors; the first is usually returned alone.
type Diagnostics []*DiagnosticError

func (d Diagnostics) Error() string {
	if len(d) == 0 {
		return ""
	}
	return d[0].Error()
}

func (d Diagnostics) First() *DiagnosticError {
	if len(d) == 0 {
		return nil
	}
	return d[0]
}

func diag(code Code, field, value, rule string) *DiagnosticError {
	return &DiagnosticError{Code: code, Field: field, Value: value, Rule: rule}
}
