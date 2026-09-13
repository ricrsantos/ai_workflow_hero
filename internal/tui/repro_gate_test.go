package tui

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reprotest"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestClaimedFindingIDsReadsCompletedFindings(t *testing.T) {
	chunks := []string{
		"generic_agent:\n" + `{"stage":"implementation","status":"complete","tasks_completed":["task-1","find-qa-1"],"tasks_remaining":[]}`,
		"backend_agent:\n" + `{"stage":"implementation","status":"partial","tasks_completed":["find-qa-2"],"tasks_remaining":["find-qa-3"]}`,
		"frontend_agent:\nnot json at all",
	}
	got := claimedFindingIDs(chunks)
	want := []string{"find-qa-1", "find-qa-2"}
	if len(got) != len(want) {
		t.Fatalf("ids=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids=%v want %v", got, want)
		}
	}
}

func TestReproGateFailureRequiresVerdictForEveryClaim(t *testing.T) {
	m := model{}
	if reason := m.reproGateFailure([]string{"find-qa-1"}); !strings.Contains(reason, "has not verified") {
		t.Fatalf("reason=%q", reason)
	}

	m.stageHandoffReproChecked = true
	m.stageHandoffReproResults = map[string]*cycle.ReproGateError{"find-qa-1": nil}
	if reason := m.reproGateFailure([]string{"find-qa-1"}); reason != "" {
		t.Fatalf("passing gate must not block: %q", reason)
	}
	if reason := m.reproGateFailure([]string{"find-qa-1", "find-qa-9"}); !strings.Contains(reason, "find-qa-9") {
		t.Fatalf("an unverified claim must block: %q", reason)
	}

	m.stageHandoffReproResults["find-qa-1"] = &cycle.ReproGateError{
		FindingID: "find-qa-1", Package: "./internal/tui", Test: "TestX", Detail: "still fails",
	}
	if reason := m.reproGateFailure([]string{"find-qa-1"}); !strings.Contains(reason, "repro_test_failed") {
		t.Fatalf("reason=%q", reason)
	}

	m.stageHandoffReproError = "store unavailable"
	if reason := m.reproGateFailure([]string{"find-qa-1"}); !strings.Contains(reason, "could not run") {
		t.Fatalf("reason=%q", reason)
	}
}

// The gate must run off the Update loop: re-running a repro can take minutes.
func TestStageHandoffRunsReproGateAsCommand(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeImplementationTasks(t, svc, "- [x] 1.1 [task-done] [agent:generic_agent] Done task\n")
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	src := "package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"
	if _, err := svc.Store.PersistFinding(store.FindingInput{
		CycleID: cycleRow.ID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
		File: "internal/tui/stage_handoff.go", Issue: "fix handoff",
		AcceptanceCriteria: "finding is verified in implementation report",
		ReproPackage:       "./internal/tui", ReproTest: "TestFindHandoffRepro", ReproSource: src,
	}); err != nil {
		t.Fatal(err)
	}
	restore := cycle.SetFindingReproRunForTest(func(context.Context, string, reprotest.Spec) error {
		return errors.New("test still fails")
	})
	t.Cleanup(restore)

	m := NewTestModel(svc)
	m.stageHandoffLive = true
	m.stageHandoffStage = stageImplementation
	m.stageHandoffWave = 1
	m.stageHandoffExpectedAgents = []string{agentGeneric}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{1: {
		agentGeneric: {{ID: "find-qa-1", Owner: agentGeneric}},
	}}
	m.stageHandoffOutputs = []string{"generic_agent:\n" +
		`{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":["find-qa-1"],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`}

	next, cmd := m.resumeOrchestratorAfterStageHandoff()
	if cmd == nil {
		t.Fatal("the gate must be dispatched as a command, not run inside Update")
	}
	if !next.stageHandoffReproRunning || next.stageHandoffReproChecked {
		t.Fatalf("gate state running=%v checked=%v", next.stageHandoffReproRunning, next.stageHandoffReproChecked)
	}
	if !next.stageHandoffLive {
		t.Fatal("the handoff must stay live while the gate runs")
	}

	msg := cmd()
	gateMsg, ok := msg.(reproGateResultMsg)
	if !ok {
		t.Fatalf("msg=%T want reproGateResultMsg", msg)
	}
	if gateMsg.results["find-qa-1"] == nil {
		t.Fatalf("gate results=%+v want a failure for find-qa-1", gateMsg.results)
	}

	done, _ := next.handleReproGateResult(gateMsg)
	f, err := svc.Store.GetFinding(cycleRow.ID, "find-qa-1")
	if err != nil {
		t.Fatal(err)
	}
	if f.Status == store.FindingStatusDone {
		t.Fatal("a failed repro must not mark the finding done")
	}
	if !done.stageHandoffInterventionRequired {
		t.Fatal("a failed gate must require intervention")
	}
}

