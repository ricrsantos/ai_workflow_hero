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
	CodeReproTestFailed         Code = "repro_test_failed"
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

// CodeFieldRenamed marks a key Hero normalized onto the contract (reopenId →
// reopen_id). It is a warning, never a rejection.
const CodeFieldRenamed Code = "field_renamed"

// ReportWarning is a contract deviation Hero tolerated. Warnings never block a
// stage: the report is decoded and persisted, and the warning is surfaced so
// the agent's prompt can be corrected.
type ReportWarning struct {
	Code  Code
	Field string
	Value string
	Rule  string
}

func (w ReportWarning) String() string {
	field := strings.TrimSpace(w.Field)
	if field == "" {
		return fmt.Sprintf("%s: %s", w.Code, w.Rule)
	}
	if strings.TrimSpace(w.Value) == "" {
		return fmt.Sprintf("%s: %s — %s", w.Code, field, w.Rule)
	}
	return fmt.Sprintf("%s: %s → %s", w.Code, field, w.Value)
}

func warn(code Code, field, value, rule string) ReportWarning {
	return ReportWarning{Code: code, Field: field, Value: value, Rule: rule}
}

// collect appends decoded warnings onto an optional sink. A nil sink means the
// caller does not surface warnings; tolerance does not depend on collecting them.
func collect(sink *[]ReportWarning, warnings []ReportWarning) {
	if sink == nil || len(warnings) == 0 {
		return
	}
	*sink = append(*sink, warnings...)
}

func collectOne(sink *[]ReportWarning, w ReportWarning) {
	if sink == nil {
		return
	}
	*sink = append(*sink, w)
}
