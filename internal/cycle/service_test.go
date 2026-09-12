package cycle_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/lifecycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestOpenServiceUsesInheritedLifecycleNotifier(t *testing.T) {
	dir := setupProject(t)
	t.Setenv(lifecycle.EventSocketEnv, filepath.Join(dir, "lifecycle.sock"))

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if svc.Engine == nil || svc.Engine.Notifier == nil {
		t.Fatal("CLI service did not install the inherited lifecycle notifier")
	}
}

func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: CLI Feature
objective: Test CLI
stages:
  research:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: false
  qa:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
  judge:
    enabled: true
    max_iterations: 1
    timeout_minutes: 10
    require_human_approval: false
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	// Touch hero.json so FindProjectRoot works via .workflow-hero dir.
	_ = os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755)
	ctxDir := filepath.Join(dir, "context")
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "current-state.md"), []byte("## Pending Features\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestServiceCycleLifecycleAndReads(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	res, err := svc.NewCycle("", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Cycle.Number != 1 || len(res.Stages) < 2 {
		t.Fatalf("new cycle: %+v", res)
	}

	st, err := svc.Status()
	if err != nil || len(st.Stages) == 0 {
		t.Fatalf("status: %+v %v", st, err)
	}
	if st.OpenspecChange != "" {
		t.Fatalf("openspec_change should start empty, got %q", st.OpenspecChange)
	}
	if err := svc.SetOpenspecChange("slash-parity-tui-harness"); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Status()
	if err != nil || st.OpenspecChange != "slash-parity-tui-harness" {
		t.Fatalf("status after set: %+v %v", st, err)
	}
	if err := svc.ClearOpenspecChange(); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Status()
	if err != nil || st.OpenspecChange != "" {
		t.Fatalf("status after clear: %+v %v", st, err)
	}

	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "done", `{"agent":"discover_agent","input_tokens":40,"output_tokens":10}`, false); err != nil {
		t.Fatal(err)
	}

	m, err := svc.Metrics()
	if err != nil || m.TotalIn != 40 {
		t.Fatalf("metrics: %+v %v", m, err)
	}
	ev, err := svc.Events("", 20)
	if err != nil || len(ev.Events) == 0 {
		t.Fatalf("events: %+v %v", ev, err)
	}

	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Approve("lgtm", `{"agent":"qa_agent","input_tokens":8}`); err != nil {
		t.Fatal(err)
	}

	if err := svc.Finish(""); err != nil {
		t.Fatal(err)
	}
	completedStatus, err := svc.Status()
	if err != nil || completedStatus.CycleNumber != 1 || completedStatus.Status != store.CycleStatusCompleted || len(completedStatus.Stages) == 0 {
		t.Fatalf("status after finish: %+v %v", completedStatus, err)
	}
	sessionCycle, err := svc.SessionCycle()
	if err != nil || sessionCycle == nil || sessionCycle.Number != 1 || sessionCycle.Status != store.CycleStatusCompleted {
		t.Fatalf("session cycle after finish: %+v %v", sessionCycle, err)
	}
	arch, err := svc.Archive()
	if err != nil {
		t.Fatal(err)
	}
	if arch.CycleNumber != 1 {
		t.Fatalf("archive: %+v", arch)
	}
	if _, err := os.Stat(arch.ArchiveDir); err != nil {
		t.Fatalf("archive dir missing: %v", err)
	}
	sessionCycle, err = svc.SessionCycle()
	if err != nil || sessionCycle != nil {
		t.Fatalf("session cycle after archive: %+v %v", sessionCycle, err)
	}

	// Resume cancelled path — restore config after archive emptied current/.
	svc2, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	cfgPath := filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	_ = os.MkdirAll(filepath.Dir(cfgPath), 0o755)
	if err := os.WriteFile(cfgPath, []byte(`title: Second
objective: obj
stages:
  research:
    enabled: true
    max_iterations: 1
    timeout_minutes: 5
    require_human_approval: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc2.NewCycle("Second", "obj"); err != nil {
		t.Fatal(err)
	}
	if err := svc2.Cancel("stop"); err != nil {
		t.Fatal(err)
	}
	if err := svc2.Resume(2); err != nil {
		t.Fatal(err)
	}
	c, err := svc2.Store.GetActiveCycle()
	if err != nil || c.Number != 2 || c.Status != store.CycleStatusActive {
		t.Fatalf("resume: %+v %v", c, err)
	}
}

func TestPrepareCycleCreatesMissingCurrentConfigFromTemplateAndArchive(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, ".workflow-hero", "templates", "workflow-config.yml")
	archivePath := filepath.Join(dir, ".workflow-hero", "cycles", "archive", "C3-previous", "workflow-config.yml")
	configDir := filepath.Join(dir, ".workflow-hero", "config")
	for _, path := range []string{filepath.Dir(templatePath), filepath.Dir(archivePath), configDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(configDir, "project.json"), []byte(`{"workflow": {"cycle": 0}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templatePath, []byte(`title: Template
objective: New cycle
stages:
  research:
    enabled: true
    max_iterations: 5
    timeout_minutes: 15
    require_human_approval: false
  qa:
    enabled: false
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, []byte(`title: Previous
objective: Previous cycle
stages:
  research:
    enabled: false
  qa:
    enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	res, err := svc.PrepareCycle()
	if err != nil {
		t.Fatal(err)
	}
	if res.Cycle.Number != 1 || len(res.Stages) != 2 {
		t.Fatalf("result=%+v", res)
	}
	currentPath := filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	data, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || !strings.Contains(string(data), "enabled: false") {
		t.Fatalf("expected archived stage settings in current config:\n%s", data)
	}
}

func TestServiceRunRecordsHarnessInvokedFallback(t *testing.T) {
	dir := setupProject(t)
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	// Force CLI-missing path so the test does not depend on a live cursor-agent on PATH.
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	svc.Harness = adapter

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Run("")
	if err != nil {
		t.Fatal(err)
	}
	if res.Dispatched {
		t.Fatalf("expected fallback dispatch, got %+v", res)
	}
	if res.Stage != "research" {
		t.Fatalf("stage=%q want research", res.Stage)
	}
	if res.Message == "" {
		t.Fatal("expected fallback message")
	}

	ev, err := svc.Events(store.EventHarnessInvoked, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Events) != 1 {
		t.Fatalf("events=%d want 1 harness_invoked", len(ev.Events))
	}
	if ev.Events[0].Type != store.EventHarnessInvoked {
		t.Fatalf("event type=%q", ev.Events[0].Type)
	}
}

func TestStageAgentAuditRoundTrip(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}

	assignmentBody := "  {\n    \"tasks\": [\"01\", \"02\"],\n    \"mode\": \"parallel\"\n  }  "
	resultBody := "{\"status\":\"partial\",\"tasks_remaining\":[\"03\"]}"
	if err := svc.RecordStageAgentAssignment(" implementation ", " generic_agent ", 2, assignmentBody); err != nil {
		t.Fatalf("record assignment: %v", err)
	}
	if err := svc.RecordStageAgentResult("implementation", "generic_agent", 2, resultBody); err != nil {
		t.Fatalf("record result: %v", err)
	}

	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := svc.Store.ListConversation(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("conversation entries=%d want 2: %+v", len(entries), entries)
	}

	want := []struct {
		role string
		kind string
		body string
	}{
		{cycle.ConversationRoleSystem, cycle.ConversationKindStageAgentAssignment, assignmentBody},
		{cycle.ConversationRoleAgent, cycle.ConversationKindStageAgentResult, resultBody},
	}
	for i, want := range want {
		got := entries[i]
		if got.Role != want.role || got.Kind != want.kind {
			t.Fatalf("entry[%d]=%+v want role=%q kind=%q", i, got, want.role, want.kind)
		}
		var audit cycle.StageAgentAuditBody
		if err := json.Unmarshal([]byte(got.Body), &audit); err != nil {
			t.Fatalf("entry[%d] body is not audit JSON: %v; body=%q", i, err, got.Body)
		}
		if audit.Stage != "implementation" || audit.Agent != "generic_agent" || audit.Wave != 2 || audit.Body != want.body {
			t.Fatalf("entry[%d] audit=%+v want stage/agent/wave/body preserved", i, audit)
		}
		if len(audit.TaskIDs) != 0 {
			t.Fatalf("entry[%d] legacy audit task IDs=%v want empty", i, audit.TaskIDs)
		}
		if i == 1 {
			if audit.ResultValidated == nil || *audit.ResultValidated {
				t.Fatalf("entry[%d] result_validated=%v want false", i, audit.ResultValidated)
			}
		} else if audit.ResultValidated != nil {
			t.Fatalf("entry[%d] assignment result_validated=%v want unset", i, audit.ResultValidated)
		}
	}
}

func TestStageAgentAuditAssignmentTaskIDsRoundTrip(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}

	wantTaskIDs := []string{"task-02", "task-01", "task-03"}
	if err := svc.RecordStageAgentAssignmentWithTasks(
		" implementation ",
		" generic_agent ",
		3,
		[]string{" task-02 ", "task-01", "\ttask-03\n"},
		"assignment",
	); err != nil {
		t.Fatalf("record assignment with task IDs: %v", err)
	}

	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := svc.Store.ListConversation(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("conversation entries=%d want 1", len(entries))
	}

	var audit cycle.StageAgentAuditBody
	if err := json.Unmarshal([]byte(entries[0].Body), &audit); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(audit.TaskIDs, wantTaskIDs) {
		t.Fatalf("task IDs=%v want %v", audit.TaskIDs, wantTaskIDs)
	}
	if audit.Stage != "implementation" || audit.Agent != "generic_agent" || audit.Wave != 3 || audit.Body != "assignment" {
		t.Fatalf("audit=%+v want normalized assignment envelope", audit)
	}
}

func TestStageAgentAuditValidationAndActiveCycleErrors(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	tests := []struct {
		name string
		call func() error
		want string
	}{
		{
			name: "missing stage",
			call: func() error {
				return svc.RecordStageAgentAssignment(" ", "agent", 1, "body")
			},
			want: "stage is required",
		},
		{
			name: "missing agent",
			call: func() error {
				return svc.RecordStageAgentResult("stage", " ", 1, "body")
			},
			want: "agent is required",
		},
		{
			name: "invalid wave",
			call: func() error {
				return svc.RecordStageAgentAssignment("stage", "agent", 0, "body")
			},
			want: "wave must be at least 1",
		},
		{
			name: "missing assignment body",
			call: func() error {
				return svc.RecordStageAgentAssignment("stage", "agent", 1, " \t")
			},
			want: "body is required",
		},
		{
			name: "empty task ID",
			call: func() error {
				return svc.RecordStageAgentAssignmentWithTasks("stage", "agent", 1, []string{"task-01", " \t"}, "body")
			},
			want: "assignment id at index 1",
		},
		{
			name: "duplicate task ID after normalization",
			call: func() error {
				return svc.RecordStageAgentAssignmentWithTasks("stage", "agent", 1, []string{"task-01", " task-01 "}, "body")
			},
			want: "duplicate task id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want containing %q", err, tt.want)
			}
		})
	}

	if err := svc.RecordStageAgentAssignment("stage", "agent", 1, "body"); err == nil || !strings.Contains(err.Error(), "get active cycle") {
		t.Fatalf("no active cycle error=%v", err)
	}
}

