package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func transcriptContains(m model, want string) bool {
	for _, msg := range m.transcript {
		if strings.Contains(msg.content, want) {
			return true
		}
	}
	return false
}

func TestQAFailedLoopBackDispatchesImplementation(t *testing.T) {
	dir := t.TempDir()
	writeHeroStartCommand(t, dir)
	writeAgentFile(t, dir, "orchestration_agent", "ORCHESTRATION_AGENT_MARKER")
	writeAgentFile(t, dir, "generic_agent", "GENERIC_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", qaHandoffYAML)
	writeImplementationTasks(t, svc, "- [x] 1.1 [task-01] [agent:generic_agent] Already complete\n")
	if err := svc.CloseStage("implementation", "completed", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true})

	genRel := make(chan struct{})
	closeGen := closeOnce(genRel)
	t.Cleanup(func() { closeGen() })
	h := &streamingHarness{
		deltas:      []string{"ok"},
		sessionIDs:  []string{"orch-sess", "gen-sess"},
		release:     genRel,
		skipRelease: 1,
	}
	svc.Harness = h
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{"cursor": h}}

	report := `qa_agent:
{"status":"failed","summary":"handoff failure","failures":[{"owner":"generic_agent","file":"internal/tui/stage_handoff.go","issue":"missing atomic close","acceptance_criteria":"scheduler owns failed close","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}}]}`
	m := withDefaultChatModel(NewTestModel(svc))
	m.orchestrationLive = true
	m.stageHandoffLive = true
	m.stageHandoffStage = stageQA
	m.stageHandoffOutputs = []string{report}
	m.runtimeAgentName = agentQA

	next, cmd := m.maybeHandoffAfterExecute()
	next, _ = pumpConversationUntil(t, next, cmd, 5*time.Second, func(m model) bool {
		for _, call := range h.Calls() {
			if call.Agent == agentGeneric && strings.Contains(call.Prompt, "find-qa-1") {
				return true
			}
		}
		return false
	})
	found := false
	for _, call := range h.Calls() {
		if call.Agent == agentGeneric && strings.Contains(call.Prompt, "find-qa-1") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("generic_agent was not launched with find-qa-1: %+v", h.Calls())
	}
	closeGen()
	_ = next
}

func TestLoopBackRunningStageStillDispatchesWhenOrchAlreadyStarted(t *testing.T) {
	dir := t.TempDir()
	writeAgentFile(t, dir, "generic_agent", "GENERIC_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", qaHandoffYAML)
	writeImplementationTasks(t, svc, "- [x] 1.1 [task-01] [agent:generic_agent] Already complete\n")
	if err := svc.CloseStage("implementation", "completed", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	report := `qa_agent:
{"status":"failed","summary":"handoff failure","failures":[{"owner":"generic_agent","file":"internal/tui/stage_handoff.go","issue":"missing atomic close","acceptance_criteria":"scheduler owns failed close","repro":{"package":"./internal/tui","test":"TestFindHandoffRepro","source":"package tui\n\nfunc TestFindHandoffRepro(t *testing.T) { t.Fatal(\"repro\") }\n"}}]}`
	m := NewTestModel(svc)
	m.stageHandoffOutputs = []string{report}
	decision := m.evaluateValidationStageHandoff(stageQA, report)
	if !decision.SchedulerHandledFailure {
		t.Fatalf("decision=%+v", decision)
	}
	if err := svc.StartStage("implementation"); err != nil {
		t.Fatal(err)
	}
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true})
	h := &streamingHarness{deltas: []string{"fixing"}, sessionIDs: []string{"gen-sess"}}
	svc.Harness = h
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{"cursor": h}}

	m = withDefaultChatModel(NewTestModel(svc))
	m.orchestrationLive = true
	m.runtimeAgentName = agentOrchestration
	m.stageHandoffInterventionRequired = true
	m.stageHandoffDoneKey = "qa:1"

	next, cmd := m.maybeHandoffAfterExecute()
	if cmd == nil {
		t.Fatal("expected Implementation Execute after orch-started loop-back")
	}
	next = drainConversationStream(t, next, cmd)
	found := false
	for _, call := range h.Calls() {
		if call.Agent == agentGeneric && strings.Contains(call.Prompt, "find-qa-1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("generic_agent missing find-qa-1 assignment: %+v", h.Calls())
	}
}

