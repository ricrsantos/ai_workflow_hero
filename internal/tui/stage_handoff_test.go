package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func closeOnce(ch chan struct{}) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			select {
			case <-ch:
			default:
				close(ch)
			}
		})
	}
}

func writeAgentFile(t *testing.T, dir, name, marker string) {
	t.Helper()
	agentDir := filepath.Join(dir, ".cursor", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\n---\n\n" + marker
	if err := os.WriteFile(filepath.Join(agentDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeHeroStartCommand(t *testing.T, dir string) {
	t.Helper()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# /hero-start\n\nHERO_START_RUNTIME_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestServiceWithRunningStage(t *testing.T, dir, stage, yamlBody string) *cycle.Service {
	t.Helper()
	heroDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(heroDir, "workflow-config.yml"), []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage(stage); err != nil {
		t.Fatal(err)
	}
	return svc
}

const planningHandoffYAML = `title: TUI Planning Handoff
objective: test
agents:
  orchestration_agent:
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
  planning_agent:
    harness: codex
    model: gpt-5.4
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
stages:
  research:
    enabled: false
    max_iterations: 1
    require_human_approval: false
  planning:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`

func TestHeroStartHandsOffToPlanningAgentPair(t *testing.T) {
	dir := t.TempDir()
	writeHeroStartCommand(t, dir)
	writeAgentFile(t, dir, "orchestration_agent", "ORCHESTRATION_AGENT_MARKER")
	writeAgentFile(t, dir, "planning_agent", "PLANNING_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "planning", planningHandoffYAML)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": true})

	orchH := &streamingHarness{deltas: []string{"started planning"}, sessionIDs: []string{"orch-sess", "orch-resume"}}
	planH := &streamingHarness{deltas: []string{"writing SDD"}, sessionIDs: []string{"plan-sess"}}
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor": orchH,
		"codex":  planH,
	}}

	m := withDefaultChatModel(NewTestModel(svc))
	m = SetWidth(m, 100)
	m = SetHeight(m, 32)
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next = drainConversationStream(t, next, cmd)

	calls := planH.Calls()
	if len(calls) != 1 {
		t.Fatalf("planning executes=%d want 1 (%+v)", len(calls), calls)
	}
	if calls[0].Agent != "planning_agent" {
		t.Fatalf("planning agent=%q want planning_agent", calls[0].Agent)
	}
	if calls[0].Model != "gpt-5.4" {
		t.Fatalf("planning model=%q want gpt-5.4", calls[0].Model)
	}
	if !strings.Contains(calls[0].Prompt, "PLANNING_AGENT_MARKER") {
		t.Fatalf("planning prompt missing agent body: %q", calls[0].Prompt)
	}
	if !strings.Contains(calls[0].Prompt, "planning_agent") {
		t.Fatalf("planning prompt missing TUI preamble: %q", calls[0].Prompt)
	}
	orchCalls := orchH.Calls()
	if len(orchCalls) < 1 || orchCalls[0].Agent != "orchestration_agent" {
		t.Fatalf("orchestrator first call=%+v", orchCalls)
	}
	if len(orchCalls) != 2 {
		t.Fatalf("orchestrator executes=%d want 2 (start + resume after planning)", len(orchCalls))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "[PLAN - gpt-5.4 · codex]") {
		t.Fatalf("expected planning speaker header: %q", view)
	}
}

const implementationHandoffYAML = `title: TUI Implementation Handoff
objective: test
scope:
  backend: true
  frontend: true
  native: false
  script: false
  infrastructure: false
agents:
  orchestration_agent:
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
  backend_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
  frontend_agent:
    harness: codex
    model: gpt-5.4
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
stages:
  research:
    enabled: false
    max_iterations: 1
    require_human_approval: false
  planning:
    enabled: false
    max_iterations: 1
    require_human_approval: false
  implementation:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`

func writeImplementationTasks(t *testing.T, svc *cycle.Service, raw string) string {
	t.Helper()
	if err := svc.SetOpenspecChange("demo"); err != nil {
		t.Fatal(err)
	}
	changeDir := filepath.Join(svc.ProjectDir, "openspec", "changes", "demo")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(changeDir, "tasks.md")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImplementationScopeStartsParallelExecutes(t *testing.T) {
	dir := t.TempDir()
	writeHeroStartCommand(t, dir)
	writeAgentFile(t, dir, "orchestration_agent", "ORCHESTRATION_AGENT_MARKER")
	writeAgentFile(t, dir, "backend_agent", "BACKEND_AGENT_MARKER")
	writeAgentFile(t, dir, "frontend_agent", "FRONTEND_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-back] [agent:backend_agent] Backend work\n- [ ] 1.2 [task-front] [agent:frontend_agent] Frontend work\n")
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": true})

	backRel := make(chan struct{})
	frntRel := make(chan struct{})
	closeBack := closeOnce(backRel)
	closeFrnt := closeOnce(frntRel)
	t.Cleanup(func() { closeBack(); closeFrnt() })
	cursorH := &streamingHarness{
		deltas:      []string{"ok"},
		sessionIDs:  []string{"orch-sess", "back-sess"},
		release:     backRel,
		skipRelease: 1,
	}
	frntH := &streamingHarness{
		deltas:     []string{"frnt work"},
		sessionIDs: []string{"frnt-sess"},
		release:    frntRel,
	}
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor": cursorH,
		"codex":  frntH,
	}}

	m := withDefaultChatModel(NewTestModel(svc))
	m = SetWidth(m, 100)
	m = SetHeight(m, 36)
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next, cmd = pumpConversationUntil(t, next, cmd, 5*time.Second, func(m model) bool {
		if cursorH.ExecuteCount() < 2 || frntH.ExecuteCount() < 1 {
			return false
		}
		var hasBack, hasFrnt bool
		for _, a := range LiveAgentsForTest(m) {
			if a.Label == "BACK" {
				hasBack = true
			}
			if a.Label == "FRNT" {
				hasFrnt = true
			}
		}
		view := ViewForTest(m)
		return hasBack && hasFrnt && IsConversationStreaming(m) &&
			strings.Contains(view, "[BACK - composer-2.5 · cursor]") &&
			strings.Contains(view, "[FRNT - gpt-5.4 · codex]")
	})

	if len(LiveAgentsForTest(next)) < 2 {
		t.Fatalf("navbar count=%d want >= 2: %+v", len(LiveAgentsForTest(next)), LiveAgentsForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "[BACK - composer-2.5 · cursor]") {
		t.Fatalf("missing BACK header: %q", view)
	}
	if !strings.Contains(view, "[FRNT - gpt-5.4 · codex]") {
		t.Fatalf("missing FRNT header: %q", view)
	}
	if !strings.Contains(view, "Waiting for harness") {
		t.Fatalf("spinner should remain while children are live: %q", view)
	}

	closeBack()
	next, cmd = pumpConversationUntil(t, next, cmd, 5*time.Second, func(m model) bool {
		if !IsConversationStreaming(m) {
			return false
		}
		var hasBack, hasFrnt bool
		for _, a := range LiveAgentsForTest(m) {
			if a.Label == "BACK" {
				hasBack = true
			}
			if a.Label == "FRNT" {
				hasFrnt = true
			}
		}
		return !hasBack && hasFrnt
	})
	if !IsConversationStreaming(next) {
		t.Fatal("first child done must not end the sibling stream")
	}

	closeFrnt()
	next = drainConversationStream(t, next, cmd)
	backCalls := cursorH.Calls()
	if len(backCalls) < 2 || backCalls[1].Agent != "backend_agent" {
		t.Fatalf("cursor calls=%+v want orch then backend_agent", backCalls)
	}
	frntCalls := frntH.Calls()
	if len(frntCalls) != 1 || frntCalls[0].Agent != "frontend_agent" {
		t.Fatalf("frontend calls=%+v", frntCalls)
	}
}

func TestImplementationCancelCancelsAllExecutes(t *testing.T) {
	dir := t.TempDir()
	writeHeroStartCommand(t, dir)
	writeAgentFile(t, dir, "orchestration_agent", "ORCHESTRATION_AGENT_MARKER")
	writeAgentFile(t, dir, "backend_agent", "BACKEND_AGENT_MARKER")
	writeAgentFile(t, dir, "frontend_agent", "FRONTEND_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-back] [agent:backend_agent] Backend work\n- [ ] 1.2 [task-front] [agent:frontend_agent] Frontend work\n")
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": true})

	backRel := make(chan struct{})
	frntRel := make(chan struct{})
	closeBack := closeOnce(backRel)
	closeFrnt := closeOnce(frntRel)
	t.Cleanup(func() { closeBack(); closeFrnt() })
	cursorH := &streamingHarness{
		deltas:      []string{"ok"},
		sessionIDs:  []string{"orch-sess", "back-sess"},
		release:     backRel,
		skipRelease: 1,
	}
	frntH := &streamingHarness{
		deltas:     []string{"frnt work"},
		sessionIDs: []string{"frnt-sess"},
		release:    frntRel,
	}
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor": cursorH,
		"codex":  frntH,
	}}

	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next, _ = pumpConversationUntil(t, next, cmd, 5*time.Second, func(m model) bool {
		return cursorH.ExecuteCount() >= 2 && frntH.ExecuteCount() >= 1 && IsConversationStreaming(m)
	})

	next, cancelCmd := CancelConversationStreamForTest(next)
	if cancelCmd != nil {
		msg := cancelCmd()
		next2, _ := next.Update(msg)
		next = next2.(model)
	}
	if IsConversationStreaming(next) {
		t.Fatal("expected streaming stopped after cancel-all")
	}
	if !cursorH.CancelCalled() || !frntH.CancelCalled() {
		t.Fatal("Ctrl+C must cancel every in-flight Execute")
	}
}

