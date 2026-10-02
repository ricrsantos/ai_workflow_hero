package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type stageBrowserPreparation struct {
	key        string
	cycleID    int64
	stage      string
	iteration  int
	generation int64
	agents     []string
	cancel     context.CancelFunc
	pending    bool
	result     *testaccess.BoundedPreparationResult
}

type stageBrowserPreparationDoneMsg struct {
	key    string
	result testaccess.BoundedPreparationResult
}

func stageBrowserPreparationKey(cycleID int64, stage string, iteration int, generation int64) string {
	return fmt.Sprintf("%d:%s:%d:%d", cycleID, strings.TrimSpace(stage), iteration, generation)
}

func (m model) beginStageBrowserPreparation(st store.Stage, agents []string) (model, tea.Cmd) {
	if m.svc == nil || m.svc.Engine == nil || len(agents) == 0 {
		return m, nil
	}
	cycle, err := m.svc.SessionCycle()
	if err != nil || cycle == nil {
		return m.returnStageAgentPreparationFailure(st.Name, "active cycle is unavailable for browser preparation")
	}
	generation := int64(0)
	remaining := testaccess.DefaultPreparationLimit
	if budget, budgetErr := m.svc.Engine.StageBudget(cycle.ID, st.Name); budgetErr == nil {
		generation = budget.Generation
		remaining = budget.RemainingAt(m.svc.Engine.Now())
		if budget.State != store.StageBudgetActive || remaining <= 0 {
			return m.cancelStageBudgetExecutions(cycle.ID, st.Name, budget.Generation, true)
		}
	} else if !errors.Is(budgetErr, store.ErrNotFound) {
		return m.returnStageAgentPreparationFailure(st.Name, "active stage budget could not be verified")
	}
	key := stageBrowserPreparationKey(cycle.ID, st.Name, st.Iteration, generation)
	if current := m.stageBrowserPreparation; current != nil && current.key == key {
		if current.pending || current.result != nil {
			return m, nil
		}
	}
	if m.stageBrowserPreparation != nil {
		m = m.discardStageBrowserPreparation(false)
	}
	ctx, cancel := context.WithCancel(context.Background())
	preparation := &stageBrowserPreparation{
		key: key, cycleID: cycle.ID, stage: st.Name, iteration: st.Iteration,
		generation: generation, agents: append([]string(nil), agents...), cancel: cancel, pending: true,
	}
	slog.Info("tui browser preparation started", "stage", st.Name, "cycle_id", cycle.ID, "iteration", st.Iteration, "budget_generation", generation)
	m.stageBrowserPreparation = preparation
	m = m.beginValidationProgress(st.Name)
	if generation > 0 {
		m = m.startStageBudgetMonitor(cycle.ID, st.Name, generation)
	}
	projectDir := m.svc.ProjectDir
	preparationOverride := m.stagePreparationOverride
	service := m.svc
	agent := strings.TrimSpace(agents[0])
	return m, func() tea.Msg {
		defer cancel()
		var result testaccess.BoundedPreparationResult
		if preparationOverride != nil {
			result = preparationOverride(ctx, projectDir, st.Name, agent, remaining)
		} else {
			result = runStageBrowserPreparation(ctx, service, projectDir, st.Name, agent, remaining)
		}
		return stageBrowserPreparationDoneMsg{key: key, result: result}
	}
}