func TestFailedGateKeepsRunningUntilExplicitHeroStart(t *testing.T) {
	dir := t.TempDir()
	writeAgentFile(t, dir, "generic_agent", "GENERIC_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", qaHandoffYAML)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-01] [agent:generic_agent] Open work\n")
	h := &streamingHarness{deltas: []string{"should-not-run"}}
	svc.Harness = h
	m := NewTestModel(svc)
	m.orchestrationLive = true
	m.runtimeAgentName = agentOrchestration
	m.stageHandoffInterventionRequired = true
	m.stageHandoffDoneKey = m.runningStageHandoffKey()

	next, cmd := m.ensureStageProgress()
	if cmd != nil || len(next.executes) != 0 {
		t.Fatalf("failed gate must not auto-relaunch: cmd=%v executes=%d", cmd, len(next.executes))
	}
	if h.ExecuteCount() != 0 {
		t.Fatalf("harness executes=%d want 0", h.ExecuteCount())
	}
	if !transcriptContains(next, "/hero-start") {
		t.Fatalf("missing /hero-start CTA: %+v", next.transcript)
	}
}

func TestEscalatedStageEmitsContinueCTA(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Engine.EscalateIfExhausted(cycleRow.ID, "implementation"); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.orchestrationLive = true
	m.runtimeAgentName = agentOrchestration
	next, cmd := m.ensureStageProgress()
	if cmd != nil {
		t.Fatal("Escalated stage must not Execute")
	}
	if !transcriptContains(next, "/hero-continue") {
		t.Fatalf("missing /hero-continue CTA: %+v", next.transcript)
	}
}

func TestPendingApprovalEmitsApproveCTA(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	m := NewTestModel(svc)
	m.orchestrationLive = true
	m.runtimeAgentName = agentOrchestration
	next, cmd := m.ensureStageProgress()
	if cmd != nil {
		t.Fatal("PendingApproval must not Execute a stage agent")
	}
	if !transcriptContains(next, "/hero-approve") {
		t.Fatalf("missing /hero-approve CTA: %+v", next.transcript)
	}
}

func TestStartStageBudgetExhaustionEmitsContinueCTA(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", qaHandoffYAML)
	if err := svc.CloseStage("implementation", "completed", "", false); err != nil {
		t.Fatal(err)
	}
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	impl, err := svc.Store.GetStage(cycleRow.ID, "implementation")
	if err != nil {
		t.Fatal(err)
	}
	impl.Status = store.StageWaiting
	impl.Iteration = impl.EffectiveMaxIterations()
	if err := svc.Store.UpdateStage(impl); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.orchestrationLive = true
	m.runtimeAgentName = agentOrchestration
	next, cmd := m.ensureStageProgress()
	if cmd != nil {
		t.Fatal("budget exhaustion must not Execute")
	}
	if !transcriptContains(next, "/hero-continue") {
		t.Fatalf("missing /hero-continue CTA: %+v", next.transcript)
	}
}