func TestApproveHandsOffToWaitingPlanning(t *testing.T) {
	dir := t.TempDir()
	writeAgentFile(t, dir, "orchestration_agent", "ORCHESTRATION_AGENT_MARKER")
	writeAgentFile(t, dir, "planning_agent", "PLANNING_AGENT_MARKER")
	yamlBody := `title: Approve Handoff
objective: test
agents:
  orchestration_agent:
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
  planning_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
stages:
  research:
    enabled: true
    max_iterations: 1
    require_human_approval: true
  planning:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`
	heroDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(heroDir, "workflow-config.yml"), []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Approve("", ""); err != nil {
		t.Fatal(err)
	}
	st, err := svc.ActiveStage()
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "planning" || st.Status != "Waiting" {
		t.Fatalf("after approve stage=%s status=%s want planning Waiting", st.Name, st.Status)
	}

	h := &streamingHarness{deltas: []string{"ok"}, sessionIDs: []string{"orch-sess", "plan-sess"}}
	svc.Harness = h
	m := withDefaultChatModel(NewTestModel(svc))
	m = EnterConversationForTest(m)
	m = SetOrchestrationLiveForTest(m, true)
	m.runtimeAgentName = agentOrchestration
	m.researchLive = true
	m = SetConversationInput(m, "continue")
	next, cmd := SubmitConversationForTest(m)
	next = drainConversationStream(t, next, cmd)

	calls := h.Calls()
	if len(calls) < 2 {
		t.Fatalf("executes=%d want orch follow-up then planning: %+v", len(calls), calls)
	}
	var sawPlan bool
	for _, c := range calls {
		if c.Agent == "planning_agent" {
			sawPlan = true
			if !strings.Contains(c.Prompt, "PLANNING_AGENT_MARKER") {
				t.Fatalf("planning prompt missing agent body: %q", c.Prompt)
			}
		}
	}
	if !sawPlan {
		t.Fatalf("expected planning_agent Execute after approve left planning Waiting: %+v", calls)
	}
	st, err = svc.ActiveStage()
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "planning" || st.Status != "Running" {
		t.Fatalf("stage=%s status=%s want planning Running", st.Name, st.Status)
	}
	if ResearchLiveForTest(next) {
		t.Fatal("researchLive should clear once research is no longer interactive")
	}
}

