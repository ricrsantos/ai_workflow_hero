package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/engine"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
)

type budgetHarnessStub struct {
	mu      sync.Mutex
	cancels []string
}

func (*budgetHarnessStub) Name() string                      { return "budget-stub" }
func (*budgetHarnessStub) IsAvailable(context.Context) error { return nil }
func (*budgetHarnessStub) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return &harness.Session{ID: "stub"}, nil
}
func (*budgetHarnessStub) ResumeSession(context.Context, string) error { return nil }
func (*budgetHarnessStub) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return &harness.ExecutionResult{}, nil
}
func (h *budgetHarnessStub) Cancel(_ context.Context, sessionID string) error {
	h.mu.Lock()
	h.cancels = append(h.cancels, sessionID)
	h.mu.Unlock()
	return nil
}
func (*budgetHarnessStub) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{State: harness.StatusIdle}, nil
}
func (*budgetHarnessStub) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}
func (*budgetHarnessStub) CheckHealth(context.Context, string) (harness.HarnessHealth, error) {
	return harness.HarnessHealth{ProcessAlive: true, ServerAlive: true, SessionAlive: true}, nil
}

func (h *budgetHarnessStub) cancelledSessions() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.cancels...)
}

func newBudgetTUIFixture(t *testing.T, stageName string, timeoutMinutes int) (model, *cycle.Service, int64, *time.Time) {
	t.Helper()
	projectDir := t.TempDir()
	s, err := store.Open(filepath.Join(projectDir, "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cycleID, err := s.CreateCycle(store.Cycle{Number: 1, Title: "budget", Status: store.CycleStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]store.Stage{{
		CycleID: cycleID, Name: stageName, Status: store.StageWaiting,
		Iteration: 0, MaxIterations: 3, TimeoutMinutes: timeoutMinutes, SortOrder: 0,
	}}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	e := engine.New(s)
	e.Now = func() time.Time { return now }
	if stageName == stageBrowserUI || stageName == "qa_end_to_end" {
		if err := writeBudgetBrowserPlan(projectDir, stageName); err != nil {
			t.Fatal(err)
		}
		e.ProjectDir = projectDir
	}
	if err := e.StartStage(cycleID, stageName); err != nil {
		t.Fatal(err)
	}
	svc := &cycle.Service{ProjectDir: projectDir, Store: s, Engine: e, Harness: &budgetHarnessStub{}}
	m := NewTestModel(svc)
	m.conversationStage = stageName
	return m, svc, cycleID, &now
}

