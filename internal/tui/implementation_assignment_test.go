package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestPartitionImplementationTasks(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		active         []string
		wantValid      bool
		wantBackend    []string
		wantFrontend   []string
		wantUnassigned []string
		wantInvalid    []string
	}{
		{
			name: "backend and frontend owners partition without overlap",
			raw: "- [ ] 1.1 [task-back] [agent:backend_agent] Add API\n" +
				"  Acceptance: API returns the documented response.\n" +
				"  Verify: go test ./internal/api\n" +
				"- [ ] 1.2 [task-front] [agent:frontend_agent] Add screen\n" +
				"  Acceptance: screen renders the response.\n",
			active:       []string{implementationBackendAgent, implementationFrontendAgent},
			wantValid:    true,
			wantBackend:  []string{"task-back"},
			wantFrontend: []string{"task-front"},
		},
		{
			name: "single agent keeps legacy ownerless tasks",
			raw: "- [ ] 1.1 [task-legacy] Keep the old task format\n" +
				"  Acceptance: old task remains executable.\n",
			active:       []string{implementationGenericAgent},
			wantValid:    true,
			wantBackend:  nil,
			wantFrontend: nil,
		},
		{
			name:           "multi agent ownerless task is unassigned",
			raw:            "- [ ] 1.1 [task-unowned] Needs an explicit owner\n",
			active:         []string{implementationBackendAgent, implementationFrontendAgent},
			wantValid:      false,
			wantUnassigned: []string{"task-unowned"},
		},
		{
			name:           "unknown owner is unassigned",
			raw:            "- [ ] 1.1 [task-unknown] [agent:qa_agent] Wrong owner\n",
			active:         []string{implementationBackendAgent, implementationFrontendAgent},
			wantValid:      false,
			wantUnassigned: []string{"task-unknown"},
		},
		{
			name:           "inactive owner is unassigned",
			raw:            "- [ ] 1.1 [task-inactive] [agent:frontend_agent] Frontend is not active\n",
			active:         []string{implementationBackendAgent},
			wantValid:      false,
			wantUnassigned: []string{"task-inactive"},
		},
		{
			name: "duplicate and missing IDs invalidate the plan",
			raw: "- [ ] 1.1 [task-duplicate] [agent:backend_agent] First copy\n" +
				"- [ ] 1.2 [task-duplicate] [agent:backend_agent] Second copy\n" +
				"- [ ] 1.3 [agent:backend_agent] Missing ID\n",
			active:      []string{implementationBackendAgent},
			wantValid:   false,
			wantInvalid: []string{"task-duplicate", "task-duplicate", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := partitionImplementationTasks(tt.raw, tt.active)
			if plan.Valid != tt.wantValid {
				t.Fatalf("valid=%v want %v; errors=%v", plan.Valid, tt.wantValid, plan.Errors)
			}
			assertImplementationTaskIDs(t, plan.ByAgent[implementationBackendAgent], tt.wantBackend)
			assertImplementationTaskIDs(t, plan.ByAgent[implementationFrontendAgent], tt.wantFrontend)
			if tt.name == "single agent keeps legacy ownerless tasks" {
				generic := plan.ByAgent[implementationGenericAgent]
				assertImplementationTaskIDs(t, generic, []string{"task-legacy"})
				if generic[0].Owner != implementationGenericAgent || plan.Tasks[0].Owner != implementationGenericAgent {
					t.Fatalf("implicit owner not retained: assigned=%q parsed=%q", generic[0].Owner, plan.Tasks[0].Owner)
				}
			}
			assertImplementationTaskIDs(t, plan.Unassigned, tt.wantUnassigned)
			assertImplementationTaskIDs(t, plan.Invalid, tt.wantInvalid)
		})
	}
}

func TestPartitionImplementationTasksPreservesMultilineCriteria(t *testing.T) {
	raw := "- [ ] 2.1 [task-multiline] [agent:backend_agent] Implement the boundary\n" +
		"  Acceptance:\n" +
		"    - rejects malformed input\n" +
		"    - keeps the caller-visible error\n" +
		"  Verify: go test ./internal/tui\n" +
		"- [ ] 2.2 [task-next] [agent:backend_agent] Follow-up\n"
	plan := partitionImplementationTasks(raw, []string{implementationBackendAgent})
	if !plan.Valid || len(plan.ByAgent[implementationBackendAgent]) != 2 {
		t.Fatalf("plan=%+v", plan)
	}
	first := plan.ByAgent[implementationBackendAgent][0]
	if first.Line != 1 || first.EndLine != 5 {
		t.Fatalf("line=%d end=%d want 1..5", first.Line, first.EndLine)
	}
	for _, want := range []string{"Acceptance:", "rejects malformed input", "keeps the caller-visible error", "go test ./internal/tui"} {
		if !strings.Contains(first.Block, want) {
			t.Fatalf("block missing %q: %q", want, first.Block)
		}
	}
	if strings.Contains(first.Block, "task-next") {
		t.Fatalf("first block consumed next task: %q", first.Block)
	}
}