func TestImplementationAgentsFromScope(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	m := NewTestModel(svc)
	got := m.implementationAgentsFromScope()
	if len(got) != 2 || got[0] != agentBackend || got[1] != agentFrontend {
		t.Fatalf("agents=%v want backend+frontend", got)
	}
}

func TestImplementationAssignmentPromptsArePartitionedByOwner(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	raw := "- [ ] 1.1 [task-back] [agent:backend_agent] Backend API\n" +
		"  Acceptance: backend criterion\n" +
		"- [ ] 1.2 [task-front] [agent:frontend_agent] Frontend screen\n" +
		"  Verify: frontend criterion\n"
	path := writeImplementationTasks(t, svc, raw)
	checklist := NewTestModel(svc).implementationChecklist()
	runAgents, assignments, expected, reason := implementationStageDispatch(checklist, []string{agentBackend, agentFrontend}, nil)
	if reason != "" || !reflect.DeepEqual(runAgents, []string{agentBackend, agentFrontend}) || !reflect.DeepEqual(expected, runAgents) {
		t.Fatalf("dispatch agents=%v expected=%v reason=%q", runAgents, expected, reason)
	}
	if path == "" || len(assignments[agentBackend]) != 1 || len(assignments[agentFrontend]) != 1 {
		t.Fatalf("assignments=%+v path=%q", assignments, path)
	}
	backendPrompt := formatImplementationAssignment(checklist, assignments[agentBackend], 1)
	frontendPrompt := formatImplementationAssignment(checklist, assignments[agentFrontend], 1)
	for _, prompt := range []string{backendPrompt, frontendPrompt} {
		if !strings.Contains(prompt, "ownership_validated:true") || !strings.Contains(prompt, "scheduler marks completed task-* IDs") {
			t.Fatalf("prompt missing ownership/scheduler contract: %q", prompt)
		}
	}
	if !strings.Contains(backendPrompt, "task-back") || strings.Contains(backendPrompt, "task-front") || !strings.Contains(backendPrompt, "backend criterion") {
		t.Fatalf("backend prompt not partitioned: %q", backendPrompt)
	}
	if !strings.Contains(frontendPrompt, "task-front") || strings.Contains(frontendPrompt, "task-back") || !strings.Contains(frontendPrompt, "frontend criterion") {
		t.Fatalf("frontend prompt not partitioned: %q", frontendPrompt)
	}
}

func TestImplementationOwnerlessMixedPlanFailsBeforeDispatch(t *testing.T) {
	checklist := implementationChecklist{
		Linked: true,
		Ready:  true,
		Raw:    "- [ ] 1.1 [task-unowned] Legacy task\n",
	}
	runAgents, assignments, expected, reason := implementationStageDispatch(checklist, []string{agentBackend, agentFrontend}, nil)
	if reason == "" || !strings.Contains(reason, "ownership") {
		t.Fatalf("reason=%q want ownership failure", reason)
	}
	if runAgents != nil || assignments != nil || expected != nil {
		t.Fatalf("invalid plan dispatched agents=%v assignments=%v expected=%v", runAgents, assignments, expected)
	}
}

func TestPendingOpenSpecTasksUsesImplementationTaskParser(t *testing.T) {
	raw := "```markdown\n" +
		"- [ ] [task-fenced] [agent:backend_agent] Must be ignored\n" +
		"```\n" +
		"+ [ ] [task-plus] [agent:backend_agent] Plus syntax\n" +
		"1. [ ] [task-ordered] [agent:frontend_agent] Ordered syntax\n" +
		"  - [ ] [task-nested] [agent:generic_agent] Acceptance detail\n" +
		"- [x] [task-done] [agent:backend_agent] Already complete\n"
	got := pendingOpenSpecTasks(raw)
	if len(got) != 2 {
		t.Fatalf("pending=%v want two top-level pending tasks", got)
	}
	if !strings.Contains(got[0], "task-plus") || !strings.Contains(got[1], "task-ordered") {
		t.Fatalf("pending=%v missing plus/ordered tasks", got)
	}
	if strings.Contains(strings.Join(got, "\n"), "fenced") || strings.Contains(strings.Join(got, "\n"), "nested") {
		t.Fatalf("pending=%v included fenced/nested checklist", got)
	}
}

func TestStartStageAgentSessionsAbortsWhenActiveStageCannotBeResolved(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	if err := svc.Close(); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	next, cmd := m.startStageAgentSessions([]string{agentBackend})
	if cmd != nil {
		t.Fatal("active-stage lookup failure must not schedule an Execute")
	}
	if len(next.executes) != 0 {
		t.Fatalf("active-stage lookup failure launched executes: %+v", next.executes)
	}
	if !next.stageHandoffInterventionRequired || !strings.Contains(next.convError, "resolve active stage") {
		t.Fatalf("model=%+v want fail-closed active-stage diagnostic", next)
	}
}