func TestLifecycleStageStartedDispatchesWhenIdle(t *testing.T) {
	dir := t.TempDir()
	writeAgentFile(t, dir, "generic_agent", "GENERIC_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", qaHandoffYAML)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-01] [agent:generic_agent] Open work\n")
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true})
	h := &streamingHarness{deltas: []string{"working"}, sessionIDs: []string{"gen-sess"}}
	svc.Harness = h
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{"cursor": h}}

	m := withDefaultChatModel(NewTestModel(svc))
	m.orchestrationLive = true
	m.runtimeAgentName = agentOrchestration

	next, cmd := m.handleLifecycleEvent(conversation.Event{
		EventID:   42,
		Kind:      conversation.EventStageStarted,
		StageName: "implementation",
	})
	if cmd == nil {
		t.Fatal("EventStageStarted while idle must dispatch stage agents")
	}
	next = drainConversationStream(t, next, cmd)
	if h.ExecuteCount() < 1 || h.Calls()[0].Agent != agentGeneric {
		t.Fatalf("calls=%+v want generic_agent", h.Calls())
	}
}

func TestExecuteDoneErrorStillDispatchesRunningStage(t *testing.T) {
	dir := t.TempDir()
	writeAgentFile(t, dir, "generic_agent", "GENERIC_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", qaHandoffYAML)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-01] [agent:generic_agent] Open work\n")
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true})
	h := &streamingHarness{deltas: []string{"retry"}, sessionIDs: []string{"gen-sess"}}
	svc.Harness = h
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{"cursor": h}}

	m := withDefaultChatModel(NewTestModel(svc))
	m.orchestrationLive = true
	m.runtimeAgentName = agentOrchestration
	m.streaming = true
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", AgentName: agentOrchestration},
	}
	next, cmd := m.Update(executeDoneMsg{executeID: "ex-1", err: errors.New("harness down")})
	got := next.(model)
	if cmd == nil {
		t.Fatal("orchestrator execute error must still dispatch the Running stage")
	}
	got = drainConversationStream(t, got, cmd)
	if h.ExecuteCount() < 1 || h.Calls()[0].Agent != agentGeneric {
		t.Fatalf("calls=%+v want generic_agent", h.Calls())
	}
}

func TestHeroContinueExecuteDispatchesRunningQA(t *testing.T) {
	dir := t.TempDir()
	writeAgentFile(t, dir, "qa_agent", "QA_AGENT_MARKER")
	writeAgentFile(t, dir, "orchestration_agent", "ORCH_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", qaHandoffYAML)
	if err := svc.CloseStage("implementation", "completed", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	qa, err := svc.Store.GetStage(cycleRow.ID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	qa.Iteration = qa.EffectiveMaxIterations()
	if err := svc.Store.UpdateStage(qa); err != nil {
		t.Fatal(err)
	}
	if err := svc.Engine.EscalateIfExhausted(cycleRow.ID, "qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Continue(1); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true})
	h := &streamingHarness{deltas: []string{"qa running"}, sessionIDs: []string{"qa-sess"}}
	svc.Harness = h
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{"cursor": h}}

	m := withDefaultChatModel(NewTestModel(svc))
	m.orchestrationLive = false
	m.runtimeCommandName = "continue"
	m.runtimeAgentName = agentOrchestration
	m.streaming = true
	m.stageHandoffLive = true
	m.stageHandoffStage = stageQA
	m.stageHandoffDoneKey = "qa:1"
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", AgentName: agentOrchestration},
	}
	next, cmd := m.Update(executeDoneMsg{executeID: "ex-1"})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("model type %T", next)
	}
	if cmd == nil {
		t.Fatal("continue executeDone must dispatch qa_agent")
	}
	got = drainConversationStream(t, got, cmd)
	if h.ExecuteCount() < 1 || h.Calls()[0].Agent != agentQA {
		t.Fatalf("calls=%+v want qa_agent", h.Calls())
	}
	if !got.orchestrationLive {
		t.Fatal("continue executeDone must re-arm the scheduler")
	}
}

func TestSchedulerCTAIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithPendingApprovalInDir(t, dir)
	m := NewTestModel(svc)
	m.orchestrationLive = true
	first, _ := m.ensureStageProgress()
	second, _ := first.ensureStageProgress()
	count := 0
	for _, msg := range second.transcript {
		if strings.Contains(msg.content, "/hero-approve") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("CTA copies=%d want 1", count)
	}
}