func TestPartitionImplementationTasksIgnoresFencedCodeAndSupportsListMarkers(t *testing.T) {
	raw := "```markdown\n" +
		"- [ ] [task-fake] [agent:backend_agent] This is code, not a task\n" +
		"```\n" +
		"+ [ ] [task-plus] [agent:backend_agent] Plus marker\n" +
		"1. [ ] [task-dot] [agent:backend_agent] Ordered marker\n" +
		"2) [ ] [task-paren] [agent:backend_agent] Ordered marker\n"
	plan := partitionImplementationTasks(raw, []string{implementationBackendAgent})
	if !plan.Valid {
		t.Fatalf("plan should be valid: %+v", plan)
	}
	assertImplementationTaskIDs(t, plan.ByAgent[implementationBackendAgent], []string{"task-plus", "task-dot", "task-paren"})
	for _, task := range plan.Tasks {
		if task.ID == "task-fake" {
			t.Fatalf("fenced code task was parsed: %+v", task)
		}
	}
}

func TestPartitionImplementationTasksRejectsUnsupportedChecklistSyntax(t *testing.T) {
	for _, raw := range []string{
		"- [y] [task-invalid] [agent:backend_agent] Unsupported checkbox\n",
		"> - [ ] [task-quoted] [agent:backend_agent] Blockquote task\n",
	} {
		plan := partitionImplementationTasks(raw, []string{implementationBackendAgent})
		if plan.Valid {
			t.Fatalf("unsupported syntax was accepted: raw=%q plan=%+v", raw, plan)
		}
		if len(plan.Errors) == 0 || !strings.Contains(strings.Join(plan.Errors, "\n"), "unsupported task checkbox syntax") {
			t.Fatalf("missing diagnostic for unsupported syntax: raw=%q errors=%v", raw, plan.Errors)
		}
	}
}

func TestPartitionImplementationTasksRejectsDuplicatePendingAndCompleted(t *testing.T) {
	raw := "- [x] [task-shared] [agent:backend_agent] Already complete\n" +
		"- [ ] [task-shared] [agent:backend_agent] Pending duplicate\n"
	plan := partitionImplementationTasks(raw, []string{implementationBackendAgent})
	if plan.Valid {
		t.Fatalf("duplicate pending/completed ID was accepted: %+v", plan)
	}
	assertImplementationTaskIDs(t, plan.Invalid, []string{"task-shared"})
	if !strings.Contains(strings.Join(plan.Errors, "\n"), "task ID \"task-shared\" is duplicated") {
		t.Fatalf("missing duplicate diagnostic: %v", plan.Errors)
	}
}