func TestImplementationReportsMarkUnionAndRedispatchOnlyRemainingOwner(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	raw := "- [ ] 1.1 [task-back] [agent:backend_agent] Backend API\n" +
		"- [ ] 1.2 [task-front] [agent:frontend_agent] Frontend screen\n"
	writeImplementationTasks(t, svc, raw)
	m := NewTestModel(svc)
	checklist := m.implementationChecklist()
	plan := partitionImplementationTasks(checklist.Raw, []string{agentBackend, agentFrontend})
	if !plan.Valid {
		t.Fatalf("plan=%+v", plan)
	}
	m.stageHandoffWave = 1
	m.stageHandoffExpectedAgents = []string{agentBackend, agentFrontend}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{1: cloneImplementationAssignments(plan.ByAgent)}
	m.stageHandoffPendingBefore = append([]string(nil), checklist.Pending...)
	backendReport := `backend_agent:
{"stage":"implementation","agent":"backend_agent","status":"complete","tasks_completed":["task-back"],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`
	frontendReport := `frontend_agent:
{"stage":"implementation","agent":"frontend_agent","status":"partial","tasks_completed":[],"tasks_remaining":["task-front"],"tests_passed":false,"acceptance_gates":{"completed_tasks_verified":false,"task_ownership_respected":true,"required_tests_passed":false},"blocker":"frontend remains","next_action":"continue frontend","summary":"test"}`
	m.stageHandoffOutputs = []string{backendReport, frontendReport}
	decision := m.evaluateStageHandoff(stageImplementation, "")
	if decision.Complete || !decision.PartialProgress {
		t.Fatalf("decision=%+v want productive partial progress", decision)
	}
	updated, err := os.ReadFile(filepath.Join(dir, "openspec", "changes", "demo", "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "- [x] 1.1 [task-back]") || !strings.Contains(string(updated), "- [ ] 1.2 [task-front]") {
		t.Fatalf("unexpected checklist after union mark: %q", updated)
	}
	nextAgents, nextAssignments, _, reason := implementationStageDispatch(m.implementationChecklist(), []string{agentBackend, agentFrontend}, nil)
	if reason != "" || !reflect.DeepEqual(nextAgents, []string{agentFrontend}) || len(nextAssignments[agentBackend]) != 0 || len(nextAssignments[agentFrontend]) != 1 {
		t.Fatalf("next dispatch agents=%v assignments=%v reason=%q", nextAgents, nextAssignments, reason)
	}
}

func TestImplementationReportsAreOrderIndependent(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-back] [agent:backend_agent] Backend\n- [ ] 1.2 [task-front] [agent:frontend_agent] Frontend\n")
	m := NewTestModel(svc)
	checklist := m.implementationChecklist()
	plan := partitionImplementationTasks(checklist.Raw, []string{agentBackend, agentFrontend})
	m.stageHandoffWave = 1
	m.stageHandoffExpectedAgents = []string{agentBackend, agentFrontend}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{1: cloneImplementationAssignments(plan.ByAgent)}
	m.stageHandoffPendingBefore = append([]string(nil), checklist.Pending...)
	complete := func(agent, task string) string {
		return fmt.Sprintf("%s:\n{\"stage\":\"implementation\",\"agent\":%q,\"status\":\"complete\",\"tasks_completed\":[%q],\"tasks_remaining\":[],\"tests_passed\":true,\"acceptance_gates\":{\"completed_tasks_verified\":true,\"task_ownership_respected\":true,\"required_tests_passed\":true},\"summary\":\"test\"}", agent, agent, task)
	}
	m.stageHandoffOutputs = []string{complete(agentFrontend, "task-front"), complete(agentBackend, "task-back")}
	decision := m.evaluateStageHandoff(stageImplementation, "")
	if !decision.Complete || decision.PartialProgress {
		t.Fatalf("decision=%+v want complete despite inverted report order", decision)
	}
}

func TestImplementationPreparationFailureDoesNotStartEarlierAgent(t *testing.T) {
	dir := t.TempDir()
	writeAgentFile(t, dir, agentOrchestration, "ORCH")
	writeAgentFile(t, dir, agentBackend, "BACKEND")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-back] [agent:backend_agent] Backend\n- [ ] 1.2 [task-front] [agent:frontend_agent] Frontend\n")
	h := &streamingHarness{deltas: []string{"ok"}, sessionIDs: []string{"orch"}}
	svc.Harness = h
	m := withDefaultChatModel(NewTestModel(svc))
	next, _ := m.startStageAgentSessions([]string{agentBackend, agentFrontend})
	if !next.stageHandoffInterventionRequired {
		t.Fatalf("expected preparation intervention: %+v", next)
	}
	for _, execute := range next.executes {
		if execute.AgentName == agentBackend || execute.AgentName == agentFrontend {
			t.Fatalf("stage agent execute survived preparation failure: %+v", execute)
		}
	}
}