func runStageBrowserPreparation(ctx context.Context, service *cycle.Service, projectDir, stage, agent string, remaining time.Duration) testaccess.BoundedPreparationResult {
	blocked := func(reason testaccess.PreparationReason) testaccess.BoundedPreparationResult {
		slog.Error("tui browser preparation setup blocked", "stage", stage, "reason", reason)
		return testaccess.BoundedPreparationResult{
			Status: testaccess.PreparationBlocked, Reason: reason,
			NextAction: "Correct the planned prerequisite outside QA, then run /hero-continue. Hero will not provision fixtures or install tools.",
		}
	}
	if ctx == nil || ctx.Err() != nil || service == nil {
		return blocked(testaccess.PreparationReasonInterrupted)
	}
	doc, err := workflowconfig.LoadCurrentDocument(projectDir)
	if err != nil || doc == nil {
		return blocked(testaccess.PreparationReasonInvalidRecipe)
	}
	plan, err := testaccess.LoadBrowserPlan(projectDir)
	if err != nil {
		return blocked(testaccess.PreparationReasonInvalidRecipe)
	}
	wantStage := testaccess.StageBrowserUIValidation
	if stage == stageQAEndToEnd {
		wantStage = testaccess.StageQAEndToEnd
	}
	if plan.Method.Stage != "" && plan.Method.Stage != wantStage {
		return blocked(testaccess.PreparationReasonInvalidRecipe)
	}
	if len(plan.Coverage) == 0 {
		return blocked(testaccess.PreparationReasonInvalidRecipe)
	}
	var selectedHarness testaccess.PreparationHarness
	harnessID := ""
	if !plan.IsHTTPOnlyE2E() {
		pair, _, pairErr := workflowconfig.AgentPairFor(projectDir, agent)
		if pairErr != nil || strings.TrimSpace(pair.Harness) == "" {
			return blocked(testaccess.PreparationReasonBrowserPermission)
		}
		harnessID = pair.Harness
		if service.Harness != nil {
			selectedHarness = service.Harness
		} else if service.Registry != nil {
			adapter, adapterErr := service.Registry.Adapter(harnessID)
			if adapterErr == nil {
				selectedHarness = adapter
			}
		}
		if selectedHarness == nil || !strings.EqualFold(strings.TrimSpace(selectedHarness.Name()), harnessID) {
			return blocked(testaccess.PreparationReasonBrowserPermission)
		}
	}
	capability := &testaccess.RuntimePreparationCapability{
		ProjectDir: projectDir, HarnessID: harnessID, Harness: selectedHarness, Plan: &plan,
		Logger: slog.Default(),
	}
	preparer := testaccess.NewPreparer(capability, nil, slog.Default())
	return preparer.PreparePlan(ctx, projectDir, plan.Coverage[0].ID, doc.Config.TestAccess.Enabled, remaining)
}

func (m model) handleStageBrowserPreparationDone(msg stageBrowserPreparationDoneMsg) (model, tea.Cmd) {
	state := m.stageBrowserPreparation
	if state == nil || state.key != msg.key || !state.pending || m.svc == nil || m.svc.Engine == nil {
		return m, nil
	}
	state.pending = false
	state.cancel = nil
	active, err := m.svc.ActiveStage()
	if err != nil || active.Status != store.StageRunning || active.Name != state.stage || active.Iteration != state.iteration {
		return m.discardStageBrowserPreparation(false), nil
	}
	if state.generation > 0 {
		budget, budgetErr := m.svc.Engine.StageBudget(state.cycleID, state.stage)
		if budgetErr != nil || budget.Generation != state.generation || budget.State != store.StageBudgetActive || budget.RemainingAt(m.svc.Engine.Now()) <= 0 {
			return m.cancelStageBudgetExecutions(state.cycleID, state.stage, state.generation, true)
		}
	}
	if msg.result.Status != testaccess.PreparationReady {
		slog.Error("tui browser preparation blocked", "stage", state.stage, "reason", msg.result.Reason, "prerequisite", msg.result.BlockedOn)
		state.result = &msg.result
		return m.reportStageBrowserPreparationBlocked(state, msg.result)
	}
	slog.Info("tui browser preparation ready", "stage", state.stage, "cycle_id", state.cycleID, "iteration", state.iteration)
	state.result = &msg.result
	return m.startStageAgentSessions(state.agents)
}