func TestStageAgentAuditPreservesEmptyResult(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordStageAgentResult("implementation", "generic_agent", 1, ""); err != nil {
		t.Fatalf("record empty result: %v", err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := svc.Store.ListConversation(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("conversation entries=%d want 1", len(entries))
	}
	var audit cycle.StageAgentAuditBody
	if err := json.Unmarshal([]byte(entries[0].Body), &audit); err != nil {
		t.Fatal(err)
	}
	if audit.Body != "" {
		t.Fatalf("empty result body=%q want empty", audit.Body)
	}
	if audit.ResultValidated == nil || *audit.ResultValidated {
		t.Fatalf("empty raw result_validated=%v want false", audit.ResultValidated)
	}
}

func TestStageAgentAuditMixedAssignmentTaskIDsRoundTrip(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}

	wantTaskIDs := []string{"task-04.1", "find-qa-1", "find-judge-2"}
	if err := svc.RecordStageAgentAssignmentWithTasks(
		"implementation",
		"generic_agent",
		4,
		[]string{" task-04.1 ", "[find-qa-1]", "find-judge-2"},
		"assignment with mixed IDs",
	); err != nil {
		t.Fatalf("record mixed assignment: %v", err)
	}

	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := svc.Store.ListConversation(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("conversation entries=%d want 1", len(entries))
	}

	var audit cycle.StageAgentAuditBody
	if err := json.Unmarshal([]byte(entries[0].Body), &audit); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(audit.TaskIDs, wantTaskIDs) {
		t.Fatalf("task_ids=%v want %v", audit.TaskIDs, wantTaskIDs)
	}
	if audit.ResultValidated != nil {
		t.Fatalf("assignment result_validated=%v want unset", audit.ResultValidated)
	}
}

func TestStageAgentAuditValidatedResultPreservesRawAndIDs(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}

	raw := "Report follows:\n```json\n{\"tasks_completed\":[\"task-03.2\",\"find-qa-1\"]}\n```"
	if err := svc.RecordStageAgentResultValidated(
		"implementation",
		"generic_agent",
		2,
		raw,
		[]string{" task-03.2 ", "find-qa-1"},
		nil,
	); err != nil {
		t.Fatalf("record validated result: %v", err)
	}

	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := svc.Store.ListConversation(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	var audit cycle.StageAgentAuditBody
	if err := json.Unmarshal([]byte(entries[0].Body), &audit); err != nil {
		t.Fatal(err)
	}
	if audit.Body != raw {
		t.Fatalf("body=%q want raw preserved", audit.Body)
	}
	if audit.ResultValidated == nil || !*audit.ResultValidated {
		t.Fatalf("result_validated=%v want true", audit.ResultValidated)
	}
	if !slices.Equal(audit.TasksCompleted, []string{"task-03.2", "find-qa-1"}) {
		t.Fatalf("tasks_completed=%v", audit.TasksCompleted)
	}
	if len(audit.TasksRemaining) != 0 {
		t.Fatalf("tasks_remaining=%v want empty", audit.TasksRemaining)
	}
}

func TestStageAgentAuditRejectsInvalidAssignmentID(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}

	err = svc.RecordStageAgentAssignmentWithTasks("implementation", "generic_agent", 1, []string{"task-01", "gap-1"}, "body")
	if err == nil || !strings.Contains(err.Error(), "assignment id at index 1") {
		t.Fatalf("error=%v want invalid assignment id at index 1", err)
	}
}

