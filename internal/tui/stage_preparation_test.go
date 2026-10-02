package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
)

func TestStageBrowserPreparationBlocksBeforeDispatchWithoutConsumingIteration(t *testing.T) {
	m, svc, cycleID, _ := newBudgetTUIFixture(t, stageBrowserUI, 10)
	m.stagePreparationOverride = func(context.Context, string, string, string, time.Duration) testaccess.BoundedPreparationResult {
		return testaccess.BoundedPreparationResult{
			Status: testaccess.PreparationBlocked, Reason: testaccess.PreparationReasonToolUnavailable,
			BlockedOn: testaccess.PrerequisiteMethodAdmission, NextAction: "Install or enable Playwright outside QA, then run /hero-continue.",
		}
	}

	stage, err := svc.ActiveStage()
	if err != nil {
		t.Fatal("load active Browser UI stage")
	}
	next, prepareCmd := m.startStageAgentSessions([]string{agentBrowserUI})
	if prepareCmd == nil || len(next.executes) != 0 || next.stageHandoffWave != 0 {
		t.Fatalf("preparation should be pending without dispatch or validation wave: cmd=%t executes=%d wave=%d", prepareCmd != nil, len(next.executes), next.stageHandoffWave)
	}
	if next.stageBrowserPreparation == nil || !next.stageBrowserPreparation.pending || next.validationProgress.phase != "preparation" {
		t.Fatalf("bounded preparation was not tracked as active stage work: %+v", next.stageBrowserPreparation)
	}

	updatedModel, _ := next.Update(prepareCmd())
	updated := updatedModel.(model)
	if len(updated.executes) != 0 || updated.stageBrowserPreparation != nil {
		t.Fatalf("blocked preflight must not dispatch an agent: executes=%d preparation=%+v", len(updated.executes), updated.stageBrowserPreparation)
	}
	closed, err := svc.Store.GetStage(cycleID, stageBrowserUI)
	if err != nil || closed.Status != store.StageBlocked || closed.Iteration != stage.Iteration-1 {
		t.Fatalf("blocked preflight stage=%+v err=%v; want Blocked without a consumed validation iteration", closed, err)
	}
	budget, err := svc.Engine.StageBudget(cycleID, stageBrowserUI)
	if err != nil || budget.State != store.StageBudgetBlocked {
		t.Fatalf("blocked preflight budget=%+v err=%v; want blocked budget state", budget, err)
	}
	var output strings.Builder
	for _, message := range updated.transcript {
		output.WriteString(message.content)
	}
	if !strings.Contains(output.String(), "Playwright") || !strings.Contains(output.String(), "/hero-continue") {
		t.Fatalf("blocked preparation omitted safe correction guidance: %s", output.String())
	}
}

func TestInterruptDuringStageBrowserPreparationRejectsLateReadyResult(t *testing.T) {
	m, svc, cycleID, _ := newBudgetTUIFixture(t, stageBrowserUI, 10)
	m.stagePreparationOverride = func(context.Context, string, string, string, time.Duration) testaccess.BoundedPreparationResult {
		return testaccess.BoundedPreparationResult{Status: testaccess.PreparationReady}
	}
	stage, err := svc.ActiveStage()
	if err != nil {
		t.Fatal("load active Browser UI stage")
	}
	pending, prepareCmd := m.beginStageBrowserPreparation(stage, []string{agentBrowserUI})
	if prepareCmd == nil || pending.stageBrowserPreparation == nil {
		t.Fatal("preparation command did not start")
	}
	cancelCmd := pending.cancelStreamCmd()
	if cancelCmd == nil || pending.stageBrowserPreparation != nil {
		t.Fatal("Ctrl+C did not synchronously revoke pending preparation state")
	}
	budget, err := svc.Engine.StageBudget(cycleID, stageBrowserUI)
	if err != nil || budget.State != store.StageBudgetInterrupted {
		t.Fatalf("interrupted preparation budget=%+v err=%v", budget, err)
	}

	updatedModel, _ := pending.Update(prepareCmd())
	updated := updatedModel.(model)
	if len(updated.executes) != 0 || updated.stageBrowserPreparation != nil {
		t.Fatalf("late preflight completion was accepted after Ctrl+C: executes=%d state=%+v", len(updated.executes), updated.stageBrowserPreparation)
	}
	afterCancel, err := svc.Store.GetStage(cycleID, stageBrowserUI)
	if err != nil || afterCancel.Status != store.StageEscalated || afterCancel.Iteration != stage.Iteration {
		t.Fatalf("user interrupt did not preserve an explicit continuation boundary: %+v err=%v", afterCancel, err)
	}
	afterLateResult, err := svc.Store.GetStage(cycleID, stageBrowserUI)
	if err != nil || afterLateResult.Status != afterCancel.Status || afterLateResult.Iteration != afterCancel.Iteration {
		t.Fatalf("late preflight result changed stage state: %+v err=%v", afterLateResult, err)
	}
	if cancelCmd != nil {
		_, _ = updated.Update(cancelCmd())
	}
}