func TestImplementationSymlinkChecklistFailsClosed(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	path := writeImplementationTasks(t, svc, "- [ ] 1.1 [task-back] [agent:backend_agent] Backend\n")
	target := path + ".target"
	if err := os.Rename(path, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	checklist := m.implementationChecklist()
	plan := partitionImplementationTasks(checklist.Raw, []string{agentBackend})
	m.stageHandoffWave = 1
	m.stageHandoffExpectedAgents = []string{agentBackend}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{1: cloneImplementationAssignments(plan.ByAgent)}
	m.stageHandoffPendingBefore = append([]string(nil), checklist.Pending...)
	m.stageHandoffOutputs = []string{`backend_agent:
{"stage":"implementation","agent":"backend_agent","status":"complete","tasks_completed":["task-back"],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`}
	decision := m.evaluateStageHandoff(stageImplementation, "")
	if decision.Complete || !strings.Contains(decision.Reason, "could not be read") {
		t.Fatalf("decision=%+v want symlink/read failure", decision)
	}
}

func TestAgentPromptRelMapsHarnessDirectories(t *testing.T) {
	tests := []struct {
		name    string
		harness string
		want    string
	}{
		{name: "cursor", harness: "cursor", want: filepath.Join(".cursor", "agents", "backend_agent.md")},
		{name: "codex", harness: "codex", want: filepath.Join(".codex", "agents", "backend_agent.md")},
		{name: "opencode", harness: "opencode", want: filepath.Join(".opencode", "agents", "backend_agent.md")},
		{name: "claude", harness: "claude", want: filepath.Join(".claude", "agents", "backend_agent.md")},
		{name: "unknown falls back to cursor", harness: "unknown", want: filepath.Join(".cursor", "agents", "backend_agent.md")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentPromptRel(tt.harness, "backend_agent"); got != tt.want {
				t.Fatalf("agentPromptRel(%q)=%q want %q", tt.harness, got, tt.want)
			}
		})
	}
}

func TestReadStageAgentPromptUsesClaudeDirectory(t *testing.T) {
	dir := t.TempDir()
	claudeYAML := strings.Replace(planningHandoffYAML, "harness: codex", "harness: claude", 1)
	svc := newTestServiceWithRunningStage(t, dir, "planning", claudeYAML)
	claudeDir := filepath.Join(dir, ".claude", "agents")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "planning_agent.md"), []byte("---\nname: planning_agent\n---\n\nCLAUDE_PLANNING_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}

	body, err := NewTestModel(svc).readStageAgentPrompt("planning_agent")
	if err != nil {
		t.Fatalf("readStageAgentPrompt() error=%v", err)
	}
	if strings.TrimSpace(body) != "CLAUDE_PLANNING_MARKER" {
		t.Fatalf("prompt=%q want Claude prompt marker", body)
	}
}

func TestStageAgentReportParserAcceptsFencedJSONWithText(t *testing.T) {
	raw := "Report follows:\n```json\n" +
		`{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":["task-01"],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}` +
		"\n```\nDone."
	report := parseStageAgentReport(raw, "generic_agent")
	if !report.Valid || report.Status != "complete" || !report.TestsPassed || !report.AcceptanceGates {
		t.Fatalf("report=%+v", report)
	}
	if !report.AcceptanceValues["completed_tasks_verified"] {
		t.Fatalf("acceptance values=%v", report.AcceptanceValues)
	}
}

func TestStageAgentReportParserRequiresTaskArrays(t *testing.T) {
	raw := `{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`
	report := parseStageAgentReport(raw, "generic_agent")
	if report.Valid || !strings.Contains(report.ValidationError, "tasks_remaining") {
		t.Fatalf("report=%+v", report)
	}
}

func TestStageAgentReportParserRequiresPartialRecoveryFields(t *testing.T) {
	raw := `{"stage":"implementation","agent":"generic_agent","status":"partial","tasks_completed":["task-01"],"tasks_remaining":["task-02"],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":false,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`
	report := parseStageAgentReport(raw, "generic_agent")
	if report.Valid || !strings.Contains(report.ValidationError, "blocker") {
		t.Fatalf("report=%+v", report)
	}
}

func TestStageAgentReportParserRejectsScalarAcceptanceGate(t *testing.T) {
	raw := `{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":true,"summary":"test"}`
	report := parseStageAgentReport(raw, "generic_agent")
	if report.Valid {
		t.Fatalf("scalar acceptance gate must be invalid: %+v", report)
	}
}

func TestStageAgentReportParserRequiresImplementationIdentity(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		agent string
		want  string
	}{
		{
			name:  "missing stage",
			raw:   `{"agent":"backend_agent","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`,
			agent: "backend_agent",
			want:  "stage",
		},
		{
			name:  "wrong stage",
			raw:   `{"stage":"qa","agent":"backend_agent","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`,
			agent: "backend_agent",
			want:  "implementation",
		},
		{
			name:  "missing agent",
			raw:   `{"stage":"implementation","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`,
			agent: "backend_agent",
			want:  "agent",
		},
		{
			name:  "agent differs from Execute prefix",
			raw:   `{"stage":"implementation","agent":"frontend_agent","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`,
			agent: "backend_agent",
			want:  "agent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := parseStageAgentReport(tt.raw, tt.agent)
			if report.Valid || !strings.Contains(report.ValidationError, tt.want) {
				t.Fatalf("report=%+v want error containing %q", report, tt.want)
			}
		})
	}
}

