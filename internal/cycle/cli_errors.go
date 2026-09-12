package cycle

import (
	"encoding/json"
	"errors"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
)

// StructuredDiagnostic is JSON-safe CLI validation output (PRD-C15 §7.1).
type StructuredDiagnostic struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

func structuredFromDiagnostic(d *reports.DiagnosticError) StructuredDiagnostic {
	if d == nil {
		return StructuredDiagnostic{Code: string(reports.CodeInvalidJSON), Message: "invalid report"}
	}
	return StructuredDiagnostic{
		Code:    string(d.Code),
		Field:   d.Field,
		Value:   d.Value,
		Message: d.Rule,
	}
}

func structuredDiagnosticJSON(d *reports.DiagnosticError) string {
	b, err := json.Marshal(structuredFromDiagnostic(d))
	if err != nil {
		return `{"code":"invalid_json","message":"could not encode diagnostic"}`
	}
	return string(b)
}

func mapCLIError(err error) *clierr.HeroError {
	if err == nil {
		return nil
	}
	var rve *ReportValidationError
	if errors.As(err, &rve) && rve != nil && rve.Diagnostic != nil {
		return clierr.New(structuredDiagnosticJSON(rve.Diagnostic))
	}
	return mapCLIErrorBase(err)
}