func TestMarkImplementationTasksCompleteUpdatesBackendAndFrontendTogether(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	raw := "# Tasks\n\n" +
		"- [ ] 1.1 [task-back] [agent:backend_agent] API\n" +
		"  Acceptance: preserve this criterion.\n" +
		"- [ ] 1.2 [task-front] [agent:frontend_agent] UI\n" +
		"  Verify: preserve this verification.\n" +
		"- [x] 1.3 [task-old] [agent:generic_agent] Already done\n"
	if err := os.WriteFile(path, []byte(raw), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := markImplementationTasksComplete(path, []string{"task-back", "[task-front]"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Tasks\n\n" +
		"- [x] 1.1 [task-back] [agent:backend_agent] API\n" +
		"  Acceptance: preserve this criterion.\n" +
		"- [x] 1.2 [task-front] [agent:frontend_agent] UI\n" +
		"  Verify: preserve this verification.\n" +
		"- [x] 1.3 [task-old] [agent:generic_agent] Already done\n"
	if string(got) != want {
		t.Fatalf("content changed unexpectedly:\n got %q\nwant %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o640 {
		t.Fatalf("mode=%#o want %#o", gotMode, 0o640)
	}
}

func TestMarkImplementationTasksCompleteValidationDoesNotMutate(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		ids  []string
	}{
		{
			name: "missing requested ID",
			raw:  "- [ ] 1.1 [task-existing] [agent:backend_agent] Existing\n",
			ids:  []string{"task-missing"},
		},
		{
			name: "duplicate task ID in file",
			raw: "- [ ] 1.1 [task-duplicate] [agent:backend_agent] First\n" +
				"- [ ] 1.2 [task-duplicate] [agent:frontend_agent] Second\n",
			ids: []string{"task-duplicate"},
		},
		{
			name: "duplicate request ID",
			raw:  "- [ ] 1.1 [task-existing] [agent:backend_agent] Existing\n",
			ids:  []string{"task-existing", "[task-existing]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "tasks.md")
			if err := os.WriteFile(path, []byte(tt.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			beforeInfo, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := markImplementationTasksComplete(path, tt.ids); err == nil {
				t.Fatal("expected validation error")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatalf("validation mutated content: before=%q after=%q", before, after)
			}
			afterInfo, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if afterInfo.Mode().Perm() != beforeInfo.Mode().Perm() {
				t.Fatalf("validation changed mode: before=%#o after=%#o", beforeInfo.Mode().Perm(), afterInfo.Mode().Perm())
			}
		})
	}
}

func TestMarkImplementationTasksCompletePreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	raw := []byte("# Tasks\r\n\r\n- [ ] [task-crlf] [agent:backend_agent] Preserve line endings\r\n")
	if err := os.WriteFile(path, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := markImplementationTasksComplete(path, []string{"task-crlf"}); err != nil {
		t.Fatal(err)
	}
	want := []byte("# Tasks\r\n\r\n- [x] [task-crlf] [agent:backend_agent] Preserve line endings\r\n")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("CRLF content changed: got %q want %q", got, want)
	}
}

func TestMarkImplementationTasksCompleteRejectsAlreadyComplete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	raw := []byte("- [x] [task-done] [agent:backend_agent] Already complete\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := markImplementationTasksComplete(path, []string{"task-done"}); err == nil || !strings.Contains(err.Error(), "already complete") {
		t.Fatalf("expected already-complete error, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("already-complete validation mutated content: got %q want %q", got, raw)
	}
}

func TestMarkImplementationTasksCompleteRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	link := filepath.Join(dir, "tasks.md")
	raw := []byte("- [ ] [task-link] [agent:backend_agent] Target\n")
	if err := os.WriteFile(target, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := markImplementationTasksComplete(link, []string{"task-link"}); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("rejected symlink was replaced: mode=%v", linkInfo.Mode())
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("symlink target changed: got %q want %q", got, raw)
	}
}

func TestWriteImplementationTasksAtomicallyRejectsConcurrentChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	initial := []byte("- [ ] [task-race] [agent:backend_agent] Initial\n")
	concurrent := []byte("- [ ] [task-race] [agent:backend_agent] Concurrent edit\n")
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, concurrent, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeImplementationTasksAtomically(path, []byte("replacement\n"), info.Mode(), initial); err == nil || !strings.Contains(err.Error(), "changed concurrently") {
		t.Fatalf("expected concurrent-change rejection, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(concurrent) {
		t.Fatalf("concurrent edit was overwritten: got %q want %q", got, concurrent)
	}
}

func assertImplementationTaskIDs(t *testing.T, tasks []implementationTaskBlock, want []string) {
	t.Helper()
	if len(tasks) != len(want) {
		t.Fatalf("task IDs=%v want %v", implementationTaskIDs(tasks), want)
	}
	for i, task := range tasks {
		if task.ID != want[i] {
			t.Fatalf("task IDs=%v want %v", implementationTaskIDs(tasks), want)
		}
	}
}

func implementationTaskIDs(tasks []implementationTaskBlock) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func TestImplementationFindingBlockFreezesContractAndHistory(t *testing.T) {
	finding := store.Finding{
		ID:                 "find-qa-19",
		Owner:              store.FindingOwnerGeneric,
		SourceStage:        store.FindingSourceQA,
		File:               "internal/tui/conversation.go",
		Requirement:        "PRD-C16-001 FR-08",
		Issue:              "Cancellation does not join the worker",
		AcceptanceCriteria: "cancel and join in-flight execution",
	}
	occs := []store.FindingOccurrence{
		{Round: 1, Kind: store.OccurrenceCreated, Issue: "Quit races persist"},
		{Round: 2, Kind: store.OccurrenceReopened, Issue: "workers use context.Background"},
	}
	block := implementationFindingBlock(finding, occs)
	if !strings.Contains(block.Block, "Contract: frozen") {
		t.Fatalf("missing frozen contract:\n%s", block.Block)
	}
	if !strings.Contains(block.Block, "r1 created: Quit races persist") {
		t.Fatalf("missing history:\n%s", block.Block)
	}
	if !strings.Contains(block.Block, "cancel and join in-flight execution") {
		t.Fatalf("missing frozen acceptance:\n%s", block.Block)
	}
}

func TestMergeImplementationAssignmentOrdersTasksBeforeFindings(t *testing.T) {
	raw := "- [ ] 1.1 [task-a] [agent:generic_agent] First task\n" +
		"- [ ] 1.2 [task-b] [agent:generic_agent] Second task\n"
	plan := partitionImplementationTasks(raw, []string{implementationGenericAgent})
	if !plan.Valid {
		t.Fatalf("plan=%+v", plan)
	}
	findings := []store.Finding{
		{ID: "find-qa-1", Owner: store.FindingOwnerGeneric, SourceStage: store.FindingSourceQA},
		{ID: "find-qa-2", Owner: store.FindingOwnerGeneric, SourceStage: store.FindingSourceQA},
	}
	byAgent, errs := mergeImplementationAssignment(plan, findings, []string{implementationGenericAgent}, nil)
	if len(errs) != 0 {
		t.Fatalf("merge errors=%v", errs)
	}
	got := implementationTaskIDs(byAgent[implementationGenericAgent])
	want := []string{"task-a", "task-b", "find-qa-1", "find-qa-2"}
	if len(got) != len(want) {
		t.Fatalf("ids=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids=%v want %v", got, want)
		}
	}
}

func TestMergeImplementationAssignmentRejectsFindingOwnerOutsideScope(t *testing.T) {
	plan := partitionImplementationTasks("", []string{implementationBackendAgent})
	plan.Valid = true
	findings := []store.Finding{{ID: "find-qa-1", Owner: store.FindingOwnerFrontend, SourceStage: store.FindingSourceQA}}
	_, errs := mergeImplementationAssignment(plan, findings, []string{implementationBackendAgent}, nil)
	if len(errs) == 0 || !strings.Contains(strings.Join(errs, "; "), "not in active implementation scope") {
		t.Fatalf("expected scope failure, got %v", errs)
	}
}

func TestBuildImplementationStageDispatchVerificationWaveWhenEmpty(t *testing.T) {
	checklist := implementationChecklist{Linked: true, Ready: true, Raw: "- [x] [task-done] [agent:generic_agent] Done\n"}
	runAgents, assignments, expected, reason := buildImplementationStageDispatch(checklist, []string{implementationGenericAgent}, nil, nil)
	if reason != "" {
		t.Fatalf("reason=%q", reason)
	}
	if len(runAgents) != 1 || len(expected) != 1 || len(assignments[implementationGenericAgent]) != 0 {
		t.Fatalf("agents=%v expected=%v assignments=%v", runAgents, expected, assignments)
	}
}

func TestValidateImplementationAssignmentUnionFindingsOnlyRejectsTaskID(t *testing.T) {
	err := validateImplementationAssignmentUnion([]string{"task-03.2"}, nil, []string{"find-judge-1"})
	if err == nil || !strings.Contains(err.Error(), "unassigned_id") {
		t.Fatalf("error=%v want unassigned_id", err)
	}
}

func TestValidateImplementationAssignmentUnionEmptyVerificationWave(t *testing.T) {
	err := validateImplementationAssignmentUnion([]string{"task-03.2"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "nonempty_empty_assignment") {
		t.Fatalf("error=%v want nonempty_empty_assignment", err)
	}
	if err := validateImplementationAssignmentUnion(nil, nil, nil); err != nil {
		t.Fatalf("empty verification wave should accept empty arrays: %v", err)
	}
}

func TestValidateImplementationAssignmentUnionExactMatch(t *testing.T) {
	assign := []string{"task-a", "find-qa-1"}
	if err := validateImplementationAssignmentUnion([]string{"task-a"}, []string{"find-qa-1"}, assign); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validateImplementationAssignmentUnion([]string{"task-a"}, nil, assign); err == nil || !strings.Contains(err.Error(), "assignment_union_mismatch") {
		t.Fatalf("error=%v want assignment_union_mismatch", err)
	}
}