func TestStageAgentReportParserRequiresCanonicalAcceptanceGates(t *testing.T) {
	tests := []struct {
		name      string
		gates     string
		status    string
		wantValid bool
		wantGates bool
	}{
		{
			name:      "all canonical gates true for complete",
			gates:     `{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true}`,
			status:    "complete",
			wantValid: true,
			wantGates: true,
		},
		{
			name:      "partial may report a false canonical gate",
			gates:     `{"completed_tasks_verified":false,"task_ownership_respected":true,"required_tests_passed":true}`,
			status:    "partial",
			wantValid: true,
			wantGates: false,
		},
		{
			name:      "complete cannot report a false canonical gate",
			gates:     `{"completed_tasks_verified":false,"task_ownership_respected":true,"required_tests_passed":true}`,
			status:    "complete",
			wantValid: false,
		},
		{
			name:      "partial rejects unknown additional gate",
			gates:     `{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true,"extra_gate":false}`,
			status:    "partial",
			wantValid: false,
		},
		{
			name:      "complete rejects unknown additional gate",
			gates:     `{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true,"extra_gate":false}`,
			status:    "complete",
			wantValid: false,
		},
		{
			name:      "missing canonical key",
			gates:     `{"completed_tasks_verified":true,"required_tests_passed":true}`,
			status:    "partial",
			wantValid: false,
		},
		{
			name:      "nonboolean canonical value",
			gates:     `{"completed_tasks_verified":"yes","task_ownership_respected":true,"required_tests_passed":true}`,
			status:    "partial",
			wantValid: false,
		},
		{
			name:      "null canonical value",
			gates:     `{"completed_tasks_verified":null,"task_ownership_respected":true,"required_tests_passed":true}`,
			status:    "partial",
			wantValid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := fmt.Sprintf(`{"stage":"implementation","agent":"generic_agent","status":%q,"tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":%s,"blocker":"blocked for test","next_action":"retry test","summary":"test"}`, tt.status, tt.gates)
			report := parseStageAgentReport(raw, "generic_agent")
			if report.Valid != tt.wantValid {
				t.Fatalf("valid=%v want %v; report=%+v", report.Valid, tt.wantValid, report)
			}
			if tt.wantValid && report.AcceptanceGates != tt.wantGates {
				t.Fatalf("acceptance gates=%v want %v; values=%v", report.AcceptanceGates, tt.wantGates, report.AcceptanceValues)
			}
		})
	}
}

func TestValidateStageAgentTaskIDs(t *testing.T) {
	tests := []struct {
		name     string
		assigned map[string]string
		report   stageAgentReport
		wantErr  string
	}{
		{
			name:     "complete exact union",
			assigned: map[string]string{"task-back-1": agentBackend, "task-back-2": agentBackend},
			report:   stageAgentReport{Agent: agentBackend, Status: "complete", TasksCompleted: []string{"task-back-1", "task-back-2"}},
		},
		{
			name:     "partial exact union",
			assigned: map[string]string{"task-back-1": agentBackend, "task-back-2": agentBackend},
			report:   stageAgentReport{Agent: agentBackend, Status: "partial", TasksCompleted: []string{"task-back-1"}, TasksRemaining: []string{"task-back-2"}},
		},
		{
			name:     "unknown task",
			assigned: map[string]string{"task-back-1": agentBackend},
			report:   stageAgentReport{Agent: agentBackend, Status: "partial", TasksCompleted: []string{"task-other"}},
			wantErr:  "unassigned_id",
		},
		{
			name:     "task owned by another agent",
			assigned: map[string]string{"task-back-1": agentBackend, "task-front-1": agentFrontend},
			report:   stageAgentReport{Agent: agentBackend, Status: "partial", TasksCompleted: []string{"task-front-1"}},
			wantErr:  "unassigned_id",
		},
		{
			name:     "omitted assigned task",
			assigned: map[string]string{"task-back-1": agentBackend, "task-back-2": agentBackend},
			report:   stageAgentReport{Agent: agentBackend, Status: "partial", TasksCompleted: []string{"task-back-1"}},
			wantErr:  "assignment_union_mismatch",
		},
		{
			name:     "duplicate task ID",
			assigned: map[string]string{"task-back-1": agentBackend},
			report:   stageAgentReport{Agent: agentBackend, Status: "partial", TasksCompleted: []string{"task-back-1", "task-back-1"}},
			wantErr:  "duplicate_id",
		},
		{
			name:     "intersection between completed and remaining",
			assigned: map[string]string{"task-back-1": agentBackend},
			report:   stageAgentReport{Agent: agentBackend, Status: "partial", TasksCompleted: []string{"task-back-1"}, TasksRemaining: []string{"task-back-1"}},
			wantErr:  "overlapping_arrays",
		},
		{
			name:     "complete with remaining task",
			assigned: map[string]string{"task-back-1": agentBackend},
			report:   stageAgentReport{Agent: agentBackend, Status: "complete", TasksRemaining: []string{"task-back-1"}},
			wantErr:  "complete reports",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStageAgentTaskIDs(tt.assigned, tt.report)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error=%v want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestImplementationAssignmentIncludesPendingTasksAndVerification(t *testing.T) {
	checklist := implementationChecklist{
		Path:   "openspec/changes/demo/tasks.md",
		Linked: true,
		Ready:  true,
		Pending: []string{
			"1.1 [task-01-protocol] Implement protocol",
			"2.1 [task-02-state] Implement state",
		},
	}
	tasks := []implementationTaskBlock{
		{ID: "task-01-protocol", Block: "- [ ] 1.1 [task-01-protocol] Implement protocol\n  Acceptance: protocol is covered."},
		{ID: "task-02-state", Block: "- [ ] 2.1 [task-02-state] Implement state\n  Verify: state tests pass."},
	}
	prompt := formatImplementationAssignment(checklist, tasks, 2)
	for _, want := range []string{
		"openspec/changes/demo/tasks.md",
		"task-01-protocol",
		"task-02-state",
		"docs/testing/TESTING.md",
		"go test ./...",
		"openspec validate <change> --strict",
		"wave 2",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("assignment missing %q: %s", want, prompt)
		}
	}
}

func TestImplementationAssignmentWithoutLinkedChecklistBlocksInference(t *testing.T) {
	prompt := formatImplementationAssignment(implementationChecklist{}, nil, 1)
	for _, want := range []string{"No OpenSpec tasks file is linked", "report status blocked", "openspec_change"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("assignment missing %q: %s", want, prompt)
		}
	}
}

func TestStageHandoffIncompletePromptNeverOrdersClose(t *testing.T) {
	prompt := tuiHeroStartContinueAfterIncompleteStagePreamble("implementation", "report invalid")
	for _, forbidden := range []string{"Close the stage", "Stage Close Sequence", "hero stage close", "hero approve"} {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(forbidden)) {
			t.Fatalf("incomplete prompt contains close instruction %q: %s", forbidden, prompt)
		}
	}
	if !strings.Contains(prompt, "Keep the stage Running") {
		t.Fatalf("missing running intervention instruction: %s", prompt)
	}
}

