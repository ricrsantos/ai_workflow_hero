package reports

import (
	"strings"
)

const (
	ValidationStatusPassed = "passed"
	ValidationStatusFailed = "failed"
)

func parseValidationStatus(root object) (string, *DiagnosticError) {
	status, err := requireString(root, "status")
	if err != nil {
		return "", err
	}
	status = strings.ToLower(status)
	switch status {
	case ValidationStatusPassed, ValidationStatusFailed:
		return status, nil
	default:
		return "", diag(CodeInvalidEnum, "status", status, "status must be passed or failed")
	}
}

func requireEmptyFailuresOnPass(status string, field string, count int) *DiagnosticError {
	if status == ValidationStatusPassed && count != 0 {
		return diag(CodeInvalidEnum, field, "", "successful reports must use an empty array")
	}
	return nil
}

// ValidateFailedClose ensures a failed-stage close has actionable findings.
func ValidateFailedClose(sourceStage string, status string, sddAmbiguity bool, entries []FailureEntry, ctx DecodeContext) *DiagnosticError {
	if status != ValidationStatusFailed {
		return nil
	}
	if sourceStage == SourceJudge && sddAmbiguity {
		return nil
	}
	if ctx.Actionable != nil {
		return ctx.Actionable.HasActionableFindings(sourceStage, entries)
	}
	return countActionableFindings(sourceStage, entries)
}