func TestServiceRetryFailedStage(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "failed", "", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.RetryFailedStage("qa"); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	stage, err := svc.Store.GetStage(c.ID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	if stage.Status != store.StageWaiting || stage.Iteration != 0 {
		t.Fatalf("retried stage=%+v", stage)
	}
	events, err := svc.Events(store.EventStageRetried, 5)
	if err != nil || len(events.Events) != 1 {
		t.Fatalf("retry events=%+v err=%v", events, err)
	}
}

func TestFindProjectRoot(t *testing.T) {
	dir := setupProject(t)
	nested := filepath.Join(dir, "pkg", "x")
	_ = os.MkdirAll(nested, 0o755)
	root, err := cycle.FindProjectRoot(nested)
	if err != nil || root != dir {
		t.Fatalf("root=%q err=%v want %q", root, err, dir)
	}
}

func TestServiceLoopBackToImplementation(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `title: Loop Back
objective: Judge gaps
stages:
  implementation:
    enabled: true
    max_iterations: 4
    timeout_minutes: 60
    require_human_approval: false
  qa:
    enabled: true
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: false
  judge:
    enabled: true
    max_iterations: 3
    timeout_minutes: 10
    require_human_approval: false
`
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755)

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("implementation"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("implementation", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ok", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("judge"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("judge", "gaps", "", true); err != nil {
		t.Fatal(err)
	}
	reason := "Judge found gaps"
	if err := svc.LoopBackToImplementation("judge", reason); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	impl, err := svc.Store.GetStage(c.ID, "implementation")
	if err != nil {
		t.Fatal(err)
	}
	if impl.Status != store.StageWaiting || impl.Summary != reason {
		t.Fatalf("implementation=%+v", impl)
	}
	judge, err := svc.Store.GetStage(c.ID, "judge")
	if err != nil {
		t.Fatal(err)
	}
	if judge.Status != store.StageWaiting {
		t.Fatalf("judge=%s want Waiting", judge.Status)
	}
	ev, err := svc.Events(store.EventLoopBack, 5)
	if err != nil || len(ev.Events) != 1 {
		t.Fatalf("loop-back events=%+v err=%v", ev, err)
	}
}
