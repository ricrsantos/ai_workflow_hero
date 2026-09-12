package reports

import (
	"strings"
	"testing"
)

func allImplOwners() ActiveOwners {
	return ActiveOwners{
		OwnerBackend:  {},
		OwnerFrontend: {},
		OwnerGeneric:  {},
	}
}

func testCtx() DecodeContext {
	return DecodeContext{
		ActiveOwners:               allImplOwners(),
		ActiveImplementationAgents: []string{OwnerGeneric},
	}
}

func TestDecodeQA_PassedEmptyFailures(t *testing.T) {
	raw := `{"status":"passed","failures":[],"summary":"ok"}`
	report, err := DecodeQA([]byte(raw), testCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Status != ValidationStatusPassed || len(report.Failures) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestDecodeQA_MissingAcceptanceCriteria(t *testing.T) {
	raw := `{
		"status":"failed",
		"summary":"bad",
		"failures":[{"owner":"generic_agent","file":"a.go","issue":"x","acceptance_criteria":""}]
	}`
	_, err := DecodeQA([]byte(raw), testCtx())
	if err == nil || err.Code != CodeMissingField {
		t.Fatalf("expected missing_field, got %v", err)
	}
	if !strings.Contains(err.Field, "acceptance_criteria") {
		t.Fatalf("field path: %s", err.Field)
	}
}

func TestDecodeQA_UnknownField(t *testing.T) {
	raw := `{"status":"passed","failures":[],"summary":"ok","extra":true}`
	_, err := DecodeQA([]byte(raw), testCtx())
	if err == nil || err.Code != CodeUnknownField {
		t.Fatalf("expected unknown_field, got %v", err)
	}
}

func TestDecodeQA_InvalidJSON(t *testing.T) {
	_, err := DecodeQA([]byte(`{`), testCtx())
	if err == nil || err.Code != CodeInvalidJSON {
		t.Fatalf("expected invalid_json, got %v", err)
	}
}

func TestDecodeQA_InvalidOwner(t *testing.T) {
	raw := `{
		"status":"failed",
		"summary":"bad",
		"failures":[{"owner":"qa_agent","file":"a.go","issue":"x","acceptance_criteria":"y"}]
	}`
	_, err := DecodeQA([]byte(raw), testCtx())
	if err == nil || err.Code != CodeInvalidOwner {
		t.Fatalf("expected invalid_owner, got %v", err)
	}
}

func TestDecodeQA_NoActionableFinding(t *testing.T) {
	raw := `{"status":"failed","failures":[],"summary":"bad"}`
	_, err := DecodeQA([]byte(raw), testCtx())
	if err == nil || err.Code != CodeNoActionableFinding {
		t.Fatalf("expected no_actionable_finding, got %v", err)
	}
}

func TestDecodeQA_UnknownReopenID(t *testing.T) {
	ctx := testCtx()
	ctx.ReopenIDs = ReopenIDValidateFunc(func(sourceStage, owner, reopenID string) *DiagnosticError {
		return diag(CodeUnknownReopenID, "failures[0].reopen_id", reopenID, "finding is not a done match in this cycle")
	})
	raw := `{
		"status":"failed",
		"summary":"bad",
		"failures":[{"owner":"generic_agent","file":"a.go","issue":"x","acceptance_criteria":"y","reopen_id":"find-qa-1"}]
	}`
	_, err := DecodeQA([]byte(raw), ctx)
	if err == nil || err.Code != CodeUnknownReopenID {
		t.Fatalf("expected unknown_reopen_id, got %v", err)
	}
}

func TestDecodeJudge_SDDAmbiguityIsolated(t *testing.T) {
	raw := `{"status":"failed","implementation_gaps":[],"sdd_ambiguity":true,"summary":"ambiguous"}`
	report, err := DecodeJudge([]byte(raw), testCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.SDDAmbiguity || len(report.ImplementationGaps) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestDecodeJudge_DefaultOwnerRequiresSingleImplAgent(t *testing.T) {
	ctx := testCtx()
	ctx.ActiveImplementationAgents = []string{OwnerBackend, OwnerFrontend}
	raw := `{
		"status":"failed",
		"summary":"gap",
		"sdd_ambiguity":false,
		"implementation_gaps":[{"requirement":"R1","issue":"i","acceptance_criteria":"a"}]
	}`
	_, err := DecodeJudge([]byte(raw), ctx)
	if err == nil || err.Code != CodeMissingField {
		t.Fatalf("expected missing_field for owner, got %v", err)
	}
}

func TestDecodeJudge_DefaultOwnerSingleAgent(t *testing.T) {
	ctx := testCtx()
	ctx.ActiveImplementationAgents = []string{OwnerGeneric}
	raw := `{
		"status":"failed",
		"summary":"gap",
		"sdd_ambiguity":false,
		"implementation_gaps":[{"requirement":"R1","issue":"i","acceptance_criteria":"a"}]
	}`
	report, err := DecodeJudge([]byte(raw), ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ImplementationGaps[0].Owner != OwnerGeneric {
		t.Fatalf("owner: %s", report.ImplementationGaps[0].Owner)
	}
}

func TestDecodeBrowserUI_FailureClassOwner(t *testing.T) {
	raw := `{
		"status":"failed",
		"summary":"ui fail",
		"failures":[{"failure_class":"frontend","file":"x.tsx","issue":"i","acceptance_criteria":"a"}]
	}`
	report, err := DecodeBrowserUI([]byte(raw), testCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Failures[0].Owner != OwnerFrontend {
		t.Fatalf("owner: %s", report.Failures[0].Owner)
	}
}

func TestDecodeImplementation_Table(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		assign []string
		code   Code
	}{
		{
			name:   "union mismatch",
			raw:    implReport(`["find-qa-1"]`, `[]`, true),
			assign: []string{"find-qa-1", "find-qa-2"},
			code:   CodeAssignmentUnionMismatch,
		},
		{
			name:   "unassigned id",
			raw:    implReport(`["task-03.2"]`, `[]`, true),
			assign: []string{"find-judge-1"},
			code:   CodeUnassignedID,
		},
		{
			name:   "nonempty empty assignment",
			raw:    implReport(`["task-03.2"]`, `[]`, true),
			assign: []string{},
			code:   CodeNonemptyEmptyAssignment,
		},
		{
			name:   "overlapping arrays",
			raw:    implReport(`["task-03.2"]`, `["task-03.2"]`, true),
			assign: []string{"task-03.2"},
			code:   CodeOverlappingArrays,
		},
		{
			name:   "false acceptance gate",
			raw:    implReport(`["task-03.2"]`, `[]`, false),
			assign: []string{"task-03.2"},
			code:   CodeFalseAcceptanceGate,
		},
		{
			name:   "duplicate id",
			raw:    implReport(`["task-03.2","task-03.2"]`, `[]`, true),
			assign: []string{"task-03.2"},
			code:   CodeDuplicateID,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeImplementation([]byte(tc.raw), OwnerGeneric, tc.assign, testCtx())
			if err == nil || err.Code != tc.code {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
		})
	}
}

func implReport(completed, remaining string, gatesTrue bool) string {
	gates := "false"
	if gatesTrue {
		gates = "true"
	}
	return `{
		"stage":"implementation",
		"agent":"generic_agent",
		"status":"complete",
		"tasks_completed":` + completed + `,
		"tasks_remaining":` + remaining + `,
		"tests_passed":true,
		"acceptance_gates":{
			"completed_tasks_verified":` + gates + `,
			"task_ownership_respected":` + gates + `,
			"required_tests_passed":` + gates + `
		},
		"summary":"done"
	}`
}

func TestDecodeImplementation_MixedAssignmentSuccess(t *testing.T) {
	raw := implReport(`["task-03.2","find-qa-1"]`, `[]`, true)
	report, err := DecodeImplementation([]byte(raw), OwnerGeneric, []string{"task-03.2", "find-qa-1"}, testCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.TasksCompleted) != 2 {
		t.Fatalf("completed: %v", report.TasksCompleted)
	}
}

func TestDiagnosticError_ErrorString(t *testing.T) {
	err := diag(CodeMissingField, "failures[0].acceptance_criteria", "", "field is required")
	if !strings.Contains(err.Error(), "missing_field") {
		t.Fatalf("error: %s", err.Error())
	}
}