func writeBudgetBrowserPlan(projectDir, stageName string) error {
	current := filepath.Join(projectDir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(current, 0o700); err != nil {
		return err
	}
	stagePurpose := testaccess.PurposeBrowserControl
	method := testaccess.MethodPlaywrightCLINoSkill
	if stageName == "qa_end_to_end" {
		stagePurpose = testaccess.PurposeRepeatableE2E
		method = testaccess.MethodPlaywrightTestSuite
	}
	e2eCommand := ""
	if stageName == "qa_end_to_end" {
		e2eCommand = "npm run test:e2e"
	}
	config := "title: synthetic\nobjective: budget fixture\nscope:\n  frontend: true\nstages:\n"
	for _, name := range []string{stageBrowserUI, "qa_end_to_end"} {
		enabled := "false"
		if name == stageName {
			enabled = "true"
		}
		config += "  " + name + ":\n    enabled: " + enabled + "\n    timeout_minutes: 10\n"
		if name == "qa_end_to_end" {
			config += "    use_playwright: true\n"
		}
	}
	if err := os.WriteFile(filepath.Join(current, "workflow-config.yml"), []byte(config), 0o600); err != nil {
		return err
	}
	coverage := []testaccess.CoverageItem{
		{ID: "screen-dashboard-admin", RequirementRef: "FR-07/AC-1", ScreenOrJourney: "dashboard", UserID: "administrator", Profile: "admin", Mandatory: true, ExpectedResult: "protected dashboard renders", EvidenceRequirements: []string{"rendered screen"}, ProtectedTarget: testaccess.ProtectedTargetRecipe{URL: "http://127.0.0.1:43127/dashboard", ExpectedRole: "admin", ExpectedAccess: testaccess.AccessAllowed}},
		{ID: "screen-settings-operator", RequirementRef: "FR-07/AC-2", ScreenOrJourney: "settings", UserID: "operator", Profile: "operator", ExpectedResult: "settings can be opened", EvidenceRequirements: []string{}, ProtectedTarget: testaccess.ProtectedTargetRecipe{URL: "http://127.0.0.1:43127/settings", ExpectedRole: "operator", ExpectedAccess: testaccess.AccessAllowed}},
	}
	plan := testaccess.BrowserPlan{
		SchemaVersion:  testaccess.BrowserPlanSchemaVersion,
		Execution:      testaccess.BrowserExecutionContract{Environment: "synthetic local fixture", BaseURL: "http://127.0.0.1:43127", ApprovedOrigins: []string{"http://127.0.0.1:43127"}, StartCommand: "in-process fixture", ReadinessCommand: "fixture ready", E2ECommand: e2eCommand, ActionTimeout: "2s", TestTimeout: "10s", Fixtures: []string{"synthetic-local"}, EvidencePaths: []string{"current/screenshots"}},
		Authentication: testaccess.BrowserAuthenticationPlan{Requirement: testaccess.AuthenticationRequired, Flow: testaccess.AuthenticationForm, Login: testaccess.LoginFormRecipe{EntryURL: "http://127.0.0.1:43127/login", LoginLocator: "#login", PasswordLocator: "#password", SubmitLocator: "button[type=submit]"}},
		Method:         testaccess.BrowserMethodPlan{Stage: testaccess.ValidationStage(stageName), Purpose: stagePurpose, Method: method, ToolName: "playwright", ToolVersion: testaccess.MinimumPlaywrightVersion, ToolVersionCommand: []string{"playwright", "--version"}, PlaywrightVersion: testaccess.MinimumPlaywrightVersion, ExistingPlaywrightSuite: method == testaccess.MethodPlaywrightTestSuite},
		Coverage:       coverage,
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(current, "browser-plan.json"), data, 0o600)
}

func newBudgetExecute(id string, cycleID int64, stageName string, generation int64) convExecute {
	return convExecute{
		ID: id, StageName: stageName, BudgetCycleID: cycleID,
		BudgetGeneration: generation, AgentName: "qa_agent", SessionID: id + "-session",
		AgentMsgIndex: 1, Wave: 1,
	}
}

func TestActiveBudgetHumanWaitPausesOnlyWithoutRunnableSibling(t *testing.T) {
	m, svc, cycleID, now := newBudgetTUIFixture(t, "qa", 10)
	b, err := svc.Engine.StageBudget(cycleID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	m.executes = map[string]convExecute{
		"qa-1": newBudgetExecute("qa-1", cycleID, "qa", b.Generation),
		"qa-2": newBudgetExecute("qa-2", cycleID, "qa", b.Generation),
	}
	*now = now.Add(3 * time.Minute)
	m = m.setExecuteWaiting("qa-1", "human_question")
	b, err = svc.Engine.StageBudget(cycleID, "qa")
	if err != nil || b.State != store.StageBudgetActive || b.Consumed != 0 {
		t.Fatalf("one blocked sibling paused runnable work: budget=%+v err=%v", b, err)
	}
	m = m.setExecuteWaiting("qa-2", "human_permission")
	b, err = svc.Engine.StageBudget(cycleID, "qa")
	if err != nil || b.State != store.StageBudgetWaiting || b.Consumed != 3*time.Minute {
		t.Fatalf("all-human wait did not checkpoint/pause: budget=%+v err=%v", b, err)
	}
	if b.PauseReason != "human_question" {
		t.Fatalf("pause reason=%q want stable human reason", b.PauseReason)
	}
	*now = now.Add(24 * time.Hour)
	b, err = svc.Engine.StageBudget(cycleID, "qa")
	if err != nil || b.RemainingAt(*now) != 7*time.Minute {
		t.Fatalf("human/offline wait consumed budget: remaining=%s err=%v", b.RemainingAt(*now), err)
	}
	m = m.refreshExecuteWaitReason("qa-1")
	b, err = svc.Engine.StageBudget(cycleID, "qa")
	if err != nil || b.State != store.StageBudgetActive || b.RemainingAt(*now) != 7*time.Minute {
		t.Fatalf("runnable sibling did not resume the shared clock: budget=%+v err=%v", b, err)
	}
	m = m.stopStageBudgetMonitor()
}

func TestActiveBudgetDeadlineTickCancelsOnlyStageAndRejectsLateResult(t *testing.T) {
	m, svc, cycleID, now := newBudgetTUIFixture(t, "qa", 1)
	b, err := svc.Engine.StageBudget(cycleID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	adapter := svc.Harness.(*budgetHarnessStub)
	m.stageHandoffLive = true
	m.stageHandoffWave = 1
	m.stageHandoffStage = "qa"
	m.executes = map[string]convExecute{
		"qa-1": newBudgetExecute("qa-1", cycleID, "qa", b.Generation),
		"qa-2": newBudgetExecute("qa-2", cycleID, "qa", b.Generation),
		"free": {ID: "free", Freechat: true, SessionID: "free-session"},
	}
	stopped := false
	ex := m.executes["qa-1"]
	ex.cancel = func() { stopped = true }
	m.executes["qa-1"] = ex
	m = m.startStageBudgetMonitor(cycleID, "qa", b.Generation)
	*now = now.Add(time.Minute)
	updated, cancelCmd := m.handleStageBudgetTick(stageBudgetTickMsg{
		cycleID: cycleID, stageName: "qa", generation: b.Generation,
		key: stageBudgetMonitorKey(cycleID, "qa", b.Generation),
	})
	if cancelCmd == nil {
		t.Fatal("expired deadline did not start async scoped cancellation")
	}
	if !stopped {
		t.Fatal("stage execution context was not cancelled")
	}
	if len(updated.executes) != 1 || updated.executes["free"].ID != "free" {
		t.Fatalf("expiry changed unrelated executions: %+v", updated.executes)
	}
	if len(updated.stageHandoffOutputs) != 0 {
		t.Fatalf("expiry accepted stage output: %+v", updated.stageHandoffOutputs)
	}
	stage, err := svc.Store.GetStage(cycleID, "qa")
	if err != nil || stage.Status != store.StageEscalated {
		t.Fatalf("expired stage=%+v err=%v", stage, err)
	}
	budget, err := svc.Engine.StageBudget(cycleID, "qa")
	if err != nil || budget.State != store.StageBudgetExpired {
		t.Fatalf("expired budget=%+v err=%v", budget, err)
	}
	cancelDone := cancelCmd()
	updatedModel, _ := updated.Update(cancelDone)
	updated = updatedModel.(model)
	gotSessions := adapter.cancelledSessions()
	sort.Strings(gotSessions)
	wantSessions := []string{"qa-1-session", "qa-2-session"}
	sort.Strings(wantSessions)
	if !reflect.DeepEqual(gotSessions, wantSessions) {
		t.Fatalf("cancelled sessions=%v, unrelated session must remain active", gotSessions)
	}
	// A late final result with the revoked generation is rejected before stage
	// reports/findings are recorded. Stream deltas already rendered remain as
	// evidence, but completion cannot mutate the scheduler.
	late := newBudgetExecute("late", cycleID, "qa", b.Generation)
	updated.executes[late.ID] = late
	updated.stageHandoffLive = true
	updated.stageHandoffWave = 1
	updated.runtimeAgentName = "qa_agent"
	updatedModel, cmd := updated.Update(executeDoneMsg{executeID: late.ID, result: &harness.ExecutionResult{Output: "late report"}})
	updated = updatedModel.(model)
	if len(updated.stageHandoffOutputs) != 0 {
		t.Fatalf("late result changed stage outputs: %+v", updated.stageHandoffOutputs)
	}
	entries, err := svc.Store.ListConversation(cycleID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Kind == cycle.ConversationKindStageAgentResult {
			t.Fatalf("late result persisted as scheduler input: %+v", entry)
		}
	}
	if cmd != nil {
		_, _ = updated.Update(cmd())
	}
}

func TestActiveBudgetMonitorTicksIndependentlyOfStreamTraffic(t *testing.T) {
	m, svc, cycleID, now := newBudgetTUIFixture(t, "browser_ui_validation", 1)
	b, err := svc.Engine.StageBudget(cycleID, "browser_ui_validation")
	if err != nil {
		t.Fatal(err)
	}
	ex := newBudgetExecute("browser", cycleID, "browser_ui_validation", b.Generation)
	m.executes = map[string]convExecute{ex.ID: ex}
	m.streaming = true // no stream messages are delivered during this interval
	m = m.startStageBudgetMonitor(cycleID, ex.StageName, ex.BudgetGeneration)
	t.Cleanup(func() { m = m.stopStageBudgetMonitor() })
	*now = now.Add(time.Minute)
	updated, cmd := m.handleStageBudgetTick(stageBudgetTickMsg{
		cycleID: cycleID, stageName: ex.StageName, generation: ex.BudgetGeneration,
		key: stageBudgetMonitorKey(cycleID, ex.StageName, ex.BudgetGeneration),
	})
	if cmd == nil {
		t.Fatal("independent budget tick did not cancel a silent stream at deadline")
	}
	if len(updated.executes) != 0 {
		t.Fatalf("expired execution remained acceptable: %+v", updated.executes)
	}
}

func TestActiveBudgetExpiryDuringRepetitiveStream(t *testing.T) {
	m, svc, cycleID, now := newBudgetTUIFixture(t, stageBrowserUI, 1)
	b, err := svc.Engine.StageBudget(cycleID, stageBrowserUI)
	if err != nil {
		t.Fatal(err)
	}
	ex := newBudgetExecute("browser", cycleID, stageBrowserUI, b.Generation)
	m.executes = map[string]convExecute{ex.ID: ex}
	m.stageHandoffLive = true
	m.stageHandoffWave = 1
	m.stageHandoffStage = stageBrowserUI
	m.streaming = true
	m = m.beginValidationProgress(stageBrowserUI)
	m = m.startStageBudgetMonitor(cycleID, stageBrowserUI, b.Generation)
	t.Cleanup(func() { m = m.stopStageBudgetMonitor() })
	for i := 0; i < 40; i++ {
		m.applyStreamDelta(streamDeltaMsg{executeID: ex.ID, delta: harness.StreamDelta{
			Kind: harness.StreamKindActivity, HarnessType: "transport.activity", Text: "repeated transport event",
		}})
	}
	if !m.validationProgress.active || m.validationProgress.currentID != "screen-dashboard-admin" || len(m.validationProgress.completedIDs) != 0 {
		t.Fatalf("generic activity changed the planned coverage progress: %+v", m.validationProgress)
	}
	*now = now.Add(time.Minute)
	updated, cancelCmd := m.handleStageBudgetTick(stageBudgetTickMsg{
		cycleID: cycleID, stageName: stageBrowserUI, generation: b.Generation,
		key: stageBudgetMonitorKey(cycleID, stageBrowserUI, b.Generation),
	})
	if cancelCmd == nil {
		t.Fatal("repetitive stream activity delayed independent budget expiry")
	}
	budget, err := svc.Engine.StageBudget(cycleID, stageBrowserUI)
	if err != nil || budget.State != store.StageBudgetExpired || budget.Generation == b.Generation {
		t.Fatalf("expired budget=%+v err=%v, want revoked generation", budget, err)
	}
	if len(updated.executes) != 0 || !updated.validationProgress.expired {
		t.Fatalf("expiry did not revoke stage and preserve progress evidence state: executes=%d progress=%+v", len(updated.executes), updated.validationProgress)
	}
}

func TestPassiveHealthDoesNotCancelExecutionOrBudget(t *testing.T) {
	m, svc, cycleID, _ := newBudgetTUIFixture(t, "qa", 10)
	b, err := svc.Engine.StageBudget(cycleID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	ex := newBudgetExecute("qa-execute", cycleID, "qa", b.Generation)
	m.executes = map[string]convExecute{ex.ID: ex}
	m.streaming = true
	adapter := svc.Harness.(*budgetHarnessStub)
	updated, cmd := m.handleHarnessHealthResult(harnessHealthResultMsg{
		status: harness.HealthFailed,
		health: harness.HarnessHealth{ProcessAlive: false, SessionAlive: false, Details: "health probe failed"},
	})
	if cmd != nil {
		t.Fatal("passive health path returned a corrective command")
	}
	if len(updated.executes) != 1 || !updated.streaming || len(adapter.cancelledSessions()) != 0 {
		t.Fatalf("passive health changed execution state: executes=%d streaming=%v cancelled=%v", len(updated.executes), updated.streaming, adapter.cancelledSessions())
	}
	after, err := svc.Engine.StageBudget(cycleID, "qa")
	if err != nil || after.State != store.StageBudgetActive || after.Generation != b.Generation {
		t.Fatalf("passive health changed budget state: %+v err=%v", after, err)
	}
}

func TestActiveBudgetDeadlineHasIndependentSubCheckpointTimer(t *testing.T) {
	m, svc, cycleID, now := newBudgetTUIFixture(t, "qa_end_to_end", 1)
	b, err := svc.Engine.StageBudget(cycleID, "qa_end_to_end")
	if err != nil {
		t.Fatal(err)
	}
	const remainder = 200 * time.Millisecond
	*now = now.Add(time.Minute - remainder)
	m.executes = map[string]convExecute{
		"e2e": newBudgetExecute("e2e", cycleID, "qa_end_to_end", b.Generation),
	}
	sink := conversationSinkChan(m)
	m = m.startStageBudgetMonitor(cycleID, "qa_end_to_end", b.Generation)
	*now = now.Add(remainder)

	msg := awaitSinkMsg(t, sink, 2*time.Second)
	tick, ok := msg.(stageBudgetTickMsg)
	if !ok {
		t.Fatalf("deadline message type=%T, want stageBudgetTickMsg", msg)
	}
	if tick.key != stageBudgetMonitorKey(cycleID, "qa_end_to_end", b.Generation) {
		t.Fatalf("deadline key=%q", tick.key)
	}
	updated, cancelCmd := m.handleStageBudgetTick(tick)
	if cancelCmd == nil {
		t.Fatal("sub-checkpoint deadline did not trigger expiry/cancellation")
	}
	if len(updated.executes) != 0 {
		t.Fatalf("execution remained registered after exact deadline: %+v", updated.executes)
	}
	if _, expired, err := svc.Engine.ExpireStageBudget(cycleID, "qa_end_to_end", b.Generation); err == nil || expired {
		t.Fatalf("old generation remained valid after deadline acceptance revoke: expired=%v err=%v", expired, err)
	}
	_, _ = updated.Update(cancelCmd())
}

func TestActiveBudgetUnconfirmedCancellationIsDisclosed(t *testing.T) {
	m := NewTestModel(nil)
	updatedModel, _ := m.Update(stageBudgetCancelDoneMsg{
		cycleID: 4, stageName: "qa_end_to_end", workersActive: 1,
		err: context.DeadlineExceeded,
	})
	updated := updatedModel.(model)
	if !strings.Contains(updated.convError, "Late results will be ignored") {
		t.Fatalf("unconfirmed scoped cancellation was not disclosed: %q", updated.convError)
	}
}