func TestEscalatedStageIsNotExecutable(t *testing.T) {
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
	if m.stageInteractive(stageImplementation) {
		t.Fatal("Escalated stage must not be interactive")
	}
	if got := m.runningStageAgents(); len(got) != 0 {
		t.Fatalf("Escalated stage agents=%v want none", got)
	}
}

func TestEvaluateImplementationHandoffRequiresChecklistAndCompleteReport(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	if err := svc.SetOpenspecChange("demo"); err != nil {
		t.Fatal(err)
	}
	changeDir := filepath.Join(dir, "openspec", "changes", "demo")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(changeDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte("- [x] 1.1 [task-01] done\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewTestModel(svc)
	m.stageHandoffExpectedAgents = []string{agentGeneric}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{0: {agentGeneric: {}}}
	m.stageHandoffOutputs = []string{"generic_agent:\n" +
		`{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`}
	decision := m.evaluateStageHandoff(stageImplementation, strings.Join(m.stageHandoffOutputs, "\n"))
	if !decision.Complete || decision.PartialProgress {
		t.Fatalf("decision=%+v want complete", decision)
	}

	if err := os.WriteFile(tasksPath, []byte("- [ ] 1.1 [task-01] todo\n- [ ] 1.2 [task-02] pending\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	decision = m.evaluateStageHandoff(stageImplementation, strings.Join(m.stageHandoffOutputs, "\n"))
	if decision.Complete {
		t.Fatalf("decision=%+v must not complete with unchecked tasks", decision)
	}
}

func TestEvaluateImplementationVerificationRevalidatesCurrentPlanBeforeComplete(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeImplementationTasks(t, svc,
		"- [x] [task-done] [agent:generic_agent] Already complete\n"+
			"- [y] [task-invalid] [agent:generic_agent] Invalid checkbox\n")
	m := NewTestModel(svc)
	m.stageHandoffWave = 1
	m.stageHandoffExpectedAgents = []string{agentGeneric}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{1: {agentGeneric: {}}}
	m.stageHandoffOutputs = []string{"generic_agent:\n" +
		`{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`}

	decision := m.evaluateStageHandoff(stageImplementation, "")
	if decision.Complete || !strings.Contains(decision.Reason, "ownership plan is invalid") {
		t.Fatalf("decision=%+v must fail closed on invalid verification checklist", decision)
	}
}

func TestEvaluateImplementationHandoffStartsFreshWaveOnlyAfterProgress(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	if err := svc.SetOpenspecChange("demo"); err != nil {
		t.Fatal(err)
	}
	changeDir := filepath.Join(dir, "openspec", "changes", "demo")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("- [ ] 1.1 [task-01] todo\n- [ ] 1.2 [task-02] pending\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := "generic_agent:\n" +
		`{"stage":"implementation","agent":"generic_agent","status":"partial","tasks_completed":["task-01"],"tasks_remaining":["task-02"],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"blocker":"task-02 remains","next_action":"implement task-02","summary":"test"}`
	m := NewTestModel(svc)
	m.stageHandoffWave = 1
	m.stageHandoffExpectedAgents = []string{agentGeneric}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{1: {
		agentGeneric: {
			{ID: "task-01", Owner: agentGeneric},
			{ID: "task-02", Owner: agentGeneric},
		},
	}}
	m.stageHandoffOutputs = []string{report}
	m.stageHandoffPendingBefore = []string{"1.1 [task-01] done", "1.2 [task-02] pending"}
	decision := m.evaluateStageHandoff(stageImplementation, report)
	if !decision.PartialProgress || decision.Complete {
		t.Fatalf("decision=%+v want partial progress", decision)
	}

	m.stageHandoffPendingBefore = []string{"1.2 [task-02] pending"}
	decision = m.evaluateStageHandoff(stageImplementation, report)
	if decision.PartialProgress || decision.Complete {
		t.Fatalf("decision=%+v must require intervention without checklist progress", decision)
	}
}

func TestEvaluateImplementationHandoffFailsClosedWithoutLinkedTasks(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	m := NewTestModel(svc)
	report := "generic_agent:\n" +
		`{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`
	m.stageHandoffOutputs = []string{report}
	decision := m.evaluateStageHandoff(stageImplementation, report)
	if decision.Complete || !strings.Contains(decision.Reason, "no linked OpenSpec") {
		t.Fatalf("decision=%+v", decision)
	}
}

const qaHandoffYAML = `title: TUI QA Handoff
objective: test
scope:
  native: true
agents:
  orchestration_agent:
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
  qa_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
  generic_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
stages:
  research:
    enabled: false
    max_iterations: 1
    require_human_approval: false
  planning:
    enabled: false
    max_iterations: 1
    require_human_approval: false
  implementation:
    enabled: true
    max_iterations: 2
    require_human_approval: false
  qa:
    enabled: true
    max_iterations: 2
    require_human_approval: false
`

func TestEvaluateQAFailedHandoffInvokesAtomicClose(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", qaHandoffYAML)
	if err := svc.CloseStage("implementation", "completed", "", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	report := `qa_agent:
{"status":"failed","summary":"handoff failure","failures":[{"owner":"generic_agent","file":"internal/tui/stage_handoff.go","issue":"missing atomic close","acceptance_criteria":"scheduler owns failed close"}]}`
	m := NewTestModel(svc)
	m.stageHandoffOutputs = []string{report}
	decision := m.evaluateValidationStageHandoff(stageQA, report)
	if !decision.SchedulerHandledFailure || decision.Complete {
		t.Fatalf("decision=%+v want scheduler-handled failure", decision)
	}
	if !strings.Contains(decision.ChatCopy, "Loop-back QA → Implementation") || !strings.Contains(decision.ChatCopy, "find-qa-1") {
		t.Fatalf("chat copy=%q", decision.ChatCopy)
	}
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	f, err := svc.Store.GetFinding(cycleRow.ID, "find-qa-1")
	if err != nil || f.Status != store.FindingStatusOpen {
		t.Fatalf("finding=%+v err=%v", f, err)
	}
	impl, err := svc.Store.GetStage(cycleRow.ID, "implementation")
	if err != nil || impl.Status != store.StageWaiting {
		t.Fatalf("implementation stage=%+v err=%v want Waiting after loop-back", impl, err)
	}
}

func TestImplementationHandoffMarksFindingDone(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeImplementationTasks(t, svc, "- [ ] 1.1 [task-done] [agent:generic_agent] Done task\n")
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Store.PersistFinding(store.FindingInput{
		CycleID:            cycleRow.ID,
		SourceStage:        store.FindingSourceQA,
		Owner:              store.FindingOwnerGeneric,
		File:               "internal/tui/stage_handoff.go",
		Issue:              "fix handoff",
		AcceptanceCriteria: "finding is verified in implementation report",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.stageHandoffWave = 1
	m.stageHandoffExpectedAgents = []string{agentGeneric}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{1: {
		agentGeneric: {
			{ID: "task-done", Owner: agentGeneric},
			{ID: "find-qa-1", Owner: agentGeneric},
		},
	}}
	m.stageHandoffPendingBefore = []string{"1.1 [task-done] todo"}
	report := `generic_agent:
{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":["task-done","find-qa-1"],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test"}`
	m.stageHandoffOutputs = []string{report}
	decision := m.evaluateStageHandoff(stageImplementation, "")
	if !decision.Complete {
		t.Fatalf("decision=%+v want complete", decision)
	}
	f, err := svc.Store.GetFinding(cycleRow.ID, "find-qa-1")
	if err != nil || f.Status != store.FindingStatusDone {
		t.Fatalf("finding=%+v err=%v", f, err)
	}
}

func TestStageAgentReportParserRejectsUnknownField(t *testing.T) {
	raw := `{"stage":"implementation","agent":"generic_agent","status":"complete","tasks_completed":[],"tasks_remaining":[],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"summary":"test","unexpected_field":true}`
	report := parseStageAgentReport(raw, "generic_agent")
	if report.Valid || !strings.Contains(report.ValidationError, "unknown_field") {
		t.Fatalf("report=%+v want unknown_field rejection", report)
	}
}

func TestEvaluateImplementationHandoffFindingsOnlyPartialProgress(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	if err := svc.SetOpenspecChange("demo"); err != nil {
		t.Fatal(err)
	}
	changeDir := filepath.Join(dir, "openspec", "changes", "demo")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Findings-only wave: every OpenSpec task is already checked.
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("- [x] 1.1 [task-done] done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range []string{"finding one", "finding two"} {
		if _, err := svc.Store.PersistFinding(store.FindingInput{
			CycleID: cycleRow.ID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerGeneric,
			File: "internal/tui/stage_handoff.go", Requirement: "PRD",
			Issue: issue, AcceptanceCriteria: "fix " + issue,
		}); err != nil {
			t.Fatal(err)
		}
	}
	m := NewTestModel(svc)
	m.stageHandoffWave = 1
	m.stageHandoffExpectedAgents = []string{agentGeneric}
	m.stageHandoffAssignments = map[int]map[string][]implementationTaskBlock{1: {
		agentGeneric: {
			{ID: "find-qa-1", Owner: agentGeneric},
			{ID: "find-qa-2", Owner: agentGeneric},
		},
	}}
	m.stageHandoffPendingBefore = nil
	report := "generic_agent:\n" +
		`{"stage":"implementation","agent":"generic_agent","status":"partial","tasks_completed":["find-qa-1"],"tasks_remaining":["find-qa-2"],"tests_passed":true,"acceptance_gates":{"completed_tasks_verified":true,"task_ownership_respected":true,"required_tests_passed":true},"blocker":"find-qa-2 remains","next_action":"fix find-qa-2","summary":"test"}`
	m.stageHandoffOutputs = []string{report}
	decision := m.evaluateStageHandoff(stageImplementation, report)
	if !decision.PartialProgress || decision.Complete {
		t.Fatalf("decision=%+v want findings-only partial progress", decision)
	}
	f, err := svc.Store.GetFinding(cycleRow.ID, "find-qa-1")
	if err != nil || f.Status != store.FindingStatusDone {
		t.Fatalf("find-qa-1=%+v err=%v", f, err)
	}
}
