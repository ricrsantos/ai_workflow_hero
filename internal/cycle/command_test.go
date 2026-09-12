package cycle_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
)

func TestStructuredDiagnosticJSON(t *testing.T) {
	d := &reports.DiagnosticError{
		Code:  reports.CodeInvalidJSON,
		Field: "failures",
		Value: "",
		Rule:  "must be valid JSON",
	}
	raw := cycle.StructuredDiagnosticJSONForTest(d)
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal: %v raw=%q", err, raw)
	}
	if parsed["code"] != "invalid_json" || parsed["field"] != "failures" {
		t.Fatalf("parsed=%v", parsed)
	}
}

func TestStageCloseFindingsJSONRequiresFailed(t *testing.T) {
	dir := setupProject(t)
	t.Chdir(dir)
	cmd := cycle.StageCloseCommandForTest()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--name", "qa", "--findings-json", `{}`})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), "findings-json requires --failed") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestCompleteTodoRequiresNote(t *testing.T) {
	dir := setupProject(t)
	t.Chdir(dir)
	cmd := cycle.CompleteTodoCommandForTest()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"find-qa-1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "note is required") {
		t.Fatalf("err=%v", err)
	}
}

func TestAddTodoRequiresFindingIDs(t *testing.T) {
	dir := setupProject(t)
	t.Chdir(dir)
	cmd := cycle.AddTodoCommandForTest()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}
