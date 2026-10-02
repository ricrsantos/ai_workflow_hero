package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

// ADR-107 regression tests: per-claim acceptance, dependency-aware waves,
// no-progress / SDD-ambiguity escalation, and the Planning SDD check.

func newImplementationGateModel(t *testing.T, tasks string, wave int, assigned []string, pendingBefore []string, report string) (model, string) {
	t.Helper()
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	path := writeImplementationTasks(t, svc, tasks)
	blocks := make([]implementationTaskBlock, 0, len(assigned))
	for _, id := range assigned {
		blocks = append(blocks, implementationTaskBlock{ID: id, Owner: agentGeneric})
	}
	m := NewTestModel(svc)
	m.stageHandoffWave = wave
	m.stageHandoffExpectedAgents = []string{agentGeneric}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{wave: {agentGeneric: blocks}}
	m.stageHandoffOutputs = []string{report}
	m.stageHandoffPendingBefore = pendingBefore
	return m, path
}

func implReport(status, completed, remaining string, testsPassed, required bool, extra string) string {
	b := func(v bool) string {
		if v {
			return "true"
		}
		return "false"
	}
	return "generic_agent:\n" + `{"stage":"implementation","agent":"generic_agent","status":"` + status +
		`","tasks_completed":[` + completed + `],"tasks_remaining":[` + remaining +
		`],"tests_passed":` + b(testsPassed) +
		`,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":` + b(required) +
		`},"blocker":"remaining work","next_action":"continue","summary":"test"` + extra + `}`
}

func TestGateAcceptsVerifiedClaimsWhenSuiteFlagIsFalse(t *testing.T) {
	tasks := "- [ ] 1.1 [task-01] done\n- [ ] 1.2 [task-02] open\n"
	report := implReport("partial", `"task-01"`, `"task-02"`, false, true, "")
	m, path := newImplementationGateModel(t, tasks, 1, []string{"task-01", "task-02"},
		[]string{"1.1 [task-01] done", "1.2 [task-02] open"}, report)

	decision := m.evaluateStageHandoff(stageImplementation, report)

	if !decision.PartialProgress {
		t.Fatalf("decision=%+v; tests_passed=false must not discard a verified claim", decision)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "- [x] 1.1 [task-01]") {
		t.Fatalf("task-01 not recorded:\n%s", raw)
	}
}

func TestGateRejectsClaimsWithFalseRequiredGateAndEscalates(t *testing.T) {
	tasks := "- [ ] 1.1 [task-01] done\n- [ ] 1.2 [task-02] open\n"
	report := implReport("partial", `"task-01"`, `"task-02"`, true, false, "")
	m, path := newImplementationGateModel(t, tasks, 1, []string{"task-01", "task-02"},
		[]string{"1.1 [task-01] done", "1.2 [task-02] open"}, report)

	decision := m.evaluateStageHandoff(stageImplementation, report)

	if decision.PartialProgress || decision.EscalateReason != escalateImplementationNoProgress {
		t.Fatalf("decision=%+v want no-progress escalation", decision)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "[x]") {
		t.Fatalf("claim with a false gate must not be recorded:\n%s", raw)
	}
}

func TestGateEscalatesOnSDDAmbiguityAfterRecordingVerifiedWork(t *testing.T) {
	tasks := "- [ ] 1.1 [task-01] done\n- [ ] 1.2 [task-02] open\n"
	report := implReport("partial", `"task-01"`, `"task-02"`, true, true, `,"sdd_ambiguity":true`)
	report = strings.Replace(report, `"blocker":"remaining work"`, `"blocker":"Which config key holds the opt-in? Recommend test_access.enabled."`, 1)
	m, path := newImplementationGateModel(t, tasks, 1, []string{"task-01", "task-02"},
		[]string{"1.1 [task-01] done", "1.2 [task-02] open"}, report)

	decision := m.evaluateStageHandoff(stageImplementation, report)

	if decision.EscalateReason != escalateImplementationSDDAmbiguity || decision.PartialProgress {
		t.Fatalf("decision=%+v want sdd ambiguity escalation", decision)
	}
	if !strings.Contains(decision.Reason, "test_access.enabled") {
		t.Fatalf("reason must carry the questions: %q", decision.Reason)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "- [x] 1.1 [task-01]") {
		t.Fatalf("verified work must be recorded before escalating:\n%s", raw)
	}
	if !strings.Contains(escalationNextSteps(decision.EscalateReason), "/hero-back") {
		t.Fatal("ambiguity escalation must offer /hero-back")
	}
}

func TestPartitionDefersTasksWithUncheckedDependencies(t *testing.T) {
	raw := "- [x] 1.1 [task-01] base\n" +
		"- [ ] 1.2 [task-02] [after:task-01] ready\n" +
		"- [ ] 1.3 [task-03] [after:task-01,task-02] waits\n" +
		"- [ ] 1.4 [task-04] [after:task-03] final\n"
	plan := partitionImplementationTasks(raw, []string{agentGeneric})
	if !plan.Valid {
		t.Fatalf("errors=%v", plan.Errors)
	}
	got := implementationAssignmentIDs(plan.ByAgent[agentGeneric])
	if strings.Join(got, ",") != "task-02" {
		t.Fatalf("assigned=%v want only task-02", got)
	}
	if len(plan.Deferred) != 2 {
		t.Fatalf("deferred=%d want 2", len(plan.Deferred))
	}
}

