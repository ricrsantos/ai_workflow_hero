package reports

// ReopenIDValidator checks reopen_id against cycle finding state without importing store.
type ReopenIDValidator interface {
	ValidateReopenID(sourceStage, owner, reopenID string) *DiagnosticError
}

// ReopenIDValidateFunc is a function adapter for ReopenIDValidator.
type ReopenIDValidateFunc func(sourceStage, owner, reopenID string) *DiagnosticError

func (f ReopenIDValidateFunc) ValidateReopenID(sourceStage, owner, reopenID string) *DiagnosticError {
	if f == nil {
		return nil
	}
	return f(sourceStage, owner, reopenID)
}

// ActionableFindingChecker validates failed-close payloads have actionable findings.
// Store-backed callers can exclude deferred-only recurrences here.
type ActionableFindingChecker interface {
	HasActionableFindings(sourceStage string, entries []FailureEntry) *DiagnosticError
}

// ActionableFindingCheckFunc is a function adapter for ActionableFindingChecker.
type ActionableFindingCheckFunc func(sourceStage string, entries []FailureEntry) *DiagnosticError

func (f ActionableFindingCheckFunc) HasActionableFindings(sourceStage string, entries []FailureEntry) *DiagnosticError {
	if f == nil {
		return countActionableFindings(sourceStage, entries)
	}
	return f(sourceStage, entries)
}

// DecodeContext carries scope and optional store-backed validators.
type DecodeContext struct {
	ActiveOwners               ActiveOwners
	ActiveImplementationAgents []string
	ReopenIDs                  ReopenIDValidator
	Actionable                 ActionableFindingChecker
}

const (
	SourceQA             = "qa"
	SourceJudge          = "judge"
	SourceBrowserUI      = "browser_ui_validation"
	SourceQAEndToEnd     = "qa_end_to_end"
	SourceImplementation = "implementation"
)

func countActionableFindings(sourceStage string, entries []FailureEntry) *DiagnosticError {
	if len(entries) == 0 {
		return diag(CodeNoActionableFinding, "failures", "", "failed close requires at least one actionable finding entry")
	}
	return nil
}
