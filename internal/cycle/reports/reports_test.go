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

func TestReportJSONFromText_IgnoresTrailingFenceAndProse(t *testing.T) {
	raw := "notes first\n```json\n{\"status\":\"passed\",\"failures\":[],\"summary\":\"ok\"}\n```\ntrailing"
	got, err := ReportJSONFromText(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report, derr := DecodeQA(got, testCtx())
	if derr != nil {
		t.Fatalf("decode: %v", derr)
	}
	if report.Status != ValidationStatusPassed {
		t.Fatalf("status=%q", report.Status)
	}
}

func TestReportJSONFromText_RepairsMissingFailureCloseBeforeArrayEnd(t *testing.T) {
	// QA agents often omit the failure-object close when repro.source contains
	// nested `{` / `}`. encoding/json then reports: invalid character ']' after
	// object key:value pair.
	raw := "→ running checks\n" +
		`{"status":"failed","failures":[{"owner":"generic_agent","file":"a.go","issue":"x","acceptance_criteria":"y","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) {\n\tt.Fatal(\"x\")\n}\n"}],"summary":"six remain"}`
	got, err := ReportJSONFromText(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report, derr := DecodeQA(got, testCtx())
	if derr != nil {
		t.Fatalf("decode: %v", derr)
	}
	if report.Status != ValidationStatusFailed || len(report.Failures) != 1 {
		t.Fatalf("report=%+v", report)
	}
	if report.Failures[0].Repro.Test != "TestFindHandoffRepro" {
		t.Fatalf("repro=%+v", report.Failures[0].Repro)
	}
	if !strings.Contains(report.Failures[0].Repro.Source, "func TestFindHandoffRepro(") {
		t.Fatalf("source=%q", report.Failures[0].Repro.Source)
	}
}

func TestDecodeQA_GoRecursiveEvidenceAllowed(t *testing.T) {
	raw := `{
		"status":"failed",
		"summary":"toolchain",
		"failures":[{
			"owner":"generic_agent",
			"file":"go.mod",
			"issue":"x",
			"acceptance_criteria":"y",
			"evidence":["go test ./internal/tui","go test ./src/api/..."],
			"repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}
		}]
	}`
	report, err := DecodeQA([]byte(raw), testCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Failures) != 1 || len(report.Failures[0].Evidence) != 2 {
		t.Fatalf("report=%+v", report)
	}
	if report.Failures[0].Evidence[1] != "go test ./src/api/..." {
		t.Fatalf("evidence=%v", report.Failures[0].Evidence)
	}
}

func TestDecodeQA_EvidenceParentSegmentRejected(t *testing.T) {
	raw := `{
		"status":"failed",
		"summary":"bad evidence",
		"failures":[{
			"owner":"generic_agent",
			"file":"a.go",
			"issue":"x",
			"acceptance_criteria":"y",
			"evidence":["go test ./internal/tui","../secret.txt"],
			"repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}
		}]
	}`
	_, err := DecodeQA([]byte(raw), testCtx())
	if err == nil || err.Code != CodeInvalidEnum {
		t.Fatalf("expected invalid_enum, got %v", err)
	}
	if !strings.Contains(err.Field, "evidence[1]") {
		t.Fatalf("field=%s", err.Field)
	}
}

func TestReportJSONFromText_InvalidStillRejected(t *testing.T) {
	_, err := ReportJSONFromText("no report here {not json}")
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
	ctx.ReopenIDs = ReopenIDValidateFunc(func(req ReopenRequest) *DiagnosticError {
		return diag(CodeUnknownReopenID, "failures[0].reopen_id", req.ReopenID, "finding is not a done match in this cycle")
	})
	raw := `{
		"status":"failed",
		"summary":"bad",
		"failures":[{"owner":"generic_agent","file":"a.go","issue":"x","acceptance_criteria":"y","reopen_id":"find-qa-1","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}}]
	}`
	_, err := DecodeQA([]byte(raw), ctx)
	if err == nil || err.Code != CodeUnknownReopenID {
		t.Fatalf("expected unknown_reopen_id, got %v", err)
	}
}

func TestDecodeQA_ReopenValidatorReceivesContract(t *testing.T) {
	ctx := testCtx()
	var got ReopenRequest
	ctx.ReopenIDs = ReopenIDValidateFunc(func(req ReopenRequest) *DiagnosticError {
		got = req
		return nil
	})
	raw := `{
		"status":"failed",
		"summary":"regression",
		"failures":[{"owner":"generic_agent","file":"a.go","requirement":"PRD","issue":"x","acceptance_criteria":"same ac","reopen_id":"find-qa-1","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}}]
	}`
	report, err := DecodeQA([]byte(raw), ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Status != ValidationStatusFailed || len(report.Failures) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if got.ReopenID != "find-qa-1" || got.File != "a.go" || got.Requirement != "PRD" || got.AcceptanceCriteria != "same ac" || got.Owner != "generic_agent" || got.SourceStage != SourceQA {
		t.Fatalf("validator request = %+v", got)
	}
	if got.ReproPackage != "./internal/tui" || got.ReproTest != "TestFindHandoffRepro" {
		t.Fatalf("validator repro = %+v", got)
	}
}

func TestDecodeQA_MissingRepro(t *testing.T) {
	raw := `{
		"status":"failed",
		"summary":"bad",
		"failures":[{"owner":"generic_agent","file":"a.go","issue":"x","acceptance_criteria":"y"}]
	}`
	_, err := DecodeQA([]byte(raw), testCtx())
	if err == nil || err.Code != CodeMissingField {
		t.Fatalf("expected missing_field, got %v", err)
	}
	if !strings.Contains(err.Field, "repro") {
		t.Fatalf("field path: %s", err.Field)
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
		"implementation_gaps":[{"requirement":"R1","issue":"i","acceptance_criteria":"a","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}}]
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
		"implementation_gaps":[{"requirement":"R1","issue":"i","acceptance_criteria":"a","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}}]
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
		"failures":[{"failure_class":"frontend","file":"x.tsx","issue":"i","acceptance_criteria":"a","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}}]
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

func TestDecodeImplementation_UnknownMetricsField(t *testing.T) {
	raw := `{
		"stage":"implementation",
		"agent":"generic_agent",
		"status":"complete",
		"tasks_completed":[],
		"tasks_remaining":[],
		"tests_passed":true,
		"acceptance_gates":{
			"completed_tasks_verified":true,
			"task_ownership_respected":true,
			"required_tests_passed":true
		},
		"summary":"done",
		"metrics":{"model":"x","input_chars":1,"output_chars":1}
	}`
	_, err := DecodeImplementation([]byte(raw), OwnerGeneric, nil, testCtx())
	if err == nil || err.Code != CodeUnknownField || err.Field != "metrics" {
		t.Fatalf("expected unknown_field metrics, got %v", err)
	}
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