func TestPartitionRejectsInvalidDependencies(t *testing.T) {
	cases := map[string]string{
		"unknown": "- [ ] 1.1 [task-01] [after:task-99] x\n",
		"self":    "- [ ] 1.1 [task-01] [after:task-01] x\n",
		"cycle":   "- [ ] 1.1 [task-01] [after:task-02] x\n- [ ] 1.2 [task-02] [after:task-01] y\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if plan := partitionImplementationTasks(raw, []string{agentGeneric}); plan.Valid {
				t.Fatalf("plan must be invalid: %+v", plan)
			}
		})
	}
}

func TestLintImplementationSDDFlagsUnsatisfiableContracts(t *testing.T) {
	raw := "- [ ] 1.1 [task-01] [agent:generic_agent] Build it. Verify: `go test ./x/`.\n" +
		"- [ ] 1.2 [task-02] [agent:generic_agent] [after:task-03] No check here.\n" +
		"- [ ] 1.3 [task-03] [agent:generic_agent] [after:task-02] Update `context/current-state.md`. Verify: diff check.\n" +
		"- [ ] 1.4 [task-04] [agent:generic_agent] Pick the key (TBD). Verify: `go test ./y/`.\n"
	problems := strings.Join(lintImplementationSDD(raw, []string{agentGeneric}, map[string]string{"design.md": "Checkpoint interval: to be decided.\n"}), "\n")
	for _, want := range []string{
		`task "task-02" has no executable verification`,
		`task "task-03" edits context/current-state.md`,
		"dependency cycle",
		`tasks.md line 4 leaves a decision open`,
		`design.md line 1 leaves a decision open`,
	} {
		if !strings.Contains(problems, want) {
			t.Errorf("missing %q in:\n%s", want, problems)
		}
	}
	if strings.Contains(problems, `task "task-01"`) {
		t.Errorf("task-01 is valid but was flagged:\n%s", problems)
	}
}

func TestLintImplementationSDDAcceptsCleanPlan(t *testing.T) {
	raw := "## 1. Work\n\n" +
		"- [ ] 1.1 [task-01] [agent:generic_agent] Parser. Verify: `go test ./p/ -run TestParse`.\n" +
		"- [ ] 1.2 [task-02] [agent:generic_agent] [after:task-01] Docs in `docs/TESTING.md`. `context/current-state.md` is not part of this task. Verify: `go test ./s/`.\n" +
		"  Pending ToDos stay in SQLite.\n"
	if problems := lintImplementationSDD(raw, []string{agentGeneric}, nil); len(problems) != 0 {
		t.Fatalf("clean plan flagged: %v", problems)
	}
}

func TestPlanningHandoffRefusesCloseOnSDDProblems(t *testing.T) {
	dir := t.TempDir()
	planningEnabled := strings.Replace(implementationHandoffYAML, "  planning:\n    enabled: false", "  planning:\n    enabled: true", 1)
	svc := newTestServiceWithRunningStage(t, dir, "planning", planningEnabled)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-01] No verification.\n")
	m := NewTestModel(svc)

	decision := m.evaluateStageHandoff(stagePlanning, "planning done")

	if decision.Complete || len(decision.PlanningSDDProblems) == 0 {
		t.Fatalf("decision=%+v; planning must stay open", decision)
	}
	if !strings.Contains(planningSDDFeedbackSection(decision.PlanningSDDProblems), "task-01") {
		t.Fatal("feedback must name the failing task")
	}

	if err := os.WriteFile(filepath.Join(dir, "openspec", "changes", "demo", "tasks.md"),
		[]byte("- [ ] 1.1 [task-01] [agent:backend_agent] Fixed. Verify: `go test ./...`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if decision = m.evaluateStageHandoff(stagePlanning, "planning done"); !decision.Complete {
		t.Fatalf("decision=%+v; corrected SDD must close", decision)
	}
}

func TestHeroBackSourceStage(t *testing.T) {
	cases := []struct {
		stages []cycle.StatusStage
		want   string
	}{
		{[]cycle.StatusStage{{Name: "judge", Status: "PendingApproval"}}, "judge"},
		{[]cycle.StatusStage{{Name: "implementation", Status: "Escalated"}}, "implementation"},
		{[]cycle.StatusStage{{Name: "qa", Status: "Escalated"}}, ""},
		{[]cycle.StatusStage{{Name: "implementation", Status: "Running"}}, ""},
	}
	for _, c := range cases {
		if got := heroBackSourceStage(cycle.StatusView{Stages: c.stages}); got != c.want {
			t.Errorf("stages=%+v got %q want %q", c.stages, got, c.want)
		}
	}
}