func (m model) reportStageBrowserPreparationBlocked(state *stageBrowserPreparation, result testaccess.BoundedPreparationResult) (model, tea.Cmd) {
	ctx, err := m.svc.ValidationDecodeContextForStage(state.stage)
	if err != nil || len(ctx.CoveragePlan) == 0 {
		slog.Error("browser preparation could not resolve approved coverage", "stage", state.stage)
		return m.returnStageAgentPreparationFailure(state.stage, "approved browser coverage could not be resolved")
	}
	usePlaywright := true
	if state.stage == stageQAEndToEnd {
		doc, docErr := workflowconfig.LoadCurrentDocument(m.svc.ProjectDir)
		if docErr != nil || doc == nil {
			return m.returnStageAgentPreparationFailure(state.stage, "current E2E mode could not be verified")
		}
		stageConfig := doc.Config.Stages[state.stage]
		usePlaywright = stageConfig.UsePlaywright
	}
	method := ctx.ExpectedBrowserMethod
	if method == "" {
		plan, planErr := testaccess.LoadBrowserPlan(m.svc.ProjectDir)
		if planErr != nil {
			m = m.discardStageBrowserPreparation(false)
			return m.returnStageAgentPreparationFailure(state.stage, "approved browser method could not be resolved")
		}
		method = reportMethodForPreparation(plan.Method.Method)
		if method == "" {
			m = m.discardStageBrowserPreparation(false)
			return m.returnStageAgentPreparationFailure(state.stage, "approved browser method could not be resolved")
		}
	}
	coverageIDs := make([]string, 0, len(ctx.CoveragePlan))
	profileIDs := make([]string, 0, len(ctx.CoveragePlan))
	items := make([]reports.CoverageItem, 0, len(ctx.CoveragePlan))
	for _, coverage := range ctx.CoveragePlan {
		coverageIDs = append(coverageIDs, coverage.ID)
		if coverage.ProfileID != "" {
			profileIDs = appendUniqueValidationID(profileIDs, coverage.ProfileID)
		}
		items = append(items, reports.CoverageItem{ID: coverage.ID, Result: "blocked", Evidence: []string{}})
	}
	if result.Reason == "" {
		result.Reason = testaccess.PreparationReasonCheckFailed
	}
	if result.NextAction == "" {
		result.NextAction = "Correct the planned prerequisite outside QA, then run /hero-continue."
	}
	report := map[string]any{
		"status":      "blocked",
		"preparation": reports.Preparation{Status: "blocked", Method: method, VerifiedProfileIDs: []string{}},
		"blockers": []reports.Blocker{{
			ID: "preparation", Reason: string(result.Reason),
			AffectedCoverageIDs: coverageIDs, AffectedProfileIDs: profileIDs,
			Uncertainty: "The planned prerequisite was not verified before validation execution.", NextAction: result.NextAction,
		}},
		"coverage": reports.Coverage{PlannedIDs: coverageIDs, Items: items},
		"failures": []reports.FailureEntry{},
		"summary":  "Bounded browser preparation blocked before validation execution.",
	}
	if state.stage == stageQAEndToEnd {
		report["use_playwright"] = usePlaywright
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return m.returnStageAgentPreparationFailure(state.stage, "bounded preparation result could not be serialized safely")
	}
	agent := agentBrowserUI
	if state.stage == stageQAEndToEnd {
		agent = agentEnd2End
	}
	m.stageBrowserPreparation = nil
	m.stageHandoffLive = true
	m.stageHandoffStage = state.stage
	m.stageHandoffOutputs = []string{agent + ":\n" + string(encoded)}
	m.stageHandoffExpectedAgents = []string{agent}
	m.stageHandoffDoneKey = m.runningStageHandoffKey()
	m.stageHandoffWave = 0
	m.stageHandoffInterventionRequired = false
	m.stageHandoffPreparationError = ""
	m = m.clearReproGateState()
	return m.resumeOrchestratorAfterStageHandoff()
}

func reportMethodForPreparation(method testaccess.ExecutionMethod) string {
	switch method {
	case testaccess.MethodPlaywrightTestSuite:
		return "playwright_test"
	case testaccess.MethodPlaywrightCLI:
		return "cli_skill"
	case testaccess.MethodPlaywrightCLINoSkill:
		return "cli"
	case testaccess.MethodMCP:
		return "mcp"
	case testaccess.MethodHTTP:
		return "http"
	default:
		return ""
	}
}

func (m model) discardStageBrowserPreparation(interruptBudget bool) model {
	state := m.stageBrowserPreparation
	if state == nil {
		return m
	}
	if state.cancel != nil {
		state.cancel()
	}
	if interruptBudget && state.generation > 0 && m.svc != nil && m.svc.Engine != nil {
		if _, err := m.svc.Engine.InterruptStageBudget(state.cycleID, state.stage, state.generation, "user_interrupt"); err != nil && !errors.Is(err, store.ErrStageBudgetGeneration) {
			slog.Error("tui browser preparation budget interruption failed", "stage", state.stage)
		}
	}
	m.stageBrowserPreparation = nil
	return m
}

func blockedPreparationResult(result testaccess.BoundedPreparationResult, fallback testaccess.PreparationReason) testaccess.BoundedPreparationResult {
	if result.Status == "" {
		result.Status = testaccess.PreparationBlocked
	}
	if result.Reason == "" {
		result.Reason = fallback
	}
	if result.NextAction == "" {
		result.NextAction = "Correct the planned prerequisite outside QA, then run /hero-continue."
	}
	return result
}