func TestStaleReproGateResultIsDiscarded(t *testing.T) {
	m := model{stageHandoffLive: true, stageHandoffStage: stageImplementation, stageHandoffWave: 2}
	next, cmd := m.handleReproGateResult(reproGateResultMsg{stage: stageImplementation, wave: 1})
	if cmd != nil {
		t.Fatal("a stale gate result must not resume the handoff")
	}
	if next.stageHandoffReproChecked {
		t.Fatal("a stale gate result must not mark the current wave verified")
	}
}

func TestFindingsOverRoundCap(t *testing.T) {
	findings := []store.Finding{
		{ID: "find-qa-1", Round: maxFindingRounds},
		{ID: "find-qa-2", Round: maxFindingRounds + 1},
	}
	over := findingsOverRoundCap(findings)
	if len(over) != 1 || over[0].ID != "find-qa-2" {
		t.Fatalf("over=%+v", over)
	}
}

// A finding that keeps coming back must stop the loop instead of consuming
// waves until the iteration budget runs out.
func TestImplementationEscalatesOnFindingRoundCap(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeImplementationTasks(t, svc, "- [x] 1.1 [task-done] [agent:generic_agent] Done task\n")
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	in := store.FindingInput{
		CycleID: cycleRow.ID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerBackend,
		File: "internal/tui/stage_handoff.go", Issue: "keeps failing", AcceptanceCriteria: "gate holds",
	}
	res, err := svc.Store.PersistFinding(in)
	if err != nil {
		t.Fatal(err)
	}
	// Each done → rediscovered pair is one validation round.
	for round := 0; round < maxFindingRounds; round++ {
		if err := svc.Store.InTx(func(tx *sql.Tx) error {
			return svc.Store.MarkFindingDoneTx(tx, cycleRow.ID, res.Finding.ID, in.Issue, in.AcceptanceCriteria, "[]")
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Store.PersistFinding(in); err != nil {
			t.Fatal(err)
		}
	}
	f, err := svc.Store.GetFinding(cycleRow.ID, res.Finding.ID)
	if err != nil {
		t.Fatal(err)
	}
	if f.Round <= maxFindingRounds {
		t.Fatalf("round=%d want > %d", f.Round, maxFindingRounds)
	}

	m := NewTestModel(svc)
	next, _ := m.startStageAgentSessions([]string{agentBackend, agentFrontend})
	if next.stageHandoffLive {
		t.Fatal("no wave may start for a finding past the round cap")
	}
	stage, err := svc.Store.GetStage(cycleRow.ID, "implementation")
	if err != nil {
		t.Fatal(err)
	}
	if stage.Status != store.StageEscalated {
		t.Fatalf("stage=%s want Escalated", stage.Status)
	}
	var cta string
	for _, msg := range next.transcript {
		cta += msg.content + "\n"
	}
	if !strings.Contains(cta, "/hero-continue") || !strings.Contains(cta, "round") {
		t.Fatalf("the user must be offered the escalation actions and told why:\n%s", cta)
	}
}
